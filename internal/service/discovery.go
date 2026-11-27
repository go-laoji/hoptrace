package service

import (
	"context"

	v1 "hoptrace/api/hoptrace/v1"
	"hoptrace/internal/biz"
)

type DiscoveryService struct {
	v1.UnimplementedDiscoveryServiceServer
	uc *biz.DiscoveryUsecase
}

func NewDiscoveryService(uc *biz.DiscoveryUsecase) *DiscoveryService {
	return &DiscoveryService{uc: uc}
}

func (s *DiscoveryService) StartDiscovery(ctx context.Context, req *v1.StartDiscoveryRequest) (*v1.DiscoveryJob, error) {
	credIDs := make([]uint, 0, len(req.GetCredentialIds()))
	for _, id := range req.GetCredentialIds() {
		credIDs = append(credIDs, uint(id))
	}
	j, err := s.uc.Start(ctx, biz.DiscoveryRequest{
		Seeds:           req.GetSeeds(),
		CIDRs:           req.GetCidrs(),
		CredentialIDs:   credIDs,
		ExpandNeighbors: req.GetExpandNeighbors(),
		MaxDevices:      int(req.GetMaxDevices()),
		Site:            req.GetSite(),
	})
	if err != nil {
		return nil, err
	}
	return toProtoJob(j), nil
}

func (s *DiscoveryService) ListJobs(ctx context.Context, req *v1.ListJobsRequest) (*v1.ListJobsReply, error) {
	items, err := s.uc.List(ctx, int(req.GetLimit()))
	if err != nil {
		return nil, err
	}
	reply := &v1.ListJobsReply{Items: make([]*v1.DiscoveryJob, 0, len(items))}
	for _, j := range items {
		reply.Items = append(reply.Items, toProtoJob(j))
	}
	return reply, nil
}

func (s *DiscoveryService) GetJob(ctx context.Context, req *v1.GetJobRequest) (*v1.DiscoveryJob, error) {
	j, err := s.uc.Get(ctx, uint(req.GetId()))
	if err != nil {
		return nil, err
	}
	return toProtoJob(j), nil
}

func toProtoJob(j *biz.DiscoveryJob) *v1.DiscoveryJob {
	var finished int64
	if j.FinishedAt != nil {
		finished = j.FinishedAt.Unix()
	}
	return &v1.DiscoveryJob{
		Id:         int64(j.ID),
		Status:     j.Status,
		Seeds:      j.Seeds,
		Cidrs:      j.CIDRs,
		Probed:     int32(j.Probed),
		Total:      int32(j.Total),
		Found:      int32(j.Found),
		Log:        j.Log,
		StartedAt:  j.StartedAt.Unix(),
		FinishedAt: finished,
	}
}
