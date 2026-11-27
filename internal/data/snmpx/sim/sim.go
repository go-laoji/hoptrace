package sim

import (
	"context"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"hoptrace/internal/biz"
	"hoptrace/internal/data/snmpx"

	"github.com/gosnmp/gosnmp"
)

// Dialer 内置华为风格模拟网络(网段 10.255.0.0/24),供无真机时联调与端到端回归测试。
// 网络结构:
//   - 10.255.0.1  FW-Edge-01     Huawei USG6510E 防火墙(网关/ARP 源,无 FDB)
//   - 10.255.0.2  SW-Core-01     Huawei CE6850-48S6Q-EI 核心交换机(iStack 2台堆叠,含 ARP)
//   - 10.255.0.3  SW-Agg-F1      Huawei S5735-S24T4X 汇聚交换机(与 Core 有 2 条成员口组成 Eth-Trunk1)
//   - 10.255.0.11 SW-Acc-F1-A    Huawei S5720-28P-LI-AC 接入交换机(PC / IP电话 / 打印机)
//   - 10.255.0.12 SW-Acc-F1-B    Huawei S5720-28P-LI-AC 接入交换机(AP / 海康摄像头 / 服务器 / 手机 + 1台未纳管邻居交换机)
type Dialer struct {
	devices map[string]map[string]snmpx.PDU
}

func NewDialer() *Dialer {
	return &Dialer{devices: buildDemoNetwork()}
}

func (d *Dialer) Dial(_ context.Context, t biz.Target) (snmpx.Session, error) {
	if t.Credential == nil {
		return nil, fmt.Errorf("sim: missing credential")
	}
	// 允许任意非空 v2c community 或 v3 username 通过演示
	if t.Credential.Version == "v2c" && t.Credential.Community == "" {
		return nil, fmt.Errorf("sim: empty community")
	}
	mib, ok := d.devices[t.IP]
	if !ok {
		return nil, fmt.Errorf("sim: host %s unreachable", t.IP)
	}
	return &simSession{mib: mib}, nil
}

type simSession struct {
	mib map[string]snmpx.PDU
}

func (s *simSession) Get(_ context.Context, oids []string) ([]snmpx.PDU, error) {
	var out []snmpx.PDU
	for _, oid := range oids {
		k := strings.TrimPrefix(oid, ".")
		if p, ok := s.mib[k]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *simSession) Walk(_ context.Context, rootOID string) ([]snmpx.PDU, error) {
	prefix := strings.TrimPrefix(rootOID, ".") + "."
	var keys []string
	for k := range s.mib {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	out := make([]snmpx.PDU, 0, len(keys))
	for _, k := range keys {
		out = append(out, s.mib[k])
	}
	return out, nil
}

func (s *simSession) Close() error { return nil }

// ---------- 模拟 MIB 生成 ----------

type simPort struct {
	idx    int
	name   string
	ifType int
	speed  int64 // Mbps
	mac    string
	aggIdx int
}

type simLLDP struct {
	locIdx    int
	remChass  string
	remPort   string
	remSys    string
	remDesc   string
	remMgmtIP string
	remCaps   byte
}

type simFDB struct {
	vlan  int
	mac   string
	ifIdx int
}

type simARP struct {
	ifIdx int
	ip    string
	mac   string
}

type simDev struct {
	ip     string
	name   string
	descr  string
	oid    string
	loc    string
	serial string
	stack  int
	cpu    int
	mem    int
	temp   int
	chass  string
	ports  []simPort
	lldp   []simLLDP
	fdb    []simFDB
	arp    []simARP
}

func buildDemoNetwork() map[string]map[string]snmpx.PDU {
	// 终端 MAC(使用真实 OUI 以便识别厂商与类型)
	const (
		macPC1    = "f8:bc:12:11:22:01" // Dell
		macPC2    = "3c:97:0e:11:22:02" // Lenovo / Wistron
		macPhone  = "00:15:65:11:22:03" // Yealink
		macPrint  = "00:1e:8f:11:22:04" // Canon
		macAP     = "78:8a:20:11:22:05" // Ubiquiti
		macCam    = "c0:56:e3:11:22:06" // Hikvision
		macServer = "0c:c4:7a:11:22:07" // Supermicro
		macMobile = "f0:18:98:11:22:08" // Apple
	)

	arps := []simARP{
		{1, "10.255.10.101", macPC1},
		{1, "10.255.10.102", macPC2},
		{1, "10.255.20.201", macPhone},
		{1, "10.255.10.150", macPrint},
		{1, "10.255.30.10", macAP},
		{1, "10.255.40.21", macCam},
		{1, "10.255.50.10", macServer},
		{1, "10.255.10.188", macMobile},
	}

	devs := []simDev{
		{
			ip: "10.255.0.1", name: "FW-Edge-01", loc: "核心机房-A01",
			oid:    "1.3.6.1.4.1.2011.2.82.1",
			descr:  "Huawei Versatile Routing Platform Software VRP (R) software, Version 5.170 (USG6510E V600R007C20SPC300)",
			serial: "2102352UAB10N1000001", stack: 1, cpu: 18, mem: 42, temp: 39,
			chass: "48:0f:cf:00:01:00",
			ports: []simPort{
				{1, "GigabitEthernet1/0/1", 6, 1000, "48:0f:cf:00:01:01", 0},
				{2, "GigabitEthernet1/0/2", 6, 1000, "48:0f:cf:00:01:02", 0},
			},
			lldp: []simLLDP{
				{1, "48:0f:cf:00:02:00", "XGigabitEthernet0/0/1", "SW-Core-01", "Huawei CE6850", "10.255.0.2", 0x28},
			},
			arp: arps,
		},
		{
			ip: "10.255.0.2", name: "SW-Core-01", loc: "核心机房-A02",
			oid:    "1.3.6.1.4.1.2011.2.239.5",
			descr:  "Huawei Versatile Routing Platform Software VRP (R) software, Version 8.180 (CE6850-48S6Q-EI V200R005C10SPC800)",
			serial: "2102351CE610N2000002", stack: 2, cpu: 24, mem: 51, temp: 41,
			chass: "48:0f:cf:00:02:00",
			ports: []simPort{
				{1, "XGigabitEthernet0/0/1", 6, 10000, "48:0f:cf:00:02:01", 0},
				{2, "XGigabitEthernet0/0/2", 6, 10000, "48:0f:cf:00:02:02", 100},
				{3, "XGigabitEthernet0/0/3", 6, 10000, "48:0f:cf:00:02:03", 100},
				{100, "Eth-Trunk1", 161, 20000, "48:0f:cf:00:02:10", 0},
			},
			lldp: []simLLDP{
				{1, "48:0f:cf:00:01:00", "GigabitEthernet1/0/1", "FW-Edge-01", "Huawei USG6510E", "10.255.0.1", 0x08},
				{2, "48:0f:cf:00:03:00", "XGigabitEthernet0/0/1", "SW-Agg-F1", "Huawei S5735", "10.255.0.3", 0x20},
				{3, "48:0f:cf:00:03:00", "XGigabitEthernet0/0/2", "SW-Agg-F1", "Huawei S5735", "10.255.0.3", 0x20},
			},
			arp: arps,
		},
		{
			ip: "10.255.0.3", name: "SW-Agg-F1", loc: "1楼弱电间",
			oid:    "1.3.6.1.4.1.2011.2.23.600",
			descr:  "Huawei Versatile Routing Platform Software VRP (R) software, Version 5.170 (S5735-S24T4X V200R021C00SPC100)",
			serial: "2102353S5710N3000003", stack: 1, cpu: 15, mem: 36, temp: 35,
			chass: "48:0f:cf:00:03:00",
			ports: []simPort{
				{1, "XGigabitEthernet0/0/1", 6, 10000, "48:0f:cf:00:03:01", 100},
				{2, "XGigabitEthernet0/0/2", 6, 10000, "48:0f:cf:00:03:02", 100},
				{3, "GigabitEthernet0/0/1", 6, 1000, "48:0f:cf:00:03:03", 0},
				{4, "GigabitEthernet0/0/2", 6, 1000, "48:0f:cf:00:03:04", 0},
				{100, "Eth-Trunk1", 161, 20000, "48:0f:cf:00:03:10", 0},
			},
			lldp: []simLLDP{
				{1, "48:0f:cf:00:02:00", "XGigabitEthernet0/0/2", "SW-Core-01", "Huawei CE6850", "10.255.0.2", 0x28},
				{2, "48:0f:cf:00:02:00", "XGigabitEthernet0/0/3", "SW-Core-01", "Huawei CE6850", "10.255.0.2", 0x28},
				{3, "48:0f:cf:00:11:00", "GigabitEthernet0/0/24", "SW-Acc-F1-A", "Huawei S5720", "10.255.0.11", 0x20},
				{4, "48:0f:cf:00:12:00", "GigabitEthernet0/0/24", "SW-Acc-F1-B", "Huawei S5720", "10.255.0.12", 0x20},
			},
			// 汇聚上联口也会学到下游 MAC,用来验证终端引擎是否正确过滤上联口
			fdb: []simFDB{
				{10, macPC1, 3}, {10, macPC2, 3}, {20, macPhone, 3}, {10, macPrint, 3},
				{30, macAP, 4}, {40, macCam, 4}, {50, macServer, 4}, {10, macMobile, 4},
			},
		},
		{
			ip: "10.255.0.11", name: "SW-Acc-F1-A", loc: "1楼办公区A",
			oid:    "1.3.6.1.4.1.2011.2.23.380",
			descr:  "Huawei Versatile Routing Platform Software VRP (R) software, Version 5.170 (S5720-28P-LI-AC V200R011C10SPC600)",
			serial: "2102354S5710N4000011", stack: 1, cpu: 12, mem: 31, temp: 33,
			chass: "48:0f:cf:00:11:00",
			ports: []simPort{
				{1, "GigabitEthernet0/0/1", 6, 1000, "48:0f:cf:00:11:01", 0},
				{2, "GigabitEthernet0/0/2", 6, 1000, "48:0f:cf:00:11:02", 0},
				{3, "GigabitEthernet0/0/3", 6, 1000, "48:0f:cf:00:11:03", 0},
				{4, "GigabitEthernet0/0/4", 6, 1000, "48:0f:cf:00:11:04", 0},
				{24, "GigabitEthernet0/0/24", 6, 1000, "48:0f:cf:00:11:24", 0},
			},
			lldp: []simLLDP{
				{24, "48:0f:cf:00:03:00", "GigabitEthernet0/0/1", "SW-Agg-F1", "Huawei S5735", "10.255.0.3", 0x20},
				{3, macPhone, "Port1", "IPPhone-FrontDesk", "Yealink SIP-T46U", "10.255.20.201", 0x04},
			},
			fdb: []simFDB{
				{10, macPC1, 1},
				{10, macPC2, 2},
				{20, macPhone, 3},
				{10, macPrint, 4},
			},
		},
		{
			ip: "10.255.0.12", name: "SW-Acc-F1-B", loc: "1楼办公区B",
			oid:    "1.3.6.1.4.1.2011.2.23.380",
			descr:  "Huawei Versatile Routing Platform Software VRP (R) software, Version 5.170 (S5720-28P-LI-AC V200R011C10SPC600)",
			serial: "2102354S5710N4000012", stack: 1, cpu: 14, mem: 33, temp: 34,
			chass: "48:0f:cf:00:12:00",
			ports: []simPort{
				{1, "GigabitEthernet0/0/1", 6, 1000, "48:0f:cf:00:12:01", 0},
				{2, "GigabitEthernet0/0/2", 6, 1000, "48:0f:cf:00:12:02", 0},
				{3, "GigabitEthernet0/0/3", 6, 1000, "48:0f:cf:00:12:03", 0},
				{4, "GigabitEthernet0/0/4", 6, 1000, "48:0f:cf:00:12:04", 0},
				{23, "GigabitEthernet0/0/23", 6, 1000, "48:0f:cf:00:12:23", 0},
				{24, "GigabitEthernet0/0/24", 6, 1000, "48:0f:cf:00:12:24", 0},
			},
			lldp: []simLLDP{
				{24, "48:0f:cf:00:03:00", "GigabitEthernet0/0/2", "SW-Agg-F1", "Huawei S5735", "10.255.0.3", 0x20},
				{1, macAP, "eth0", "AP-F1-Meeting", "UniFi AP-AC-Pro", "10.255.30.10", 0x10},
				// 1 台无法 SNMP 访问的未纳管邻居,用来演示占位设备
				{23, "aa:bb:cc:99:00:01", "GigabitEthernet0/0/1", "SW-Unmanaged-Lab", "Third-party Switch", "10.255.0.99", 0x20},
			},
			fdb: []simFDB{
				{30, macAP, 1},
				{40, macCam, 2},
				{50, macServer, 3},
				{10, macMobile, 4},
			},
		},
	}

	out := make(map[string]map[string]snmpx.PDU, len(devs))
	for _, d := range devs {
		out[d.ip] = encodeDevice(d)
	}
	return out
}

func encodeDevice(d simDev) map[string]snmpx.PDU {
	m := map[string]snmpx.PDU{}
	putStr := func(oid, v string) { m[oid] = snmpx.PDU{OID: oid, Type: gosnmp.OctetString, Value: []byte(v)} }
	putBytes := func(oid string, v []byte) { m[oid] = snmpx.PDU{OID: oid, Type: gosnmp.OctetString, Value: v} }
	putInt := func(oid string, v int64) { m[oid] = snmpx.PDU{OID: oid, Type: gosnmp.Integer, Value: v} }

	putStr("1.3.6.1.2.1.1.1.0", d.descr)
	m["1.3.6.1.2.1.1.2.0"] = snmpx.PDU{OID: "1.3.6.1.2.1.1.2.0", Type: gosnmp.ObjectIdentifier, Value: d.oid}
	putInt("1.3.6.1.2.1.1.3.0", 86400*100*12)
	putStr("1.3.6.1.2.1.1.5.0", d.name)
	putStr("1.3.6.1.2.1.1.6.0", d.loc)
	putStr("1.3.6.1.2.1.4.20.1.1."+d.ip, d.ip)
	putBytes("1.0.8802.1.1.2.1.3.2.0", mustMAC(d.chass))

	for i := 1; i <= d.stack; i++ {
		putInt("1.3.6.1.2.1.47.1.1.1.1.5."+strconv.Itoa(i), 3)
	}
	putStr("1.3.6.1.2.1.47.1.1.1.1.11.1", d.serial)
	putInt("1.3.6.1.4.1.2011.5.25.31.1.1.1.1.5.1", int64(d.cpu))
	putInt("1.3.6.1.4.1.2011.5.25.31.1.1.1.1.7.1", int64(d.mem))
	putInt("1.3.6.1.4.1.2011.5.25.31.1.1.1.1.11.1", int64(d.temp))

	for _, p := range d.ports {
		s := strconv.Itoa(p.idx)
		putStr("1.3.6.1.2.1.2.2.1.2."+s, p.name)
		putStr("1.3.6.1.2.1.31.1.1.1.1."+s, p.name)
		putInt("1.3.6.1.2.1.2.2.1.3."+s, int64(p.ifType))
		putInt("1.3.6.1.2.1.31.1.1.1.15."+s, p.speed)
		putBytes("1.3.6.1.2.1.2.2.1.6."+s, mustMAC(p.mac))
		putInt("1.3.6.1.2.1.2.2.1.7."+s, 1)
		putInt("1.3.6.1.2.1.2.2.1.8."+s, 1)
		putInt("1.3.6.1.2.1.17.1.4.1.2."+s, int64(p.idx))
		putStr("1.0.8802.1.1.2.1.3.7.1.3."+s, p.name)
		if p.aggIdx > 0 {
			putInt("1.2.840.10006.300.43.1.2.1.1.13."+s, int64(p.aggIdx))
		}
	}

	for i, n := range d.lldp {
		idx := fmt.Sprintf("0.%d.%d", n.locIdx, i+1)
		putInt("1.0.8802.1.1.2.1.4.1.1.4."+idx, 4) // macAddress
		putBytes("1.0.8802.1.1.2.1.4.1.1.5."+idx, mustMAC(n.remChass))
		putInt("1.0.8802.1.1.2.1.4.1.1.6."+idx, 5) // interfaceName
		putStr("1.0.8802.1.1.2.1.4.1.1.7."+idx, n.remPort)
		putStr("1.0.8802.1.1.2.1.4.1.1.8."+idx, n.remPort)
		putStr("1.0.8802.1.1.2.1.4.1.1.9."+idx, n.remSys)
		putStr("1.0.8802.1.1.2.1.4.1.1.10."+idx, n.remDesc)
		putBytes("1.0.8802.1.1.2.1.4.1.1.12."+idx, []byte{n.remCaps, 0})
		if n.remMgmtIP != "" {
			putInt("1.0.8802.1.1.2.1.4.2.1.3."+idx+".1.4."+n.remMgmtIP, 2)
		}
	}

	for _, f := range d.fdb {
		dec := macToDecOID(f.mac)
		suf := fmt.Sprintf("%d.%s", f.vlan, dec)
		putInt("1.3.6.1.2.1.17.7.1.2.2.1.2."+suf, int64(f.ifIdx))
		putInt("1.3.6.1.2.1.17.7.1.2.2.1.3."+suf, 3) // learned
	}

	for _, a := range d.arp {
		suf := fmt.Sprintf("%d.1.4.%s", a.ifIdx, a.ip)
		putBytes("1.3.6.1.2.1.4.35.1.4."+suf, mustMAC(a.mac))
	}

	return m
}

func mustMAC(s string) []byte {
	b, _ := hex.DecodeString(strings.ReplaceAll(s, ":", ""))
	return b
}

func macToDecOID(s string) string {
	b := mustMAC(s)
	parts := make([]string, len(b))
	for i, x := range b {
		parts[i] = strconv.Itoa(int(x))
	}
	return strings.Join(parts, ".")
}
