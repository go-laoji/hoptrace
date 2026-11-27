package data

import (
	"context"
	"errors"
	"time"

	"hoptrace/internal/biz"

	"gorm.io/gorm"
)

type WriteCredentialPO struct {
	ID            uint   `gorm:"primaryKey"`
	Name          string `gorm:"size:100;uniqueIndex"`
	Protocol      string `gorm:"size:20"`
	Username      string `gorm:"size:100"`
	Password      string `gorm:"size:512"`
	Port          int
	NACWebhookURL string `gorm:"size:500"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (WriteCredentialPO) TableName() string { return "write_credentials" }

type ControlActionPO struct {
	ID              uint   `gorm:"primaryKey"`
	Action          string `gorm:"size:40;index"`
	DeviceID        uint   `gorm:"index"`
	DeviceName      string `gorm:"size:200"`
	IfIndex         int
	IfName          string `gorm:"size:100"`
	EndpointID      uint   `gorm:"index"`
	EndpointMAC     string `gorm:"size:20"`
	ParamValue      string `gorm:"size:200"`
	PrevValue       string `gorm:"size:200"`
	Protocol        string `gorm:"size:20"`
	DryRunPreview   string `gorm:"type:text"`
	RollbackPreview string `gorm:"type:text"`
	Status          string `gorm:"size:30;index"`
	Reason          string `gorm:"size:500"`
	RequestedBy     string `gorm:"size:100"`
	ApprovedBy      string `gorm:"size:100"`
	ResultLog       string `gorm:"type:text"`
	CreatedAt       time.Time `gorm:"index"`
	ExecutedAt      *time.Time
	RolledBackAt    *time.Time
}

func (ControlActionPO) TableName() string { return "control_actions" }

func controlActionToPO(a *biz.ControlAction) *ControlActionPO {
	return &ControlActionPO{
		ID: a.ID, Action: a.Action, DeviceID: a.DeviceID, DeviceName: a.DeviceName,
		IfIndex: a.IfIndex, IfName: a.IfName, EndpointID: a.EndpointID, EndpointMAC: a.EndpointMAC,
		ParamValue: a.ParamValue, PrevValue: a.PrevValue, Protocol: a.Protocol,
		DryRunPreview: a.DryRunPreview, RollbackPreview: a.RollbackPreview, Status: a.Status,
		Reason: a.Reason, RequestedBy: a.RequestedBy, ApprovedBy: a.ApprovedBy, ResultLog: a.ResultLog,
		CreatedAt: a.CreatedAt, ExecutedAt: a.ExecutedAt, RolledBackAt: a.RolledBackAt,
	}
}

func controlActionToBiz(p *ControlActionPO) *biz.ControlAction {
	return &biz.ControlAction{
		ID: p.ID, Action: p.Action, DeviceID: p.DeviceID, DeviceName: p.DeviceName,
		IfIndex: p.IfIndex, IfName: p.IfName, EndpointID: p.EndpointID, EndpointMAC: p.EndpointMAC,
		ParamValue: p.ParamValue, PrevValue: p.PrevValue, Protocol: p.Protocol,
		DryRunPreview: p.DryRunPreview, RollbackPreview: p.RollbackPreview, Status: p.Status,
		Reason: p.Reason, RequestedBy: p.RequestedBy, ApprovedBy: p.ApprovedBy, ResultLog: p.ResultLog,
		CreatedAt: p.CreatedAt, ExecutedAt: p.ExecutedAt, RolledBackAt: p.RolledBackAt,
	}
}

type controlRepo struct {
	data *Data
}

func NewControlRepo(d *Data) biz.ControlRepo {
	return &controlRepo{data: d}
}

func (r *controlRepo) ListActions(ctx context.Context, f biz.ControlFilter, page, size int) ([]*biz.ControlAction, int64, error) {
	q := r.data.db.WithContext(ctx).Model(&ControlActionPO{})
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.DeviceID != 0 {
		q = q.Where("device_id = ?", f.DeviceID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 200 {
		size = 50
	}
	var pos []ControlActionPO
	if err := q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*biz.ControlAction, 0, len(pos))
	for i := range pos {
		out = append(out, controlActionToBiz(&pos[i]))
	}
	return out, total, nil
}

func (r *controlRepo) GetAction(ctx context.Context, id uint) (*biz.ControlAction, error) {
	var po ControlActionPO
	err := r.data.db.WithContext(ctx).First(&po, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, biz.ErrControlNotFound
	}
	if err != nil {
		return nil, err
	}
	return controlActionToBiz(&po), nil
}

func (r *controlRepo) CreateAction(ctx context.Context, a *biz.ControlAction) (*biz.ControlAction, error) {
	po := controlActionToPO(a)
	if err := r.data.db.WithContext(ctx).Create(po).Error; err != nil {
		return nil, err
	}
	return controlActionToBiz(po), nil
}

func (r *controlRepo) UpdateAction(ctx context.Context, a *biz.ControlAction) error {
	return r.data.db.WithContext(ctx).Save(controlActionToPO(a)).Error
}

func (r *controlRepo) writeCredToBiz(p *WriteCredentialPO) (*biz.WriteCredential, error) {
	pwd, err := r.data.cipher.Decrypt(p.Password)
	if err != nil {
		return nil, err
	}
	return &biz.WriteCredential{
		ID: p.ID, Name: p.Name, Protocol: p.Protocol, Username: p.Username,
		Password: pwd, Port: p.Port, NACWebhookURL: p.NACWebhookURL, CreatedAt: p.CreatedAt,
	}, nil
}

func (r *controlRepo) ListWriteCredentials(ctx context.Context) ([]*biz.WriteCredential, error) {
	var pos []WriteCredentialPO
	if err := r.data.db.WithContext(ctx).Order("id asc").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*biz.WriteCredential, 0, len(pos))
	for i := range pos {
		c, err := r.writeCredToBiz(&pos[i])
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func (r *controlRepo) GetWriteCredential(ctx context.Context, id uint) (*biz.WriteCredential, error) {
	var po WriteCredentialPO
	err := r.data.db.WithContext(ctx).First(&po, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, biz.ErrControlNotFound
	}
	if err != nil {
		return nil, err
	}
	return r.writeCredToBiz(&po)
}

func (r *controlRepo) SaveWriteCredential(ctx context.Context, c *biz.WriteCredential) (*biz.WriteCredential, error) {
	enc, err := r.data.cipher.Encrypt(c.Password)
	if err != nil {
		return nil, err
	}
	po := &WriteCredentialPO{
		ID: c.ID, Name: c.Name, Protocol: c.Protocol, Username: c.Username,
		Password: enc, Port: c.Port, NACWebhookURL: c.NACWebhookURL, CreatedAt: c.CreatedAt,
	}
	if err := r.data.db.WithContext(ctx).Save(po).Error; err != nil {
		return nil, err
	}
	return r.writeCredToBiz(po)
}

func (r *controlRepo) DeleteWriteCredential(ctx context.Context, id uint) error {
	res := r.data.db.WithContext(ctx).Delete(&WriteCredentialPO{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biz.ErrControlNotFound
	}
	return nil
}
