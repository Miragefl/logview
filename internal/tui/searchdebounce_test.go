package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// withDebounce 注入短防抖间隔(测试可控),恢复由 t.Cleanup 保证。
func withDebounce(t *testing.T, d time.Duration) {
	t.Helper()
	old := searchDebounce
	searchDebounce = d
	t.Cleanup(func() { searchDebounce = old })
}

// openSearch 打开搜索弹窗(tab 0)并全量重算基线。
func openSearch(app *App) {
	app.searchMode = true
	app.searchTab = 0
	app.searchInput = ""
	app.recomputeView()
}

// 输入变更不立即过滤;防抖 tick 到达后过滤生效。
func TestSearchDebounceDeferred(t *testing.T) {
	withDebounce(t, 20*time.Millisecond)
	app := newTestApp()
	openSearch(app)
	base := len(app.filteredView)

	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}) // q:测试数据无命中
	if cmd == nil {
		t.Fatal("输入变更应返回防抖 tick cmd")
	}
	if !app.searchPending {
		t.Fatal("pending 应置位")
	}
	if len(app.filteredView) != base {
		t.Fatal("防抖期内不应立即过滤")
	}
	app.Update(cmd()) // 推进 tick
	if app.searchPending {
		t.Fatal("tick 后 pending 应清除")
	}
	if len(app.filteredView) != 0 {
		t.Fatalf("tick 后应过滤为 0 行, got %d", len(app.filteredView))
	}
}

// 连续两键:旧令牌 tick 作废,新令牌生效(只认最后一次)。
func TestSearchDebounceTokenInvalidation(t *testing.T) {
	withDebounce(t, 20*time.Millisecond)
	app := newTestApp()
	openSearch(app)

	_, cmd1 := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	_, cmd2 := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	app.Update(cmd1()) // 旧令牌到达
	if !app.searchPending {
		t.Fatal("旧令牌应被作废,pending 保持")
	}
	if len(app.filteredView) != 20 {
		t.Fatalf("旧令牌不应触发过滤, got %d", len(app.filteredView))
	}
	app.Update(cmd2()) // 新令牌到达
	if app.searchPending || len(app.filteredView) != 0 {
		t.Fatalf("新令牌应生效: pending=%v filtered=%d", app.searchPending, len(app.filteredView))
	}
}

// 空格立即过滤(词边界),不安排 tick。
// 注:纯空格查询被 parseSearchQuery TrimSpace 为空(root=nil)恒匹配全部行,
// 无从观察过滤效果——故先敲 q 进入防抖期再敲空格,查询 "q" 零命中即可见。
func TestSearchDebounceSpaceImmediate(t *testing.T) {
	withDebounce(t, 20*time.Millisecond)
	app := newTestApp()
	openSearch(app)

	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}) // 先进入防抖期
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeySpace})
	if app.searchPending {
		t.Fatal("空格应立即过滤,pending 清除")
	}
	if len(app.filteredView) != 0 {
		t.Fatalf("空格立即过滤应生效(查询 q 零命中), got %d", len(app.filteredView))
	}
	if cmd != nil {
		t.Fatal("空格路径不应安排防抖 tick")
	}
}

// Esc 关闭前 flush 应用当前输入(与现状一致,不丢输入)。
func TestSearchDebounceEscFlushes(t *testing.T) {
	withDebounce(t, 20*time.Millisecond)
	app := newTestApp()
	openSearch(app)

	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}) // pending
	app.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if app.searchPending {
		t.Fatal("Esc 应 flush pending")
	}
	if len(app.filteredView) != 0 {
		t.Fatal("Esc flush 后过滤应生效")
	}
}

// Tab 切分区取消 pending(不触发)。
func TestSearchDebounceTabCancels(t *testing.T) {
	withDebounce(t, 20*time.Millisecond)
	app := newTestApp()
	openSearch(app)

	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	app.Update(tea.KeyMsg{Type: tea.KeyTab}) // 切到高亮 tab
	if app.searchPending {
		t.Fatal("Tab 切分区应取消 pending")
	}
	app.Update(cmd()) // 在途 tick 到达
	if app.searchTab != 1 || len(app.filteredView) != 20 {
		t.Fatalf("在途 tick 应被丢弃: tab=%d filtered=%d", app.searchTab, len(app.filteredView))
	}
}
