package biz

import (
	"hoptrace/internal/conf"

	"github.com/google/wire"
)

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(
	NewOptions,
	NewCredentialUsecase,
	NewDeviceUsecase,
	NewTopologyUsecase,
	NewEndpointUsecase,
	NewPollUsecase,
	NewDiscoveryUsecase,
	NewDashboardUsecase,
	NewAuthUsecase,
)

// NewOptions 从配置构建 biz 运行参数。
func NewOptions(c *conf.Collector) Options {
	if c == nil {
		return Options{}.withDefaults()
	}
	var pollInterval = c.GetPollInterval().AsDuration()
	return Options{
		PollInterval:        pollInterval,
		Concurrency:         int(c.GetConcurrency()),
		SuspectMACThreshold: int(c.GetSuspectMacThreshold()),
		OfflineMissRounds:   int(c.GetOfflineMissRounds()),
		MoveConfirmRounds:   int(c.GetMoveConfirmRounds()),
		DisablePoller:       c.GetDisablePoller(),
	}.withDefaults()
}
