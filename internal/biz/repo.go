package biz

import (
	"context"
	"time"
)

// Target SNMP 采集目标。
type Target struct {
	IP         string
	Port       int
	Credential *Credential
}

// Collector 设备采集器(SNMP 实现位于 data 层)。
type Collector interface {
	// Probe 轻量探测:仅读取 sysDescr/sysObjectID/sysName,用于发现与凭据试探。
	Probe(ctx context.Context, t Target) (*SysInfo, error)
	// Collect 完整采集:系统/接口/LLDP/FDB/ARP/LAG/健康。
	Collect(ctx context.Context, t Target) (*Snapshot, error)
}

// CredentialRepo 凭据仓储。
type CredentialRepo interface {
	List(ctx context.Context) ([]*Credential, error)
	Get(ctx context.Context, id uint) (*Credential, error)
	Save(ctx context.Context, c *Credential) (*Credential, error)
	Delete(ctx context.Context, id uint) error
	CountDevices(ctx context.Context, id uint) (int64, error)
}

// DeviceFilter 设备查询过滤条件。
type DeviceFilter struct {
	Keyword string
	Status  string
	Family  string
	Role    string
}

// DeviceRepo 设备与接口仓储。
type DeviceRepo interface {
	List(ctx context.Context, f DeviceFilter, page, size int) ([]*Device, int64, error)
	ListAll(ctx context.Context) ([]*Device, error)
	Get(ctx context.Context, id uint) (*Device, error)
	FindByIP(ctx context.Context, ip string) (*Device, error)
	Create(ctx context.Context, d *Device) (*Device, error)
	Update(ctx context.Context, d *Device) error
	Delete(ctx context.Context, id uint) error
	ListInterfaces(ctx context.Context, deviceID uint) ([]*Interface, error)
	ListAllInterfaces(ctx context.Context) ([]*Interface, error)
	ListInterfaceMetrics(ctx context.Context, deviceID uint, ifIndex, limit int) ([]InterfaceMetricPoint, error)
	SaveLayout(ctx context.Context, positions []NodePos) error
	ResetLayout(ctx context.Context) error
	// SaveSnapshot 在一个事务中更新设备字段并替换接口/邻居/FDB/ARP/无线快照, 并计算速率采样。
	SaveSnapshot(ctx context.Context, d *Device, s *Snapshot) error
	// RecordFailure 记录采集失败;连续失败达到 offlineAfter 次后标记设备离线。
	RecordFailure(ctx context.Context, id uint, msg string, offlineAfter int) error
	ListNeighbors(ctx context.Context, deviceID uint) ([]*Neighbor, error)
}

// TopologyRepo 拓扑相关仓储。
type TopologyRepo interface {
	ListAllNeighbors(ctx context.Context) ([]*Neighbor, error)
	PortMACCounts(ctx context.Context) (map[IfKey]int, error)
	ArpCounts(ctx context.Context) (map[uint]int, error)
	CreateStubs(ctx context.Context, stubs []*Device) error
	DeleteDevices(ctx context.Context, ids []uint) error
	// SaveTopology 事务内 upsert 链路并回写接口类别与设备角色;未出现的旧链路标记为 stale。
	SaveTopology(ctx context.Context, r *TopoResult, now time.Time) error
	ListLinks(ctx context.Context) ([]*Link, error)
	// PurgeStaleLinks 删除超过 before 未见的 stale 链路。
	PurgeStaleLinks(ctx context.Context, before time.Time) error
}

// EndpointFilter 终端查询过滤条件。
type EndpointFilter struct {
	Keyword    string
	Status     string
	Trust      string
	Type       string
	DeviceID   uint
	AccessType string
}

// EventFilter 事件查询过滤条件。
type EventFilter struct {
	EndpointID uint
	Type       string
}

// EndpointRepo 终端仓储。
type EndpointRepo interface {
	List(ctx context.Context, f EndpointFilter, page, size int) ([]*Endpoint, int64, error)
	ListAll(ctx context.Context) ([]*Endpoint, error)
	ListByDevices(ctx context.Context, deviceIDs []uint, onlineOnly bool) ([]*Endpoint, error)
	Get(ctx context.Context, id uint) (*Endpoint, error)
	FindByMAC(ctx context.Context, mac string) (*Endpoint, error)
	Update(ctx context.Context, e *Endpoint) error
	Delete(ctx context.Context, id uint) error
	CountByDevice(ctx context.Context) (map[uint]int, error)
	ListFdb(ctx context.Context) ([]FdbEntry, error)
	ListArp(ctx context.Context) ([]ArpEntry, error)
	ListWireless(ctx context.Context) ([]WirelessSTA, error)
	// ApplyChanges 事务内落库终端变更、位置历史与事件。
	ApplyChanges(ctx context.Context, changes []*EndpointChange, now time.Time) error
	Locations(ctx context.Context, endpointID uint) ([]*EndpointLocation, error)
	Events(ctx context.Context, f EventFilter, page, size int) ([]*EndpointEvent, int64, error)
}

// JobRepo 发现任务仓储。
type JobRepo interface {
	Create(ctx context.Context, j *DiscoveryJob) (*DiscoveryJob, error)
	Update(ctx context.Context, j *DiscoveryJob) error
	Get(ctx context.Context, id uint) (*DiscoveryJob, error)
	List(ctx context.Context, limit int) ([]*DiscoveryJob, error)
	// FailRunning 启动时将遗留 running 状态的任务标记为失败。
	FailRunning(ctx context.Context) error
}

// StatsRepo 统计仓储。
type StatsRepo interface {
	Stats(ctx context.Context, now time.Time) (*Stats, error)
}
