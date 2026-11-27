package data

import (
	"context"
	"errors"
	"strings"
	"time"

	"hoptrace/internal/biz"

	"gorm.io/gorm"
)

// SitePO 机房/办公区持久化对象。
type SitePO struct {
	ID          uint       `gorm:"primaryKey"`
	Name        string     `gorm:"size:100;uniqueIndex"`
	Code        string     `gorm:"size:50;index"`
	Type        string     `gorm:"size:30;index"`
	Address     string     `gorm:"size:300"`
	Contact     string     `gorm:"size:120"`
	CIDRs       StringList `gorm:"column:cidrs;type:text"`
	Description string     `gorm:"size:500"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (SitePO) TableName() string { return "sites" }

func siteToPO(s *biz.Site) *SitePO {
	return &SitePO{
		ID:          s.ID,
		Name:        s.Name,
		Code:        s.Code,
		Type:        s.Type,
		Address:     s.Address,
		Contact:     s.Contact,
		CIDRs:       StringList(s.CIDRs),
		Description: s.Description,
		CreatedAt:   s.CreatedAt,
	}
}

func siteToBiz(p *SitePO) *biz.Site {
	return &biz.Site{
		ID:          p.ID,
		Name:        p.Name,
		Code:        p.Code,
		Type:        p.Type,
		Address:     p.Address,
		Contact:     p.Contact,
		CIDRs:       []string(p.CIDRs),
		Description: p.Description,
		CreatedAt:   p.CreatedAt,
	}
}

type siteRepo struct {
	data *Data
}

func NewSiteRepo(d *Data) biz.SiteRepo {
	return &siteRepo{data: d}
}

func (r *siteRepo) fillStats(ctx context.Context, sites []*biz.Site) {
	if len(sites) == 0 {
		return
	}
	names := make([]string, 0, len(sites))
	for _, s := range sites {
		names = append(names, s.Name)
	}
	var devs []DevicePO
	_ = r.data.db.WithContext(ctx).Select("id, site, status").Where("site IN ?", names).Find(&devs).Error
	devCount := map[string]int{}
	onlineCount := map[string]int{}
	devSite := map[uint]string{}
	devIDs := make([]uint, 0, len(devs))
	for _, d := range devs {
		devCount[d.Site]++
		if d.Status == biz.StatusOnline {
			onlineCount[d.Site]++
		}
		devSite[d.ID] = d.Site
		devIDs = append(devIDs, d.ID)
	}

	epCount := map[string]int{}
	if len(devIDs) > 0 {
		type row struct {
			DeviceID uint
			Cnt      int
		}
		var rows []row
		_ = r.data.db.WithContext(ctx).Model(&EndpointPO{}).
			Select("device_id, count(*) as cnt").
			Where("device_id IN ?", devIDs).
			Group("device_id").
			Scan(&rows).Error
		for _, x := range rows {
			if sn, ok := devSite[x.DeviceID]; ok {
				epCount[sn] += x.Cnt
			}
		}
	}

	for _, s := range sites {
		s.DeviceCount = devCount[s.Name]
		s.OnlineDeviceCount = onlineCount[s.Name]
		s.EndpointCount = epCount[s.Name]
	}
}

func (r *siteRepo) List(ctx context.Context, f biz.SiteFilter) ([]*biz.Site, error) {
	q := r.data.db.WithContext(ctx).Model(&SitePO{})
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name LIKE ? OR code LIKE ? OR address LIKE ? OR contact LIKE ? OR description LIKE ?", like, like, like, like, like)
	}
	if t := strings.TrimSpace(f.Type); t != "" {
		q = q.Where("type = ?", t)
	}
	var pos []SitePO
	if err := q.Order("id asc").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*biz.Site, 0, len(pos))
	for i := range pos {
		out = append(out, siteToBiz(&pos[i]))
	}
	r.fillStats(ctx, out)
	return out, nil
}

func (r *siteRepo) Get(ctx context.Context, id uint) (*biz.Site, error) {
	var po SitePO
	err := r.data.db.WithContext(ctx).First(&po, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, biz.ErrSiteNotFound
	}
	if err != nil {
		return nil, err
	}
	s := siteToBiz(&po)
	r.fillStats(ctx, []*biz.Site{s})
	return s, nil
}

func (r *siteRepo) FindByName(ctx context.Context, name string) (*biz.Site, error) {
	var po SitePO
	err := r.data.db.WithContext(ctx).Where("name = ?", name).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return siteToBiz(&po), nil
}

func (r *siteRepo) Create(ctx context.Context, s *biz.Site) (*biz.Site, error) {
	po := siteToPO(s)
	if err := r.data.db.WithContext(ctx).Create(po).Error; err != nil {
		return nil, err
	}
	out := siteToBiz(po)
	r.fillStats(ctx, []*biz.Site{out})
	return out, nil
}

func (r *siteRepo) Update(ctx context.Context, s *biz.Site, oldName string) (*biz.Site, error) {
	err := r.data.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&SitePO{}).Where("id = ?", s.ID).Updates(map[string]any{
			"name":        s.Name,
			"code":        s.Code,
			"type":        s.Type,
			"address":     s.Address,
			"contact":     s.Contact,
			"cidrs":       StringList(s.CIDRs),
			"description": s.Description,
		}).Error; err != nil {
			return err
		}
		// 若站点改名，同步更新已归属设备的 site 字段
		if oldName != "" && oldName != s.Name {
			if err := tx.Model(&DevicePO{}).Where("site = ?", oldName).Update("site", s.Name).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, s.ID)
}

func (r *siteRepo) Delete(ctx context.Context, id uint) error {
	res := r.data.db.WithContext(ctx).Delete(&SitePO{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biz.ErrSiteNotFound
	}
	return nil
}

func (r *siteRepo) AssignDevices(ctx context.Context, siteName string, deviceIDs []uint) error {
	if len(deviceIDs) == 0 {
		return nil
	}
	return r.data.db.WithContext(ctx).Model(&DevicePO{}).Where("id IN ?", deviceIDs).Update("site", siteName).Error
}
