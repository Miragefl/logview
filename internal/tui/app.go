package tui

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/justfun/logview/internal/buffer"
	"github.com/justfun/logview/internal/export"
	"github.com/justfun/logview/internal/model"
	"github.com/justfun/logview/internal/parser"
	"github.com/justfun/logview/internal/stacktrace"
	"github.com/justfun/logview/internal/stream"
)

var cancelFunc context.CancelFunc

type starField struct {
	Name  string
	Value string
}

type App struct {
	stream      stream.LogStream
	parsers     *parser.AutoDetect
	buffer      *buffer.RingBuffer
	searchIdx   *buffer.SearchIndex
	keymap      KeyMap
	fieldAlias  map[string]string // custom field -> standard field mapping

	filteredView []*model.ParsedLine
	stGroups     []stacktrace.Group
	expanded     map[int]bool

	width      int
	height     int
	cursor     int
	offset     int
	autoscroll bool
	newLogs    int

	searchMode  bool
	searchInput string
	cachedQuery SearchQuery

	fieldMask model.FieldMask

	panelFocus  bool
	fieldCursor int

	exportMode  bool
	exportState ExportState

	visualMode  bool
	visualStart int

	pendingKey  string

	levelFilter string

	starFields  []starField
	starCursor   int
		searchCursor int

		helpMode      bool

	highlights     []string
	highlightMode  bool
	highlightInput string

	parserName string
}

type ExportState struct {
	Scope    int
	Format   int
	FilePath string
	Cursor   int
	Done     bool
	Exported int
}

func newExportState() ExportState {
	return ExportState{
		FilePath: fmt.Sprintf("./logview-export-%s.log", time.Now().Format("20060102")),
	}
}

var overrideFieldMask model.FieldMask
var overrideFieldAlias map[string]string

// SetFieldMask sets the global field mask override (called from config loader).
func SetFieldMask(mask model.FieldMask) {
	overrideFieldMask = mask
}

// SetFieldAlias sets the global field alias mapping (called from config loader).
func SetFieldAlias(aliases map[string]string) {
	overrideFieldAlias = aliases
}

func NewApp(src stream.LogStream, parsers *parser.AutoDetect, bufSize int) *App {
	fm := model.DefaultFieldMask()
	if overrideFieldMask != nil {
		fm = overrideFieldMask
	}
	return &App{
		stream:      src,
		parsers:     parsers,
		buffer:      buffer.NewRingBuffer(bufSize),
		searchIdx:   buffer.NewSearchIndex(),
		keymap:      DefaultKeyMap(),
		fieldMask:   fm,
		fieldAlias:  overrideFieldAlias,
		expanded:    make(map[int]bool),
		autoscroll:  true,
		exportState: newExportState(),
	}
}

type streamMsg struct{ line model.RawLine }
type tickMsg struct{}

func waitForStream(ch <-chan model.RawLine) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return nil
		}
		return streamMsg{line: line}
	}
}

func tickEvery() tea.Cmd {
	return tea.Tick(33*time.Millisecond, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

var streamCh <-chan model.RawLine

func (a *App) Init() tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	cancelFunc = cancel
	ch, err := a.stream.Start(ctx)
	if err != nil {
		cancel()
		return nil
	}
	streamCh = ch
	return tea.Batch(waitForStream(ch), tickEvery())
}

func (a *App) shutdown() tea.Cmd {
	if cancelFunc != nil {
		cancelFunc()
	}
	a.stream.Cleanup()
	return tea.Quit
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil
	case streamMsg:
		a.processLine(msg.line)
		return a, waitForStream(streamCh)
	case tickMsg:
		return a, tickEvery()
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return a, a.shutdown()
		}
		if a.helpMode {
			return a.handleHelpKeys(msg)
		}
		if a.highlightMode {
			return a.handleHighlightKeys(msg)
		}
		if a.exportMode {
			return a.handleExportKeys(msg)
		}
		if a.searchMode {
			return a.handleSearchKeys(msg)
		}
		if a.panelFocus {
			return a.handlePanelKeys(msg)
		}
		return a.handleNormalKeys(msg)
	case tea.InterruptMsg:
		return a, a.shutdown()
	}
	return a, nil
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func (a *App) processLine(raw model.RawLine) {
	cleaned := ansiRe.ReplaceAllString(raw.Text, "")
	raw.Text = cleaned
	var pl *model.ParsedLine
	if a.parsers != nil {
		if p := a.parsers.Detect(raw); p != nil {
			pl = p.Parse(raw)
			a.parserName = p.Name()
			a.reparsePending(p)
		}
	}
	if pl == nil {
		pl = &model.ParsedLine{
			Raw:     raw,
			Message: raw.Text,
			Fields:  map[model.Field]string{model.FieldMessage: raw.Text},
		}
	}
	a.applyFieldAlias(pl)
	a.buffer.Push(pl)
	a.searchIdx.Add(int(a.buffer.TotalReceived()-1), raw.Text)
	if !a.autoscroll {
		a.newLogs++
	}
	a.recomputeView()
}

func (a *App) reparsePending(p parser.Parser) {
	pending := a.parsers.DrainPending()
	if len(pending) == 0 {
		return
	}
	for i := 0; i < a.buffer.Len(); i++ {
		line := a.buffer.Get(i)
		if line == nil || line.Fields != nil && line.Fields[model.FieldMessage] == line.Raw.Text {
			pl := p.Parse(line.Raw)
			if pl != nil {
				a.buffer.Set(i, pl)
			}
		}
	}
}

func (a *App) recomputeView() {
	var view []*model.ParsedLine
	for i := 0; i < a.buffer.Len(); i++ {
		line := a.buffer.Get(i)
		if line == nil {
			continue
		}
		if a.searchInput != "" {
			if !a.currentQuery().MatchLine(line) {
				continue
			}
		}
		if a.levelFilter != "" {
			if !a.matchLevelFilter(line) {
				continue
			}
		}
		view = append(view, line)
	}
	a.filteredView = view
	if a.cursor >= len(a.filteredView) {
		a.cursor = max(0, len(a.filteredView)-1)
	}
	a.stGroups = stacktrace.Detect(view)
}

func containsIgnoreCase(s, sub string) bool {
	ls, lsub := strings.ToLower(s), strings.ToLower(sub)
	return len(ls) >= len(lsub) && strings.Contains(ls, lsub)
}

func (a *App) matchLevelFilter(line *model.ParsedLine) bool {
	lv := strings.ToUpper(line.Level)
	switch a.levelFilter {
	case "ERROR":
		return lv == "ERROR" || lv == "ERR" || lv == "FATAL"
	case "WARN":
		return lv == "ERROR" || lv == "ERR" || lv == "FATAL" || lv == "WARN" || lv == "WARNING"
	case "INFO":
		return lv == "ERROR" || lv == "ERR" || lv == "FATAL" || lv == "WARN" || lv == "WARNING" || lv == "INFO"
	case "DEBUG":
		return true
	}
	return true
}

// applyFieldAlias maps custom field names to standard struct fields.
// e.g. if config has "th" maps_to "thread", sets pl.Thread from Fields["th"].
func (a *App) applyFieldAlias(pl *model.ParsedLine) {
	if a.fieldAlias == nil || pl.Fields == nil {
		return
	}
	for custom, standard := range a.fieldAlias {
		v, ok := pl.Fields[model.Field(custom)]
		if !ok || v == "" {
			continue
		}
		switch model.Field(standard) {
		case model.FieldTime:
			if t, err := time.Parse("2006-01-02 15:04:05.000", v); err == nil {
				pl.Time = t
			}
		case model.FieldLevel:
			pl.Level = v
		case model.FieldThread:
			pl.Thread = v
		case model.FieldTraceID:
			pl.TraceID = v
		case model.FieldLogger:
			pl.Logger = v
		case model.FieldMessage:
			pl.Message = v
		}
	}
}

func (a *App) currentQuery() SearchQuery {
	if a.cachedQuery.Raw != a.searchInput {
		a.cachedQuery = parseSearchQuery(a.searchInput)
	}
	return a.cachedQuery
}

func (a *App) jumpSearchMatch(dir int) {
	if a.searchInput == "" || len(a.filteredView) == 0 {
		return
	}
	q := a.currentQuery()
	if q.IsEmpty() {
		return
	}
	var matches []int
	for i, line := range a.filteredView {
		if q.MatchLine(line) {
			matches = append(matches, i)
		}
	}
	if len(matches) == 0 {
		return
	}
	cur := a.cursor
	idx := 0
	for idx < len(matches) && matches[idx] < cur {
		idx++
	}
	if dir > 0 {
		if idx < len(matches) {
			a.cursor = matches[idx]
		} else {
			a.cursor = matches[0]
		}
	} else {
		if idx > 0 {
			a.cursor = matches[idx-1]
		} else {
			a.cursor = matches[len(matches)-1]
		}
	}
	a.autoscroll = false
}

func (a *App) handlePanelKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		a.panelFocus = false
	case "up", "k":
		if a.fieldCursor > 0 {
			a.fieldCursor--
		}
	case "down", "j":
		if a.fieldCursor < len(model.AllFields)-1 {
			a.fieldCursor++
		}
	case "enter", " ":
		field := model.AllFields[a.fieldCursor]
		a.fieldMask.Toggle(field)
	}
	return a, nil
}

func (a *App) handleNormalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// handle pending key sequences (viw, zt/zz/zb)
	if a.pendingKey != "" {
		key := msg.String()
		switch a.pendingKey {
		case "z":
			vl := a.visibleLines()
			switch key {
			case "t":
				a.offset = a.cursor
			case "z":
				a.offset = max(0, a.cursor-vl/2)
			case "b":
				a.offset = max(0, a.cursor-vl+1)
			}
			a.autoscroll = false
			a.pendingKey = ""
			return a, nil
		}
		a.pendingKey = ""
		return a, nil
	}

	if a.visualMode {
		return a.handleVisualKeys(msg)
	}

	switch msg.String() {
	case "q":
		return a, tea.Quit
	case "esc":
		if a.visualMode {
			a.visualMode = false
			return a, nil
		}
		if a.searchInput != "" {
			var curLine *model.ParsedLine
			if a.cursor >= 0 && a.cursor < len(a.filteredView) {
				curLine = a.filteredView[a.cursor]
			}
			a.searchInput = ""
			a.recomputeView()
			if curLine != nil {
				for i, l := range a.filteredView {
					if l == curLine {
						a.cursor = i
						break
					}
				}
			}
		}
	case "/":
		a.searchMode = true
		a.populateSearchFields()
		a.searchCursor = len([]rune(a.searchInput))
	case "v":
		a.visualMode = true
		a.visualStart = a.cursor
		a.autoscroll = false
	case "V":
		a.visualMode = true
		a.visualStart = a.cursor
		a.autoscroll = false
	case "y":
		a.yankLines(a.cursor, a.cursor)
	case "f":
		a.searchMode = true
		a.populateSearchFields()
		a.searchCursor = len([]rune(a.searchInput))
	case "F":
		a.panelFocus = true
		a.fieldCursor = 0
	case "?":
		a.helpMode = true
	case "h":
		a.highlightMode = true
		if a.highlightInput == "" && len(a.highlights) > 0 {
			a.highlightInput = strings.Join(a.highlights, ", ")
		}
	case "s":
		a.exportMode = true
	case "g":
		a.cursor = 0
		a.autoscroll = false
	case "G":
		a.cursor = max(0, len(a.filteredView)-1)
		a.autoscroll = true
	case "H":
		a.cursor = a.offset
		a.autoscroll = false
	case "M":
		a.cursor = a.offset + a.visibleLines()/2
		if a.cursor >= len(a.filteredView) { a.cursor = len(a.filteredView)-1 }
		a.autoscroll = false
	case "L":
		a.cursor = a.offset + a.visibleLines() - 1
		if a.cursor >= len(a.filteredView) { a.cursor = len(a.filteredView)-1 }
		a.autoscroll = false
	case "z":
		a.pendingKey = "z"
	case "I":
		a.toggleLevelFilter("INFO")
	case "D":
		a.toggleLevelFilter("DEBUG")
	case "E":
		a.toggleLevelFilter("ERROR")
	case "W":
		a.toggleLevelFilter("WARN")
	case "A":
		a.toggleLevelFilter("")
	case "ctrl+d":
		hs := a.visibleLines() / 2
		a.cursor += hs
		if a.cursor >= len(a.filteredView) {
			a.cursor = max(0, len(a.filteredView)-1)
		}
		a.autoscroll = (a.cursor == len(a.filteredView)-1)
	case "ctrl+f":
		ps := a.visibleLines()
		a.cursor += ps
		if a.cursor >= len(a.filteredView) {
			a.cursor = len(a.filteredView) - 1
		}
		a.autoscroll = (a.cursor == len(a.filteredView)-1)
	case "ctrl+u":
		hs := a.visibleLines() / 2
		a.cursor -= hs
		if a.cursor < 0 {
			a.cursor = 0
		}
		a.autoscroll = false
	case "ctrl+b":
		ps := a.visibleLines()
		a.cursor -= ps
		if a.cursor < 0 {
			a.cursor = 0
		}
		a.autoscroll = false
	case "up", "k":
		if a.cursor > 0 {
			a.cursor--
			a.autoscroll = false
		}
	case "down", "j":
		if a.cursor < len(a.filteredView)-1 {
			a.cursor++
		}
		a.autoscroll = (a.cursor == len(a.filteredView)-1)
	case "pgup":
		ps := a.visibleLines()
		a.cursor -= ps
		if a.cursor < 0 {
			a.cursor = 0
		}
		a.autoscroll = false
	case "pgdown":
		ps := a.visibleLines()
		a.cursor += ps
		if a.cursor >= len(a.filteredView) {
			a.cursor = len(a.filteredView) - 1
		}
		a.autoscroll = (a.cursor == len(a.filteredView)-1)
	}
	return a, nil
}

func (a *App) handleVisualKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		a.yankLines(a.visualStart, a.cursor)
	case "esc":
		a.visualMode = false
	case "up", "k":
		if a.cursor > 0 {
			a.cursor--
		}
	case "down", "j":
		if a.cursor < len(a.filteredView)-1 {
			a.cursor++
		}
	case "G":
		a.cursor = max(0, len(a.filteredView)-1)
	case "g":
		a.cursor = 0
	}
	return a, nil
}

func (a *App) handleSearchKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		a.searchMode = false
		a.starFields = nil
		a.starCursor = 0
	case tea.KeyEnter:
		if len(a.starFields) > 0 && a.starCursor < len(a.starFields) {
			sf := a.starFields[a.starCursor]
			if sf.Name != "" {
				term := sf.Name + ":" + sf.Value
				runes := []rune(a.searchInput)
				pos := a.searchCursor
				insert := term
				if a.searchInput != "" && pos > 0 && runes[pos-1] != ' ' {
					insert = " " + insert
				}
				if a.searchInput != "" && pos < len(runes) && runes[pos] != ' ' {
					insert = insert + " "
				}
				runes = append(runes[:pos], append([]rune(insert), runes[pos:]...)...)
				a.searchInput = string(runes)
				a.searchCursor = pos + len([]rune(insert))
			}
		}
		a.searchMode = false
		a.starFields = nil
		a.starCursor = 0
		a.recomputeView()
	case tea.KeyBackspace:
		runes := []rune(a.searchInput)
		if a.searchCursor > 0 && len(runes) > 0 {
			a.searchCursor--
			runes = append(runes[:a.searchCursor], runes[a.searchCursor+1:]...)
			a.searchInput = string(runes)
			a.recomputeView()
		}
	case tea.KeyTab:
		if len(a.starFields) > 0 {
			a.starCursor = (a.starCursor + 1) % len(a.starFields)
		}
	case tea.KeyShiftTab:
		if len(a.starFields) > 0 {
			a.starCursor = (a.starCursor - 1 + len(a.starFields)) % len(a.starFields)
		}
	case tea.KeyRunes:
		insert := string(msg.Runes)
		runes := []rune(a.searchInput)
		pos := a.searchCursor
		if pos > len(runes) {
			pos = len(runes)
		}
		runes = append(runes[:pos], append([]rune(insert), runes[pos:]...)...)
		a.searchInput = string(runes)
		a.searchCursor = pos + len([]rune(insert))
		a.recomputeView()
	default:
		switch msg.String() {
		case "left":
			if a.searchCursor > 0 {
				a.searchCursor--
			}
		case "right":
			if a.searchCursor < len([]rune(a.searchInput)) {
				a.searchCursor++
			}
		case "home", "ctrl+a":
			a.searchCursor = 0
		case "end", "ctrl+e":
			a.searchCursor = len([]rune(a.searchInput))
		case "delete":
			runes := []rune(a.searchInput)
			if a.searchCursor < len(runes) {
				runes = append(runes[:a.searchCursor], runes[a.searchCursor+1:]...)
				a.searchInput = string(runes)
				a.recomputeView()
			}
		case "ctrl+u":
			a.searchInput = ""
			a.searchCursor = 0
			a.recomputeView()
		case " ":
			runes := []rune(a.searchInput)
			pos := a.searchCursor
			runes = append(runes[:pos], append([]rune(" "), runes[pos:]...)...)
			a.searchInput = string(runes)
			a.searchCursor = pos + 1
			a.recomputeView()
		case "ctrl+j":
			if len(a.starFields) > 0 {
				a.starCursor = (a.starCursor + 1) % len(a.starFields)
			}
		case "ctrl+k":
			if len(a.starFields) > 0 {
				a.starCursor = (a.starCursor - 1 + len(a.starFields)) % len(a.starFields)
			}
		}
	}
	return a, nil
}

func (a *App) handleHighlightKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		a.highlightMode = false
	case tea.KeyEnter:
		// parse comma-separated keywords
		kw := strings.TrimSpace(a.highlightInput)
		if kw != "" {
			a.highlights = strings.Split(kw, ",")
			for i := range a.highlights {
				a.highlights[i] = strings.TrimSpace(a.highlights[i])
			}
			// remove empty
			var clean []string
			for _, h := range a.highlights {
				if h != "" {
					clean = append(clean, h)
				}
			}
			a.highlights = clean
		} else {
			a.highlights = nil
		}
		a.highlightMode = false
	case tea.KeyBackspace:
		if len(a.highlightInput) > 0 {
			runes := []rune(a.highlightInput)
			a.highlightInput = string(runes[:len(runes)-1])
		}
	case tea.KeyRunes:
		a.highlightInput += string(msg.Runes)
	default:
		switch msg.String() {
		case "ctrl+u":
			a.highlightInput = ""
		case " ":
			a.highlightInput += " "
		}
	}
	return a, nil
}

func (a *App) handleHelpKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "?", "esc", "q", "enter":
		a.helpMode = false
	}
	return a, nil
}

func (a *App) handleExportKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		a.exportMode = false
	case "up", "k":
		if a.exportState.Cursor > 0 {
			a.exportState.Cursor--
		}
	case "down", "j":
		if a.exportState.Cursor < 2 {
			a.exportState.Cursor++
		}
	case "left", "h":
		switch a.exportState.Cursor {
		case 0:
			a.exportState.Scope = 0
		case 1:
			a.exportState.Format = 0
		}
	case "right", "l":
		switch a.exportState.Cursor {
		case 0:
			a.exportState.Scope = 1
		case 1:
			a.exportState.Format = 1
		}
	case "enter":
		a.doExport()
	}
	return a, nil
}

func (a *App) populateSearchFields() {
	if a.cursor < 0 || a.cursor >= len(a.filteredView) {
		a.starFields = nil
		return
	}
	line := a.filteredView[a.cursor]
	var fields []starField
	fields = append(fields, starField{Name: "", Value: ""})
	for _, f := range model.AllFields {
		val := line.Get(f)
		if val == "" {
			continue
		}
		if f == model.FieldMessage {
			for _, w := range strings.Fields(val) {
				if len(w) > 1 {
					fields = append(fields, starField{Name: string(f), Value: w})
				}
			}
			continue
		}
		fields = append(fields, starField{Name: string(f), Value: val})
	}
	a.starFields = fields
}

func (a *App) toggleLevelFilter(level string) {
	if a.levelFilter == level {
		a.levelFilter = ""
	} else {
		a.levelFilter = level
	}
	a.recomputeView()
}

func (a *App) doExport() {
	s := &a.exportState
	var lines []*model.ParsedLine
	if s.Scope == 0 {
		lines = a.filteredView
	} else {
		for i := 0; i < a.buffer.Len(); i++ {
			if l := a.buffer.Get(i); l != nil {
				lines = append(lines, l)
			}
		}
	}
	format := export.FormatRaw
	if s.Format == 1 {
		format = export.FormatJSON
	}
	n, _ := export.ToFile(lines, s.FilePath, format)
	s.Done = true
	s.Exported = n
}

func (a *App) visibleLines() int {
	// fixed lines: title, sep, bar, sep, sep(bottom) = 5, plus helpBar (1-2 lines)
	vl := a.height - 5 - a.helpBarHeight()
	if vl < 1 {
		vl = 1
	}
	return vl
}

func (a *App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	w := a.width

	pLabel := a.parserName
	if pLabel == "" {
		pLabel = "raw"
	}
	title := TitleStyle.Width(w).Render(
		fmt.Sprintf(" LogView ─ %s [%s] ─ %d条 ", a.stream.Label(), pLabel, a.buffer.Len()),
	)

	sep := strings.Repeat(HorizontalLine, w)

	// truncate every line to terminal width to prevent wrapping
	trunc := lipgloss.NewStyle().MaxWidth(w)
	bar := trunc.Render(a.renderSearchBar())
	helpBar := a.renderHelpBarContent()

	vl := a.visibleLines()
	var logLines []string
	if a.helpMode {
		logLines = a.buildHelpPopup(vl)
	} else if a.highlightMode {
		logLines = a.buildHighlightPopup(vl)
	} else if a.panelFocus {
		logLines = a.buildPopupLines(vl)
	} else {
		logLines = a.buildLogLines(vl)
	}

	allLines := make([]string, 0, vl+6)
	allLines = append(allLines, title, sep, bar, sep)
	// overlay search popup on top of log lines if active
	popupLines := a.buildSearchPopup()
	for i, l := range logLines {
		rendered := trunc.Render(l)
		if a.searchMode && len(popupLines) > 0 && i < len(popupLines) {
			rendered = lipgloss.NewStyle().Width(w).MaxWidth(w).Render(popupLines[i])
		}
		allLines = append(allLines, rendered)
	}
	allLines = append(allLines, sep, helpBar)
	return strings.Join(allLines, "\n")
}

func (a *App) yankLines(start, end int) {
	if start > end {
		start, end = end, start
	}
	var buf strings.Builder
	for i := start; i <= end && i < len(a.filteredView); i++ {
		buf.WriteString(a.filteredView[i].Raw.Text)
		buf.WriteByte('\n')
	}
	copyToClipboard(buf.String())
	a.visualMode = false
}

func (a *App) yankWord() {
	if a.cursor < 0 || a.cursor >= len(a.filteredView) {
		return
	}
	line := a.filteredView[a.cursor]
	var parts []string
	for _, f := range model.AllFields {
		if !a.fieldMask.IsVisible(f) {
			continue
		}
		val := line.Get(f)
		if val == "" {
			continue
		}
		parts = append(parts, val)
	}
	text := strings.Join(parts, "  ")
	center := len(text) / 2
	if center > a.width/2 {
		center = a.width / 2
	}
	word := wordAtPos(text, center)
	if word != "" {
		copyToClipboard(word)
	}
}

func wordAtPos(text string, pos int) string {
	if pos >= len(text) {
		pos = len(text) - 1
	}
	if pos < 0 {
		return ""
	}
	isSpace := func(c byte) bool { return c == ' ' || c == '	' }
	if isSpace(text[pos]) {
		left := pos
		for left >= 0 && isSpace(text[left]) { left-- }
		right := pos
		for right < len(text) && isSpace(text[right]) { right++ }
		if left >= 0 {
			pos = left
		} else if right < len(text) {
			pos = right
		} else {
			return ""
		}
	}
	start := pos
	for start > 0 && !isSpace(text[start-1]) { start-- }
	end := pos
	for end < len(text) && !isSpace(text[end]) { end++ }
	return text[start:end]
}

func copyToClipboard(text string) {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	cmd.Run()
}
