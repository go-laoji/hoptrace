// hoptrace-walk: 独立的纯 Go SNMP 数据导出工具(无 Net-SNMP 依赖,支持 SNMPv3 AES192/AES192C/AES256/AES256C)。
// 输出标准 `snmpwalk -On` 兼容格式,可直接用于 HopTrace 离线回放与华为 MIB 校准(Issue #13)。
package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"hoptrace/internal/biz"
	"hoptrace/internal/data/snmpx"

	"github.com/gosnmp/gosnmp"
)

type walkTree struct {
	name string
	oid  string
}

var defaultTrees = []walkTree{
	{"SNMPv2-MIB system", "1.3.6.1.2.1.1"},
	{"IF-MIB ifTable", "1.3.6.1.2.1.2.2"},
	{"IF-MIB ifXTable", "1.3.6.1.2.1.31.1.1"},
	{"IP-MIB ipAddrTable", "1.3.6.1.2.1.4.20"},
	{"RFC1213 ARP (ipNetToMedia)", "1.3.6.1.2.1.4.22"},
	{"IP-MIB ARP (ipNetToPhysical)", "1.3.6.1.2.1.4.35"},
	{"IEEE8023-LAG dot3adAggPort", "1.2.840.10006.300.43.1.2.1.1.13"},
	{"BRIDGE-MIB base", "1.3.6.1.2.1.17.1"},
	{"BRIDGE-MIB FDB (dot1dTpFdb)", "1.3.6.1.2.1.17.4.3"},
	{"Q-BRIDGE-MIB FDB (dot1qTpFdb)", "1.3.6.1.2.1.17.7.1.2.2"},
	{"LLDP-MIB local", "1.0.8802.1.1.2.1.3"},
	{"LLDP-MIB remote", "1.0.8802.1.1.2.1.4"},
	{"ENTITY-MIB class", "1.3.6.1.2.1.47.1.1.1.1.5"},
	{"ENTITY-MIB serial", "1.3.6.1.2.1.47.1.1.1.1.11"},
	{"Huawei ENTITY CPU", "1.3.6.1.4.1.2011.5.25.31.1.1.1.1.5"},
	{"Huawei ENTITY Mem", "1.3.6.1.4.1.2011.5.25.31.1.1.1.1.7"},
	{"Huawei ENTITY Temp", "1.3.6.1.4.1.2011.5.25.31.1.1.1.1.11"},
	{"Huawei Trunk/LAG (hwTrunk)", "1.3.6.1.4.1.2011.5.25.41"},
	{"Huawei NDP/NTDP (hgmp)", "1.3.6.1.4.1.2011.6.7"},
}

func main() {
	host := flag.String("h", "", "目标设备 IP (必填)")
	port := flag.Int("p", 161, "SNMP 端口")
	ver := flag.String("v", "3", "SNMP 版本: v2c / v3 (或 2c / 3)")
	comm := flag.String("c", "public", "SNMP v2c 只读团体字")
	user := flag.String("u", "", "SNMP v3 用户名")
	authProto := flag.String("a", "SHA", "SNMP v3 认证算法: MD5/SHA/SHA224/SHA256/SHA384/SHA512/NONE")
	authPass := flag.String("A", "", "SNMP v3 认证密码")
	privProto := flag.String("x", "AES192", "SNMP v3 加密算法: DES/AES/AES192/AES192C/AES256/AES256C/NONE")
	privPass := flag.String("X", "", "SNMP v3 加密密码")
	outFile := flag.String("o", "device.walk", "输出文件路径")
	timeout := flag.Duration("timeout", 5*time.Second, "单次 SNMP 请求超时")
	retries := flag.Int("retries", 2, "重试次数")
	maxRep := flag.Uint("max-rep", 20, "GetBulk max-repetitions")
	flag.Parse()

	if strings.TrimSpace(*host) == "" {
		fmt.Fprintf(os.Stderr, "用法: %s -h <IP> [-v 2c|3] [-c community] [-u user -a SHA -A auth -x AES192 -X priv] [-o out.walk]\n", os.Args[0])
		os.Exit(1)
	}

	version := strings.ToLower(strings.TrimSpace(*ver))
	if version == "2c" {
		version = "v2c"
	} else if version == "3" {
		version = "v3"
	}

	cred := &biz.Credential{
		Version:      version,
		Community:    *comm,
		Username:     *user,
		AuthProtocol: normalizeProto(*authProto),
		AuthPassword: *authPass,
		PrivProtocol: normalizeProto(*privProto),
		PrivPassword: *privPass,
		Port:         *port,
	}

	dialer := &snmpx.GoSNMPDialer{
		Timeout:        *timeout,
		Retries:        *retries,
		MaxRepetitions: uint32(*maxRep),
	}

	ctx := context.Background()
	sess, err := dialer.Dial(ctx, biz.Target{IP: *host, Port: *port, Credential: cred})
	if err != nil {
		fmt.Fprintf(os.Stderr, "连接失败: %v\n", err)
		os.Exit(1)
	}
	defer sess.Close()

	f, err := os.Create(*outFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建输出文件失败: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()
	w := bufio.NewWriter(f)

	fmt.Fprintf(w, "# hoptrace-walk host=%s version=%s time=%s\n", *host, version, time.Now().Format(time.RFC3339))

	total := 0
	for _, tree := range defaultTrees {
		t0 := time.Now()
		pdus, err := sess.Walk(ctx, tree.oid)
		dur := time.Since(t0).Round(time.Millisecond)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] %-32s (%s): err=%v (%s)\n", tree.name, tree.oid, err, dur)
			fmt.Fprintf(w, "# WARN %s (%s): %v\n", tree.name, tree.oid, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "[ OK ] %-32s (%s): %5d 条 (%s)\n", tree.name, tree.oid, len(pdus), dur)
		fmt.Fprintf(w, "# %s (%s): %d\n", tree.name, tree.oid, len(pdus))
		for _, p := range pdus {
			fmt.Fprintf(w, ".%s = %s\n", strings.TrimPrefix(p.OID, "."), formatPDU(p))
		}
		total += len(pdus)
	}
	_ = w.Flush()
	fmt.Fprintf(os.Stderr, "\n导出完成: %s (共 %d 条记录)\n", *outFile, total)
}

func normalizeProto(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", "")
	if s == "NONE" || s == "NOAUTH" || s == "NOPRIV" {
		return ""
	}
	return s
}

func formatPDU(p snmpx.PDU) string {
	switch p.Type {
	case gosnmp.OctetString:
		var b []byte
		switch v := p.Value.(type) {
		case []byte:
			b = v
		case string:
			b = []byte(v)
		}
		if forceHexOID(p.OID) {
			return "Hex-STRING: " + formatHex(b)
		}
		if isPrintableASCII(b) {
			s := strings.ReplaceAll(string(b), "\r", " ")
			s = strings.ReplaceAll(s, "\n", " ")
			return fmt.Sprintf("STRING: %q", s)
		}
		return "Hex-STRING: " + formatHex(b)
	case gosnmp.ObjectIdentifier:
		s := fmt.Sprintf("%v", p.Value)
		if !strings.HasPrefix(s, ".") {
			s = "." + s
		}
		return "OID: " + s
	case gosnmp.IPAddress:
		return fmt.Sprintf("IpAddress: %v", p.Value)
	case gosnmp.TimeTicks:
		n := gosnmp.ToBigInt(p.Value).Int64()
		return fmt.Sprintf("Timeticks: (%d) %ds", n, n/100)
	case gosnmp.Gauge32:
		return fmt.Sprintf("Gauge32: %s", gosnmp.ToBigInt(p.Value).String())
	case gosnmp.Counter32:
		return fmt.Sprintf("Counter32: %s", gosnmp.ToBigInt(p.Value).String())
	case gosnmp.Counter64:
		return fmt.Sprintf("Counter64: %s", gosnmp.ToBigInt(p.Value).String())
	default:
		return fmt.Sprintf("INTEGER: %s", gosnmp.ToBigInt(p.Value).String())
	}
}

func isPrintableASCII(b []byte) bool {
	if len(b) == 0 {
		return true
	}
	// 6 字节常见为 MAC 地址, 若含控制字符或非 ASCII 则输出 Hex-STRING
	if len(b) == 6 {
		for _, c := range b {
			if c < 0x20 || c > 0x7e {
				return false
			}
		}
	}
	if !utf8.Valid(b) {
		return false
	}
	for _, c := range b {
		if c < 0x20 && c != '\t' && c != '\n' && c != '\r' {
			return false
		}
		if c == 0x7f {
			return false
		}
	}
	return true
}

func forceHexOID(oid string) bool {
	oid = strings.TrimPrefix(oid, ".")
	for _, p := range []string{
		"1.3.6.1.2.1.2.2.1.6.",       // ifPhysAddress
		"1.3.6.1.2.1.4.22.1.2.",      // ipNetToMediaPhysAddress
		"1.3.6.1.2.1.4.35.1.4.",      // ipNetToPhysicalPhysAddress
		"1.3.6.1.2.1.17.1.1.0",       // dot1dBaseBridgeAddress
		"1.0.8802.1.1.2.1.3.2.0",     // lldpLocChassisId
		"1.0.8802.1.1.2.1.4.1.1.5.",  // lldpRemChassisId
		"1.0.8802.1.1.2.1.4.1.1.12.", // lldpRemSysCapEnabled
	} {
		if oid == p || strings.HasPrefix(oid, p) {
			return true
		}
	}
	return false
}

func formatHex(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	h := strings.ToUpper(hex.EncodeToString(b))
	parts := make([]string, 0, len(b))
	for i := 0; i < len(h); i += 2 {
		parts = append(parts, h[i:i+2])
	}
	return strings.Join(parts, " ")
}
