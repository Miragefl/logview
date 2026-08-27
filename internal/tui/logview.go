package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/justfun/logview/internal/model"
	"github.com/justfun/logview/internal/stacktrace"
	"github.com/mattn/go-runewidth"
)

const scrollOff = 5

// lineNumStyle 行号前缀的暗淡样式。
var lineNumStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))

// prefixLineNums 给 wrap 后的行首加行号前缀（首行行号、续行空白对齐）。
func prefixLineNums(wrapped []string, lineIdx, total int) []string {
	if len(wrapped) == 0 {
		return wrapped
	}
	nw := len(fmt.Sprintf("%d", total))
	numStr := fmt.Sprintf("%*d │ ", nw, lineIdx+1)
	pad := fmt.Sprintf("%*s │ ", nw, "")
	wrapped[0] = lineNumStyle.Render(numStr) + wrapped[0]
	for j := 1; j < len(wrapped); j++ {
		wrapped[j] = lineNumStyle.Render(pad) + wrapped[j]
	}
	return wrapped
}

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
	w := a.contentWidth()
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
		if a.showLineNum {
			wrapped = prefixLineNums(wrapped, i, len(a.filteredView))
		}

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
	w := a.contentWidth()
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

// headerLabels 表头显示名（技术名词保留英文）。
var headerLabels = map[model.Field]string{
	model.FieldTime:    "时间",
	model.FieldLevel:   "级别",
	model.FieldSource:  "来源",
	model.FieldThread:  "线程",
	model.FieldTraceID: "Trace",
	model.FieldLogger:  "Logger",
	model.FieldMessage: "消息",
}

// renderHeaderLine 渲染表头行：跟随字段可见性，列宽与数据列严格对齐。
func (a *App) renderHeaderLine() string {
	var parts []string
	for _, f := range model.AllFields {
		if !a.fieldMask.IsVisible(f) {
			continue
		}
		label, ok := headerLabels[f]
		if !ok {
			label = string(f)
		}
		styled := DetailDimStyle.Render(label)
		w := columnWidths[f]
		if f == model.FieldLevel {
			w = 5
		}
		if w > 0 {
			styled = padDisplayWidth(styled, w)
		}
		parts = append(parts, styled)
	}
	return "  " + strings.Join(parts, "  ")
}

// renderLineWrapped renders the cursor line with selected background baked into each field.
func (a *App) renderLineWrapped(line *model.ParsedLine, lineIdx int) []string {
	text := SelArrowStyle.Render("▶ ") + a.renderLineTextWithBg(line, SelectedBgColor, SelectedFgColor)
	w := a.contentWidth()
	if w < 1 {
		w = 1
	}
	result := wrapAnsiText(text, w)
	if a.showLineNum && len(result) > 0 {
		result = prefixLineNums(result, lineIdx, len(a.filteredView))
	}
	return result
}

// applyHighlights 叠加搜索命中高亮与用户自定义高亮词。
func (a *App) applyHighlights(text string) string {
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

// renderLineText builds the full text of a line without truncation or selection styling.
func (a *App) renderLineText(line *model.ParsedLine) string {
	var parts []string
	for _, f := range model.AllFields {
		if !a.fieldMask.IsVisible(f) {
			continue
		}
		val := line.Get(f)
		cw := columnWidths[f]
		if val == "" {
			if cw > 0 {
				parts = append(parts, strings.Repeat(" ", cw)) // 空字段占位，保持列对齐
			}
			continue
		}
		var styled string
		switch f {
		case model.FieldLevel:
			styled = LevelStyle(val).Render(padLevel(val))
		case model.FieldSource:
			srcStyle := lipgloss.NewStyle().Foreground(SourceColors[0])
			if idx, ok := a.sourceColorIdx[val]; ok {
				srcStyle = lipgloss.NewStyle().Foreground(SourceColors[idx%len(SourceColors)])
			}
			styled = srcStyle.Render(fmt.Sprintf("[%s]", val))
		case model.FieldTime:
			styled = TimeStyle.Render(fitFieldVal(f, val))
		case model.FieldTraceID:
			styled = TraceIDStyle.Render(fitFieldVal(f, val))
		case model.FieldThread:
			styled = ThreadStyle.Render(fitFieldVal(f, val))
		case model.FieldMessage:
			styled = compactJSON(val)
		default:
			styled = fitFieldVal(f, val)
		}
		if cw > 0 && f != model.FieldLevel {
			styled = padDisplayWidth(styled, cw)
		}
		parts = append(parts, styled)
	}
	return a.applyHighlights(strings.Join(parts, "  "))
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
		cw := columnWidths[f]
		if val == "" {
			if cw > 0 {
				parts = append(parts, lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", cw)))
			}
			continue
		}
		var styled string
		switch f {
		case model.FieldLevel:
			// 选中行保留级别徽章原色，徽章即定位锚点
			styled = LevelStyle(val).Render(padLevel(val))
		case model.FieldSource:
			styled = SourceStyle.Background(bg).Foreground(fg).Render(fmt.Sprintf("[%s]", val))
		case model.FieldTime:
			styled = TimeStyle.Background(bg).Foreground(fg).Render(fitFieldVal(f, val))
		case model.FieldTraceID:
			styled = TraceIDStyle.Background(bg).Foreground(fg).Render(fitFieldVal(f, val))
		case model.FieldThread:
			styled = ThreadStyle.Background(bg).Foreground(fg).Render(fitFieldVal(f, val))
		default:
			styled = lipgloss.NewStyle().Background(bg).Foreground(fg).Render(fitFieldVal(f, val))
		}
		if cw > 0 && f != model.FieldLevel {
			styled = padDisplayWidth(styled, cw)
		}
		parts = append(parts, styled)
	}
	text := strings.Join(parts, "  ")
	return a.applyHighlights(text)
}

func (a *App) renderLine(line *model.ParsedLine, selected bool, lineIdx int) string {
	text := a.renderLineText(line)
	prefix := "  "
	if a.bookmarks[line.Raw.Seq] {
		prefix = BookmarkStyle.Render("▸ ")
	}
	if selected {
		prefix = SelArrowStyle.Render("▶ ")
	}
	text = prefix + text
	if a.showLineNum {
		total := len(a.filteredView)
		w := len(fmt.Sprintf("%d", total))
		numStr := fmt.Sprintf("%*d │ ", w, lineIdx+1)
		text = lineNumStyle.Render(numStr) + text
	}
	inVisualRange := a.visualMode && lineIdx >= min(a.visualStart, a.cursor) && lineIdx <= max(a.visualStart, a.cursor)
	cw := a.contentWidth()
	if selected && !inVisualRange {
		truncated := lipgloss.NewStyle().MaxWidth(cw).Render(text)
		return SelectedStyle.Width(cw).Render(truncated)
	}
	if inVisualRange {
		truncated := lipgloss.NewStyle().MaxWidth(cw).Render(text)
		return VisualStyle.Width(cw).Render(truncated)
	}
	return text
}

func highlightText(text, query string) string {
	return highlightTextWithStyle(text, query, HighlightStyle)
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

// compactJSON compresses JSON string to single line, returns original if not JSON.
func compactJSON(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return s
	}
	if s[0] != '{' && s[0] != '[' {
		return s
	}
	var buf bytes.Buffer
	if json.Compact(&buf, []byte(s)) == nil {
		return buf.String()
	}
	return s
}

// contentWidth 日志内容区宽度（终端宽 - 左右边框 "│ " + " │" 共 4 列）。
func (a *App) contentWidth() int {
	w := a.width - 4
	if w < 1 {
		w = 1
	}
	return w
}

// columnWidths 字段固定列宽（message 弹性不设限），保证 message 列起点恒定。
var columnWidths = map[model.Field]int{
	model.FieldTime:    12,
	model.FieldThread:  10,
	model.FieldTraceID: 12,
	model.FieldLogger:  20,
}

// SetColumnWidths 从 rules.yaml fields[].width 覆盖字段列宽。
func SetColumnWidths(w map[model.Field]int) {
	for f, v := range w {
		if v > 0 {
			columnWidths[f] = v
		}
	}
}

// fitFieldVal 字段值适配列宽：time 超长取尾部（保时分秒弃日期），
// logger 先缩写（com.a.b.Foo → c.a.b.Foo），再按显示宽度截断加 …。
func fitFieldVal(f model.Field, val string) string {
	w := columnWidths[f]
	if w <= 0 || val == "" {
		return val
	}
	switch f {
	case model.FieldTime:
		if runewidth.StringWidth(val) > w && len(val) >= 19 {
			val = val[len(val)-w:] // 完整日期时间为 ASCII，字节截取安全
		}
	case model.FieldLogger:
		val = abbreviateLogger(val)
	}
	return runewidth.Truncate(val, w, "…")
}

// abbreviateLogger 缩写 logger：末段类名全保留，包名段取首字母。
func abbreviateLogger(s string) string {
	parts := strings.Split(s, ".")
	if len(parts) <= 1 {
		return s
	}
	var b strings.Builder
	for _, p := range parts[:len(parts)-1] {
		if p == "" {
			continue
		}
		b.WriteByte(p[0])
		b.WriteByte('.')
	}
	b.WriteString(parts[len(parts)-1])
	return b.String()
}

// frameTop 构造顶部边框行，标题嵌入：╭─ core ────╮。
func (a *App) frameTop(core string) string {
	core = lipgloss.NewStyle().MaxWidth(a.width - 2).Render(core)
	fill := a.width - displayWidth(core) - 3 // "╭─"占2列 + "╮"占1列
	if fill < 0 {
		fill = 0
	}
	return FrameStyle.Render("╭─") + core + FrameStyle.Render(strings.Repeat("─", fill)) + FrameStyle.Render("╮")
}

// frameBottom 构造底部边框行，统计嵌入：╰─ core ────╯。
func (a *App) frameBottom(core string) string {
	core = lipgloss.NewStyle().MaxWidth(a.width - 2).Render(core)
	fill := a.width - displayWidth(core) - 3
	if fill < 0 {
		fill = 0
	}
	return FrameStyle.Render("╰─") + core + FrameStyle.Render(strings.Repeat("─", fill)) + FrameStyle.Render("╯")
}

// padDisplayWidth ANSI 安全地把行补齐到指定显示宽度，保证右边框竖线对齐。
func padDisplayWidth(s string, w int) string {
	d := displayWidth(s)
	if d >= w {
		return s
	}
	return s + strings.Repeat(" ", w-d)
}
