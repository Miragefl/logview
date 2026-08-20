package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
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

type starField struct {
	Name  string
	Value string
}

type App struct {
	stream     stream.LogStream
	parsers    *parser.AutoDetect
	buffer     *buffer.RingBuffer
	keymap     KeyMap
	fieldAlias map[string]string // custom field -> standard field mapping

	cancelFunc context.CancelFunc
	streamCh   <-chan model.RawLine

	filteredView []*model.ParsedLine
	stGroups     []stacktrace.Group
	expanded     map[int]bool

	width        int
	height       int
	cursor       int
	offset       int
	autoscroll   bool
	scrollAnchor int // 0=auto, 1=top, 2=center, 3=bottom
	newLogs      int

	searchMode  bool
	searchTab   int // 0=搜索 1=高亮 2=隐藏
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

	pendingKey string

	levelFilter string
	wrapMode    bool

	starFields   []starField
	starCursor   int
	searchCursor int

	helpMode bool
	yankMsg  string

	highlights      []string
	highlightInput  string
	highlightCursor int

	hides         []string
	hideInput     string
	hideCursor    int
	hiddenByHides int // recomputeView 统计：当前被 hides 隐藏的行数（供状态栏提示）

	levelCounts map[string]int // 缓冲内级别统计（标题栏用）
	showKeyHints bool          // 底部快捷键提示栏开关（\ 切换）

	parserName string

	searchMatchCount int
	searchMatchIdx   int

	searchHistory    []string
	searchHistMode   bool // ctrl+r 历史列表 overlay 是否展开
	searchHistCursor int  // 列表选中索引，0=最新（列表倒序）

	showLineNum bool

	sourcePickerMode bool
	sourceTab        int // 0=K8s 1=本地 2=SSH
	pickSourceOnStart bool // 启动即打开源选择器（picker 子命令）

	pickerK8sLevel   int    // K8s 浏览层级：0=context 1=namespace 2=资源
	pickerContexts   []sourceCandidate
	pickerNamespaces []sourceCandidate
	pickerNsInput    string
	pickerNsCursor   int

	pickerLocalDir   string // 本地浏览当前目录

	pickerSSHHost    string // SSH 已连接浏览的主机（空=主机层）
	pickerSSHDir     string // 远程浏览当前目录
	pickerSSHRoot    string // 进入浏览时的起始目录（Backspace 到此层再按回主机层）
	pickerDirFilter  string // 目录浏览过滤前缀（本地/SSH 共用）
	pickerFilterCursor int
	k8sUseContextFn  func(string) error // 可注入的 context 切换（测试用）
	pickerHostInput  string
	pickerHostCursor int
	pickerRemotePath string
	pickerRemoteCursor int
	pickerSshFocus   int // SSH 主机层焦点：0=主机 1=路径

	pickerPathInput  string
	pickerPathCursor int
	pickerCursor     int
	pickerChecked    map[string]bool
	pickerCandidates []sourceCandidate // 当前层候选（k8s 资源层 / ssh 目录层）
	pickerLoading    bool

	sourceColorIdx map[string]int

	rulesPath string

	bookmarks   map[uint64]bool
	bookmarkSeq []uint64
}

type ExportState struct {
	Scope    int
	Format   int
	FilePath string
	Cursor   int
	Done     bool
	Err      error
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
		stream:         src,
		parsers:        parsers,
		buffer:         buffer.NewRingBuffer(bufSize),
		keymap:         DefaultKeyMap(),
		fieldMask:      fm,
		fieldAlias:     overrideFieldAlias,
		expanded:       make(map[int]bool),
		hides:          hides,
		autoscroll:     true,
		showKeyHints:   true,
		exportState:    newExportState(),
		sourceColorIdx: make(map[string]int),
		bookmarks:      make(map[uint64]bool),
	}
}

type batchMsg struct{ lines []model.RawLine }
type tickMsg struct{}

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

func (a *App) Init() tea.Cmd {
	return a.openSourcePickerInit()
}

// openSourcePickerInit 预备初始 cmd：启动即打开源选择器（picker 子命令）时附带候选拉取。
func (a *App) openSourcePickerInit() tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	a.cancelFunc = cancel
	ch, err := a.stream.Start(ctx)
	if err != nil {
		cancel()
		return nil
	}
	a.streamCh = ch
	base := tea.Batch(waitForStream(ch), tickEvery())
	if a.pickSourceOnStart {
		a.openSourcePicker(0)
		return tea.Batch(base, fetchK8sContextsCmd())
	}
	return base
}

// ReplaceStream 热切换日志源：停旧流、起新流、清屏重置全部视图状态。
// 返回 waitForStream cmd（新流的监听必须重新启动）。
func (a *App) ReplaceStream(src stream.LogStream) tea.Cmd {
	if a.cancelFunc != nil {
		a.cancelFunc()
	}
	a.stream.Cleanup()
	a.stream = src
	ctx, cancel := context.WithCancel(context.Background())
	a.cancelFunc = cancel
	// 无论 Start 成败都按新源重置视图（失败时在干净视图上展示错误行）
	a.buffer.Clear()
	a.filteredView = nil
	a.stGroups = nil
	a.expanded = make(map[int]bool)
	a.levelCounts = make(map[string]int)
	a.bookmarks = make(map[uint64]bool)
	a.bookmarkSeq = nil
	a.cursor = 0
	a.offset = 0
	a.autoscroll = true

	ch, err := src.Start(ctx)
	if err != nil {
		cancel()
		a.cancelFunc = nil
		a.appendErrorLine(fmt.Sprintf("打开源失败 %s: %v", src.Label(), err))
		return nil
	}
	a.streamCh = ch
	// 切换提示行（非错误，便于确认新源）
	a.processLine(model.RawLine{Text: fmt.Sprintf("-- 已切换到 %s --", src.Label()), Source: "logview"})
	a.yankMsg = ""
	return waitForStream(ch)
}

// appendErrorLine 在日志流顶部插入一条本地错误提示行。
func (a *App) appendErrorLine(msg string) {
	raw := model.RawLine{Text: "ERROR " + msg, Source: "logview"}
	a.processLine(raw)
}

func (a *App) shutdown() tea.Cmd {
	SaveSession(SessionState{
		SearchQuery:  a.searchInput,
		LevelFilter:  a.levelFilter,
		HiddenFields: a.hides,
		ShowLineNum:  a.showLineNum,
	})
	if a.cancelFunc != nil {
		a.cancelFunc()
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
		return a, waitForStream(a.streamCh)
	case tickMsg:
		return a, tickEvery()
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return a, a.shutdown()
		}
		if a.helpMode {
			return a.handleHelpKeys(msg)
		}
		if a.exportMode {
			return a.handleExportKeys(msg)
		}
		if a.searchMode {
			return a.handleSearchKeys(msg)
		}
		if a.sourcePickerMode {
			return a.handleSourcePickerKeys(msg)
		}
		if a.panelFocus {
			return a.handlePanelKeys(msg)
		}
		return a.handleNormalKeys(msg)
	case tea.InterruptMsg:
		return a, a.shutdown()
	case candidatesMsg:
		// 候选异步回填（仅当仍停留在选择器对应层）
		if !a.sourcePickerMode || a.sourceTab != msg.tab {
			return a, nil
		}
		switch msg.kind {
		case "contexts":
			if a.pickerK8sLevel == 0 {
				a.pickerContexts = msg.items
			}
		case "namespaces":
			if a.pickerK8sLevel == 1 {
				a.pickerNamespaces = msg.items
			}
		case "sshdir":
			if a.pickerSSHHost != "" && msg.ns == a.pickerSSHHost+":"+a.pickerSSHDir {
				a.pickerCandidates = msg.items
			}
		default: // resources
			if a.pickerK8sLevel == 2 && msg.ns == a.pickerNsInput {
				a.pickerCandidates = msg.items
			}
		}
		// 回填后总清 loading（空列表也视为完成，UI 显示"无资源"）
		a.pickerLoading = false
		return a, nil
	}
	return a, nil
}

func (a *App) processBatch(lines []model.RawLine) {
	for _, raw := range lines {
		a.processLine(raw)
	}
	if a.cursor >= len(a.filteredView) {
		a.cursor = max(0, len(a.filteredView)-1)
	}
	a.stGroups = stacktrace.Detect(a.filteredView)
}

func (a *App) processLine(raw model.RawLine) {
	raw.Text = model.StripANSI(raw.Text)
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
	if lv := pl.Get(model.FieldLevel); lv != "" {
		if a.levelCounts == nil {
			a.levelCounts = make(map[string]int)
		}
		a.levelCounts[lv]++
	}
	if len(a.hides) > 0 && a.matchHides(pl) {
		a.hiddenByHides++
	}
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
	hiddenByHides := 0
	levelCounts := make(map[string]int)
	for i := 0; i < a.buffer.Len(); i++ {
		line := a.buffer.Get(i)
		if line == nil {
			continue
		}
		if lv := line.Get(model.FieldLevel); lv != "" {
			levelCounts[lv]++
		}
		if len(a.hides) > 0 && a.matchHides(line) {
			hiddenByHides++
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
	a.hiddenByHides = hiddenByHides
	a.levelCounts = levelCounts
	a.filteredView = view
	if a.cursor >= len(a.filteredView) {
		a.cursor = max(0, len(a.filteredView)-1)
	}
	a.stGroups = stacktrace.Detect(view)
	a.updateSearchStats()
}

func (a *App) SetRulesPath(path string) {
	a.rulesPath = path
}

// OpenSourcePickerOnStart 标记启动即打开源选择器（logview picker 子命令）。
func (a *App) OpenSourcePickerOnStart() {
	a.pickSourceOnStart = true
}

// recountLevels 重算缓冲内级别统计（recomputeView 内已同步维护，此函数供特殊路径调用）。
func (a *App) recountLevels() {
	counts := make(map[string]int)
	for i := 0; i < a.buffer.Len(); i++ {
		line := a.buffer.Get(i)
		if line == nil {
			continue
		}
		if lv := line.Get(model.FieldLevel); lv != "" {
			counts[lv]++
		}
	}
	a.levelCounts = counts
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
	if a.handlePendingKey(msg) {
		return a, nil
	}
	if a.visualMode {
		return a.handleVisualKeys(msg)
	}

	switch msg.String() {
	case "q":
		return a, tea.Quit
	case "C":
		a.clearScreen()
	case "esc":
		a.escapeToNormal()
	case "/", "f":
		a.openSearch()
	case "v", "V":
		a.beginVisual()
	case "y":
		a.yankLines(a.cursor, a.cursor)
	case "F":
		a.panelFocus = true
		a.fieldCursor = 0
	case "?":
		a.helpMode = true
	case "h":
		a.openUnifiedPopup(1)
	case "x":
		a.openUnifiedPopup(2)
	case "\\":
		a.showKeyHints = !a.showKeyHints
	case "o":
		a.openSourcePicker(0)
		return a, fetchK8sContextsCmd()
	case "s":
		a.exportMode = true
	case "n":
		a.jumpSearchMatch(1)
	case "N":
		a.jumpSearchMatch(-1)
	case "w":
		a.wrapMode = !a.wrapMode
	case "#":
		a.showLineNum = !a.showLineNum
	case "S":
		a.statsPanel = !a.statsPanel
	case "m":
		a.toggleBookmark()
	case "'":
		a.jumpBookmark()
	case "e":
		a.toggleFold()
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
	case "g", "G", "H", "M", "L",
		"ctrl+d", "ctrl+f", "ctrl+u", "ctrl+b",
		"up", "k", "down", "j", "pgup", "pgdown":
		a.moveCursor(msg.String())
	}
	return a, nil
}

// handlePendingKey 处理 zt/zz/zb 等多键序列的第二键；返回 true 表示已消费。
func (a *App) handlePendingKey(msg tea.KeyMsg) bool {
	if a.pendingKey == "" {
		return false
	}
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
	}
	a.pendingKey = ""
	return true
}

func (a *App) clearScreen() {
	a.buffer.Clear()
	a.filteredView = nil
	a.stGroups = nil
	a.expanded = make(map[int]bool)
	a.levelCounts = make(map[string]int)
	a.cursor = 0
	a.offset = 0
	a.newLogs = 0
	a.yankMsg = "屏幕已清空"
}

// escapeToNormal：Esc 逐层退出（统计面板 → 清空搜索词并保持当前行）。
func (a *App) escapeToNormal() {
	if a.statsPanel {
		a.statsPanel = false
		return
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
}

func (a *App) openSearch() {
	a.openUnifiedPopup(0)
}

func (a *App) beginVisual() {
	a.visualMode = true
	a.visualStart = a.cursor
	a.autoscroll = false
}

// openUnifiedPopup 打开统一弹窗（搜索/高亮/隐藏三 tab），tab 指定直达分区。
func (a *App) openUnifiedPopup(tab int) {
	a.searchMode = true
	a.searchTab = tab
	a.searchHistMode = false
	if tab == 0 {
		a.populateSearchFields()
		a.searchCursor = len([]rune(a.searchInput))
	} else if tab == 1 {
		if a.highlightInput == "" && len(a.highlights) > 0 {
			a.highlightInput = strings.Join(a.highlights, ", ")
		}
		a.highlightCursor = len([]rune(a.highlightInput))
	} else {
		if a.hideInput == "" && len(a.hides) > 0 {
			a.hideInput = strings.Join(a.hides, ", ")
		}
		a.hideCursor = len([]rune(a.hideInput))
	}
}

func (a *App) toggleBookmark() {
	if len(a.filteredView) > 0 && a.cursor >= 0 && a.cursor < len(a.filteredView) {
		seq := a.filteredView[a.cursor].Raw.Seq
		if a.bookmarks[seq] {
			delete(a.bookmarks, seq)
		} else {
			a.bookmarks[seq] = true
			a.bookmarkSeq = append(a.bookmarkSeq, seq)
		}
	}
}

// toggleFold 展开或折叠光标所在的堆栈组。
func (a *App) toggleFold() {
	for _, g := range a.stGroups {
		if a.cursor >= g.Start && a.cursor <= g.End {
			a.expanded[g.Start] = !a.expanded[g.Start]
			return
		}
	}
}

// moveCursor 统一处理光标移动键族（gg/G/H/M/L/半页/整页/上下/pgup/pgdown）。
func (a *App) moveCursor(key string) {
	last := len(a.filteredView) - 1
	switch key {
	case "g":
		a.cursor = 0
		a.autoscroll = false
		a.scrollAnchor = 0
	case "G":
		a.cursor = max(0, last)
		a.autoscroll = true
		a.scrollAnchor = 0
	case "H":
		a.cursor = a.offset
		a.autoscroll = false
		a.scrollAnchor = 0
	case "M":
		a.cursor = a.offset + a.visibleLines()/2
		if a.cursor > last {
			a.cursor = last
		}
		a.autoscroll = false
		a.scrollAnchor = 0
	case "L":
		a.cursor = a.offset + a.visibleLines() - 1
		if a.cursor > last {
			a.cursor = last
		}
		a.autoscroll = false
		a.scrollAnchor = 0
	case "ctrl+d":
		a.cursor += a.visibleLines() / 2
		if a.cursor > last {
			a.cursor = max(0, last)
		}
		a.cursor = a.skipFolded(a.cursor, 1)
		a.autoscroll = (a.cursor == last)
	case "ctrl+f":
		a.cursor += a.visibleLines()
		if a.cursor > last {
			a.cursor = last
		}
		a.cursor = a.skipFolded(a.cursor, 1)
		a.autoscroll = (a.cursor == last)
	case "ctrl+u":
		a.cursor -= a.visibleLines() / 2
		if a.cursor < 0 {
			a.cursor = 0
		}
		a.cursor = a.skipFolded(a.cursor, -1)
		a.autoscroll = false
	case "ctrl+b":
		a.cursor -= a.visibleLines()
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
		if a.cursor < last {
			a.cursor++
			a.cursor = a.skipFolded(a.cursor, 1)
			if a.cursor > last {
				a.cursor = last
			}
		}
		a.autoscroll = (a.cursor == last)
	case "pgup":
		a.cursor -= a.visibleLines()
		if a.cursor < 0 {
			a.cursor = 0
		}
		a.cursor = a.skipFolded(a.cursor, -1)
		a.autoscroll = false
	case "pgdown":
		a.cursor += a.visibleLines()
		if a.cursor > last {
			a.cursor = last
		}
		a.cursor = a.skipFolded(a.cursor, 1)
		a.autoscroll = (a.cursor == last)
	}
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
	n, err := export.ToFile(lines, s.FilePath, format)
	s.Done = true
	s.Err = err
	s.Exported = n
}

func (a *App) visibleLines() int {
	// fixed lines: title, sep, bar, sep, sep(bottom) = 5, plus footer (status bar + optional key hints)
	vl := a.height - 5 - a.footerHeight()
	if vl < 1 {
		vl = 1
	}
	return vl
}

// levelBadge 返回标题栏级别统计（E:3 W:30 形式），无统计时为空。
func (a *App) levelBadge() string {
	e := a.levelCounts["ERROR"] + a.levelCounts["ERR"] + a.levelCounts["FATAL"]
	w := a.levelCounts["WARN"] + a.levelCounts["WARNING"]
	var parts []string
	if e > 0 {
		parts = append(parts, fmt.Sprintf(" E:%d", e))
	}
	if w > 0 {
		parts = append(parts, fmt.Sprintf(" W:%d", w))
	}
	if len(parts) == 0 {
		return ""
	}
	return " ─" + strings.Join(parts, "")
}

// scrollPercent 返回光标位置百分比（" ─ 62%"），无内容时为空。
func (a *App) scrollPercent() string {
	n := len(a.filteredView)
	if n == 0 {
		return ""
	}
	pct := (a.cursor + 1) * 100 / n
	return fmt.Sprintf(" ─ %d%%", pct)
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
		fmt.Sprintf(" LogView ─ %s [%s]%s ─ %d条%s",
			a.streamLabel(), pLabel, a.levelBadge(), a.buffer.Len(), a.scrollPercent()),
	)

	sep := strings.Repeat(HorizontalLine, w)

	// truncate every line to terminal width to prevent wrapping
	trunc := lipgloss.NewStyle().MaxWidth(w)
	bar := trunc.Render(a.renderSearchBar())
	helpBar := a.renderFooter()

	vl := a.visibleLines()
	var logLines []string
	if a.searchMode {
		logLines = a.buildSearchModeLines(vl)
	} else if a.sourcePickerMode {
		logLines = a.buildSourcePickerLines(vl)
	} else if a.helpMode {
		logLines = a.buildHelpPopup(vl)
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
	for _, l := range logLines {
		allLines = append(allLines, trunc.Render(l))
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

// buildSearchModeLines 渲染搜索模式下的日志区：popup 紧贴搜索栏 inline 显示，
// 日志在下方独立渲染，互不覆盖。按可用高度限制 popup（先砍 starFields 字段建议），
// 保证匹配日志始终可见。
func (a *App) buildSearchModeLines(vl int) []string {
	logReserve := vl / 3
	if logReserve < 3 {
		logReserve = 3
	}
	popupMaxH := vl - logReserve
	if popupMaxH < 1 {
		popupMaxH = 1
	}
	popupLines := a.buildSearchPopup(popupMaxH)
	return a.inlinePopupLines(strings.Join(popupLines, "\n"), vl)
}

// inlinePopupLines 把 popup 行列表渲染进日志区（上方 popup + 下方保留日志）。
func (a *App) inlinePopupLines(popupContent string, vl int) []string {
	popupLines := strings.Split(popupContent, "\n")
	popStyle := lipgloss.NewStyle().Width(a.width).MaxWidth(a.width)

	var lines []string
	for _, p := range popupLines {
		lines = append(lines, popStyle.Render(p))
	}
	ph := len(popupLines)
	if ph > vl {
		ph = vl
	}
	if remain := vl - ph; remain > 0 {
		lines = append(lines, a.buildLogLines(remain)...)
	}
	return lines
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
	if err := copyToClipboard(buf.String()); err != nil {
		a.yankMsg = fmt.Sprintf("复制失败: %v", err)
	} else {
		a.yankMsg = fmt.Sprintf("已复制 %d 行", end-start+1)
	}
	a.visualMode = false
}

func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		if p, _ := exec.LookPath("wl-copy"); p != "" {
			cmd = exec.Command("wl-copy")
		} else {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		}
	default:
		cmd = exec.Command("pbcopy")
	}
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("clipboard command %s: %w", cmd.Path, err)
	}
	return nil
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
	a.recountLevels()
}
