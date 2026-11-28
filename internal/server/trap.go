package server

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"

	"hoptrace/internal/biz"
	"hoptrace/internal/conf"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/gosnmp/gosnmp"
)

// 触发局部增量重扫的标准与多厂商常见 Trap OID
const (
	oidSnmpTrapOID     = "1.3.6.1.6.3.1.1.4.1.0"
	oidLinkDown        = "1.3.6.1.6.3.1.1.5.3"
	oidLinkUp          = "1.3.6.1.6.3.1.1.5.4"
	oidLldpRemChg      = "1.0.8802.1.1.2.0.0.1" // lldpRemTablesChange
	oidHwMacFlap       = "1.3.6.1.4.1.2011.5.25.42.2.1.7.1"
	oidRuijieMacChange = "1.3.6.1.4.1.4881.1.1.10.2.10.2.0.1"
	oidCiscoMacChange  = "1.3.6.1.4.1.9.9.215.2.0.1"
	oidH3CMacChange    = "1.3.6.1.4.1.25506.2.40.2.0.1"
)

// TrapServer 实现 kratos transport.Server, 监听 SNMP Trap(linkUp/linkDown/LLDP 变更等)
// 并在防抖窗口内触发来源设备的增量采集与拓扑刷新。
type TrapServer struct {
	addr     string
	debounce time.Duration
	devices  biz.DeviceRepo
	creds    biz.CredentialRepo
	poll     *biz.PollUsecase

	listener *gosnmp.TrapListener

	mu       sync.Mutex
	timers   map[string]*time.Timer
	stopped  bool
	onNotify func(ip, trapOID string) // 仅用于测试观察
}

func NewTrapServer(c *conf.Collector, devices biz.DeviceRepo, creds biz.CredentialRepo, poll *biz.PollUsecase) *TrapServer {
	deb := c.GetTrapDebounce().AsDuration()
	if deb <= 0 {
		deb = 15 * time.Second
	}
	return &TrapServer{
		addr:     strings.TrimSpace(c.GetTrapAddr()),
		debounce: deb,
		devices:  devices,
		creds:    creds,
		poll:     poll,
		timers:   map[string]*time.Timer{},
	}
}

func (s *TrapServer) Start(ctx context.Context) error {
	if s.addr == "" {
		log.Info("SNMP trap listener disabled (collector.trap_addr is empty)")
		return nil
	}
	tl := gosnmp.NewTrapListener()
	tl.OnNewTrap = s.handleTrap
	params := *gosnmp.Default
	// 若凭据库存在 SNMPv3 凭据, 自动装配 USM 参数以支持 SNMPv3 加密 Trap (同时仍兼容 v1/v2c)
	if s.creds != nil {
		if list, err := s.creds.List(ctx); err == nil {
			for _, c := range list {
				if strings.EqualFold(c.Version, "v3") && c.Username != "" {
					params.Version = gosnmp.Version3
					params.SecurityModel = gosnmp.UserSecurityModel
					usp := &gosnmp.UsmSecurityParameters{
						UserName:                 c.Username,
						AuthenticationProtocol:   trapAuthProto(c.AuthProtocol),
						AuthenticationPassphrase: c.AuthPassword,
						PrivacyProtocol:          trapPrivProto(c.PrivProtocol),
						PrivacyPassphrase:        c.PrivPassword,
					}
					switch {
					case usp.AuthenticationProtocol != gosnmp.NoAuth && usp.PrivacyProtocol != gosnmp.NoPriv:
						params.MsgFlags = gosnmp.AuthPriv
					case usp.AuthenticationProtocol != gosnmp.NoAuth:
						params.MsgFlags = gosnmp.AuthNoPriv
					default:
						params.MsgFlags = gosnmp.NoAuthNoPriv
					}
					params.SecurityParameters = usp
					log.Info("SNMP trap listener loaded v3 USM credential", "credential", c.Name, "user", c.Username)
					break
				}
			}
		}
	}
	tl.Params = &params
	s.listener = tl
	log.Info("SNMP trap listener starting", "addr", s.addr, "debounce", s.debounce)
	err := tl.Listen(s.addr)
	s.mu.Lock()
	stopped := s.stopped
	s.mu.Unlock()
	if stopped {
		return nil
	}
	return err
}

func (s *TrapServer) Stop(_ context.Context) error {
	s.mu.Lock()
	s.stopped = true
	for ip, t := range s.timers {
		t.Stop()
		delete(s.timers, ip)
	}
	s.mu.Unlock()
	if s.listener != nil {
		s.listener.Close()
	}
	return nil
}

func (s *TrapServer) handleTrap(pkt *gosnmp.SnmpPacket, addr *net.UDPAddr) {
	if pkt == nil || addr == nil {
		return
	}
	trapOID := extractTrapOID(pkt)
	if !shouldTriggerPoll(trapOID) {
		return
	}
	ip := addr.IP.String()
	log.Info("SNMP trap received, scheduling incremental poll", "ip", ip, "trap_oid", trapOID)
	s.schedule(ip, trapOID)
}

func (s *TrapServer) schedule(ip, trapOID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	if old, ok := s.timers[ip]; ok {
		old.Reset(s.debounce)
		return
	}
	s.timers[ip] = time.AfterFunc(s.debounce, func() {
		s.mu.Lock()
		delete(s.timers, ip)
		stopped := s.stopped
		s.mu.Unlock()
		if stopped {
			return
		}
		if s.onNotify != nil {
			s.onNotify(ip, trapOID)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		dev, err := s.devices.FindByIP(ctx, ip)
		if err != nil || dev == nil || !dev.Managed {
			return
		}
		if err := s.poll.PollDevice(ctx, dev.ID); err != nil {
			log.Warn("trap incremental poll failed", "ip", ip, "device", dev.ID, "err", err)
			return
		}
		if err := s.poll.RefreshWithNotice(ctx, "trap", dev.Name, "收到 SNMP Trap，已增量更新 "+dev.Name); err != nil {
			log.Warn("trap refresh failed", "ip", ip, "device", dev.ID, "err", err)
			return
		}
		log.Info("trap incremental poll completed", "ip", ip, "device", dev.Name)
	})
}

func extractTrapOID(pkt *gosnmp.SnmpPacket) string {
	for _, v := range pkt.Variables {
		name := strings.TrimPrefix(v.Name, ".")
		if name == oidSnmpTrapOID {
			switch val := v.Value.(type) {
			case string:
				return strings.TrimPrefix(val, ".")
			case []byte:
				return strings.TrimPrefix(string(val), ".")
			}
		}
	}
	// 兼容 SNMPv1 Trap GenericTrap(2=linkDown, 3=linkUp)
	switch pkt.GenericTrap {
	case 2:
		return oidLinkDown
	case 3:
		return oidLinkUp
	}
	return ""
}

func shouldTriggerPoll(trapOID string) bool {
	switch trapOID {
	case oidLinkDown, oidLinkUp, oidLldpRemChg, oidHwMacFlap, oidRuijieMacChange, oidCiscoMacChange, oidH3CMacChange:
		return true
	default:
		return strings.HasPrefix(trapOID, oidLinkDown) || strings.HasPrefix(trapOID, oidLinkUp)
	}
}

func trapAuthProto(s string) gosnmp.SnmpV3AuthProtocol {
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

func trapPrivProto(s string) gosnmp.SnmpV3PrivProtocol {
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
