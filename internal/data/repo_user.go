package data

import (
	"context"
	"errors"
	"time"

	"hoptrace/internal/biz"

	"gorm.io/gorm"
)

type UserPO struct {
	ID           uint   `gorm:"primaryKey"`
	Username     string `gorm:"size:100;uniqueIndex"`
	DisplayName  string `gorm:"size:100"`
	PasswordHash string `gorm:"size:256"`
	Role         string `gorm:"size:20"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (UserPO) TableName() string { return "users" }

func userToBiz(p *UserPO) *biz.User {
	return &biz.User{
		ID:           p.ID,
		Username:     p.Username,
		DisplayName:  p.DisplayName,
		PasswordHash: p.PasswordHash,
		Role:         p.Role,
		CreatedAt:    p.CreatedAt,
	}
}

type userRepo struct {
	data *Data
}

func NewUserRepo(d *Data) biz.UserRepo {
	return &userRepo{data: d}
}

func (r *userRepo) Count(ctx context.Context) (int64, error) {
	var n int64
	err := r.data.db.WithContext(ctx).Model(&UserPO{}).Count(&n).Error
	return n, err
}

func (r *userRepo) List(ctx context.Context) ([]*biz.User, error) {
	var pos []UserPO
	if err := r.data.db.WithContext(ctx).Order("id asc").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*biz.User, 0, len(pos))
	for i := range pos {
		out = append(out, userToBiz(&pos[i]))
	}
	return out, nil
}

func (r *userRepo) Get(ctx context.Context, id uint) (*biz.User, error) {
	var po UserPO
	err := r.data.db.WithContext(ctx).First(&po, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, biz.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return userToBiz(&po), nil
}

func (r *userRepo) FindByUsername(ctx context.Context, username string) (*biz.User, error) {
	var po UserPO
	err := r.data.db.WithContext(ctx).Where("username = ?", username).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, biz.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return userToBiz(&po), nil
}

func (r *userRepo) Create(ctx context.Context, u *biz.User) (*biz.User, error) {
	po := &UserPO{
		Username:     u.Username,
		DisplayName:  u.DisplayName,
		PasswordHash: u.PasswordHash,
		Role:         u.Role,
	}
	if err := r.data.db.WithContext(ctx).Create(po).Error; err != nil {
		return nil, err
	}
	return userToBiz(po), nil
}

func (r *userRepo) UpdatePassword(ctx context.Context, id uint, hash string) error {
	return r.data.db.WithContext(ctx).Model(&UserPO{}).Where("id = ?", id).Update("password_hash", hash).Error
}

func (r *userRepo) Delete(ctx context.Context, id uint) error {
	res := r.data.db.WithContext(ctx).Delete(&UserPO{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biz.ErrUserNotFound
	}
	return nil
}
