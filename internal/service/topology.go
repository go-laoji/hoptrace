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
		})
	}
	for _, l := range t.Links {
		reply.Links = append(reply.Links, &v1.TopoLink{
			Id:         l.ID,
			Source:     l.Source,
			Target:     l.Target,
			SourcePort: l.SourcePort,
			TargetPort: l.TargetPort,
			Kind:       l.Kind,
			Confidence: l.Confidence,
			Status:     l.Status,
			Members:    l.Members,
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
