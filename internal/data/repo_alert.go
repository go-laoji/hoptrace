package data

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"hoptrace/internal/biz"

	"gorm.io/gorm"
)

type AlertPO struct {
	ID         uint   `gorm:"primaryKey"`
	Kind       string `gorm:"size:40;index"`
	Severity   string `gorm:"size:20;index"`
	Title      string `gorm:"size:300"`
	Message    string `gorm:"type:text"`
	TargetKey  string `gorm:"size:200;index"`
	DeviceID   uint   `gorm:"index"`
	EndpointID uint   `gorm:"index"`
	Status     string `gorm:"size:20;index"`
	AckedBy    string `gorm:"size:100"`
	CreatedAt  time.Time `gorm:"index"`
	UpdatedAt  time.Time
}

func (AlertPO) TableName() string { return "alerts" }

type AlertChannelPO struct {
	ID         uint       `gorm:"primaryKey"`
	Name       string     `gorm:"size:100;uniqueIndex"`
	Kind       string     `gorm:"size:20"`
	WebhookURL string     `gorm:"size:500"`
	Secret     string     `gorm:"size:512"`
	Enabled    bool
	Kinds      StringList `gorm:"type:text"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (AlertChannelPO) TableName() string { return "alert_channels" }

type SyslogPO struct {
	ID         uint      `gorm:"primaryKey"`
	SourceIP   string    `gorm:"size:64;index"`
	DeviceID   uint      `gorm:"index"`
	DeviceName string    `gorm:"size:200"`
	Severity   int       `gorm:"index"`
	Facility   string    `gorm:"size:40"`
	Tag        string    `gorm:"size:100"`
	Message    string    `gorm:"type:text"`
	ReceivedAt time.Time `gorm:"index"`
}

func (SyslogPO) TableName() string { return "syslogs" }

type alertRepo struct{ data *Data }

func NewAlertRepo(d *Data) biz.AlertRepo { return &alertRepo{data: d} }

func alertToBiz(p *AlertPO) *biz.Alert {
	return &biz.Alert{
		ID: p.ID, Kind: p.Kind, Severity: p.Severity, Title: p.Title, Message: p.Message,
		TargetKey: p.TargetKey, DeviceID: p.DeviceID, EndpointID: p.EndpointID,
		Status: p.Status, AckedBy: p.AckedBy, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func (r *alertRepo) ListAlerts(ctx context.Context, f biz.AlertFilter, page, size int) ([]*biz.Alert, int64, error) {
	q := r.data.db.WithContext(ctx).Model(&AlertPO{})
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.Severity != "" {
		q = q.Where("severity = ?", f.Severity)
	}
	if f.Kind != "" {
		q = q.Where("kind = ?", f.Kind)
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
	var pos []AlertPO
	if err := q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*biz.Alert, 0, len(pos))
	for i := range pos {
		out = append(out, alertToBiz(&pos[i]))
	}
	return out, total, nil
}

func (r *alertRepo) GetAlert(ctx context.Context, id uint) (*biz.Alert, error) {
	var po AlertPO
	err := r.data.db.WithContext(ctx).First(&po, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, biz.ErrControlNotFound
	}
	if err != nil {
		return nil, err
	}
	return alertToBiz(&po), nil
}

func (r *alertRepo) FindOpenByKey(ctx context.Context, kind, targetKey string) (*biz.Alert, error) {
	var po AlertPO
	err := r.data.db.WithContext(ctx).
		Where("kind = ? AND target_key = ? AND status IN ?", kind, targetKey, []string{biz.AlertOpen, biz.AlertAcked}).
		Order("id desc").First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return alertToBiz(&po), nil
}

func (r *alertRepo) CreateAlert(ctx context.Context, a *biz.Alert) (*biz.Alert, error) {
	po := &AlertPO{
		Kind: a.Kind, Severity: a.Severity, Title: a.Title, Message: a.Message,
		TargetKey: a.TargetKey, DeviceID: a.DeviceID, EndpointID: a.EndpointID,
		Status: a.Status, AckedBy: a.AckedBy, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
	if err := r.data.db.WithContext(ctx).Create(po).Error; err != nil {
		return nil, err
	}
	return alertToBiz(po), nil
}

func (r *alertRepo) UpdateAlert(ctx context.Context, a *biz.Alert) error {
	return r.data.db.WithContext(ctx).Model(&AlertPO{}).Where("id = ?", a.ID).Updates(map[string]any{
		"status": a.Status, "acked_by": a.AckedBy, "message": a.Message, "updated_at": a.UpdatedAt,
	}).Error
}

func (r *alertRepo) channelToBiz(p *AlertChannelPO) *biz.AlertChannel {
	sec, _ := r.data.cipher.Decrypt(p.Secret)
	return &biz.AlertChannel{
		ID: p.ID, Name: p.Name, Kind: p.Kind, WebhookURL: p.WebhookURL,
		Secret: sec, Enabled: p.Enabled, Kinds: []string(p.Kinds), CreatedAt: p.CreatedAt,
	}
}

func (r *alertRepo) ListChannels(ctx context.Context) ([]*biz.AlertChannel, error) {
	var pos []AlertChannelPO
	if err := r.data.db.WithContext(ctx).Order("id asc").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*biz.AlertChannel, 0, len(pos))
	for i := range pos {
		out = append(out, r.channelToBiz(&pos[i]))
	}
	return out, nil
}

func (r *alertRepo) GetChannel(ctx context.Context, id uint) (*biz.AlertChannel, error) {
	var po AlertChannelPO
	err := r.data.db.WithContext(ctx).First(&po, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, biz.ErrControlNotFound
	}
	if err != nil {
		return nil, err
	}
	return r.channelToBiz(&po), nil
}

func (r *alertRepo) SaveChannel(ctx context.Context, c *biz.AlertChannel) (*biz.AlertChannel, error) {
	encSec, _ := r.data.cipher.Encrypt(c.Secret)
	po := &AlertChannelPO{
		ID: c.ID, Name: c.Name, Kind: c.Kind, WebhookURL: c.WebhookURL,
		Secret: encSec, Enabled: c.Enabled, Kinds: StringList(c.Kinds),
	}
	if err := r.data.db.WithContext(ctx).Save(po).Error; err != nil {
		return nil, err
	}
	return r.channelToBiz(po), nil
}

func (r *alertRepo) DeleteChannel(ctx context.Context, id uint) error {
	return r.data.db.WithContext(ctx).Delete(&AlertChannelPO{}, id).Error
}

func (r *alertRepo) CreateSyslog(ctx context.Context, e *biz.SyslogEntry) error {
	po := &SyslogPO{
		SourceIP: e.SourceIP, DeviceID: e.DeviceID, DeviceName: e.DeviceName,
		Severity: e.Severity, Facility: e.Facility, Tag: e.Tag, Message: e.Message, ReceivedAt: e.ReceivedAt,
	}
	return r.data.db.WithContext(ctx).Create(po).Error
}

func (r *alertRepo) ListSyslogs(ctx context.Context, f biz.SyslogFilter, page, size int) ([]*biz.SyslogEntry, int64, error) {
	q := r.data.db.WithContext(ctx).Model(&SyslogPO{})
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("message LIKE ? OR tag LIKE ? OR device_name LIKE ? OR source_ip LIKE ?", like, like, like, like)
	}
	if f.SourceIP != "" {
		q = q.Where("source_ip = ?", f.SourceIP)
	}
	if f.MaxSeverity > 0 {
		q = q.Where("severity <= ?", f.MaxSeverity)
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
	var pos []SyslogPO
	if err := q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*biz.SyslogEntry, 0, len(pos))
	for _, p := range pos {
		out = append(out, &biz.SyslogEntry{
			ID: p.ID, SourceIP: p.SourceIP, DeviceID: p.DeviceID, DeviceName: p.DeviceName,
			Severity: p.Severity, Facility: p.Facility, Tag: p.Tag, Message: p.Message, ReceivedAt: p.ReceivedAt,
		})
	}
	return out, total, nil
}

// ---------- Webhook 通知发送器 (企业微信 / 钉钉 / 飞书 / 通用 JSON) ----------

type webhookNotifier struct{ client *http.Client }

func NewAlertNotifier() biz.AlertNotifier {
	return &webhookNotifier{client: &http.Client{Timeout: 8 * time.Second}}
}

func (n *webhookNotifier) Send(ctx context.Context, ch *biz.AlertChannel, a *biz.Alert) (string, error) {
	md := fmt.Sprintf("### [HopTrace %s] %s\n- **类型**: %s\n- **详情**: %s\n- **时间**: %s",
		strings.ToUpper(a.Severity), a.Title, a.Kind, a.Message, a.CreatedAt.Format("2006-01-02 15:04:05"))

	var payload any
	switch ch.Kind {
	case "wecom":
		payload = map[string]any{"msgtype": "markdown", "markdown": map[string]string{"content": md}}
	case "dingtalk":
		payload = map[string]any{"msgtype": "markdown", "markdown": map[string]string{"title": a.Title, "text": md}}
	case "feishu":
		payload = map[string]any{"msg_type": "text", "content": map[string]string{"text": fmt.Sprintf("[HopTrace %s] %s\n%s", strings.ToUpper(a.Severity), a.Title, a.Message)}}
	default:
		payload = map[string]any{
			"id": a.ID, "kind": a.Kind, "severity": a.Severity, "title": a.Title,
			"message": a.Message, "target": a.TargetKey, "at": a.CreatedAt.Unix(),
		}
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ch.WebhookURL, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
	}
	return fmt.Sprintf("HTTP %d OK", resp.StatusCode), nil
}
