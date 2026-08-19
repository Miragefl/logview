package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// buildPopupLines returns exactly vl lines for the field settings popup overlay.
func (a *App) buildPopupLines(vl int) []string {
	panelContent := a.renderFieldsPanel()
	hint := PopupTabStyle.Render("[Up/Down] [Space] [Esc]")
	content := "字段显示设置\n\n" + panelContent + "\n\n" + hint

	boxW := min(60, a.width-6)
	box := PopupBoxStyle.Width(boxW).Render(content)
	return a.overlayToVL(box, vl)
}

// overlayToVL 把 box 居中放置在 vl 高的区域并切分成恰好 vl 行（各弹窗共用）。
func (a *App) overlayToVL(box string, vl int) []string {
	lines := make([]string, vl)
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
