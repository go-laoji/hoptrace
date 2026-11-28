package snmpx

import (
	"context"
	"strings"

	"hoptrace/internal/biz"
)

const (
	// RUIJIE-CPU-MIB / MY-CPU-MIB (企业号 4881, RGOS 交换机/路由器)
	oidRuijieCPU5s = "1.3.6.1.4.1.4881.1.1.10.2.36.1.1.1"
	oidRuijieCPU1m = "1.3.6.1.4.1.4881.1.1.10.2.36.1.1.2"
	oidRuijieCPU5m = "1.3.6.1.4.1.4881.1.1.10.2.36.1.1.3"

	// RUIJIE-MEMORY-MIB / MY-MEMORY-MIB (RGOS 内存池利用率 %)
	oidRuijieMemPoolUtil = "1.3.6.1.4.1.4881.1.1.10.2.35.1.1.1.3"

	// RUIJIE-ENTITY-MIB / RUIJIE-SYSTEM-MIB (RGOS 温度、序列号与软件版本)
	oidRuijieModuleTemp   = "1.3.6.1.4.1.4881.1.1.10.2.21.1.4.1.3"
	oidRuijieDeviceSerial = "1.3.6.1.4.1.4881.1.1.10.2.21.1.2.1.1.6"
	oidRuijieSysSwVersion = "1.3.6.1.4.1.4881.1.1.10.2.1.1.2.0"
	oidRuijieSysSerial    = "1.3.6.1.4.1.4881.1.1.10.2.1.1.26.0"

	// 锐捷 NBR / NTOS 网关私有 MIB (1.3.6.1.4.1.4881.1.6.1.2, 已在 RG-NBR-N7204-E v1.11 真机校准)
	oidRuijieNbrCPUUsage  = "1.3.6.1.4.1.4881.1.6.1.2.3.1.3.0" // 整机 CPU 利用率 (%)
	oidRuijieNbrCPU5m     = "1.3.6.1.4.1.4881.1.6.1.2.3.1.4.0" // CPU 兜底 (%)
	oidRuijieNbrMemUsage  = "1.3.6.1.4.1.4881.1.6.1.2.4.1.7.0" // 实际内存利用率 (%)
	oidRuijieNbrSerial    = "1.3.6.1.4.1.4881.1.6.1.2.5.1.9.0" // 设备序列号
	oidRuijieNbrHwVer     = "1.3.6.1.4.1.4881.1.6.1.2.5.1.10.0"
	oidRuijieNbrOSVersion = "1.3.6.1.4.1.4881.1.6.1.2.5.1.11.0" // 如 "NBR_NTOS 1.0R13P1"
)

type ruijieDriver struct{ baseDriver }

func (ruijieDriver) Name() string { return "Ruijie" }

func (ruijieDriver) Match(sys biz.SysInfo) bool {
	if hasEnterprisePrefix(sys.ObjectID, "4881") {
		return true
	}
	lower := strings.ToLower(sys.Descr)
	return strings.Contains(lower, "ruijie") ||
		strings.Contains(lower, "reyee") ||
		strings.Contains(lower, "rgos") ||
		strings.Contains(lower, "rg-")
}

func (ruijieDriver) ReadHealth(ctx context.Context, sess Session) (cpu, mem, temp int) {
	// 1. 优先 RGOS 交换机/路由器私有 MIB (4881.1.1.10.2)
	cpu = walkMaxInt(ctx, sess, oidRuijieCPU1m, 1, 100)
	if cpu == 0 {
		cpu = walkMaxInt(ctx, sess, oidRuijieCPU5s, 1, 100)
	}
	if cpu == 0 {
		cpu = walkMaxInt(ctx, sess, oidRuijieCPU5m, 1, 100)
	}
	mem = walkMaxInt(ctx, sess, oidRuijieMemPoolUtil, 1, 100)
	temp = walkMaxInt(ctx, sess, oidRuijieModuleTemp, 1, 120)

	// 2. 锐捷 NBR / NTOS 网关私有 MIB (4881.1.6.1.2)
	if cpu == 0 {
		cpu = getScalarInt(ctx, sess, oidRuijieNbrCPUUsage, 1, 100)
	}
	if cpu == 0 {
		cpu = getScalarInt(ctx, sess, oidRuijieNbrCPU5m, 1, 100)
	}
	if mem == 0 {
		mem = getScalarInt(ctx, sess, oidRuijieNbrMemUsage, 1, 100)
	}
	return cpu, mem, temp
}

func (ruijieDriver) ReadEntityExtra(ctx context.Context, sess Session) EntityExtra {
	var extra EntityExtra
	extra.Serial = getScalarString(ctx, sess, oidRuijieSysSerial)
	if extra.Serial == "" {
		extra.Serial = walkFirstString(ctx, sess, oidRuijieDeviceSerial)
	}
	if extra.Serial == "" {
		extra.Serial = getScalarString(ctx, sess, oidRuijieNbrSerial)
	}

	extra.OSVersion = getScalarString(ctx, sess, oidRuijieSysSwVersion)
	if extra.OSVersion == "" {
		osVer := getScalarString(ctx, sess, oidRuijieNbrOSVersion)
		hwVer := getScalarString(ctx, sess, oidRuijieNbrHwVer)
		switch {
		case osVer != "" && hwVer != "":
			extra.OSVersion = osVer + " (v" + strings.TrimPrefix(hwVer, "v") + ")"
		case osVer != "":
			extra.OSVersion = osVer
		case hwVer != "":
			extra.OSVersion = "v" + strings.TrimPrefix(hwVer, "v")
		}
	}
	return extra
}

func (ruijieDriver) NormalizeInterface(it *biz.Interface) {
	lower := strings.ToLower(strings.TrimSpace(it.Name))
	if strings.HasPrefix(lower, "aggregateport") || isLagPrefixWithDigit(lower, "ag") {
		it.IfType = 161
	}
}
