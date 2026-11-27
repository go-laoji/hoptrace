package biz

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

var hexOnly = regexp.MustCompile(`[^0-9a-fA-F]`)

// NormalizeMAC 将各种格式的 MAC 规范化为小写冒号分隔(aa:bb:cc:dd:ee:ff);非法返回空串。
func NormalizeMAC(s string) string {
	h := hexOnly.ReplaceAllString(s, "")
	if len(h) != 12 {
		return ""
	}
	h = strings.ToLower(h)
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s", h[0:2], h[2:4], h[4:6], h[6:8], h[8:10], h[10:12])
}

// FormatMAC 将 6 字节格式化为 MAC 字符串。
func FormatMAC(b []byte) string {
	if len(b) != 6 {
		return ""
	}
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", b[0], b[1], b[2], b[3], b[4], b[5])
}

// ValidEndpointMAC 判断是否为可作为终端的单播 MAC(排除全 0、组播/广播)。
func ValidEndpointMAC(mac string) bool {
	m := NormalizeMAC(mac)
	if m == "" || m == "00:00:00:00:00:00" {
		return false
	}
	var first int
	fmt.Sscanf(m[:2], "%x", &first)
	return first&1 == 0
}

// ValidEndpointIP 过滤 0.0.0.0 / 链路本地 / 组播等无意义的 IP。
func ValidEndpointIP(ip string) bool {
	p := net.ParseIP(ip)
	if p == nil || p.IsUnspecified() || p.IsLinkLocalUnicast() || p.IsMulticast() || p.IsLoopback() {
		return false
	}
	return true
}

// canonIfName 规范化接口名用于匹配:小写、去空白、展开华为常见缩写(GE/XGE/Eth...)。
func canonIfName(s string) string {
	s = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
	for _, r := range []struct{ short, long string }{
		{"xge", "xgigabitethernet"},
		{"ge", "gigabitethernet"},
		{"fe", "fastethernet"},
		{"eth-trunk", "eth-trunk"},
		{"eth", "ethernet"},
	} {
		if strings.HasPrefix(s, r.short) && len(s) > len(r.short) && s[len(r.short)] >= '0' && s[len(r.short)] <= '9' {
			return r.long + s[len(r.short):]
		}
	}
	return s
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
