package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/justfun/logview/internal/model"
	"github.com/justfun/logview/internal/stacktrace"
)

const scrollOff = 5

// buildLogLines returns rendered lines for the log area.
func (a *App) buildLogLines(vl int) []string {
	if vl < 1 {
		vl = 1
	}

	if a.autoscroll && len(a.filteredView) > 0 {
		a.cursor = len(a.filteredView) - 1
	}

	if a.wrapMode {
		return a.buildWrapLines(vl)
	}

	// --- normal mode: cursor wraps, other lines truncated ---

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

	// calculate start based on scroll anchor, counting visual lines (not indices)
	var start int
	switch a.scrollAnchor {
	case 1: // zt: cursor at top
		start = a.cursor
	case 2: // zz: cursor at center
		start = a.visualStartFrom(a.cursor, singleSlots/2)
	case 3: // zb: cursor at bottom
		start = a.visualStartFrom(a.cursor, singleSlots)
	default: // auto: scrolloff
		if a.autoscroll {
			start = a.visualStartFrom(a.cursor, singleSlots)
		} else {
			start = a.offset
			beforeCursor := a.visualLinesBetween(start, a.cursor)
			if beforeCursor < scrollOff {
				start = a.visualStartFrom(a.cursor, scrollOff)
			} else if beforeCursor > vl-scrollOff-cursorHeight {
				start = a.visualStartFrom(a.cursor, vl-scrollOff-cursorHeight)
			}
		}
	}
	a.offset = start

	var lines []string
	rendered := make(map[int]bool)

	addLine := func(idx int) {
		if g := a.foldedGroup(idx); g != nil && !rendered[g.Start] {
			rendered[g.Start] = true
			hint := "e展开"
			if a.expanded[g.Start] {
				hint = "e收起"
			}
			lines = append(lines, FoldedStyle.Render(fmt.Sprintf("  (%d lines) [%s]", g.End-g.Start, hint)))
		} else if g == nil {
			lines = append(lines, a.renderLine(a.filteredView[idx], false, idx))
		}
	}

	// fill before cursor
	for i := start; i < a.cursor; i++ {
		addLine(i)
	}

	// cursor line(s)
	lines = append(lines, cursorWrapped...)

	// fill after cursor until we reach vl
	for i := a.cursor + 1; i < len(a.filteredView) && len(lines) < vl; i++ {
		addLine(i)
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

// buildWrapLines renders all lines with word wrap enabled.
func (a *App) buildWrapLines(vl int) []string {
	w := a.width
	if w < 1 {
		w = 1
	}
	var start int
	switch a.scrollAnchor {
	case 1:
		start = a.cursor
	case 2:
		start = a.visualStartFromWrap(a.cursor, vl/2)
	case 3:
		start = a.visualStartFromWrap(a.cursor, vl-1)
	default:
		if a.autoscroll {
			start = a.visualStartFromWrap(a.cursor, vl-1)
		} else {
			start = a.offset
			// cursor jumped above viewport
			if a.cursor < start {
				start = a.cursor
			} else if a.cursor > start {
				// count visual rows from start, check if cursor beyond viewport
				rows := 0
				seen := make(map[int]bool)
				for i := start; i <= a.cursor && i < len(a.filteredView); i++ {
					if g := a.foldedGroup(i); g != nil {
						if seen[g.Start] {
							continue
						}
						seen[g.Start] = true
						rows++
						i = g.End
						continue
					}
					text := a.renderLineText(a.filteredView[i])
					rows += len(wrapAnsiText(text, w))
				}
				if rows > vl {
					start = a.visualStartFromWrap(a.cursor, vl-1)
				}
			}
		}
	}
	a.offset = start

	var lines []string
	rendered := make(map[int]bool)

	for i := start; i < len(a.filteredView) && len(lines) < vl; i++ {
		// folded group handling
		if g := a.foldedGroup(i); g != nil {
			if rendered[g.Start] {
				continue
			}
			rendered[g.Start] = true
			hint := "e展开"
			if a.expanded[g.Start] {
				hint = "e收起"
			}
			lines = append(lines, FoldedStyle.Render(fmt.Sprintf("  (%d lines) [%s]", g.End-g.Start, hint)))
			if len(lines) >= vl {
				break
			}
			continue
		}

		isCursor := i == a.cursor
		inVisual := a.visualMode && i >= min(a.visualStart, a.cursor) && i <= max(a.visualStart, a.cursor)
		var text string
		if isCursor {
			text = a.renderLineTextWithBg(a.filteredView[i], SelectedBgColor, SelectedFgColor)
		} else if inVisual {
			text = a.renderLineTextWithBg(a.filteredView[i], VisualBgColor, VisualFgColor)
		} else {
			text = a.renderLineText(a.filteredView[i])
		}
		wrapped := wrapAnsiText(text, w)

		for _, wl := range wrapped {
			lines = append(lines, lipgloss.NewStyle().MaxWidth(w).Render(wl))
			if len(lines) >= vl {
				break
			}
		}
	}

	for len(lines) < vl {
		lines = append(lines, "")
	}
	if len(lines) > vl {
		lines = lines[:vl]
	}
	return lines
}

func (a *App) foldedGroup(lineIdx int) *stacktrace.Group {
	for i := range a.stGroups {
		g := &a.stGroups[i]
		if lineIdx > g.Start && lineIdx <= g.End && !a.expanded[g.Start] {
			return g
		}
	}
	return nil
}

// skipFolded adjusts target to skip over collapsed stacktrace groups.
// dir: +1 for downward, -1 for upward. Returns adjusted index.
func (a *App) skipFolded(target, dir int) int {
	for {
		g := a.foldedGroup(target)
		if g == nil {
			return target
		}
		if dir > 0 {
			target = g.End + 1
		} else {
			target = g.Start
		}
	}
}

// visualLinesBetween counts visual lines from startIdx to endIdx (exclusive),
// treating each folded group as 1 placeholder line.
func (a *App) visualLinesBetween(startIdx, endIdx int) int {
	if endIdx <= startIdx {
		return 0
	}
	rendered := make(map[int]bool)
	count := 0
	for i := startIdx; i < endIdx; i++ {
		g := a.foldedGroup(i)
		if g != nil {
			if rendered[g.Start] {
				continue
			}
			rendered[g.Start] = true
			count++
			i = g.End
			continue
		}
		count++
	}
	return count
}

// visualStartFrom walks backward from cursor, counting visual lines
// (each folded group = 1 placeholder line), returns the start index
// that produces exactly n visual lines before cursor.
// visualStartFromWrap walks backward from cursor, counting visual rows
// (each log entry wrapped to terminal width), returns the start index
// that produces exactly targetRows visual rows before cursor.
func (a *App) visualStartFromWrap(cursor, targetRows int) int {
	if targetRows <= 0 || cursor <= 0 {
		return cursor
	}
	w := a.width
	if w < 1 {
		w = 1
	}
	rendered := make(map[int]bool)
	rows := 0
	for i := cursor - 1; i >= 0; i-- {
		g := a.foldedGroup(i)
		if g != nil {
			if rendered[g.Start] {
				continue
			}
			rendered[g.Start] = true
			rows++
			if rows >= targetRows {
				return g.Start + 1
			}
			i = g.Start + 1
			continue
		}
		text := a.renderLineText(a.filteredView[i])
		rows += len(wrapAnsiText(text, w))
		if rows >= targetRows {
			return i
		}
	}
	return 0
}

func (a *App) visualStartFrom(cursor, n int) int {
	if n <= 0 || cursor <= 0 {
		return cursor
	}
	rendered := make(map[int]bool)
	count := 0
	for i := cursor - 1; i >= 0; i-- {
		g := a.foldedGroup(i)
		if g != nil {
			if rendered[g.Start] {
				continue
			}
			rendered[g.Start] = true
			count++
			if count >= n {
				return g.Start + 1
			}
			i = g.Start + 1
			continue
		}
		count++
		if count >= n {
			return i
		}
	}
	return 0
}

// renderLineWrapped renders the cursor line with selected background baked into each field.
func (a *App) renderLineWrapped(line *model.ParsedLine, lineIdx int) []string {
	text := a.renderLineTextWithBg(line, SelectedBgColor, SelectedFgColor)
	w := a.width
	if w < 1 {
		w = 1
	}
	return wrapAnsiText(text, w)
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

// renderLineTextWithBg renders line text with a forced background color on every field,
// so inner ANSI resets don't break the selection background.
func (a *App) renderLineTextWithBg(line *model.ParsedLine, bg lipgloss.Color, fg lipgloss.Color) string {
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
			parts = append(parts, LevelStyle(val).Background(bg).Foreground(fg).Render(val))
		case model.FieldSource:
			parts = append(parts, SourceStyle.Background(bg).Foreground(fg).Render(fmt.Sprintf("[%s]", val)))
		case model.FieldTime:
			parts = append(parts, TimeStyle.Background(bg).Foreground(fg).Render(val))
		case model.FieldTraceID:
			parts = append(parts, TraceIDStyle.Background(bg).Foreground(fg).Render(val))
		case model.FieldThread:
			parts = append(parts, ThreadStyle.Background(bg).Foreground(fg).Render(val))
		default:
			parts = append(parts, lipgloss.NewStyle().Background(bg).Foreground(fg).Render(val))
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
// Active SGR sequences are tracked and replayed at line breaks so wrapped
// sub-lines keep the current color/style state.
func wrapAnsiText(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	var lines []string
	var cur strings.Builder
	var activeSGR []string
	col := 0
	i := 0
	runes := []rune(text)
	for i < len(runes) {
		if runes[i] == '' {
			var seq strings.Builder
			seq.WriteRune(runes[i])
			i++
			for i < len(runes) {
				seq.WriteRune(runes[i])
				if (runes[i] >= 'a' && runes[i] <= 'z') || (runes[i] >= 'A' && runes[i] <= 'Z') {
					i++
					break
				}
				i++
			}
			s := seq.String()
			cur.WriteString(s)
			// track SGR sequences (ending with 'm') for replay on line break
			if len(s) >= 3 && s[len(s)-1] == 'm' {
				inner := s[2 : len(s)-1]
				if inner == "0" || inner == "" {
					activeSGR = activeSGR[:0]
				} else {
					activeSGR = append(activeSGR, s)
				}
			}
			continue
		}
		rw := 1
		if runes[i] > 0x7f {
			rw = 2
		}
		if col+rw > width {
			lines = append(lines, cur.String())
			cur.Reset()
			for _, s := range activeSGR {
				cur.WriteString(s)
			}
			col = 0
		}
		cur.WriteRune(runes[i])
		col += rw
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
