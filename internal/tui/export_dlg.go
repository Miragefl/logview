package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (a *App) renderExportDialog() string {
	s := a.exportState
	dlgStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2)

	var b strings.Builder
	b.WriteString("导出日志\n\n")

	scopeMarker := " "
	if s.Cursor == 0 {
		scopeMarker = "▸"
	}
	scopeOpts := []string{
		fmt.Sprintf("○ 当前筛选结果(%d条)", len(a.filteredView)),
		fmt.Sprintf("○ 全部缓冲区(%d条)", a.buffer.Len()),
	}
	scopeOpts[s.Scope] = strings.Replace(scopeOpts[s.Scope], "○", "●", 1)
	b.WriteString(fmt.Sprintf("%s 范围: %s  %s\n", scopeMarker, scopeOpts[0], scopeOpts[1]))

	formatMarker := " "
	if s.Cursor == 1 {
		formatMarker = "▸"
	}
	formatOpts := []string{"○ 原始日志", "○ 结构化(JSON)"}
	formatOpts[s.Format] = strings.Replace(formatOpts[s.Format], "○", "●", 1)
	b.WriteString(fmt.Sprintf("%s 格式: %s  %s\n", formatMarker, formatOpts[0], formatOpts[1]))

	pathMarker := " "
	if s.Cursor == 2 {
		pathMarker = "▸"
	}
	b.WriteString(fmt.Sprintf("%s 路径: %s\n", pathMarker, s.FilePath))

	if s.Done {
		b.WriteString(fmt.Sprintf("\n✓ 已导出 %d 条 → %s", s.Exported, s.FilePath))
	}
	b.WriteString("\n\n[Enter 确认]  [Esc 取消]")
	return dlgStyle.Render(b.String())
}