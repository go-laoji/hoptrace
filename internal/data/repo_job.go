package data

import (
	"context"
	"errors"
	"time"

	"hoptrace/internal/biz"

	"gorm.io/gorm"
)

type jobRepo struct {
	data *Data
}

func NewJobRepo(d *Data) biz.JobRepo {
	return &jobRepo{data: d}
}

func (r *jobRepo) Create(ctx context.Context, j *biz.DiscoveryJob) (*biz.DiscoveryJob, error) {
	po := jobToPO(j)
	if err := r.data.db.WithContext(ctx).Create(po).Error; err != nil {
		return nil, err
	}
	return jobToBiz(po), nil
}

func (r *jobRepo) Update(ctx context.Context, j *biz.DiscoveryJob) error {
	return r.data.db.WithContext(ctx).Save(jobToPO(j)).Error
}

func (r *jobRepo) Get(ctx context.Context, id uint) (*biz.DiscoveryJob, error) {
	var po DiscoveryJobPO
	err := r.data.db.WithContext(ctx).First(&po, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, biz.ErrDiscoveryNotFound
	}
	if err != nil {
		return nil, err
	}
	return jobToBiz(&po), nil
}

func (r *jobRepo) List(ctx context.Context, limit int) ([]*biz.DiscoveryJob, error) {
	if limit <= 0 {
		limit = 20
	}
	var pos []DiscoveryJobPO
	if err := r.data.db.WithContext(ctx).Order("id desc").Limit(limit).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*biz.DiscoveryJob, 0, len(pos))
	for i := range pos {
		out = append(out, jobToBiz(&pos[i]))
	}
	return out, nil
}

func (r *jobRepo) FailRunning(ctx context.Context) error {
	now := time.Now()
	return r.data.db.WithContext(ctx).Model(&DiscoveryJobPO{}).
		Where("status = ?", biz.JobRunning).
		Updates(map[string]any{"status": biz.JobFailed, "finished_at": now}).Error
}
