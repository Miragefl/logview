package tui

import (
	"fmt"
	"strings"
)

func (a *App) buildHidePopup(vl int) []string {
	var content strings.Builder
	content.WriteString(HelpKeyStyle.Render("隐藏关键词") + "\n\n")

	if len(a.hides) > 0 {
		content.WriteString(DetailDimStyle.Render("当前隐藏:") + "\n")
		for _, kw := range a.hides {
			content.WriteString(fmt.Sprintf("  %s %s\n", HideMarkStyle.Render("✕"), DetailDimStyle.Render(kw)))
		}
		content.WriteString("\n")
	}

	content.WriteString(a.inputLine(a.hideInput, a.hideCursor, "输入关键词，逗号分隔...") + "\n")
	content.WriteString("\n" + PopupTabStyle.Render(" Enter确认 Esc取消 C-u清空"))

	boxW := min(50, a.width-4)
	box := PopupBoxStyle.Width(boxW).Render(content.String())
	return a.overlayToVL(box, vl)
}
