package data

import (
	"fmt"
	"os"
	"path/filepath"

	"hoptrace/internal/conf"

	"github.com/glebarez/sqlite"
	"github.com/go-kratos/kratos/v3/log"
	"github.com/google/wire"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(
	NewData,
	NewCredentialRepo,
	NewDeviceRepo,
	NewTopologyRepo,
	NewEndpointRepo,
	NewJobRepo,
	NewStatsRepo,
	NewUserRepo,
	NewControlRepo,
	NewAlertRepo,
	NewAlertNotifier,
	NewBackupRepo,
	NewConfigFetcher,
	NewSiteRepo,
	NewCollector,
	NewControlExecutor,
)

// Data 持有长生命周期的 GORM 客户端与字段加密器。
type Data struct {
	db     *gorm.DB
	cipher *Cipher
}

// NewData 打开数据库(支持 sqlite / postgres)并按配置执行 AutoMigrate。
func NewData(c *conf.Data, sec *conf.Security) (*Data, func(), error) {
	dc := c.GetDatabase()
	driver := dc.GetDriver()
	source := dc.GetSource()
	if driver == "" {
		driver = "sqlite"
	}
	if source == "" {
		source = "data/hoptrace.db"
	}

	var dialector gorm.Dialector
	switch driver {
	case "sqlite":
		if source != ":memory:" {
			if dir := filepath.Dir(source); dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return nil, nil, err
				}
			}
		}
		dialector = sqlite.Open(source + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	case "postgres", "postgresql":
		dialector = postgres.Open(source)
	default:
		return nil, nil, fmt.Errorf("unsupported database driver: %s", driver)
	}

	gormCfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	if dc.GetDebug() {
		gormCfg.Logger = logger.Default.LogMode(logger.Info)
	}
	db, err := gorm.Open(dialector, gormCfg)
	if err != nil {
		return nil, nil, err
	}
	if driver == "sqlite" {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.SetMaxOpenConns(1) // 避免 SQLite 并发写锁冲突
		}
	}

	if dc.GetAutoMigrate() {
		if err := db.AutoMigrate(
			&CredentialPO{},
			&DevicePO{},
			&InterfacePO{},
			&NeighborPO{},
			&FdbPO{},
			&ArpPO{},
			&LinkPO{},
			&EndpointPO{},
			&EndpointLocationPO{},
			&EndpointEventPO{},
			&DiscoveryJobPO{},
			&UserPO{},
			&WriteCredentialPO{},
			&ControlActionPO{},
			&AlertPO{},
			&AlertChannelPO{},
			&SyslogPO{},
			&ConfigBackupPO{},
			&InterfaceMetricPO{},
			&WirelessSTAPO{},
			&SitePO{},
		); err != nil {
			return nil, nil, err
		}
	}

	cipher, err := NewCipher(sec.GetSecretKey())
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		log.Info("closing the data resources")
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	return &Data{db: db, cipher: cipher}, cleanup, nil
}
