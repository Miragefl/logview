package tui

import "fmt"

func (a *App) renderSearchBarContent() string {
	if a.searchMode {
		return fmt.Sprintf(" 搜索: %s█  [Esc取消] [Enter确认]", a.searchInput)
	}
	if a.searchInput != "" {
		return fmt.Sprintf(" 搜索: %s  [/修改] [Esc清除]", a.searchInput)
	}
	return " 按 / 搜索"
}
