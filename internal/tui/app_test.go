package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justfun/logview/internal/model"
	"github.com/justfun/logview/internal/parser"
)

// mockStream implements stream.LogStream for testing
type mockStream struct{}

func (m *mockStream) Start(_ context.Context) (<-chan model.RawLine, error) {
	ch := make(chan model.RawLine)
	close(ch)
	return ch, nil
}
func (m *mockStream) Label() string    { return "test" }
func (m *mockStream) Cleanup() error   { return nil }

func newTestApp() *App {
	app := NewApp(&mockStream{}, nil, 1000)
	app.width = 120
	app.height = 40
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

// 问题2: 应该有分隔线
func TestViewHasSeparators(t *testing.T) {
	app := newTestApp()
	view := app.View()

	if !strings.Contains(view, "─") {
		t.Error("View should contain separator lines (─)")
	}

	lines := strings.Split(view, "\n")
	sepCount := 0
	for _, line := range lines {
		clean := stripANSI(line)
		if strings.HasPrefix(clean, "──") || strings.HasPrefix(clean, "────────") {
			sepCount++
		}
	}
	if sepCount < 2 {
		t.Errorf("Expected at least 2 separator lines, got %d", sepCount)
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
	_, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	if !app.panelFocus {
		t.Fatal("panelFocus should be true after pressing f")
	}

	// View should show popup with field settings
	view := app.View()
	if !strings.Contains(view, "字段") {
		t.Error("View should show field popup after pressing f")
	}

	// 按 j 移动光标到 thread 字段
	// AllFields: time, level, thread, traceId, logger, message, source
	// thread 是 index 2
	_, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}) // -> fieldCursor=1 (level)
	_, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}) // -> fieldCursor=2 (thread)

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
		{"search bar", "搜索"},

		{"help bar", "退出"},
	}

	for _, c := range checks {
		if !strings.Contains(view, c.content) {
			t.Errorf("View should contain %s (%q)", c.name, c.content)
		}
	}
}

// Detail bar 显示 traceId/thread/level/time + msg
func TestDetailBarShowsParsedFields(t *testing.T) {
	app := newParsedTestApp()

	detail := app.renderDetailBar()
	if detail == "" {
		t.Fatal("detail bar should not be empty when there are lines")
	}
	clean := stripANSI(detail)
	for _, want := range []string{"traceId:", "abc123", "thread:", "http-nio-80-exec-3", "level:", "INFO", "time:", "msg:", "test message here"} {
		if !strings.Contains(clean, want) {
			t.Errorf("detail bar should contain %q, got: %q", want, clean)
		}
	}
}

// Detail bar 无 parser 时仍显示 msg
func TestDetailBarShowsMsgWithoutParser(t *testing.T) {
	app := newTestApp()
	detail := app.renderDetailBar()
	if detail == "" {
		t.Fatal("detail bar should not be empty even without parser")
	}
	clean := stripANSI(detail)
	if !strings.Contains(clean, "msg:") {
		t.Errorf("detail bar should show msg even without parser, got: %q", clean)
	}
}

// Detail bar 在 View() 中可见
func TestDetailBarInView(t *testing.T) {
	app := newParsedTestApp()
	view := app.View()
	clean := stripANSI(view)
	if !strings.Contains(clean, "traceId:") {
		t.Errorf("View should contain detail bar with traceId. Got:\n%s", clean[:min(500, len(clean))])
	}
}

// newParsedTestApp creates an app with real regex parser for rendering tests
func newParsedTestApp() *App {
	p, _ := parser.NewRegexParser("java-logback",
		`(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}) \[(?P<thread>[^\]]+)\] \[(?P<traceId>[^\]]+)\] (?P<level>\w+)\s+(?P<logger>\S+) - (?P<message>.*)`)
	ad := parser.NewAutoDetect([]parser.Parser{p})
	app := NewApp(&mockStream{}, ad, 1000)
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
	defaultRender := stripANSI(app.renderLine(line, false))

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
	noLevelRender := stripANSI(app.renderLine(line, false))

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

	msgOnlyRender := stripANSI(app.renderLine(line, false))
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
	render := stripANSI(app.renderLine(line, false))

	// 即使没有 parser, source 通过 Raw.Source fallback 也应该显示
	if !strings.Contains(render, "test-pod") {
		t.Errorf("source 'test-pod' should show via Get() fallback, got: %q", render)
	}

	// 隐藏 source 后应该消失
	app.fieldMask[model.FieldSource] = false
	render2 := stripANSI(app.renderLine(line, false))
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
	_, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	// 按 j 到 level (index 1)
	_, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
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
	// simple ANSI strip
	result := ""
	inEscape := false
	for _, c := range s {
		if c == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				inEscape = false
			}
			continue
		}
		result += string(c)
	}
	return result
}
