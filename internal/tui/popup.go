package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
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

// overlayToVL 把 box 居中放置在日志区上：底层渲染日志行，弹窗覆盖中部，
// 弹窗上下/左右的日志保持可见（可边看日志边操作）。切分成恰好 vl 行。
func (a *App) overlayToVL(box string, vl int) []string {
	boxLines := strings.Split(box, "\n")
	bh := len(boxLines)
	if bh > vl {
		boxLines = boxLines[:vl]
		bh = vl
	}
	bw := 0
	for _, l := range boxLines {
		if w := lipgloss.Width(l); w > bw {
			bw = w
		}
	}
	// 垂直 1/3 处起（偏上），水平居中（以内容区宽度为基准，弹窗在边框内居中）
	cw := a.contentWidth()
	top := (vl - bh) / 3
	if top < 0 {
		top = 0
	}
	left := (cw - bw) / 2
	if left < 0 {
		left = 0
	}

	logLines := a.buildLogLines(vl)
	out := make([]string, vl)
	for i := 0; i < vl; i++ {
		var log string
		if i < len(logLines) {
			log = logLines[i]
		}
		if i >= top && i < top+bh {
			bl := boxLines[i-top]
			leftPart := truncateVisible(log, left)
			// 日志行不足 left 宽时补空格，保证弹窗水平位置稳定
			if pad := left - lipgloss.Width(leftPart); pad > 0 {
				leftPart += strings.Repeat(" ", pad)
			}
			rightStart := left + lipgloss.Width(bl)
			rightPart := ""
			if rightStart < cw {
				rightPart = truncateVisible(stripANSIPrefix(log, rightStart), cw-rightStart)
			}
			out[i] = leftPart + bl + rightPart
		} else {
			out[i] = log
		}
	}
	return out
}

// overlayToVLPlain 兼容保留：与 overlayToVL 相同（日志透出）。
func (a *App) overlayToVLPlain(box string, vl int) []string {
	return a.overlayToVL(box, vl)
}

// truncateVisible 按显示宽度截取行（保留行内 ANSI 序列，在宽度处截断）。
func truncateVisible(line string, w int) string {
	if w <= 0 {
		return ""
	}
	var b strings.Builder
	acc := 0
	inEsc := false
	for _, r := range line {
		if inEsc {
			b.WriteRune(r)
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		if r == '\x1b' {
			inEsc = true
			b.WriteRune(r)
			continue
		}
		rw := runewidth.RuneWidth(r)
		if acc+rw > w {
			break
		}
		acc += rw
		b.WriteRune(r)
	}
	return b.String()
}

// stripANSIPrefix 跳过行首指定显示宽度后返回剩余部分（ANSI 感知）。
func stripANSIPrefix(line string, w int) string {
	acc := 0
	inEsc := false
	for i, r := range line {
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		if r == '\x1b' {
			inEsc = true
			continue
		}
		acc += runewidth.RuneWidth(r)
		if acc >= w {
			rest := line[i+len(string(r)):]
			return rest
		}
	}
	return ""
}
