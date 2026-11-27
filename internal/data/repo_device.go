package data

import (
	"context"
	"errors"
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
		if len(s.Interfaces) > 0 {
			rows := make([]InterfacePO, 0, len(s.Interfaces))
			for _, it := range s.Interfaces {
				po := interfaceToPO(d.ID, it)
				if prev, ok := oldMap[it.IfIndex]; ok {
					po.Kind, po.MACCount, po.Neighbor = prev.Kind, prev.MACCount, prev.Neighbor
				}
				rows = append(rows, po)
			}
			if err := tx.CreateInBatches(rows, 200).Error; err != nil {
				return err
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
