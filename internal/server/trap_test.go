package server

import (
	"net"
	"sync/atomic"
	"testing"
	"time"

	"hoptrace/internal/conf"

	"github.com/gosnmp/gosnmp"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestTrapDebounceAndOIDMatch(t *testing.T) {
	srv := NewTrapServer(&conf.Collector{
		TrapAddr:     "127.0.0.1:0",
		TrapDebounce: durationpb.New(40 * time.Millisecond),
	}, nil, nil, nil)

	var calls atomic.Int32
	srv.onNotify = func(ip, trapOID string) {
		if ip != "10.255.0.11" || trapOID != oidLinkUp {
			t.Errorf("unexpected notify: %s %s", ip, trapOID)
		}
		calls.Add(1)
	}

	addr := &net.UDPAddr{IP: net.ParseIP("10.255.0.11"), Port: 54321}
	pkt := &gosnmp.SnmpPacket{
		Variables: []gosnmp.SnmpPDU{
			{Name: "." + oidSnmpTrapOID, Type: gosnmp.ObjectIdentifier, Value: "." + oidLinkUp},
		},
	}

	// 短时间内连续收到 3 次同一设备的 linkUp Trap, 应合并为 1 次增量重扫
	srv.handleTrap(pkt, addr)
	srv.handleTrap(pkt, addr)
	srv.handleTrap(pkt, addr)

	// 无关 Trap 应被忽略
	srv.handleTrap(&gosnmp.SnmpPacket{
		Variables: []gosnmp.SnmpPDU{
			{Name: "." + oidSnmpTrapOID, Type: gosnmp.ObjectIdentifier, Value: ".1.3.6.1.6.3.1.1.5.1"},
		},
	}, addr)

	time.Sleep(100 * time.Millisecond)
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 debounced poll trigger, got %d", got)
	}
}

func TestParseSyslogPacket(t *testing.T) {
	sev, fac, tag, msg := ParseSyslogPacket("<188>Nov 27 18:00:00 hx-01 %%01IFNET/4/LINK_STATE(l)[12]:Interface GE0/0/1 turned DOWN.")
	if sev != 4 || fac != "local7" || tag != "%%01IFNET/4/LINK_STATE(l)[12]" || msg == "" {
		t.Fatalf("unexpected syslog parse: sev=%d fac=%s tag=%s msg=%s", sev, fac, tag, msg)
	}
}
