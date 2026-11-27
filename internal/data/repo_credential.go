package data

import (
	"context"
	"errors"

	"hoptrace/internal/biz"

	"gorm.io/gorm"
)

type credentialRepo struct {
	data *Data
}

func NewCredentialRepo(d *Data) biz.CredentialRepo {
	return &credentialRepo{data: d}
}

func (r *credentialRepo) toBiz(p *CredentialPO) (*biz.Credential, error) {
	comm, err := r.data.cipher.Decrypt(p.Community)
	if err != nil {
		return nil, err
	}
	authPwd, err := r.data.cipher.Decrypt(p.AuthPassword)
	if err != nil {
		return nil, err
	}
	privPwd, err := r.data.cipher.Decrypt(p.PrivPassword)
	if err != nil {
		return nil, err
	}
	return &biz.Credential{
		ID: p.ID, Name: p.Name, Version: p.Version, Community: comm,
		Username: p.Username, AuthProtocol: p.AuthProtocol, AuthPassword: authPwd,
		PrivProtocol: p.PrivProtocol, PrivPassword: privPwd, Port: p.Port, CreatedAt: p.CreatedAt,
	}, nil
}

func (r *credentialRepo) toPO(c *biz.Credential) (*CredentialPO, error) {
	comm, err := r.data.cipher.Encrypt(c.Community)
	if err != nil {
		return nil, err
	}
	authPwd, err := r.data.cipher.Encrypt(c.AuthPassword)
	if err != nil {
		return nil, err
	}
	privPwd, err := r.data.cipher.Encrypt(c.PrivPassword)
	if err != nil {
		return nil, err
	}
	return &CredentialPO{
		ID: c.ID, Name: c.Name, Version: c.Version, Community: comm,
		Username: c.Username, AuthProtocol: c.AuthProtocol, AuthPassword: authPwd,
		PrivProtocol: c.PrivProtocol, PrivPassword: privPwd, Port: c.Port, CreatedAt: c.CreatedAt,
	}, nil
}

func (r *credentialRepo) List(ctx context.Context) ([]*biz.Credential, error) {
	var pos []CredentialPO
	if err := r.data.db.WithContext(ctx).Order("id asc").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*biz.Credential, 0, len(pos))
	for i := range pos {
		c, err := r.toBiz(&pos[i])
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func (r *credentialRepo) Get(ctx context.Context, id uint) (*biz.Credential, error) {
	var po CredentialPO
	err := r.data.db.WithContext(ctx).First(&po, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, biz.ErrCredentialNotFound
	}
	if err != nil {
		return nil, err
	}
	return r.toBiz(&po)
}

func (r *credentialRepo) Save(ctx context.Context, c *biz.Credential) (*biz.Credential, error) {
	po, err := r.toPO(c)
	if err != nil {
		return nil, err
	}
	if err := r.data.db.WithContext(ctx).Save(po).Error; err != nil {
		return nil, err
	}
	return r.toBiz(po)
}

func (r *credentialRepo) Delete(ctx context.Context, id uint) error {
	res := r.data.db.WithContext(ctx).Delete(&CredentialPO{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biz.ErrCredentialNotFound
	}
	return nil
}

func (r *credentialRepo) CountDevices(ctx context.Context, id uint) (int64, error) {
	var n int64
	err := r.data.db.WithContext(ctx).Model(&DevicePO{}).Where("credential_id = ?", id).Count(&n).Error
	return n, err
}
