package tui

import (
	"fmt"
	"strings"

	"github.com/justfun/logview/internal/model"
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
			{"Tab", "切换分区"},
			{"C-j/C-k", "切换字段"},
			{"C-u", "清空输入"},
			{"Esc", "取消"},
		}
		if a.searchTab == 0 && a.searchInput != "" {
			items = append(items, helpItem{"", fmt.Sprintf("[匹配: %d条]", len(a.filteredView))})
		}
		items = append(items, a.activeFilterBadges()...)
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
		items = append(items, a.activeFilterBadges()...)
		if a.searchInput != "" {
			items = append(items, helpItem{"", fmt.Sprintf("[搜索: %s]", a.searchInput)})
		}
		if a.yankMsg != "" {
			items = append(items, helpItem{"", NewLogStyle.Render(a.yankMsg)})
		}
		return items
	}
}

// activeFilterBadges 返回持久过滤器（级别/隐藏）的醒目状态提示。
// 在所有模式（含搜索模式）下都显示，避免用户误激活过滤器后误以为行数显示有 bug
// （如误按 Tab 切到 hide tab 输词回车，hides 激活却毫无察觉）。
func (a *App) activeFilterBadges() []helpItem {
	var items []helpItem
	if a.levelFilter != "" {
		items = append(items, helpItem{"", LevelStyle(a.levelFilter).Render(fmt.Sprintf("[过滤: %s]", a.levelFilter))})
	}
	if len(a.hides) > 0 {
		words := runewidth.Truncate(strings.Join(a.hides, ","), 16, "…")
		items = append(items, helpItem{"", HideMarkStyle.Render(fmt.Sprintf("[隐藏:%s 藏%d行]", words, a.hiddenByHides))})
	}
	return items
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
	return runewidth.StringWidth(model.StripANSI(s))
}
