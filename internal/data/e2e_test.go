package data

import (
	"context"
	"testing"
	"time"

	"hoptrace/internal/biz"
	"hoptrace/internal/conf"
	"hoptrace/internal/data/snmpx"
	"hoptrace/internal/data/snmpx/sim"
)

func TestEndToEndDiscoveryWithHuaweiSim(t *testing.T) {
	ctx := context.Background()
	d, cleanup, err := NewData(
		&conf.Data{Database: &conf.Data_Database{Driver: "sqlite", Source: ":memory:", AutoMigrate: true}},
		&conf.Security{SecretKey: "test-secret"},
	)
	if err != nil {
		t.Fatalf("NewData: %v", err)
	}
	defer cleanup()

	credRepo := NewCredentialRepo(d)
	devRepo := NewDeviceRepo(d)
	topoRepo := NewTopologyRepo(d)
	epRepo := NewEndpointRepo(d)
	jobRepo := NewJobRepo(d)
	statsRepo := NewStatsRepo(d)
	collector := snmpx.NewCollector(sim.NewDialer())

	opts := biz.Options{Concurrency: 4, SuspectMACThreshold: 30, OfflineMissRounds: 2, MoveConfirmRounds: 2}
	bus := biz.NewEventBus()
	alertRepo := NewAlertRepo(d)
	alertUC := biz.NewAlertUsecase(alertRepo, NewAlertNotifier(), bus)
	topoUC := biz.NewTopologyUsecase(devRepo, topoRepo, epRepo, opts)
	epUC := biz.NewEndpointUsecase(epRepo, devRepo, alertUC, opts)
	pollUC := biz.NewPollUsecase(devRepo, credRepo, collector, topoUC, epUC, alertUC, bus, opts)
	credUC := biz.NewCredentialUsecase(credRepo)
	discUC := biz.NewDiscoveryUsecase(jobRepo, devRepo, credRepo, collector, pollUC)
	dashUC := biz.NewDashboardUsecase(statsRepo)

	// 1. 创建 SNMP 凭据(验证加密读写)
	cred, err := credUC.Save(ctx, &biz.Credential{Name: "huawei-demo", Version: "v2c", Community: "public", Port: 161})
	if err != nil {
		t.Fatalf("Save cred: %v", err)
	}

	// 2. 仅从防火墙种子 10.255.0.1 启动发现并开启 LLDP 邻居扩散
	job, err := discUC.Start(ctx, biz.DiscoveryRequest{
		Seeds:           []string{"10.255.0.1"},
		CredentialIDs:   []uint{cred.ID},
		ExpandNeighbors: true,
		Site:            "总部园区",
	})
	if err != nil {
		t.Fatalf("Start discovery: %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		j, err := discUC.Get(ctx, job.ID)
		if err != nil {
			t.Fatalf("Get job: %v", err)
		}
		if j.Status == biz.JobDone {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// 3. 验证仪表盘统计: 5 台纳管 + 1 台占位 = 6 台设备; 5 条逻辑链路; 8 个在线终端
	st, err := dashUC.Get(ctx)
	if err != nil {
		t.Fatalf("GetDashboard: %v", err)
	}
	if st.DevicesOnline != 5 || st.DevicesUnmanaged != 1 {
		t.Fatalf("unexpected devices: online=%d unmanaged=%d total=%d", st.DevicesOnline, st.DevicesUnmanaged, st.DevicesTotal)
	}
	if st.LinksTotal != 5 {
		t.Fatalf("expected 5 links (FW-Core, Core-Agg LAG, Agg-AccA, Agg-AccB, AccB-Unmanaged), got %d", st.LinksTotal)
	}
	if st.EndpointsOnline != 8 {
		t.Fatalf("expected 8 online endpoints, got %d", st.EndpointsOnline)
	}

	// 4. 验证 Eth-Trunk1 聚合链路与拓扑视图
	topo, err := topoUC.Get(ctx, nil, true, "")
	if err != nil {
		t.Fatalf("GetTopology: %v", err)
	}
	foundLAG := false
	for _, l := range topo.Links {
		if l.SourcePort == "Eth-Trunk1" && l.TargetPort == "Eth-Trunk1" && len(l.Members) == 2 && l.Confidence == 1.0 {
			foundLAG = true
		}
	}
	if !foundLAG {
		t.Fatalf("Eth-Trunk1 merged link not found in topology: %+v", topo.Links)
	}

	// 5. 验证每个终端都被定位到接入交换机(SW-Acc-F1-A / SW-Acc-F1-B),而不是汇聚/核心上联口
	eps, total, err := epUC.List(ctx, biz.EndpointFilter{}, 1, 50)
	if err != nil || total != 8 {
		t.Fatalf("ListEndpoints err=%v total=%d", err, total)
	}
	for _, e := range eps {
		if e.DeviceName != "SW-Acc-F1-A" && e.DeviceName != "SW-Acc-F1-B" {
			t.Errorf("endpoint %s (%s) mislocated on %s %s", e.MAC, e.IP, e.DeviceName, e.IfName)
		}
		if e.IP == "" {
			t.Errorf("endpoint %s missing ARP IP", e.MAC)
		}
	}

	// 6. 验证管控护栏、审批流、执行与一键回滚闭环
	ctrlRepo := NewControlRepo(d)
	ctrlExec := NewControlExecutor(&conf.Collector{Simulate: true})
	ctrlUC := biz.NewControlUsecase(ctrlRepo, devRepo, epRepo, ctrlExec)

	// 6a. 保护名单: 禁止对核心交换机/防火墙执行 shutdown
	devs, _ := devRepo.ListAll(ctx)
	for _, dev := range devs {
		if dev.Role == biz.RoleCore {
			if _, err := ctrlUC.CreateAction(ctx, biz.CreateControlInput{
				Action: biz.ActionShutdownPort, DeviceID: dev.ID, IfIndex: 1,
			}); err != biz.ErrControlProtectedTarget {
				t.Fatalf("expected ErrControlProtectedTarget on core switch, got %v", err)
			}
		}
	}

	// 6b. 封禁终端 -> 审批并执行 -> 自动标记 blocked -> 一键回滚 -> 恢复 unknown
	target := eps[0]
	act, err := ctrlUC.CreateAction(ctx, biz.CreateControlInput{
		Action: biz.ActionBlockEndpoint, EndpointID: target.ID, Protocol: biz.ProtoSSH, Reason: "未知终端隔离测试",
	})
	if err != nil || act.Status != biz.ControlPending {
		t.Fatalf("CreateAction failed: err=%v act=%+v", err, act)
	}
	act, err = ctrlUC.Approve(ctx, act.ID, true)
	if err != nil || act.Status != biz.ControlExecuted {
		t.Fatalf("Approve+Execute failed: err=%v act=%+v", err, act)
	}
	afterBlock, _ := epRepo.Get(ctx, target.ID)
	if afterBlock.Trust != biz.TrustBlocked {
		t.Fatalf("expected endpoint trust=blocked, got %s", afterBlock.Trust)
	}
	act, err = ctrlUC.Rollback(ctx, act.ID)
	if err != nil || act.Status != biz.ControlRolledBack {
		t.Fatalf("Rollback failed: err=%v act=%+v", err, act)
	}
	afterRB, _ := epRepo.Get(ctx, target.ID)
	if afterRB.Trust != biz.TrustUnknown {
		t.Fatalf("expected endpoint trust restored to unknown, got %s", afterRB.Trust)
	}

	// 7. 验证 M3: 路径追踪(终端 -> 防火墙)、坐标保存、IPAM 子网、批量导入与配置备份 Diff
	tr, err := topoUC.TracePath(ctx, target.IP, "")
	if err != nil || !tr.Found || len(tr.Hops) < 3 {
		t.Fatalf("TracePath failed: err=%v tr=%+v", err, tr)
	}
	if err := topoUC.SaveLayout(ctx, []biz.NodePos{{DeviceID: target.DeviceID, X: 120, Y: 240}}); err != nil {
		t.Fatalf("SaveLayout: %v", err)
	}

	ipamUC := biz.NewIpamUsecase(devRepo, epRepo)
	subnets, err := ipamUC.ListSubnets(ctx)
	if err != nil || len(subnets) == 0 {
		t.Fatalf("ListSubnets: err=%v subnets=%+v", err, subnets)
	}

	impRes, err := epUC.BatchImport(ctx, []biz.EndpointImportRow{
		{MAC: target.MAC, Owner: "张三", Trust: biz.TrustTrusted, Tags: []string{"研发部"}},
	})
	if err != nil || impRes.Updated != 1 {
		t.Fatalf("BatchImport: err=%v res=%+v", err, impRes)
	}

	backupUC := biz.NewBackupUsecase(NewBackupRepo(d), devRepo, ctrlRepo, NewConfigFetcher(&conf.Collector{Simulate: true}))
	b1, err := backupUC.Run(ctx, target.DeviceID, "manual")
	if err != nil || b1.Status != "ok" || !b1.Changed {
		t.Fatalf("Backup1: err=%v b=%+v", err, b1)
	}
	b2, err := backupUC.Run(ctx, target.DeviceID, "manual")
	if err != nil || b2.Changed {
		t.Fatalf("Backup2 should be unchanged: err=%v b=%+v", err, b2)
	}
	diff, err := backupUC.Diff(ctx, b1.ID, b2.ID)
	if err != nil || diff.Added != 0 || diff.Removed != 0 {
		t.Fatalf("Diff: err=%v diff=%+v", err, diff)
	}
}
