package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/justfun/logview/internal/model"
)

func (a *App) renderFieldsPanel() string {
	var items []string
	for i, f := range model.AllFields {
		cb := CheckboxUnchecked
		if a.fieldMask.IsVisible(f) {
			cb = CheckboxChecked
		}
		item := fmt.Sprintf("%s %s", cb, f)
		if a.panelFocus && i == a.fieldCursor {
			item = SelectedStyle.Render(item)
		}
		items = append(items, item)
	}
	return strings.Join(items, "\n")
}

func (a *App) buildStatsPanel(vl int) []string {
	// calculate stats from filteredView
	counts := map[string]int{
		"DEBUG": 0, "INFO": 0, "WARN": 0, "ERROR": 0, "OTHER": 0,
	}
	for _, line := range a.filteredView {
		lv := strings.ToUpper(line.Level)
		switch {
		case lv == "DEBUG" || lv == "DBG":
			counts["DEBUG"]++
		case lv == "INFO":
			counts["INFO"]++
		case lv == "WARN" || lv == "WARNING":
			counts["WARN"]++
		case lv == "ERROR" || lv == "ERR" || lv == "FATAL":
			counts["ERROR"]++
		default:
			counts["OTHER"]++
		}
	}

	total := len(a.filteredView)
	bufTotal := a.buffer.Len()

	var rows []string
	for _, lvl := range []string{"ERROR", "WARN", "INFO", "DEBUG"} {
		c := counts[lvl]
		pct := 0.0
		if total > 0 {
			pct = float64(c) / float64(total) * 100
		}
		rows = append(rows, fmt.Sprintf("  %-8s %6d (%5.1f%%)", lvl, c, pct))
	}
	if counts["OTHER"] > 0 {
		pct := float64(counts["OTHER"]) / float64(total) * 100
		rows = append(rows, fmt.Sprintf("  %-8s %6d (%5.1f%%)", "OTHER", counts["OTHER"], pct))
	}

	rows = append(rows, strings.Repeat("─", 30))
	rows = append(rows, fmt.Sprintf("  显示: %d  总计: %d  隐藏: %d", total, bufTotal, bufTotal-total))

	content := "统计\n\n" + strings.Join(rows, "\n") + "\n\n" + PopupTabStyle.Render("[S或Esc关闭]")
	boxW := min(40, a.width-6)
	box := PopupBoxStyle.Width(boxW).Render(content)

	overlay := lipgloss.NewStyle().Width(a.width).Height(vl).
		Render(lipgloss.Place(a.width, vl,
			lipgloss.Center, lipgloss.Center,
			box))

	lines := make([]string, vl)
	parts := strings.Split(overlay, "\n")
	for i := 0; i < vl; i++ {
		if i < len(parts) {
			lines[i] = parts[i]
		}
	}
	return lines
}
