package service

import (
	"context"

	v1 "hoptrace/api/hoptrace/v1"
	"hoptrace/internal/biz"

	"google.golang.org/protobuf/types/known/emptypb"
)

type AuthService struct {
	v1.UnimplementedAuthServiceServer
	uc *biz.AuthUsecase
}

func NewAuthService(uc *biz.AuthUsecase) *AuthService {
	return &AuthService{uc: uc}
}

func (s *AuthService) Login(ctx context.Context, req *v1.LoginRequest) (*v1.LoginReply, error) {
	tok, exp, u, err := s.uc.Login(ctx, req.GetUsername(), req.GetPassword())
	if err != nil {
		return nil, err
	}
	return &v1.LoginReply{Token: tok, ExpiresAt: exp, User: toProtoUser(u)}, nil
}

func (s *AuthService) CurrentUser(ctx context.Context, _ *emptypb.Empty) (*v1.CurrentUserReply, error) {
	u, enabled, err := s.uc.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	reply := &v1.CurrentUserReply{AuthEnabled: enabled}
	if u != nil {
		reply.User = toProtoUser(u)
	}
	return reply, nil
}

func (s *AuthService) ChangePassword(ctx context.Context, req *v1.ChangePasswordRequest) (*emptypb.Empty, error) {
	if err := s.uc.ChangePassword(ctx, req.GetOldPassword(), req.GetNewPassword()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *AuthService) ListUsers(ctx context.Context, _ *emptypb.Empty) (*v1.ListUsersReply, error) {
	users, err := s.uc.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	reply := &v1.ListUsersReply{Items: make([]*v1.User, 0, len(users))}
	for _, u := range users {
		reply.Items = append(reply.Items, toProtoUser(u))
	}
	return reply, nil
}

func (s *AuthService) CreateUser(ctx context.Context, req *v1.CreateUserRequest) (*v1.User, error) {
	u, err := s.uc.CreateUser(ctx, req.GetUsername(), req.GetPassword(), req.GetDisplayName(), req.GetRole())
	if err != nil {
		return nil, err
	}
	return toProtoUser(u), nil
}

func (s *AuthService) DeleteUser(ctx context.Context, req *v1.DeleteUserRequest) (*emptypb.Empty, error) {
	if err := s.uc.DeleteUser(ctx, uint(req.GetId())); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func toProtoUser(u *biz.User) *v1.User {
	return &v1.User{
		Id:          int64(u.ID),
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Role:        u.Role,
		CreatedAt:   u.CreatedAt.Unix(),
	}
}
