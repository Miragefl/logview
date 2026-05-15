package tui

import (
	"fmt"
	"strings"

	"github.com/justfun/logview/internal/model"
)

func (a *App) renderLogView() string {
	vl := a.visibleLines()
	if vl < 1 {
		vl = 1
	}

	if a.autoscroll && len(a.filteredView) > 0 {
		a.cursor = len(a.filteredView) - 1
	}

	start := a.cursor - vl/2
	if start < 0 {
		start = 0
	}
	end := start + vl
	if end > len(a.filteredView) {
		end = len(a.filteredView)
	}
	a.offset = start

	var b strings.Builder
	for i := start; i < end; i++ {
		line := a.filteredView[i]
		folded := false
		for _, g := range a.stGroups {
			if i > g.Start && i <= g.End && !a.expanded[g.Start] {
				if i == g.Start+1 {
					b.WriteString(FoldedStyle.Render(fmt.Sprintf("  (%d lines folded) [e展开]", g.End-g.Start)))
					b.WriteByte('\n')
				}
				folded = true
				break
			}
		}
		if folded {
			continue
		}

		text := a.renderLine(line, i == a.cursor)
		b.WriteString(text)
		b.WriteByte('\n')
	}

	// 填充剩余行，保证日志区域高度一致
	for i := end - start; i < vl; i++ {
		b.WriteByte('\n')
	}

	if a.newLogs > 0 && !a.autoscroll {
		b.WriteString(NewLogStyle.Render(fmt.Sprintf(" [新日志: %d条] 按g跳转", a.newLogs)))
	}
	return b.String()
}

func (a *App) renderLine(line *model.ParsedLine, selected bool) string {
	var parts []string
	for _, f := range model.AllFields {
		if !a.fieldMask.IsVisible(f) {
			continue
		}
		val := line.Get(f)
		if val == "" {
			continue
		}
		switch f {
		case model.FieldLevel:
			parts = append(parts, LevelStyle(val).Render(val))
		case model.FieldSource:
			parts = append(parts, fmt.Sprintf("[%s]", val))
		default:
			parts = append(parts, val)
		}
	}
	text := strings.Join(parts, "  ")
	if a.searchInput != "" {
		text = highlightText(text, a.searchInput)
	}
	if selected {
		return SelectedStyle.Width(a.width).Render(text)
	}
	return text
}

func highlightText(text, query string) string {
	if query == "" {
		return text
	}
	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(query)
	qLen := len(lowerQuery)
	var result strings.Builder
	i := 0
	for i <= len(lowerText)-qLen {
		if lowerText[i:i+qLen] == lowerQuery {
			result.WriteString(HighlightStyle.Render(text[i : i+qLen]))
			i += qLen
		} else {
			result.WriteByte(text[i])
			i++
		}
	}
	for ; i < len(text); i++ {
		result.WriteByte(text[i])
	}
	return result.String()
}
