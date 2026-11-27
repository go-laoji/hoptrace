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
