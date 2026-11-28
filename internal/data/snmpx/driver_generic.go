package snmpx

import (
	"context"
	"math"
	"strings"
)

// readGenericHealth 通过标准 MIB (HOST-RESOURCES-MIB / UCD-SNMP-MIB / ENTITY-SENSOR-MIB)
// 为未覆盖私有 MIB 的设备(以及 Linux/OpenWrt/ReyeeOS 架构网关)提供通用 CPU/内存/温度兜底。
func readGenericHealth(ctx context.Context, sess Session, needCPU, needMem, needTemp bool) (cpu, mem, temp int) {
	if needCPU {
		// 1. HOST-RESOURCES-MIB hrProcessorLoad (多核取均值)
		cpu = walkAvgInt(ctx, sess, oidHrProcessorLd, 0, 100)
		// 2. UCD-SNMP-MIB ssCpuIdle -> 100 - idle
		if cpu == 0 {
			if pdus, err := sess.Get(ctx, []string{oidUcdSSCpuIdle}); err == nil && len(pdus) > 0 {
				idle := int(pduInt64(pdus[0]))
				if idle >= 0 && idle <= 100 {
					cpu = 100 - idle
				}
			}
		}
	}

	if needMem {
		mem = readHostResourcesMem(ctx, sess)
		if mem == 0 {
			mem = readUCDMem(ctx, sess)
		}
	}

	if needTemp {
		temp = readEntitySensorTemp(ctx, sess)
	}
	return cpu, mem, temp
}

func readHostResourcesMem(ctx context.Context, sess Session) int {
	types, err := sess.Walk(ctx, oidHrStorageType)
	if err != nil || len(types) == 0 {
		return 0
	}
	ramIdx := map[string]bool{}
	for _, p := range types {
		if pduOID(p) == oidHrStorageRam {
			ramIdx[suffix(p.OID, oidHrStorageType)] = true
		}
	}
	if len(ramIdx) == 0 {
		if descrs, err := sess.Walk(ctx, oidHrStorageDescr); err == nil {
			for _, p := range descrs {
				d := strings.ToLower(pduString(p))
				if strings.Contains(d, "physical memory") || strings.Contains(d, "real memory") || d == "ram" || d == "main memory" {
					ramIdx[suffix(p.OID, oidHrStorageDescr)] = true
				}
			}
		}
	}
	if len(ramIdx) == 0 {
		return 0
	}

	sizes := map[string]int64{}
	if pdus, err := sess.Walk(ctx, oidHrStorageSize); err == nil {
		for _, p := range pdus {
			idx := suffix(p.OID, oidHrStorageSize)
			if ramIdx[idx] {
				sizes[idx] = pduInt64(p)
			}
		}
	}
	useds := map[string]int64{}
	if pdus, err := sess.Walk(ctx, oidHrStorageUsed); err == nil {
		for _, p := range pdus {
			idx := suffix(p.OID, oidHrStorageUsed)
			if ramIdx[idx] {
				useds[idx] = pduInt64(p)
			}
		}
	}

	best := 0
	for idx, sz := range sizes {
		if sz <= 0 {
			continue
		}
		u := useds[idx]
		if u <= 0 || u > sz {
			continue
		}
		pct := int(u * 100 / sz)
		if pct > best && pct <= 100 {
			best = pct
		}
	}
	return best
}

func readUCDMem(ctx context.Context, sess Session) int {
	pdus, err := sess.Get(ctx, []string{oidUcdMemTotalReal, oidUcdMemAvailReal, oidUcdMemBuffer, oidUcdMemCached})
	if err != nil || len(pdus) == 0 {
		return 0
	}
	var total, avail, buf, cached int64
	for _, p := range pdus {
		switch p.OID {
		case oidUcdMemTotalReal:
			total = pduInt64(p)
		case oidUcdMemAvailReal:
			avail = pduInt64(p)
		case oidUcdMemBuffer:
			buf = pduInt64(p)
		case oidUcdMemCached:
			cached = pduInt64(p)
		}
	}
	if total <= 0 {
		return 0
	}
	used := total - avail - buf - cached
	if used < 0 {
		used = total - avail
	}
	if used < 0 || used > total {
		return 0
	}
	return int(used * 100 / total)
}

func readEntitySensorTemp(ctx context.Context, sess Session) int {
	types, err := sess.Walk(ctx, oidEntPhySensorType)
	if err != nil || len(types) == 0 {
		return 0
	}
	celsiusIdx := map[string]bool{}
	for _, p := range types {
		if pduInt64(p) == 8 { // celsius(8)
			celsiusIdx[suffix(p.OID, oidEntPhySensorType)] = true
		}
	}
	if len(celsiusIdx) == 0 {
		return 0
	}
	prec := map[string]int{}
	if pdus, err := sess.Walk(ctx, oidEntPhySensorPrecision); err == nil {
		for _, p := range pdus {
			idx := suffix(p.OID, oidEntPhySensorPrecision)
			if celsiusIdx[idx] {
				prec[idx] = int(pduInt64(p))
			}
		}
	}
	best := 0
	if pdus, err := sess.Walk(ctx, oidEntPhySensorValue); err == nil {
		for _, p := range pdus {
			idx := suffix(p.OID, oidEntPhySensorValue)
			if !celsiusIdx[idx] {
				continue
			}
			v := float64(pduInt64(p))
			if pr := prec[idx]; pr > 0 && pr <= 6 {
				v /= math.Pow10(pr)
			}
			iv := int(math.Round(v))
			if iv >= 1 && iv <= 120 && iv > best {
				best = iv
			}
		}
	}
	return best
}
