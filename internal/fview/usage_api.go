// 源热点/内存徽章的桌面端绑定逻辑（Go 侧实现，fview/app.go 薄转发）。
package fview

import (
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/Miragefl/logview/internal/usage"
)

// usageRecentLimit 本地面板最近源列表长度上限。
const usageRecentLimit = 50

// UsageRecent 返回源热点 key 列表（带 file:/ssh: 等前缀，衰减分降序），
// 供 fview 本地文件面板与欢迎页展示。file: 项过滤已消失的本地文件——只有
// os.IsNotExist 才剔（权限等其他 stat 错误保留，点击时既有静默兜底，误杀比
// 漏显示伤）；远程 key（ssh:/frp:/k8sres:）无法本地检测，原样保留；
// 持久化数据不动（7 天半衰期自清，外接盘/临时路径文件可能回来）。
func UsageRecent() []string {
	keys := usage.RecentSourceKeys(usageRecentLimit)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if strings.HasPrefix(k, "file:") {
			if _, err := os.Stat(k[len("file:"):]); os.IsNotExist(err) {
				continue
			}
		}
		out = append(out, k)
	}
	return out
}

// UsageTouch 记录一次源使用（key 由前端按前缀规范拼接，如 file:/path/app.log）。
func UsageTouch(key string) {
	usage.Bump(key)
}

// UsageScores 批量取衰减分（panel-hot-sort 三面板排序数据源；7 天半衰期，
// 与 TUI sortCandidatesHot 同一口径）。缺失 key 不出现在结果——前端 ?? 0 归一。
func UsageScores(keys []string) map[string]float64 {
	m := usage.Load()
	now := time.Now()
	out := make(map[string]float64, len(keys))
	for _, k := range keys {
		if e, ok := m[k]; ok {
			out[k] = e.Decayed(now)
		}
	}
	return out
}

// MemStatsBytes 状态栏内存徽章口径：Sys - HeapReleased（对齐 TUI perf-1g-load 口径，
// 返回字节数，前端换算 MB）。
func MemStatsBytes() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Sys - m.HeapReleased
}
