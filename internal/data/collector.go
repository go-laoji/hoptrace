package data

import (
	"hoptrace/internal/biz"
	"hoptrace/internal/conf"
	"hoptrace/internal/data/snmpx"
	"hoptrace/internal/data/snmpx/sim"

	"github.com/go-kratos/kratos/v3/log"
)

// NewCollector 根据配置返回真实 gosnmp 采集器或内置华为风格模拟器。
func NewCollector(c *conf.Collector) biz.Collector {
	if c.GetSimulate() {
		log.Info("collector running in SIMULATE mode (demo Huawei network 10.255.0.0/24)")
		return snmpx.NewCollector(sim.NewDialer())
	}
	d := &snmpx.GoSNMPDialer{
		Timeout:        c.GetTimeout().AsDuration(),
		Retries:        int(c.GetRetries()),
		MaxRepetitions: uint32(c.GetMaxRepetitions()),
	}
	return snmpx.NewCollector(d)
}
