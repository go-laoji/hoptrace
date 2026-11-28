package snmpx

import (
	"context"
	"strings"

	"hoptrace/internal/biz"
)

const (
	// HH3C-ENTITY-EXT-MIB (企业号 25506)
	oidH3CEntityCPU  = "1.3.6.1.4.1.25506.2.6.1.1.1.1.6"
	oidH3CEntityMem  = "1.3.6.1.4.1.25506.2.6.1.1.1.1.8"
	oidH3CEntityTemp = "1.3.6.1.4.1.25506.2.6.1.1.1.1.12"
)

type h3cDriver struct{ baseDriver }

func (h3cDriver) Name() string { return "H3C" }

func (h3cDriver) Match(sys biz.SysInfo) bool {
	if hasEnterprisePrefix(sys.ObjectID, "25506") {
		return true
	}
	lower := strings.ToLower(sys.Descr)
	return strings.Contains(lower, "h3c") || strings.Contains(lower, "comware")
}

func (h3cDriver) ReadHealth(ctx context.Context, sess Session) (cpu, mem, temp int) {
	return walkMaxInt(ctx, sess, oidH3CEntityCPU, 1, 100),
		walkMaxInt(ctx, sess, oidH3CEntityMem, 1, 100),
		walkMaxInt(ctx, sess, oidH3CEntityTemp, 1, 120)
}

func (h3cDriver) NormalizeInterface(it *biz.Interface) {
	lower := strings.ToLower(strings.TrimSpace(it.Name))
	if strings.HasPrefix(lower, "bridge-aggregation") ||
		strings.HasPrefix(lower, "route-aggregation") ||
		isLagPrefixWithDigit(lower, "bagg") ||
		isLagPrefixWithDigit(lower, "ragg") {
		it.IfType = 161
	}
}
