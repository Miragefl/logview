package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Miragefl/logview/internal/model"
	"github.com/Miragefl/logview/internal/parser"
)

// mockStream implements stream.LogStream for testing
type mockStream struct{}

func (m *mockStream) Start(_ context.Context) (<-chan []model.RawLine, error) {
	ch := make(chan []model.RawLine)
	close(ch)
	return ch, nil
}
func (m *mockStream) Label() string    { return "test" }
func (m *mockStream) Scope() string    { return "" }
func (m *mockStream) Cleanup() error   { return nil }

func newTestApp() *App {
	app := NewApp(&mockStream{}, nil, 1000, nil)
	app.width = 120
	app.height = 40
	app.availableTabs = []int{0, 1, 2, 3} // 固定全量：环境过滤由专门测试覆盖
	// push some test data
	for i := 0; i < 20; i++ {
		app.processLine(model.RawLine{
			Text:   "2026-05-15 09:27:01.130 [http-nio-80-exec-3] [abc123] INFO  com.example.App - test message",
			Source: "test-pod",
		})
	}
	return app
}

// 问题1: TUI 应该占满终端宽度
func TestViewFillsWidth(t *testing.T) {
	app := newTestApp()
	view := app.View()

	lines := strings.Split(view, "\n")
	if len(lines) < 5 {
		t.Fatalf("View has only %d lines, expected at least 5", len(lines))
	}

	// title 应该占满宽度
	title := lines[0]
	// lipgloss 渲染后可能带 ANSI 码，用 strip 后检查
	cleanTitle := stripANSI(title)
	if len(cleanTitle) < 80 {
		t.Errorf("title line too short (%d chars), should fill width 120: %q", len(cleanTitle), cleanTitle)
	}
}

// 问题2: 日志区应有圆角边框盒（顶行嵌标题、底行嵌统计）
func TestViewHasSeparators(t *testing.T) {
	app := newTestApp()
	view := app.View()

	if !strings.Contains(view, "─") {
		t.Error("View should contain frame lines (─)")
	}

	lines := strings.Split(view, "\n")
	if len(lines) < 3 {
		t.Fatalf("View too short: %d lines", len(lines))
	}
	first := stripANSI(lines[0])
	if !strings.HasPrefix(first, "╭─") {
		t.Errorf("first line should be frame top (╭─), got %q", first)
	}
	hasBottom := false
	for _, line := range lines {
		if strings.HasPrefix(stripANSI(line), "╰─") {
			hasBottom = true
			break
		}
	}
	if !hasBottom {
		t.Error("frame bottom (╰─) not found")
	}
	if !strings.Contains(first, "LogView") {
		t.Error("frame top should embed title")
	}
}

// 问题3: 搜索应该能工作
func TestSearchWorks(t *testing.T) {
	app := newTestApp()

	// 按 / 进入搜索模式
	_, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})

	if !app.searchMode {
		t.Fatal("searchMode should be true after pressing /")
	}

	// 输入搜索词 "test"
	_, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	_, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	_, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	_, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})

	if app.searchInput != "test" {
		t.Errorf("searchInput = %q, want 'test'", app.searchInput)
	}

	// 搜索应该过滤日志
	if len(app.filteredView) == 0 {
		t.Error("filteredView should have results after searching 'test'")
	}

	// 按 Enter 退出搜索
	_, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if app.searchMode {
		t.Error("searchMode should be false after Enter")
	}

	// 搜索词应该保留
	if app.searchInput != "test" {
		t.Errorf("searchInput should be preserved, got %q", app.searchInput)
	}

	// View 应该显示搜索词
	view := app.View()
	if !strings.Contains(view, "test") {
		t.Error("View should show the search term")
	}
}

// 搜索模式弹出搜索框
func TestSearchDialogPopup(t *testing.T) {
	app := newTestApp()
	// 按 / 进入搜索
	_, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !app.searchMode {
		t.Fatal("searchMode should be true after pressing /")
	}
	view := app.View()
	if !strings.Contains(view, "搜索:") {
		t.Error("search dialog should show '搜索:' when in search mode")
	}
	if !strings.Contains(view, "Esc") {
		t.Error("search dialog should show Esc hint")
	}
}

// 问题4: 按f应该进入字段面板交互模式
func TestFieldPanelWorks(t *testing.T) {
	app := newTestApp()

	// 记录初始状态
	initialMask := app.fieldMask[model.FieldThread]

	// 按 f 进入字段面板
	_, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})

	if !app.panelFocus {
		t.Fatal("panelFocus should be true after pressing F")
	}

	// View should show popup with field settings
	view := app.View()
	if !strings.Contains(view, "字段") {
		t.Error("View should show field popup after pressing f")
	}

	// 按 C-j 移动光标到 thread 字段
	// AllFields: time(0), source(1), level(2), thread(3), traceId(4), logger(5), message(6)
	_, _ = app.Update(fakeKey("ctrl+j")) // -> fieldCursor=1 (source)
	_, _ = app.Update(fakeKey("ctrl+j")) // -> fieldCursor=2 (level)
	_, _ = app.Update(fakeKey("ctrl+j")) // -> fieldCursor=3 (thread)

	// 按空格切换 thread 的显示
	_, _ = app.Update(tea.KeyMsg{Type: tea.KeySpace})

	// thread 的可见性应该翻转
	if app.fieldMask.IsVisible(model.FieldThread) == initialMask {
		t.Error("thread field visibility should have toggled")
	}

	// 按 Esc 退出面板
	_, _ = app.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if app.panelFocus {
		t.Error("panelFocus should be false after Esc")
	}
}

// 综合测试: View 结构完整性
func TestViewStructure(t *testing.T) {
	app := newTestApp()
	view := app.View()

	checks := []struct {
		name    string
		content string
	}{
		{"title", "LogView"},
		{"separator", "─"},
		{"search bar", "LogView"},

		{"help bar", "帮助"},
	}

	for _, c := range checks {
		if !strings.Contains(view, c.content) {
			t.Errorf("View should contain %s (%q)", c.name, c.content)
		}
	}
}

// Detail bar 已移除：日常信息走 d 键详情面板，底部行只在搜索时出现

// newParsedTestApp creates an app with real regex parser for rendering tests
func newParsedTestApp() *App {
	p, _ := parser.NewRegexParser("java-logback",
		`(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}) \[(?P<thread>[^\]]+)\] \[(?P<traceId>[^\]]+)\] (?P<level>\w+)\s+(?P<logger>\S+) - (?P<message>.*)`)
	ad := parser.NewAutoDetect([]parser.Parser{p})
	app := NewApp(&mockStream{}, ad, 1000, nil)
	app.width = 120
	app.height = 40
	app.processLine(model.RawLine{
		Text:   "2026-05-15 09:27:01.130 [http-nio-80-exec-3] [abc123] INFO  com.example.App - test message here",
		Source: "test-pod",
	})
	return app
}

// 验证字段切换后渲染输出确实不同
func TestFieldToggleChangesRender(t *testing.T) {
	app := newParsedTestApp()

	// 默认 mask: time=true, level=true, thread=false, traceId=false, logger=false, message=true, source=true
	line := app.filteredView[0]
	defaultRender := stripANSI(app.renderLine(line, false, 0))

	// 检查默认渲染包含关键字段
	if !strings.Contains(defaultRender, "INFO") {
		t.Errorf("default render should contain level 'INFO', got: %q", defaultRender)
	}
	if !strings.Contains(defaultRender, "09:27:01.130") {
		t.Errorf("default render should contain time, got: %q", defaultRender)
	}
	if !strings.Contains(defaultRender, "test message here") {
		t.Errorf("default render should contain message, got: %q", defaultRender)
	}

	// 隐藏 level 字段
	app.fieldMask.Toggle(model.FieldLevel)
	noLevelRender := stripANSI(app.renderLine(line, false, 0))

	if strings.Contains(noLevelRender, "INFO") {
		t.Errorf("after hiding level, render should NOT contain 'INFO', got: %q", noLevelRender)
	}
	if noLevelRender == defaultRender {
		t.Errorf("render should change after toggling field, got same: %q", defaultRender)
	}

	// 只保留 message
	app.fieldMask[model.FieldTime] = false
	app.fieldMask[model.FieldLevel] = false
	app.fieldMask[model.FieldSource] = false
	app.fieldMask[model.FieldMessage] = true

	msgOnlyRender := stripANSI(app.renderLine(line, false, 0))
	if !strings.Contains(msgOnlyRender, "test message here") {
		t.Errorf("message-only render should contain message, got: %q", msgOnlyRender)
	}
	if strings.Contains(msgOnlyRender, "09:27:01") {
		t.Errorf("message-only render should NOT contain time, got: %q", msgOnlyRender)
	}
	if strings.Contains(msgOnlyRender, "INFO") {
		t.Errorf("message-only render should NOT contain level, got: %q", msgOnlyRender)
	}
}

// 验证无 parser 时 source 字段通过 Get() fallback 仍然显示
func TestFieldToggleNoParserSourceVisible(t *testing.T) {
	app := newTestApp() // nil parsers

	line := app.filteredView[0]
	render := stripANSI(app.renderLine(line, false, 0))

	// 即使没有 parser, source 通过 Raw.Source fallback 也应该显示
	if !strings.Contains(render, "test-pod") {
		t.Errorf("source 'test-pod' should show via Get() fallback, got: %q", render)
	}

	// 隐藏 source 后应该消失
	app.fieldMask[model.FieldSource] = false
	render2 := stripANSI(app.renderLine(line, false, 0))
	if strings.Contains(render2, "test-pod") {
		t.Errorf("source should be hidden after toggle, got: %q", render2)
	}
}

// 端到端测试：通过 View() 验证字段切换前后整体输出变化
func TestFieldToggleFullViewChange(t *testing.T) {
	app := newParsedTestApp()

	// 截取初始 View 中日志行区域（第3行开始是 logView）
	viewBefore := app.View()
	beforeLines := strings.Split(viewBefore, "\n")

	// 找到包含 "INFO" 的日志行
	var logLineBefore string
	for _, l := range beforeLines {
		clean := stripANSI(l)
		if strings.Contains(clean, "INFO") && strings.Contains(clean, "test message") {
			logLineBefore = clean
			break
		}
	}
	if logLineBefore == "" {
		t.Fatalf("could not find parsed log line in view. Full view:\n%s", viewBefore)
	}

	// 按 f 进入字段面板
	_, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
	// 按 C-j 到 level (index 1)
	_, _ = app.Update(fakeKey("ctrl+j"))
	// 按空格切换 level 为不可见
	_, _ = app.Update(tea.KeyMsg{Type: tea.KeySpace})
	// 按 Esc 退出面板
	_, _ = app.Update(tea.KeyMsg{Type: tea.KeyEscape})

	// 重新获取 View
	viewAfter := app.View()
	afterLines := strings.Split(viewAfter, "\n")

	var logLineAfter string
	for _, l := range afterLines {
		clean := stripANSI(l)
		if strings.Contains(clean, "test message") {
			logLineAfter = clean
			break
		}
	}
	if logLineAfter == "" {
		t.Fatalf("could not find log line after toggle. Full view:\n%s", viewAfter)
	}

	// 验证 INFO 不再出现在日志行中
	if strings.Contains(logLineAfter, "INFO") && !strings.Contains(logLineAfter, "test message here") {
		t.Errorf("level 'INFO' should be hidden after toggle.\nBefore: %q\nAfter: %q", logLineBefore, logLineAfter)
	}

	// 整体 View 应该不同
	if viewBefore == viewAfter {
		// 面板状态提示文字不同，View 不应该完全一样
		t.Errorf("View should change after field toggle")
	}
}

// 验证解析器名称显示在标题栏
func TestParserNameInTitle(t *testing.T) {
	app := newParsedTestApp()
	view := app.View()
	if !strings.Contains(view, "java-logback") {
		t.Errorf("title should show parser name 'java-logback', got: %s", view[:200])
	}
}

func stripANSI(s string) string {
	return model.StripANSI(s)
}

func TestPartialOperatorStrippedSearch(t *testing.T) {
	app := newTestApp() // 20 行 test message
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	// not → 剥离末尾 not → 空 → 显示全部 20
	for _, r := range "not" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if len(app.filteredView) != 20 {
		t.Errorf("not 应剥离为空显示全部 20，实际 %d", len(app.filteredView))
	}
	// 输完 timeout → "not timeout"（非中间态）
	for _, r := range " timeout" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if isPartialQuery(app.searchInput) {
		t.Errorf("not timeout 应非中间态")
	}
}

// d 键详情面板：完整字段 + 消息全文 + 原始行；Esc 关闭
func TestDetailPanel(t *testing.T) {
	app := newParsedTestApp()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if !app.detailMode {
		t.Fatal("d 应进入详情面板")
	}
	view := stripANSI(app.View())
	for _, want := range []string{"time", "level", "thread", "traceId", "logger", "message", "raw", "http-nio-80-exec-3"} {
		if !strings.Contains(view, want) {
			t.Errorf("详情面板应包含 %q", want)
		}
	}
	// C-j 联动移动光标不崩
	app.Update(fakeKey("ctrl+j"))
	// Esc 关闭
	app.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if app.detailMode {
		t.Fatal("Esc 应关闭详情面板")
	}
}

// 迭代 3:主日志界面移动键改 vim 风格——裸 j/k 移动,C-j/C-k 无绑定不再移动。
func TestMainViewVimJKMove(t *testing.T) {
	app := newTestApp()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if app.cursor != 1 {
		t.Fatalf("裸 j 应下移光标, cursor=%d", app.cursor)
	}
	app.Update(fakeKey("ctrl+j"))
	if app.cursor != 1 {
		t.Fatalf("C-j 在主日志界面应不再移动光标, cursor=%d", app.cursor)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if app.cursor != 0 {
		t.Fatalf("裸 k 应上移光标, cursor=%d", app.cursor)
	}
}

// TestSearchPopupHidesCurrentLineFields 打开搜索弹窗不得显示当前行日志信息
// (第九轮验收:starFields 自动填充退役,当前行字段值不进弹窗)。
func TestSearchPopupHidesCurrentLineFields(t *testing.T) {
	app := newTestApp()
	app.openUnifiedPopup(0)
	joined := strings.Join(app.buildSearchPopup(), "\n") // 只看弹窗本体(overlay 底层日志区不在此列)
	// 当前行(光标=首行)的字段值不得出现在弹窗里
	for _, secret := range []string{"http-nio-80-exec-3", "abc123", "com.example.App"} {
		if strings.Contains(joined, secret) {
			t.Fatalf("搜索弹窗泄露当前行字段值 %q:\n%s", secret, joined)
		}
	}
}

// keybind-unify-r2 A8：帮助弹窗任意键关闭（fview HelpModal 同款），按键不透传。
func TestHelpAnyKeyClose(t *testing.T) {
	app := newTestApp()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !app.helpMode {
		t.Fatal("按 ? 应打开帮助弹窗")
	}
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if app.helpMode {
		t.Fatal("任意键（含非关闭键 m）应关闭帮助弹窗")
	}
	if len(app.bookmarkSeq) != 0 {
		t.Fatal("帮助弹窗内按 m 不应透传成标记动作")
	}
}

// markSetup 灌 12 行唯一 Seq 日志（newTestApp 默认 20 行 Seq 全 0，标记/列表语义无法区分行）。
func markSetup(t *testing.T) *App {
	t.Helper()
	app := newTestApp()
	app.width = 200
	app.height = 50
	app.buffer.Clear()
	app.levelCounts = map[string]int{}
	app.filteredView = nil
	for i := 1; i <= 12; i++ {
		msg := fmt.Sprintf("L%02d", i)
		app.buffer.Push(&model.ParsedLine{
			Raw:    model.RawLine{Text: "2026-11-15 10:00:00.000 [t] INFO  c.x.Svc - " + msg, Source: "mark.log", Seq: uint64(i)},
			Fields: map[model.Field]string{model.FieldLevel: "INFO", model.FieldMessage: msg},
		})
	}
	app.recomputeView()
	app.cursor = 0
	return app
}

// keybind-unify-r2 A2：标记列表浮层——' 开关/C-jC-k 导航(不动日志光标)/Enter 跳转
// (列表保持开)/Esc 只关列表/m 删标记后 clamp，非模态对齐 fview 书签列表。
func TestMarkListOverlay(t *testing.T) {
	app := markSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}}) // 标记 L01 行
	for i := 0; i < 8; i++ {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}}) // 标记 L09 行(cursor=8)
	if len(app.bookmarks) != 2 {
		t.Fatalf("应有两处标记，实际 %d", len(app.bookmarks))
	}
	if rows := app.markListRows(); len(rows) != 2 || rows[0] != 0 || rows[1] != 8 {
		t.Fatalf("markListRows 应为 [0 8]，实际 %v", rows)
	}
	second := 8

	// ' 开列表：光标落第一项，浮层渲染 fview 同款头部与选中前缀
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'\''}})
	if !app.markListOpen || app.markIdx != 0 {
		t.Fatalf("' 应开列表且光标落首项 open=%v idx=%d", app.markListOpen, app.markIdx)
	}
	view := stripANSI(app.View())
	if !strings.Contains(view, "标记(Esc 关闭") || !strings.Contains(view, "Enter 跳转") {
		t.Fatalf("浮层应含 fview 同款头部提示:\n%s", view)
	}
	if !strings.Contains(view, "▶") {
		t.Fatal("浮层应有选中行 ▶ 前缀")
	}
	if len(app.markListRows()) != 2 {
		t.Fatalf("markListRows 应为 2，实际 %d", len(app.markListRows()))
	}

	// C-j：列表光标下移，日志光标不动
	cur := app.cursor
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	if app.markIdx != 1 || app.cursor != cur {
		t.Fatalf("C-j 应动列表不动日志光标 idx=%d cursor=%d(原 %d)", app.markIdx, app.cursor, cur)
	}

	// Enter：跳到第二个标记并居中（Final Review I4），列表关闭（对齐 fview
	// gotoBookmark 跳转即关列表——Final Declined#2 Ruling）
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.cursor != second {
		t.Fatalf("Enter 应跳到第二标记行，实际 cursor=%d(期望 %d)", app.cursor, second)
	}
	if app.markListOpen {
		t.Fatal("Enter 跳转后列表应关闭(对齐 fview)")
	}
	if app.scrollAnchor != 2 {
		t.Fatalf("Enter 跳转应居中锚定(scrollAnchor=2)，实际 %d", app.scrollAnchor)
	}

	// Esc（列表已关）：无副作用，光标保持
	app.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if app.cursor != second {
		t.Fatalf("列表已关 Esc 应无副作用 cursor=%d", app.cursor)
	}

	// 重开；删当前行标记 → 列表缩短 markIdx clamp 0
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'\''}})
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})                    // idx=1
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}}) // 取消当前行标记
	if len(app.markListRows()) != 1 || app.markIdx != 0 {
		t.Fatalf("删标记后列表应缩为1且 idx clamp，实际 rows=%d idx=%d", len(app.markListRows()), app.markIdx)
	}
}

// A2 边界：空标记也可开列表(空态提示)，C-k 不越界，Enter 无选中无动作。
func TestMarkListEmpty(t *testing.T) {
	app := newTestApp()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'\''}})
	if !app.markListOpen {
		t.Fatal("空标记按 ' 也应开列表")
	}
	if view := stripANSI(app.View()); !strings.Contains(view, "无标记") {
		t.Fatalf("空列表应有空态提示:\n%s", view)
	}
	cur := app.cursor
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlK}) // 越界保护
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 无选中无动作
	if app.cursor != cur || !app.markListOpen {
		t.Fatalf("空列表 C-k/Enter 应无副作用 cursor=%d open=%v", app.cursor, app.markListOpen)
	}
}

// A8 回归：原关闭键 Esc/q/Enter 仍关闭。
func TestHelpLegacyKeysClose(t *testing.T) {
	app := newTestApp()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if app.helpMode {
		t.Fatal("按 Esc 应关闭帮助弹窗")
	}
	app2 := newTestApp()
	app2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	app2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if app2.helpMode {
		t.Fatal("按 q 应关闭帮助弹窗")
	}
	app3 := newTestApp()
	app3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	app3.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app3.helpMode {
		t.Fatal("按 Enter 应关闭帮助弹窗")
	}
}
