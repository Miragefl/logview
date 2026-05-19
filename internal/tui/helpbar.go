package tui

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
)

type helpItem struct {
	key  string
	desc string
}

func (a *App) helpItems() []helpItem {
	switch {
	case a.searchMode:
		items := []helpItem{
			{"Enter", "确认"},
			{"Tab/S-Tab", "切换字段"},
			{"C-j/C-k", "上下字段"},
			{"C-u", "清空输入"},
			{"Esc", "取消"},
		}
		if a.searchInput != "" {
			items = append(items, helpItem{"", fmt.Sprintf("[匹配: %d条]", len(a.filteredView))})
		}
		return items
	case a.visualMode:
		return []helpItem{
			{"j/k", "上下移动"},
			{"g/G", "顶/底"},
			{"y", "复制选中"},
			{"Esc", "退出选择"},
		}
	case a.panelFocus:
		return []helpItem{
			{"↑/k ↓/j", "移动"},
			{"Space/Enter", "切换显示"},
			{"Esc/q", "关闭"},
		}
	case a.exportMode:
		return []helpItem{
			{"↑/k ↓/j", "移动"},
			{"←/h →/l", "切换选项"},
			{"Enter", "导出"},
			{"Esc/q", "关闭"},
		}
	default:
		items := []helpItem{
			{"j/k/g/G", "移动"},
			{"C-d/C-u", "半页翻"},
			{"C-f/C-b", "翻页"},
			{"/", "搜索"},
			{"v/V", "选择"},
			{"y", "复制"},
			{"H/M/L", "屏顶/中/底"},
			{"zt/zz/zb", "置顶/居中/置底"},
			{"F", "字段"},
			{"s", "导出"},
			{"E/W/I/D/A", "级别"},
			{"h", "高亮"},
			{"x", "隐藏"},
			{"w", "换行"},
			{"e", "展开"},
			{"S-c", "清屏"},
				{"?", "帮助"},
		}
		if a.levelFilter != "" {
			items = append(items, helpItem{"", LevelStyle(a.levelFilter).Render(fmt.Sprintf("[过滤: %s]", a.levelFilter))})
		}
		if a.searchInput != "" {
			items = append(items, helpItem{"", fmt.Sprintf("[搜索: %s]", a.searchInput)})
		}
		if len(a.hides) > 0 {
			items = append(items, helpItem{"", fmt.Sprintf("[隐藏: %d词]", len(a.hides))})
		}
		if a.yankMsg != "" {
			items = append(items, helpItem{"", NewLogStyle.Render(a.yankMsg)})
		}
		return items
	}
}

// renderHelpBarContent returns 1-2 lines of help text.
func (a *App) renderHelpBarContent() string {
	items := a.helpItems()
	var parts []string
	for _, it := range items {
		if it.key == "" {
			parts = append(parts, it.desc)
		} else {
			parts = append(parts, fmt.Sprintf("%s%s", HelpKeyStyle.Render(it.key), HelpStyle.Render(it.desc)))
		}
	}

	full := strings.Join(parts, "  ")

	// measure display width (not byte length)
	if displayWidth(full) <= a.width {
		return full
	}

	// split into two lines at the midpoint that fits
	mid := len(parts) / 2
	line1 := strings.Join(parts[:mid], "  ")
	line2 := strings.Join(parts[mid:], "  ")
	return line1 + "\n" + line2
}

// helpBarHeight returns how many lines the help bar occupies.
func (a *App) helpBarHeight() int {
	items := a.helpItems()
	total := 0
	for i, it := range items {
		if i > 0 {
			total += 2
		}
		total += runewidth.StringWidth(it.key) + runewidth.StringWidth(it.desc)
	}
	if total > a.width {
		return 2
	}
	return 1
}

func displayWidth(s string) int {
	return runewidth.StringWidth(stripAnsi(s))
}

func stripAnsi(s string) string {
	var result strings.Builder
	esc := false
	for _, c := range s {
		if c == '\x1b' {
			esc = true
			continue
		}
		if esc {
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				esc = false
			}
			continue
		}
		result.WriteRune(c)
	}
	return result.String()
}
