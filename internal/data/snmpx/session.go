package snmpx

import (
	"context"
	"fmt"
	"strings"
	"time"

	"hoptrace/internal/biz"

	"github.com/gosnmp/gosnmp"
)

// PDU 表示一条 SNMP 返回的变量绑定。
type PDU struct {
	OID   string // 规范化为无前导点,例如 1.3.6.1.2.1.1.1.0
	Type  gosnmp.Asn1BER
	Value any
}

// Session 单设备 SNMP 会话抽象(便于测试与模拟器复用同一套解析代码)。
type Session interface {
	Get(ctx context.Context, oids []string) ([]PDU, error)
	Walk(ctx context.Context, rootOID string) ([]PDU, error)
	Close() error
}

// Dialer 根据 Target 建立 Session。
type Dialer interface {
	Dial(ctx context.Context, t biz.Target) (Session, error)
}

// GoSNMPDialer 基于 gosnmp 的真实网络采集拨号器。
type GoSNMPDialer struct {
	Timeout        time.Duration
	Retries        int
	MaxRepetitions uint32
}

func (d *GoSNMPDialer) Dial(_ context.Context, t biz.Target) (Session, error) {
	if t.Credential == nil {
		return nil, fmt.Errorf("missing credential")
	}
	port := t.Port
	if port <= 0 {
		port = t.Credential.Port
	}
	if port <= 0 {
		port = 161
	}
	timeout := d.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	retries := d.Retries
	if retries <= 0 {
		retries = 1
	}
	maxRep := d.MaxRepetitions
	if maxRep == 0 {
		maxRep = 25
	}

	g := &gosnmp.GoSNMP{
		Target:         t.IP,
		Port:           uint16(port),
		Timeout:        timeout,
		Retries:        retries,
		MaxRepetitions: maxRep,
		ExponentialTimeout: true,
	}

	switch strings.ToLower(t.Credential.Version) {
	case "v3":
		g.Version = gosnmp.Version3
		g.SecurityModel = gosnmp.UserSecurityModel
		usp := &gosnmp.UsmSecurityParameters{
			UserName:                 t.Credential.Username,
			AuthenticationProtocol:   parseAuthProto(t.Credential.AuthProtocol),
			AuthenticationPassphrase: t.Credential.AuthPassword,
			PrivacyProtocol:          parsePrivProto(t.Credential.PrivProtocol),
			PrivacyPassphrase:        t.Credential.PrivPassword,
		}
		switch {
		case usp.AuthenticationProtocol != gosnmp.NoAuth && usp.PrivacyProtocol != gosnmp.NoPriv:
			g.MsgFlags = gosnmp.AuthPriv
		case usp.AuthenticationProtocol != gosnmp.NoAuth:
			g.MsgFlags = gosnmp.AuthNoPriv
		default:
			g.MsgFlags = gosnmp.NoAuthNoPriv
		}
		g.SecurityParameters = usp
	default:
		g.Version = gosnmp.Version2c
		g.Community = t.Credential.Community
	}

	if err := g.Connect(); err != nil {
		return nil, err
	}
	return &goSNMPSession{g: g}, nil
}

type goSNMPSession struct {
	g *gosnmp.GoSNMP
}

func (s *goSNMPSession) Get(ctx context.Context, oids []string) ([]PDU, error) {
	s.g.Context = ctx
	pkt, err := s.g.Get(oids)
	if err != nil {
		return nil, err
	}
	out := make([]PDU, 0, len(pkt.Variables))
	for _, v := range pkt.Variables {
		if v.Type == gosnmp.NoSuchObject || v.Type == gosnmp.NoSuchInstance || v.Type == gosnmp.EndOfMibView {
			continue
		}
		out = append(out, PDU{OID: strings.TrimPrefix(v.Name, "."), Type: v.Type, Value: v.Value})
	}
	return out, nil
}

func (s *goSNMPSession) Walk(ctx context.Context, rootOID string) ([]PDU, error) {
	s.g.Context = ctx
	root := strings.TrimPrefix(rootOID, ".")
	var out []PDU
	err := s.g.BulkWalk(root, func(v gosnmp.SnmpPDU) error {
		if v.Type == gosnmp.NoSuchObject || v.Type == gosnmp.NoSuchInstance || v.Type == gosnmp.EndOfMibView {
			return nil
		}
		out = append(out, PDU{OID: strings.TrimPrefix(v.Name, "."), Type: v.Type, Value: v.Value})
		return nil
	})
	return out, err
}

func (s *goSNMPSession) Close() error {
	if s.g.Conn != nil {
		return s.g.Conn.Close()
	}
	return nil
}

func parseAuthProto(s string) gosnmp.SnmpV3AuthProtocol {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "MD5":
		return gosnmp.MD5
	case "SHA", "SHA1":
		return gosnmp.SHA
	case "SHA224":
		return gosnmp.SHA224
	case "SHA256":
		return gosnmp.SHA256
	case "SHA384":
		return gosnmp.SHA384
	case "SHA512":
		return gosnmp.SHA512
	default:
		return gosnmp.NoAuth
	}
}

func parsePrivProto(s string) gosnmp.SnmpV3PrivProtocol {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "DES":
		return gosnmp.DES
	case "AES", "AES128":
		return gosnmp.AES
	case "AES192":
		return gosnmp.AES192
	case "AES192C", "AES-192-C":
		return gosnmp.AES192C
	case "AES256":
		return gosnmp.AES256
	case "AES256C", "AES-256-C":
		return gosnmp.AES256C
	default:
		return gosnmp.NoPriv
	}
}

// ---------- PDU 解码辅助 ----------

func pduString(p PDU) string {
	switch v := p.Value.(type) {
	case []byte:
		return strings.TrimSpace(string(v))
	case string:
		return strings.TrimSpace(v)
	default:
		return ""
	}
}

func pduBytes(p PDU) []byte {
	switch v := p.Value.(type) {
	case []byte:
		return v
	case string:
		return []byte(v)
	default:
		return nil
	}
}

func pduInt64(p PDU) int64 {
	return gosnmp.ToBigInt(p.Value).Int64()
}

func pduOID(p PDU) string {
	switch v := p.Value.(type) {
	case string:
		return strings.TrimPrefix(v, ".")
	case []byte:
		return strings.TrimPrefix(string(v), ".")
	default:
		return ""
	}
}
