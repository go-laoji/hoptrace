package service

import (
	"context"

	v1 "hoptrace/api/hoptrace/v1"
	"hoptrace/internal/biz"

	"google.golang.org/protobuf/types/known/emptypb"
)

type DashboardService struct {
	v1.UnimplementedDashboardServiceServer
	uc *biz.DashboardUsecase
}

func NewDashboardService(uc *biz.DashboardUsecase) *DashboardService {
	return &DashboardService{uc: uc}
}

func (s *DashboardService) GetDashboard(ctx context.Context, _ *emptypb.Empty) (*v1.Dashboard, error) {
	st, err := s.uc.Get(ctx)
	if err != nil {
		return nil, err
	}
	reply := &v1.Dashboard{
		DevicesTotal:       st.DevicesTotal,
		DevicesOnline:      st.DevicesOnline,
		DevicesOffline:     st.DevicesOffline,
		DevicesUnmanaged:   st.DevicesUnmanaged,
		LinksTotal:         st.LinksTotal,
		LinksDown:          st.LinksDown,
		LinksStale:         st.LinksStale,
		EndpointsTotal:     st.EndpointsTotal,
		EndpointsOnline:    st.EndpointsOnline,
		EndpointsNew_24H:   st.EndpointsNew24h,
		EndpointsUntrusted: st.EndpointsUntrust,
		DevicesByRole:      make([]*v1.KV, 0, len(st.DevicesByRole)),
		EndpointsByType:    make([]*v1.KV, 0, len(st.EndpointsByType)),
		RecentEvents:       make([]*v1.EndpointEvent, 0, len(st.RecentEvents)),
	}
	for _, kv := range st.DevicesByRole {
		reply.DevicesByRole = append(reply.DevicesByRole, &v1.KV{Name: kv.Name, Value: kv.Value})
	}
	for _, kv := range st.EndpointsByType {
		reply.EndpointsByType = append(reply.EndpointsByType, &v1.KV{Name: kv.Name, Value: kv.Value})
	}
	for _, ev := range st.RecentEvents {
		reply.RecentEvents = append(reply.RecentEvents, toProtoEvent(ev))
	}
	return reply, nil
}
