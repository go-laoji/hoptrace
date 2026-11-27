package biz

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v3/log"
)

// ---------- 拓扑 ----------

// TopologyUsecase 拓扑融合与查询。
type TopologyUsecase struct {
	devices   DeviceRepo
	topo      TopologyRepo
	endpoints EndpointRepo
	opts      Options
	mu        sync.Mutex
}

func NewTopologyUsecase(devices DeviceRepo, topo TopologyRepo, endpoints EndpointRepo, opts Options) *TopologyUsecase {
	return &TopologyUsecase{devices: devices, topo: topo, endpoints: endpoints, opts: opts.withDefaults()}
}

// Rebuild 依据最新 LLDP/FDB 快照重建链路,必要时创建占位设备并二次融合。
func (uc *TopologyUsecase) Rebuild(ctx context.Context) error {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	now := time.Now()
	for pass := 0; pass < 3; pass++ {
		in, err := uc.load(ctx, now)
		if err != nil {
			return err
		}
		res := BuildTopology(*in)
		if len(res.ObsoleteStubs) > 0 {
			if err := uc.topo.DeleteDevices(ctx, res.ObsoleteStubs); err != nil {
				return err
			}
		}
		if len(res.Stubs) > 0 && pass < 2 {
			if err := uc.topo.CreateStubs(ctx, res.Stubs); err != nil {
				return err
			}
			continue // 重新加载后占位设备可被解析为链路对端
		}
		if err := uc.topo.SaveTopology(ctx, res, now); err != nil {
			return err
		}
		return uc.topo.PurgeStaleLinks(ctx, now.Add(-7*24*time.Hour))
	}
	return nil
}

func (uc *TopologyUsecase) load(ctx context.Context, now time.Time) (*TopoInput, error) {
	devs, err := uc.devices.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	ifs, err := uc.devices.ListAllInterfaces(ctx)
	if err != nil {
		return nil, err
	}
	nbs, err := uc.topo.ListAllNeighbors(ctx)
	if err != nil {
		return nil, err
	}
	fdb, err := uc.endpoints.ListFdb(ctx)
	if err != nil {
		return nil, err
	}
	macs, err := uc.topo.PortMACCounts(ctx)
	if err != nil {
		return nil, err
	}
	arps, err := uc.topo.ArpCounts(ctx)
	if err != nil {
		return nil, err
	}
	return &TopoInput{Devices: devs, Interfaces: ifs, Neighbors: nbs, Fdb: fdb, MACCounts: macs, ArpCounts: arps,
		Threshold: uc.opts.SuspectMACThreshold, Now: now}, nil
}

// Get 返回拓扑视图。expand 为需要展开接入终端的设备 ID;all 展开全部设备。
func (uc *TopologyUsecase) Get(ctx context.Context, expand []uint, all bool, site string) (*Topology, error) {
	devs, err := uc.devices.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	links, err := uc.topo.ListLinks(ctx)
	if err != nil {
		return nil, err
	}
	counts, err := uc.endpoints.CountByDevice(ctx)
	if err != nil {
		return nil, err
	}
	out := &Topology{}
	inSite := map[uint]bool{}
	for _, d := range devs {
		if site != "" && d.Site != site {
			continue
		}
		inSite[d.ID] = true
		name := d.Name
		if name == "" {
			name = d.MgmtIP
		}
		out.Nodes = append(out.Nodes, &TopoNode{
			ID: devNodeID(d.ID), Kind: "device", RefID: d.ID, Name: name, IP: d.MgmtIP, Role: d.Role,
			Family: d.Family, Status: d.Status, Vendor: d.Vendor, Model: d.Model, Managed: d.Managed,
			EndpointCount: counts[d.ID],
		})
	}
	for _, l := range links {
		if !inSite[l.ADeviceID] || !inSite[l.BDeviceID] {
			continue
		}
		out.Links = append(out.Links, &TopoLink{
			ID: "l" + strconv.Itoa(int(l.ID)), Source: devNodeID(l.ADeviceID), Target: devNodeID(l.BDeviceID),
			SourcePort: l.AIfName, TargetPort: l.BIfName, Kind: l.Kind, Confidence: l.Confidence,
			Status: l.Status, Members: l.Members,
		})
	}
	var ids []uint
	if all {
		for id := range inSite {
			ids = append(ids, id)
		}
	} else {
		for _, id := range expand {
			if inSite[id] {
				ids = append(ids, id)
			}
		}
	}
	if len(ids) > 0 {
		eps, err := uc.endpoints.ListByDevices(ctx, ids, false)
		if err != nil {
			return nil, err
		}
		for _, e := range eps {
			name := e.Name
			if name == "" {
				name = e.IP
			}
			if name == "" {
				name = e.MAC
			}
			id := "e" + strconv.Itoa(int(e.ID))
			out.Nodes = append(out.Nodes, &TopoNode{
				ID: id, Kind: "endpoint", RefID: e.ID, Name: name, IP: e.IP, Status: e.Status,
				Vendor: e.Vendor, Type: e.Type,
			})
			out.Links = append(out.Links, &TopoLink{
				ID: "a" + strconv.Itoa(int(e.ID)), Source: devNodeID(e.DeviceID), Target: id,
				SourcePort: e.IfName, Kind: "access", Confidence: 1, Status: map[bool]string{true: LinkUp, false: LinkStale}[e.Status == EndpointOnline],
			})
		}
	}
	return out, nil
}

func devNodeID(id uint) string { return "d" + strconv.Itoa(int(id)) }

// ---------- 终端 ----------

// EndpointUsecase 终端同步与管理。
type EndpointUsecase struct {
	repo    EndpointRepo
	devices DeviceRepo
	opts    Options
	mu      sync.Mutex
}

func NewEndpointUsecase(repo EndpointRepo, devices DeviceRepo, opts Options) *EndpointUsecase {
	return &EndpointUsecase{repo: repo, devices: devices, opts: opts.withDefaults()}
}

// Sync 运行终端定位引擎并落库。
func (uc *EndpointUsecase) Sync(ctx context.Context) error {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	devs, err := uc.devices.ListAll(ctx)
	if err != nil {
		return err
	}
	ifs, err := uc.devices.ListAllInterfaces(ctx)
	if err != nil {
		return err
	}
	fdb, err := uc.repo.ListFdb(ctx)
	if err != nil {
		return err
	}
	arp, err := uc.repo.ListArp(ctx)
	if err != nil {
		return err
	}
	nbs := []*Neighbor(nil)
	for _, d := range devs {
		if !d.Managed {
			continue
		}
		n, err := uc.devices.ListNeighbors(ctx, d.ID)
		if err != nil {
			return err
		}
		nbs = append(nbs, n...)
	}
	existing, err := uc.repo.ListAll(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	changes := ComputeEndpointChanges(EndpointInput{
		Devices: devs, Interfaces: ifs, Fdb: fdb, Arp: arp, Neighbors: nbs, Existing: existing, Now: now,
		Opts: EngineOptions{MoveConfirmRounds: uc.opts.MoveConfirmRounds, OfflineMissRounds: uc.opts.OfflineMissRounds},
	})
	if len(changes) == 0 {
		return nil
	}
	return uc.repo.ApplyChanges(ctx, changes, now)
}

func (uc *EndpointUsecase) List(ctx context.Context, f EndpointFilter, page, size int) ([]*Endpoint, int64, error) {
	return uc.repo.List(ctx, f, page, size)
}

// Detail 终端详情:位置历史与事件。
func (uc *EndpointUsecase) Detail(ctx context.Context, id uint) (*Endpoint, []*EndpointLocation, []*EndpointEvent, error) {
	e, err := uc.repo.Get(ctx, id)
	if err != nil {
		return nil, nil, nil, err
	}
	locs, err := uc.repo.Locations(ctx, id)
	if err != nil {
		return nil, nil, nil, err
	}
	evs, _, err := uc.repo.Events(ctx, EventFilter{EndpointID: id}, 1, 50)
	if err != nil {
		return nil, nil, nil, err
	}
	return e, locs, evs, nil
}

// Update 更新台账信息。typ 为空表示恢复自动识别。
func (uc *EndpointUsecase) Update(ctx context.Context, id uint, name, owner string, tags []string, trust, typ, remark string) (*Endpoint, error) {
	e, err := uc.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	switch trust {
	case "", TrustUnknown, TrustTrusted, TrustBlocked:
	default:
		return nil, ErrEndpointInvalid
	}
	if trust != "" {
		e.Trust = trust
	}
	e.Name, e.Owner, e.Tags, e.Remark = name, owner, uniqueStrings(tags), remark
	if typ != "" {
		e.Type, e.TypeManual = typ, true
	} else if e.TypeManual {
		e.TypeManual = false
		e.Type = InferEndpointType(e.Vendor, e.Name)
	}
	if err := uc.repo.Update(ctx, e); err != nil {
		return nil, err
	}
	return uc.repo.Get(ctx, id)
}

func (uc *EndpointUsecase) Delete(ctx context.Context, id uint) error {
	if _, err := uc.repo.Get(ctx, id); err != nil {
		return err
	}
	return uc.repo.Delete(ctx, id)
}

func (uc *EndpointUsecase) Events(ctx context.Context, f EventFilter, page, size int) ([]*EndpointEvent, int64, error) {
	return uc.repo.Events(ctx, f, page, size)
}

// ---------- 仪表盘 ----------

// DashboardUsecase 仪表盘统计。
type DashboardUsecase struct{ repo StatsRepo }

func NewDashboardUsecase(repo StatsRepo) *DashboardUsecase { return &DashboardUsecase{repo: repo} }

func (uc *DashboardUsecase) Get(ctx context.Context) (*Stats, error) {
	s, err := uc.repo.Stats(ctx, time.Now())
	if err != nil {
		return nil, err
	}
	sort.Slice(s.DevicesByRole, func(i, j int) bool { return s.DevicesByRole[i].Value > s.DevicesByRole[j].Value })
	sort.Slice(s.EndpointsByType, func(i, j int) bool { return s.EndpointsByType[i].Value > s.EndpointsByType[j].Value })
	return s, nil
}

var _ = fmt.Sprintf
var _ = log.Info
