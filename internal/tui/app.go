package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justfun/logview/internal/buffer"
	"github.com/justfun/logview/internal/export"
	"github.com/justfun/logview/internal/model"
	"github.com/justfun/logview/internal/parser"
	"github.com/justfun/logview/internal/stacktrace"
	"github.com/justfun/logview/internal/stream"
)

type App struct {
	stream    stream.LogStream
	parsers   *parser.AutoDetect
	buffer    *buffer.RingBuffer
	searchIdx *buffer.SearchIndex
	keymap    KeyMap

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

	fieldMask     model.FieldMask
	levelMask     map[string]bool
	filterTraceID string
	filterThread  string

	activePanel int // 0=fields, 1=levels, 2=filters

	exportMode  bool
	exportState ExportState

	parserName string
}

type ExportState struct {
	Scope    int    // 0=filtered, 1=all
	Format   int    // 0=raw, 1=json
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

func NewApp(src stream.LogStream, parsers *parser.AutoDetect, bufSize int) *App {
	return &App{
		stream:      src,
		parsers:     parsers,
		buffer:      buffer.NewRingBuffer(bufSize),
		searchIdx:   buffer.NewSearchIndex(),
		keymap:      DefaultKeyMap(),
		fieldMask:   model.DefaultFieldMask(),
		levelMask:   map[string]bool{"DEBUG": false, "INFO": true, "WARN": true, "ERROR": true},
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
		return a.handleNormalKeys(msg)
	}
	return a, nil
}

func (a *App) processLine(raw model.RawLine) {
	var pl *model.ParsedLine
	if a.parsers != nil {
		if p := a.parsers.Detect(raw); p != nil {
			pl = p.Parse(raw)
			a.parserName = p.Name()
		}
	}
	if pl == nil {
		pl = &model.ParsedLine{
			Raw:     raw,
			Message: raw.Text,
			Fields:  map[model.Field]string{model.FieldMessage: raw.Text},
		}
	}
	a.buffer.Push(pl)
	a.searchIdx.Add(int(a.buffer.TotalReceived()-1), raw.Text)
	if !a.autoscroll {
		a.newLogs++
	}
	a.recomputeView()
}

func (a *App) recomputeView() {
	var view []*model.ParsedLine
	for i := 0; i < a.buffer.Len(); i++ {
		line := a.buffer.Get(i)
		if line == nil {
			continue
		}
		if !a.levelMask[line.Level] && line.Level != "" {
			continue
		}
		if a.filterTraceID != "" && line.TraceID != a.filterTraceID {
			continue
		}
		if a.filterThread != "" && line.Thread != a.filterThread {
			continue
		}
		if a.searchInput != "" {
			if !containsIgnoreCase(line.Message, a.searchInput) &&
				!containsIgnoreCase(line.Raw.Text, a.searchInput) {
				continue
			}
		}
		view = append(view, line)
	}
	a.filteredView = view
	a.stGroups = stacktrace.Detect(view)
}

func containsIgnoreCase(s, sub string) bool {
	ls, lsub := strings.ToLower(s), strings.ToLower(sub)
	return len(ls) >= len(lsub) && strings.Contains(ls, lsub)
}

func (a *App) handleNormalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return a, tea.Quit
	case "/":
		a.searchMode = true
		a.searchInput = ""
	case "tab":
		a.activePanel = (a.activePanel + 1) % 3
	case "f":
		a.activePanel = 0
	case "e":
		for _, g := range a.stGroups {
			if a.cursor >= g.Start && a.cursor <= g.End {
				a.expanded[g.Start] = !a.expanded[g.Start]
				break
			}
		}
	case "s":
		a.exportMode = true
	case "g":
		if a.cursor == 0 {
			a.cursor = max(0, len(a.filteredView)-1)
		} else {
			a.cursor = 0
		}
		a.autoscroll = (a.cursor == len(a.filteredView)-1)
	case "enter":
		if a.cursor < len(a.filteredView) {
			line := a.filteredView[a.cursor]
			if line.TraceID != "" && line.TraceID != "NA" {
				a.filterTraceID = line.TraceID
			}
			if line.Thread != "" {
				a.filterThread = line.Thread
			}
			a.activePanel = 2
			a.recomputeView()
		}
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
	switch msg.String() {
	case "esc":
		a.searchMode = false
	case "enter":
		a.searchMode = false
		a.recomputeView()
	case "backspace":
		if len(a.searchInput) > 0 {
			a.searchInput = a.searchInput[:len(a.searchInput)-1]
			a.recomputeView()
		}
	default:
		if len(msg.String()) == 1 {
			a.searchInput += msg.String()
			a.recomputeView()
		}
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
	return a.height - 6
}

func (a *App) View() string {
	if a.width == 0 {
		return "Loading..."
	}
	title := TitleStyle.Render(fmt.Sprintf(" LogView ─ %s [%s] ─ %d条 ", a.stream.Label(), a.parserName, a.buffer.Len()))
	logView := a.renderLogView()
	searchBar := a.renderSearchBar()
	panel := a.renderPanel()
	helpBar := a.renderHelpBar()
	base := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", title, logView, searchBar, panel, helpBar)
	if a.exportMode {
		return base + "\n" + a.renderExportDialog()
	}
	return base
}