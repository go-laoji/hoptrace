package snmpx

import (
	"context"
	"strings"

	"hoptrace/internal/biz"
)

const (
	// CISCO-PROCESS-MIB (企业号 9)
	oidCiscoCPU1mRev = "1.3.6.1.4.1.9.9.109.1.1.1.1.7"
	oidCiscoCPU5sRev = "1.3.6.1.4.1.9.9.109.1.1.1.1.6"
	oidCiscoCPU1mOld = "1.3.6.1.4.1.9.9.109.1.1.1.1.4"

	// CISCO-MEMORY-POOL-MIB
	oidCiscoMemPoolUsed = "1.3.6.1.4.1.9.9.48.1.1.1.5"
	oidCiscoMemPoolFree = "1.3.6.1.4.1.9.9.48.1.1.1.6"

	// CISCO-ENVMON-MIB (温度)
	oidCiscoEnvTempValue = "1.3.6.1.4.1.9.9.13.1.3.1.3"
)

type ciscoDriver struct{ baseDriver }

func (ciscoDriver) Name() string { return "Cisco" }

func (ciscoDriver) Match(sys biz.SysInfo) bool {
	if hasEnterprisePrefix(sys.ObjectID, "9") {
		return true
	}
	lower := strings.ToLower(sys.Descr)
	return strings.Contains(lower, "cisco") || strings.Contains(lower, "nx-os")
}

func (ciscoDriver) ReadHealth(ctx context.Context, sess Session) (cpu, mem, temp int) {
	cpu = walkMaxInt(ctx, sess, oidCiscoCPU1mRev, 1, 100)
	if cpu == 0 {
		cpu = walkMaxInt(ctx, sess, oidCiscoCPU5sRev, 1, 100)
	}
	if cpu == 0 {
		cpu = walkMaxInt(ctx, sess, oidCiscoCPU1mOld, 1, 100)
	}
	mem = readCiscoMemoryPool(ctx, sess)
	temp = walkMaxInt(ctx, sess, oidCiscoEnvTempValue, 1, 120)
	return cpu, mem, temp
}

func readCiscoMemoryPool(ctx context.Context, sess Session) int {
	usedMap := map[string]int64{}
	if pdus, err := sess.Walk(ctx, oidCiscoMemPoolUsed); err == nil {
		for _, p := range pdus {
			usedMap[suffix(p.OID, oidCiscoMemPoolUsed)] = pduInt64(p)
		}
	}
	if len(usedMap) == 0 {
		return 0
	}
	best := 0
	if pdus, err := sess.Walk(ctx, oidCiscoMemPoolFree); err == nil {
		for _, p := range pdus {
			idx := suffix(p.OID, oidCiscoMemPoolFree)
			used, ok := usedMap[idx]
			if !ok {
				continue
			}
			free := pduInt64(p)
			total := used + free
			if total <= 0 {
				continue
			}
			pct := int(used * 100 / total)
			if pct > best && pct <= 100 {
				best = pct
			}
		}
	}
	return best
}

func (ciscoDriver) NormalizeInterface(it *biz.Interface) {
	lower := strings.ToLower(strings.TrimSpace(it.Name))
	if strings.HasPrefix(lower, "port-channel") || isLagPrefixWithDigit(lower, "po") {
		it.IfType = 161
	}
}
