package data

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"hoptrace/internal/biz"
)

// StringList 以 JSON 文本存储的字符串切片。
type StringList []string

func (s StringList) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	b, err := json.Marshal([]string(s))
	return string(b), err
}

func (s *StringList) Scan(v any) error {
	var raw []byte
	switch t := v.(type) {
	case nil:
		*s = nil
		return nil
	case string:
		raw = []byte(t)
	case []byte:
		raw = t
	default:
		return fmt.Errorf("StringList: unsupported type %T", v)
	}
	if len(raw) == 0 {
		*s = nil
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return err
	}
	*s = out
	return nil
}

// ---------- PO 模型(GORM 持久化对象,仅限 data 层使用) ----------

type CredentialPO struct {
	ID           uint   `gorm:"primaryKey"`
	Name         string `gorm:"size:100;uniqueIndex"`
	Version      string `gorm:"size:10"`
	Community    string `gorm:"size:512"`
	Username     string `gorm:"size:100"`
	AuthProtocol string `gorm:"size:20"`
	AuthPassword string `gorm:"size:512"`
	PrivProtocol string `gorm:"size:20"`
	PrivPassword string `gorm:"size:512"`
	Port         int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (CredentialPO) TableName() string { return "credentials" }

type DevicePO struct {
	ID           uint       `gorm:"primaryKey"`
	Name         string     `gorm:"size:200"`
	MgmtIP       string     `gorm:"size:64;index"`
	SNMPPort     int
	CredentialID uint       `gorm:"index"`
	Managed      bool       `gorm:"index"`
	SysName      string     `gorm:"size:200"`
	SysDescr     string     `gorm:"size:1024"`
	SysOID       string     `gorm:"size:200"`
	Vendor       string     `gorm:"size:100"`
	Family       string     `gorm:"size:30;index"`
	Model        string     `gorm:"size:100"`
	OSVersion    string     `gorm:"size:100"`
	Serial       string     `gorm:"size:100"`
	Role         string     `gorm:"size:30;index"`
	RoleManual   bool
	Status       string     `gorm:"size:20;index"`
	Site         string     `gorm:"size:100"`
	Location     string     `gorm:"size:200"`
	IPs          StringList `gorm:"type:text"`
	ChassisMAC   string     `gorm:"size:20;index"`
	CPUUsage     int
	MemUsage     int
	Temperature  int
	StackMembers int
	UptimeSecs   int64
	LastPollAt   *time.Time
	LastError    string `gorm:"size:500"`
	FailCount    int
	ArpCount     int
	PosX         float64
	PosY         float64
	HasPos       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (DevicePO) TableName() string { return "devices" }

type InterfacePO struct {
	ID             uint   `gorm:"primaryKey"`
	DeviceID       uint   `gorm:"uniqueIndex:idx_if_dev_idx"`
	IfIndex        int    `gorm:"uniqueIndex:idx_if_dev_idx"`
	Name           string `gorm:"size:100"`
	Descr          string `gorm:"size:200"`
	Alias          string `gorm:"size:200"`
	IfType         int
	SpeedMbps      int64
	AdminStatus    string `gorm:"size:10"`
	OperStatus     string `gorm:"size:10"`
	MAC            string `gorm:"size:20;index"`
	Kind           string `gorm:"size:20"`
	AggIfIndex     int
	MACCount       int
	Neighbor       string `gorm:"size:300"`
	InOctets       uint64
	OutOctets      uint64
	InBps          int64
	OutBps         int64
	UtilizationPct float64
	InErrors       int64
	OutErrors      int64
	SampledAt      *time.Time
}

func (InterfacePO) TableName() string { return "interfaces" }

type InterfaceMetricPO struct {
	ID             uint      `gorm:"primaryKey"`
	DeviceID       uint      `gorm:"index:idx_ifm_dev_idx"`
	IfIndex        int       `gorm:"index:idx_ifm_dev_idx"`
	At             time.Time `gorm:"index"`
	InBps          int64
	OutBps         int64
	UtilizationPct float64
	InErrors       int64
	OutErrors      int64
}

func (InterfaceMetricPO) TableName() string { return "interface_metrics" }

type WirelessSTAPO struct {
	ID       uint   `gorm:"primaryKey"`
	DeviceID uint   `gorm:"index"`
	MAC      string `gorm:"size:20;index"`
	IP       string `gorm:"size:64"`
	SSID     string `gorm:"size:100"`
	APName   string `gorm:"size:120"`
	RSSI     int
	VLAN     int
}

func (WirelessSTAPO) TableName() string { return "wireless_stas" }

type NeighborPO struct {
	ID            uint   `gorm:"primaryKey"`
	DeviceID      uint   `gorm:"index"`
	LocalIfIndex  int
	RemChassisID  string `gorm:"size:100"`
	RemChassisSub int
	RemPortID     string `gorm:"size:200"`
	RemPortSub    int
	RemPortDesc   string `gorm:"size:200"`
	RemSysName    string `gorm:"size:200"`
	RemSysDescr   string `gorm:"size:500"`
	RemMgmtIP     string `gorm:"size:64"`
	RemCaps       int
}

func (NeighborPO) TableName() string { return "lldp_neighbors" }

type FdbPO struct {
	ID       uint   `gorm:"primaryKey"`
	DeviceID uint   `gorm:"index:idx_fdb_dev;index:idx_fdb_mac_dev,priority:2"`
	MAC      string `gorm:"size:20;index:idx_fdb_mac_dev,priority:1"`
	VLAN     int
	IfIndex  int
}

func (FdbPO) TableName() string { return "fdb_entries" }

type ArpPO struct {
	ID       uint   `gorm:"primaryKey"`
	DeviceID uint   `gorm:"index"`
	IP       string `gorm:"size:64"`
	MAC      string `gorm:"size:20;index"`
	IfIndex  int
}

func (ArpPO) TableName() string { return "arp_entries" }

type LinkPO struct {
	ID         uint       `gorm:"primaryKey"`
	Key        string     `gorm:"size:200;uniqueIndex"`
	ADeviceID  uint       `gorm:"index"`
	AIfIndex   int
	AIfName    string     `gorm:"size:100"`
	BDeviceID  uint       `gorm:"index"`
	BIfIndex   int
	BIfName    string     `gorm:"size:100"`
	Kind       string     `gorm:"size:20"`
	Confidence float64
	Status     string     `gorm:"size:10"`
	Members    StringList `gorm:"type:text"`
	FirstSeen  time.Time
	LastSeen   time.Time
}

func (LinkPO) TableName() string { return "links" }

type EndpointPO struct {
	ID              uint       `gorm:"primaryKey"`
	MAC             string     `gorm:"size:20;uniqueIndex"`
	IP              string     `gorm:"size:64;index"`
	IPs             StringList `gorm:"type:text"`
	Vendor          string     `gorm:"size:200"`
	Type            string     `gorm:"size:20;index"`
	TypeManual      bool
	Name            string     `gorm:"size:200"`
	Owner           string     `gorm:"size:100"`
	Tags            StringList `gorm:"type:text"`
	Remark          string     `gorm:"size:500"`
	Status          string     `gorm:"size:20;index"`
	Trust           string     `gorm:"size:20;index"`
	DeviceID        uint       `gorm:"index"`
	IfIndex         int
	IfName          string     `gorm:"size:100"`
	VLAN            int
	AccessType      string     `gorm:"size:20;index"`
	SSID            string     `gorm:"size:100"`
	APName          string     `gorm:"size:120"`
	RSSI            int
	FirstSeen       time.Time
	LastSeen        time.Time
	MissCount       int
	PendingDeviceID uint
	PendingIfIndex  int
	PendingRounds   int
}

func (EndpointPO) TableName() string { return "endpoints" }

type EndpointLocationPO struct {
	ID         uint   `gorm:"primaryKey"`
	EndpointID uint   `gorm:"index"`
	DeviceID   uint
	IfIndex    int
	IfName     string `gorm:"size:100"`
	VLAN       int
	Since      time.Time
	Until      *time.Time
}

func (EndpointLocationPO) TableName() string { return "endpoint_locations" }

type EndpointEventPO struct {
	ID         uint      `gorm:"primaryKey"`
	EndpointID uint      `gorm:"index"`
	MAC        string    `gorm:"size:20"`
	IP         string    `gorm:"size:64"`
	Name       string    `gorm:"size:200"`
	Type       string    `gorm:"size:20;index"`
	Detail     string    `gorm:"size:500"`
	At         time.Time `gorm:"index"`
}

func (EndpointEventPO) TableName() string { return "endpoint_events" }

type DiscoveryJobPO struct {
	ID         uint       `gorm:"primaryKey"`
	Status     string     `gorm:"size:20"`
	Seeds      StringList `gorm:"type:text"`
	CIDRs      StringList `gorm:"type:text"`
	Probed     int
	Total      int
	Found      int
	Log        string `gorm:"type:text"`
	StartedAt  time.Time
	FinishedAt *time.Time
}

func (DiscoveryJobPO) TableName() string { return "discovery_jobs" }

// ---------- DO ↔ PO 转换函数 ----------

func deviceToPO(d *biz.Device) *DevicePO {
	return &DevicePO{
		ID: d.ID, Name: d.Name, MgmtIP: d.MgmtIP, SNMPPort: d.SNMPPort, CredentialID: d.CredentialID,
		Managed: d.Managed, SysName: d.SysName, SysDescr: d.SysDescr, SysOID: d.SysOID,
		Vendor: d.Vendor, Family: d.Family, Model: d.Model, OSVersion: d.OSVersion, Serial: d.Serial,
		Role: d.Role, RoleManual: d.RoleManual, Status: d.Status, Site: d.Site, Location: d.Location,
		IPs: StringList(d.IPs), ChassisMAC: d.ChassisMAC, CPUUsage: d.CPUUsage, MemUsage: d.MemUsage,
		Temperature: d.Temperature, StackMembers: d.StackMembers, UptimeSecs: d.UptimeSecs,
		LastPollAt: d.LastPollAt, LastError: d.LastError, FailCount: d.FailCount, ArpCount: d.ArpCount,
		PosX: d.PosX, PosY: d.PosY, HasPos: d.HasPos, CreatedAt: d.CreatedAt,
	}
}

func deviceToBiz(p *DevicePO) *biz.Device {
	return &biz.Device{
		ID: p.ID, Name: p.Name, MgmtIP: p.MgmtIP, SNMPPort: p.SNMPPort, CredentialID: p.CredentialID,
		Managed: p.Managed, SysName: p.SysName, SysDescr: p.SysDescr, SysOID: p.SysOID,
		Vendor: p.Vendor, Family: p.Family, Model: p.Model, OSVersion: p.OSVersion, Serial: p.Serial,
		Role: p.Role, RoleManual: p.RoleManual, Status: p.Status, Site: p.Site, Location: p.Location,
		IPs: []string(p.IPs), ChassisMAC: p.ChassisMAC, CPUUsage: p.CPUUsage, MemUsage: p.MemUsage,
		Temperature: p.Temperature, StackMembers: p.StackMembers, UptimeSecs: p.UptimeSecs,
		LastPollAt: p.LastPollAt, LastError: p.LastError, FailCount: p.FailCount, ArpCount: p.ArpCount,
		PosX: p.PosX, PosY: p.PosY, HasPos: p.HasPos, CreatedAt: p.CreatedAt,
	}
}

func interfaceToPO(deviceID uint, it *biz.Interface) InterfacePO {
	return InterfacePO{
		DeviceID: deviceID, IfIndex: it.IfIndex, Name: it.Name, Descr: it.Descr, Alias: it.Alias,
		IfType: it.IfType, SpeedMbps: it.SpeedMbps, AdminStatus: it.AdminStatus, OperStatus: it.OperStatus,
		MAC: it.MAC, Kind: it.Kind, AggIfIndex: it.AggIfIndex, MACCount: it.MACCount, Neighbor: it.Neighbor,
		InOctets: it.InOctets, OutOctets: it.OutOctets, InBps: it.InBps, OutBps: it.OutBps,
		UtilizationPct: it.UtilizationPct, InErrors: it.InErrors, OutErrors: it.OutErrors,
	}
}

func interfaceToBiz(p *InterfacePO) *biz.Interface {
	return &biz.Interface{
		ID: p.ID, DeviceID: p.DeviceID, IfIndex: p.IfIndex, Name: p.Name, Descr: p.Descr, Alias: p.Alias,
		IfType: p.IfType, SpeedMbps: p.SpeedMbps, AdminStatus: p.AdminStatus, OperStatus: p.OperStatus,
		MAC: p.MAC, Kind: p.Kind, AggIfIndex: p.AggIfIndex, MACCount: p.MACCount, Neighbor: p.Neighbor,
		InOctets: p.InOctets, OutOctets: p.OutOctets, InBps: p.InBps, OutBps: p.OutBps,
		UtilizationPct: p.UtilizationPct, InErrors: p.InErrors, OutErrors: p.OutErrors,
	}
}

func neighborToPO(deviceID uint, n *biz.Neighbor) NeighborPO {
	return NeighborPO{
		DeviceID: deviceID, LocalIfIndex: n.LocalIfIndex, RemChassisID: n.RemChassisID,
		RemChassisSub: n.RemChassisSub, RemPortID: n.RemPortID, RemPortSub: n.RemPortSub,
		RemPortDesc: n.RemPortDesc, RemSysName: n.RemSysName, RemSysDescr: n.RemSysDescr,
		RemMgmtIP: n.RemMgmtIP, RemCaps: n.RemCaps,
	}
}

func neighborToBiz(p *NeighborPO) *biz.Neighbor {
	return &biz.Neighbor{
		DeviceID: p.DeviceID, LocalIfIndex: p.LocalIfIndex, RemChassisID: p.RemChassisID,
		RemChassisSub: p.RemChassisSub, RemPortID: p.RemPortID, RemPortSub: p.RemPortSub,
		RemPortDesc: p.RemPortDesc, RemSysName: p.RemSysName, RemSysDescr: p.RemSysDescr,
		RemMgmtIP: p.RemMgmtIP, RemCaps: p.RemCaps,
	}
}

func linkToBiz(p *LinkPO) *biz.Link {
	return &biz.Link{
		ID: p.ID, Key: p.Key, ADeviceID: p.ADeviceID, AIfIndex: p.AIfIndex, AIfName: p.AIfName,
		BDeviceID: p.BDeviceID, BIfIndex: p.BIfIndex, BIfName: p.BIfName, Kind: p.Kind,
		Confidence: p.Confidence, Status: p.Status, Members: []string(p.Members),
		FirstSeen: p.FirstSeen, LastSeen: p.LastSeen,
	}
}

func endpointToPO(e *biz.Endpoint) *EndpointPO {
	return &EndpointPO{
		ID: e.ID, MAC: e.MAC, IP: e.IP, IPs: StringList(e.IPs), Vendor: e.Vendor,
		Type: e.Type, TypeManual: e.TypeManual, Name: e.Name, Owner: e.Owner,
		Tags: StringList(e.Tags), Remark: e.Remark, Status: e.Status, Trust: e.Trust,
		DeviceID: e.DeviceID, IfIndex: e.IfIndex, IfName: e.IfName, VLAN: e.VLAN,
		AccessType: e.AccessType, SSID: e.SSID, APName: e.APName, RSSI: e.RSSI,
		FirstSeen: e.FirstSeen, LastSeen: e.LastSeen, MissCount: e.MissCount,
		PendingDeviceID: e.PendingDeviceID, PendingIfIndex: e.PendingIfIndex, PendingRounds: e.PendingRounds,
	}
}

func endpointToBiz(p *EndpointPO) *biz.Endpoint {
	at := p.AccessType
	if at == "" {
		at = "wired"
	}
	return &biz.Endpoint{
		ID: p.ID, MAC: p.MAC, IP: p.IP, IPs: []string(p.IPs), Vendor: p.Vendor,
		Type: p.Type, TypeManual: p.TypeManual, Name: p.Name, Owner: p.Owner,
		Tags: []string(p.Tags), Remark: p.Remark, Status: p.Status, Trust: p.Trust,
		DeviceID: p.DeviceID, IfIndex: p.IfIndex, IfName: p.IfName, VLAN: p.VLAN,
		AccessType: at, SSID: p.SSID, APName: p.APName, RSSI: p.RSSI,
		FirstSeen: p.FirstSeen, LastSeen: p.LastSeen, MissCount: p.MissCount,
		PendingDeviceID: p.PendingDeviceID, PendingIfIndex: p.PendingIfIndex, PendingRounds: p.PendingRounds,
	}
}

func eventToBiz(p *EndpointEventPO) *biz.EndpointEvent {
	return &biz.EndpointEvent{
		ID: p.ID, EndpointID: p.EndpointID, MAC: p.MAC, IP: p.IP,
		Name: p.Name, Type: p.Type, Detail: p.Detail, At: p.At,
	}
}

func jobToPO(j *biz.DiscoveryJob) *DiscoveryJobPO {
	return &DiscoveryJobPO{
		ID: j.ID, Status: j.Status, Seeds: StringList(j.Seeds), CIDRs: StringList(j.CIDRs),
		Probed: j.Probed, Total: j.Total, Found: j.Found, Log: j.Log,
		StartedAt: j.StartedAt, FinishedAt: j.FinishedAt,
	}
}

func jobToBiz(p *DiscoveryJobPO) *biz.DiscoveryJob {
	return &biz.DiscoveryJob{
		ID: p.ID, Status: p.Status, Seeds: []string(p.Seeds), CIDRs: []string(p.CIDRs),
		Probed: p.Probed, Total: p.Total, Found: p.Found, Log: p.Log,
		StartedAt: p.StartedAt, FinishedAt: p.FinishedAt,
	}
}
