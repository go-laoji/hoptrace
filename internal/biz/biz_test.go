package biz

import (
	"testing"
	"time"
)

func TestIdentifyHuawei(t *testing.T) {
	cases := []struct {
		oid, descr     string
		vendor, family string
		model, ver     string
	}{
		{
			"1.3.6.1.4.1.2011.2.23.380",
			"Huawei Versatile Routing Platform Software VRP (R) software, Version 5.170 (S5720-28P-LI-AC V200R011C10SPC600)",
			"Huawei", FamilySwitch, "S5720-28P-LI-AC", "V200R011C10SPC600",
		},
		{
			"1.3.6.1.4.1.2011.2.82.1",
			"Huawei Versatile Routing Platform Software VRP (R) software, Version 5.170 (USG6510E V600R007C20SPC300)",
			"Huawei", FamilyFirewall, "USG6510E", "V600R007C20SPC300",
		},
		{
			"1.3.6.1.4.1.2011.2.239.5",
			"Huawei VRP (CE6850-48S6Q-EI V200R005C10SPC800)",
			"Huawei", FamilySwitch, "CE6850-48S6Q-EI", "V200R005C10SPC800",
		},
	}
	for _, tc := range cases {
		fp := Identify(tc.oid, tc.descr)
		if fp.Vendor != tc.vendor || fp.Family != tc.family || fp.Model != tc.model || fp.OSVersion != tc.ver {
			t.Errorf("Identify(%s) = %+v, want %s/%s/%s/%s", tc.oid, fp, tc.vendor, tc.family, tc.model, tc.ver)
		}
	}
}

func TestIdentifyMultiVendor(t *testing.T) {
	cases := []struct {
		oid, descr     string
		vendor, family string
		model, ver     string
	}{
		{
			"1.3.6.1.4.1.4881.1.6.1.1.16",
			"Linux Ruijie 4.14.76-g4a2d7de97 #1 SMP PREEMPT aarch64",
			"Ruijie", FamilyRouter, "RG-NBR-N7204-E", "",
		},
		{
			"1.3.6.1.4.1.4881.1.1.10.1.99",
			"Ruijie RG-NBR-N7204-E(v1.11)",
			"Ruijie", FamilyRouter, "RG-NBR-N7204-E", "v1.11",
		},
		{
			"1.3.6.1.4.1.8072.3.2.10", // Net-SNMP OID fallback via sysDescr
			"RG-NBR-N7204-E(v1.11)",
			"Ruijie", FamilyRouter, "RG-NBR-N7204-E", "v1.11",
		},
		{
			"1.3.6.1.4.1.4881.1.1.10.1.123",
			"Ruijie Gigabit Ethernet Switch(RG-S5750-24GT/8SFP-P) By Ruijie Networks, RGOS 11.4(1)B2",
			"Ruijie", FamilySwitch, "RG-S5750-24GT/8SFP-P", "RGOS 11.4(1)B2",
		},
		{
			"1.3.6.1.4.1.25506.1.513",
			"H3C Comware Platform Software, Software Version 7.1.070, Release 6326P01\nH3C S5130S-28P-EI",
			"H3C", FamilySwitch, "S5130S-28P-EI", "Version 7.1.070, Release 6326P01",
		},
		{
			"1.3.6.1.4.1.9.1.1745",
			"Cisco IOS Software, C2960X Software (WS-C2960X-48FPS-L), Version 15.2(4)E7, RELEASE SOFTWARE",
			"Cisco", FamilySwitch, "WS-C2960X-48FPS-L", "15.2(4)E7",
		},
		{
			"1.3.6.1.4.1.2636.1.1.1.2.63",
			"Juniper Networks, Inc. ex4300-48p Ethernet Switch, kernel JUNOS 18.4R3, Build date: 2020-01-01",
			"Juniper", FamilySwitch, "EX4300-48P", "JUNOS 18.4R3",
		},
		{
			"1.3.6.1.4.1.12356.101.1.1000",
			"FortiGate-100F v7.2.4,build1396",
			"Fortinet", FamilyFirewall, "FortiGate-100F", "v7.2.4,build1396",
		},
		{
			"1.3.6.1.4.1.14988.1",
			"RouterOS 7.11.2 (CCR1036-12G-4S)",
			"MikroTik", FamilyRouter, "CCR1036-12G-4S", "RouterOS 7.11.2",
		},
		{
			"1.3.6.1.4.1.30065.1.3011.7050",
			"Arista Networks EOS version 4.28.3M running on an Arista Networks DCS-7050SX3-48YC8",
			"Arista", FamilySwitch, "DCS-7050SX3-48YC8", "EOS 4.28.3M",
		},
		{
			"1.3.6.1.4.1.3902.3.100",
			"ZXR10 5950-52TM ZROS V3.00.11",
			"ZTE", FamilySwitch, "ZXR10 5950-52TM", "V3.00.11",
		},
	}
	for _, tc := range cases {
		fp := Identify(tc.oid, tc.descr)
		if fp.Vendor != tc.vendor || fp.Family != tc.family || fp.Model != tc.model || fp.OSVersion != tc.ver {
			t.Errorf("Identify(%s, %q) = %+v, want %s/%s/%s/%s", tc.oid, tc.descr, fp, tc.vendor, tc.family, tc.model, tc.ver)
		}
	}
}

func TestCanonIfNameMultiVendor(t *testing.T) {
	pairs := [][2]string{
		{"Gi0/1", "GigabitEthernet 0/1"},
		{"GE0/0/1", "GigabitEthernet0/0/1"},
		{"Te1/0/1", "TenGigabitEthernet 1/0/1"},
		{"Ag1", "AggregatePort 1"},
		{"Po10", "Port-channel10"},
		{"BAGG2", "Bridge-Aggregation2"},
	}
	for _, p := range pairs {
		if gotA, gotB := canonIfName(p[0]), canonIfName(p[1]); gotA != gotB {
			t.Errorf("canonIfName(%q)=%q != canonIfName(%q)=%q", p[0], gotA, p[1], gotB)
		}
	}
}

func TestBuildTopologyLAGAndConfidence(t *testing.T) {
	now := time.Now()
	devs := []*Device{
		{ID: 1, Name: "Core", Managed: true, Family: FamilySwitch, ChassisMAC: "00:00:00:00:00:01"},
		{ID: 2, Name: "Agg", Managed: true, Family: FamilySwitch, ChassisMAC: "00:00:00:00:00:02"},
	}
	ifs := []*Interface{
		{DeviceID: 1, IfIndex: 1, Name: "XGE0/0/1", IfType: 6, OperStatus: "up", AggIfIndex: 100},
		{DeviceID: 1, IfIndex: 2, Name: "XGE0/0/2", IfType: 6, OperStatus: "up", AggIfIndex: 100},
		{DeviceID: 1, IfIndex: 100, Name: "Eth-Trunk1", IfType: 161, OperStatus: "up"},
		{DeviceID: 2, IfIndex: 1, Name: "XGE0/0/1", IfType: 6, OperStatus: "up", AggIfIndex: 100},
		{DeviceID: 2, IfIndex: 2, Name: "XGE0/0/2", IfType: 6, OperStatus: "up", AggIfIndex: 100},
		{DeviceID: 2, IfIndex: 100, Name: "Eth-Trunk1", IfType: 161, OperStatus: "up"},
		{DeviceID: 2, IfIndex: 10, Name: "GE0/0/10", IfType: 6, OperStatus: "up"},
	}
	nbs := []*Neighbor{
		{DeviceID: 1, LocalIfIndex: 1, RemChassisID: "00:00:00:00:00:02", RemPortID: "XGE0/0/1", RemCaps: capBridge},
		{DeviceID: 1, LocalIfIndex: 2, RemChassisID: "00:00:00:00:00:02", RemPortID: "XGE0/0/2", RemCaps: capBridge},
		{DeviceID: 2, LocalIfIndex: 1, RemChassisID: "00:00:00:00:00:01", RemPortID: "XGE0/0/1", RemCaps: capBridge},
		{DeviceID: 2, LocalIfIndex: 2, RemChassisID: "00:00:00:00:00:01", RemPortID: "XGE0/0/2", RemCaps: capBridge},
	}

	res := BuildTopology(TopoInput{
		Devices:    devs,
		Interfaces: ifs,
		Neighbors:  nbs,
		MACCounts:  map[IfKey]int{{DeviceID: 2, IfIndex: 10}: 1},
		ArpCounts:  map[uint]int{1: 20},
		Now:        now,
	})
	if len(res.Links) != 1 {
		t.Fatalf("expected 1 merged LAG link, got %d", len(res.Links))
	}
	l := res.Links[0]
	if l.Confidence != 1.0 || len(l.Members) != 2 || l.AIfName != "Eth-Trunk1" || l.BIfName != "Eth-Trunk1" {
		t.Fatalf("unexpected link: %+v", l)
	}
	if res.IfInfos[IfKey{2, 10}].Kind != IfKindEdge {
		t.Fatalf("expected GE0/0/10 to be edge, got %s", res.IfInfos[IfKey{2, 10}].Kind)
	}
	if res.Roles[1] != RoleCore || res.Roles[2] != RoleAccess {
		t.Fatalf("unexpected roles: %+v", res.Roles)
	}
}

func TestComputeEndpointChangesMoveAndOffline(t *testing.T) {
	t0 := time.Now()
	devs := []*Device{
		{ID: 1, Name: "SW-Acc", Managed: true, Status: StatusOnline, ChassisMAC: "48:0f:cf:00:00:01"},
	}
	ifs := []*Interface{
		{DeviceID: 1, IfIndex: 1, Name: "GE0/0/1", Kind: IfKindEdge, MACCount: 1},
		{DeviceID: 1, IfIndex: 2, Name: "GE0/0/2", Kind: IfKindEdge, MACCount: 1},
		{DeviceID: 1, IfIndex: 24, Name: "GE0/0/24", Kind: IfKindUplink, MACCount: 50},
	}
	opts := EngineOptions{MoveConfirmRounds: 2, OfflineMissRounds: 2}

	// 第 1 轮: 发现新终端(同时出现在上联口和边缘口,应只选边缘口)
	ch1 := ComputeEndpointChanges(EndpointInput{
		Devices: devs, Interfaces: ifs,
		Fdb: []FdbEntry{
			{DeviceID: 1, MAC: "f8:bc:12:aa:bb:cc", VLAN: 10, IfIndex: 24},
			{DeviceID: 1, MAC: "f8:bc:12:aa:bb:cc", VLAN: 10, IfIndex: 1},
		},
		Arp:  []ArpEntry{{DeviceID: 1, IP: "10.0.10.5", MAC: "f8:bc:12:aa:bb:cc"}},
		Now:  t0,
		Opts: opts,
	})
	if len(ch1) != 1 || !ch1[0].IsNew || ch1[0].Endpoint.IfIndex != 1 || ch1[0].Endpoint.IP != "10.0.10.5" {
		t.Fatalf("round1 unexpected: %+v", ch1)
	}
	ep := ch1[0].Endpoint
	ep.ID = 1

	// 第 2 轮: 漂移到 GE0/0/2 第 1 次(防抖未确认)
	ch2 := ComputeEndpointChanges(EndpointInput{
		Devices: devs, Interfaces: ifs, Existing: []*Endpoint{ep},
		Fdb:  []FdbEntry{{DeviceID: 1, MAC: ep.MAC, VLAN: 10, IfIndex: 2}},
		Now:  t0.Add(time.Minute),
		Opts: opts,
	})
	if ch2[0].LocationChanged || ep.IfIndex != 1 {
		t.Fatalf("round2 should not confirm move yet: %+v", ep)
	}

	// 第 3 轮: 漂移到 GE0/0/2 第 2 次(确认漂移并产生 moved 事件)
	ch3 := ComputeEndpointChanges(EndpointInput{
		Devices: devs, Interfaces: ifs, Existing: []*Endpoint{ep},
		Fdb:  []FdbEntry{{DeviceID: 1, MAC: ep.MAC, VLAN: 10, IfIndex: 2}},
		Now:  t0.Add(2 * time.Minute),
		Opts: opts,
	})
	if !ch3[0].LocationChanged || ep.IfIndex != 2 || len(ch3[0].Events) != 1 || ch3[0].Events[0].Type != EventMoved {
		t.Fatalf("round3 should confirm move: %+v events=%+v", ep, ch3[0].Events)
	}

	// 第 4、5 轮: 连续 2 轮未见 -> 离线
	ComputeEndpointChanges(EndpointInput{Devices: devs, Interfaces: ifs, Existing: []*Endpoint{ep}, Now: t0.Add(3 * time.Minute), Opts: opts})
	if ep.Status != EndpointOnline {
		t.Fatalf("round4 should still be online")
	}
	ch5 := ComputeEndpointChanges(EndpointInput{Devices: devs, Interfaces: ifs, Existing: []*Endpoint{ep}, Now: t0.Add(4 * time.Minute), Opts: opts})
	if ep.Status != EndpointOffline || len(ch5[0].Events) != 1 || ch5[0].Events[0].Type != EventOffline {
		t.Fatalf("round5 should be offline: %+v", ep)
	}
}

func TestBuildTopologyFDBInference(t *testing.T) {
	// 3 台串联交换机 A(1) -- B(2) -- C(3), 全部未开启 LLDP, 仅通过 FDB 推断直连
	devs := []*Device{
		{ID: 1, Name: "SW-A", Managed: true, Family: FamilySwitch, ChassisMAC: "00:00:00:00:00:0a"},
		{ID: 2, Name: "SW-B", Managed: true, Family: FamilySwitch, ChassisMAC: "00:00:00:00:00:0b"},
		{ID: 3, Name: "SW-C", Managed: true, Family: FamilySwitch, ChassisMAC: "00:00:00:00:00:0c"},
	}
	ifs := []*Interface{
		{DeviceID: 1, IfIndex: 1, Name: "GE0/0/1", IfType: 6, OperStatus: "up"},
		{DeviceID: 2, IfIndex: 1, Name: "GE0/0/1", IfType: 6, OperStatus: "up"},
		{DeviceID: 2, IfIndex: 2, Name: "GE0/0/2", IfType: 6, OperStatus: "up"},
		{DeviceID: 3, IfIndex: 1, Name: "GE0/0/1", IfType: 6, OperStatus: "up"},
	}
	fdb := []FdbEntry{
		// A:1 看到 B 和 C
		{DeviceID: 1, IfIndex: 1, MAC: "00:00:00:00:00:0b", VLAN: 1},
		{DeviceID: 1, IfIndex: 1, MAC: "00:00:00:00:00:0c", VLAN: 1},
		// B:1 看到 A; B:2 看到 C
		{DeviceID: 2, IfIndex: 1, MAC: "00:00:00:00:00:0a", VLAN: 1},
		{DeviceID: 2, IfIndex: 2, MAC: "00:00:00:00:00:0c", VLAN: 1},
		// C:1 看到 B 和 A
		{DeviceID: 3, IfIndex: 1, MAC: "00:00:00:00:00:0b", VLAN: 1},
		{DeviceID: 3, IfIndex: 1, MAC: "00:00:00:00:00:0a", VLAN: 1},
	}

	res := BuildTopology(TopoInput{Devices: devs, Interfaces: ifs, Fdb: fdb, Now: time.Now()})
	if len(res.Links) != 2 {
		t.Fatalf("expected exactly 2 inferred FDB links (A-B and B-C), got %d: %+v", len(res.Links), res.Links)
	}
	for _, l := range res.Links {
		if l.Kind != "fdb" || l.Confidence != 0.5 {
			t.Errorf("unexpected link meta: %+v", l)
		}
		if (l.ADeviceID == 1 && l.BDeviceID == 3) || (l.ADeviceID == 3 && l.BDeviceID == 1) {
			t.Errorf("false direct link inferred between A and C: %+v", l)
		}
	}
	if res.IfInfos[IfKey{1, 1}].Kind != IfKindUplink || res.IfInfos[IfKey{2, 2}].Kind != IfKindUplink {
		t.Errorf("inferred FDB ports must be marked uplink: %+v", res.IfInfos)
	}
}

func TestClassifyPCLLDPAsStation(t *testing.T) {
	// Windows/PC 网卡(Gigabyte/ASUS/Realtek/Lenovo)发送的无 sysName/sysDescr LLDP 报文(即使带 capBridge)
	// 不应被识别为网络设备占位节点
	pcMACs := []string{
		"40:8d:5c:3e:56:e4", // Gigabyte
		"10:7b:44:7e:24:c6", // ASUSTek
		"00:e0:4c:c0:f3:d5", // Realtek
		"68:f7:28:c4:fc:bd", // LCFC / Lenovo
	}
	for _, mac := range pcMACs {
		cls := ClassifyNeighbor(&Neighbor{RemChassisID: mac, RemCaps: capBridge})
		if cls != NeighborStation {
			t.Errorf("PC MAC %s without sysName/sysDescr should be NeighborStation, got %d", mac, cls)
		}
	}
	// 真正的交换机(带 sysName)仍应识别为 NeighborNetwork
	if cls := ClassifyNeighbor(&Neighbor{RemChassisID: "68:8f:84:01:af:db", RemSysName: "140.151", RemCaps: capBridge}); cls != NeighborNetwork {
		t.Errorf("Huawei switch should be NeighborNetwork, got %d", cls)
	}
}

func TestBuildTopologyL3ARPAndFDBInference(t *testing.T) {
	t.Run("RouterARPWithCoreFDB", func(t *testing.T) {
		// 模拟 172.16.35.252 (锐捷 RG-NBR, 无 LLDP、无 FDB) 直连核心交换机 hx-01 (192.168.140.254)
		devs := []*Device{
			{ID: 1, Name: "hx-01", Managed: true, Family: FamilySwitch, MgmtIP: "192.168.140.254", IPs: []string{"172.16.35.254"}, ChassisMAC: "fc:e3:3c:ad:0b:a1"},
			{ID: 2, Name: "172.16.35.252", Managed: true, Family: FamilyRouter, MgmtIP: "172.16.35.252", ChassisMAC: "4c:49:68:48:20:f5"},
		}
		ifs := []*Interface{
			{DeviceID: 1, IfIndex: 24, Name: "GE1/0/24", IfType: 6, OperStatus: "up"},
			{DeviceID: 2, IfIndex: 19, Name: "Ge0/7", IfType: 117, OperStatus: "up", MAC: "4c:49:68:48:20:fc"},
		}
		fdb := []FdbEntry{
			{DeviceID: 1, IfIndex: 24, MAC: "4c:49:68:48:20:fc", VLAN: 35},
		}
		arp := []ArpEntry{
			{DeviceID: 2, IfIndex: 19, IP: "172.16.35.254", MAC: "fc:e3:3c:ad:0b:a1"},
		}

		res := BuildTopology(TopoInput{Devices: devs, Interfaces: ifs, Fdb: fdb, Arp: arp, Now: time.Now()})
		if len(res.Links) != 1 {
			t.Fatalf("expected 1 ARP+FDB link, got %d: %+v", len(res.Links), res.Links)
		}
		l := res.Links[0]
		if l.Kind != "arp" || l.Confidence != 0.6 || l.AIfName != "GE1/0/24" || l.BIfName != "Ge0/7" {
			t.Fatalf("unexpected link: %+v", l)
		}
		if res.IfInfos[IfKey{1, 24}].Kind != IfKindUplink || res.IfInfos[IfKey{2, 19}].Kind != IfKindUplink {
			t.Fatalf("both ports should be marked uplink: %+v", res.IfInfos)
		}
	})

	t.Run("RouterBehindAccessSwitchNoFalseShortcut", func(t *testing.T) {
		// R(3) -- Acc(2) -- Core(1), 其中 Acc--Core 已有 LLDP 链路; R 的 ARP 指向 Core, 应连到 Acc 而非跨跳连 Core
		devs := []*Device{
			{ID: 1, Name: "hx-01", Managed: true, Family: FamilySwitch, IPs: []string{"172.16.35.254"}, ChassisMAC: "fc:e3:3c:ad:0b:a1"},
			{ID: 2, Name: "acc-01", Managed: true, Family: FamilySwitch, ChassisMAC: "00:00:00:00:00:02"},
			{ID: 3, Name: "nbr-01", Managed: true, Family: FamilyRouter, ChassisMAC: "4c:49:68:48:20:fc"},
		}
		ifs := []*Interface{
			{DeviceID: 1, IfIndex: 1, Name: "GE1/0/1", IfType: 6, OperStatus: "up"},
			{DeviceID: 2, IfIndex: 24, Name: "GE0/0/24", IfType: 6, OperStatus: "up"},
			{DeviceID: 2, IfIndex: 5, Name: "GE0/0/5", IfType: 6, OperStatus: "up"},
			{DeviceID: 3, IfIndex: 19, Name: "Ge0/7", IfType: 117, OperStatus: "up", MAC: "4c:49:68:48:20:fc"},
		}
		nbs := []*Neighbor{
			{DeviceID: 1, LocalIfIndex: 1, RemChassisID: "00:00:00:00:00:02", RemPortID: "GE0/0/24", RemCaps: capBridge},
			{DeviceID: 2, LocalIfIndex: 24, RemChassisID: "fc:e3:3c:ad:0b:a1", RemPortID: "GE1/0/1", RemCaps: capBridge},
		}
		fdb := []FdbEntry{
			{DeviceID: 1, IfIndex: 1, MAC: "4c:49:68:48:20:fc", VLAN: 35},
			{DeviceID: 2, IfIndex: 5, MAC: "4c:49:68:48:20:fc", VLAN: 35},
		}
		arp := []ArpEntry{
			{DeviceID: 3, IfIndex: 19, IP: "172.16.35.254", MAC: "fc:e3:3c:ad:0b:a1"},
		}

		res := BuildTopology(TopoInput{Devices: devs, Interfaces: ifs, Neighbors: nbs, Fdb: fdb, Arp: arp, Now: time.Now()})
		if len(res.Links) != 2 {
			t.Fatalf("expected 2 links (Core-Acc + Acc-Router), got %d: %+v", len(res.Links), res.Links)
		}
		foundAccRouter := false
		for _, l := range res.Links {
			if (l.ADeviceID == 1 && l.BDeviceID == 3) || (l.ADeviceID == 3 && l.BDeviceID == 1) {
				t.Fatalf("false shortcut link between Core and Router: %+v", l)
			}
			if l.ADeviceID == 2 && l.BDeviceID == 3 && l.AIfName == "GE0/0/5" && l.BIfName == "Ge0/7" {
				foundAccRouter = true
			}
		}
		if !foundAccRouter {
			t.Fatalf("expected Acc(GE0/0/5) <-> Router(Ge0/7) link, got %+v", res.Links)
		}
	})

	t.Run("PureL3ARPFallbackWithoutFDB", func(t *testing.T) {
		devs := []*Device{
			{ID: 1, Name: "hx-01", Managed: true, Family: FamilySwitch, IPs: []string{"172.16.35.254"}, ChassisMAC: "fc:e3:3c:ad:0b:a1"},
			{ID: 2, Name: "172.16.35.252", Managed: true, Family: FamilyRouter, MgmtIP: "172.16.35.252", ChassisMAC: "4c:49:68:48:20:f5"},
		}
		ifs := []*Interface{
			{DeviceID: 2, IfIndex: 19, Name: "Ge0/7", IfType: 117, OperStatus: "up", MAC: "4c:49:68:48:20:fc"},
		}
		arp := []ArpEntry{
			{DeviceID: 2, IfIndex: 19, IP: "172.16.35.254", MAC: "fc:e3:3c:ad:0b:a1"},
		}

		res := BuildTopology(TopoInput{Devices: devs, Interfaces: ifs, Arp: arp, Now: time.Now()})
		if len(res.Links) != 1 || res.Links[0].Kind != "arp" || res.Links[0].BIfName != "Ge0/7" || res.Links[0].AIfName != "L3" {
			t.Fatalf("expected L3 ARP fallback link, got %+v", res.Links)
		}
	})
}
