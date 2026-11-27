package service

import (
	"context"

	v1 "hoptrace/api/hoptrace/v1"
	"hoptrace/internal/biz"

	"google.golang.org/protobuf/types/known/emptypb"
)

type IpamService struct {
	v1.UnimplementedIpamServiceServer
	uc *biz.IpamUsecase
}

func NewIpamService(uc *biz.IpamUsecase) *IpamService { return &IpamService{uc: uc} }

func (s *IpamService) ListSubnets(ctx context.Context, _ *emptypb.Empty) (*v1.ListSubnetsReply, error) {
	items, err := s.uc.ListSubnets(ctx)
	if err != nil {
		return nil, err
	}
	reply := &v1.ListSubnetsReply{Items: make([]*v1.SubnetSummary, 0, len(items))}
	for _, x := range items {
		reply.Items = append(reply.Items, toProtoSubnetSummary(x))
	}
	return reply, nil
}

func (s *IpamService) GetSubnetDetail(ctx context.Context, req *v1.GetSubnetDetailRequest) (*v1.SubnetDetail, error) {
	d, err := s.uc.GetSubnetDetail(ctx, req.GetCidr())
	if err != nil {
		return nil, err
	}
	reply := &v1.SubnetDetail{
		Summary: toProtoSubnetSummary(d.Summary),
		Leases:  make([]*v1.IpLease, 0, len(d.Leases)),
	}
	for _, l := range d.Leases {
		reply.Leases = append(reply.Leases, &v1.IpLease{
			Ip: l.IP, Used: l.Used, Mac: l.MAC, Macs: l.MACs, Conflict: l.Conflict,
			Vendor: l.Vendor, EndpointName: l.EndpointName, DeviceName: l.DeviceName, IfName: l.IfName,
		})
	}
	return reply, nil
}

func (s *IpamService) ListAnomalies(ctx context.Context, _ *emptypb.Empty) (*v1.ListAnomaliesReply, error) {
	items, err := s.uc.ListAnomalies(ctx)
	if err != nil {
		return nil, err
	}
	reply := &v1.ListAnomaliesReply{Items: make([]*v1.NetworkAnomaly, 0, len(items))}
	for _, a := range items {
		reply.Items = append(reply.Items, &v1.NetworkAnomaly{
			Kind: a.Kind, Severity: a.Severity, Target: a.Target, Detail: a.Detail,
			DeviceId: int64(a.DeviceID), DeviceName: a.DeviceName, IfName: a.IfName,
		})
	}
	return reply, nil
}

func toProtoSubnetSummary(x biz.SubnetSummary) *v1.SubnetSummary {
	return &v1.SubnetSummary{
		Cidr: x.CIDR, TotalHosts: int32(x.TotalHosts), UsedHosts: int32(x.UsedHosts),
		FreeHosts: int32(x.FreeHosts), UsagePct: x.UsagePct, ConflictCount: int32(x.ConflictCount),
		GatewayDevice: x.GatewayDevice,
	}
}
