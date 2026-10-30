// 源热点/内存徽章的桌面端绑定逻辑（Go 侧实现，fview/app.go 薄转发）。
package fview

import (
	"runtime"

	"github.com/justfun/logview/internal/usage"
)

// usageRecentLimit 本地面板最近源列表长度上限。
const usageRecentLimit = 50

// UsageRecent 返回源热点 key 列表（带 file:/ssh: 等前缀，衰减分降序），
// 供 fview 本地文件面板与欢迎页展示。
func UsageRecent() []string {
	return usage.RecentSourceKeys(usageRecentLimit)
}

// UsageTouch 记录一次源使用（key 由前端按前缀规范拼接，如 file:/path/app.log）。
func UsageTouch(key string) {
	usage.Bump(key)
}

// MemStatsBytes 状态栏内存徽章口径：Sys - HeapReleased（对齐 TUI perf-1g-load 口径，
// 返回字节数，前端换算 MB）。
func MemStatsBytes() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Sys - m.HeapReleased
}
