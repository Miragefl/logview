package tui

import "fmt"

func (a *App) renderSearchBar() string {
	if a.searchMode {
		return SearchStyle.Render(fmt.Sprintf(" 搜索: %s█", a.searchInput))
	}
	if a.searchInput != "" {
		return SearchStyle.Render(fmt.Sprintf(" 搜索: %s", a.searchInput))
	}
	return SearchStyle.Render(" 按 / 搜索")
}