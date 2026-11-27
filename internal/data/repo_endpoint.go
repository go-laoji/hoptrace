package data

import (
	"context"
	"errors"
	"strings"
	"time"

	"hoptrace/internal/biz"

	"gorm.io/gorm"
)

type endpointRepo struct {
	data *Data
}

func NewEndpointRepo(d *Data) biz.EndpointRepo {
	return &endpointRepo{data: d}
}

func (r *endpointRepo) deviceNameMap(ctx context.Context) map[uint]string {
	var rows []struct {
		ID     uint
		Name   string
		MgmtIP string
	}
	_ = r.data.db.WithContext(ctx).Model(&DevicePO{}).Select("id, name, mgmt_ip").Scan(&rows).Error
	m := make(map[uint]string, len(rows))
	for _, x := range rows {
		if x.Name != "" {
			m[x.ID] = x.Name
		} else {
			m[x.ID] = x.MgmtIP
		}
	}
	return m
}

func (r *endpointRepo) List(ctx context.Context, f biz.EndpointFilter, page, size int) ([]*biz.Endpoint, int64, error) {
	q := r.data.db.WithContext(ctx).Model(&EndpointPO{})
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("mac LIKE ? OR ip LIKE ? OR name LIKE ? OR vendor LIKE ? OR owner LIKE ? OR tags LIKE ?",
			like, like, like, like, like, like)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.Trust != "" {
		q = q.Where("trust = ?", f.Trust)
	}
	if f.Type != "" {
		q = q.Where("type = ?", f.Type)
	}
	if f.DeviceID != 0 {
		q = q.Where("device_id = ?", f.DeviceID)
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
	var pos []EndpointPO
	if err := q.Order("status desc, last_seen desc").Offset((page - 1) * size).Limit(size).Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	names := r.deviceNameMap(ctx)
	out := make([]*biz.Endpoint, 0, len(pos))
	for i := range pos {
		e := endpointToBiz(&pos[i])
		e.DeviceName = names[e.DeviceID]
		out = append(out, e)
	}
	return out, total, nil
}

func (r *endpointRepo) ListAll(ctx context.Context) ([]*biz.Endpoint, error) {
	var pos []EndpointPO
	if err := r.data.db.WithContext(ctx).Order("id asc").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*biz.Endpoint, 0, len(pos))
	for i := range pos {
		out = append(out, endpointToBiz(&pos[i]))
	}
	return out, nil
}

func (r *endpointRepo) ListByDevices(ctx context.Context, deviceIDs []uint, onlineOnly bool) ([]*biz.Endpoint, error) {
	if len(deviceIDs) == 0 {
		return nil, nil
	}
	q := r.data.db.WithContext(ctx).Where("device_id IN ?", deviceIDs)
	if onlineOnly {
		q = q.Where("status = ?", biz.EndpointOnline)
	}
	var pos []EndpointPO
	if err := q.Order("if_index asc, id asc").Find(&pos).Error; err != nil {
		return nil, err
	}
	names := r.deviceNameMap(ctx)
	out := make([]*biz.Endpoint, 0, len(pos))
	for i := range pos {
		e := endpointToBiz(&pos[i])
		e.DeviceName = names[e.DeviceID]
		out = append(out, e)
	}
	return out, nil
}

func (r *endpointRepo) Get(ctx context.Context, id uint) (*biz.Endpoint, error) {
	var po EndpointPO
	err := r.data.db.WithContext(ctx).First(&po, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, biz.ErrEndpointNotFound
	}
	if err != nil {
		return nil, err
	}
	e := endpointToBiz(&po)
	e.DeviceName = r.deviceNameMap(ctx)[e.DeviceID]
	return e, nil
}

func (r *endpointRepo) Update(ctx context.Context, e *biz.Endpoint) error {
	return r.data.db.WithContext(ctx).Save(endpointToPO(e)).Error
}

func (r *endpointRepo) Delete(ctx context.Context, id uint) error {
	return r.data.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("endpoint_id = ?", id).Delete(&EndpointLocationPO{}).Error; err != nil {
			return err
		}
		if err := tx.Where("endpoint_id = ?", id).Delete(&EndpointEventPO{}).Error; err != nil {
			return err
		}
		return tx.Delete(&EndpointPO{}, id).Error
	})
}

func (r *endpointRepo) CountByDevice(ctx context.Context) (map[uint]int, error) {
	type row struct {
		DeviceID uint
		Cnt      int
	}
	var rows []row
	err := r.data.db.WithContext(ctx).Model(&EndpointPO{}).
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

func (r *endpointRepo) ListFdb(ctx context.Context) ([]biz.FdbEntry, error) {
	var pos []FdbPO
	if err := r.data.db.WithContext(ctx).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]biz.FdbEntry, 0, len(pos))
	for _, p := range pos {
		out = append(out, biz.FdbEntry{DeviceID: p.DeviceID, MAC: p.MAC, VLAN: p.VLAN, IfIndex: p.IfIndex})
	}
	return out, nil
}

func (r *endpointRepo) ListArp(ctx context.Context) ([]biz.ArpEntry, error) {
	var pos []ArpPO
	if err := r.data.db.WithContext(ctx).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]biz.ArpEntry, 0, len(pos))
	for _, p := range pos {
		out = append(out, biz.ArpEntry{DeviceID: p.DeviceID, IP: p.IP, MAC: p.MAC, IfIndex: p.IfIndex})
	}
	return out, nil
}

func (r *endpointRepo) ApplyChanges(ctx context.Context, changes []*biz.EndpointChange, now time.Time) error {
	return r.data.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, ch := range changes {
			po := endpointToPO(ch.Endpoint)
			if ch.IsNew {
				if err := tx.Create(po).Error; err != nil {
					return err
				}
				ch.Endpoint.ID = po.ID
			} else {
				if err := tx.Save(po).Error; err != nil {
					return err
				}
			}
			if ch.LocationChanged {
				if err := tx.Model(&EndpointLocationPO{}).
					Where("endpoint_id = ? AND until IS NULL", po.ID).
					Update("until", now).Error; err != nil {
					return err
				}
				loc := &EndpointLocationPO{
					EndpointID: po.ID, DeviceID: po.DeviceID, IfIndex: po.IfIndex,
					IfName: po.IfName, VLAN: po.VLAN, Since: now,
				}
				if err := tx.Create(loc).Error; err != nil {
					return err
				}
			}
			for _, ev := range ch.Events {
				row := &EndpointEventPO{
					EndpointID: po.ID, MAC: ev.MAC, IP: ev.IP, Name: ev.Name,
					Type: ev.Type, Detail: ev.Detail, At: ev.At,
				}
				if err := tx.Create(row).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *endpointRepo) Locations(ctx context.Context, endpointID uint) ([]*biz.EndpointLocation, error) {
	var pos []EndpointLocationPO
	if err := r.data.db.WithContext(ctx).Where("endpoint_id = ?", endpointID).Order("since desc").Limit(50).Find(&pos).Error; err != nil {
		return nil, err
	}
	names := r.deviceNameMap(ctx)
	out := make([]*biz.EndpointLocation, 0, len(pos))
	for _, p := range pos {
		out = append(out, &biz.EndpointLocation{
			ID: p.ID, EndpointID: p.EndpointID, DeviceID: p.DeviceID, DeviceName: names[p.DeviceID],
			IfIndex: p.IfIndex, IfName: p.IfName, VLAN: p.VLAN, Since: p.Since, Until: p.Until,
		})
	}
	return out, nil
}

func (r *endpointRepo) Events(ctx context.Context, f biz.EventFilter, page, size int) ([]*biz.EndpointEvent, int64, error) {
	q := r.data.db.WithContext(ctx).Model(&EndpointEventPO{})
	if f.EndpointID != 0 {
		q = q.Where("endpoint_id = ?", f.EndpointID)
	}
	if f.Type != "" {
		q = q.Where("type = ?", f.Type)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 200 {
		size = 50
	}
	var pos []EndpointEventPO
	if err := q.Order("at desc, id desc").Offset((page - 1) * size).Limit(size).Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*biz.EndpointEvent, 0, len(pos))
	for i := range pos {
		out = append(out, eventToBiz(&pos[i]))
	}
	return out, total, nil
}
