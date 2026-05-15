package tui

import (
	"context"
	"fmt"
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
	ctx := context.Background()
	ch, err := a.stream.Start(ctx)
	if err != nil {
		return nil
	}
	streamCh = ch
	return tea.Batch(waitForStream(ch), tickEvery())
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
	switch msg.String() {
	case "q":
		return a, tea.Quit
	case "esc":
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
		a.searchInput = ""
	case "f":
		a.panelFocus = true
		a.fieldCursor = 0
	case "s":
		a.exportMode = true
	case "g":
		a.cursor = 0
		a.autoscroll = false
	case "G":
		a.cursor = max(0, len(a.filteredView)-1)
		a.autoscroll = true
	case "n":
		a.jumpSearchMatch(1)
	case "N":
		a.jumpSearchMatch(-1)
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

func (a *App) handleSearchKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		a.searchMode = false
	case tea.KeyEnter:
		a.searchMode = false
		a.recomputeView()
	case tea.KeyBackspace:
		if len(a.searchInput) > 0 {
			a.searchInput = a.searchInput[:len([]rune(a.searchInput))-1]
			a.recomputeView()
		}
	case tea.KeyRunes:
		a.searchInput += string(msg.Runes)
		a.recomputeView()
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
	// 6 fixed lines: title, sep, bar, sep, sep(bottom), helpBar
	vl := a.height - 6
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
	helpBar := HelpStyle.Width(w).Render(a.renderHelpBarContent())

	vl := a.visibleLines()
	var logLines []string
	if a.panelFocus {
		logLines = a.buildPopupLines(vl)
	} else {
		logLines = a.buildLogLines(vl)
	}

	allLines := make([]string, 0, vl+6)
	allLines = append(allLines, title, sep, bar, sep)
	for _, l := range logLines {
		allLines = append(allLines, trunc.Render(l))
	}
	allLines = append(allLines, sep, helpBar)
	return strings.Join(allLines, "\n")
}

