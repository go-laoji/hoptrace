package data

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"hoptrace/internal/biz"

	"gorm.io/gorm"
)

type deviceRepo struct {
	data *Data
}

func NewDeviceRepo(d *Data) biz.DeviceRepo {
	return &deviceRepo{data: d}
}

func (r *deviceRepo) List(ctx context.Context, f biz.DeviceFilter, page, size int) ([]*biz.Device, int64, error) {
	q := r.data.db.WithContext(ctx).Model(&DevicePO{})
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name LIKE ? OR mgmt_ip LIKE ? OR model LIKE ? OR sys_name LIKE ? OR site LIKE ?", like, like, like, like, like)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.Family != "" {
		q = q.Where("family = ?", f.Family)
	}
	if f.Role != "" {
		q = q.Where("role = ?", f.Role)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 500 {
		size = 50
	}
	var pos []DevicePO
	if err := q.Order("managed desc, id asc").Offset((page - 1) * size).Limit(size).Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*biz.Device, 0, len(pos))
	ids := make([]uint, 0, len(pos))
	for i := range pos {
		out = append(out, deviceToBiz(&pos[i]))
		ids = append(ids, pos[i].ID)
	}
	if len(ids) > 0 {
		ifCnt := r.countByDevice(ctx, &InterfacePO{}, ids)
		epCnt := r.countByDevice(ctx, &EndpointPO{}, ids)
		for _, d := range out {
			d.InterfaceCount = ifCnt[d.ID]
			d.EndpointCount = epCnt[d.ID]
		}
	}
	return out, total, nil
}

func (r *deviceRepo) countByDevice(ctx context.Context, model any, ids []uint) map[uint]int {
	type row struct {
		DeviceID uint
		Cnt      int
	}
	var rows []row
	_ = r.data.db.WithContext(ctx).Model(model).Select("device_id, count(*) as cnt").Where("device_id IN ?", ids).Group("device_id").Scan(&rows).Error
	m := make(map[uint]int, len(rows))
	for _, x := range rows {
		m[x.DeviceID] = x.Cnt
	}
	return m
}

func (r *deviceRepo) ListAll(ctx context.Context) ([]*biz.Device, error) {
	var pos []DevicePO
	if err := r.data.db.WithContext(ctx).Order("id asc").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*biz.Device, 0, len(pos))
	for i := range pos {
		out = append(out, deviceToBiz(&pos[i]))
	}
	return out, nil
}

func (r *deviceRepo) Get(ctx context.Context, id uint) (*biz.Device, error) {
	var po DevicePO
	err := r.data.db.WithContext(ctx).First(&po, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, biz.ErrDeviceNotFound
	}
	if err != nil {
		return nil, err
	}
	d := deviceToBiz(&po)
	var ifc, epc int64
	_ = r.data.db.WithContext(ctx).Model(&InterfacePO{}).Where("device_id = ?", id).Count(&ifc).Error
	_ = r.data.db.WithContext(ctx).Model(&EndpointPO{}).Where("device_id = ?", id).Count(&epc).Error
	d.InterfaceCount, d.EndpointCount = int(ifc), int(epc)
	return d, nil
}

func (r *deviceRepo) FindByIP(ctx context.Context, ip string) (*biz.Device, error) {
	var po DevicePO
	err := r.data.db.WithContext(ctx).Where("mgmt_ip = ?", ip).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return deviceToBiz(&po), nil
}

func (r *deviceRepo) Create(ctx context.Context, d *biz.Device) (*biz.Device, error) {
	po := deviceToPO(d)
	if err := r.data.db.WithContext(ctx).Create(po).Error; err != nil {
		return nil, err
	}
	return deviceToBiz(po), nil
}

func (r *deviceRepo) Update(ctx context.Context, d *biz.Device) error {
	return r.data.db.WithContext(ctx).Save(deviceToPO(d)).Error
}

func (r *deviceRepo) Delete(ctx context.Context, id uint) error {
	return r.data.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("device_id = ?", id).Delete(&InterfacePO{}).Error; err != nil {
			return err
		}
		if err := tx.Where("device_id = ?", id).Delete(&NeighborPO{}).Error; err != nil {
			return err
		}
		if err := tx.Where("device_id = ?", id).Delete(&FdbPO{}).Error; err != nil {
			return err
		}
		if err := tx.Where("device_id = ?", id).Delete(&ArpPO{}).Error; err != nil {
			return err
		}
		if err := tx.Where("a_device_id = ? OR b_device_id = ?", id, id).Delete(&LinkPO{}).Error; err != nil {
			return err
		}
		return tx.Delete(&DevicePO{}, id).Error
	})
}

func (r *deviceRepo) ListInterfaces(ctx context.Context, deviceID uint) ([]*biz.Interface, error) {
	var pos []InterfacePO
	if err := r.data.db.WithContext(ctx).Where("device_id = ?", deviceID).Order("if_index asc").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*biz.Interface, 0, len(pos))
	for i := range pos {
		out = append(out, interfaceToBiz(&pos[i]))
	}
	return out, nil
}

func (r *deviceRepo) ListAllInterfaces(ctx context.Context) ([]*biz.Interface, error) {
	var pos []InterfacePO
	if err := r.data.db.WithContext(ctx).Order("device_id asc, if_index asc").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*biz.Interface, 0, len(pos))
	for i := range pos {
		out = append(out, interfaceToBiz(&pos[i]))
	}
	return out, nil
}

func (r *deviceRepo) SaveSnapshot(ctx context.Context, d *biz.Device, s *biz.Snapshot) error {
	return r.data.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(deviceToPO(d)).Error; err != nil {
			return err
		}
		// 保留之前接口计算出的 Kind/MACCount/Neighbor,待拓扑重建覆盖
		var oldIfs []InterfacePO
		_ = tx.Where("device_id = ?", d.ID).Find(&oldIfs).Error
		oldMap := make(map[int]InterfacePO, len(oldIfs))
		for _, o := range oldIfs {
			oldMap[o.IfIndex] = o
		}

		if err := tx.Where("device_id = ?", d.ID).Delete(&InterfacePO{}).Error; err != nil {
			return err
		}
		now := time.Now()
		if len(s.Interfaces) > 0 {
			rows := make([]InterfacePO, 0, len(s.Interfaces))
			var metrics []InterfaceMetricPO
			for _, it := range s.Interfaces {
				po := interfaceToPO(d.ID, it)
				po.SampledAt = &now
				if prev, ok := oldMap[it.IfIndex]; ok {
					po.Kind, po.MACCount, po.Neighbor = prev.Kind, prev.MACCount, prev.Neighbor
					if prev.SampledAt != nil && now.After(*prev.SampledAt) {
						secs := now.Sub(*prev.SampledAt).Seconds()
						if secs >= 1 {
							if po.InOctets >= prev.InOctets && (po.InOctets > 0 || prev.InOctets > 0) {
								po.InBps = int64(float64(po.InOctets-prev.InOctets) * 8 / secs)
							}
							if po.OutOctets >= prev.OutOctets && (po.OutOctets > 0 || prev.OutOctets > 0) {
								po.OutBps = int64(float64(po.OutOctets-prev.OutOctets) * 8 / secs)
							}
							if po.SpeedMbps > 0 {
								peak := po.InBps
								if po.OutBps > peak {
									peak = po.OutBps
								}
								util := float64(peak) * 100 / float64(po.SpeedMbps*1_000_000)
								if util > 100 {
									util = 100
								}
								po.UtilizationPct = math.Round(util*10) / 10
							}
							if po.InBps > 0 || po.OutBps > 0 || po.InErrors > 0 || po.OutErrors > 0 {
								metrics = append(metrics, InterfaceMetricPO{
									DeviceID: d.ID, IfIndex: po.IfIndex, At: now,
									InBps: po.InBps, OutBps: po.OutBps, UtilizationPct: po.UtilizationPct,
									InErrors: po.InErrors, OutErrors: po.OutErrors,
								})
							}
						}
					}
				}
				rows = append(rows, po)
			}
			if err := tx.CreateInBatches(rows, 200).Error; err != nil {
				return err
			}
			if len(metrics) > 0 {
				_ = tx.CreateInBatches(metrics, 200).Error
				// 仅保留最近 7 天采样点
				_ = tx.Where("at < ?", now.Add(-7*24*time.Hour)).Delete(&InterfaceMetricPO{}).Error
			}
		}

		if err := tx.Where("device_id = ?", d.ID).Delete(&NeighborPO{}).Error; err != nil {
			return err
		}
		if len(s.Neighbors) > 0 {
			rows := make([]NeighborPO, 0, len(s.Neighbors))
			for _, n := range s.Neighbors {
				rows = append(rows, neighborToPO(d.ID, n))
			}
			if err := tx.CreateInBatches(rows, 200).Error; err != nil {
				return err
			}
		}

		if err := tx.Where("device_id = ?", d.ID).Delete(&FdbPO{}).Error; err != nil {
			return err
		}
		if len(s.Fdb) > 0 {
			rows := make([]FdbPO, 0, len(s.Fdb))
			for _, f := range s.Fdb {
				rows = append(rows, FdbPO{DeviceID: d.ID, MAC: f.MAC, VLAN: f.VLAN, IfIndex: f.IfIndex})
			}
			if err := tx.CreateInBatches(rows, 500).Error; err != nil {
				return err
			}
		}

		if err := tx.Where("device_id = ?", d.ID).Delete(&ArpPO{}).Error; err != nil {
			return err
		}
		if len(s.Arp) > 0 {
			rows := make([]ArpPO, 0, len(s.Arp))
			for _, a := range s.Arp {
				rows = append(rows, ArpPO{DeviceID: d.ID, IP: a.IP, MAC: a.MAC, IfIndex: a.IfIndex})
			}
			if err := tx.CreateInBatches(rows, 500).Error; err != nil {
				return err
			}
		}

		if err := tx.Where("device_id = ?", d.ID).Delete(&WirelessSTAPO{}).Error; err != nil {
			return err
		}
		if len(s.Wireless) > 0 {
			rows := make([]WirelessSTAPO, 0, len(s.Wireless))
			for _, w := range s.Wireless {
				rows = append(rows, WirelessSTAPO{
					DeviceID: d.ID, MAC: w.MAC, IP: w.IP, SSID: w.SSID, APName: w.APName, RSSI: w.RSSI, VLAN: w.VLAN,
				})
			}
			if err := tx.CreateInBatches(rows, 200).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *deviceRepo) RecordFailure(ctx context.Context, id uint, msg string, offlineAfter int) error {
	var po DevicePO
	if err := r.data.db.WithContext(ctx).First(&po, id).Error; err != nil {
		return err
	}
	now := time.Now()
	po.FailCount++
	po.LastError = msg
	po.LastPollAt = &now
	if offlineAfter <= 0 {
		offlineAfter = 2
	}
	if po.FailCount >= offlineAfter {
		po.Status = biz.StatusOffline
	}
	return r.data.db.WithContext(ctx).Save(&po).Error
}

func (r *deviceRepo) ListNeighbors(ctx context.Context, deviceID uint) ([]*biz.Neighbor, error) {
	var pos []NeighborPO
	if err := r.data.db.WithContext(ctx).Where("device_id = ?", deviceID).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*biz.Neighbor, 0, len(pos))
	for i := range pos {
		out = append(out, neighborToBiz(&pos[i]))
	}
	return out, nil
}

func (r *deviceRepo) ListInterfaceMetrics(ctx context.Context, deviceID uint, ifIndex, limit int) ([]biz.InterfaceMetricPoint, error) {
	if limit <= 0 || limit > 200 {
		limit = 30
	}
	var pos []InterfaceMetricPO
	if err := r.data.db.WithContext(ctx).
		Where("device_id = ? AND if_index = ?", deviceID, ifIndex).
		Order("at desc").Limit(limit).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]biz.InterfaceMetricPoint, 0, len(pos))
	for i := len(pos) - 1; i >= 0; i-- {
		p := pos[i]
		out = append(out, biz.InterfaceMetricPoint{
			At: p.At, InBps: p.InBps, OutBps: p.OutBps, UtilizationPct: p.UtilizationPct,
			InErrors: p.InErrors, OutErrors: p.OutErrors,
		})
	}
	return out, nil
}

func (r *deviceRepo) SaveLayout(ctx context.Context, positions []biz.NodePos) error {
	return r.data.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, p := range positions {
			if err := tx.Model(&DevicePO{}).Where("id = ?", p.DeviceID).Updates(map[string]any{
				"pos_x": p.X, "pos_y": p.Y, "has_pos": true,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *deviceRepo) ResetLayout(ctx context.Context) error {
	return r.data.db.WithContext(ctx).Model(&DevicePO{}).Where("has_pos = ?", true).Updates(map[string]any{
		"pos_x": 0, "pos_y": 0, "has_pos": false,
	}).Error
}
