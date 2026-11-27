package service

import (
	"context"

	v1 "hoptrace/api/hoptrace/v1"
	"hoptrace/internal/biz"
)

type BackupService struct {
	v1.UnimplementedBackupServiceServer
	uc *biz.BackupUsecase
}

func NewBackupService(uc *biz.BackupUsecase) *BackupService { return &BackupService{uc: uc} }

func (s *BackupService) ListBackups(ctx context.Context, req *v1.ListBackupsRequest) (*v1.ListBackupsReply, error) {
	items, total, err := s.uc.List(ctx, uint(req.GetDeviceId()), int(req.GetPage()), int(req.GetPageSize()))
	if err != nil {
		return nil, err
	}
	reply := &v1.ListBackupsReply{Total: total, Items: make([]*v1.ConfigBackup, 0, len(items))}
	for _, b := range items {
		reply.Items = append(reply.Items, toProtoBackup(b))
	}
	return reply, nil
}

func (s *BackupService) RunBackup(ctx context.Context, req *v1.RunBackupRequest) (*v1.ConfigBackup, error) {
	b, err := s.uc.Run(ctx, uint(req.GetDeviceId()), "manual")
	if err != nil {
		return nil, err
	}
	return toProtoBackup(b), nil
}

func (s *BackupService) GetBackup(ctx context.Context, req *v1.GetBackupRequest) (*v1.ConfigBackupDetail, error) {
	b, err := s.uc.Get(ctx, uint(req.GetId()))
	if err != nil {
		return nil, err
	}
	return &v1.ConfigBackupDetail{Meta: toProtoBackup(b), Content: b.Content}, nil
}

func (s *BackupService) DiffBackups(ctx context.Context, req *v1.DiffBackupsRequest) (*v1.DiffBackupsReply, error) {
	res, err := s.uc.Diff(ctx, uint(req.GetFromId()), uint(req.GetToId()))
	if err != nil {
		return nil, err
	}
	reply := &v1.DiffBackupsReply{
		From: toProtoBackup(res.From), To: toProtoBackup(res.To),
		Added: int32(res.Added), Removed: int32(res.Removed),
		Lines: make([]*v1.DiffLine, 0, len(res.Lines)),
	}
	for _, l := range res.Lines {
		reply.Lines = append(reply.Lines, &v1.DiffLine{
			Kind: l.Kind, OldLine: int32(l.OldLine), NewLine: int32(l.NewLine), Text: l.Text,
		})
	}
	return reply, nil
}

func toProtoBackup(b *biz.ConfigBackup) *v1.ConfigBackup {
	return &v1.ConfigBackup{
		Id: int64(b.ID), DeviceId: int64(b.DeviceID), DeviceName: b.DeviceName,
		Sha256: b.SHA256, SizeBytes: int32(b.SizeBytes), Changed: b.Changed,
		Trigger: b.Trigger, Status: b.Status, Error: b.Error, CreatedAt: b.CreatedAt.Unix(),
	}
}
