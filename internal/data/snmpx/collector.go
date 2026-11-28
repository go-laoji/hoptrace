package snmpx

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	"hoptrace/internal/biz"
)

// Collector 实现 biz.Collector, 通过 SNMP 读取系统/接口/LLDP/FDB/ARP/LAG/硬件信息。
type Collector struct {
	dialer Dialer
}

func NewCollector(d Dialer) *Collector {
	return &Collector{dialer: d}
}

// Probe 轻量探测: 只读 SNMPv2-MIB 系统信息。
func (c *Collector) Probe(ctx context.Context, t biz.Target) (*biz.SysInfo, error) {
	sess, err := c.dialer.Dial(ctx, t)
	if err != nil {
		return nil, err
	}
	defer sess.Close()
	return readSysInfo(ctx, sess)
}

// Collect 完整采集一台设备的拓扑与终端基础数据。
func (c *Collector) Collect(ctx context.Context, t biz.Target) (*biz.Snapshot, error) {
	sess, err := c.dialer.Dial(ctx, t)
	if err != nil {
		return nil, err
	}
	defer sess.Close()

	sys, err := readSysInfo(ctx, sess)
	if err != nil {
		return nil, err
	}
	snap := &biz.Snapshot{Sys: *sys}
	drv := matchDriver(*sys)

	// 1. 接口 + LAG
	ifs, warn := readInterfaces(ctx, sess, drv)
	snap.Interfaces = ifs
	snap.Warnings = append(snap.Warnings, warn...)

	// 2. 本机 IP
	if pdus, err := sess.Walk(ctx, oidIPAdEntAddr); err == nil {
		for _, p := range pdus {
			ip := pduString(p)
			if ip == "" {
				ip = suffix(p.OID, oidIPAdEntAddr)
			}
			if biz.ValidEndpointIP(ip) {
				snap.IPs = append(snap.IPs, ip)
			}
		}
		sort.Strings(snap.IPs)
	}

	// 3. 底盘 MAC (优先 lldpLocChassisId, 兜底 dot1dBaseBridgeAddress, 再兜底物理口 MAC)
	snap.ChassisMAC = readChassisMAC(ctx, sess, ifs)

	// 4. LLDP 邻居
	nbs, warn := readLLDP(ctx, sess, ifs)
	snap.Neighbors = nbs
	snap.Warnings = append(snap.Warnings, warn...)

	// 5. FDB (MAC 地址表)
	fdb, warn := readFDB(ctx, sess, drv)
	snap.Fdb = fdb
	snap.Warnings = append(snap.Warnings, warn...)

	// 6. ARP 表
	arp, warn := readARP(ctx, sess)
	snap.Arp = arp
	snap.Warnings = append(snap.Warnings, warn...)

	// 7. 物理实体(序列号 / 型号 / 版本 / 堆叠成员数)
	ent := readEntity(ctx, sess, drv)
	snap.Serial = ent.Serial
	snap.Model = ent.Model
	snap.OSVersion = ent.OSVersion
	snap.StackMembers = ent.StackMembers

	// 8. 硬件健康(厂商私有 MIB 优先, 缺失项由 HOST-RESOURCES / UCD-SNMP / ENTITY-SENSOR 通用兜底)
	var cpu, mem, temp int
	if drv != nil {
		cpu, mem, temp = drv.ReadHealth(ctx, sess)
		snap.Wireless = drv.ReadWirelessSTAs(ctx, sess)
	}
	if cpu == 0 || mem == 0 || temp == 0 {
		gCPU, gMem, gTemp := readGenericHealth(ctx, sess, cpu == 0, mem == 0, temp == 0)
		if cpu == 0 {
			cpu = gCPU
		}
		if mem == 0 {
			mem = gMem
		}
		if temp == 0 {
			temp = gTemp
		}
	}
	snap.CPU, snap.Mem, snap.Temp = cpu, mem, temp

	return snap, nil
}

func readSysInfo(ctx context.Context, sess Session) (*biz.SysInfo, error) {
	pdus, err := sess.Get(ctx, []string{oidSysDescr, oidSysObjectID, oidSysUpTime, oidSysName, oidSysLocation})
	if err != nil {
		return nil, err
	}
	if len(pdus) == 0 {
		return nil, fmt.Errorf("empty SNMP sysInfo response")
	}
	info := &biz.SysInfo{}
	for _, p := range pdus {
		switch p.OID {
		case oidSysDescr:
			info.Descr = pduString(p)
		case oidSysObjectID:
			info.ObjectID = pduOID(p)
		case oidSysUpTime:
			info.UptimeSecs = pduInt64(p) / 100
		case oidSysName:
			info.Name = pduString(p)
		case oidSysLocation:
			info.Location = pduString(p)
		}
	}
	return info, nil
}

func readInterfaces(ctx context.Context, sess Session, drv VendorDriver) ([]*biz.Interface, []string) {
	var warns []string
	byIdx := map[int]*biz.Interface{}
	get := func(idx int) *biz.Interface {
		it := byIdx[idx]
		if it == nil {
			it = &biz.Interface{IfIndex: idx, Kind: biz.IfKindEdge}
			byIdx[idx] = it
		}
		return it
	}

	walkCol := func(oid string, fn func(idx int, p PDU)) {
		pdus, err := sess.Walk(ctx, oid)
		if err != nil {
			warns = append(warns, oid+": "+err.Error())
			return
		}
		for _, p := range pdus {
			idx, err := strconv.Atoi(suffix(p.OID, oid))
			if err != nil || idx <= 0 {
				continue
			}
			fn(idx, p)
		}
	}

	walkCol(oidIfDescr, func(idx int, p PDU) { get(idx).Descr = pduString(p) })
	walkCol(oidIfName, func(idx int, p PDU) { get(idx).Name = pduString(p) })
	walkCol(oidIfAlias, func(idx int, p PDU) { get(idx).Alias = pduString(p) })
	walkCol(oidIfType, func(idx int, p PDU) { get(idx).IfType = int(pduInt64(p)) })
	walkCol(oidIfSpeed, func(idx int, p PDU) {
		if get(idx).SpeedMbps == 0 {
			get(idx).SpeedMbps = pduInt64(p) / 1_000_000
		}
	})
	walkCol(oidIfHighSpeed, func(idx int, p PDU) {
		if v := pduInt64(p); v > 0 {
			get(idx).SpeedMbps = v
		}
	})
	walkCol(oidIfPhysAddress, func(idx int, p PDU) {
		if b := pduBytes(p); len(b) == 6 {
			get(idx).MAC = biz.FormatMAC(b)
		}
	})
	walkCol(oidIfAdminStatus, func(idx int, p PDU) { get(idx).AdminStatus = ifStatusStr(pduInt64(p)) })
	walkCol(oidIfOperStatus, func(idx int, p PDU) { get(idx).OperStatus = ifStatusStr(pduInt64(p)) })
	walkCol(oidIfHCInOctets, func(idx int, p PDU) { get(idx).InOctets = uint64(pduInt64(p)) })
	walkCol(oidIfHCOutOctets, func(idx int, p PDU) { get(idx).OutOctets = uint64(pduInt64(p)) })
	// 兜底 32 位计数器(部分老款百兆口或轻量网关不报 ifHC*Octets)
	walkCol(oidIfInOctets, func(idx int, p PDU) {
		if get(idx).InOctets == 0 {
			get(idx).InOctets = uint64(pduInt64(p))
		}
	})
	walkCol(oidIfOutOctets, func(idx int, p PDU) {
		if get(idx).OutOctets == 0 {
			get(idx).OutOctets = uint64(pduInt64(p))
		}
	})
	walkCol(oidIfInErrors, func(idx int, p PDU) { get(idx).InErrors = pduInt64(p) })
	walkCol(oidIfOutErrors, func(idx int, p PDU) { get(idx).OutErrors = pduInt64(p) })

	// LAG 成员关系: 优先标准 IEEE8023-LAG, 兜底厂商私有 MIB (如华为 hwTrunk)
	lagMapped := 0
	if pdus, err := sess.Walk(ctx, oidDot3adAggPortAttached); err == nil {
		for _, p := range pdus {
			idx, err := strconv.Atoi(suffix(p.OID, oidDot3adAggPortAttached))
			if err != nil {
				continue
			}
			agg := int(pduInt64(p))
			if agg > 0 && agg != idx {
				if it := byIdx[idx]; it != nil {
					it.AggIfIndex = agg
					lagMapped++
				}
			}
		}
	}
	if lagMapped == 0 && drv != nil {
		for memIf, agg := range drv.ReadLAGMembers(ctx, sess) {
			if agg > 0 && agg != memIf {
				if it := byIdx[memIf]; it != nil {
					it.AggIfIndex = agg
				}
			}
		}
	}

	var out []*biz.Interface
	for _, it := range byIdx {
		if it.Name == "" {
			it.Name = it.Descr
		}
		if drv != nil {
			drv.NormalizeInterface(it)
		}
		if !keepInterface(it) {
			continue
		}
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].IfIndex < out[j].IfIndex })
	return out, warns
}

func keepInterface(it *biz.Interface) bool {
	// 仅保留物理以太口(6/62/69/117)和链路聚合口(161)
	switch it.IfType {
	case 6, 62, 69, 117, 161:
	default:
		return false
	}
	name := strings.ToLower(strings.TrimSpace(it.Name + " " + it.Descr))
	for _, skip := range []string{
		"null", "loopback", "vlanif", "meth", "console", "inloopback",
		"virtual", "tunnel", "dialer", "pppoe", "erspan", "docker", "veth",
		"ifb", "sit0", "br-lan", "bvi",
	} {
		if strings.Contains(name, skip) {
			return false
		}
	}
	// 跳过纯 SVI/VLAN 三层接口 (如 "Vlan10" / "VLAN 10") 及 GRE 隧道口 (避免误伤 AggregatePort/Bridge-Aggregation)
	short := strings.ToLower(strings.TrimSpace(it.Name))
	if (strings.HasPrefix(short, "vlan") && !strings.Contains(short, "eth")) ||
		strings.HasPrefix(short, "gre") {
		return false
	}
	return true
}

func ifStatusStr(v int64) string {
	switch v {
	case 1:
		return "up"
	case 2, 6, 7: // down(2), notPresent(6), lowerLayerDown(7)
		return "down"
	default:
		return "unknown"
	}
}

func readChassisMAC(ctx context.Context, sess Session, ifs []*biz.Interface) string {
	if pdus, err := sess.Get(ctx, []string{oidLldpLocChassisID, oidDot1dBaseBridgeAddr}); err == nil {
		for _, p := range pdus {
			if b := pduBytes(p); len(b) == 6 {
				if m := biz.FormatMAC(b); m != "" && m != "00:00:00:00:00:00" {
					return m
				}
			}
			if m := biz.NormalizeMAC(pduString(p)); m != "" && m != "00:00:00:00:00:00" {
				return m
			}
		}
	}
	// 兜底: 纯三层路由器/网关(如 RG-NBR)未开启 LLDP/Bridge 时, 取最小索引物理口的 MAC
	for _, it := range ifs {
		if biz.ValidEndpointMAC(it.MAC) {
			return it.MAC
		}
	}
	return ""
}

func readLLDP(ctx context.Context, sess Session, ifs []*biz.Interface) ([]*biz.Neighbor, []string) {
	var warns []string
	// 本地 portNum -> ifIndex 映射(通过 lldpLocPortId / lldpLocPortDesc 匹配接口名, 忽略大小写和空格)
	norm := func(s string) string {
		return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
	}
	ifByName := map[string]int{}
	ifByIdx := map[int]bool{}
	for _, it := range ifs {
		ifByIdx[it.IfIndex] = true
		if it.Name != "" {
			ifByName[norm(it.Name)] = it.IfIndex
		}
		if it.Descr != "" {
			ifByName[norm(it.Descr)] = it.IfIndex
		}
	}
	locPortToIf := map[int]int{}
	mapLoc := func(oid string) {
		pdus, err := sess.Walk(ctx, oid)
		if err != nil {
			return
		}
		for _, p := range pdus {
			pnum, err := strconv.Atoi(suffix(p.OID, oid))
			if err != nil {
				continue
			}
			if _, ok := locPortToIf[pnum]; ok {
				continue
			}
			if idx, ok := ifByName[norm(pduString(p))]; ok {
				locPortToIf[pnum] = idx
			}
		}
	}
	mapLoc(oidLldpLocPortID)
	mapLoc(oidLldpLocPortDesc)

	type remKey struct {
		locPort int
		remIdx  int
	}
	byKey := map[remKey]*biz.Neighbor{}
	get := func(k remKey) *biz.Neighbor {
		n := byKey[k]
		if n == nil {
			ifIdx := locPortToIf[k.locPort]
			if ifIdx == 0 && ifByIdx[k.locPort] {
				ifIdx = k.locPort
			}
			n = &biz.Neighbor{LocalIfIndex: ifIdx}
			byKey[k] = n
		}
		return n
	}

	// lldpRemTable 索引: timeMark.localPortNum.remIndex
	parseRemKey := func(oid, base string) (remKey, bool) {
		parts := strings.Split(suffix(oid, base), ".")
		if len(parts) < 3 {
			return remKey{}, false
		}
		lp, err1 := strconv.Atoi(parts[1])
		ri, err2 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil {
			return remKey{}, false
		}
		return remKey{lp, ri}, true
	}

	walkRem := func(oid string, fn func(n *biz.Neighbor, p PDU)) {
		pdus, err := sess.Walk(ctx, oid)
		if err != nil {
			warns = append(warns, oid+": "+err.Error())
			return
		}
		for _, p := range pdus {
			if k, ok := parseRemKey(p.OID, oid); ok {
				fn(get(k), p)
			}
		}
	}

	walkRem(oidLldpRemChassisSub, func(n *biz.Neighbor, p PDU) { n.RemChassisSub = int(pduInt64(p)) })
	walkRem(oidLldpRemChassisID, func(n *biz.Neighbor, p PDU) {
		if b := pduBytes(p); len(b) == 6 {
			n.RemChassisID = biz.FormatMAC(b)
		} else {
			n.RemChassisID = pduString(p)
		}
	})
	walkRem(oidLldpRemPortSub, func(n *biz.Neighbor, p PDU) { n.RemPortSub = int(pduInt64(p)) })
	walkRem(oidLldpRemPortID, func(n *biz.Neighbor, p PDU) {
		if b := pduBytes(p); len(b) == 6 && n.RemPortSub == 3 {
			n.RemPortID = biz.FormatMAC(b)
		} else {
			n.RemPortID = pduString(p)
		}
	})
	walkRem(oidLldpRemPortDesc, func(n *biz.Neighbor, p PDU) { n.RemPortDesc = pduString(p) })
	walkRem(oidLldpRemSysName, func(n *biz.Neighbor, p PDU) { n.RemSysName = pduString(p) })
	walkRem(oidLldpRemSysDesc, func(n *biz.Neighbor, p PDU) { n.RemSysDescr = pduString(p) })
	walkRem(oidLldpRemSysCapEn, func(n *biz.Neighbor, p PDU) {
		if b := pduBytes(p); len(b) > 0 {
			n.RemCaps = int(b[0])
		}
	})

	// lldpRemManAddrIfSubtype 索引: timeMark.localPort.remIdx.addrSubtype.addrLen.<addr bytes...>
	if pdus, err := sess.Walk(ctx, oidLldpRemManAddrIfSub); err == nil {
		for _, p := range pdus {
			parts := strings.Split(suffix(p.OID, oidLldpRemManAddrIfSub), ".")
			if len(parts) < 9 {
				continue
			}
			lp, err1 := strconv.Atoi(parts[1])
			ri, err2 := strconv.Atoi(parts[2])
			sub, err3 := strconv.Atoi(parts[3])
			alen, err4 := strconv.Atoi(parts[4])
			if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
				continue
			}
			// subtype=1(IPv4), len=4
			if sub == 1 && alen == 4 && len(parts) >= 9 {
				ip := strings.Join(parts[5:9], ".")
				if biz.ValidEndpointIP(ip) {
					if n := byKey[remKey{lp, ri}]; n != nil && n.RemMgmtIP == "" {
						n.RemMgmtIP = ip
					}
				}
			}
		}
	}

	var out []*biz.Neighbor
	for _, n := range byKey {
		if n.LocalIfIndex == 0 {
			continue
		}
		out = append(out, n)
	}
	return out, warns
}

func readFDB(ctx context.Context, sess Session, drv VendorDriver) ([]biz.FdbEntry, []string) {
	var warns []string
	// dot1dBasePort -> ifIndex
	portToIf := map[int]int{}
	if pdus, err := sess.Walk(ctx, oidDot1dBasePortIfIndex); err == nil {
		for _, p := range pdus {
			bp, err := strconv.Atoi(suffix(p.OID, oidDot1dBasePortIfIndex))
			if err == nil {
				portToIf[bp] = int(pduInt64(p))
			}
		}
	}

	// 优先 1: Q-BRIDGE-MIB (带 VLAN): 索引 vlan.m1.m2.m3.m4.m5.m6
	qPorts, err := sess.Walk(ctx, oidDot1qTpFdbPort)
	if err == nil && len(qPorts) > 0 {
		status := map[string]int{}
		if qSt, err := sess.Walk(ctx, oidDot1qTpFdbStatus); err == nil {
			for _, p := range qSt {
				status[suffix(p.OID, oidDot1qTpFdbStatus)] = int(pduInt64(p))
			}
		}
		var out []biz.FdbEntry
		for _, p := range qPorts {
			suf := suffix(p.OID, oidDot1qTpFdbPort)
			if st, ok := status[suf]; ok && st != 3 { // 仅取 learned(3)
				continue
			}
			parts := strings.Split(suf, ".")
			if len(parts) != 7 {
				continue
			}
			vlan, err := strconv.Atoi(parts[0])
			if err != nil {
				continue
			}
			mac := oidPartsToMAC(parts[1:7])
			if !biz.ValidEndpointMAC(mac) {
				continue
			}
			bp := int(pduInt64(p))
			ifIdx := portToIf[bp]
			if ifIdx == 0 {
				ifIdx = bp
			}
			if ifIdx <= 0 {
				continue
			}
			out = append(out, biz.FdbEntry{MAC: mac, VLAN: vlan, IfIndex: ifIdx})
		}
		return out, warns
	}

	// 优先 2: 厂商私有带 VLAN FDB (如华为 HUAWEI-L2MAM-MIB)
	if drv != nil {
		if priv := drv.ReadPrivateFDB(ctx, sess); len(priv) > 0 {
			return priv, warns
		}
	}

	// 兜底 BRIDGE-MIB: 索引 m1.m2.m3.m4.m5.m6 (本身不含 VLAN, 尝试用 dot1qPvid 回填端口 PVID)
	dPorts, err := sess.Walk(ctx, oidDot1dTpFdbPort)
	if err != nil {
		warns = append(warns, "fdb: "+err.Error())
		return nil, warns
	}
	pvidByPort := map[int]int{}
	if pvids, err := sess.Walk(ctx, oidDot1qPvid); err == nil {
		for _, p := range pvids {
			if bp, err := strconv.Atoi(suffix(p.OID, oidDot1qPvid)); err == nil {
				if v := int(pduInt64(p)); v > 0 && v <= 4094 {
					pvidByPort[bp] = v
				}
			}
		}
	}
	status := map[string]int{}
	if dSt, err := sess.Walk(ctx, oidDot1dTpFdbStatus); err == nil {
		for _, p := range dSt {
			status[suffix(p.OID, oidDot1dTpFdbStatus)] = int(pduInt64(p))
		}
	}
	var out []biz.FdbEntry
	for _, p := range dPorts {
		suf := suffix(p.OID, oidDot1dTpFdbPort)
		if st, ok := status[suf]; ok && st != 3 {
			continue
		}
		parts := strings.Split(suf, ".")
		if len(parts) != 6 {
			continue
		}
		mac := oidPartsToMAC(parts)
		if !biz.ValidEndpointMAC(mac) {
			continue
		}
		bp := int(pduInt64(p))
		ifIdx := portToIf[bp]
		if ifIdx == 0 {
			ifIdx = bp
		}
		if ifIdx <= 0 {
			continue
		}
		vlan := pvidByPort[bp]
		if vlan == 0 {
			vlan = 1
		}
		out = append(out, biz.FdbEntry{MAC: mac, VLAN: vlan, IfIndex: ifIdx})
	}
	return out, warns
}

func readARP(ctx context.Context, sess Session) ([]biz.ArpEntry, []string) {
	var warns []string
	var out []biz.ArpEntry
	seen := map[string]bool{}

	// 优先 IP-MIB ipNetToPhysicalPhysAddress: 索引 ifIndex.addrType.addrLen.<ip...>
	if pdus, err := sess.Walk(ctx, oidIPNetToPhysicalPhys); err == nil && len(pdus) > 0 {
		for _, p := range pdus {
			parts := strings.Split(suffix(p.OID, oidIPNetToPhysicalPhys), ".")
			if len(parts) < 7 {
				continue
			}
			ifIdx, _ := strconv.Atoi(parts[0])
			addrType, _ := strconv.Atoi(parts[1])
			addrLen, _ := strconv.Atoi(parts[2])
			if addrType != 1 || addrLen != 4 || len(parts) < 7 {
				continue
			}
			ip := strings.Join(parts[3:7], ".")
			mac := biz.FormatMAC(pduBytes(p))
			if biz.ValidEndpointIP(ip) && biz.ValidEndpointMAC(mac) && !seen[ip+"|"+mac] {
				seen[ip+"|"+mac] = true
				out = append(out, biz.ArpEntry{IP: ip, MAC: mac, IfIndex: ifIdx})
			}
		}
		if len(out) > 0 {
			return out, warns
		}
	}

	// 兜底 RFC1213 ipNetToMediaPhysAddress: 索引 ifIndex.a.b.c.d
	pdus, err := sess.Walk(ctx, oidIPNetToMediaPhys)
	if err != nil {
		warns = append(warns, "arp: "+err.Error())
		return nil, warns
	}
	for _, p := range pdus {
		parts := strings.Split(suffix(p.OID, oidIPNetToMediaPhys), ".")
		if len(parts) != 5 {
			continue
		}
		ifIdx, _ := strconv.Atoi(parts[0])
		ip := strings.Join(parts[1:5], ".")
		mac := biz.FormatMAC(pduBytes(p))
		if net.ParseIP(ip) != nil && biz.ValidEndpointIP(ip) && biz.ValidEndpointMAC(mac) && !seen[ip+"|"+mac] {
			seen[ip+"|"+mac] = true
			out = append(out, biz.ArpEntry{IP: ip, MAC: mac, IfIndex: ifIdx})
		}
	}
	return out, warns
}

func readEntity(ctx context.Context, sess Session, drv VendorDriver) EntityExtra {
	var ent EntityExtra
	chassisIdx := map[string]bool{}
	if pdus, err := sess.Walk(ctx, oidEntPhysicalClass); err == nil {
		for _, p := range pdus {
			if pduInt64(p) == 3 { // chassis(3)
				ent.StackMembers++
				chassisIdx[suffix(p.OID, oidEntPhysicalClass)] = true
			}
		}
	}

	pickEntityStr := func(oid string) string {
		pdus, err := sess.Walk(ctx, oid)
		if err != nil {
			return ""
		}
		var fallback string
		for _, p := range pdus {
			s := pduString(p)
			if s == "" {
				continue
			}
			if chassisIdx[suffix(p.OID, oid)] {
				return s
			}
			if fallback == "" {
				fallback = s
			}
		}
		return fallback
	}

	ent.Serial = pickEntityStr(oidEntPhysicalSerialNum)
	ent.Model = pickEntityStr(oidEntPhysicalModelName)
	ent.OSVersion = pickEntityStr(oidEntPhysicalSoftwareRev)

	if drv != nil {
		extra := drv.ReadEntityExtra(ctx, sess)
		if ent.Serial == "" {
			ent.Serial = extra.Serial
		}
		if ent.Model == "" {
			ent.Model = extra.Model
		}
		if ent.OSVersion == "" {
			ent.OSVersion = extra.OSVersion
		}
		if ent.StackMembers == 0 && extra.StackMembers > 0 {
			ent.StackMembers = extra.StackMembers
		}
	}
	if ent.StackMembers == 0 {
		ent.StackMembers = 1
	}
	return ent
}

func suffix(oid, base string) string {
	return strings.TrimPrefix(strings.TrimPrefix(oid, base), ".")
}

func oidPartsToMAC(parts []string) string {
	if len(parts) != 6 {
		return ""
	}
	b := make([]byte, 6)
	for i, s := range parts {
		v, err := strconv.Atoi(s)
		if err != nil || v < 0 || v > 255 {
			return ""
		}
		b[i] = byte(v)
	}
	return biz.FormatMAC(b)
}
