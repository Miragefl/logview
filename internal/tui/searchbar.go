package tui

import (
	"fmt"
)

func (a *App) renderSearchBar() string {
	if a.searchMode {
		q := parseSearchQuery(a.searchInput)
		hint := ""
		if tr := q.TimeRangeHint(); tr != "" {
			hint = fmt.Sprintf("  [时间: %s]", tr)
		}
		fieldHint := ""
		if len(a.starFields) > 0 {
			fieldHint = " [Tab插入字段]"
		}
		// insert cursor indicator at searchCursor position
		runes := []rune(a.searchInput)
		pos := a.searchCursor
		if pos > len(runes) {
			pos = len(runes)
		}
		before := string(runes[:pos])
		after := string(runes[pos:])
		return fmt.Sprintf(" 搜索: %s█%s  [Esc取消] [Enter确认]%s%s", before, after, hint, fieldHint)
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
	return a.renderDetailBar()
}
