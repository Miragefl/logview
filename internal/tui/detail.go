package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/justfun/logview/internal/model"
)

func (a *App) renderDetailBar() string {
	if len(a.filteredView) == 0 || a.cursor < 0 || a.cursor >= len(a.filteredView) {
		return DetailDimStyle.Render(" 选中日志行查看详情")
	}
	line := a.filteredView[a.cursor]
	if line == nil {
		return ""
	}

	var parts []string
	for _, f := range model.AllFields {
		val := line.Get(f)
		if val == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s",
			DetailLabelStyle.Render(string(f)+":"),
			DetailValueStyle.Render(val)))
	}

	msg := line.Message
	if msg == "" {
		msg = line.Raw.Text
	}
	msg = compactDetailJSON(msg)
	parts = append(parts, fmt.Sprintf("%s %s",
		DetailLabelStyle.Render("msg:"),
		DetailValueStyle.Render(msg)))

	return strings.Join(parts, "  ")
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
