package tui

import "github.com/Miragefl/logview/internal/logcore"

// SearchQuery 搜索查询——实现已迁 internal/logcore（fview 桌面端共享，单源不分叉），
// tui 保留类型别名与包内私有同名委托，包内调用点零改动。
type SearchQuery = logcore.SearchQuery

var (
	// 包内私有委托（tui 调用点沿用原小写名）
	parseSearchQuery   = logcore.ParseSearchQuery
	isPartialQuery     = logcore.IsPartialQuery
	strippedQuery      = logcore.StrippedQuery
	containsIgnoreCase = logcore.ContainsIgnoreCase
)
