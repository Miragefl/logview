package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justfun/logview/internal/model"
)

// handleDetailKeys 详情面板按键：j/k 联动移动光标（面板内容跟随），Esc/d/Enter/q 关闭。
func (a *App) handleDetailKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter", "q", "d":
		a.detailMode = false
	case "j", "down":
		if a.cursor < len(a.filteredView)-1 {
			a.cursor++
			a.detailClampOffset()
		}
	case "k", "up":
		if a.cursor > 0 {
			a.cursor--
			a.detailClampOffset()
		}
	case "y":
		a.yankLines(a.cursor, a.cursor)
	}
	return a, nil
}

// detailClampOffset 面板模式下移动光标后把视口拉回可见范围。
func (a *App) detailClampOffset() {
	vl := a.visibleLines() - 1 // 表头占 1 行
	if vl < 1 {
		vl = 1
	}
	if a.cursor < a.offset {
		a.offset = a.cursor
	} else if a.cursor >= a.offset+vl {
		a.offset = a.cursor - vl + 1
	}
}

// buildDetailPanel d 键详情面板：完整字段 + 消息全文（wrap，JSON 自动美化）+ 原始行。
func (a *App) buildDetailPanel(vl int) []string {
	if len(a.filteredView) == 0 || a.cursor < 0 || a.cursor >= len(a.filteredView) {
		return a.buildLogLines(vl)
	}
	line := a.filteredView[a.cursor]

	boxW := min(60, a.width-4)
	inner := boxW - 8 // padding/边框/缩进预留
	if inner < 20 {
		inner = 20
	}

	var content strings.Builder
	row := func(label, val string) {
		content.WriteString(fmt.Sprintf("%s %s\n",
			DetailLabelStyle.Render(fmt.Sprintf("%-7s", label)),
			DetailValueStyle.Render(val)))
	}
	if !line.Time.IsZero() {
		row("time", line.Time.Format("2006-01-02 15:04:05.000"))
	}
	if line.Level != "" {
		row("level", line.Level)
	}
	if line.Raw.Source != "" {
		row("source", line.Raw.Source)
	}
	if line.Thread != "" {
		row("thread", line.Thread)
	}
	if line.TraceID != "" {
		row("traceId", line.TraceID)
	}
	if line.Logger != "" {
		row("logger", line.Logger)
	}

	msg := lineMsg(line)
	if p := prettyPrintJSON(msg); p != msg {
		msg = p // JSON 消息自动美化缩进
	}
	content.WriteString(DetailLabelStyle.Render("message") + "\n")
	for _, wl := range wrapAnsiText(msg, inner) {
		content.WriteString("  " + DetailValueStyle.Render(wl) + "\n")
	}
	content.WriteString(DetailLabelStyle.Render("raw") + "\n")
	for _, wl := range wrapAnsiText(line.Raw.Text, inner) {
		content.WriteString("  " + DetailDimStyle.Render(wl) + "\n")
	}

	box := PopupBoxStyle.Width(boxW).Render(strings.TrimRight(content.String(), "\n"))
	return a.overlayToVL(box, vl)
}

// lineMsg 行正文：解析出的 message，无则回退原始行。
func lineMsg(line *model.ParsedLine) string {
	if line.Message != "" {
		return line.Message
	}
	return line.Raw.Text
}

func prettyPrintJSON(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return s
	}
	if s[0] != '{' && s[0] != '[' {
		return s
	}
	var buf bytes.Buffer
	if json.Indent(&buf, []byte(s), "", "  ") == nil {
		return buf.String()
	}
	return s
}

// compactDetailJSON 把多行 JSON 压缩成单行（详情栏单行显示，避免撑破布局）。
func compactDetailJSON(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 2 || (s[0] != '{' && s[0] != '[') {
		return s
	}
	var out bytes.Buffer
	if json.Compact(&out, []byte(s)) == nil {
		return out.String()
	}
	return s
}
