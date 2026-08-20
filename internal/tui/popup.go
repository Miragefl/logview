package tui

import (
	"fmt"
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

// PopupMaskBgHex 遮罩底色 hex（ApplyTheme 按主题更新）：dark 压暗、light 提亮。
var PopupMaskBgHex = "#1A1B26"

// maskSeq 返回遮罩背景 ANSI 序列（空色返回空串）。
func maskSeq() string {
	if PopupMaskBgHex == "" {
		return ""
	}
	r, g, b := hexToRGB(PopupMaskBgHex)
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
}

func (a *App) overlayToVLWithMask(box string, vl int, masked bool) []string {
	base := lipgloss.Place(a.width, vl, lipgloss.Center, lipgloss.Center, box)
	lines := make([]string, vl)
	parts := strings.Split(base, "\n")
	seq := ""
	if masked {
		seq = maskSeq()
	}
	for i := 0; i < vl; i++ {
		l := ""
		if i < len(parts) {
			l = parts[i]
		}
		if seq != "" {
			pad := a.width - lipgloss.Width(l)
			if pad > 0 {
				l += strings.Repeat(" ", pad)
			}
			// 弹窗行自带背景：遮罩色置于行首，被弹窗自身的色序列覆盖是预期
			l = seq + l + "\x1b[49m"
		}
		lines[i] = l
	}
	return lines
}
