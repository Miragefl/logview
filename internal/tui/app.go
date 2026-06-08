package tui

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"sort"
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
	autoscroll    bool
	scrollAnchor  int // 0=auto, 1=top, 2=center, 3=bottom
	newLogs       int

	searchMode  bool
	searchInput string
	cachedQuery SearchQuery

	fieldMask model.FieldMask

	panelFocus  bool
	statsPanel  bool
	fieldCursor int

	exportMode  bool
	exportState ExportState

	visualMode  bool
	visualStart int

	pendingKey  string

	levelFilter string
	wrapMode  bool

	starFields  []starField
	starCursor   int
		searchCursor int

		helpMode      bool
		yankMsg       string

	highlights     []string
	highlightMode  bool
	highlightInput string

	hides     []string
	hideMode  bool
	hideInput string

	parserName string

	searchMatchCount int
	searchMatchIdx   int

	searchHistory []string
	searchHistIdx int

	showLineNum bool

	sourceColorIdx map[string]int

	rulesPath     string
	configWatcher io.Closer
	configToast   string
	reloadFunc    func()

	bookmarks   map[uint64]bool
	bookmarkSeq []uint64
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

func NewApp(src stream.LogStream, parsers *parser.AutoDetect, bufSize int, hides []string) *App {
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
		hides:       hides,
		autoscroll:  true,
		exportState: newExportState(),
		sourceColorIdx: make(map[string]int),
		bookmarks:      make(map[uint64]bool),
	}
}

type batchMsg struct{ lines []model.RawLine }
type tickMsg struct{}
type configReloadMsg struct{}
type configToastMsg struct{}

func waitForStream(ch <-chan model.RawLine) tea.Cmd {
	return func() tea.Msg {
		var lines []model.RawLine
		line, ok := <-ch
		if !ok {
			return nil
		}
		lines = append(lines, line)
	loop:
		for len(lines) < 1000 {
			select {
			case l, ok := <-ch:
				if !ok {
					break loop
				}
				lines = append(lines, l)
			default:
				break loop
			}
		}
		return batchMsg{lines: lines}
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
	cmds := []tea.Cmd{waitForStream(ch), tickEvery()}
	if a.rulesPath != "" {
		reloadCh := make(chan struct{}, 1)
		if w, err := parser.WatchRules(a.rulesPath, func() {
			if a.reloadFunc != nil {
				a.reloadFunc()
			}
			select {
			case reloadCh <- struct{}{}:
			default:
			}
		}); err == nil {
			a.configWatcher = w
			cmds = append(cmds, func() tea.Msg {
				<-reloadCh
				return configReloadMsg{}
			})
		}
	}
	return tea.Batch(cmds...)
}

func (a *App) shutdown() tea.Cmd {
	SaveSession(SessionState{
		SearchQuery:  a.searchInput,
		LevelFilter:  a.levelFilter,
		HiddenFields: a.hides,
		ShowLineNum:  a.showLineNum,
	})
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
	case batchMsg:
		a.processBatch(msg.lines)
		return a, waitForStream(streamCh)
	case tickMsg:
		return a, tickEvery()
	case configReloadMsg:
		if a.reloadFunc != nil {
			a.reloadFunc()
			a.configToast = "配置已重新加载"
			return a, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return configToastMsg{} })
		}
		return a, nil
	case configToastMsg:
		a.configToast = ""
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
		if a.hideMode {
			return a.handleHideKeys(msg)
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

func (a *App) processBatch(lines []model.RawLine) {
	for _, raw := range lines {
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
		if _, ok := a.sourceColorIdx[pl.Raw.Source]; !ok {
			a.sourceColorIdx[pl.Raw.Source] = len(a.sourceColorIdx) % len(SourceColors)
		}
		a.buffer.Push(pl)
		a.searchIdx.Add(int(a.buffer.TotalReceived()-1), raw.Text)
		if a.matchLineForFilter(pl) {
			a.filteredView = append(a.filteredView, pl)
		}
	}
	if !a.autoscroll {
		a.newLogs += len(lines)
	}
	if a.cursor >= len(a.filteredView) {
		a.cursor = max(0, len(a.filteredView)-1)
	}
	a.stGroups = stacktrace.Detect(a.filteredView)
}

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
	if a.matchLineForFilter(pl) {
		a.filteredView = append(a.filteredView, pl)
	}
	if !a.autoscroll {
		a.newLogs++
	}
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

func (a *App) matchLineForFilter(line *model.ParsedLine) bool {
	if a.searchInput != "" {
		if !a.currentQuery().MatchLine(line) {
			return false
		}
	}
	if a.levelFilter != "" {
		if !a.matchLevelFilter(line) {
			return false
		}
	}
	if len(a.hides) > 0 {
		if a.matchHides(line) {
			return false
		}
	}
	return true
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
		if len(a.hides) > 0 {
			if a.matchHides(line) {
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
	a.updateSearchStats()
}

func (a *App) updateSearchStats() {
	if a.searchInput == "" {
		a.searchMatchCount = 0
		a.searchMatchIdx = 0
		return
	}
	q := a.currentQuery()
	count := 0
	idx := 0
	for i, line := range a.filteredView {
		if q.MatchLine(line) {
			count++
			if i <= a.cursor {
				idx = count
			}
		}
	}
	a.searchMatchCount = count
	a.searchMatchIdx = idx
}

func (a *App) SetRulesPath(path string) {
	a.rulesPath = path
}

func (a *App) SetReloadFunc(fn func()) {
	a.reloadFunc = fn
}

func (a *App) jumpBookmark() {
	if len(a.bookmarkSeq) == 0 || len(a.filteredView) == 0 {
		return
	}
	// collect bookmark positions in filteredView order
	var positions []int
	for i, line := range a.filteredView {
		if a.bookmarks[line.Raw.Seq] {
			positions = append(positions, i)
		}
	}
	if len(positions) == 0 {
		return
	}
	// find next bookmark after cursor
	idx := sort.Search(len(positions), func(i int) bool { return positions[i] > a.cursor })
	if idx >= len(positions) {
		idx = 0
	}
	a.cursor = positions[idx]
	a.autoscroll = false
}

func (a *App) streamLabel() string {
	label := a.stream.Label()
	if label == "file" {
		return "只读"
	}
	return "跟踪中"
}

func (a *App) addSearchHistory(query string) {
	if query == "" {
		return
	}
	// deduplicate
	for i, h := range a.searchHistory {
		if h == query {
			a.searchHistory = append(a.searchHistory[:i], a.searchHistory[i+1:]...)
			break
		}
	}
	a.searchHistory = append(a.searchHistory, query)
	if len(a.searchHistory) > 20 {
		a.searchHistory = a.searchHistory[len(a.searchHistory)-20:]
	}
	a.searchHistIdx = 0
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

func (a *App) matchHides(line *model.ParsedLine) bool {
	text := line.Raw.Text
	for _, kw := range a.hides {
		if containsIgnoreCase(text, kw) {
			return true
		}
	}
	return false
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
	if len(a.filteredView) == 0 {
		return
	}
	var matches []int
	if a.searchInput != "" {
		q := a.currentQuery()
		if q.IsEmpty() {
			return
		}
		for i, line := range a.filteredView {
			if q.MatchLine(line) {
				matches = append(matches, i)
			}
		}
	} else if len(a.highlights) > 0 {
		for i, line := range a.filteredView {
			msg := line.Get(model.FieldMessage)
			for _, kw := range a.highlights {
				if strings.Contains(msg, kw) {
					matches = append(matches, i)
					break
				}
			}
		}
	}
	if len(matches) == 0 {
		return
	}
	cur := a.cursor
	idx := sort.Search(len(matches), func(i int) bool { return matches[i] >= cur })
	if dir > 0 {
		next := idx + 1
		if next >= len(matches) {
			next = 0
		}
		a.cursor = matches[next]
	} else {
		prev := idx - 1
		if prev < 0 {
			prev = len(matches) - 1
		}
		a.cursor = matches[prev]
	}
	a.autoscroll = false
	a.updateSearchStats()
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
	a.yankMsg = ""
	// handle pending key sequences (viw, zt/zz/zb)
	if a.pendingKey != "" {
		key := msg.String()
		switch a.pendingKey {
		case "z":
			switch key {
			case "t":
				a.scrollAnchor = 1
			case "z":
				a.scrollAnchor = 2
			case "b":
				a.scrollAnchor = 3
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
	case "C":
		a.buffer.Clear()
		a.searchIdx.Clear()
		a.filteredView = nil
		a.stGroups = nil
		a.expanded = make(map[int]bool)
		a.cursor = 0
		a.offset = 0
		a.newLogs = 0
		a.yankMsg = "屏幕已清空"
		return a, nil
	case "q":
		return a, tea.Quit
	case "esc":
		if a.statsPanel {
			a.statsPanel = false
			return a, nil
		}
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
	case "x":
		a.hideMode = true
		if a.hideInput == "" && len(a.hides) > 0 {
			a.hideInput = strings.Join(a.hides, ", ")
		}
	case "s":
		a.exportMode = true
	case "g":
		a.cursor = 0
		a.autoscroll = false
		a.scrollAnchor = 0
	case "G":
		a.cursor = max(0, len(a.filteredView)-1)
		a.autoscroll = true
		a.scrollAnchor = 0
	case "n":
		a.jumpSearchMatch(1)
	case "N":
		a.jumpSearchMatch(-1)
	case "H":
		a.cursor = a.offset
		a.autoscroll = false
		a.scrollAnchor = 0
	case "M":
		a.cursor = a.offset + a.visibleLines()/2
		if a.cursor >= len(a.filteredView) { a.cursor = len(a.filteredView)-1 }
		a.autoscroll = false
		a.scrollAnchor = 0
	case "L":
		a.cursor = a.offset + a.visibleLines() - 1
		if a.cursor >= len(a.filteredView) { a.cursor = len(a.filteredView)-1 }
		a.autoscroll = false
		a.scrollAnchor = 0
		case "w":
			a.wrapMode = !a.wrapMode
		case "#":
			a.showLineNum = !a.showLineNum
		case "S":
			a.statsPanel = !a.statsPanel
		case "m":
			if len(a.filteredView) > 0 && a.cursor >= 0 && a.cursor < len(a.filteredView) {
				seq := a.filteredView[a.cursor].Raw.Seq
				if a.bookmarks[seq] {
					delete(a.bookmarks, seq)
				} else {
					a.bookmarks[seq] = true
					a.bookmarkSeq = append(a.bookmarkSeq, seq)
				}
			}
		case "'":
			a.jumpBookmark()
			_ = 0
	case "e":
		for _, g := range a.stGroups {
			if a.cursor >= g.Start && a.cursor <= g.End {
				a.expanded[g.Start] = !a.expanded[g.Start]
				break
			}
		}
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
		a.cursor = a.skipFolded(a.cursor, 1)
		a.autoscroll = (a.cursor == len(a.filteredView)-1)
	case "ctrl+f":
		ps := a.visibleLines()
		a.cursor += ps
		if a.cursor >= len(a.filteredView) {
			a.cursor = len(a.filteredView) - 1
		}
		a.cursor = a.skipFolded(a.cursor, 1)
		a.autoscroll = (a.cursor == len(a.filteredView)-1)
	case "ctrl+u":
		hs := a.visibleLines() / 2
		a.cursor -= hs
		if a.cursor < 0 {
			a.cursor = 0
		}
		a.cursor = a.skipFolded(a.cursor, -1)
		a.autoscroll = false
	case "ctrl+b":
		ps := a.visibleLines()
		a.cursor -= ps
		if a.cursor < 0 {
			a.cursor = 0
		}
		a.cursor = a.skipFolded(a.cursor, -1)
		a.autoscroll = false
	case "up", "k":
		if a.cursor > 0 {
			a.cursor--
			a.cursor = a.skipFolded(a.cursor, -1)
			a.autoscroll = false
		}
	case "down", "j":
		if a.cursor < len(a.filteredView)-1 {
			a.cursor++
			a.cursor = a.skipFolded(a.cursor, 1)
			if a.cursor >= len(a.filteredView) {
				a.cursor = len(a.filteredView) - 1
			}
		}
		a.autoscroll = (a.cursor == len(a.filteredView)-1)
	case "pgup":
		ps := a.visibleLines()
		a.cursor -= ps
		if a.cursor < 0 {
			a.cursor = 0
		}
		a.cursor = a.skipFolded(a.cursor, -1)
		a.autoscroll = false
	case "pgdown":
		ps := a.visibleLines()
		a.cursor += ps
		if a.cursor >= len(a.filteredView) {
			a.cursor = len(a.filteredView) - 1
		}
		a.cursor = a.skipFolded(a.cursor, 1)
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
		a.addSearchHistory(a.searchInput)
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
		case "ctrl+r":
			if len(a.searchHistory) > 0 {
				if a.searchHistIdx == 0 {
					a.searchHistIdx = len(a.searchHistory)
				}
				a.searchHistIdx--
				a.searchInput = a.searchHistory[a.searchHistIdx]
				a.searchCursor = len([]rune(a.searchInput))
				a.recomputeView()
			}
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

func (a *App) handleHideKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		a.hideMode = false
	case tea.KeyEnter:
		kw := strings.TrimSpace(a.hideInput)
		if kw != "" {
			a.hides = strings.Split(kw, ",")
			for i := range a.hides {
				a.hides[i] = strings.TrimSpace(a.hides[i])
			}
			var clean []string
			for _, h := range a.hides {
				if h != "" {
					clean = append(clean, h)
				}
			}
			a.hides = clean
		} else {
			a.hides = nil
		}
		a.hideMode = false
		a.recomputeView()
	case tea.KeyBackspace:
		if len(a.hideInput) > 0 {
			runes := []rune(a.hideInput)
			a.hideInput = string(runes[:len(runes)-1])
		}
	case tea.KeyRunes:
		a.hideInput += string(msg.Runes)
	default:
		switch msg.String() {
		case "ctrl+u":
			a.hideInput = ""
		case " ":
			a.hideInput += " "
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
		fmt.Sprintf(" LogView ─ %s [%s] ─ %d条", a.streamLabel(), pLabel, a.buffer.Len()),
	)
	if a.configToast != "" {
		title += "  " + NewLogStyle.Render(a.configToast)
	}

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
	} else if a.hideMode {
		logLines = a.buildHidePopup(vl)
	} else if a.exportMode {
		logLines = a.buildExportPopup(vl)
	} else if a.panelFocus {
		logLines = a.buildPopupLines(vl)
	} else if a.statsPanel {
		logLines = a.buildStatsPanel(vl)
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
	out := strings.Join(allLines, "\n")
	if AppBgSeq != "" {
		reset := "\x1b[0m"
		out = strings.ReplaceAll(out, reset, reset+AppBgSeq)
		out = AppBgSeq + out + reset
	}
	return out
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
	a.yankMsg = fmt.Sprintf("已复制 %d 行", end-start+1)
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

func (a *App) ApplySession(s *SessionState) {
	if s.SearchQuery != "" {
		a.searchInput = s.SearchQuery
		a.searchCursor = len([]rune(s.SearchQuery))
	}
	if s.LevelFilter != "" {
		a.levelFilter = s.LevelFilter
	}
	if len(s.HiddenFields) > 0 {
		a.hides = s.HiddenFields
	}
	a.showLineNum = s.ShowLineNum
	a.recomputeView()
}
