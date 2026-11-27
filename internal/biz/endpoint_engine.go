package biz

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"hoptrace/internal/pkg/oui"
)

// EngineOptions 终端引擎参数。
type EngineOptions struct {
	MoveConfirmRounds int // 连续 N 轮出现在新位置才确认漂移
	OfflineMissRounds int // 连续 N 轮未见判定下线
}

// EndpointInput 终端同步输入。
type EndpointInput struct {
	Devices    []*Device
	Interfaces []*Interface
	Fdb        []FdbEntry
	Arp        []ArpEntry
	Neighbors  []*Neighbor
	Existing   []*Endpoint
	Now        time.Time
	Opts       EngineOptions
}

type candidate struct {
	MAC      string
	DeviceID uint
	IfIndex  int
	IfName   string
	VLAN     int
	MACCount int
	Suspect  bool
	HintType string
	HintName string
}

func (c candidate) key() IfKey { return IfKey{c.DeviceID, c.IfIndex} }

// InferEndpointType 依据 OUI 厂商与名称关键字推断终端类型。
func InferEndpointType(vendor, name string) string {
	s := strings.ToLower(vendor + " " + name)
	has := func(ks ...string) bool {
		for _, k := range ks {
			if strings.Contains(s, k) {
				return true
			}
		}
		return false
	}
	switch {
	case has("hikvision", "dahua", "uniview", "axis comm", "vivotek", "camera"):
		return TypeCamera
	case has("canon", "brother", "epson", "xerox", "lexmark", "kyocera", "ricoh", "konica", "printer", "zebra tech"):
		return TypePrinter
	case has("polycom", "yealink", "grandstream", "avaya", "fanvil", "snom", "phone"):
		return TypePhone
	case has("ubiquiti", "aruba", "ruckus", "tp-link systems", "cambium"):
		return TypeAP
	case has("apple", "samsung", "xiaomi", "oppo", "vivo", "oneplus", "honor device"):
		return TypeMobile
	case has("espressif", "tuya", "shelly", "sonoff", "raspberry"):
		return TypeIoT
	case has("supermicro", "inspur", "vmware", "xensource", "qemu", "proxmox"):
		return TypeServer
	case has("dell", "hewlett", "hp inc", "lenovo", "lcfc", "compal", "quanta", "wistron", "hon hai", "foxconn", "pegatron", "asustek", "acer", "intel", "realtek", "microsoft", "giga-byte", "micro-star", "huawei device", "huawei technologies", "ibm"):
		return TypePC
	}
	return TypeUnknown
}

func hintToType(c NeighborClass) string {
	switch c {
	case NeighborAP:
		return TypeAP
	case NeighborPhone:
		return TypePhone
	}
	return ""
}

// ComputeEndpointChanges 终端定位与事件引擎:
// 以 FDB(MAC→端口)为主、ARP(MAC→IP)和 LLDP 终端提示为辅,
// 过滤上联口/聚合口/设备自身 MAC,多端口时取最靠近终端侧的端口,
// 并通过防抖产生 新增/上线/下线/漂移/IP 变化 事件。纯函数,不依赖存储。
func ComputeEndpointChanges(in EndpointInput) []*EndpointChange {
	opts := in.Opts
	if opts.MoveConfirmRounds <= 0 {
		opts.MoveConfirmRounds = 2
	}
	if opts.OfflineMissRounds <= 0 {
		opts.OfflineMissRounds = 2
	}
	now := in.Now

	devName := map[uint]string{}
	reachable := map[uint]bool{}
	self := map[string]bool{}
	for _, d := range in.Devices {
		n := d.Name
		if n == "" {
			n = d.MgmtIP
		}
		devName[d.ID] = n
		reachable[d.ID] = d.Managed && d.Status == StatusOnline
		if m := NormalizeMAC(d.ChassisMAC); m != "" {
			self[m] = true
		}
	}
	ifByKey := map[IfKey]*Interface{}
	for _, it := range in.Interfaces {
		ifByKey[IfKey{it.DeviceID, it.IfIndex}] = it
		if m := NormalizeMAC(it.MAC); m != "" {
			self[m] = true
		}
	}

	// 1. 候选位置
	cands := map[string][]candidate{}
	seenLoc := map[string]bool{}
	add := func(c candidate) {
		k := fmt.Sprintf("%s|%d|%d", c.MAC, c.DeviceID, c.IfIndex)
		if seenLoc[k] {
			if c.HintType != "" {
				list := cands[c.MAC]
				for i := range list {
					if list[i].DeviceID == c.DeviceID && list[i].IfIndex == c.IfIndex {
						list[i].HintType, list[i].HintName = c.HintType, c.HintName
					}
				}
			}
			return
		}
		seenLoc[k] = true
		cands[c.MAC] = append(cands[c.MAC], c)
	}
	for _, f := range in.Fdb {
		mac := NormalizeMAC(f.MAC)
		if !ValidEndpointMAC(mac) || self[mac] {
			continue
		}
		it := ifByKey[IfKey{f.DeviceID, f.IfIndex}]
		if it == nil || (it.Kind != IfKindEdge && it.Kind != IfKindSuspect) {
			continue
		}
		add(candidate{MAC: mac, DeviceID: f.DeviceID, IfIndex: f.IfIndex, IfName: it.Name, VLAN: f.VLAN,
			MACCount: it.MACCount, Suspect: it.Kind == IfKindSuspect})
	}
	for _, n := range in.Neighbors {
		cls := ClassifyNeighbor(n)
		if cls == NeighborNetwork || cls == NeighborUnknown {
			continue
		}
		mac := NormalizeMAC(n.RemChassisID)
		if !ValidEndpointMAC(mac) || self[mac] {
			continue
		}
		it := ifByKey[IfKey{n.DeviceID, n.LocalIfIndex}]
		if it == nil || it.Kind == IfKindUplink || it.Kind == IfKindLAG {
			continue
		}
		add(candidate{MAC: mac, DeviceID: n.DeviceID, IfIndex: n.LocalIfIndex, IfName: it.Name,
			MACCount: it.MACCount, Suspect: it.Kind == IfKindSuspect, HintType: hintToType(cls), HintName: n.RemSysName})
	}

	// 2. ARP: MAC → IPs
	arp := map[string][]string{}
	for _, a := range in.Arp {
		mac := NormalizeMAC(a.MAC)
		if mac == "" || self[mac] || !ValidEndpointIP(a.IP) {
			continue
		}
		arp[mac] = append(arp[mac], a.IP)
	}
	for m := range arp {
		arp[m] = uniqueStrings(arp[m])
		sort.Strings(arp[m])
	}

	existing := map[string]*Endpoint{}
	for _, e := range in.Existing {
		existing[e.MAC] = e
	}

	best := func(list []candidate) candidate {
		sort.SliceStable(list, func(i, j int) bool {
			a, b := list[i], list[j]
			if a.Suspect != b.Suspect {
				return !a.Suspect
			}
			if a.MACCount != b.MACCount {
				return a.MACCount < b.MACCount
			}
			if a.DeviceID != b.DeviceID {
				return a.DeviceID < b.DeviceID
			}
			return a.IfIndex < b.IfIndex
		})
		return list[0]
	}
	loc := func(c candidate) string {
		return fmt.Sprintf("%s %s VLAN%d", devName[c.DeviceID], c.IfName, c.VLAN)
	}
	ev := func(e *Endpoint, typ, detail string) *EndpointEvent {
		return &EndpointEvent{MAC: e.MAC, IP: e.IP, Name: e.Name, Type: typ, Detail: detail, At: now}
	}

	macs := make([]string, 0, len(cands))
	for m := range cands {
		macs = append(macs, m)
	}
	sort.Strings(macs)

	var changes []*EndpointChange

	// 3. 在线终端
	for _, mac := range macs {
		list := cands[mac]
		e := existing[mac]
		if e == nil {
			c := best(list)
			vendor := oui.Lookup(mac)
			name := c.HintName
			typ := c.HintType
			if typ == "" {
				typ = InferEndpointType(vendor, name)
			}
			ips := arp[mac]
			ne := &Endpoint{
				MAC: mac, IPs: ips, Vendor: vendor, Type: typ, Name: name, Status: EndpointOnline,
				Trust: TrustUnknown, DeviceID: c.DeviceID, IfIndex: c.IfIndex, IfName: c.IfName, VLAN: c.VLAN,
				FirstSeen: now, LastSeen: now,
			}
			if len(ips) > 0 {
				ne.IP = ips[0]
			}
			ne.DeviceName = devName[c.DeviceID]
			changes = append(changes, &EndpointChange{
				Endpoint: ne, IsNew: true, LocationChanged: true,
				Events: []*EndpointEvent{ev(ne, EventNew, "首次发现于 "+loc(c))},
			})
			continue
		}

		ch := &EndpointChange{Endpoint: e}
		var evs []*EndpointEvent
		prevStatus := e.Status

		// 位置选择:现位置仍在候选中则保持
		var chosen candidate
		stay := false
		for _, c := range list {
			if c.DeviceID == e.DeviceID && c.IfIndex == e.IfIndex {
				chosen, stay = c, true
				break
			}
		}
		if !stay {
			chosen = best(list)
		}

		switch {
		case stay:
			e.PendingDeviceID, e.PendingIfIndex, e.PendingRounds = 0, 0, 0
			if e.VLAN != chosen.VLAN {
				e.VLAN = chosen.VLAN
			}
		case prevStatus == EndpointOffline:
			// 离线后在新位置重新出现:直接确认
			evs = append(evs, ev(e, EventMoved, fmt.Sprintf("离线后出现在新位置: %s -> %s", e.IfName+"@"+devName[e.DeviceID], loc(chosen))))
			e.DeviceID, e.IfIndex, e.IfName, e.VLAN = chosen.DeviceID, chosen.IfIndex, chosen.IfName, chosen.VLAN
			e.PendingDeviceID, e.PendingIfIndex, e.PendingRounds = 0, 0, 0
			ch.LocationChanged = true
		default:
			if e.PendingDeviceID == chosen.DeviceID && e.PendingIfIndex == chosen.IfIndex {
				e.PendingRounds++
			} else {
				e.PendingDeviceID, e.PendingIfIndex, e.PendingRounds = chosen.DeviceID, chosen.IfIndex, 1
			}
			if e.PendingRounds >= opts.MoveConfirmRounds {
				evs = append(evs, ev(e, EventMoved, fmt.Sprintf("%s -> %s", e.IfName+"@"+devName[e.DeviceID], loc(chosen))))
				e.DeviceID, e.IfIndex, e.IfName, e.VLAN = chosen.DeviceID, chosen.IfIndex, chosen.IfName, chosen.VLAN
				e.PendingDeviceID, e.PendingIfIndex, e.PendingRounds = 0, 0, 0
				ch.LocationChanged = true
			}
		}
		e.DeviceName = devName[e.DeviceID]

		// 状态
		e.MissCount = 0
		e.LastSeen = now
		if prevStatus != EndpointOnline {
			e.Status = EndpointOnline
			evs = append(evs, ev(e, EventOnline, "上线于 "+loc(candidateOf(e))))
		}

		// IP
		ips := arp[mac]
		if len(ips) > 0 {
			keep := false
			for _, ip := range ips {
				if ip == e.IP {
					keep = true
				}
			}
			if !keep {
				old := e.IP
				e.IP = ips[0]
				if old != "" {
					evs = append(evs, ev(e, EventIPChanged, old+" -> "+e.IP))
				}
			}
			e.IPs = ips
		}

		// 类型、名称、厂商
		if e.Vendor == "" {
			e.Vendor = oui.Lookup(mac)
		}
		if !e.TypeManual {
			var hint, hname string
			for _, c := range list {
				if c.HintType != "" {
					hint, hname = c.HintType, c.HintName
				}
			}
			switch {
			case hint != "":
				e.Type = hint
			case e.Type == "" || e.Type == TypeUnknown:
				e.Type = InferEndpointType(e.Vendor, e.Name)
			}
			if e.Name == "" && hname != "" {
				e.Name = hname
			}
		}
		ch.Events = evs
		changes = append(changes, ch)
	}

	// 4. 未出现的已有终端
	missing := make([]*Endpoint, 0)
	for _, e := range in.Existing {
		if _, ok := cands[e.MAC]; !ok {
			missing = append(missing, e)
		}
	}
	sort.Slice(missing, func(i, j int) bool { return missing[i].MAC < missing[j].MAC })
	for _, e := range missing {
		if e.Status != EndpointOnline || !reachable[e.DeviceID] {
			continue // 已离线,或所属设备不可达(不能据此判断终端下线)
		}
		e.MissCount++
		ch := &EndpointChange{Endpoint: e}
		if e.MissCount >= opts.OfflineMissRounds {
			e.Status = EndpointOffline
			ch.Events = []*EndpointEvent{ev(e, EventOffline, fmt.Sprintf("最后位置 %s@%s", e.IfName, devName[e.DeviceID]))}
		}
		changes = append(changes, ch)
	}
	return changes
}

func candidateOf(e *Endpoint) candidate {
	return candidate{DeviceID: e.DeviceID, IfIndex: e.IfIndex, IfName: e.IfName, VLAN: e.VLAN}
}
