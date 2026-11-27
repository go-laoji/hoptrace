package server

import (
	"context"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"hoptrace/internal/biz"
	"hoptrace/internal/conf"

	"github.com/go-kratos/kratos/v3/log"
)

// SyslogServer 实现 kratos transport.Server, 监听 UDP Syslog (默认 0.0.0.0:1514),
// 解析 RFC3164/RFC5424 华为交换机日志并入库, 高危安全/环路事件自动升级告警。
type SyslogServer struct {
	addr    string
	devices biz.DeviceRepo
	alerts  *biz.AlertUsecase

	mu      sync.Mutex
	conn    *net.UDPConn
	stopped bool
}

func NewSyslogServer(c *conf.Collector, devices biz.DeviceRepo, alerts *biz.AlertUsecase) *SyslogServer {
	return &SyslogServer{
		addr:    strings.TrimSpace(c.GetSyslogAddr()),
		devices: devices,
		alerts:  alerts,
	}
}

func (s *SyslogServer) Start(_ context.Context) error {
	if s.addr == "" {
		log.Info("Syslog listener disabled (collector.syslog_addr is empty)")
		return nil
	}
	udpAddr, err := net.ResolveUDPAddr("udp", s.addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()
	log.Info("Syslog UDP listener started", "addr", s.addr)

	buf := make([]byte, 8192)
	for {
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			s.mu.Lock()
			stopped := s.stopped
			s.mu.Unlock()
			if stopped {
				return nil
			}
			log.Warn("syslog read error", "err", err)
			continue
		}
		raw := strings.TrimSpace(string(buf[:n]))
		if raw == "" || remote == nil {
			continue
		}
		s.handleMessage(remote.IP.String(), raw)
	}
}

func (s *SyslogServer) Stop(_ context.Context) error {
	s.mu.Lock()
	s.stopped = true
	conn := s.conn
	s.mu.Unlock()
	if conn != nil {
		return conn.Close()
	}
	return nil
}

func (s *SyslogServer) handleMessage(sourceIP, raw string) {
	sev, fac, tag, msg := ParseSyslogPacket(raw)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var devID uint
	devName := ""
	if s.devices != nil {
		if d, err := s.devices.FindByIP(ctx, sourceIP); err == nil && d != nil {
			devID, devName = d.ID, d.Name
		}
	}
	entry := &biz.SyslogEntry{
		SourceIP: sourceIP, DeviceID: devID, DeviceName: devName,
		Severity: sev, Facility: fac, Tag: tag, Message: msg, ReceivedAt: time.Now(),
	}
	if err := s.alerts.RecordSyslog(ctx, entry); err != nil {
		log.Warn("record syslog failed", "ip", sourceIP, "err", err)
	}
}

var syslogFacilities = []string{
	"kern", "user", "mail", "daemon", "auth", "syslog", "lpr", "news",
	"uucp", "cron", "authpriv", "ftp", "ntp", "audit", "alert", "clock",
	"local0", "local1", "local2", "local3", "local4", "local5", "local6", "local7",
}

// ParseSyslogPacket 解析 RFC3164/华为 Syslog 报文中的 PRI/Facility/Severity/Tag/Message。
func ParseSyslogPacket(raw string) (severity int, facility, tag, message string) {
	severity = 6 // info
	facility = "local7"
	rest := raw
	if strings.HasPrefix(rest, "<") {
		if end := strings.IndexByte(rest, '>'); end > 1 && end <= 4 {
			if pri, err := strconv.Atoi(rest[1:end]); err == nil && pri >= 0 {
				severity = pri & 7
				facIdx := pri >> 3
				if facIdx >= 0 && facIdx < len(syslogFacilities) {
					facility = syslogFacilities[facIdx]
				}
				rest = strings.TrimSpace(rest[end+1:])
			}
		}
	}
	// 华为常见格式: ... %%01IFNET/4/LINK_STATE(l)[12]: ...
	if idx := strings.Index(rest, "%%"); idx >= 0 {
		sub := rest[idx:]
		if colon := strings.Index(sub, ":"); colon > 0 && colon < 80 {
			tag = strings.TrimSpace(sub[:colon])
			message = strings.TrimSpace(sub[colon+1:])
			return
		}
	}
	message = rest
	return
}
