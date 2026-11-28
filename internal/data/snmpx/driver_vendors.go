package snmpx

import (
	"context"
	"strings"

	"hoptrace/internal/biz"
)

// ---------- Juniper (企业号 2636) ----------

const (
	oidJnxBoxSerialNo     = "1.3.6.1.4.1.2636.3.1.3.0"
	oidJnxOperatingTemp   = "1.3.6.1.4.1.2636.3.1.13.1.7"
	oidJnxOperatingCPU    = "1.3.6.1.4.1.2636.3.1.13.1.8"
	oidJnxOperatingBuffer = "1.3.6.1.4.1.2636.3.1.13.1.11"
)

type juniperDriver struct{ baseDriver }

func (juniperDriver) Name() string { return "Juniper" }

func (juniperDriver) Match(sys biz.SysInfo) bool {
	return hasEnterprisePrefix(sys.ObjectID, "2636") || strings.Contains(strings.ToLower(sys.Descr), "juniper")
}

func (juniperDriver) ReadHealth(ctx context.Context, sess Session) (cpu, mem, temp int) {
	return walkMaxInt(ctx, sess, oidJnxOperatingCPU, 1, 100),
		walkMaxInt(ctx, sess, oidJnxOperatingBuffer, 1, 100),
		walkMaxInt(ctx, sess, oidJnxOperatingTemp, 1, 120)
}

func (juniperDriver) ReadEntityExtra(ctx context.Context, sess Session) EntityExtra {
	return EntityExtra{Serial: getScalarString(ctx, sess, oidJnxBoxSerialNo)}
}

func (juniperDriver) NormalizeInterface(it *biz.Interface) {
	if isLagPrefixWithDigit(strings.ToLower(strings.TrimSpace(it.Name)), "ae") {
		it.IfType = 161
	}
}

// ---------- Fortinet (企业号 12356) ----------

const (
	oidFortiSerial   = "1.3.6.1.4.1.12356.100.1.1.1.0"
	oidFortiVersion  = "1.3.6.1.4.1.12356.101.4.1.1.0"
	oidFortiCPUUsage = "1.3.6.1.4.1.12356.101.4.1.3.0"
	oidFortiMemUsage = "1.3.6.1.4.1.12356.101.4.1.4.0"
)

type fortinetDriver struct{ baseDriver }

func (fortinetDriver) Name() string { return "Fortinet" }

func (fortinetDriver) Match(sys biz.SysInfo) bool {
	if hasEnterprisePrefix(sys.ObjectID, "12356") {
		return true
	}
	lower := strings.ToLower(sys.Descr)
	return strings.Contains(lower, "fortigate") || strings.Contains(lower, "fortinet")
}

func (fortinetDriver) ReadHealth(ctx context.Context, sess Session) (cpu, mem, temp int) {
	return getScalarInt(ctx, sess, oidFortiCPUUsage, 1, 100),
		getScalarInt(ctx, sess, oidFortiMemUsage, 1, 100),
		0
}

func (fortinetDriver) ReadEntityExtra(ctx context.Context, sess Session) EntityExtra {
	return EntityExtra{
		Serial:    getScalarString(ctx, sess, oidFortiSerial),
		OSVersion: getScalarString(ctx, sess, oidFortiVersion),
	}
}

// ---------- MikroTik (企业号 14988) ----------

const (
	oidMtxrTemp       = "1.3.6.1.4.1.14988.1.1.3.10.0" // 单位 0.1℃
	oidMtxrLicVersion = "1.3.6.1.4.1.14988.1.1.4.4.0"
	oidMtxrSerial     = "1.3.6.1.4.1.14988.1.1.7.3.0"
)

type mikrotikDriver struct{ baseDriver }

func (mikrotikDriver) Name() string { return "MikroTik" }

func (mikrotikDriver) Match(sys biz.SysInfo) bool {
	if hasEnterprisePrefix(sys.ObjectID, "14988") {
		return true
	}
	lower := strings.ToLower(sys.Descr)
	return strings.Contains(lower, "mikrotik") || strings.Contains(lower, "routeros")
}

func (mikrotikDriver) ReadHealth(ctx context.Context, sess Session) (cpu, mem, temp int) {
	// CPU 与 内存由标准 HOST-RESOURCES-MIB 兜底读取
	if raw := getScalarInt(ctx, sess, oidMtxrTemp, 10, 1200); raw > 0 {
		temp = raw / 10
	}
	return 0, 0, temp
}

func (mikrotikDriver) ReadEntityExtra(ctx context.Context, sess Session) EntityExtra {
	ver := getScalarString(ctx, sess, oidMtxrLicVersion)
	if ver != "" && !strings.HasPrefix(strings.ToLower(ver), "routeros") {
		ver = "RouterOS " + ver
	}
	return EntityExtra{
		Serial:    getScalarString(ctx, sess, oidMtxrSerial),
		OSVersion: ver,
	}
}

// ---------- Arista (企业号 30065) ----------

type aristaDriver struct{ baseDriver }

func (aristaDriver) Name() string { return "Arista" }

func (aristaDriver) Match(sys biz.SysInfo) bool {
	return hasEnterprisePrefix(sys.ObjectID, "30065") || strings.Contains(strings.ToLower(sys.Descr), "arista")
}

func (aristaDriver) NormalizeInterface(it *biz.Interface) {
	lower := strings.ToLower(strings.TrimSpace(it.Name))
	if strings.HasPrefix(lower, "port-channel") || isLagPrefixWithDigit(lower, "po") {
		it.IfType = 161
	}
}

// ---------- ZTE (企业号 3902) ----------

const (
	oidZteCPU = "1.3.6.1.4.1.3902.3.1.1.1.1.3"
	oidZteMem = "1.3.6.1.4.1.3902.3.1.1.1.1.4"
)

type zteDriver struct{ baseDriver }

func (zteDriver) Name() string { return "ZTE" }

func (zteDriver) Match(sys biz.SysInfo) bool {
	if hasEnterprisePrefix(sys.ObjectID, "3902") {
		return true
	}
	lower := strings.ToLower(sys.Descr)
	return strings.Contains(lower, "zte") || strings.Contains(lower, "zxr10")
}

func (zteDriver) ReadHealth(ctx context.Context, sess Session) (cpu, mem, temp int) {
	return walkMaxInt(ctx, sess, oidZteCPU, 1, 100),
		walkMaxInt(ctx, sess, oidZteMem, 1, 100),
		0
}

func (zteDriver) NormalizeInterface(it *biz.Interface) {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(it.Name)), "smartgroup") {
		it.IfType = 161
	}
}
