package biz

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v3/log"
)

// Options biz 层运行参数(由 conf 转换)。
type Options struct {
	PollInterval       time.Duration
	Concurrency        int
	SuspectMACThreshold int
	OfflineMissRounds  int
	MoveConfirmRounds  int
	DisablePoller      bool
}

func (o Options) withDefaults() Options {
	if o.PollInterval <= 0 {
		o.PollInterval = 5 * time.Minute
	}
	if o.Concurrency <= 0 {
		o.Concurrency = 20
	}
	if o.SuspectMACThreshold <= 0 {
		o.SuspectMACThreshold = 30
	}
	if o.OfflineMissRounds <= 0 {
		o.OfflineMissRounds = 2
	}
	if o.MoveConfirmRounds <= 0 {
		o.MoveConfirmRounds = 2
	}
	return o
}

// ---------- 凭据 ----------

// CredentialUsecase 凭据管理。
type CredentialUsecase struct {
	repo CredentialRepo
}

func NewCredentialUsecase(repo CredentialRepo) *CredentialUsecase {
	return &CredentialUsecase{repo: repo}
}

func (uc *CredentialUsecase) List(ctx context.Context) ([]*Credential, error) {
	return uc.repo.List(ctx)
}

// Save 创建或更新凭据;更新时密文字段为空表示保持不变。
func (uc *CredentialUsecase) Save(ctx context.Context, c *Credential) (*Credential, error) {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return nil, ErrCredentialInvalid
	}
	switch c.Version {
	case "v2c":
		if c.ID == 0 && c.Community == "" {
			return nil, ErrCredentialInvalid
		}
	case "v3":
		if c.Username == "" {
			return nil, ErrCredentialInvalid
		}
	default:
		return nil, ErrCredentialInvalid
	}
	if c.Port == 0 {
		c.Port = 161
	}
	if c.ID != 0 {
		old, err := uc.repo.Get(ctx, c.ID)
		if err != nil {
			return nil, err
		}
		if c.Community == "" {
			c.Community = old.Community
		}
		if c.AuthPassword == "" {
			c.AuthPassword = old.AuthPassword
		}
		if c.PrivPassword == "" {
			c.PrivPassword = old.PrivPassword
		}
		c.CreatedAt = old.CreatedAt
	}
	return uc.repo.Save(ctx, c)
}

func (uc *CredentialUsecase) Delete(ctx context.Context, id uint) error {
	n, err := uc.repo.CountDevices(ctx, id)
	if err != nil {
		return err
	}
	if n > 0 {
		return ErrCredentialInUse
	}
	return uc.repo.Delete(ctx, id)
}

// ---------- 设备 ----------

// DeviceUsecase 设备管理。
type DeviceUsecase struct {
	repo  DeviceRepo
	creds CredentialRepo
	poll  *PollUsecase
}

func NewDeviceUsecase(repo DeviceRepo, creds CredentialRepo, poll *PollUsecase) *DeviceUsecase {
	return &DeviceUsecase{repo: repo, creds: creds, poll: poll}
}

func (uc *DeviceUsecase) List(ctx context.Context, f DeviceFilter, page, size int) ([]*Device, int64, error) {
	return uc.repo.List(ctx, f, page, size)
}

func (uc *DeviceUsecase) Get(ctx context.Context, id uint) (*Device, error) {
	return uc.repo.Get(ctx, id)
}

// Create 手工添加设备并立即采集一次。
func (uc *DeviceUsecase) Create(ctx context.Context, d *Device) (*Device, error) {
	d.MgmtIP = strings.TrimSpace(d.MgmtIP)
	if d.MgmtIP == "" || d.CredentialID == 0 {
		return nil, ErrDeviceInvalidArgument
	}
	if _, err := uc.creds.Get(ctx, d.CredentialID); err != nil {
		return nil, err
	}
	if old, err := uc.repo.FindByIP(ctx, d.MgmtIP); err != nil {
		return nil, err
	} else if old != nil && old.Managed {
		return nil, ErrDeviceAlreadyExists
	} else if old != nil { // 占位设备升级为纳管设备
		old.Managed, old.CredentialID, old.SNMPPort = true, d.CredentialID, d.SNMPPort
		if d.Name != "" {
			old.Name = d.Name
		}
		d = old
		if err := uc.repo.Update(ctx, d); err != nil {
			return nil, err
		}
		return uc.pollAfterSave(ctx, d.ID)
	}
	d.Managed = true
	d.Status = StatusUnknown
	d.Family = FamilyUnknown
	d.Role = RoleUnknown
	if d.Name == "" {
		d.Name = d.MgmtIP
	}
	created, err := uc.repo.Create(ctx, d)
	if err != nil {
		return nil, err
	}
	return uc.pollAfterSave(ctx, created.ID)
}

func (uc *DeviceUsecase) pollAfterSave(ctx context.Context, id uint) (*Device, error) {
	// 采集失败不影响设备创建,错误信息记录在设备上
	if err := uc.poll.PollAndRefresh(ctx, id); err != nil {
		log.Warn("initial poll failed", "device", id, "err", err)
	}
	return uc.repo.Get(ctx, id)
}

// Update 更新名称/凭据/角色/站点。role 为空表示恢复自动判定。
func (uc *DeviceUsecase) Update(ctx context.Context, id uint, name string, credentialID uint, role, site, location string) (*Device, error) {
	d, err := uc.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if name != "" {
		d.Name = name
	}
	if credentialID != 0 && credentialID != d.CredentialID {
		if _, err := uc.creds.Get(ctx, credentialID); err != nil {
			return nil, err
		}
		d.CredentialID = credentialID
	}
	d.Site, d.Location = site, location
	if role != "" {
		d.Role, d.RoleManual = role, true
	} else {
		d.RoleManual = false
	}
	if err := uc.repo.Update(ctx, d); err != nil {
		return nil, err
	}
	if !d.RoleManual {
		go uc.poll.Refresh(context.Background())
	}
	return uc.repo.Get(ctx, id)
}

func (uc *DeviceUsecase) Delete(ctx context.Context, id uint) error {
	if _, err := uc.repo.Get(ctx, id); err != nil {
		return err
	}
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}
	return uc.poll.Refresh(ctx)
}

// Poll 立即采集并刷新派生数据。
func (uc *DeviceUsecase) Poll(ctx context.Context, id uint) (*Device, error) {
	if _, err := uc.repo.Get(ctx, id); err != nil {
		return nil, err
	}
	if err := uc.poll.PollAndRefresh(ctx, id); err != nil {
		return nil, err
	}
	return uc.repo.Get(ctx, id)
}

func (uc *DeviceUsecase) Interfaces(ctx context.Context, id uint) ([]*Interface, error) {
	return uc.repo.ListInterfaces(ctx, id)
}

func (uc *DeviceUsecase) InterfaceMetrics(ctx context.Context, deviceID uint, ifIndex, limit int) ([]InterfaceMetricPoint, error) {
	if limit <= 0 {
		limit = 30
	}
	return uc.repo.ListInterfaceMetrics(ctx, deviceID, ifIndex, limit)
}

// ---------- 采集编排 ----------

// PollUsecase 设备采集编排:单设备采集、到期轮询、拓扑与终端刷新。
type PollUsecase struct {
	devices   DeviceRepo
	creds     CredentialRepo
	collector Collector
	topo      *TopologyUsecase
	endpoints *EndpointUsecase
	alerts    *AlertUsecase
	bus       *EventBus
	opts      Options

	mu       sync.Mutex
	inflight map[uint]struct{}
	refresh  sync.Mutex
}

func NewPollUsecase(devices DeviceRepo, creds CredentialRepo, c Collector, topo *TopologyUsecase, eps *EndpointUsecase, alerts *AlertUsecase, bus *EventBus, opts Options) *PollUsecase {
	return &PollUsecase{
		devices: devices, creds: creds, collector: c, topo: topo, endpoints: eps, alerts: alerts, bus: bus,
		opts: opts.withDefaults(), inflight: map[uint]struct{}{},
	}
}

func (uc *PollUsecase) Options() Options { return uc.opts }

func (uc *PollUsecase) acquire(id uint) bool {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	if _, ok := uc.inflight[id]; ok {
		return false
	}
	uc.inflight[id] = struct{}{}
	return true
}

func (uc *PollUsecase) release(id uint) {
	uc.mu.Lock()
	delete(uc.inflight, id)
	uc.mu.Unlock()
}

// PollDevice 采集单台设备并保存快照(不触发拓扑/终端刷新)。同一设备同一时刻只允许一个采集。
func (uc *PollUsecase) PollDevice(ctx context.Context, id uint) error {
	if !uc.acquire(id) {
		return ErrCollectFailed("device is being polled")
	}
	defer uc.release(id)

	d, err := uc.devices.Get(ctx, id)
	if err != nil {
		return err
	}
	if !d.Managed {
		return ErrDeviceInvalidArgument
	}
	cred, err := uc.creds.Get(ctx, d.CredentialID)
	if err != nil {
		_ = uc.devices.RecordFailure(ctx, id, "credential missing", 1)
		return err
	}
	port := d.SNMPPort
	if port == 0 {
		port = cred.Port
	}
	snap, err := uc.collector.Collect(ctx, Target{IP: d.MgmtIP, Port: port, Credential: cred})
	if err != nil {
		_ = uc.devices.RecordFailure(ctx, id, err.Error(), 2)
		if d.FailCount+1 >= 2 && uc.alerts != nil {
			uc.alerts.Raise(ctx, &Alert{
				Kind: AlertDeviceOffline, Severity: SeverityCritical,
				Title:   fmt.Sprintf("网络设备离线: %s (%s)", d.Name, d.MgmtIP),
				Message: "SNMP 连续采集失败: " + err.Error(), TargetKey: d.MgmtIP, DeviceID: d.ID,
			})
		}
		return ErrCollectFailed(err.Error())
	}
	applySnapshot(d, snap)
	return uc.devices.SaveSnapshot(ctx, d, snap)
}

func applySnapshot(d *Device, s *Snapshot) {
	fp := Identify(s.Sys.ObjectID, s.Sys.Descr)
	now := time.Now()
	d.SysName, d.SysDescr, d.SysOID = s.Sys.Name, s.Sys.Descr, s.Sys.ObjectID
	d.Vendor, d.Family, d.Model, d.OSVersion = fp.Vendor, fp.Family, fp.Model, fp.OSVersion
	d.Serial, d.StackMembers, d.ChassisMAC = s.Serial, s.StackMembers, s.ChassisMAC
	d.IPs = s.IPs
	d.CPUUsage, d.MemUsage, d.Temperature = s.CPU, s.Mem, s.Temp
	d.UptimeSecs = s.Sys.UptimeSecs
	d.ArpCount = len(s.Arp)
	if d.Location == "" {
		d.Location = s.Sys.Location
	}
	if d.Name == "" || d.Name == d.MgmtIP {
		if s.Sys.Name != "" {
			d.Name = s.Sys.Name
		}
	}
	if !d.RoleManual {
		d.Role = RoleByFamily(fp.Family)
	}
	d.Status, d.LastError, d.FailCount = StatusOnline, "", 0
	d.LastPollAt = &now
}

// PollAndRefresh 采集单台设备,随后刷新拓扑与终端。
func (uc *PollUsecase) PollAndRefresh(ctx context.Context, id uint) error {
	err := uc.PollDevice(ctx, id)
	if rerr := uc.Refresh(ctx); rerr != nil {
		log.Warn("refresh failed", "err", rerr)
	}
	return err
}

// Refresh 重建拓扑并同步终端。串行执行。
func (uc *PollUsecase) Refresh(ctx context.Context) error {
	return uc.RefreshWithNotice(ctx, "poll", "", "拓扑与终端已更新")
}

// RefreshWithNotice 重建拓扑与终端并通过 SSE 通知在线前端。
func (uc *PollUsecase) RefreshWithNotice(ctx context.Context, source, device, msg string) error {
	uc.refresh.Lock()
	defer uc.refresh.Unlock()
	if err := uc.topo.Rebuild(ctx); err != nil {
		return fmt.Errorf("rebuild topology: %w", err)
	}
	if err := uc.endpoints.Sync(ctx); err != nil {
		return fmt.Errorf("sync endpoints: %w", err)
	}
	uc.bus.Publish(LiveEvent{Type: "topology_updated", Source: source, Device: device, Message: msg})
	return nil
}

// RunDue 采集所有到期设备(受全局并发限制),完成后刷新一次。
func (uc *PollUsecase) RunDue(ctx context.Context) (int, error) {
	devs, err := uc.devices.ListAll(ctx)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	var due []uint
	for _, d := range devs {
		if !d.Managed {
			continue
		}
		if d.LastPollAt == nil || now.Sub(*d.LastPollAt) >= uc.opts.PollInterval {
			due = append(due, d.ID)
		}
	}
	if len(due) == 0 {
		return 0, nil
	}
	sem := make(chan struct{}, uc.opts.Concurrency)
	var wg sync.WaitGroup
	for _, id := range due {
		sem <- struct{}{}
		wg.Add(1)
		go func(id uint) {
			defer func() { <-sem; wg.Done() }()
			if err := uc.PollDevice(ctx, id); err != nil {
				log.Warn("poll failed", "device", id, "err", err)
			}
		}(id)
	}
	wg.Wait()
	return len(due), uc.Refresh(ctx)
}
