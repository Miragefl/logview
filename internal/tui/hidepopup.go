package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (a *App) buildHidePopup(vl int) []string {
	lines := make([]string, vl)

	var content strings.Builder
	content.WriteString(HelpKeyStyle.Render("隐藏关键词") + "\n\n")

	if len(a.hides) > 0 {
		content.WriteString(DetailDimStyle.Render("当前隐藏:") + "\n")
		for _, kw := range a.hides {
			content.WriteString(fmt.Sprintf("  %s %s\n", HideMarkStyle.Render("✕"), DetailDimStyle.Render(kw)))
		}
		content.WriteString("\n")
	}

	inputDisplay := a.hideInput
	if inputDisplay == "" {
		inputDisplay = DetailDimStyle.Render("输入关键词，逗号分隔...")
	}
	content.WriteString(fmt.Sprintf(" %s█\n", inputDisplay))

	content.WriteString("\n" + PopupTabStyle.Render(" Enter确认 Esc取消 C-u清空"))

	boxW := min(50, a.width-4)
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
