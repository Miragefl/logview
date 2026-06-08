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
			parts = append(parts, fmt.Sprintf("%s %s",
				DetailLabelStyle.Render(string(f)+":"),
				DetailDimStyle.Render("-")))
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
	msg = prettyPrintJSON(msg)
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
