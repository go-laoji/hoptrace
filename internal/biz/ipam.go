package biz

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"sort"
	"strings"

	"hoptrace/internal/pkg/oui"
)

// SubnetSummary 子网 IPAM 统计。
type SubnetSummary struct {
	CIDR          string
	TotalHosts    int
	UsedHosts     int
	FreeHosts     int
	UsagePct      float64
	ConflictCount int
	GatewayDevice string
}

// IPLease 单个 IP 的分配与冲突状态。
type IPLease struct {
	IP           string
	Used         bool
	MAC          string
	MACs         []string
	Conflict     bool
	Vendor       string
	EndpointName string
	DeviceName   string
	IfName       string
}

// SubnetDetail 子网明细。
type SubnetDetail struct {
	Summary SubnetSummary
	Leases  []IPLease
}

// NetworkAnomaly 网络异常(IP 冲突 / 私接小交换机 / MAC 多口漂移)。
type NetworkAnomaly struct {
	Kind       string // ip_conflict / suspect_hub / mac_flap
	Severity   string
	Target     string
	Detail     string
	DeviceID   uint
	DeviceName string
	IfName     string
}

// IpamUsecase IP 地址管理与异常检测。
type IpamUsecase struct {
	devices   DeviceRepo
	endpoints EndpointRepo
}

func NewIpamUsecase(devices DeviceRepo, endpoints EndpointRepo) *IpamUsecase {
	return &IpamUsecase{devices: devices, endpoints: endpoints}
}

// ListSubnets 按 /24 聚合所有 ARP 与设备接口 IP，计算占用率与冲突数。
func (uc *IpamUsecase) ListSubnets(ctx context.Context) ([]SubnetSummary, error) {
	devs, err := uc.devices.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	arps, err := uc.endpoints.ListArp(ctx)
	if err != nil {
		return nil, err
	}

	// 按 /24 归类(若接口带 CIDR 可直接解析，默认按 /24 展示最直观)
	type bucket struct {
		gw   string
		ips  map[string]map[string]bool // ip -> set[mac]
	}
	buckets := map[string]*bucket{}
	getBucket := func(cidr string) *bucket {
		b := buckets[cidr]
		if b == nil {
			b = &bucket{ips: map[string]map[string]bool{}}
			buckets[cidr] = b
		}
		return b
	}

	for _, d := range devs {
		for _, ip := range append([]string{d.MgmtIP}, d.IPs...) {
			cidr := ipv4To24(ip)
			if cidr == "" {
				continue
			}
			b := getBucket(cidr)
			if b.gw == "" && (d.Role == RoleCore || d.Role == RoleFirewall || d.Role == RoleRouter) {
				b.gw = d.Name
			}
			if b.ips[ip] == nil {
				b.ips[ip] = map[string]bool{}
			}
			if d.ChassisMAC != "" {
				b.ips[ip][NormalizeMAC(d.ChassisMAC)] = true
			}
		}
	}
	for _, a := range arps {
		cidr := ipv4To24(a.IP)
		if cidr == "" {
			continue
		}
		b := getBucket(cidr)
		if b.ips[a.IP] == nil {
			b.ips[a.IP] = map[string]bool{}
		}
		if m := NormalizeMAC(a.MAC); m != "" {
			b.ips[a.IP][m] = true
		}
	}

	out := make([]SubnetSummary, 0, len(buckets))
	for cidr, b := range buckets {
		const total = 254
		used := len(b.ips)
		conflicts := 0
		for _, macs := range b.ips {
			if len(macs) > 1 {
				conflicts++
			}
		}
		free := total - used
		if free < 0 {
			free = 0
		}
		pct := math.Round(float64(used)*1000/float64(total)) / 10
		out = append(out, SubnetSummary{
			CIDR:          cidr,
			TotalHosts:    total,
			UsedHosts:     used,
			FreeHosts:     free,
			UsagePct:      pct,
			ConflictCount: conflicts,
			GatewayDevice: b.gw,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CIDR < out[j].CIDR })
	return out, nil
}

// GetSubnetDetail 返回指定 /24 子网内全部 1..254 主机 IP 的使用与冲突明细。
func (uc *IpamUsecase) GetSubnetDetail(ctx context.Context, cidr string) (*SubnetDetail, error) {
	ip, ipNet, err := net.ParseCIDR(strings.TrimSpace(cidr))
	if err != nil || ip.To4() == nil {
		return nil, ErrDiscoveryInvalid
	}
	arps, err := uc.endpoints.ListArp(ctx)
	if err != nil {
		return nil, err
	}
	eps, err := uc.endpoints.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	devs, err := uc.devices.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	devName := map[uint]string{}
	for _, d := range devs {
		devName[d.ID] = d.Name
	}
	epByMAC := map[string]*Endpoint{}
	for _, e := range eps {
		epByMAC[e.MAC] = e
	}

	ipMACs := map[string]map[string]bool{}
	for _, a := range arps {
		p := net.ParseIP(a.IP)
		if p == nil || !ipNet.Contains(p) {
			continue
		}
		if ipMACs[a.IP] == nil {
			ipMACs[a.IP] = map[string]bool{}
		}
		if m := NormalizeMAC(a.MAC); m != "" {
			ipMACs[a.IP][m] = true
		}
	}

	ones, bits := ipNet.Mask.Size()
	hostCount := int(uint32(1)<<uint(bits-ones)) - 2
	if hostCount > 254 {
		hostCount = 254
	}
	base := binary.BigEndian.Uint32(ipNet.IP.To4())

	leases := make([]IPLease, 0, hostCount)
	used, conflicts := 0, 0
	for i := 1; i <= hostCount; i++ {
		b := make(net.IP, 4)
		binary.BigEndian.PutUint32(b, base+uint32(i))
		ipStr := b.String()
		mset := ipMACs[ipStr]
		if len(mset) == 0 {
			leases = append(leases, IPLease{IP: ipStr, Used: false})
			continue
		}
		used++
		macs := make([]string, 0, len(mset))
		for m := range mset {
			macs = append(macs, m)
		}
		sort.Strings(macs)
		conflict := len(macs) > 1
		if conflict {
			conflicts++
		}
		primary := macs[0]
		lease := IPLease{
			IP: ipStr, Used: true, MAC: primary, MACs: macs, Conflict: conflict,
			Vendor: oui.Lookup(primary),
		}
		if ep := epByMAC[primary]; ep != nil {
			lease.EndpointName = ep.Name
			lease.DeviceName = devName[ep.DeviceID]
			lease.IfName = ep.IfName
		}
		leases = append(leases, lease)
	}

	pct := 0.0
	if hostCount > 0 {
		pct = math.Round(float64(used)*1000/float64(hostCount)) / 10
	}
	return &SubnetDetail{
		Summary: SubnetSummary{
			CIDR:          cidr,
			TotalHosts:    hostCount,
			UsedHosts:     used,
			FreeHosts:     hostCount - used,
			UsagePct:      pct,
			ConflictCount: conflicts,
		},
		Leases: leases,
	}, nil
}

// ListAnomalies 聚合检测三类网络异常: IP 冲突 / 疑似私接小交换机 / MAC 在多个边缘口同时出现(环路/漂移)。
func (uc *IpamUsecase) ListAnomalies(ctx context.Context) ([]NetworkAnomaly, error) {
	devs, err := uc.devices.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	ifs, err := uc.devices.ListAllInterfaces(ctx)
	if err != nil {
		return nil, err
	}
	arps, err := uc.endpoints.ListArp(ctx)
	if err != nil {
		return nil, err
	}
	fdb, err := uc.endpoints.ListFdb(ctx)
	if err != nil {
		return nil, err
	}

	devName := map[uint]string{}
	for _, d := range devs {
		devName[d.ID] = d.Name
	}
	ifByKey := map[IfKey]*Interface{}
	var out []NetworkAnomaly

	// 1. 疑似私接小交换机/HUB
	for _, it := range ifs {
		ifByKey[IfKey{it.DeviceID, it.IfIndex}] = it
		if it.Kind == IfKindSuspect {
			out = append(out, NetworkAnomaly{
				Kind:       "suspect_hub",
				Severity:   SeverityWarning,
				Target:     fmt.Sprintf("%s %s", devName[it.DeviceID], it.Name),
				Detail:     fmt.Sprintf("无 LLDP 邻居但学习到 %d 个 MAC，疑似私接小交换机/HUB", it.MACCount),
				DeviceID:   it.DeviceID,
				DeviceName: devName[it.DeviceID],
				IfName:     it.Name,
			})
		}
	}

	// 2. IP 冲突(同一 IP 对应多个不同 MAC)
	ipMACs := map[string]map[string]bool{}
	for _, a := range arps {
		if !ValidEndpointIP(a.IP) {
			continue
		}
		if ipMACs[a.IP] == nil {
			ipMACs[a.IP] = map[string]bool{}
		}
		if m := NormalizeMAC(a.MAC); m != "" {
			ipMACs[a.IP][m] = true
		}
	}
	for ip, mset := range ipMACs {
		if len(mset) > 1 {
			macs := make([]string, 0, len(mset))
			for m := range mset {
				macs = append(macs, m)
			}
			sort.Strings(macs)
			out = append(out, NetworkAnomaly{
				Kind:     "ip_conflict",
				Severity: SeverityCritical,
				Target:   ip,
				Detail:   fmt.Sprintf("检测到 %d 个不同 MAC 争用同一 IP: %s", len(macs), strings.Join(macs, ", ")),
			})
		}
	}

	// 3. MAC 同时出现在同一台交换机的多个边缘口(疑似二层环路或频繁漂移)
	macEdgePorts := map[string]map[string]bool{}
	for _, f := range fdb {
		it := ifByKey[IfKey{f.DeviceID, f.IfIndex}]
		if it == nil || (it.Kind != IfKindEdge && it.Kind != IfKindSuspect) {
			continue
		}
		m := NormalizeMAC(f.MAC)
		if !ValidEndpointMAC(m) {
			continue
		}
		if macEdgePorts[m] == nil {
			macEdgePorts[m] = map[string]bool{}
		}
		macEdgePorts[m][fmt.Sprintf("%s:%s", devName[f.DeviceID], it.Name)] = true
	}
	for m, ports := range macEdgePorts {
		if len(ports) > 1 {
			ps := make([]string, 0, len(ports))
			for p := range ports {
				ps = append(ps, p)
			}
			sort.Strings(ps)
			out = append(out, NetworkAnomaly{
				Kind:     "mac_flap",
				Severity: SeverityWarning,
				Target:   m,
				Detail:   "同一 MAC 同时出现在多个边缘端口: " + strings.Join(ps, ", "),
			})
		}
	}

	return out, nil
}

func ipv4To24(ipStr string) string {
	p := net.ParseIP(strings.TrimSpace(ipStr)).To4()
	if p == nil || !ValidEndpointIP(ipStr) {
		return ""
	}
	return fmt.Sprintf("%d.%d.%d.0/24", p[0], p[1], p[2])
}
