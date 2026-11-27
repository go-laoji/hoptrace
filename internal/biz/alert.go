package biz

import (
	"context"
	"strings"
	"time"
)

// 告警类型
const (
	AlertDeviceOffline  = "device_offline"
	AlertLinkDown       = "link_down"
	AlertNewEndpoint    = "new_endpoint"
	AlertEndpointMoved  = "endpoint_moved"
	AlertIPConflict     = "ip_conflict"
	AlertSuspectHub     = "suspect_hub"
	AlertSyslogSecurity = "syslog_security"
)

// 告警级别与状态
const (
	SeverityCritical = "critical"
	SeverityWarning  = "warning"
	SeverityInfo     = "info"

	AlertOpen     = "open"
	AlertAcked    = "acked"
	AlertResolved = "resolved"
)

// Alert 告警记录 DO。
type Alert struct {
	ID         uint
	Kind       string
	Severity   string
	Title      string
	Message    string
	TargetKey  string
	DeviceID   uint
	EndpointID uint
	Status     string
	AckedBy    string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// AlertChannel 告警通知渠道(企业微信/钉钉/飞书/通用 Webhook)。
type AlertChannel struct {
	ID         uint
	Name       string
	Kind       string // wecom / dingtalk / feishu / webhook
	WebhookURL string
	Secret     string
	Enabled    bool
	Kinds      []string
	CreatedAt  time.Time
}

// SyslogEntry 设备 Syslog 日志 DO。
type SyslogEntry struct {
	ID         uint
	SourceIP   string
	DeviceID   uint
	DeviceName string
	Severity   int
	Facility   string
	Tag        string
	Message    string
	ReceivedAt time.Time
}

type AlertFilter struct {
	Status   string
	Severity string
	Kind     string
}

type SyslogFilter struct {
	Keyword     string
	SourceIP    string
	MaxSeverity int
}

type AlertRepo interface {
	ListAlerts(ctx context.Context, f AlertFilter, page, size int) ([]*Alert, int64, error)
	GetAlert(ctx context.Context, id uint) (*Alert, error)
	FindOpenByKey(ctx context.Context, kind, targetKey string) (*Alert, error)
	CreateAlert(ctx context.Context, a *Alert) (*Alert, error)
	UpdateAlert(ctx context.Context, a *Alert) error
	ListChannels(ctx context.Context) ([]*AlertChannel, error)
	GetChannel(ctx context.Context, id uint) (*AlertChannel, error)
	SaveChannel(ctx context.Context, c *AlertChannel) (*AlertChannel, error)
	DeleteChannel(ctx context.Context, id uint) error
	CreateSyslog(ctx context.Context, e *SyslogEntry) error
	ListSyslogs(ctx context.Context, f SyslogFilter, page, size int) ([]*SyslogEntry, int64, error)
}

// AlertNotifier 发送告警到外部 Webhook(企业微信/钉钉/飞书/通用 JSON)。
type AlertNotifier interface {
	Send(ctx context.Context, ch *AlertChannel, a *Alert) (string, error)
}

// AlertUsecase 告警触发、去重、确认与渠道推送编排。
type AlertUsecase struct {
	repo     AlertRepo
	notifier AlertNotifier
	bus      *EventBus
}

func NewAlertUsecase(repo AlertRepo, notifier AlertNotifier, bus *EventBus) *AlertUsecase {
	return &AlertUsecase{repo: repo, notifier: notifier, bus: bus}
}

func (uc *AlertUsecase) ListAlerts(ctx context.Context, f AlertFilter, page, size int) ([]*Alert, int64, error) {
	return uc.repo.ListAlerts(ctx, f, page, size)
}

func (uc *AlertUsecase) Ack(ctx context.Context, id uint) (*Alert, error) {
	a, err := uc.repo.GetAlert(ctx, id)
	if err != nil {
		return nil, err
	}
	who := "admin"
	if c, ok := ClaimsFromContext(ctx); ok && c.Username != "" {
		who = c.Username
	}
	a.Status = AlertAcked
	a.AckedBy = who
	a.UpdatedAt = time.Now()
	if err := uc.repo.UpdateAlert(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (uc *AlertUsecase) Resolve(ctx context.Context, id uint) (*Alert, error) {
	a, err := uc.repo.GetAlert(ctx, id)
	if err != nil {
		return nil, err
	}
	a.Status = AlertResolved
	a.UpdatedAt = time.Now()
	if err := uc.repo.UpdateAlert(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// Raise 触发告警(同 kind+targetKey 若已有未关闭告警则自动去重, 避免风暴), 并异步推送到已启用的通知渠道。
func (uc *AlertUsecase) Raise(ctx context.Context, a *Alert) {
	if uc == nil || uc.repo == nil {
		return
	}
	if a.TargetKey != "" {
		if old, err := uc.repo.FindOpenByKey(ctx, a.Kind, a.TargetKey); err == nil && old != nil {
			old.Message = a.Message
			old.UpdatedAt = time.Now()
			_ = uc.repo.UpdateAlert(ctx, old)
			return
		}
	}
	now := time.Now()
	a.Status = AlertOpen
	a.CreatedAt = now
	a.UpdatedAt = now
	created, err := uc.repo.CreateAlert(ctx, a)
	if err != nil {
		return
	}
	uc.bus.Publish(LiveEvent{Type: "topology_updated", Source: "alert", Message: "新告警: " + created.Title})
	go uc.dispatch(created)
}

func (uc *AlertUsecase) dispatch(a *Alert) {
	if uc.notifier == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	chs, err := uc.repo.ListChannels(ctx)
	if err != nil {
		return
	}
	for _, ch := range chs {
		if !ch.Enabled || strings.TrimSpace(ch.WebhookURL) == "" {
			continue
		}
		if len(ch.Kinds) > 0 && !containsStr(ch.Kinds, a.Kind) {
			continue
		}
		_, _ = uc.notifier.Send(ctx, ch, a)
	}
}

func (uc *AlertUsecase) ListChannels(ctx context.Context) ([]*AlertChannel, error) {
	return uc.repo.ListChannels(ctx)
}

func (uc *AlertUsecase) SaveChannel(ctx context.Context, c *AlertChannel) (*AlertChannel, error) {
	c.Name = strings.TrimSpace(c.Name)
	c.WebhookURL = strings.TrimSpace(c.WebhookURL)
	if c.Name == "" || c.WebhookURL == "" {
		return nil, ErrControlInvalidState
	}
	if c.Kind == "" {
		c.Kind = "webhook"
	}
	return uc.repo.SaveChannel(ctx, c)
}

func (uc *AlertUsecase) DeleteChannel(ctx context.Context, id uint) error {
	return uc.repo.DeleteChannel(ctx, id)
}

func (uc *AlertUsecase) TestChannel(ctx context.Context, id uint) (bool, string, error) {
	ch, err := uc.repo.GetChannel(ctx, id)
	if err != nil {
		return false, "", err
	}
	detail, err := uc.notifier.Send(ctx, ch, &Alert{
		Kind:      "test",
		Severity:  SeverityInfo,
		Title:     "HopTrace 测试告警通知",
		Message:   "这是一条来自 HopTrace 的测试推送，收到说明通知渠道配置成功。",
		CreatedAt: time.Now(),
	})
	if err != nil {
		return false, err.Error(), nil
	}
	return true, detail, nil
}

func (uc *AlertUsecase) RecordSyslog(ctx context.Context, e *SyslogEntry) error {
	if err := uc.repo.CreateSyslog(ctx, e); err != nil {
		return err
	}
	// 识别华为端口安全/环路/MAC 漂移/认证失败等高危 Syslog 自动升级为告警
	upper := strings.ToUpper(e.Message)
	if e.Severity <= 3 || strings.Contains(upper, "MACFLAP") || strings.Contains(upper, "LOOPBACK") ||
		strings.Contains(upper, "PORTSEC") || strings.Contains(upper, "LOGIN_FAILED") {
		uc.Raise(ctx, &Alert{
			Kind:      AlertSyslogSecurity,
			Severity:  SeverityWarning,
			Title:     "设备安全/异常 Syslog: " + firstNonEmpty(e.DeviceName, e.SourceIP),
			Message:   e.Message,
			TargetKey: e.SourceIP + ":" + e.Tag,
			DeviceID:  e.DeviceID,
		})
	}
	return nil
}

func (uc *AlertUsecase) ListSyslogs(ctx context.Context, f SyslogFilter, page, size int) ([]*SyslogEntry, int64, error) {
	return uc.repo.ListSyslogs(ctx, f, page, size)
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
