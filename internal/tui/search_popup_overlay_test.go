package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justfun/logview/internal/model"
)

// 回归保护：搜索 popup 曾从日志区顶部逐行 overlay，当 popup 行数 >= 匹配结果
// 行数时把日志全盖成空白（用户看到 popup 下面一片空）。修复后 popup 垂直居中，
// 上下日志保留可见。此测试锁死"搜索时日志区必须能看见匹配行"。
func TestSearchPopupDoesNotBlankLog(t *testing.T) {
	app := NewApp(&mockStream{}, nil, 1000, nil)
	app.width = 120
	app.height = 40
	// 只喂 5 行 ERROR：匹配结果(5) < popup 高度(~14)，最易触发空白
	for i := 0; i < 5; i++ {
		app.processLine(model.RawLine{Text: "2026-05-15 09:27:01.130 [t] [abc] ERROR App - boom failure", Source: "p"})
	}

	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}) // 进搜索
	for _, r := range "ERROR" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if !app.searchMode {
		t.Fatalf("应处于搜索模式")
	}
	if len(app.filteredView) != 5 {
		t.Fatalf("ERROR 应匹配 5 行，实际 %d", len(app.filteredView))
	}

	// View() 日志区（跳过 title/sep/bar/sep 共 4 行）必须至少一行含原始日志内容
	view := app.View()
	lines := strings.Split(view, "\n")
	vl := app.visibleLines()
	hasLog := false
	for i := 4; i < 4+vl && i < len(lines); i++ {
		if strings.Contains(stripAnsi(lines[i]), "boom") {
			hasLog = true
			break
		}
	}
	if !hasLog {
		t.Fatalf("搜索 popup 不应把日志盖成空白：日志区 %d 行找不到匹配日志", vl)
	}
}
