package data

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"hoptrace/internal/biz"
	"hoptrace/internal/conf"

	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

type ConfigBackupPO struct {
	ID         uint   `gorm:"primaryKey"`
	DeviceID   uint   `gorm:"index"`
	DeviceName string `gorm:"size:200"`
	SHA256     string `gorm:"size:64"`
	SizeBytes  int
	Changed    bool
	Trigger    string    `gorm:"size:20"`
	Status     string    `gorm:"size:20"`
	Error      string    `gorm:"size:500"`
	Content    string    `gorm:"type:text"`
	CreatedAt  time.Time `gorm:"index"`
}

func (ConfigBackupPO) TableName() string { return "config_backups" }

func backupToBiz(p *ConfigBackupPO, withContent bool) *biz.ConfigBackup {
	b := &biz.ConfigBackup{
		ID: p.ID, DeviceID: p.DeviceID, DeviceName: p.DeviceName, SHA256: p.SHA256,
		SizeBytes: p.SizeBytes, Changed: p.Changed, Trigger: p.Trigger,
		Status: p.Status, Error: p.Error, CreatedAt: p.CreatedAt,
	}
	if withContent {
		b.Content = p.Content
	}
	return b
}

type backupRepo struct{ data *Data }

func NewBackupRepo(d *Data) biz.BackupRepo { return &backupRepo{data: d} }

func (r *backupRepo) ListBackups(ctx context.Context, deviceID uint, page, size int) ([]*biz.ConfigBackup, int64, error) {
	q := r.data.db.WithContext(ctx).Model(&ConfigBackupPO{})
	if deviceID != 0 {
		q = q.Where("device_id = ?", deviceID)
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
	var pos []ConfigBackupPO
	if err := q.Select("id, device_id, device_name, sha256, size_bytes, changed, trigger, status, error, created_at").
		Order("id desc").Offset((page - 1) * size).Limit(size).Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*biz.ConfigBackup, 0, len(pos))
	for i := range pos {
		out = append(out, backupToBiz(&pos[i], false))
	}
	return out, total, nil
}

func (r *backupRepo) GetBackup(ctx context.Context, id uint) (*biz.ConfigBackup, error) {
	var po ConfigBackupPO
	err := r.data.db.WithContext(ctx).First(&po, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, biz.ErrControlNotFound
	}
	if err != nil {
		return nil, err
	}
	return backupToBiz(&po, true), nil
}

func (r *backupRepo) LatestBackup(ctx context.Context, deviceID uint) (*biz.ConfigBackup, error) {
	var po ConfigBackupPO
	err := r.data.db.WithContext(ctx).
		Where("device_id = ? AND status = ?", deviceID, "ok").
		Order("id desc").First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return backupToBiz(&po, false), nil
}

func (r *backupRepo) CreateBackup(ctx context.Context, b *biz.ConfigBackup) (*biz.ConfigBackup, error) {
	po := &ConfigBackupPO{
		DeviceID: b.DeviceID, DeviceName: b.DeviceName, SHA256: b.SHA256,
		SizeBytes: b.SizeBytes, Changed: b.Changed, Trigger: b.Trigger,
		Status: b.Status, Error: b.Error, Content: b.Content, CreatedAt: b.CreatedAt,
	}
	if err := r.data.db.WithContext(ctx).Create(po).Error; err != nil {
		return nil, err
	}
	return backupToBiz(po, false), nil
}

// ---------- 配置拉取器 (SSH CLI / 模拟模式) ----------

type configFetcher struct{ simulate bool }

func NewConfigFetcher(c *conf.Collector) biz.ConfigFetcher {
	return &configFetcher{simulate: c.GetSimulate()}
}

func (f *configFetcher) FetchRunningConfig(_ context.Context, dev *biz.Device, cred *biz.WriteCredential) (string, error) {
	if f.simulate {
		return fmt.Sprintf(`#
!Software Version %s
sysname %s
#
lldp enable
snmp-agent
snmp-agent trap enable
#
interface Vlanif140
 ip address %s 255.255.255.0
#
return
`, firstNonEmpty(dev.OSVersion, "V200R011C10SPC600"), dev.Name, dev.MgmtIP), nil
	}
	if cred == nil {
		return "", fmt.Errorf("未配置 SSH 写/管理凭据，请先在「管控与审批 → 写凭据」中添加 SSH 账号")
	}
	port := cred.Port
	if port <= 0 || cred.Protocol == biz.ProtoNetconf {
		port = 22
	}
	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", dev.MgmtIP, port), &ssh.ClientConfig{
		User:            cred.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(cred.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	})
	if err != nil {
		return "", fmt.Errorf("ssh dial %s:%d: %w", dev.MgmtIP, port, err)
	}
	defer conn.Close()

	sess, err := conn.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()

	var out bytes.Buffer
	sess.Stdout = &out
	sess.Stderr = &out
	stdin, err := sess.StdinPipe()
	if err != nil {
		return "", err
	}
	if err := sess.Shell(); err != nil {
		return "", err
	}
	_, _ = io.WriteString(stdin, "screen-length 0 temporary\ndisplay current-configuration\nquit\n")
	_ = stdin.Close()
	_ = sess.Wait()
	return out.String(), nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
