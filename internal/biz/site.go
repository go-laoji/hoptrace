package biz

import (
	"context"
	"strings"
	"time"
)

// 机房/办公区类型
const (
	SiteTypeDatacenter = "datacenter" // 机房
	SiteTypeOffice     = "office"     // 办公区
	SiteTypeBranch     = "branch"     // 分支机构
	SiteTypeCampus     = "campus"     // 园区
)

// Site 机房/办公区领域对象(DO)。
type Site struct {
	ID                uint
	Name              string
	Code              string
	Type              string
	Address           string
	Contact           string
	CIDRs             []string
	Description       string
	DeviceCount       int
	OnlineDeviceCount int
	EndpointCount     int
	CreatedAt         time.Time
}

// SiteFilter 机房/办公区查询条件。
type SiteFilter struct {
	Keyword string
	Type    string
}

// SiteRepo 机房/办公区仓储接口。
type SiteRepo interface {
	List(ctx context.Context, f SiteFilter) ([]*Site, error)
	Get(ctx context.Context, id uint) (*Site, error)
	FindByName(ctx context.Context, name string) (*Site, error)
	Create(ctx context.Context, s *Site) (*Site, error)
	Update(ctx context.Context, s *Site, oldName string) (*Site, error)
	Delete(ctx context.Context, id uint) error
	AssignDevices(ctx context.Context, siteName string, deviceIDs []uint) error
}

// SiteUsecase 机房/办公区业务用例。
type SiteUsecase struct {
	repo SiteRepo
}

func NewSiteUsecase(repo SiteRepo) *SiteUsecase {
	return &SiteUsecase{repo: repo}
}

func normalizeSiteType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case SiteTypeDatacenter, SiteTypeOffice, SiteTypeBranch, SiteTypeCampus:
		return strings.ToLower(strings.TrimSpace(t))
	default:
		return SiteTypeOffice
	}
}

func cleanCIDRs(cidrs []string) []string {
	out := make([]string, 0, len(cidrs))
	seen := map[string]bool{}
	for _, c := range cidrs {
		c = strings.TrimSpace(c)
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
}

func (uc *SiteUsecase) List(ctx context.Context, f SiteFilter) ([]*Site, error) {
	return uc.repo.List(ctx, f)
}

func (uc *SiteUsecase) Get(ctx context.Context, id uint) (*Site, error) {
	return uc.repo.Get(ctx, id)
}

func (uc *SiteUsecase) Create(ctx context.Context, s *Site) (*Site, error) {
	s.Name = strings.TrimSpace(s.Name)
	if s.Name == "" {
		return nil, ErrSiteInvalid
	}
	if exist, err := uc.repo.FindByName(ctx, s.Name); err != nil {
		return nil, err
	} else if exist != nil {
		return nil, ErrSiteAlreadyExists
	}
	s.Code = strings.TrimSpace(s.Code)
	s.Type = normalizeSiteType(s.Type)
	s.Address = strings.TrimSpace(s.Address)
	s.Contact = strings.TrimSpace(s.Contact)
	s.Description = strings.TrimSpace(s.Description)
	s.CIDRs = cleanCIDRs(s.CIDRs)
	return uc.repo.Create(ctx, s)
}

func (uc *SiteUsecase) Update(ctx context.Context, s *Site) (*Site, error) {
	cur, err := uc.repo.Get(ctx, s.ID)
	if err != nil {
		return nil, err
	}
	oldName := cur.Name
	name := strings.TrimSpace(s.Name)
	if name == "" {
		return nil, ErrSiteInvalid
	}
	if name != oldName {
		if exist, err := uc.repo.FindByName(ctx, name); err != nil {
			return nil, err
		} else if exist != nil && exist.ID != cur.ID {
			return nil, ErrSiteAlreadyExists
		}
	}
	cur.Name = name
	cur.Code = strings.TrimSpace(s.Code)
	cur.Type = normalizeSiteType(s.Type)
	cur.Address = strings.TrimSpace(s.Address)
	cur.Contact = strings.TrimSpace(s.Contact)
	cur.Description = strings.TrimSpace(s.Description)
	cur.CIDRs = cleanCIDRs(s.CIDRs)
	return uc.repo.Update(ctx, cur, oldName)
}

func (uc *SiteUsecase) Delete(ctx context.Context, id uint) error {
	cur, err := uc.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if cur.DeviceCount > 0 {
		return ErrSiteInUse
	}
	return uc.repo.Delete(ctx, id)
}

func (uc *SiteUsecase) AssignDevices(ctx context.Context, id uint, deviceIDs []uint) error {
	var siteName string
	if id > 0 {
		cur, err := uc.repo.Get(ctx, id)
		if err != nil {
			return err
		}
		siteName = cur.Name
	}
	return uc.repo.AssignDevices(ctx, siteName, deviceIDs)
}
