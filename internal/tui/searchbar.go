package tui

import (
	"fmt"
)

// timeFeedback 时间条件反馈：解析错误红字优先，其次显示实际解析出的绝对时间窗口。
// 输入中间态（time: / time:> 等悬空）不闪错误，确定错误才红字。
func (a *App) timeFeedback(q SearchQuery) string {
	if pe := q.ParseError(); pe != "" && !isPartialQuery(a.searchInput) {
		return "  [" + LevelError.Render("✗ time: "+pe) + "]"
	}
	if tr := q.TimeRangeHint(); tr != "" {
		return fmt.Sprintf("  [时间: %s]", tr)
	}
	return ""
}

func (a *App) renderSearchBar() string {
	if a.searchMode {
		labels := []string{"搜索", "高亮", "隐藏"}
		label := labels[a.searchTab]
		input := a.activeSearchInput()
		runes := []rune(*input.text)
		pos := *input.cursor
		if pos > len(runes) {
			pos = len(runes)
		}
		before := string(runes[:pos])
		after := string(runes[pos:])
		if a.searchTab != 0 {
			return fmt.Sprintf(" %s: %s█%s  [Esc取消] [Enter确认]", label, before, after)
		}
		q := parseSearchQuery(a.searchInput)
		fieldHint := ""
		if len(a.starFields) > 0 {
			fieldHint = " [Tab插入字段]"
		}
		return fmt.Sprintf(" %s: %s█%s  [Esc取消] [Enter确认]%s%s", label, before, after, a.timeFeedback(q), fieldHint)
	}
	if a.searchInput != "" {
		q := parseSearchQuery(a.searchInput)
		return fmt.Sprintf(" 搜索: %s  [%d/%d匹配] [/修改] [Esc清除]%s",
			a.searchInput, a.searchMatchIdx, a.searchMatchCount, a.timeFeedback(q))
	}
	return ""
}
