package data

import (
	"context"
	"time"

	"hoptrace/internal/biz"

	"gorm.io/gorm"
)

type topologyRepo struct {
	data *Data
}

func NewTopologyRepo(d *Data) biz.TopologyRepo {
	return &topologyRepo{data: d}
}

func (r *topologyRepo) ListAllNeighbors(ctx context.Context) ([]*biz.Neighbor, error) {
	var pos []NeighborPO
	if err := r.data.db.WithContext(ctx).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*biz.Neighbor, 0, len(pos))
	for i := range pos {
		out = append(out, neighborToBiz(&pos[i]))
	}
	return out, nil
}

func (r *topologyRepo) PortMACCounts(ctx context.Context) (map[biz.IfKey]int, error) {
	type row struct {
		DeviceID uint
		IfIndex  int
		Cnt      int
	}
	var rows []row
	err := r.data.db.WithContext(ctx).Model(&FdbPO{}).
		Select("device_id, if_index, count(distinct mac) as cnt").
		Group("device_id, if_index").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	m := make(map[biz.IfKey]int, len(rows))
	for _, x := range rows {
		m[biz.IfKey{DeviceID: x.DeviceID, IfIndex: x.IfIndex}] = x.Cnt
	}
	return m, nil
}

func (r *topologyRepo) ArpCounts(ctx context.Context) (map[uint]int, error) {
	type row struct {
		DeviceID uint
		Cnt      int
	}
	var rows []row
	err := r.data.db.WithContext(ctx).Model(&ArpPO{}).
		Select("device_id, count(*) as cnt").Group("device_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	m := make(map[uint]int, len(rows))
	for _, x := range rows {
		m[x.DeviceID] = x.Cnt
	}
	return m, nil
}

func (r *topologyRepo) CreateStubs(ctx context.Context, stubs []*biz.Device) error {
	if len(stubs) == 0 {
		return nil
	}
	rows := make([]DevicePO, 0, len(stubs))
	for _, s := range stubs {
		rows = append(rows, *deviceToPO(s))
	}
	return r.data.db.WithContext(ctx).Create(&rows).Error
}

func (r *topologyRepo) DeleteDevices(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	return r.data.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("a_device_id IN ? OR b_device_id IN ?", ids, ids).Delete(&LinkPO{}).Error; err != nil {
			return err
		}
		return tx.Where("id IN ?", ids).Delete(&DevicePO{}).Error
	})
}

func (r *topologyRepo) SaveTopology(ctx context.Context, res *biz.TopoResult, now time.Time) error {
	return r.data.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 链路 upsert,未出现的旧链路标为 stale
		var existing []LinkPO
		if err := tx.Find(&existing).Error; err != nil {
			return err
		}
		byKey := make(map[string]*LinkPO, len(existing))
		for i := range existing {
			byKey[existing[i].Key] = &existing[i]
		}
		seen := make(map[string]bool, len(res.Links))
		for _, l := range res.Links {
			seen[l.Key] = true
			if old, ok := byKey[l.Key]; ok {
				old.AIfName, old.BIfName = l.AIfName, l.BIfName
				old.Confidence, old.Status, old.Members, old.LastSeen = l.Confidence, l.Status, StringList(l.Members), now
				if err := tx.Save(old).Error; err != nil {
					return err
				}
			} else {
				po := &LinkPO{
					Key: l.Key, ADeviceID: l.ADeviceID, AIfIndex: l.AIfIndex, AIfName: l.AIfName,
					BDeviceID: l.BDeviceID, BIfIndex: l.BIfIndex, BIfName: l.BIfName,
					Kind: l.Kind, Confidence: l.Confidence, Status: l.Status, Members: StringList(l.Members),
					FirstSeen: now, LastSeen: now,
				}
				if err := tx.Create(po).Error; err != nil {
					return err
				}
			}
		}
		for k, old := range byKey {
			if !seen[k] && old.Status != biz.LinkStale {
				if err := tx.Model(old).Update("status", biz.LinkStale).Error; err != nil {
					return err
				}
			}
		}

		// 2. 回写接口类别/MAC数/邻居描述
		for key, info := range res.IfInfos {
			if err := tx.Model(&InterfacePO{}).
				Where("device_id = ? AND if_index = ?", key.DeviceID, key.IfIndex).
				Updates(map[string]any{
					"kind":      info.Kind,
					"mac_count": info.MACCount,
					"neighbor":  info.Neighbor,
				}).Error; err != nil {
				return err
			}
		}

		// 3. 回写设备自动判定角色
		for devID, role := range res.Roles {
			if err := tx.Model(&DevicePO{}).
				Where("id = ? AND role_manual = ?", devID, false).
				Update("role", role).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *topologyRepo) ListLinks(ctx context.Context) ([]*biz.Link, error) {
	var pos []LinkPO
	if err := r.data.db.WithContext(ctx).Order("id asc").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*biz.Link, 0, len(pos))
	for i := range pos {
		out = append(out, linkToBiz(&pos[i]))
	}
	return out, nil
}

func (r *topologyRepo) PurgeStaleLinks(ctx context.Context, before time.Time) error {
	return r.data.db.WithContext(ctx).
		Where("status = ? AND last_seen < ?", biz.LinkStale, before).
		Delete(&LinkPO{}).Error
}
