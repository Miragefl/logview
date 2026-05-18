package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (a *App) buildExportPopup(vl int) []string {
	lines := make([]string, vl)
	s := a.exportState

	var content strings.Builder
	content.WriteString(HelpKeyStyle.Render("导出日志") + "\n\n")

	scopeMarker := " "
	if s.Cursor == 0 {
		scopeMarker = "▸"
	}
	scopeOpts := []string{
		fmt.Sprintf("○ 当前筛选(%d条)", len(a.filteredView)),
		fmt.Sprintf("○ 全部缓冲(%d条)", a.buffer.Len()),
	}
	scopeOpts[s.Scope] = strings.Replace(scopeOpts[s.Scope], "○", "●", 1)
	content.WriteString(fmt.Sprintf("%s 范围:  %s  %s\n", scopeMarker, scopeOpts[0], scopeOpts[1]))

	formatMarker := " "
	if s.Cursor == 1 {
		formatMarker = "▸"
	}
	formatOpts := []string{"○ 原始", "○ JSON"}
	formatOpts[s.Format] = strings.Replace(formatOpts[s.Format], "○", "●", 1)
	content.WriteString(fmt.Sprintf("%s 格式:  %s  %s\n", formatMarker, formatOpts[0], formatOpts[1]))

	pathMarker := " "
	if s.Cursor == 2 {
		pathMarker = "▸"
	}
	content.WriteString(fmt.Sprintf("%s 路径:  %s\n", pathMarker, s.FilePath))

	if s.Done {
		content.WriteString(fmt.Sprintf("\n✓ 已导出 %d 条 → %s", s.Exported, s.FilePath))
	}

	content.WriteString("\n\n" + PopupTabStyle.Render(" Enter确认  Esc取消  ←→切换"))

	boxW := min(52, a.width-4)
	box := PopupBoxStyle.Width(boxW).Render(content.String())

	overlay := lipgloss.NewStyle().Width(a.width).Height(vl).
		Render(lipgloss.Place(a.width, vl,
			lipgloss.Center, lipgloss.Center,
			box))

	parts := strings.Split(overlay, "\n")
	for i := 0; i < vl; i++ {
		if i < len(parts) {
			lines[i] = parts[i]
		}
	}
	return lines
}
