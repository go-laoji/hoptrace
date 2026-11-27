package snmpx

import (
	"bufio"
	"context"
	"encoding/hex"
	"io"
	"sort"
	"strconv"
	"strings"

	"hoptrace/internal/biz"

	"github.com/gosnmp/gosnmp"
)

// WalkReplaySession 从标准 `snmpwalk -On` 输出构造的离线回放 Session,
// 用于将真机采集的 .walk 样本放入 testdata/ 后做回归测试。
type WalkReplaySession struct {
	pdus map[string]PDU
}

// ParseWalkFile 解析 `snmpwalk -On` 文本输出。
func ParseWalkFile(r io.Reader) (*WalkReplaySession, error) {
	pdus := map[string]PDU{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, " = ", 2)
		if len(parts) != 2 {
			continue
		}
		oid := strings.TrimPrefix(strings.TrimSpace(parts[0]), ".")
		rhs := strings.TrimSpace(parts[1])
		p, ok := parseWalkValue(oid, rhs)
		if ok {
			pdus[oid] = p
		}
	}
	return &WalkReplaySession{pdus: pdus}, sc.Err()
}

func parseWalkValue(oid, rhs string) (PDU, bool) {
	colon := strings.Index(rhs, ":")
	if colon <= 0 {
		return PDU{}, false
	}
	typ := strings.TrimSpace(rhs[:colon])
	val := strings.TrimSpace(rhs[colon+1:])

	switch typ {
	case "STRING":
		val = strings.Trim(val, `"`)
		return PDU{OID: oid, Type: gosnmp.OctetString, Value: []byte(val)}, true
	case "Hex-STRING":
		hexStr := strings.ReplaceAll(strings.ReplaceAll(val, " ", ""), ":", "")
		b, err := hex.DecodeString(hexStr)
		if err != nil {
			return PDU{}, false
		}
		return PDU{OID: oid, Type: gosnmp.OctetString, Value: b}, true
	case "OID":
		return PDU{OID: oid, Type: gosnmp.ObjectIdentifier, Value: strings.TrimPrefix(val, ".")}, true
	case "INTEGER", "Gauge32", "Counter32", "Counter64":
		// 兼容 "up(1)" 形式
		if i := strings.Index(val, "("); i >= 0 {
			if j := strings.Index(val[i:], ")"); j > 1 {
				val = val[i+1 : i+j]
			}
		}
		n, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
		if err != nil {
			return PDU{}, false
		}
		return PDU{OID: oid, Type: gosnmp.Integer, Value: n}, true
	case "Timeticks":
		// 格式: (123456) 0:20:34.56
		if i := strings.Index(val, "("); i >= 0 {
			if j := strings.Index(val[i:], ")"); j > 1 {
				if n, err := strconv.ParseInt(val[i+1:i+j], 10, 64); err == nil {
					return PDU{OID: oid, Type: gosnmp.TimeTicks, Value: n}, true
				}
			}
		}
	case "IpAddress":
		return PDU{OID: oid, Type: gosnmp.IPAddress, Value: val}, true
	}
	return PDU{}, false
}

func (s *WalkReplaySession) Get(_ context.Context, oids []string) ([]PDU, error) {
	var out []PDU
	for _, oid := range oids {
		k := strings.TrimPrefix(oid, ".")
		if p, ok := s.pdus[k]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *WalkReplaySession) Walk(_ context.Context, rootOID string) ([]PDU, error) {
	prefix := strings.TrimPrefix(rootOID, ".") + "."
	var keys []string
	for k := range s.pdus {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	out := make([]PDU, 0, len(keys))
	for _, k := range keys {
		out = append(out, s.pdus[k])
	}
	return out, nil
}

func (s *WalkReplaySession) Close() error { return nil }

// SingleSessionDialer 测试/回放辅助拨号器。
type SingleSessionDialer struct {
	Session Session
}

func (d SingleSessionDialer) Dial(_ context.Context, _ biz.Target) (Session, error) {
	return d.Session, nil
}
