package service

import (
	"context"

	v1 "hoptrace/api/hoptrace/v1"
	"hoptrace/internal/biz"

	"google.golang.org/protobuf/types/known/emptypb"
)

type CredentialService struct {
	v1.UnimplementedCredentialServiceServer
	uc *biz.CredentialUsecase
}

func NewCredentialService(uc *biz.CredentialUsecase) *CredentialService {
	return &CredentialService{uc: uc}
}

func (s *CredentialService) ListCredentials(ctx context.Context, _ *emptypb.Empty) (*v1.ListCredentialsReply, error) {
	items, err := s.uc.List(ctx)
	if err != nil {
		return nil, err
	}
	reply := &v1.ListCredentialsReply{Items: make([]*v1.Credential, 0, len(items))}
	for _, c := range items {
		reply.Items = append(reply.Items, toProtoCredential(c))
	}
	return reply, nil
}

func (s *CredentialService) CreateCredential(ctx context.Context, req *v1.SaveCredentialRequest) (*v1.Credential, error) {
	c, err := s.uc.Save(ctx, convertCredential(req, 0))
	if err != nil {
		return nil, err
	}
	return toProtoCredential(c), nil
}

func (s *CredentialService) UpdateCredential(ctx context.Context, req *v1.SaveCredentialRequest) (*v1.Credential, error) {
	c, err := s.uc.Save(ctx, convertCredential(req, uint(req.GetId())))
	if err != nil {
		return nil, err
	}
	return toProtoCredential(c), nil
}

func (s *CredentialService) DeleteCredential(ctx context.Context, req *v1.DeleteCredentialRequest) (*emptypb.Empty, error) {
	if err := s.uc.Delete(ctx, uint(req.GetId())); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func convertCredential(req *v1.SaveCredentialRequest, id uint) *biz.Credential {
	return &biz.Credential{
		ID:           id,
		Name:         req.GetName(),
		Version:      req.GetVersion(),
		Community:    req.GetCommunity(),
		Username:     req.GetUsername(),
		AuthProtocol: req.GetAuthProtocol(),
		AuthPassword: req.GetAuthPassword(),
		PrivProtocol: req.GetPrivProtocol(),
		PrivPassword: req.GetPrivPassword(),
		Port:         int(req.GetPort()),
	}
}

func toProtoCredential(c *biz.Credential) *v1.Credential {
	return &v1.Credential{
		Id:              int64(c.ID),
		Name:            c.Name,
		Version:         c.Version,
		Username:        c.Username,
		AuthProtocol:    c.AuthProtocol,
		PrivProtocol:    c.PrivProtocol,
		Port:            int32(c.Port),
		HasCommunity:    c.Community != "",
		HasAuthPassword: c.AuthPassword != "",
		HasPrivPassword: c.PrivPassword != "",
		CreatedAt:       c.CreatedAt.Unix(),
	}
}
