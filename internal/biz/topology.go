package biz

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// lldpRemSysCapEnabled 首字节位定义(IEEE 802.1AB)
const (
	capRepeater = 0x40
	capBridge   = 0x20
	capWLAN     = 0x10
	capRouter   = 0x08
	capPhone    = 0x04
	capStation  = 0x01
)

// NeighborClass LLDP 邻居的设备类别。
type NeighborClass int

const (
	NeighborNetwork NeighborClass = iota // 交换机/路由器/防火墙
	NeighborAP
	NeighborPhone
	NeighborStation
	NeighborUnknown
)

// ClassifyNeighbor 依据 LLDP 系统能力位判定邻居类别。
func ClassifyNeighbor(n *Neighbor) NeighborClass {
	c := n.RemCaps
	switch {
	case c&capWLAN != 0:
		return NeighborAP
	case c&capPhone != 0:
		return NeighborPhone
	case c&(capBridge|capRouter|capRepeater) != 0:
		return NeighborNetwork
	case c&capStation != 0:
		return NeighborStation
	case c == 0 && n.RemMgmtIP != "":
		return NeighborNetwork
	}
	return NeighborUnknown
}

// TopoInput 拓扑融合输入。
type TopoInput struct {
	Devices    []*Device
	Interfaces []*Interface
	Neighbors  []*Neighbor
	Fdb        []FdbEntry    // 用于 LLDP 缺失时的 FDB 链路推断
	MACCounts  map[IfKey]int // 各端口 FDB 条目数
	ArpCounts  map[uint]int  // 各设备 ARP 条目数
	Threshold  int           // 疑似私接 MAC 数阈值
	Now        time.Time
}

// IfInfo 拓扑计算后回写到接口的信息。
type IfInfo struct {
	Kind     string
	MACCount int
	Neighbor string
}

// TopoResult 拓扑融合输出。
type TopoResult struct {
	Links         []*Link
	Stubs         []*Device // 未能纳管的网络邻居占位设备
	ObsoleteStubs []uint    // 已被真实设备替代的占位设备
	IfInfos       map[IfKey]IfInfo
	Roles         map[uint]string
}

type linkAcc struct {
	link       *Link
	members    map[string]struct{}
	reportedA  bool
	reportedB  bool
	remoteIdOK bool
}

// BuildTopology 将 LLDP 邻居融合为设备间逻辑链路,并判定接口类别与设备角色。纯函数,不依赖存储。
func BuildTopology(in TopoInput) *TopoResult {
	res := &TopoResult{IfInfos: map[IfKey]IfInfo{}, Roles: map[uint]string{}}
	thr := in.Threshold
	if thr <= 0 {
		thr = 30
	}

	devByID := map[uint]*Device{}
	macIdx := map[string]*Device{}
	ipIdx := map[string]*Device{}
	nameIdx := map[string]*Device{}
	ifByDev := map[uint][]*Interface{}
	ifByKey := map[IfKey]*Interface{}

	// 优先级:已纳管设备 > 占位设备
	devs := append([]*Device(nil), in.Devices...)
	sort.SliceStable(devs, func(i, j int) bool {
		if devs[i].Managed != devs[j].Managed {
			return devs[i].Managed
		}
		return devs[i].ID < devs[j].ID
	})
	for _, d := range devs {
		devByID[d.ID] = d
	}
	for _, it := range in.Interfaces {
		ifByDev[it.DeviceID] = append(ifByDev[it.DeviceID], it)
		ifByKey[IfKey{it.DeviceID, it.IfIndex}] = it
	}
	setIdx := func(m map[string]*Device, k string, d *Device) {
		if k == "" {
			return
		}
		if _, ok := m[k]; !ok {
			m[k] = d
		}
	}
	for _, d := range devs {
		setIdx(macIdx, NormalizeMAC(d.ChassisMAC), d)
		for _, it := range ifByDev[d.ID] {
			setIdx(macIdx, NormalizeMAC(it.MAC), d)
		}
		setIdx(ipIdx, d.MgmtIP, d)
		for _, ip := range d.IPs {
			setIdx(ipIdx, ip, d)
		}
		setIdx(nameIdx, strings.ToLower(d.SysName), d)
		setIdx(nameIdx, strings.ToLower(d.Name), d)
	}

	// 占位设备若与已纳管设备身份重复,则标记为过期
	for _, d := range devs {
		if d.Managed {
			continue
		}
		for _, other := range []*Device{macIdx[NormalizeMAC(d.ChassisMAC)], ipIdx[d.MgmtIP]} {
			if other != nil && other.ID != d.ID && other.Managed {
				res.ObsoleteStubs = append(res.ObsoleteStubs, d.ID)
				delete(devByID, d.ID)
				break
			}
		}
	}
	obsolete := map[uint]bool{}
	for _, id := range res.ObsoleteStubs {
		obsolete[id] = true
	}
	live := func(d *Device) *Device {
		if d == nil || obsolete[d.ID] {
			return nil
		}
		return d
	}

	resolve := func(n *Neighbor) *Device {
		if d := live(macIdx[NormalizeMAC(n.RemChassisID)]); d != nil {
			return d
		}
		if d := live(ipIdx[n.RemMgmtIP]); d != nil && n.RemMgmtIP != "" {
			return d
		}
		if n.RemSysName != "" {
			if d := live(nameIdx[strings.ToLower(n.RemSysName)]); d != nil {
				return d
			}
		}
		return nil
	}

	matchRemoteIf := func(rem *Device, n *Neighbor) *Interface {
		ports := ifByDev[rem.ID]
		if n.RemPortSub == 3 {
			mac := NormalizeMAC(n.RemPortID)
			var hit *Interface
			cnt := 0
			for _, p := range ports {
				if mac != "" && NormalizeMAC(p.MAC) == mac {
					hit = p
					cnt++
				}
			}
			if cnt == 1 {
				return hit
			}
		}
		if n.RemPortSub == 7 {
			if idx, err := strconv.Atoi(n.RemPortID); err == nil {
				if p := ifByKey[IfKey{rem.ID, idx}]; p != nil {
					return p
				}
			}
		}
		want := []string{canonIfName(n.RemPortID), canonIfName(n.RemPortDesc)}
		for _, w := range want {
			if w == "" {
				continue
			}
			for _, p := range ports {
				if canonIfName(p.Name) == w || canonIfName(p.Descr) == w || (p.Alias != "" && canonIfName(p.Alias) == w) {
					return p
				}
			}
		}
		return nil
	}

	logicalOf := func(devID uint, ifIndex int) (int, *Interface) {
		it := ifByKey[IfKey{devID, ifIndex}]
		if it != nil && it.AggIfIndex > 0 {
			if agg := ifByKey[IfKey{devID, it.AggIfIndex}]; agg != nil {
				return agg.IfIndex, agg
			}
			return it.AggIfIndex, nil
		}
		return ifIndex, it
	}

	accs := map[string]*linkAcc{}
	stubs := map[string]*Device{}
	neighborDesc := map[IfKey]string{}
	uplinkIf := map[IfKey]bool{}

	for _, n := range in.Neighbors {
		local := devByID[n.DeviceID]
		if local == nil {
			continue
		}
		desc := n.RemSysName
		if desc == "" {
			desc = n.RemChassisID
		}
		if n.RemPortID != "" {
			desc += " / " + n.RemPortID
		}
		neighborDesc[IfKey{n.DeviceID, n.LocalIfIndex}] = desc

		if ClassifyNeighbor(n) != NeighborNetwork {
			continue // AP/电话/终端:交给终端引擎
		}
		rem := resolve(n)
		if rem == nil {
			chassis := NormalizeMAC(n.RemChassisID)
			key := chassis
			if key == "" {
				key = n.RemMgmtIP
			}
			if key == "" {
				key = n.RemSysName
			}
			if key == "" {
				continue
			}
			if _, ok := stubs[key]; !ok {
				name := n.RemSysName
				if name == "" {
					name = key
				}
				fam := FamilySwitch
				if n.RemCaps&capRouter != 0 && n.RemCaps&capBridge == 0 {
					fam = FamilyRouter
				}
				stubs[key] = &Device{
					Name: name, SysName: n.RemSysName, SysDescr: n.RemSysDescr, MgmtIP: n.RemMgmtIP,
					ChassisMAC: chassis, Managed: false, Status: StatusUnknown, Family: fam, Role: RoleUnknown,
					Vendor: "Unknown", Site: local.Site,
				}
			}
			continue
		}
		if rem.ID == n.DeviceID {
			continue
		}

		lLogical, lIf := logicalOf(n.DeviceID, n.LocalIfIndex)
		rIf := matchRemoteIf(rem, n)
		rLogical, rLogicalIf := 0, (*Interface)(nil)
		lName, rName := n.RemPortID, n.RemPortID
		if lIf != nil {
			lName = lIf.Name
		} else {
			lName = strconv.Itoa(lLogical)
		}
		remoteResolved := rIf != nil
		if rIf != nil {
			rLogical, rLogicalIf = logicalOf(rem.ID, rIf.IfIndex)
			rName = rIf.Name
			if rLogicalIf != nil {
				rName = rLogicalIf.Name
			}
		} else {
			rName = n.RemPortID
			if rName == "" {
				rName = n.RemPortDesc
			}
		}
		localMember, remoteMember := lName, rName
		if lIf != nil && n.LocalIfIndex != lLogical { // 聚合成员:成员名用物理口
			if phys := ifByKey[IfKey{n.DeviceID, n.LocalIfIndex}]; phys != nil {
				localMember = phys.Name
			}
		}
		if rIf != nil {
			remoteMember = rIf.Name
		} else {
			remoteMember = rName
		}

		uplinkIf[IfKey{n.DeviceID, lLogical}] = true
		if rIf != nil {
			uplinkIf[IfKey{rem.ID, rLogical}] = true
		}

		idStr := func(idx int, name string, resolved bool) string {
			if resolved {
				return strconv.Itoa(idx)
			}
			return "n_" + name
		}
		la := struct {
			dev  uint
			idx  int
			name string
			id   string
			mem  string
		}{n.DeviceID, lLogical, lName, idStr(lLogical, lName, true), localMember}
		ra := struct {
			dev  uint
			idx  int
			name string
			id   string
			mem  string
		}{rem.ID, rLogical, rName, idStr(rLogical, rName, remoteResolved), remoteMember}

		localIsA := la.dev < ra.dev || (la.dev == ra.dev && la.idx <= ra.idx)
		a, b := la, ra
		if !localIsA {
			a, b = ra, la
		}
		key := fmt.Sprintf("%d:%s|%d:%s", a.dev, a.id, b.dev, b.id)
		acc := accs[key]
		if acc == nil {
			acc = &linkAcc{members: map[string]struct{}{}, link: &Link{
				Key: key, ADeviceID: a.dev, AIfIndex: a.idx, AIfName: a.name,
				BDeviceID: b.dev, BIfIndex: b.idx, BIfName: b.name, Kind: "lldp",
			}}
			accs[key] = acc
		}
		if remoteResolved {
			acc.remoteIdOK = true
		}
		if localIsA {
			acc.reportedA = true
		} else {
			acc.reportedB = true
		}
		acc.members[a.mem+" <-> "+b.mem] = struct{}{}
	}

	// 生成链路
	keys := make([]string, 0, len(accs))
	for k := range accs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		acc := accs[k]
		l := acc.link
		switch {
		case acc.reportedA && acc.reportedB:
			l.Confidence = 1.0
		case acc.remoteIdOK:
			l.Confidence = 0.8
		default:
			l.Confidence = 0.6
		}
		for m := range acc.members {
			l.Members = append(l.Members, m)
		}
		sort.Strings(l.Members)
		l.Status = LinkUp
		ai, bi := ifByKey[IfKey{l.ADeviceID, l.AIfIndex}], ifByKey[IfKey{l.BDeviceID, l.BIfIndex}]
		if (ai != nil && ai.OperStatus == "down") || (bi != nil && bi.OperStatus == "down") {
			l.Status = LinkDown
		}
		l.FirstSeen, l.LastSeen = in.Now, in.Now
		res.Links = append(res.Links, l)
	}
	for _, s := range stubs {
		res.Stubs = append(res.Stubs, s)
	}
	sort.Slice(res.Stubs, func(i, j int) bool { return res.Stubs[i].Name < res.Stubs[j].Name })

	// FDB 链路推断(AFT): 当两台已纳管设备之间无 LLDP 链路时, 若 A:pA 学到 B 的 MAC 且 B:pB 学到 A 的 MAC,
	// 且两侧端口看到的设备集合交集为空(中间无其他交换机), 则推断 A:pA <-> B:pB 直连(confidence=0.5)。
	if len(in.Fdb) > 0 {
		lldpPairs := map[[2]uint]bool{}
		for _, l := range res.Links {
			a, b := l.ADeviceID, l.BDeviceID
			if a > b {
				a, b = b, a
			}
			lldpPairs[[2]uint{a, b}] = true
		}
		// portDevs[IfKey] = 该逻辑口方向学到的其他已纳管设备集合
		portDevs := map[IfKey]map[uint]bool{}
		for _, f := range in.Fdb {
			src := live(devByID[f.DeviceID])
			if src == nil || !src.Managed {
				continue
			}
			dst := live(macIdx[NormalizeMAC(f.MAC)])
			if dst == nil || !dst.Managed || dst.ID == src.ID {
				continue
			}
			lIdx, _ := logicalOf(src.ID, f.IfIndex)
			pk := IfKey{src.ID, lIdx}
			if uplinkIf[pk] {
				continue // 已有 LLDP 互联的端口不参与推断
			}
			if portDevs[pk] == nil {
				portDevs[pk] = map[uint]bool{}
			}
			portDevs[pk][dst.ID] = true
		}

		// 设备对 -> 候选端口对
		fdbKeys := make([]IfKey, 0, len(portDevs))
		for k := range portDevs {
			fdbKeys = append(fdbKeys, k)
		}
		sort.Slice(fdbKeys, func(i, j int) bool {
			if fdbKeys[i].DeviceID != fdbKeys[j].DeviceID {
				return fdbKeys[i].DeviceID < fdbKeys[j].DeviceID
			}
			return fdbKeys[i].IfIndex < fdbKeys[j].IfIndex
		})

		inferredPairs := map[[2]uint]bool{}
		for _, pkA := range fdbKeys {
			devsA := portDevs[pkA]
			for bID := range devsA {
				if pkA.DeviceID >= bID {
					continue
				}
				pair := [2]uint{pkA.DeviceID, bID}
				if lldpPairs[pair] || inferredPairs[pair] {
					continue
				}
				// 在设备 B 上寻找学到了 A 的端口
				for _, pkB := range fdbKeys {
					if pkB.DeviceID != bID {
						continue
					}
					devsB := portDevs[pkB]
					if !devsB[pkA.DeviceID] {
						continue
					}
					// 检查两侧看到的设备集合交集是否为空
					disjoint := true
					for x := range devsA {
						if x != bID && devsB[x] {
							disjoint = false
							break
						}
					}
					if !disjoint {
						continue
					}
					inferredPairs[pair] = true
					uplinkIf[pkA] = true
					uplinkIf[pkB] = true

					ai, bi := ifByKey[pkA], ifByKey[pkB]
					aName, bName := strconv.Itoa(pkA.IfIndex), strconv.Itoa(pkB.IfIndex)
					if ai != nil && ai.Name != "" {
						aName = ai.Name
					}
					if bi != nil && bi.Name != "" {
						bName = bi.Name
					}
					st := LinkUp
					if (ai != nil && ai.OperStatus == "down") || (bi != nil && bi.OperStatus == "down") {
						st = LinkDown
					}
					res.Links = append(res.Links, &Link{
						Key:        fmt.Sprintf("%d:%d|%d:%d", pkA.DeviceID, pkA.IfIndex, pkB.DeviceID, pkB.IfIndex),
						ADeviceID:  pkA.DeviceID,
						AIfIndex:   pkA.IfIndex,
						AIfName:    aName,
						BDeviceID:  pkB.DeviceID,
						BIfIndex:   pkB.IfIndex,
						BIfName:    bName,
						Kind:       "fdb",
						Confidence: 0.5,
						Status:     st,
						Members:    []string{aName + " <-> " + bName},
						FirstSeen:  in.Now,
						LastSeen:   in.Now,
					})
					break
				}
			}
		}
	}

	// 接口类别
	edgeMACs := map[uint]int{}
	for _, it := range in.Interfaces {
		key := IfKey{it.DeviceID, it.IfIndex}
		cnt := in.MACCounts[key]
		kind := IfKindEdge
		switch {
		case uplinkIf[key]:
			kind = IfKindUplink
		case it.AggIfIndex > 0:
			kind = IfKindLAGMember
		case it.IfType == 161:
			kind = IfKindLAG
		case cnt > thr:
			kind = IfKindSuspect
		}
		res.IfInfos[key] = IfInfo{Kind: kind, MACCount: cnt, Neighbor: neighborDesc[key]}
		if kind == IfKindEdge || kind == IfKindSuspect {
			edgeMACs[it.DeviceID] += cnt
		}
	}

	// 设备角色
	fwAdj := map[uint]bool{}
	for _, l := range res.Links {
		a, b := devByID[l.ADeviceID], devByID[l.BDeviceID]
		if a != nil && b != nil {
			if a.Family == FamilyFirewall {
				fwAdj[b.ID] = true
			}
			if b.Family == FamilyFirewall {
				fwAdj[a.ID] = true
			}
		}
	}
	for _, d := range devs {
		if obsolete[d.ID] || d.RoleManual {
			continue
		}
		role := RoleByFamily(d.Family)
		if d.Family == FamilySwitch && d.Managed {
			switch {
			case fwAdj[d.ID] || in.ArpCounts[d.ID] >= 10:
				role = RoleCore
			case edgeMACs[d.ID] > 0:
				role = RoleAccess
			default:
				role = RoleAggregation
			}
		} else if !d.Managed {
			role = RoleUnknown
		}
		res.Roles[d.ID] = role
	}
	return res
}
