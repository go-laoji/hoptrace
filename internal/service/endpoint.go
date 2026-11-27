package service

import (
	"context"

	v1 "hoptrace/api/hoptrace/v1"
	"hoptrace/internal/biz"

	"google.golang.org/protobuf/types/known/emptypb"
)

type EndpointService struct {
	v1.UnimplementedEndpointServiceServer
	uc *biz.EndpointUsecase
}

func NewEndpointService(uc *biz.EndpointUsecase) *EndpointService {
	return &EndpointService{uc: uc}
}

func (s *EndpointService) ListEndpoints(ctx context.Context, req *v1.ListEndpointsRequest) (*v1.ListEndpointsReply, error) {
	items, total, err := s.uc.List(ctx, biz.EndpointFilter{
		Keyword:  req.GetKeyword(),
		Status:   req.GetStatus(),
		Trust:    req.GetTrust(),
		Type:     req.GetType(),
		DeviceID: uint(req.GetDeviceId()),
	}, int(req.GetPage()), int(req.GetPageSize()))
	if err != nil {
		return nil, err
	}
	reply := &v1.ListEndpointsReply{Total: total, Items: make([]*v1.Endpoint, 0, len(items))}
	for _, e := range items {
		reply.Items = append(reply.Items, toProtoEndpoint(e))
	}
	return reply, nil
}

func (s *EndpointService) GetEndpoint(ctx context.Context, req *v1.GetEndpointRequest) (*v1.EndpointDetail, error) {
	e, locs, evs, err := s.uc.Detail(ctx, uint(req.GetId()))
	if err != nil {
		return nil, err
	}
	reply := &v1.EndpointDetail{
		Endpoint:  toProtoEndpoint(e),
		Locations: make([]*v1.EndpointLocation, 0, len(locs)),
		Events:    make([]*v1.EndpointEvent, 0, len(evs)),
	}
	for _, l := range locs {
		var until int64
		if l.Until != nil {
			until = l.Until.Unix()
		}
		reply.Locations = append(reply.Locations, &v1.EndpointLocation{
			Id:         int64(l.ID),
			DeviceId:   int64(l.DeviceID),
			DeviceName: l.DeviceName,
			IfIndex:    int32(l.IfIndex),
			IfName:     l.IfName,
			Vlan:       int32(l.VLAN),
			Since:      l.Since.Unix(),
			Until:      until,
		})
	}
	for _, ev := range evs {
		reply.Events = append(reply.Events, toProtoEvent(ev))
	}
	return reply, nil
}

func (s *EndpointService) UpdateEndpoint(ctx context.Context, req *v1.UpdateEndpointRequest) (*v1.Endpoint, error) {
	e, err := s.uc.Update(ctx, uint(req.GetId()), req.GetName(), req.GetOwner(),
		req.GetTags(), req.GetTrust(), req.GetType(), req.GetRemark())
	if err != nil {
		return nil, err
	}
	return toProtoEndpoint(e), nil
}

func (s *EndpointService) DeleteEndpoint(ctx context.Context, req *v1.DeleteEndpointRequest) (*emptypb.Empty, error) {
	if err := s.uc.Delete(ctx, uint(req.GetId())); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *EndpointService) ListEvents(ctx context.Context, req *v1.ListEventsRequest) (*v1.ListEventsReply, error) {
	items, total, err := s.uc.Events(ctx, biz.EventFilter{
		EndpointID: uint(req.GetEndpointId()),
		Type:       req.GetType(),
	}, int(req.GetPage()), int(req.GetPageSize()))
	if err != nil {
		return nil, err
	}
	reply := &v1.ListEventsReply{Total: total, Items: make([]*v1.EndpointEvent, 0, len(items))}
	for _, ev := range items {
		reply.Items = append(reply.Items, toProtoEvent(ev))
	}
	return reply, nil
}

func toProtoEndpoint(e *biz.Endpoint) *v1.Endpoint {
	return &v1.Endpoint{
		Id:         int64(e.ID),
		Mac:        e.MAC,
		Ip:         e.IP,
		Ips:        e.IPs,
		Vendor:     e.Vendor,
		Type:       e.Type,
		TypeManual: e.TypeManual,
		Name:       e.Name,
		Owner:      e.Owner,
		Tags:       e.Tags,
		Status:     e.Status,
		Trust:      e.Trust,
		DeviceId:   int64(e.DeviceID),
		DeviceName: e.DeviceName,
		IfIndex:    int32(e.IfIndex),
		IfName:     e.IfName,
		Vlan:       int32(e.VLAN),
		FirstSeen:  e.FirstSeen.Unix(),
		LastSeen:   e.LastSeen.Unix(),
		Remark:     e.Remark,
	}
}

func toProtoEvent(ev *biz.EndpointEvent) *v1.EndpointEvent {
	return &v1.EndpointEvent{
		Id:         int64(ev.ID),
		EndpointId: int64(ev.EndpointID),
		Mac:        ev.MAC,
		Ip:         ev.IP,
		Name:       ev.Name,
		Type:       ev.Type,
		Detail:     ev.Detail,
		At:         ev.At.Unix(),
	}
}
