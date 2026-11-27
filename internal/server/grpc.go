package server

import (
	v1 "hoptrace/api/hoptrace/v1"
	"hoptrace/internal/biz"
	"hoptrace/internal/conf"
	"hoptrace/internal/service"

	"github.com/go-kratos/kratos/v3/middleware/recovery"
	"github.com/go-kratos/kratos/v3/transport/grpc"
)

// NewGRPCServer new a gRPC server.
func NewGRPCServer(
	c *conf.Server,
	authUC *biz.AuthUsecase,
	dev *service.DeviceService,
	cred *service.CredentialService,
	disc *service.DiscoveryService,
	topo *service.TopologyService,
	ep *service.EndpointService,
	dash *service.DashboardService,
	auth *service.AuthService,
	ctrl *service.ControlService,
) *grpc.Server {
	opts := []grpc.ServerOption{
		grpc.Middleware(
			recovery.Recovery(),
			AuthMiddleware(authUC),
		),
	}
	if c.GetGrpc().GetNetwork() != "" {
		opts = append(opts, grpc.Network(c.Grpc.Network))
	}
	if c.GetGrpc().GetAddr() != "" {
		opts = append(opts, grpc.Address(c.Grpc.Addr))
	}
	if c.GetGrpc().GetTimeout() != nil {
		opts = append(opts, grpc.Timeout(c.Grpc.Timeout.AsDuration()))
	}
	srv := grpc.NewServer(opts...)
	v1.RegisterDeviceServiceServer(srv, dev)
	v1.RegisterCredentialServiceServer(srv, cred)
	v1.RegisterDiscoveryServiceServer(srv, disc)
	v1.RegisterTopologyServiceServer(srv, topo)
	v1.RegisterEndpointServiceServer(srv, ep)
	v1.RegisterDashboardServiceServer(srv, dash)
	v1.RegisterAuthServiceServer(srv, auth)
	v1.RegisterControlServiceServer(srv, ctrl)
	return srv
}
