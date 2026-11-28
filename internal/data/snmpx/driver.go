package snmpx

import (
	"context"
	"strings"

	"hoptrace/internal/biz"
)

// EntityExtra 厂商私有 MIB 对实体信息(序列号/型号/版本/堆叠)的补充。
type EntityExtra struct {
	Serial       string
	Model        string
	OSVersion    string
	StackMembers int
}

// VendorDriver 厂商私有 SNMP 采集扩展接口。
// 所有方法均为尽力而为: 未读到数据时返回零值, 由主流程或 GenericDriver 自动兜底。
type VendorDriver interface {
	Name() string
	Match(sys biz.SysInfo) bool

	ReadHealth(ctx context.Context, sess Session) (cpu, mem, temp int)
	ReadEntityExtra(ctx context.Context, sess Session) EntityExtra
	ReadLAGMembers(ctx context.Context, sess Session) map[int]int
	ReadPrivateFDB(ctx context.Context, sess Session) []biz.FdbEntry
	ReadWirelessSTAs(ctx context.Context, sess Session) []biz.WirelessSTA
	NormalizeInterface(it *biz.Interface)
}

// baseDriver 为各厂商驱动提供默认空实现, 便于按需覆写。
type baseDriver struct{}

func (baseDriver) ReadHealth(context.Context, Session) (int, int, int)       { return 0, 0, 0 }
func (baseDriver) ReadEntityExtra(context.Context, Session) EntityExtra      { return EntityExtra{} }
func (baseDriver) ReadLAGMembers(context.Context, Session) map[int]int       { return nil }
func (baseDriver) ReadPrivateFDB(context.Context, Session) []biz.FdbEntry    { return nil }
func (baseDriver) ReadWirelessSTAs(context.Context, Session) []biz.WirelessSTA { return nil }
func (baseDriver) NormalizeInterface(*biz.Interface)                         {}

var defaultDrivers = []VendorDriver{
	huaweiDriver{},
	ruijieDriver{},
	h3cDriver{},
	ciscoDriver{},
	juniperDriver{},
	fortinetDriver{},
	mikrotikDriver{},
	aristaDriver{},
	zteDriver{},
}

func matchDriver(sys biz.SysInfo) VendorDriver {
	for _, d := range defaultDrivers {
		if d.Match(sys) {
			return d
		}
	}
	return nil
}

// ---------- 公共 SNMP 读取辅助 ----------

func walkMaxInt(ctx context.Context, sess Session, oid string, lo, hi int) int {
	pdus, err := sess.Walk(ctx, oid)
	if err != nil {
		return 0
	}
	best := 0
	for _, p := range pdus {
		v := int(pduInt64(p))
		if v >= lo && v <= hi && v > best {
			best = v
		}
	}
	return best
}

func walkAvgInt(ctx context.Context, sess Session, oid string, lo, hi int) int {
	pdus, err := sess.Walk(ctx, oid)
	if err != nil || len(pdus) == 0 {
		return 0
	}
	sum, cnt := 0, 0
	for _, p := range pdus {
		v := int(pduInt64(p))
		if v >= lo && v <= hi {
			sum += v
			cnt++
		}
	}
	if cnt == 0 {
		return 0
	}
	return sum / cnt
}

func getScalarInt(ctx context.Context, sess Session, oid string, lo, hi int) int {
	pdus, err := sess.Get(ctx, []string{oid})
	if err != nil || len(pdus) == 0 {
		return 0
	}
	v := int(pduInt64(pdus[0]))
	if v < lo || v > hi {
		return 0
	}
	return v
}

func getScalarString(ctx context.Context, sess Session, oid string) string {
	pdus, err := sess.Get(ctx, []string{oid})
	if err != nil || len(pdus) == 0 {
		return ""
	}
	return pduString(pdus[0])
}

func walkFirstString(ctx context.Context, sess Session, oid string) string {
	pdus, err := sess.Walk(ctx, oid)
	if err != nil {
		return ""
	}
	for _, p := range pdus {
		if s := pduString(p); s != "" {
			return s
		}
	}
	return ""
}

func hasEnterprisePrefix(sysOID, entID string) bool {
	oid := strings.TrimPrefix(sysOID, ".")
	prefix := "1.3.6.1.4.1." + entID
	return oid == prefix || strings.HasPrefix(oid, prefix+".")
}

func isLagPrefixWithDigit(lowerName, prefix string) bool {
	if !strings.HasPrefix(lowerName, prefix) {
		return false
	}
	rest := strings.TrimSpace(lowerName[len(prefix):])
	return len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9'
}
