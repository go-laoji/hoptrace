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
	}, nil, nil)

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
