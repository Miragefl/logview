package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (a *App) buildHighlightPopup(vl int) []string {
	var content strings.Builder
	content.WriteString(HelpKeyStyle.Render("高亮关键词") + "\n\n")

	if len(a.highlights) > 0 {
		content.WriteString(DetailDimStyle.Render("当前高亮:") + "\n")
		for i, kw := range a.highlights {
			colorIdx := i % len(HighlightColors)
			style := lipgloss.NewStyle().Background(HighlightColors[colorIdx]).Foreground(lipgloss.Color("0")) // dark text on highlight
			content.WriteString(fmt.Sprintf("  %s %s\n", style.Render(" "+kw+" "), DetailDimStyle.Render(kw)))
		}
		content.WriteString("\n")
	}

	content.WriteString(a.inputLine(a.highlightInput, a.highlightCursor, "输入关键词，逗号分隔...") + "\n")
	content.WriteString("\n" + PopupTabStyle.Render(" Enter确认 Esc取消 C-u清空"))

	boxW := min(50, a.width-4)
	box := PopupBoxStyle.Width(boxW).Render(content.String())
	return a.overlayToVL(box, vl)
}
