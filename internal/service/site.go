package service

import (
	"context"

	v1 "hoptrace/api/hoptrace/v1"
	"hoptrace/internal/biz"

	"google.golang.org/protobuf/types/known/emptypb"
)

type SiteService struct {
	v1.UnimplementedSiteServiceServer
	uc *biz.SiteUsecase
}

func NewSiteService(uc *biz.SiteUsecase) *SiteService {
	return &SiteService{uc: uc}
}

func (s *SiteService) ListSites(ctx context.Context, req *v1.ListSitesRequest) (*v1.ListSitesReply, error) {
	items, err := s.uc.List(ctx, biz.SiteFilter{
		Keyword: req.GetKeyword(),
		Type:    req.GetType(),
	})
	if err != nil {
		return nil, err
	}
	reply := &v1.ListSitesReply{Items: make([]*v1.Site, 0, len(items))}
	for _, it := range items {
		reply.Items = append(reply.Items, toProtoSite(it))
	}
	return reply, nil
}

func (s *SiteService) GetSite(ctx context.Context, req *v1.GetSiteRequest) (*v1.Site, error) {
	it, err := s.uc.Get(ctx, uint(req.GetId()))
	if err != nil {
		return nil, err
	}
	return toProtoSite(it), nil
}

func (s *SiteService) CreateSite(ctx context.Context, req *v1.CreateSiteRequest) (*v1.Site, error) {
	it, err := s.uc.Create(ctx, &biz.Site{
		Name:        req.GetName(),
		Code:        req.GetCode(),
		Type:        req.GetType(),
		Address:     req.GetAddress(),
		Contact:     req.GetContact(),
		CIDRs:       req.GetCidrs(),
		Description: req.GetDescription(),
	})
	if err != nil {
		return nil, err
	}
	return toProtoSite(it), nil
}

func (s *SiteService) UpdateSite(ctx context.Context, req *v1.UpdateSiteRequest) (*v1.Site, error) {
	it, err := s.uc.Update(ctx, &biz.Site{
		ID:          uint(req.GetId()),
		Name:        req.GetName(),
		Code:        req.GetCode(),
		Type:        req.GetType(),
		Address:     req.GetAddress(),
		Contact:     req.GetContact(),
		CIDRs:       req.GetCidrs(),
		Description: req.GetDescription(),
	})
	if err != nil {
		return nil, err
	}
	return toProtoSite(it), nil
}

func (s *SiteService) DeleteSite(ctx context.Context, req *v1.DeleteSiteRequest) (*emptypb.Empty, error) {
	if err := s.uc.Delete(ctx, uint(req.GetId())); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *SiteService) AssignDevices(ctx context.Context, req *v1.AssignSiteDevicesRequest) (*emptypb.Empty, error) {
	ids := make([]uint, 0, len(req.GetDeviceIds()))
	for _, id := range req.GetDeviceIds() {
		if id > 0 {
			ids = append(ids, uint(id))
		}
	}
	if err := s.uc.AssignDevices(ctx, uint(req.GetId()), ids); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func toProtoSite(it *biz.Site) *v1.Site {
	return &v1.Site{
		Id:                int64(it.ID),
		Name:              it.Name,
		Code:              it.Code,
		Type:              it.Type,
		Address:           it.Address,
		Contact:           it.Contact,
		Cidrs:             it.CIDRs,
		Description:       it.Description,
		DeviceCount:       int32(it.DeviceCount),
		OnlineDeviceCount: int32(it.OnlineDeviceCount),
		EndpointCount:     int32(it.EndpointCount),
		CreatedAt:         it.CreatedAt.Unix(),
	}
}
