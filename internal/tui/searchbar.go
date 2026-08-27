package tui

import (
	"fmt"
)

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
		hint := ""
		if tr := q.TimeRangeHint(); tr != "" {
			hint = fmt.Sprintf("  [时间: %s]", tr)
		}
		fieldHint := ""
		if len(a.starFields) > 0 {
			fieldHint = " [Tab插入字段]"
		}
		return fmt.Sprintf(" %s: %s█%s  [Esc取消] [Enter确认]%s%s", label, before, after, hint, fieldHint)
	}
	if a.searchInput != "" {
		q := parseSearchQuery(a.searchInput)
		hint := ""
		if tr := q.TimeRangeHint(); tr != "" {
			hint = fmt.Sprintf("  [时间: %s]", tr)
		}
		return fmt.Sprintf(" 搜索: %s  [%d/%d匹配] [/修改] [Esc清除]%s",
			a.searchInput, a.searchMatchIdx, a.searchMatchCount, hint)
	}
	return ""
}
