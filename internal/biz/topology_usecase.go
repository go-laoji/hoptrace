package biz

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
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
	ifs, err := uc.devices.ListAllInterfaces(ctx)
	if err != nil {
		return nil, err
	}
	ifByKey := map[IfKey]*Interface{}
	for _, it := range ifs {
		ifByKey[IfKey{it.DeviceID, it.IfIndex}] = it
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
			EndpointCount: counts[d.ID], PosX: d.PosX, PosY: d.PosY, HasPos: d.HasPos,
		})
	}
	for _, l := range links {
		if !inSite[l.ADeviceID] || !inSite[l.BDeviceID] {
			continue
		}
		var inBps, outBps int64
		var util float64
		for _, it := range []*Interface{ifByKey[IfKey{l.ADeviceID, l.AIfIndex}], ifByKey[IfKey{l.BDeviceID, l.BIfIndex}]} {
			if it != nil && it.UtilizationPct >= util {
				inBps, outBps, util = it.InBps, it.OutBps, it.UtilizationPct
			}
		}
		out.Links = append(out.Links, &TopoLink{
			ID: "l" + strconv.Itoa(int(l.ID)), Source: devNodeID(l.ADeviceID), Target: devNodeID(l.BDeviceID),
			SourcePort: l.AIfName, TargetPort: l.BIfName, Kind: l.Kind, Confidence: l.Confidence,
			Status: l.Status, Members: l.Members, InBps: inBps, OutBps: outBps, UtilizationPct: util,
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
			var inBps, outBps int64
			var util float64
			if e.Status == EndpointOnline && e.IfIndex > 0 {
				if it := ifByKey[IfKey{e.DeviceID, e.IfIndex}]; it != nil {
					inBps, outBps, util = it.InBps, it.OutBps, it.UtilizationPct
				}
			}
			out.Links = append(out.Links, &TopoLink{
				ID: "a" + strconv.Itoa(int(e.ID)), Source: devNodeID(e.DeviceID), Target: id,
				SourcePort: e.IfName, Kind: "access", Confidence: 1,
				Status: map[bool]string{true: LinkUp, false: LinkStale}[e.Status == EndpointOnline],
				InBps:  inBps, OutBps: outBps, UtilizationPct: util,
			})
		}
	}
	return out, nil
}

func devNodeID(id uint) string { return "d" + strconv.Itoa(int(id)) }

// SaveLayout 持久化拓扑节点手工拖拽坐标。
func (uc *TopologyUsecase) SaveLayout(ctx context.Context, positions []NodePos) error {
	return uc.devices.SaveLayout(ctx, positions)
}

// ResetLayout 清除自定义拓扑坐标，恢复自动布局。
func (uc *TopologyUsecase) ResetLayout(ctx context.Context) error {
	return uc.devices.ResetLayout(ctx)
}

// TracePath 基于当前活跃拓扑链路做 BFS 最短路径追踪。
// source/target 均支持终端 IP/MAC/名称 或 网络设备 IP/名称；target 为空时自动追踪到防火墙或核心交换机。
func (uc *TopologyUsecase) TracePath(ctx context.Context, source, target string) (*TraceResult, error) {
	source = strings.TrimSpace(source)
	target = strings.TrimSpace(target)
	if source == "" {
		return &TraceResult{Found: false, Message: "请输入源 IP / MAC / 设备名"}, nil
	}
	devs, err := uc.devices.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	links, err := uc.topo.ListLinks(ctx)
	if err != nil {
		return nil, err
	}
	eps, err := uc.endpoints.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	devByID := map[uint]*Device{}
	for _, d := range devs {
		devByID[d.ID] = d
	}

	findSide := func(q string) (ep *Endpoint, dev *Device) {
		lq := strings.ToLower(q)
		mac := NormalizeMAC(q)
		for _, e := range eps {
			if (mac != "" && e.MAC == mac) || e.IP == q || strings.ToLower(e.Name) == lq {
				return e, devByID[e.DeviceID]
			}
			for _, ip := range e.IPs {
				if ip == q {
					return e, devByID[e.DeviceID]
				}
			}
		}
		for _, d := range devs {
			if d.MgmtIP == q || strings.ToLower(d.Name) == lq || strings.ToLower(d.SysName) == lq {
				return nil, d
			}
			for _, ip := range d.IPs {
				if ip == q {
					return nil, d
				}
			}
		}
		return nil, nil
	}

	srcEp, srcDev := findSide(source)
	if srcDev == nil {
		return &TraceResult{Found: false, Message: "未找到匹配的源终端或设备: " + source}, nil
	}
	var dstEp *Endpoint
	var dstDev *Device
	if target != "" {
		dstEp, dstDev = findSide(target)
		if dstDev == nil {
			return &TraceResult{Found: false, Message: "未找到匹配的目标终端或设备: " + target}, nil
		}
	} else {
		// 目标留空: 优先追踪到防火墙, 其次核心交换机
		for _, wantRole := range []string{RoleFirewall, RoleCore} {
			for _, d := range devs {
				if d.Role == wantRole && d.ID != srcDev.ID {
					dstDev = d
					break
				}
			}
			if dstDev != nil {
				break
			}
		}
		if dstDev == nil {
			dstDev = srcDev
		}
	}

	type edge struct {
		to       uint
		linkID   string
		localIf  string
		remoteIf string
	}
	adj := map[uint][]edge{}
	for _, l := range links {
		if l.Status == LinkStale {
			continue
		}
		lid := "l" + strconv.Itoa(int(l.ID))
		adj[l.ADeviceID] = append(adj[l.ADeviceID], edge{l.BDeviceID, lid, l.AIfName, l.BIfName})
		adj[l.BDeviceID] = append(adj[l.BDeviceID], edge{l.ADeviceID, lid, l.BIfName, l.AIfName})
	}

	// BFS
	type prevInfo struct {
		from uint
		e    edge
	}
	visited := map[uint]bool{srcDev.ID: true}
	prev := map[uint]prevInfo{}
	queue := []uint{srcDev.ID}
	for len(queue) > 0 && !visited[dstDev.ID] {
		cur := queue[0]
		queue = queue[1:]
		for _, e := range adj[cur] {
			if !visited[e.to] {
				visited[e.to] = true
				prev[e.to] = prevInfo{from: cur, e: e}
				queue = append(queue, e.to)
			}
		}
	}
	if !visited[dstDev.ID] {
		return &TraceResult{Found: false, Message: fmt.Sprintf("设备 %s 与 %s 之间暂无连通路径", srcDev.Name, dstDev.Name)}, nil
	}

	// 回溯设备路径
	var devPath []uint
	var edgePath []edge
	for cur := dstDev.ID; cur != srcDev.ID; {
		p := prev[cur]
		devPath = append(devPath, cur)
		edgePath = append(edgePath, p.e)
		cur = p.from
	}
	devPath = append(devPath, srcDev.ID)
	for i, j := 0, len(devPath)-1; i < j; i, j = i+1, j-1 {
		devPath[i], devPath[j] = devPath[j], devPath[i]
	}
	for i, j := 0, len(edgePath)-1; i < j; i, j = i+1, j-1 {
		edgePath[i], edgePath[j] = edgePath[j], edgePath[i]
	}

	res := &TraceResult{Found: true}
	if srcEp != nil {
		eid := "e" + strconv.Itoa(int(srcEp.ID))
		alid := "a" + strconv.Itoa(int(srcEp.ID))
		res.Hops = append(res.Hops, PathHop{
			NodeID: eid, Kind: "endpoint", Name: firstNonEmpty(srcEp.Name, srcEp.MAC),
			IP: srcEp.IP, Role: srcEp.Type, OutPort: srcEp.IfName, LinkID: alid,
		})
		res.NodeIDs = append(res.NodeIDs, eid)
		res.LinkIDs = append(res.LinkIDs, alid)
	}
	for i, did := range devPath {
		d := devByID[did]
		hop := PathHop{NodeID: devNodeID(did), Kind: "device", Name: firstNonEmpty(d.Name, d.MgmtIP), IP: d.MgmtIP, Role: d.Role}
		if i == 0 && srcEp != nil {
			hop.InPort = srcEp.IfName
		} else if i > 0 {
			hop.InPort = edgePath[i-1].remoteIf
		}
		if i < len(edgePath) {
			hop.OutPort = edgePath[i].localIf
			hop.LinkID = edgePath[i].linkID
			res.LinkIDs = append(res.LinkIDs, edgePath[i].linkID)
		} else if dstEp != nil {
			hop.OutPort = dstEp.IfName
		}
		res.Hops = append(res.Hops, hop)
		res.NodeIDs = append(res.NodeIDs, devNodeID(did))
	}
	if dstEp != nil && (srcEp == nil || dstEp.ID != srcEp.ID) {
		eid := "e" + strconv.Itoa(int(dstEp.ID))
		alid := "a" + strconv.Itoa(int(dstEp.ID))
		res.Hops = append(res.Hops, PathHop{
			NodeID: eid, Kind: "endpoint", Name: firstNonEmpty(dstEp.Name, dstEp.MAC),
			IP: dstEp.IP, Role: dstEp.Type, InPort: dstEp.IfName,
		})
		res.NodeIDs = append(res.NodeIDs, eid)
		res.LinkIDs = append(res.LinkIDs, alid)
	}
	res.Message = fmt.Sprintf("共经过 %d 跳", len(res.Hops))
	return res, nil
}

// ---------- 终端 ----------

// EndpointUsecase 终端同步与管理。
type EndpointUsecase struct {
	repo    EndpointRepo
	devices DeviceRepo
	alerts  *AlertUsecase
	opts    Options
	mu      sync.Mutex
}

func NewEndpointUsecase(repo EndpointRepo, devices DeviceRepo, alerts *AlertUsecase, opts Options) *EndpointUsecase {
	return &EndpointUsecase{repo: repo, devices: devices, alerts: alerts, opts: opts.withDefaults()}
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
	wireless, err := uc.repo.ListWireless(ctx)
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
		Devices: devs, Interfaces: ifs, Fdb: fdb, Arp: arp, Neighbors: nbs, Wireless: wireless, Existing: existing, Now: now,
		Opts: EngineOptions{MoveConfirmRounds: uc.opts.MoveConfirmRounds, OfflineMissRounds: uc.opts.OfflineMissRounds},
	})
	if len(changes) == 0 {
		return nil
	}
	if err := uc.repo.ApplyChanges(ctx, changes, now); err != nil {
		return err
	}
	// 触发告警: 仅在已有终端基线后对新增未知终端或漂移终端发出告警
	if uc.alerts != nil && len(existing) > 0 {
		for _, ch := range changes {
			for _, ev := range ch.Events {
				switch ev.Type {
				case EventNew:
					uc.alerts.Raise(ctx, &Alert{
						Kind: AlertNewEndpoint, Severity: SeverityInfo,
						Title: fmt.Sprintf("新终端接入: %s (%s)", ch.Endpoint.MAC, firstNonEmpty(ch.Endpoint.IP, ch.Endpoint.Vendor)),
						Message: ev.Detail, TargetKey: ch.Endpoint.MAC, EndpointID: ch.Endpoint.ID, DeviceID: ch.Endpoint.DeviceID,
					})
				case EventMoved:
					uc.alerts.Raise(ctx, &Alert{
						Kind: AlertEndpointMoved, Severity: SeverityWarning,
						Title: fmt.Sprintf("终端位置漂移: %s", ch.Endpoint.MAC),
						Message: ev.Detail, TargetKey: ch.Endpoint.MAC + ":" + ev.Detail, EndpointID: ch.Endpoint.ID, DeviceID: ch.Endpoint.DeviceID,
					})
				}
			}
		}
	}
	return nil
}

// BatchImport 按 MAC 批量更新终端台账(名称/责任人/标签/信任状态/类型/备注)。
func (uc *EndpointUsecase) BatchImport(ctx context.Context, rows []EndpointImportRow) (*BatchImportResult, error) {
	res := &BatchImportResult{}
	for idx, row := range rows {
		mac := NormalizeMAC(row.MAC)
		if mac == "" {
			res.Skipped++
			res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行 MAC 无效: %s", idx+1, row.MAC))
			continue
		}
		ep, err := uc.repo.FindByMAC(ctx, mac)
		if err != nil || ep == nil {
			res.Skipped++
			res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行未找到终端: %s", idx+1, mac))
			continue
		}
		if row.Name != "" {
			ep.Name = row.Name
		}
		if row.Owner != "" {
			ep.Owner = row.Owner
		}
		if len(row.Tags) > 0 {
			ep.Tags = uniqueStrings(row.Tags)
		}
		if row.Trust == TrustTrusted || row.Trust == TrustUnknown || row.Trust == TrustBlocked {
			ep.Trust = row.Trust
		}
		if row.Type != "" {
			ep.Type, ep.TypeManual = row.Type, true
		}
		if row.Remark != "" {
			ep.Remark = row.Remark
		}
		if err := uc.repo.Update(ctx, ep); err != nil {
			res.Skipped++
			res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行保存失败: %v", idx+1, err))
			continue
		}
		res.Updated++
	}
	return res, nil
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
