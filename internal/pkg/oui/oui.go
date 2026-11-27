// Package oui 提供 MAC 前 3 字节(OUI)到厂商名称的查询。
// 数据来自 IEEE MA-L 注册表(scripts/oui_convert.py 转换),通过 go:embed 内置。
package oui

import (
	_ "embed"
	"strings"
	"sync"
)

//go:embed oui.tsv
var data string

var (
	once  sync.Once
	table map[string]string
)

func load() {
	table = make(map[string]string, 40000)
	for _, line := range strings.Split(data, "\n") {
		if i := strings.IndexByte(line, '\t'); i == 6 {
			table[line[:6]] = line[7:]
		}
	}
}

// Lookup 返回 MAC 对应的厂商;未知返回空串。接受任意常见 MAC 格式。
func Lookup(mac string) string {
	once.Do(load)
	var b strings.Builder
	for _, r := range mac {
		switch {
		case r >= '0' && r <= '9', r >= 'A' && r <= 'F':
			b.WriteRune(r)
		case r >= 'a' && r <= 'f':
			b.WriteRune(r - 'a' + 'A')
		}
		if b.Len() >= 6 {
			break
		}
	}
	key := b.String()
	if len(key) < 6 {
		return ""
	}
	return table[key[:6]]
}
