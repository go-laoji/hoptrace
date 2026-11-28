package snmpx

import (
	"context"
	"strings"
	"testing"

	"hoptrace/internal/biz"
)

func TestWalkReplayHuaweiSwitchSample(t *testing.T) {
	sample := `
.1.3.6.1.2.1.1.1.0 = STRING: "Huawei Versatile Routing Platform Software VRP (R) software, Version 5.170 (S5720-28P-LI-AC V200R011C10SPC600)"
.1.3.6.1.2.1.1.2.0 = OID: .1.3.6.1.4.1.2011.2.23.380
.1.3.6.1.2.1.1.3.0 = Timeticks: (8640000) 1 day, 0:00:00.00
.1.3.6.1.2.1.1.5.0 = STRING: "SW-Lab-S5720"
.1.3.6.1.2.1.2.2.1.2.5 = STRING: "GigabitEthernet0/0/1"
.1.3.6.1.2.1.2.2.1.3.5 = INTEGER: ethernetCsmacd(6)
.1.3.6.1.2.1.2.2.1.6.5 = Hex-STRING: 48 0F CF 12 34 05
.1.3.6.1.2.1.2.2.1.7.5 = INTEGER: up(1)
.1.3.6.1.2.1.2.2.1.8.5 = INTEGER: up(1)
.1.3.6.1.2.1.31.1.1.1.1.5 = STRING: "GigabitEthernet0/0/1"
.1.3.6.1.2.1.31.1.1.1.15.5 = Gauge32: 1000
.1.3.6.1.2.1.17.1.4.1.2.1 = INTEGER: 5
.1.3.6.1.2.1.17.7.1.2.2.1.2.10.248.188.18.17.34.51 = INTEGER: 1
.1.3.6.1.2.1.17.7.1.2.2.1.3.10.248.188.18.17.34.51 = INTEGER: 3
.1.3.6.1.4.1.2011.5.25.31.1.1.1.1.5.9 = INTEGER: 21
.1.3.6.1.4.1.2011.5.25.31.1.1.1.1.7.9 = INTEGER: 44
.1.3.6.1.4.1.2011.5.25.31.1.1.1.1.11.9 = INTEGER: 38
`
	sess, err := ParseWalkFile(strings.NewReader(sample))
	if err != nil {
		t.Fatalf("ParseWalkFile: %v", err)
	}
	col := NewCollector(SingleSessionDialer{Session: sess})
	snap, err := col.Collect(context.Background(), biz.Target{IP: "10.0.0.10"})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if snap.Sys.Name != "SW-Lab-S5720" || snap.Sys.UptimeSecs != 86400 {
		t.Fatalf("unexpected sys: %+v", snap.Sys)
	}
	if len(snap.Interfaces) != 1 || snap.Interfaces[0].Name != "GigabitEthernet0/0/1" || snap.Interfaces[0].MAC != "48:0f:cf:12:34:05" {
		t.Fatalf("unexpected interfaces: %+v", snap.Interfaces)
	}
	if len(snap.Fdb) != 1 || snap.Fdb[0].MAC != "f8:bc:12:11:22:33" || snap.Fdb[0].VLAN != 10 || snap.Fdb[0].IfIndex != 5 {
		t.Fatalf("unexpected fdb: %+v", snap.Fdb)
	}
	if snap.CPU != 21 || snap.Mem != 44 || snap.Temp != 38 {
		t.Fatalf("unexpected health: cpu=%d mem=%d temp=%d", snap.CPU, snap.Mem, snap.Temp)
	}
}

func TestWalkReplayHuaweiS9312TrunkAndBridgeFDB(t *testing.T) {
	// 基于真实 S9312 V200R003C00SPC500 样本特征的脱敏回归测试:
	// - Eth-Trunk2 ifType=6 (非 161), 通过 hwTrunkIfIndex + hwTrunkMemStatus 还原成员口 71/72 -> 166
	// - Q-BRIDGE-MIB 为空, 走 BRIDGE-MIB (dot1dTpFdb) 兜底
	sample := `
.1.3.6.1.2.1.1.1.0 = STRING: "Quidway S9312  Huawei Versatile Routing Platform Software  VRP (R) Software, Version 5.130 (S9300 V200R003C00SPC500)"
.1.3.6.1.2.1.1.2.0 = OID: .1.3.6.1.4.1.2011.2.170.3
.1.3.6.1.2.1.1.5.0 = STRING: "hx-01"
.1.3.6.1.2.1.2.2.1.2.71 = STRING: "GigabitEthernet10/0/36"
.1.3.6.1.2.1.2.2.1.3.71 = INTEGER: 6
.1.3.6.1.2.1.2.2.1.8.71 = INTEGER: 1
.1.3.6.1.2.1.31.1.1.1.1.71 = STRING: "GigabitEthernet10/0/36"
.1.3.6.1.2.1.2.2.1.2.166 = STRING: "Eth-Trunk2"
.1.3.6.1.2.1.2.2.1.3.166 = INTEGER: 6
.1.3.6.1.2.1.2.2.1.8.166 = INTEGER: 1
.1.3.6.1.2.1.31.1.1.1.1.166 = STRING: "Eth-Trunk2"
.1.3.6.1.4.1.2011.5.25.41.1.3.3.1.4.1 = INTEGER: 166
.1.3.6.1.4.1.2011.5.25.41.1.4.1.1.2.1.71 = INTEGER: 1
.1.3.6.1.2.1.17.1.4.1.2.99 = INTEGER: 166
.1.3.6.1.2.1.17.4.3.1.2.248.188.18.17.34.51 = INTEGER: 99
.1.3.6.1.2.1.17.4.3.1.3.248.188.18.17.34.51 = INTEGER: 3
`
	sess, err := ParseWalkFile(strings.NewReader(sample))
	if err != nil {
		t.Fatalf("ParseWalkFile: %v", err)
	}
	col := NewCollector(SingleSessionDialer{Session: sess})
	snap, err := col.Collect(context.Background(), biz.Target{IP: "192.168.140.254"})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	fp := biz.Identify(snap.Sys.ObjectID, snap.Sys.Descr)
	if fp.Model != "S9312" || fp.OSVersion != "V200R003C00SPC500" {
		t.Fatalf("unexpected fingerprint: %+v", fp)
	}
	byIdx := map[int]*biz.Interface{}
	for _, it := range snap.Interfaces {
		byIdx[it.IfIndex] = it
	}
	if byIdx[166] == nil || byIdx[166].IfType != 161 {
		t.Fatalf("Eth-Trunk2 should be normalized to ifType=161: %+v", byIdx[166])
	}
	if byIdx[71] == nil || byIdx[71].AggIfIndex != 166 {
		t.Fatalf("GE10/0/36 should map to AggIfIndex=166 via hwTrunk: %+v", byIdx[71])
	}
	if len(snap.Fdb) != 1 || snap.Fdb[0].IfIndex != 166 || snap.Fdb[0].MAC != "f8:bc:12:11:22:33" {
		t.Fatalf("BRIDGE-MIB FDB fallback failed: %+v", snap.Fdb)
	}
}

func TestWalkReplayRuijieNBR7204E(t *testing.T) {
	// 模拟锐捷 RG-NBR-N7204-E(v1.11) 网关:
	// - sysObjectID 为 Net-SNMP(8072) 或 4881, sysDescr 含 RG-NBR-N7204-E(v1.11)
	// - 无 LLDP/BRIDGE-MIB, 底盘 MAC 应自动回退到物理口 MAC
	// - 无 4881 私有健康节点, 应通过 HOST-RESOURCES / UCD-SNMP 通用兜底读出 CPU 与内存
	sample := `
.1.3.6.1.2.1.1.1.0 = STRING: "RG-NBR-N7204-E(v1.11)"
.1.3.6.1.2.1.1.2.0 = OID: .1.3.6.1.4.1.8072.3.2.10
.1.3.6.1.2.1.1.3.0 = Timeticks: (4320000) 12:00:00.00
.1.3.6.1.2.1.1.5.0 = STRING: "GW-NBR-7204"
.1.3.6.1.2.1.2.2.1.2.1 = STRING: "GigabitEthernet 0/0"
.1.3.6.1.2.1.2.2.1.3.1 = INTEGER: ethernetCsmacd(6)
.1.3.6.1.2.1.2.2.1.5.1 = Gauge32: 1000000000
.1.3.6.1.2.1.2.2.1.6.1 = Hex-STRING: 58 69 6C 11 22 33
.1.3.6.1.2.1.2.2.1.7.1 = INTEGER: up(1)
.1.3.6.1.2.1.2.2.1.8.1 = INTEGER: up(1)
.1.3.6.1.2.1.2.2.1.2.2 = STRING: "br-lan"
.1.3.6.1.2.1.2.2.1.3.2 = INTEGER: ethernetCsmacd(6)
.1.3.6.1.2.1.4.20.1.1.10.10.102.1 = IpAddress: 10.10.102.1
.1.3.6.1.2.1.4.22.1.2.1.10.10.102.50 = Hex-STRING: AA BB CC 01 02 03
.1.3.6.1.2.1.25.3.3.1.2.1 = INTEGER: 18
.1.3.6.1.2.1.25.3.3.1.2.2 = INTEGER: 26
.1.3.6.1.2.1.25.2.3.1.2.1 = OID: .1.3.6.1.2.1.25.2.1.2
.1.3.6.1.2.1.25.2.3.1.5.1 = INTEGER: 2048000
.1.3.6.1.2.1.25.2.3.1.6.1 = INTEGER: 1024000
`
	sess, err := ParseWalkFile(strings.NewReader(sample))
	if err != nil {
		t.Fatalf("ParseWalkFile: %v", err)
	}
	col := NewCollector(SingleSessionDialer{Session: sess})
	snap, err := col.Collect(context.Background(), biz.Target{IP: "10.10.102.1"})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	fp := biz.Identify(snap.Sys.ObjectID, snap.Sys.Descr)
	if fp.Vendor != "Ruijie" || fp.Family != biz.FamilyRouter || fp.Model != "RG-NBR-N7204-E" || fp.OSVersion != "v1.11" {
		t.Fatalf("unexpected Ruijie NBR fingerprint: %+v", fp)
	}
	if snap.ChassisMAC != "58:69:6c:11:22:33" {
		t.Fatalf("expected physical interface MAC fallback for ChassisMAC, got %q", snap.ChassisMAC)
	}
	if len(snap.Interfaces) != 1 || snap.Interfaces[0].Name != "GigabitEthernet 0/0" || snap.Interfaces[0].SpeedMbps != 1000 {
		t.Fatalf("unexpected interfaces: %+v", snap.Interfaces)
	}
	if snap.CPU != 22 || snap.Mem != 50 {
		t.Fatalf("expected HOST-RESOURCES fallback cpu=22 mem=50, got cpu=%d mem=%d", snap.CPU, snap.Mem)
	}
	if len(snap.Arp) != 1 || snap.Arp[0].IP != "10.10.102.50" || snap.Arp[0].MAC != "aa:bb:cc:01:02:03" {
		t.Fatalf("unexpected arp: %+v", snap.Arp)
	}
}

func TestWalkReplayRuijieNBR7204ECalibrated(t *testing.T) {
	// 基于真实 RG-NBR-N7204-E (v1.11 / NBR_NTOS 1.0R13P1) 样本特征的脱敏回归测试:
	// - sysDescr 为 Linux Ruijie 内核串, 通过 sysObjectID .4881.1.6.1.1.16 识别型号 RG-NBR-N7204-E 与 router 家族
	// - CPU/Mem 优先走 4881.1.6.1.2.3.1.3.0 (6%) 与 .4.1.7.0 (40%), 不受 DPDK 满载核(hrProcessorLoad=100)和 Linux 缓存干扰
	// - 序列号与版本从 .5.1.9.0 / .5.1.10.0 / .5.1.11.0 读取
	// - ifOperStatus=7 (lowerLayerDown) 规范化为 down; br0(209)/Ge0/3.33(135) 自动过滤
	sample := `
.1.3.6.1.2.1.1.1.0 = STRING: "Linux Ruijie 4.14.76-g4a2d7de97 #1 SMP PREEMPT aarch64"
.1.3.6.1.2.1.1.2.0 = OID: .1.3.6.1.4.1.4881.1.6.1.1.16
.1.3.6.1.2.1.1.3.0 = Timeticks: (161800) 1618s
.1.3.6.1.2.1.2.2.1.2.12 = STRING: "Ge0/0"
.1.3.6.1.2.1.2.2.1.3.12 = INTEGER: 117
.1.3.6.1.2.1.2.2.1.6.12 = Hex-STRING: 4C 49 68 00 00 01
.1.3.6.1.2.1.2.2.1.7.12 = INTEGER: 1
.1.3.6.1.2.1.2.2.1.8.12 = INTEGER: 7
.1.3.6.1.2.1.31.1.1.1.1.12 = STRING: "Ge0/0"
.1.3.6.1.2.1.31.1.1.1.15.12 = Gauge32: 1000
.1.3.6.1.2.1.2.2.1.2.18 = STRING: "Ge0/3"
.1.3.6.1.2.1.2.2.1.3.18 = INTEGER: 117
.1.3.6.1.2.1.2.2.1.6.18 = Hex-STRING: 4C 49 68 00 00 04
.1.3.6.1.2.1.2.2.1.7.18 = INTEGER: 1
.1.3.6.1.2.1.2.2.1.8.18 = INTEGER: 1
.1.3.6.1.2.1.31.1.1.1.1.18 = STRING: "Ge0/3"
.1.3.6.1.2.1.31.1.1.1.15.18 = Gauge32: 1000
.1.3.6.1.2.1.2.2.1.2.24 = STRING: "br0"
.1.3.6.1.2.1.2.2.1.3.24 = INTEGER: 209
.1.3.6.1.2.1.2.2.1.2.26 = STRING: "Ge0/3.33"
.1.3.6.1.2.1.2.2.1.3.26 = INTEGER: 135
.1.3.6.1.2.1.25.3.3.1.2.196608 = INTEGER: 13
.1.3.6.1.2.1.25.3.3.1.2.196609 = INTEGER: 100
.1.3.6.1.4.1.4881.1.6.1.2.3.1.3.0 = Gauge32: 6
.1.3.6.1.4.1.4881.1.6.1.2.4.1.7.0 = Gauge32: 40
.1.3.6.1.4.1.4881.1.6.1.2.5.1.9.0 = STRING: "SN-NBR7204-TEST"
.1.3.6.1.4.1.4881.1.6.1.2.5.1.10.0 = STRING: "1.11"
.1.3.6.1.4.1.4881.1.6.1.2.5.1.11.0 = STRING: "NBR_NTOS 1.0R13P1"
`
	sess, err := ParseWalkFile(strings.NewReader(sample))
	if err != nil {
		t.Fatalf("ParseWalkFile: %v", err)
	}
	snap, err := NewCollector(SingleSessionDialer{Session: sess}).Collect(context.Background(), biz.Target{IP: "10.0.0.252"})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	fp := biz.Identify(snap.Sys.ObjectID, snap.Sys.Descr)
	if fp.Vendor != "Ruijie" || fp.Family != biz.FamilyRouter || fp.Model != "RG-NBR-N7204-E" {
		t.Fatalf("unexpected fingerprint: %+v", fp)
	}
	if snap.CPU != 6 || snap.Mem != 40 {
		t.Fatalf("expected NBR private health cpu=6 mem=40, got cpu=%d mem=%d", snap.CPU, snap.Mem)
	}
	if snap.Serial != "SN-NBR7204-TEST" || snap.OSVersion != "NBR_NTOS 1.0R13P1 (v1.11)" {
		t.Fatalf("unexpected entity extra: serial=%q osVer=%q", snap.Serial, snap.OSVersion)
	}
	if snap.ChassisMAC != "4c:49:68:00:00:01" {
		t.Fatalf("unexpected ChassisMAC: %q", snap.ChassisMAC)
	}
	if len(snap.Interfaces) != 2 || snap.Interfaces[0].OperStatus != "down" || snap.Interfaces[1].OperStatus != "up" {
		t.Fatalf("unexpected interfaces: %+v", snap.Interfaces)
	}
}

func TestWalkReplayRuijieRGOSPrivateMIB(t *testing.T) {
	sample := `
.1.3.6.1.2.1.1.1.0 = STRING: "Ruijie Gigabit Ethernet Switch(RG-S5750-24GT/8SFP-P), RGOS 11.4(1)B2"
.1.3.6.1.2.1.1.2.0 = OID: .1.3.6.1.4.1.4881.1.1.10.1.123
.1.3.6.1.2.1.1.5.0 = STRING: "SW-Ruijie-01"
.1.3.6.1.2.1.2.2.1.2.1 = STRING: "GigabitEthernet 0/1"
.1.3.6.1.2.1.2.2.1.3.1 = INTEGER: 6
.1.3.6.1.2.1.2.2.1.8.1 = INTEGER: 1
.1.3.6.1.2.1.2.2.1.2.50 = STRING: "AggregatePort 1"
.1.3.6.1.2.1.2.2.1.3.50 = INTEGER: 6
.1.3.6.1.2.1.2.2.1.8.50 = INTEGER: 1
.1.3.6.1.4.1.4881.1.1.10.2.36.1.1.2.0 = INTEGER: 15
.1.3.6.1.4.1.4881.1.1.10.2.35.1.1.1.3.1 = INTEGER: 42
.1.3.6.1.4.1.4881.1.1.10.2.21.1.4.1.3.1.1 = INTEGER: 39
.1.3.6.1.4.1.4881.1.1.10.2.21.1.2.1.1.6.1 = STRING: "G1L900123456"
`
	sess, err := ParseWalkFile(strings.NewReader(sample))
	if err != nil {
		t.Fatalf("ParseWalkFile: %v", err)
	}
	col := NewCollector(SingleSessionDialer{Session: sess})
	snap, err := col.Collect(context.Background(), biz.Target{IP: "10.0.0.20"})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if snap.CPU != 15 || snap.Mem != 42 || snap.Temp != 39 {
		t.Fatalf("unexpected Ruijie private health: cpu=%d mem=%d temp=%d", snap.CPU, snap.Mem, snap.Temp)
	}
	if snap.Serial != "G1L900123456" {
		t.Fatalf("unexpected Ruijie serial: %q", snap.Serial)
	}
	if len(snap.Interfaces) != 2 || snap.Interfaces[1].IfType != 161 {
		t.Fatalf("AggregatePort should normalize to ifType=161: %+v", snap.Interfaces)
	}
}

func TestWalkReplayH3CAndCiscoAndFortinet(t *testing.T) {
	t.Run("H3C", func(t *testing.T) {
		sample := `
.1.3.6.1.2.1.1.1.0 = STRING: "H3C Comware Platform Software, Software Version 7.1.070, Release 6326P01 H3C S5130S-28P-EI"
.1.3.6.1.2.1.1.2.0 = OID: .1.3.6.1.4.1.25506.1.513
.1.3.6.1.2.1.2.2.1.2.10 = STRING: "Bridge-Aggregation1"
.1.3.6.1.2.1.2.2.1.3.10 = INTEGER: 6
.1.3.6.1.4.1.25506.2.6.1.1.1.1.6.1 = INTEGER: 19
.1.3.6.1.4.1.25506.2.6.1.1.1.1.8.1 = INTEGER: 53
.1.3.6.1.4.1.25506.2.6.1.1.1.1.12.1 = INTEGER: 36
`
		sess, _ := ParseWalkFile(strings.NewReader(sample))
		snap, err := NewCollector(SingleSessionDialer{Session: sess}).Collect(context.Background(), biz.Target{IP: "10.0.0.30"})
		if err != nil {
			t.Fatalf("Collect: %v", err)
		}
		if snap.CPU != 19 || snap.Mem != 53 || snap.Temp != 36 {
			t.Fatalf("unexpected H3C health: cpu=%d mem=%d temp=%d", snap.CPU, snap.Mem, snap.Temp)
		}
		if len(snap.Interfaces) != 1 || snap.Interfaces[0].IfType != 161 {
			t.Fatalf("Bridge-Aggregation should normalize to 161: %+v", snap.Interfaces)
		}
	})

	t.Run("Cisco", func(t *testing.T) {
		sample := `
.1.3.6.1.2.1.1.1.0 = STRING: "Cisco IOS Software, C2960X Software (WS-C2960X-48FPS-L), Version 15.2(4)E7"
.1.3.6.1.2.1.1.2.0 = OID: .1.3.6.1.4.1.9.1.1745
.1.3.6.1.2.1.2.2.1.2.10 = STRING: "Port-channel1"
.1.3.6.1.2.1.2.2.1.3.10 = INTEGER: 6
.1.3.6.1.4.1.9.9.109.1.1.1.1.7.1 = Gauge32: 27
.1.3.6.1.4.1.9.9.48.1.1.1.5.1 = Gauge32: 60000000
.1.3.6.1.4.1.9.9.48.1.1.1.6.1 = Gauge32: 40000000
.1.3.6.1.4.1.9.9.13.1.3.1.3.1001 = Gauge32: 41
`
		sess, _ := ParseWalkFile(strings.NewReader(sample))
		snap, err := NewCollector(SingleSessionDialer{Session: sess}).Collect(context.Background(), biz.Target{IP: "10.0.0.40"})
		if err != nil {
			t.Fatalf("Collect: %v", err)
		}
		if snap.CPU != 27 || snap.Mem != 60 || snap.Temp != 41 {
			t.Fatalf("unexpected Cisco health: cpu=%d mem=%d temp=%d", snap.CPU, snap.Mem, snap.Temp)
		}
		if len(snap.Interfaces) != 1 || snap.Interfaces[0].IfType != 161 {
			t.Fatalf("Port-channel should normalize to 161: %+v", snap.Interfaces)
		}
	})

	t.Run("Fortinet", func(t *testing.T) {
		sample := `
.1.3.6.1.2.1.1.1.0 = STRING: "FortiGate-100F v7.2.4,build1396"
.1.3.6.1.2.1.1.2.0 = OID: .1.3.6.1.4.1.12356.101.1.1000
.1.3.6.1.4.1.12356.100.1.1.1.0 = STRING: "FG100FTK21001234"
.1.3.6.1.4.1.12356.101.4.1.3.0 = Gauge32: 12
.1.3.6.1.4.1.12356.101.4.1.4.0 = Gauge32: 38
`
		sess, _ := ParseWalkFile(strings.NewReader(sample))
		snap, err := NewCollector(SingleSessionDialer{Session: sess}).Collect(context.Background(), biz.Target{IP: "10.0.0.50"})
		if err != nil {
			t.Fatalf("Collect: %v", err)
		}
		if snap.CPU != 12 || snap.Mem != 38 || snap.Serial != "FG100FTK21001234" {
			t.Fatalf("unexpected Fortinet snapshot: cpu=%d mem=%d serial=%q", snap.CPU, snap.Mem, snap.Serial)
		}
	})
}
