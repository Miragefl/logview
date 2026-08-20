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
// 带遮罩：日志区先铺遮罩底色，弹窗浮在上面，视觉聚焦。
func (a *App) overlayToVL(box string, vl int) []string {
	return a.overlayToVLWithMask(box, vl, true)
}

// overlayToVLPlain 居中且不加遮罩（搜索类弹窗保持日志可见）。
func (a *App) overlayToVLPlain(box string, vl int) []string {
	return a.overlayToVLWithMask(box, vl, false)
}

// PopupMaskBg 遮罩底色（ApplyTheme 按主题更新）：dark 压暗、light 提亮。
var PopupMaskBg = lipgloss.Color("#1A1B26")

func (a *App) overlayToVLWithMask(box string, vl int, masked bool) []string {
	base := lipgloss.Place(a.width, vl, lipgloss.Center, lipgloss.Center, box)
	if masked {
		base = lipgloss.NewStyle().Background(PopupMaskBg).Width(a.width).Height(vl).Render(base)
	}
	lines := make([]string, vl)
	overlay := lipgloss.NewStyle().Width(a.width).Height(vl).Render(base)
	parts := strings.Split(overlay, "\n")
	for i := 0; i < vl; i++ {
		if i < len(parts) {
			lines[i] = parts[i]
		}
	}
	return lines
}
