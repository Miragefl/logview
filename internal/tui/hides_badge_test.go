package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justfun/logview/internal/model"
)

// 防回归：hides 激活时必须 (1) 正确统计被隐藏的行数 (2) 在搜索模式下也显示醒目 badge。
// 这是 v0.12.12 "输入not删not只剩11行" 误会的根因——用户在搜索弹窗误按 Tab 切到 hide
// tab 输词回车，hides 激活却提示不明显。修复后搜索模式也显示 [隐藏:xxx 藏N行]。
func TestHidesBadgeInSearchMode(t *testing.T) {
	app := newTestApp()          // 20 行 "test message"
	app.hides = []string{"test"} // 每行都含 test → 全部隐藏
	app.recomputeView()

	if app.hiddenByHides != 20 {
		t.Fatalf("hiddenByHides 应为 20，实际 %d", app.hiddenByHides)
	}
	if len(app.filteredView) != 0 {
		t.Fatalf("hides=test 应隐藏全部，filteredView 应为 0，实际 %d", len(app.filteredView))
	}

	// 进搜索模式
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	items := app.helpItems()
	if !containsItem(items, "隐藏") {
		t.Fatalf("搜索模式下应显示 hides badge，items=%v", itemDescs(items))
	}
	if !containsItem(items, "藏") {
		t.Fatalf("hides badge 应含隐藏行数提示，items=%v", itemDescs(items))
	}
}

// 流式追加期间（不触发 recomputeView）hiddenByHides 也必须正确累加，
// 否则从 session 恢复 hides 后持续 tail 日志时 badge 会显示"藏0行"。
func TestHidesCountedDuringStream(t *testing.T) {
	app := NewApp(&mockStream{}, nil, 1000, []string{"hide-me"})
	app.width = 120
	app.height = 40
	for i := 0; i < 5; i++ {
		app.processLine(model.RawLine{Text: "2026 hide-me line", Source: "p"})
	}
	for i := 0; i < 3; i++ {
		app.processLine(model.RawLine{Text: "2026 clean line", Source: "p"})
	}
	if app.hiddenByHides != 5 {
		t.Fatalf("流式期间 hiddenByHides 应为 5，实际 %d", app.hiddenByHides)
	}
	if len(app.filteredView) != 3 {
		t.Fatalf("filteredView 应为 3（clean 行），实际 %d", len(app.filteredView))
	}
}

func containsItem(items []helpItem, sub string) bool {
	for _, it := range items {
		if strings.Contains(it.desc, sub) {
			return true
		}
	}
	return false
}

func itemDescs(items []helpItem) []string {
	var out []string
	for _, it := range items {
		out = append(out, it.desc)
	}
	return out
}
