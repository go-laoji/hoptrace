package data

import (
	"context"
	"time"

	"hoptrace/internal/biz"
)

type statsRepo struct {
	data *Data
}

func NewStatsRepo(d *Data) biz.StatsRepo {
	return &statsRepo{data: d}
}

func (r *statsRepo) Stats(ctx context.Context, now time.Time) (*biz.Stats, error) {
	db := r.data.db.WithContext(ctx)
	s := &biz.Stats{}

	db.Model(&DevicePO{}).Count(&s.DevicesTotal)
	db.Model(&DevicePO{}).Where("managed = ? AND status = ?", true, biz.StatusOnline).Count(&s.DevicesOnline)
	db.Model(&DevicePO{}).Where("managed = ? AND status = ?", true, biz.StatusOffline).Count(&s.DevicesOffline)
	db.Model(&DevicePO{}).Where("managed = ?", false).Count(&s.DevicesUnmanaged)

	db.Model(&LinkPO{}).Count(&s.LinksTotal)
	db.Model(&LinkPO{}).Where("status = ?", biz.LinkDown).Count(&s.LinksDown)
	db.Model(&LinkPO{}).Where("status = ?", biz.LinkStale).Count(&s.LinksStale)

	db.Model(&EndpointPO{}).Count(&s.EndpointsTotal)
	db.Model(&EndpointPO{}).Where("status = ?", biz.EndpointOnline).Count(&s.EndpointsOnline)
	db.Model(&EndpointPO{}).Where("first_seen >= ?", now.Add(-24*time.Hour)).Count(&s.EndpointsNew24h)
	db.Model(&EndpointPO{}).Where("trust = ?", biz.TrustUnknown).Count(&s.EndpointsUntrust)

	type kvRow struct {
		Name  string
		Value int64
	}
	var roles []kvRow
	db.Model(&DevicePO{}).Select("role as name, count(*) as value").Group("role").Scan(&roles)
	for _, x := range roles {
		s.DevicesByRole = append(s.DevicesByRole, biz.KV{Name: x.Name, Value: x.Value})
	}

	var types []kvRow
	db.Model(&EndpointPO{}).Select("type as name, count(*) as value").Group("type").Scan(&types)
	for _, x := range types {
		s.EndpointsByType = append(s.EndpointsByType, biz.KV{Name: x.Name, Value: x.Value})
	}

	var evs []EndpointEventPO
	db.Order("at desc, id desc").Limit(15).Find(&evs)
	for i := range evs {
		s.RecentEvents = append(s.RecentEvents, eventToBiz(&evs[i]))
	}
	return s, nil
}
