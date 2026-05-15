package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justfun/logview/internal/model"
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

// 问题3补充: 搜索栏应该显示明显
func TestSearchBarVisible(t *testing.T) {
	app := newTestApp()
	view := app.View()
	if !strings.Contains(view, "搜索") {
		t.Error("View should contain search bar with '搜索' text")
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
	if app.activePanel != 0 {
		t.Fatalf("activePanel = %d, want 0 (fields)", app.activePanel)
	}

	// View 应该显示字段面板，带有操作提示
	view := app.View()
	if !strings.Contains(view, "字段") {
		t.Error("View should show field panel after pressing f")
	}
	if !strings.Contains(view, "选择") || !strings.Contains(view, "切换") {
		t.Errorf("Field panel should show interaction hints, got: %s", view)
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

	// 应该包含这些元素
	checks := []struct {
		name    string
		content string
	}{
		{"title", "LogView"},
		{"separator", "─"},
		{"search bar", "搜索"},
		{"panel", "字段"},
		{"help bar", "退出"},
	}

	for _, c := range checks {
		if !strings.Contains(view, c.content) {
			t.Errorf("View should contain %s (%q)", c.name, c.content)
		}
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
