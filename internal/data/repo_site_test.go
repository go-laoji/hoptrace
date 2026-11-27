package data

import (
	"context"
	"errors"
	"testing"

	"hoptrace/internal/biz"
	"hoptrace/internal/conf"
)

func TestSiteLifecycleAndDeviceAssignment(t *testing.T) {
	ctx := context.Background()
	d, cleanup, err := NewData(&conf.Data{
		Database: &conf.Data_Database{Driver: "sqlite", Source: ":memory:", AutoMigrate: true},
	}, &conf.Security{SecretKey: "test-secret"})
	if err != nil {
		t.Fatalf("NewData: %v", err)
	}
	defer cleanup()

	siteRepo := NewSiteRepo(d)
	siteUC := biz.NewSiteUsecase(siteRepo)
	devRepo := NewDeviceRepo(d)

	dc, err := siteUC.Create(ctx, &biz.Site{
		Name:    "总部核心机房",
		Code:    "HQ-DC1",
		Type:    biz.SiteTypeDatacenter,
		Address: "A座3层机房",
		Contact: "张三",
		CIDRs:   []string{"192.168.140.0/24"},
	})
	if err != nil {
		t.Fatalf("Create site: %v", err)
	}
	if dc.ID == 0 || dc.Type != biz.SiteTypeDatacenter {
		t.Fatalf("unexpected site: %+v", dc)
	}

	// 同名校验
	if _, err := siteUC.Create(ctx, &biz.Site{Name: "总部核心机房"}); !errors.Is(err, biz.ErrSiteAlreadyExists) {
		t.Fatalf("expected ErrSiteAlreadyExists, got %v", err)
	}

	dev, err := devRepo.Create(ctx, &biz.Device{
		Name:    "hx-01",
		MgmtIP:  "192.168.140.254",
		Managed: true,
		Status:  biz.StatusOnline,
	})
	if err != nil {
		t.Fatalf("Create device: %v", err)
	}

	if err := siteUC.AssignDevices(ctx, dc.ID, []uint{dev.ID}); err != nil {
		t.Fatalf("AssignDevices: %v", err)
	}

	got, err := siteUC.Get(ctx, dc.ID)
	if err != nil {
		t.Fatalf("Get site: %v", err)
	}
	if got.DeviceCount != 1 || got.OnlineDeviceCount != 1 {
		t.Fatalf("expected 1/1 devices, got %d/%d", got.DeviceCount, got.OnlineDeviceCount)
	}

	// 仍有关联设备时禁止删除
	if err := siteUC.Delete(ctx, dc.ID); !errors.Is(err, biz.ErrSiteInUse) {
		t.Fatalf("expected ErrSiteInUse, got %v", err)
	}

	// 站点改名后自动同步到设备
	renamed, err := siteUC.Update(ctx, &biz.Site{
		ID:      dc.ID,
		Name:    "总部一号机房",
		Code:    "HQ-DC1",
		Type:    biz.SiteTypeDatacenter,
		Address: "A座3层机房",
	})
	if err != nil {
		t.Fatalf("Update site: %v", err)
	}
	if renamed.DeviceCount != 1 {
		t.Fatalf("expected device count preserved after rename, got %d", renamed.DeviceCount)
	}

	filtered, total, err := devRepo.List(ctx, biz.DeviceFilter{Site: "总部一号机房"}, 1, 20)
	if err != nil || total != 1 || len(filtered) != 1 || filtered[0].Site != "总部一号机房" {
		t.Fatalf("expected 1 device in renamed site, got total=%d len=%d err=%v", total, len(filtered), err)
	}

	// 解绑后再删除
	if err := siteUC.AssignDevices(ctx, 0, []uint{dev.ID}); err != nil {
		t.Fatalf("Unassign devices: %v", err)
	}
	if err := siteUC.Delete(ctx, dc.ID); err != nil {
		t.Fatalf("Delete site: %v", err)
	}
}
