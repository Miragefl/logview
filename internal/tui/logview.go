package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/justfun/logview/internal/model"
)

// buildLogLines returns exactly vl rendered lines for the log area.
func (a *App) buildLogLines(vl int) []string {
	if vl < 1 {
		vl = 1
	}
	lines := make([]string, vl)

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
		start = max(0, end-vl)
	}
	a.offset = start

	idx := 0
	for i := start; i < end; i++ {
		line := a.filteredView[i]
		folded := false
		for _, g := range a.stGroups {
			if i > g.Start && i <= g.End && !a.expanded[g.Start] {
				if i == g.Start+1 {
					lines[idx] = FoldedStyle.Render(fmt.Sprintf("  (%d lines folded) [e展开]", g.End-g.Start))
				}
				folded = true
				break
			}
		}
		if !folded {
			lines[idx] = a.renderLine(line, i == a.cursor)
		}
		idx++
	}

	if a.newLogs > 0 && !a.autoscroll && idx < vl {
		lines[vl-1] = NewLogStyle.Render(fmt.Sprintf(" [新日志: %d条] 按g跳转", a.newLogs))
	}

	return lines
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
		q := a.currentQuery()
		for _, kw := range q.HighlightKeywords() {
			text = highlightText(text, kw)
		}
	}
	if selected {
		// MaxWidth truncates first, Width fills background — Width alone WRAPS!
		truncated := lipgloss.NewStyle().MaxWidth(a.width).Render(text)
		return SelectedStyle.Width(a.width).Render(truncated)
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
