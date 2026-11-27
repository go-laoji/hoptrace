package biz

import "time"

// 说明:本文件中的类型均为领域对象(DO),不含 proto 与存储标签。

// 设备状态
const (
	StatusOnline  = "online"
	StatusOffline = "offline"
	StatusUnknown = "unknown"
)

// 设备家族
const (
	FamilySwitch   = "switch"
	FamilyFirewall = "firewall"
	FamilyRouter   = "router"
	FamilyWLANAC   = "wlan-ac"
	FamilyUnknown  = "unknown"
)

// 设备角色
const (
	RoleCore        = "core"
	RoleAggregation = "aggregation"
	RoleAccess      = "access"
	RoleFirewall    = "firewall"
	RoleRouter      = "router"
	RoleAC          = "ac"
	RoleUnknown     = "unknown"
)

// 接口类别
const (
	IfKindEdge      = "edge"       // 接入口,其上学到的 MAC 视为终端
	IfKindUplink    = "uplink"     // 与网络设备互联
	IfKindSuspect   = "suspect"    // 无邻居但 MAC 过多,疑似私接 HUB/小交换机
	IfKindLAG       = "lag"        // 聚合口(未解析到互联)
	IfKindLAGMember = "lag_member" // 聚合成员口
)

// 链路状态
const (
	LinkUp    = "up"
	LinkDown  = "down"
	LinkStale = "stale"
)

// 终端状态/信任状态/类型
const (
	EndpointOnline  = "online"
	EndpointOffline = "offline"

	TrustUnknown = "unknown"
	TrustTrusted = "trusted"
	TrustBlocked = "blocked"

	TypePC      = "pc"
	TypePhone   = "phone"
	TypeAP      = "ap"
	TypeCamera  = "camera"
	TypePrinter = "printer"
	TypeServer  = "server"
	TypeMobile  = "mobile"
	TypeIoT     = "iot"
	TypeUnknown = "unknown"
)

// 终端事件类型
const (
	EventNew       = "new"
	EventOnline    = "online"
	EventOffline   = "offline"
	EventMoved     = "moved"
	EventIPChanged = "ip_changed"
)

// 发现任务状态
const (
	JobRunning = "running"
	JobDone    = "done"
	JobFailed  = "failed"
)

// Credential SNMP 凭据。敏感字段在 data 层加密落库,biz 层始终使用明文。
type Credential struct {
	ID           uint
	Name         string
	Version      string // v2c / v3
	Community    string
	Username     string
	AuthProtocol string
	AuthPassword string
	PrivProtocol string
	PrivPassword string
	Port         int
	CreatedAt    time.Time
}

// Device 网络设备。Managed=false 表示仅通过 LLDP 发现的占位设备。
type Device struct {
	ID           uint
	Name         string
	MgmtIP       string
	SNMPPort     int
	CredentialID uint
	Managed      bool
	SysName      string
	SysDescr     string
	SysOID       string
	Vendor       string
	Family       string
	Model        string
	OSVersion    string
	Serial       string
	Role         string
	RoleManual   bool
	Status       string
	Site         string
	Location     string
	IPs          []string
	ChassisMAC   string
	CPUUsage     int
	MemUsage     int
	Temperature  int
	StackMembers int
	UptimeSecs   int64
	LastPollAt   *time.Time
	LastError    string
	FailCount    int
	ArpCount     int
	PosX         float64
	PosY         float64
	HasPos       bool
	CreatedAt    time.Time

	// 查询时填充的统计字段
	InterfaceCount int
	EndpointCount  int
}

// Interface 设备接口(仅保留物理口与聚合口)。
type Interface struct {
	ID             uint
	DeviceID       uint
	IfIndex        int
	Name           string
	Descr          string
	Alias          string
	IfType         int
	SpeedMbps      int64
	AdminStatus    string
	OperStatus     string
	MAC            string
	Kind           string
	AggIfIndex     int
	MACCount       int
	Neighbor       string
	InOctets       uint64
	OutOctets      uint64
	InBps          int64
	OutBps         int64
	UtilizationPct float64
	InErrors       int64
	OutErrors      int64
}

// InterfaceMetricPoint 接口历史速率与错包采样点。
type InterfaceMetricPoint struct {
	At             time.Time
	InBps          int64
	OutBps         int64
	UtilizationPct float64
	InErrors       int64
	OutErrors      int64
}

// WirelessSTA 华为无线控制器(AC)采集到的无线终端关联信息。
type WirelessSTA struct {
	DeviceID uint
	MAC      string
	IP       string
	SSID     string
	APName   string
	RSSI     int
	VLAN     int
}

// Neighbor 设备上报的 LLDP 邻居。
type Neighbor struct {
	DeviceID      uint
	LocalIfIndex  int
	RemChassisID  string
	RemChassisSub int
	RemPortID     string
	RemPortSub    int
	RemPortDesc   string
	RemSysName    string
	RemSysDescr   string
	RemMgmtIP     string
	RemCaps       int
}

// FdbEntry 设备 MAC 地址表条目。
type FdbEntry struct {
	DeviceID uint
	MAC      string
	VLAN     int
	IfIndex  int
}

// ArpEntry 设备 ARP 表条目。
type ArpEntry struct {
	DeviceID uint
	IP       string
	MAC      string
	IfIndex  int
}

// Link 设备间逻辑链路(聚合链路合并为一条,成员见 Members)。
type Link struct {
	ID         uint
	Key        string
	ADeviceID  uint
	AIfIndex   int
	AIfName    string
	BDeviceID  uint
	BIfIndex   int
	BIfName    string
	Kind       string
	Confidence float64
	Status     string
	Members    []string
	FirstSeen  time.Time
	LastSeen   time.Time
}

// Endpoint 接入终端。
type Endpoint struct {
	ID         uint
	MAC        string
	IP         string
	IPs        []string
	Vendor     string
	Type       string
	TypeManual bool
	Name       string
	Owner      string
	Tags       []string
	Remark     string
	Status     string
	Trust      string
	DeviceID   uint
	IfIndex    int
	IfName     string
	VLAN       int
	AccessType string // wired / wireless
	SSID       string
	APName     string
	RSSI       int
	FirstSeen  time.Time
	LastSeen   time.Time
	MissCount  int

	// 漂移防抖
	PendingDeviceID uint
	PendingIfIndex  int
	PendingRounds   int

	DeviceName string
}

// EndpointLocation 终端位置历史。Until 为空表示当前位置。
type EndpointLocation struct {
	ID         uint
	EndpointID uint
	DeviceID   uint
	DeviceName string
	IfIndex    int
	IfName     string
	VLAN       int
	Since      time.Time
	Until      *time.Time
}

// EndpointEvent 终端事件。
type EndpointEvent struct {
	ID         uint
	EndpointID uint
	MAC        string
	IP         string
	Name       string
	Type       string
	Detail     string
	At         time.Time
}

// EndpointChange 一次终端同步产生的单个终端变更。
type EndpointChange struct {
	Endpoint        *Endpoint
	IsNew           bool
	LocationChanged bool
	Events          []*EndpointEvent
}

// DiscoveryJob 发现任务。
type DiscoveryJob struct {
	ID         uint
	Status     string
	Seeds      []string
	CIDRs      []string
	Probed     int
	Total      int
	Found      int
	Log        string
	StartedAt  time.Time
	FinishedAt *time.Time
}

// SysInfo SNMPv2-MIB 系统信息。
type SysInfo struct {
	Descr      string
	ObjectID   string
	Name       string
	Location   string
	UptimeSecs int64
}

// Snapshot 一次完整采集的结果。
type Snapshot struct {
	Sys          SysInfo
	Interfaces   []*Interface
	Neighbors    []*Neighbor
	Fdb          []FdbEntry
	Arp          []ArpEntry
	Wireless     []WirelessSTA
	IPs          []string
	ChassisMAC   string
	Serial       string
	StackMembers int
	CPU          int
	Mem          int
	Temp         int
	Warnings     []string
}

// IfKey 接口唯一标识。
type IfKey struct {
	DeviceID uint
	IfIndex  int
}

// TopoNode / TopoLink / Topology 拓扑视图。
type TopoNode struct {
	ID            string
	Kind          string // device / endpoint
	RefID         uint
	Name          string
	IP            string
	Role          string
	Family        string
	Status        string
	Vendor        string
	Model         string
	Type          string
	Managed       bool
	EndpointCount int
	PosX          float64
	PosY          float64
	HasPos        bool
}

type TopoLink struct {
	ID             string
	Source         string
	Target         string
	SourcePort     string
	TargetPort     string
	Kind           string
	Confidence     float64
	Status         string
	Members        []string
	InBps          int64
	OutBps         int64
	UtilizationPct float64
}

type Topology struct {
	Nodes []*TopoNode
	Links []*TopoLink
}

type PathHop struct {
	NodeID  string
	Kind    string
	Name    string
	IP      string
	Role    string
	InPort  string
	OutPort string
	LinkID  string
}

type TraceResult struct {
	Found   bool
	Message string
	Hops    []PathHop
	NodeIDs []string
	LinkIDs []string
}

type NodePos struct {
	DeviceID uint
	X        float64
	Y        float64
}

type EndpointImportRow struct {
	MAC    string
	Name   string
	Owner  string
	Tags   []string
	Trust  string
	Type   string
	Remark string
}

type BatchImportResult struct {
	Updated int
	Skipped int
	Errors  []string
}

// Stats 仪表盘统计。
type Stats struct {
	DevicesTotal      int64
	DevicesOnline     int64
	DevicesOffline    int64
	DevicesUnmanaged  int64
	LinksTotal        int64
	LinksDown         int64
	LinksStale        int64
	EndpointsTotal    int64
	EndpointsOnline   int64
	EndpointsNew24h   int64
	EndpointsUntrust  int64
	DevicesByRole     []KV
	EndpointsByType   []KV
	RecentEvents      []*EndpointEvent
}

// KV 名称-数量。
type KV struct {
	Name  string
	Value int64
}
