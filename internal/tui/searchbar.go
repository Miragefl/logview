package tui

import "fmt"

func (a *App) renderSearchBar() string {
	if a.searchMode {
		q := parseSearchQuery(a.searchInput)
		hint := ""
		if tr := q.TimeRangeHint(); tr != "" {
			hint = fmt.Sprintf("  [时间: %s]", tr)
		}
		return fmt.Sprintf(" 搜索: %s█  [AND/OR] [field:value] [Esc取消] [Enter确认]%s", a.searchInput, hint)
	}
	if a.searchInput != "" {
		q := parseSearchQuery(a.searchInput)
		hint := ""
		if tr := q.TimeRangeHint(); tr != "" {
			hint = fmt.Sprintf("  [时间: %s]", tr)
		}
		return fmt.Sprintf(" 搜索: %s  [%d条匹配 cur:%d] [/修改] [Esc清除]%s",
			a.searchInput, len(a.filteredView), a.cursor, hint)
	}
	return a.renderDetailBar()
}
