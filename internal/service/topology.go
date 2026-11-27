package service

import (
	"context"

	v1 "hoptrace/api/hoptrace/v1"
	"hoptrace/internal/biz"

	"google.golang.org/protobuf/types/known/emptypb"
)

type TopologyService struct {
	v1.UnimplementedTopologyServiceServer
	uc   *biz.TopologyUsecase
	poll *biz.PollUsecase
}

func NewTopologyService(uc *biz.TopologyUsecase, poll *biz.PollUsecase) *TopologyService {
	return &TopologyService{uc: uc, poll: poll}
}

func (s *TopologyService) GetTopology(ctx context.Context, req *v1.GetTopologyRequest) (*v1.Topology, error) {
	expand := make([]uint, 0, len(req.GetExpandDeviceIds()))
	for _, id := range req.GetExpandDeviceIds() {
		expand = append(expand, uint(id))
	}
	t, err := s.uc.Get(ctx, expand, req.GetIncludeAllEndpoints(), req.GetSite())
	if err != nil {
		return nil, err
	}
	reply := &v1.Topology{
		Nodes: make([]*v1.TopoNode, 0, len(t.Nodes)),
		Links: make([]*v1.TopoLink, 0, len(t.Links)),
	}
	for _, n := range t.Nodes {
		reply.Nodes = append(reply.Nodes, &v1.TopoNode{
			Id:            n.ID,
			Kind:          n.Kind,
			RefId:         int64(n.RefID),
			Name:          n.Name,
			Ip:            n.IP,
			Role:          n.Role,
			Family:        n.Family,
			Status:        n.Status,
			Vendor:        n.Vendor,
			Model:         n.Model,
			Type:          n.Type,
			Managed:       n.Managed,
			EndpointCount: int32(n.EndpointCount),
			PosX:          n.PosX,
			PosY:          n.PosY,
			HasPos:        n.HasPos,
		})
	}
	for _, l := range t.Links {
		reply.Links = append(reply.Links, &v1.TopoLink{
			Id:             l.ID,
			Source:         l.Source,
			Target:         l.Target,
			SourcePort:     l.SourcePort,
			TargetPort:     l.TargetPort,
			Kind:           l.Kind,
			Confidence:     l.Confidence,
			Status:         l.Status,
			Members:        l.Members,
			InBps:          l.InBps,
			OutBps:         l.OutBps,
			UtilizationPct: l.UtilizationPct,
		})
	}
	return reply, nil
}

func (s *TopologyService) Rebuild(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	if err := s.poll.Refresh(ctx); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *TopologyService) TracePath(ctx context.Context, req *v1.TracePathRequest) (*v1.TracePathReply, error) {
	res, err := s.uc.TracePath(ctx, req.GetSource(), req.GetTarget())
	if err != nil {
		return nil, err
	}
	reply := &v1.TracePathReply{
		Found: res.Found, Message: res.Message, NodeIds: res.NodeIDs, LinkIds: res.LinkIDs,
		Hops: make([]*v1.PathHop, 0, len(res.Hops)),
	}
	for _, h := range res.Hops {
		reply.Hops = append(reply.Hops, &v1.PathHop{
			NodeId: h.NodeID, Kind: h.Kind, Name: h.Name, Ip: h.IP, Role: h.Role,
			InPort: h.InPort, OutPort: h.OutPort, LinkId: h.LinkID,
		})
	}
	return reply, nil
}

func (s *TopologyService) SaveLayout(ctx context.Context, req *v1.SaveLayoutRequest) (*emptypb.Empty, error) {
	pos := make([]biz.NodePos, 0, len(req.GetPositions()))
	for _, p := range req.GetPositions() {
		pos = append(pos, biz.NodePos{DeviceID: uint(p.GetDeviceId()), X: p.GetX(), Y: p.GetY()})
	}
	if err := s.uc.SaveLayout(ctx, pos); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *TopologyService) ResetLayout(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	if err := s.uc.ResetLayout(ctx); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
