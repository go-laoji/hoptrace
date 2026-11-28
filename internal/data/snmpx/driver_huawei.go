package snmpx

import (
	"context"
	"strconv"
	"strings"

	"hoptrace/internal/biz"
)

const (
	// HUAWEI-L2MAM-MIB (华为带 VLAN 的动态 MAC 表; 当 Q-BRIDGE-MIB 为空时作为优先兜底)
	oidHwDynFdbPort = "1.3.6.1.4.1.2011.5.25.42.2.1.3.1.4"

	// HUAWEI-ENTITY-EXTENT-MIB (企业号 2011)
	oidHwEntityCPUUsage    = "1.3.6.1.4.1.2011.5.25.31.1.1.1.1.5"
	oidHwEntityMemUsage    = "1.3.6.1.4.1.2011.5.25.31.1.1.1.1.7"
	oidHwEntityTemperature = "1.3.6.1.4.1.2011.5.25.31.1.1.1.1.11"

	// HUAWEI-IF-EXT-MIB (华为 Eth-Trunk 聚合口与成员映射, 已在 S9312 VRP 5.130 真机校准)
	oidHwTrunkIfIndex   = "1.3.6.1.4.1.2011.5.25.41.1.3.3.1.4"
	oidHwTrunkMemStatus = "1.3.6.1.4.1.2011.5.25.41.1.4.1.1.2"

	// HUAWEI-WLAN-STATION-MIB (华为无线控制器 AC STA 关联表; 索引为 6 字节 MAC 十进制)
	oidHwWlanStaAPName = "1.3.6.1.4.1.2011.6.139.18.1.2.1.4"
	oidHwWlanStaSSID   = "1.3.6.1.4.1.2011.6.139.18.1.2.1.19"
	oidHwWlanStaVLAN   = "1.3.6.1.4.1.2011.6.139.18.1.2.1.24"
	oidHwWlanStaIP     = "1.3.6.1.4.1.2011.6.139.18.1.2.1.26"
	oidHwWlanStaRSSI   = "1.3.6.1.4.1.2011.6.139.18.1.2.1.40"
)

type huaweiDriver struct{ baseDriver }

func (huaweiDriver) Name() string { return "Huawei" }

func (huaweiDriver) Match(sys biz.SysInfo) bool {
	return hasEnterprisePrefix(sys.ObjectID, "2011") || strings.Contains(strings.ToLower(sys.Descr), "huawei")
}

func (huaweiDriver) ReadHealth(ctx context.Context, sess Session) (cpu, mem, temp int) {
	return walkMaxInt(ctx, sess, oidHwEntityCPUUsage, 1, 100),
		walkMaxInt(ctx, sess, oidHwEntityMemUsage, 1, 100),
		walkMaxInt(ctx, sess, oidHwEntityTemperature, 1, 120)
}

func (huaweiDriver) ReadLAGMembers(ctx context.Context, sess Session) map[int]int {
	trunkIf := map[int]int{}
	if pdus, err := sess.Walk(ctx, oidHwTrunkIfIndex); err == nil {
		for _, p := range pdus {
			if tIdx, err := strconv.Atoi(suffix(p.OID, oidHwTrunkIfIndex)); err == nil {
				trunkIf[tIdx] = int(pduInt64(p))
			}
		}
	}
	if len(trunkIf) == 0 {
		return nil
	}
	out := map[int]int{}
	if pdus, err := sess.Walk(ctx, oidHwTrunkMemStatus); err == nil {
		for _, p := range pdus {
			parts := strings.Split(suffix(p.OID, oidHwTrunkMemStatus), ".")
			if len(parts) != 2 {
				continue
			}
			tIdx, err1 := strconv.Atoi(parts[0])
			memIf, err2 := strconv.Atoi(parts[1])
			if err1 != nil || err2 != nil {
				continue
			}
			if agg := trunkIf[tIdx]; agg > 0 && agg != memIf {
				out[memIf] = agg
			}
		}
	}
	return out
}

func (huaweiDriver) ReadPrivateFDB(ctx context.Context, sess Session) []biz.FdbEntry {
	hwPorts, err := sess.Walk(ctx, oidHwDynFdbPort)
	if err != nil || len(hwPorts) == 0 {
		return nil
	}
	var out []biz.FdbEntry
	for _, p := range hwPorts {
		parts := strings.Split(suffix(p.OID, oidHwDynFdbPort), ".")
		if len(parts) < 7 {
			continue
		}
		mac := oidPartsToMAC(parts[0:6])
		vlan, err := strconv.Atoi(parts[6])
		if !biz.ValidEndpointMAC(mac) || err != nil || vlan <= 0 || vlan > 4094 {
			continue
		}
		ifIdx := int(pduInt64(p))
		if ifIdx <= 0 {
			continue
		}
		out = append(out, biz.FdbEntry{MAC: mac, VLAN: vlan, IfIndex: ifIdx})
	}
	return out
}

func (huaweiDriver) ReadWirelessSTAs(ctx context.Context, sess Session) []biz.WirelessSTA {
	byMAC := map[string]*biz.WirelessSTA{}
	get := func(mac string) *biz.WirelessSTA {
		s := byMAC[mac]
		if s == nil {
			s = &biz.WirelessSTA{MAC: mac, VLAN: 1}
			byMAC[mac] = s
		}
		return s
	}
	walkSTA := func(oid string, fn func(s *biz.WirelessSTA, p PDU)) {
		pdus, err := sess.Walk(ctx, oid)
		if err != nil {
			return
		}
		for _, p := range pdus {
			parts := strings.Split(suffix(p.OID, oid), ".")
			if len(parts) < 6 {
				continue
			}
			mac := oidPartsToMAC(parts[len(parts)-6:])
			if !biz.ValidEndpointMAC(mac) {
				continue
			}
			fn(get(mac), p)
		}
	}
	walkSTA(oidHwWlanStaAPName, func(s *biz.WirelessSTA, p PDU) { s.APName = pduString(p) })
	walkSTA(oidHwWlanStaSSID, func(s *biz.WirelessSTA, p PDU) { s.SSID = pduString(p) })
	walkSTA(oidHwWlanStaVLAN, func(s *biz.WirelessSTA, p PDU) { s.VLAN = int(pduInt64(p)) })
	walkSTA(oidHwWlanStaIP, func(s *biz.WirelessSTA, p PDU) { s.IP = pduString(p) })
	walkSTA(oidHwWlanStaRSSI, func(s *biz.WirelessSTA, p PDU) { s.RSSI = int(pduInt64(p)) })

	if len(byMAC) == 0 {
		return nil
	}
	out := make([]biz.WirelessSTA, 0, len(byMAC))
	for _, s := range byMAC {
		out = append(out, *s)
	}
	return out
}

func (huaweiDriver) NormalizeInterface(it *biz.Interface) {
	// 华为 VRP 5 的 Eth-Trunk 上报 ifType=6, 统一规范为 161 (ieee8023adLag)
	if strings.HasPrefix(strings.ToLower(it.Name), "eth-trunk") {
		it.IfType = 161
	}
}
