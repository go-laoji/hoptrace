package biz

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v3/log"
)

// DiscoveryRequest 发现任务参数。
type DiscoveryRequest struct {
	Seeds           []string
	CIDRs           []string
	CredentialIDs   []uint
	ExpandNeighbors bool
	MaxDevices      int
	Site            string
}

// DiscoveryUsecase 设备发现任务编排(种子 + 网段扫描 + LLDP 邻居 BFS 扩散)。
type DiscoveryUsecase struct {
	jobs      JobRepo
	devices   DeviceRepo
	creds     CredentialRepo
	collector Collector
	poll      *PollUsecase
}

func NewDiscoveryUsecase(jobs JobRepo, devices DeviceRepo, creds CredentialRepo, c Collector, poll *PollUsecase) *DiscoveryUsecase {
	return &DiscoveryUsecase{jobs: jobs, devices: devices, creds: creds, collector: c, poll: poll}
}

func (uc *DiscoveryUsecase) List(ctx context.Context, limit int) ([]*DiscoveryJob, error) {
	if limit <= 0 {
		limit = 20
	}
	return uc.jobs.List(ctx, limit)
}

func (uc *DiscoveryUsecase) Get(ctx context.Context, id uint) (*DiscoveryJob, error) {
	return uc.jobs.Get(ctx, id)
}

// Start 创建并异步执行发现任务。
func (uc *DiscoveryUsecase) Start(ctx context.Context, req DiscoveryRequest) (*DiscoveryJob, error) {
	if len(req.Seeds) == 0 && len(req.CIDRs) == 0 {
		return nil, ErrDiscoveryInvalid
	}
	initialIPs, err := expandTargets(req.Seeds, req.CIDRs, 4096)
	if err != nil {
		return nil, ErrDiscoveryInvalid
	}
	if len(initialIPs) == 0 {
		return nil, ErrDiscoveryInvalid
	}

	creds, err := uc.selectCredentials(ctx, req.CredentialIDs)
	if err != nil {
		return nil, err
	}
	if len(creds) == 0 {
		return nil, ErrCredentialNotFound
	}

	maxDevs := req.MaxDevices
	if maxDevs <= 0 {
		maxDevs = 256
	}

	job, err := uc.jobs.Create(ctx, &DiscoveryJob{
		Status:    JobRunning,
		Seeds:     req.Seeds,
		CIDRs:     req.CIDRs,
		Total:     len(initialIPs),
		StartedAt: time.Now(),
	})
	if err != nil {
		return nil, err
	}

	go uc.run(job, initialIPs, creds, req.ExpandNeighbors, maxDevs, req.Site)
	return job, nil
}

func (uc *DiscoveryUsecase) selectCredentials(ctx context.Context, ids []uint) ([]*Credential, error) {
	all, err := uc.creds.List(ctx)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return all, nil
	}
	want := make(map[uint]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	var out []*Credential
	for _, c := range all {
		if want[c.ID] {
			out = append(out, c)
		}
	}
	return out, nil
}

func (uc *DiscoveryUsecase) run(job *DiscoveryJob, initial []string, creds []*Credential, expand bool, maxDevs int, site string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	var (
		mu       sync.Mutex
		visited  = map[string]bool{}
		queue    = make([]string, 0, len(initial))
		foundIDs []uint
		logs     []string
	)
	for _, ip := range initial {
		if !visited[ip] {
			visited[ip] = true
			queue = append(queue, ip)
		}
	}

	appendLog := func(format string, args ...any) {
		mu.Lock()
		defer mu.Unlock()
		line := fmt.Sprintf("[%s] "+format, append([]any{time.Now().Format("15:04:05")}, args...)...)
		logs = append(logs, line)
		if len(logs) > 200 {
			logs = logs[len(logs)-200:]
		}
		job.Log = strings.Join(logs, "\n")
	}

	appendLog("开始发现: 初始目标 %d 个, 凭据 %d 个, 邻居扩散=%v", len(queue), len(creds), expand)
	_ = uc.jobs.Update(ctx, job)

	conc := uc.poll.Options().Concurrency
	if conc <= 0 {
		conc = 10
	}

	// 按波次 BFS 扩散
	for wave := 0; len(queue) > 0 && len(foundIDs) < maxDevs && wave < 8; wave++ {
		batch := queue
		queue = nil
		nextIPs := map[string]bool{}

		sem := make(chan struct{}, conc)
		var wg sync.WaitGroup
		for _, ip := range batch {
			mu.Lock()
			if len(foundIDs) >= maxDevs {
				mu.Unlock()
				break
			}
			mu.Unlock()

			sem <- struct{}{}
			wg.Add(1)
			go func(ip string) {
				defer func() { <-sem; wg.Done() }()
				cred, sys := uc.probeAny(ctx, ip, creds)
				mu.Lock()
				job.Probed++
				mu.Unlock()
				if cred == nil {
					return
				}

				dev, err := uc.upsertDevice(ctx, ip, cred, sys, site)
				if err != nil {
					appendLog("保存设备 %s 失败: %v", ip, err)
					return
				}
				if err := uc.poll.PollDevice(ctx, dev.ID); err != nil {
					appendLog("采集 %s (%s) 失败: %v", dev.Name, ip, err)
				} else {
					appendLog("发现并采集成功: %s (%s)", dev.Name, ip)
				}

				mu.Lock()
				foundIDs = append(foundIDs, dev.ID)
				job.Found = len(foundIDs)
				mu.Unlock()

				if expand {
					nbs, err := uc.devices.ListNeighbors(ctx, dev.ID)
					if err == nil {
						mu.Lock()
						for _, n := range nbs {
							if ClassifyNeighbor(n) != NeighborNetwork {
								continue
							}
							if ValidEndpointIP(n.RemMgmtIP) && !visited[n.RemMgmtIP] {
								visited[n.RemMgmtIP] = true
								nextIPs[n.RemMgmtIP] = true
							}
						}
						mu.Unlock()
					}
				}
			}(ip)
		}
		wg.Wait()

		for ip := range nextIPs {
			queue = append(queue, ip)
		}
		if len(queue) > 0 {
			job.Total += len(queue)
			appendLog("第 %d 轮通过 LLDP 发现 %d 个新邻居 IP", wave+1, len(queue))
		}
		_ = uc.jobs.Update(ctx, job)
	}

	if err := uc.poll.Refresh(ctx); err != nil {
		appendLog("刷新拓扑与终端失败: %v", err)
		log.Warn("discovery refresh failed", "job", job.ID, "err", err)
	} else {
		appendLog("拓扑与终端同步完成, 共纳管 %d 台设备", len(foundIDs))
	}

	now := time.Now()
	job.Status = JobDone
	job.FinishedAt = &now
	_ = uc.jobs.Update(ctx, job)
}

func (uc *DiscoveryUsecase) probeAny(ctx context.Context, ip string, creds []*Credential) (*Credential, *SysInfo) {
	for _, c := range creds {
		port := c.Port
		if port == 0 {
			port = 161
		}
		sys, err := uc.collector.Probe(ctx, Target{IP: ip, Port: port, Credential: c})
		if err == nil && sys != nil {
			return c, sys
		}
	}
	return nil, nil
}

func (uc *DiscoveryUsecase) upsertDevice(ctx context.Context, ip string, cred *Credential, sys *SysInfo, site string) (*Device, error) {
	old, err := uc.devices.FindByIP(ctx, ip)
	if err != nil {
		return nil, err
	}
	name := sys.Name
	if name == "" {
		name = ip
	}
	if old != nil {
		old.Managed = true
		old.CredentialID = cred.ID
		old.SNMPPort = cred.Port
		if old.Name == "" || old.Name == old.MgmtIP {
			old.Name = name
		}
		if site != "" && old.Site == "" {
			old.Site = site
		}
		if err := uc.devices.Update(ctx, old); err != nil {
			return nil, err
		}
		return old, nil
	}
	fp := Identify(sys.ObjectID, sys.Descr)
	return uc.devices.Create(ctx, &Device{
		Name:         name,
		MgmtIP:       ip,
		SNMPPort:     cred.Port,
		CredentialID: cred.ID,
		Managed:      true,
		SysName:      sys.Name,
		SysDescr:     sys.Descr,
		SysOID:       sys.ObjectID,
		Vendor:       fp.Vendor,
		Family:       fp.Family,
		Model:        fp.Model,
		OSVersion:    fp.OSVersion,
		Role:         RoleByFamily(fp.Family),
		Status:       StatusOnline,
		Site:         site,
		Location:     sys.Location,
	})
}

func expandTargets(seeds, cidrs []string, limit int) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	add := func(ip string) {
		ip = strings.TrimSpace(ip)
		if ip == "" || seen[ip] {
			return
		}
		if net.ParseIP(ip) == nil {
			return
		}
		seen[ip] = true
		out = append(out, ip)
	}
	for _, s := range seeds {
		add(s)
	}
	for _, c := range cidrs {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		ip, ipNet, err := net.ParseCIDR(c)
		if err != nil {
			return nil, err
		}
		ip4 := ip.To4()
		if ip4 == nil {
			return nil, fmt.Errorf("ipv4 only: %s", c)
		}
		ones, bits := ipNet.Mask.Size()
		if bits-ones > 16 {
			return nil, fmt.Errorf("cidr too large: %s", c)
		}
		start := binary.BigEndian.Uint32(ipNet.IP.To4())
		count := uint32(1) << uint(bits-ones)
		first, last := start+1, start+count-1
		if count <= 2 {
			first, last = start, start+count
		}
		for v := first; v < last && len(out) < limit; v++ {
			b := make(net.IP, 4)
			binary.BigEndian.PutUint32(b, v)
			add(b.String())
		}
	}
	return out, nil
}
