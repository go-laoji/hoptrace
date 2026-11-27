package service

import (
	"context"

	v1 "hoptrace/api/hoptrace/v1"
	"hoptrace/internal/biz"

	"google.golang.org/protobuf/types/known/emptypb"
)

type ControlService struct {
	v1.UnimplementedControlServiceServer
	uc *biz.ControlUsecase
}

func NewControlService(uc *biz.ControlUsecase) *ControlService {
	return &ControlService{uc: uc}
}

func (s *ControlService) ListControlActions(ctx context.Context, req *v1.ListControlActionsRequest) (*v1.ListControlActionsReply, error) {
	items, total, err := s.uc.ListActions(ctx, biz.ControlFilter{
		Status:   req.GetStatus(),
		DeviceID: uint(req.GetDeviceId()),
	}, int(req.GetPage()), int(req.GetPageSize()))
	if err != nil {
		return nil, err
	}
	reply := &v1.ListControlActionsReply{Total: total, Items: make([]*v1.ControlAction, 0, len(items))}
	for _, a := range items {
		reply.Items = append(reply.Items, toProtoControlAction(a))
	}
	return reply, nil
}

func (s *ControlService) CreateControlAction(ctx context.Context, req *v1.CreateControlActionRequest) (*v1.ControlAction, error) {
	a, err := s.uc.CreateAction(ctx, biz.CreateControlInput{
		Action:     req.GetAction(),
		DeviceID:   uint(req.GetDeviceId()),
		IfIndex:    int(req.GetIfIndex()),
		EndpointID: uint(req.GetEndpointId()),
		ParamValue: req.GetParamValue(),
		Protocol:   req.GetProtocol(),
		Reason:     req.GetReason(),
		DryRunOnly: req.GetDryRunOnly(),
	})
	if err != nil {
		return nil, err
	}
	return toProtoControlAction(a), nil
}

func (s *ControlService) ApproveControlAction(ctx context.Context, req *v1.ApproveControlActionRequest) (*v1.ControlAction, error) {
	a, err := s.uc.Approve(ctx, uint(req.GetId()), req.GetAutoExecute())
	if err != nil {
		return nil, err
	}
	return toProtoControlAction(a), nil
}

func (s *ControlService) RejectControlAction(ctx context.Context, req *v1.RejectControlActionRequest) (*v1.ControlAction, error) {
	a, err := s.uc.Reject(ctx, uint(req.GetId()), req.GetReason())
	if err != nil {
		return nil, err
	}
	return toProtoControlAction(a), nil
}

func (s *ControlService) ExecuteControlAction(ctx context.Context, req *v1.ExecuteControlActionRequest) (*v1.ControlAction, error) {
	a, err := s.uc.Execute(ctx, uint(req.GetId()))
	if err != nil {
		return nil, err
	}
	return toProtoControlAction(a), nil
}

func (s *ControlService) RollbackControlAction(ctx context.Context, req *v1.RollbackControlActionRequest) (*v1.ControlAction, error) {
	a, err := s.uc.Rollback(ctx, uint(req.GetId()))
	if err != nil {
		return nil, err
	}
	return toProtoControlAction(a), nil
}

func (s *ControlService) ListWriteCredentials(ctx context.Context, _ *emptypb.Empty) (*v1.ListWriteCredentialsReply, error) {
	items, err := s.uc.ListWriteCredentials(ctx)
	if err != nil {
		return nil, err
	}
	reply := &v1.ListWriteCredentialsReply{Items: make([]*v1.WriteCredential, 0, len(items))}
	for _, c := range items {
		reply.Items = append(reply.Items, toProtoWriteCredential(c))
	}
	return reply, nil
}

func (s *ControlService) SaveWriteCredential(ctx context.Context, req *v1.SaveWriteCredentialRequest) (*v1.WriteCredential, error) {
	c, err := s.uc.SaveWriteCredential(ctx, &biz.WriteCredential{
		ID:            uint(req.GetId()),
		Name:          req.GetName(),
		Protocol:      req.GetProtocol(),
		Username:      req.GetUsername(),
		Password:      req.GetPassword(),
		Port:          int(req.GetPort()),
		NACWebhookURL: req.GetNacWebhookUrl(),
	})
	if err != nil {
		return nil, err
	}
	return toProtoWriteCredential(c), nil
}

func (s *ControlService) DeleteWriteCredential(ctx context.Context, req *v1.DeleteWriteCredentialRequest) (*emptypb.Empty, error) {
	if err := s.uc.DeleteWriteCredential(ctx, uint(req.GetId())); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func toProtoControlAction(a *biz.ControlAction) *v1.ControlAction {
	var execAt, rbAt int64
	if a.ExecutedAt != nil {
		execAt = a.ExecutedAt.Unix()
	}
	if a.RolledBackAt != nil {
		rbAt = a.RolledBackAt.Unix()
	}
	return &v1.ControlAction{
		Id:              int64(a.ID),
		Action:          a.Action,
		DeviceId:        int64(a.DeviceID),
		DeviceName:      a.DeviceName,
		IfIndex:         int32(a.IfIndex),
		IfName:          a.IfName,
		EndpointId:      int64(a.EndpointID),
		EndpointMac:     a.EndpointMAC,
		ParamValue:      a.ParamValue,
		PrevValue:       a.PrevValue,
		Protocol:        a.Protocol,
		DryRunPreview:   a.DryRunPreview,
		RollbackPreview: a.RollbackPreview,
		Status:          a.Status,
		Reason:          a.Reason,
		RequestedBy:     a.RequestedBy,
		ApprovedBy:      a.ApprovedBy,
		ResultLog:       a.ResultLog,
		CreatedAt:       a.CreatedAt.Unix(),
		ExecutedAt:      execAt,
		RolledBackAt:    rbAt,
	}
}

func toProtoWriteCredential(c *biz.WriteCredential) *v1.WriteCredential {
	return &v1.WriteCredential{
		Id:            int64(c.ID),
		Name:          c.Name,
		Protocol:      c.Protocol,
		Username:      c.Username,
		Port:          int32(c.Port),
		HasPassword:   c.Password != "",
		NacWebhookUrl: c.NACWebhookURL,
		CreatedAt:     c.CreatedAt.Unix(),
	}
}
