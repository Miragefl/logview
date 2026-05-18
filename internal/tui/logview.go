package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/justfun/logview/internal/model"
)

// buildLogLines returns rendered lines for the log area.
// The cursor line wraps to show full content, other lines are truncated.
func (a *App) buildLogLines(vl int) []string {
	if vl < 1 {
		vl = 1
	}

	if a.autoscroll && len(a.filteredView) > 0 {
		a.cursor = len(a.filteredView) - 1
	}

	// pre-render cursor line to know its wrap height
	cursorWrapped := []string{""}
	cursorHeight := 1
	if a.cursor >= 0 && a.cursor < len(a.filteredView) {
		cursorWrapped = a.renderLineWrapped(a.filteredView[a.cursor], a.cursor)
		cursorHeight = len(cursorWrapped)
	}

	// how many single-line entries fit alongside the wrapped cursor
	singleSlots := vl - cursorHeight
	if singleSlots < 0 {
		singleSlots = 0
	}

	// cursor near bottom: show cursor at bottom, fill above
	// cursor in middle: center cursor
	start := max(0, a.cursor-singleSlots)
	a.offset = start

	var lines []string

	// fill before cursor
	for i := start; i < a.cursor; i++ {
		if a.isFolded(i) {
			lines = append(lines, FoldedStyle.Render(fmt.Sprintf("  (%d lines folded) [e展开]", a.foldedCount(i))))
		} else {
			lines = append(lines, a.renderLine(a.filteredView[i], false, i))
		}
	}

	// cursor line(s)
	lines = append(lines, cursorWrapped...)

	// fill after cursor until we reach vl
	for i := a.cursor + 1; i < len(a.filteredView) && len(lines) < vl; i++ {
		if a.isFolded(i) {
			lines = append(lines, FoldedStyle.Render(fmt.Sprintf("  (%d lines folded) [e展开]", a.foldedCount(i))))
		} else {
			lines = append(lines, a.renderLine(a.filteredView[i], false, i))
		}
	}

	// pad or trim to exactly vl
	for len(lines) < vl {
		lines = append(lines, "")
	}
	if len(lines) > vl {
		lines = lines[:vl]
	}

	return lines
}

func (a *App) isFolded(lineIdx int) bool {
	for _, g := range a.stGroups {
		if lineIdx > g.Start && lineIdx <= g.End && !a.expanded[g.Start] {
			return true
		}
	}
	return false
}

func (a *App) foldedCount(lineIdx int) int {
	for _, g := range a.stGroups {
		if lineIdx > g.Start && lineIdx <= g.End && !a.expanded[g.Start] {
			return g.End - g.Start
		}
	}
	return 0
}

// renderLineWrapped renders the cursor line without truncation, wrapped to terminal width.
func (a *App) renderLineWrapped(line *model.ParsedLine, lineIdx int) []string {
	text := a.renderLineText(line)
	w := a.width
	if w < 1 {
		w = 1
	}
	wrapped := wrapAnsiText(text, w)
	for i, l := range wrapped {
		wrapped[i] = SelectedStyle.Width(w).Render(l)
	}
	return wrapped
}

// renderLineText builds the full text of a line without truncation or selection styling.
func (a *App) renderLineText(line *model.ParsedLine) string {
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
			parts = append(parts, SourceStyle.Render(fmt.Sprintf("[%s]", val)))
		case model.FieldTime:
			parts = append(parts, TimeStyle.Render(val))
		case model.FieldTraceID:
			parts = append(parts, TraceIDStyle.Render(val))
		case model.FieldThread:
			parts = append(parts, ThreadStyle.Render(val))
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
	for i, kw := range a.highlights {
		if kw == "" {
			continue
		}
		colorIdx := i % len(HighlightColors)
		style := lipgloss.NewStyle().Background(HighlightColors[colorIdx]).Foreground(lipgloss.Color("0"))
		text = highlightTextWithStyle(text, kw, style)
	}
	return text
}

func (a *App) renderLine(line *model.ParsedLine, selected bool, lineIdx int) string {
	text := a.renderLineText(line)
	inVisualRange := a.visualMode && lineIdx >= min(a.visualStart, a.cursor) && lineIdx <= max(a.visualStart, a.cursor)
	if selected && !inVisualRange {
		truncated := lipgloss.NewStyle().MaxWidth(a.width).Render(text)
		return SelectedStyle.Width(a.width).Render(truncated)
	}
	if inVisualRange {
		truncated := lipgloss.NewStyle().MaxWidth(a.width).Render(text)
		return VisualStyle.Width(a.width).Render(truncated)
	}
	if line.Level == "ERROR" || line.Level == "ERR" || line.Level == "FATAL" {
		truncated := lipgloss.NewStyle().MaxWidth(a.width).Render(text)
		return lipgloss.NewStyle().Background(ErrorLineBg).Width(a.width).Render(truncated)
	}
	if line.Level == "WARN" || line.Level == "WARNING" {
		truncated := lipgloss.NewStyle().MaxWidth(a.width).Render(text)
		return lipgloss.NewStyle().Background(WarnLineBg).Width(a.width).Render(truncated)
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

func highlightTextWithStyle(text, query string, style lipgloss.Style) string {
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
			result.WriteString(style.Render(text[i : i+qLen]))
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

// wrapAnsiText wraps text at the given display width, preserving ANSI escape codes.
func wrapAnsiText(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	var lines []string
	var cur strings.Builder
	col := 0
	i := 0
	runes := []rune(text)
	for i < len(runes) {
		if runes[i] == '\x1b' {
			cur.WriteRune(runes[i])
			i++
			for i < len(runes) {
				cur.WriteRune(runes[i])
				if (runes[i] >= 'a' && runes[i] <= 'z') || (runes[i] >= 'A' && runes[i] <= 'Z') {
					i++
					break
				}
				i++
			}
			continue
		}
		if col >= width {
			lines = append(lines, cur.String())
			cur.Reset()
			col = 0
		}
		cur.WriteRune(runes[i])
		if runes[i] > 0x7f {
			col += 2
		} else {
			col++
		}
		i++
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	if len(lines) == 0 {
		lines = append(lines, "")
	}
	return lines
}
