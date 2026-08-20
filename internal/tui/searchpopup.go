package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// buildSearchPopup returns the popup box lines (only the box itself, no full-area fill).
// maxPopupRows caps the box height so the popup never eats the whole log area;
// starFields rows are trimmed first to keep matching log lines visible.
func (a *App) buildSearchPopup(maxPopupRows int) []string {
	var content strings.Builder

	// tab bar: 搜索 | 高亮 | 隐藏
	tabs := []string{"搜索", "高亮", "隐藏"}
	var tabParts []string
	for i, t := range tabs {
		if i == a.searchTab {
			tabParts = append(tabParts, PopupActiveTabStyle.Render(" "+t+" "))
		} else {
			tabParts = append(tabParts, PopupTabStyle.Render(" "+t+" "))
		}
	}
	content.WriteString(strings.Join(tabParts, PopupTabStyle.Render("│")) + "\n\n")

	// 固定行预算：tab 区(2) + starFields 后空行(1) + input(1) + 空行提示(2) + 圆角边框(2) = 8
	const fixedRows = 8
	maxStarRows := maxPopupRows - fixedRows
	if maxStarRows < 0 {
		maxStarRows = 0
	}

	switch a.searchTab {
	case 1:
		a.renderHighlightSection(&content)
	case 2:
		a.renderHideSection(&content)
	default:
		a.renderSearchSection(&content, maxStarRows)
	}

	return a.popupBox(content.String())
}

func (a *App) renderSearchSection(content *strings.Builder, maxStarRows int) {
	if a.searchHistMode {
		a.renderSearchHistoryList(content)
		content.WriteString(a.inputLine(a.searchInput, a.searchCursor, "输入搜索词，支持 field:value AND/OR") + "\n")
		content.WriteString("\n" + PopupTabStyle.Render(" Tab切分区 j/k选择 Enter填入 Esc取消"))
		return
	}
	if len(a.starFields) > 0 {
		nRows := len(a.starFields)
		if nRows > 6 {
			nRows = 6
		}
		if nRows > maxStarRows {
			nRows = maxStarRows
		}
		for i := 0; i < nRows; i++ {
			sf := a.starFields[i]
			prefix := "  "
			if i == a.starCursor {
				prefix = SelectedStyle.Render(" >")
			}
			if sf.Name == "" {
				content.WriteString(prefix + " " + HelpKeyStyle.Render("确认搜索") + "\n")
			} else {
				name := DetailLabelStyle.Render(sf.Name + ":")
				val := DetailValueStyle.Render(sf.Value)
				content.WriteString(prefix + " " + name + " " + val + "\n")
			}
		}
		content.WriteString("\n")
	}
	content.WriteString(a.inputLine(a.searchInput, a.searchCursor, "输入搜索词，支持 field:value AND/OR") + "\n")
	content.WriteString("\n" + PopupTabStyle.Render(" Tab切分区 C-j/k字段 Enter确认 C-r历史 Esc取消"))
}

// renderSearchHistoryList 渲染倒序历史列表（最新在上），最多 8 行，光标跟随滚动。
// 列表内容随当前分区（搜索/高亮/隐藏）取对应历史。
func (a *App) renderSearchHistoryList(content *strings.Builder) {
	hist := a.currentTabHistory()
	n := len(hist)
	if n == 0 {
		return
	}
	content.WriteString(HelpKeyStyle.Render("历史") + "\n")
	maxRows := 8
	if n < maxRows {
		maxRows = n
	}
	// 滚动窗口起点：保证 searchHistCursor 可见
	start := 0
	if a.searchHistCursor > maxRows-1 {
		start = a.searchHistCursor - maxRows + 1
	}
	for i := 0; i < maxRows; i++ {
		row := start + i // 列表第 i 个可见行
		if row >= n {
			break
		}
		histIdx := n - 1 - row // 倒序映射到 searchHistory
		prefix := "  "
		if row == a.searchHistCursor {
			prefix = SelectedStyle.Render(" >")
		}
		content.WriteString(prefix + " " + DetailDimStyle.Render(hist[histIdx]) + "\n")
	}
	content.WriteString("\n")
}

func (a *App) renderHighlightSection(content *strings.Builder) {
	if a.searchHistMode {
		a.renderSearchHistoryList(content)
		content.WriteString(a.inputLine(a.highlightInput, a.highlightCursor, "高亮关键词，逗号分隔") + "\n")
		content.WriteString("\n" + PopupTabStyle.Render(" Tab切分区 j/k选择 Enter填入 Esc取消"))
		return
	}
	if len(a.highlights) > 0 {
		content.WriteString(DetailDimStyle.Render("当前高亮:") + "\n")
		for i, kw := range a.highlights {
			colorIdx := i % len(HighlightColors)
			style := lipgloss.NewStyle().Background(HighlightColors[colorIdx]).Foreground(lipgloss.Color("0"))
			content.WriteString(fmt.Sprintf("  %s\n", style.Render(" "+kw+" ")))
		}
		content.WriteString("\n")
	}
	content.WriteString(a.inputLine(a.highlightInput, a.highlightCursor, "高亮关键词，逗号分隔") + "\n")
	content.WriteString("\n" + PopupTabStyle.Render(" Tab切分区 Enter确认 C-r历史 Esc取消"))
}

func (a *App) renderHideSection(content *strings.Builder) {
	if a.searchHistMode {
		a.renderSearchHistoryList(content)
		content.WriteString(a.inputLine(a.hideInput, a.hideCursor, "隐藏关键词，逗号分隔") + "\n")
		content.WriteString("\n" + PopupTabStyle.Render(" Tab切分区 j/k选择 Enter填入 Esc取消"))
		return
	}
	if len(a.hides) > 0 {
		content.WriteString(DetailDimStyle.Render("当前隐藏:") + "\n")
		for _, kw := range a.hides {
			content.WriteString(fmt.Sprintf("  %s %s\n", HideMarkStyle.Render("✕"), DetailDimStyle.Render(kw)))
		}
		content.WriteString("\n")
	}
	content.WriteString(a.inputLine(a.hideInput, a.hideCursor, "隐藏关键词，逗号分隔") + "\n")
	content.WriteString("\n" + PopupTabStyle.Render(" Tab切分区 Enter确认 C-r历史 Esc取消"))
}

// inputLine renders an input field with a cursor block at cursor position.
func (a *App) inputLine(input string, cursor int, placeholder string) string {
	if input == "" {
		return fmt.Sprintf(" %s█", DetailDimStyle.Render(placeholder))
	}
	runes := []rune(input)
	pos := cursor
	if pos > len(runes) {
		pos = len(runes)
	}
	return fmt.Sprintf(" %s█%s", string(runes[:pos]), string(runes[pos:]))
}

// popupBox wraps content in a centered popup box and splits into lines.
func (a *App) popupBox(content string) []string {
	boxW := min(60, a.width-4)
	box := PopupBoxStyle.Width(boxW).Render(content)
	return strings.Split(box, "\n")
}
