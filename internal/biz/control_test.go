package biz

import (
	"context"
	"strings"
	"testing"
)

func TestControlProtectionGuardrails(t *testing.T) {
	core := &Device{ID: 1, Name: "SW-Core", Role: RoleCore, Family: FamilySwitch, Managed: true}
	fw := &Device{ID: 2, Name: "FW-Edge", Role: RoleFirewall, Family: FamilyFirewall, Managed: true}
	acc := &Device{ID: 3, Name: "SW-Acc", Role: RoleAccess, Family: FamilySwitch, Managed: true}

	uplink := &Interface{DeviceID: 3, IfIndex: 24, Name: "GE0/0/24", Kind: IfKindUplink}
	edge := &Interface{DeviceID: 3, IfIndex: 1, Name: "GE0/0/1", Kind: IfKindEdge}

	if err := checkProtection(ActionShutdownPort, core, edge); err != ErrControlProtectedTarget {
		t.Fatalf("expected core switch to be protected, got %v", err)
	}
	if err := checkProtection(ActionBlockEndpoint, fw, edge); err != ErrControlProtectedTarget {
		t.Fatalf("expected firewall to be protected, got %v", err)
	}
	if err := checkProtection(ActionShutdownPort, acc, uplink); err != ErrControlProtectedTarget {
		t.Fatalf("expected uplink port to be protected, got %v", err)
	}
	if err := checkProtection(ActionShutdownPort, acc, edge); err != nil {
		t.Fatalf("expected access edge port to be allowed, got %v", err)
	}
}

func TestBuildHuaweiCommandsAndRollback(t *testing.T) {
	prev, rb, err := buildHuaweiCommand(ActionBlockEndpoint, ProtoSSH, "GE0/0/1", "f8:bc:12:11:22:01", "", "10")
	if err != nil {
		t.Fatalf("build block cmd: %v", err)
	}
	if !strings.Contains(prev, "mac-address blackhole f8bc-1211-2201 vlan 10") {
		t.Fatalf("unexpected block preview: %s", prev)
	}
	if !strings.Contains(rb, "undo mac-address blackhole f8bc-1211-2201 vlan 10") {
		t.Fatalf("unexpected block rollback: %s", rb)
	}

	ncPrev, ncRb, err := buildHuaweiCommand(ActionShutdownPort, ProtoNetconf, "GigabitEthernet0/0/1", "", "", "")
	if err != nil {
		t.Fatalf("build netconf cmd: %v", err)
	}
	if !strings.Contains(ncPrev, "<ifAdminStatus>down</ifAdminStatus>") || !strings.Contains(ncRb, "<ifAdminStatus>up</ifAdminStatus>") {
		t.Fatalf("unexpected netconf payload: prev=%s rb=%s", ncPrev, ncRb)
	}
}

var _ = context.Background
