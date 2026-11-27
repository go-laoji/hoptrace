package biz

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// ConfigBackup 设备运行配置备份 DO。
type ConfigBackup struct {
	ID         uint
	DeviceID   uint
	DeviceName string
	SHA256     string
	SizeBytes  int
	Changed    bool
	Trigger    string // manual / scheduled
	Status     string // ok / failed
	Error      string
	Content    string
	CreatedAt  time.Time
}

// DiffLine 配置变更对比行。
type DiffLine struct {
	Kind    string // context / add / del
	OldLine int
	NewLine int
	Text    string
}

// DiffResult 两版本配置对比结果。
type DiffResult struct {
	From    *ConfigBackup
	To      *ConfigBackup
	Added   int
	Removed int
	Lines   []DiffLine
}

type BackupRepo interface {
	ListBackups(ctx context.Context, deviceID uint, page, size int) ([]*ConfigBackup, int64, error)
	GetBackup(ctx context.Context, id uint) (*ConfigBackup, error)
	LatestBackup(ctx context.Context, deviceID uint) (*ConfigBackup, error)
	CreateBackup(ctx context.Context, b *ConfigBackup) (*ConfigBackup, error)
}

// ConfigFetcher 从华为设备拉取运行配置(display current-configuration)或模拟配置。
type ConfigFetcher interface {
	FetchRunningConfig(ctx context.Context, dev *Device, cred *WriteCredential) (string, error)
}

// BackupUsecase 配置备份与版本 Diff。
type BackupUsecase struct {
	repo    BackupRepo
	devices DeviceRepo
	ctrl    ControlRepo
	fetcher ConfigFetcher
}

func NewBackupUsecase(repo BackupRepo, devices DeviceRepo, ctrl ControlRepo, fetcher ConfigFetcher) *BackupUsecase {
	return &BackupUsecase{repo: repo, devices: devices, ctrl: ctrl, fetcher: fetcher}
}

func (uc *BackupUsecase) List(ctx context.Context, deviceID uint, page, size int) ([]*ConfigBackup, int64, error) {
	return uc.repo.ListBackups(ctx, deviceID, page, size)
}

func (uc *BackupUsecase) Get(ctx context.Context, id uint) (*ConfigBackup, error) {
	return uc.repo.GetBackup(ctx, id)
}

// Run 立即备份指定设备运行配置，并与上次成功备份做 SHA256 比对标记是否发生变更。
func (uc *BackupUsecase) Run(ctx context.Context, deviceID uint, trigger string) (*ConfigBackup, error) {
	dev, err := uc.devices.Get(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	if trigger == "" {
		trigger = "manual"
	}
	var cred *WriteCredential
	if creds, err := uc.ctrl.ListWriteCredentials(ctx); err == nil && len(creds) > 0 {
		cred = creds[0]
		for _, c := range creds {
			if c.Protocol == ProtoSSH {
				cred = c
				break
			}
		}
	}

	content, fetchErr := uc.fetcher.FetchRunningConfig(ctx, dev, cred)
	b := &ConfigBackup{
		DeviceID:   dev.ID,
		DeviceName: dev.Name,
		Trigger:    trigger,
		CreatedAt:  time.Now(),
	}
	if fetchErr != nil {
		b.Status = "failed"
		b.Error = fetchErr.Error()
		return uc.repo.CreateBackup(ctx, b)
	}

	sum := sha256.Sum256([]byte(content))
	hash := hex.EncodeToString(sum[:])
	changed := true
	if prev, err := uc.repo.LatestBackup(ctx, dev.ID); err == nil && prev != nil && prev.SHA256 == hash {
		changed = false
	}
	b.Status = "ok"
	b.SHA256 = hash
	b.SizeBytes = len(content)
	b.Changed = changed
	b.Content = content
	return uc.repo.CreateBackup(ctx, b)
}

// Diff 计算两个配置版本的行级差异。
func (uc *BackupUsecase) Diff(ctx context.Context, fromID, toID uint) (*DiffResult, error) {
	a, err := uc.repo.GetBackup(ctx, fromID)
	if err != nil {
		return nil, err
	}
	b, err := uc.repo.GetBackup(ctx, toID)
	if err != nil {
		return nil, err
	}
	lines, added, removed := ComputeLineDiff(a.Content, b.Content)
	return &DiffResult{From: a, To: b, Added: added, Removed: removed, Lines: lines}, nil
}

// ComputeLineDiff 简洁高效的 LCS 行级 Diff 算法。
func ComputeLineDiff(oldText, newText string) ([]DiffLine, int, int) {
	a := strings.Split(strings.ReplaceAll(oldText, "\r\n", "\n"), "\n")
	b := strings.Split(strings.ReplaceAll(newText, "\r\n", "\n"), "\n")
	n, m := len(a), len(b)
	// 防止超大配置 O(n*m) 内存膨胀: 超过 4000 行退化为逐行比对
	if n*m > 4_000_000 {
		var out []DiffLine
		added, removed := 0, 0
		maxL := n
		if m > maxL {
			maxL = m
		}
		for i := 0; i < maxL; i++ {
			switch {
			case i < n && i < m && a[i] == b[i]:
				out = append(out, DiffLine{Kind: "context", OldLine: i + 1, NewLine: i + 1, Text: a[i]})
			default:
				if i < n {
					out = append(out, DiffLine{Kind: "del", OldLine: i + 1, Text: a[i]})
					removed++
				}
				if i < m {
					out = append(out, DiffLine{Kind: "add", NewLine: i + 1, Text: b[i]})
					added++
				}
			}
		}
		return out, added, removed
	}

	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	var out []DiffLine
	added, removed := 0, 0
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && a[i] == b[j]:
			out = append(out, DiffLine{Kind: "context", OldLine: i + 1, NewLine: j + 1, Text: a[i]})
			i++
			j++
		case j < m && (i == n || dp[i][j+1] >= dp[i+1][j]):
			out = append(out, DiffLine{Kind: "add", NewLine: j + 1, Text: b[j]})
			added++
			j++
		default:
			out = append(out, DiffLine{Kind: "del", OldLine: i + 1, Text: a[i]})
			removed++
			i++
		}
	}
	return out, added, removed
}
