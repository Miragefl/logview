package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (a *App) buildHighlightPopup(vl int) []string {
	lines := make([]string, vl)

	var content strings.Builder
	content.WriteString(HelpKeyStyle.Render("高亮关键词") + "\n\n")

	if len(a.highlights) > 0 {
		content.WriteString(DetailDimStyle.Render("当前高亮:") + "\n")
		for i, kw := range a.highlights {
			colorIdx := i % len(HighlightColors)
			style := lipgloss.NewStyle().Background(HighlightColors[colorIdx]).Foreground(lipgloss.Color("0"))
			content.WriteString(fmt.Sprintf("  %s %s\n", style.Render(" "+kw+" "), DetailDimStyle.Render(kw)))
		}
		content.WriteString("\n")
	}

	inputDisplay := a.highlightInput
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
