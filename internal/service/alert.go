package service

import (
	"context"

	v1 "hoptrace/api/hoptrace/v1"
	"hoptrace/internal/biz"

	"google.golang.org/protobuf/types/known/emptypb"
)

type AlertService struct {
	v1.UnimplementedAlertServiceServer
	uc *biz.AlertUsecase
}

func NewAlertService(uc *biz.AlertUsecase) *AlertService { return &AlertService{uc: uc} }

func (s *AlertService) ListAlerts(ctx context.Context, req *v1.ListAlertsRequest) (*v1.ListAlertsReply, error) {
	items, total, err := s.uc.ListAlerts(ctx, biz.AlertFilter{
		Status: req.GetStatus(), Severity: req.GetSeverity(), Kind: req.GetKind(),
	}, int(req.GetPage()), int(req.GetPageSize()))
	if err != nil {
		return nil, err
	}
	reply := &v1.ListAlertsReply{Total: total, Items: make([]*v1.Alert, 0, len(items))}
	for _, a := range items {
		reply.Items = append(reply.Items, toProtoAlert(a))
	}
	return reply, nil
}

func (s *AlertService) AckAlert(ctx context.Context, req *v1.AckAlertRequest) (*v1.Alert, error) {
	a, err := s.uc.Ack(ctx, uint(req.GetId()))
	if err != nil {
		return nil, err
	}
	return toProtoAlert(a), nil
}

func (s *AlertService) ResolveAlert(ctx context.Context, req *v1.ResolveAlertRequest) (*v1.Alert, error) {
	a, err := s.uc.Resolve(ctx, uint(req.GetId()))
	if err != nil {
		return nil, err
	}
	return toProtoAlert(a), nil
}

func (s *AlertService) ListChannels(ctx context.Context, _ *emptypb.Empty) (*v1.ListChannelsReply, error) {
	items, err := s.uc.ListChannels(ctx)
	if err != nil {
		return nil, err
	}
	reply := &v1.ListChannelsReply{Items: make([]*v1.AlertChannel, 0, len(items))}
	for _, c := range items {
		reply.Items = append(reply.Items, toProtoChannel(c))
	}
	return reply, nil
}

func (s *AlertService) SaveChannel(ctx context.Context, req *v1.SaveChannelRequest) (*v1.AlertChannel, error) {
	c, err := s.uc.SaveChannel(ctx, &biz.AlertChannel{
		ID: uint(req.GetId()), Name: req.GetName(), Kind: req.GetKind(),
		WebhookURL: req.GetWebhookUrl(), Secret: req.GetSecret(), Enabled: req.GetEnabled(), Kinds: req.GetKinds(),
	})
	if err != nil {
		return nil, err
	}
	return toProtoChannel(c), nil
}

func (s *AlertService) DeleteChannel(ctx context.Context, req *v1.DeleteChannelRequest) (*emptypb.Empty, error) {
	if err := s.uc.DeleteChannel(ctx, uint(req.GetId())); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *AlertService) TestChannel(ctx context.Context, req *v1.TestChannelRequest) (*v1.TestChannelReply, error) {
	ok, detail, err := s.uc.TestChannel(ctx, uint(req.GetId()))
	if err != nil {
		return nil, err
	}
	return &v1.TestChannelReply{Ok: ok, Detail: detail}, nil
}

func (s *AlertService) ListSyslogs(ctx context.Context, req *v1.ListSyslogsRequest) (*v1.ListSyslogsReply, error) {
	items, total, err := s.uc.ListSyslogs(ctx, biz.SyslogFilter{
		Keyword: req.GetKeyword(), SourceIP: req.GetSourceIp(), MaxSeverity: int(req.GetMaxSeverity()),
	}, int(req.GetPage()), int(req.GetPageSize()))
	if err != nil {
		return nil, err
	}
	reply := &v1.ListSyslogsReply{Total: total, Items: make([]*v1.SyslogEntry, 0, len(items))}
	for _, e := range items {
		reply.Items = append(reply.Items, &v1.SyslogEntry{
			Id: int64(e.ID), SourceIp: e.SourceIP, DeviceId: int64(e.DeviceID), DeviceName: e.DeviceName,
			Severity: int32(e.Severity), Facility: e.Facility, Tag: e.Tag, Message: e.Message,
			ReceivedAt: e.ReceivedAt.Unix(),
		})
	}
	return reply, nil
}

func toProtoAlert(a *biz.Alert) *v1.Alert {
	return &v1.Alert{
		Id: int64(a.ID), Kind: a.Kind, Severity: a.Severity, Title: a.Title, Message: a.Message,
		TargetKey: a.TargetKey, DeviceId: int64(a.DeviceID), EndpointId: int64(a.EndpointID),
		Status: a.Status, AckedBy: a.AckedBy, CreatedAt: a.CreatedAt.Unix(), UpdatedAt: a.UpdatedAt.Unix(),
	}
}

func toProtoChannel(c *biz.AlertChannel) *v1.AlertChannel {
	return &v1.AlertChannel{
		Id: int64(c.ID), Name: c.Name, Kind: c.Kind, WebhookUrl: c.WebhookURL,
		Enabled: c.Enabled, Kinds: c.Kinds, CreatedAt: c.CreatedAt.Unix(),
	}
}
