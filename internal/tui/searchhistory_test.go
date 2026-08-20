package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// fakeKey 构造 default-case 字符串键（如 "ctrl+r"）的 KeyMsg。
// bubbletea 里 ctrl+r 是 KeyCtrlR，其 String() 返回 "ctrl+r"，匹配 handleSearchKeys 的 default 分支。
func fakeKey(s string) tea.KeyMsg {
	switch s {
	case "ctrl+r":
		return tea.KeyMsg{Type: tea.KeyCtrlR}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// 进搜索 + 确认一次搜索产生历史后，ctrl+r 应打开历史列表。
func TestSearchHistPopupOpens(t *testing.T) {
	app := newTestApp()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}) // 进搜索
	for _, r := range "ERROR" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})                     // 确认 → 历史记 "ERROR" + 关弹窗
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}) // 再进搜索
	app.Update(fakeKey("ctrl+r"))                                  // 打开历史列表
	if !app.searchHistMode {
		t.Fatalf("ctrl+r 后 searchHistMode 应为 true，实际 false")
	}
	if app.searchHistCursor != 0 {
		t.Fatalf("searchHistCursor 应为 0（最新），实际 %d", app.searchHistCursor)
	}
}

// 空历史时 ctrl+r 不打开列表。
func TestSearchHistPopupEmpty(t *testing.T) {
	app := newTestApp()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}) // 进搜索，无历史
	app.Update(fakeKey("ctrl+r"))
	if app.searchHistMode {
		t.Fatalf("空历史时 searchHistMode 应保持 false")
	}
}

// 列表展开时，搜索弹窗应渲染出历史词条与选中标记、改提示行。
func TestSearchHistPopupRender(t *testing.T) {
	app := newTestApp()
	app.searchHistory = []string{"ERROR", "WARN"} // append 顺序，WARN 最新
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}) // 进搜索
	app.Update(fakeKey("ctrl+r"))                                  // 打开列表

	view := app.View()
	if !strings.Contains(view, "历史") {
		t.Fatalf("应渲染“历史”标题，view=\n%s", view)
	}
	// 倒序：WARN（最新）在 ERROR 之上
	warnPos := strings.Index(view, "WARN")
	errPos := strings.Index(view, "ERROR")
	if warnPos < 0 || errPos < 0 || warnPos > errPos {
		t.Fatalf("最新历史 WARN 应在 ERROR 之上，warnPos=%d errPos=%d", warnPos, errPos)
	}
	if !strings.Contains(view, "j/k选择") {
		t.Fatalf("列表展开时提示行应为 j/k选择，view=\n%s", view)
	}
}

// 列表打开后 Esc 关闭。
func TestSearchHistPopupEscCloses(t *testing.T) {
	app := newTestApp()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "ERROR" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 记历史
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	app.Update(fakeKey("ctrl+r")) // 打开
	app.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if app.searchHistMode {
		t.Fatalf("Esc 后 searchHistMode 应为 false")
	}
}

// 历史 ["ERROR","WARN","INFO"]，倒序显示 WARN/INFO/ERROR... 展开后 cursor=0（最新=最后append的）。
// j/↓ 往下（更旧），k/↑ 往上（更新），夹紧。
func TestSearchHistPopupNavigate(t *testing.T) {
	app := newTestApp()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, q := range []string{"ERROR", "WARN", "INFO"} {
		for _, r := range q {
			app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
		app.Update(tea.KeyMsg{Type: tea.KeyEnter})
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	}
	app.Update(fakeKey("ctrl+r")) // 打开，cursor=0
	n := len(app.searchHistory)   // 3

	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}) // 往下
	if app.searchHistCursor != 1 {
		t.Fatalf("j 后 cursor 应=1，实际 %d", app.searchHistCursor)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyDown}) // ↓ 往下
	if app.searchHistCursor != 2 {
		t.Fatalf("↓ 后 cursor 应=2，实际 %d", app.searchHistCursor)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyDown}) // 再 ↓ 夹紧
	if app.searchHistCursor != 2 {
		t.Fatalf("到底应夹紧 2，实际 %d", app.searchHistCursor)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyUp}) // ↑ 往上
	if app.searchHistCursor != 1 {
		t.Fatalf("↑ 后 cursor 应=1，实际 %d", app.searchHistCursor)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}) // k 往上
	if app.searchHistCursor != 0 {
		t.Fatalf("k 后 cursor 应=0，实际 %d", app.searchHistCursor)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}) // 到顶夹紧
	if app.searchHistCursor != 0 {
		t.Fatalf("到顶应夹紧 0，实际 %d", app.searchHistCursor)
	}
	_ = n
}

// Enter 选中：填入 searchInput、关闭列表、按该词过滤。
func TestSearchHistPopupEnterFills(t *testing.T) {
	app := newTestApp() // 20 行 "test message"
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "test" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 历史 ["test"]，过滤到全部含 test
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	// 进入搜索不会清空 searchInput（app.go 进搜索分支仅重置 cursor），
	// 这里显式清空，确保后续 Enter 必须真正从历史“填入”，否则 RED。
	app.searchInput = ""
	app.Update(fakeKey("ctrl+r"))              // 打开，cursor=0 → 选中 "test"
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 选中填入

	if app.searchHistMode {
		t.Fatalf("Enter 后应关闭列表")
	}
	if app.searchInput != "test" {
		t.Fatalf("应填入 test，实际 %q", app.searchInput)
	}
	if len(app.filteredView) != 20 {
		t.Fatalf("test 应匹配全部 20 行（实时过滤），实际 %d", len(app.filteredView))
	}
}

// 列表内 ctrl+r 无操作（不关列表，且不重置光标位置）。
func TestSearchHistPopupCtrlRNoop(t *testing.T) {
	app := newTestApp()
	app.searchHistory = []string{"ERROR", "WARN"} // n=2，使 KeyDown 能把 cursor 移到 1
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	app.Update(fakeKey("ctrl+r"))                     // 打开 → cursor=0
	app.Update(tea.KeyMsg{Type: tea.KeyDown})         // cursor=1
	app.Update(fakeKey("ctrl+r"))                     // 列表内再 ctrl+r：应无操作
	if !app.searchHistMode {
		t.Fatalf("列表内 ctrl+r 应保持展开")
	}
	if app.searchHistCursor != 1 {
		t.Fatalf("列表内 ctrl+r 不应重置光标，应保留 1，实际 %d", app.searchHistCursor)
	}
}

// 列表内按其他字符：关列表，字符进入搜索框。
func TestSearchHistPopupCharClosesAndTypes(t *testing.T) {
	app := newTestApp()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "ERROR" {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	// 进入搜索不会清空 searchInput（app.go 进搜索分支仅重置 cursor），
	// 这里显式清空，确保 'x' 是续输产生的唯一输入（与 EnterFills 测试一致）。
	app.searchInput = ""
	app.Update(fakeKey("ctrl+r")) // 打开
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}) // 续输字符
	if app.searchHistMode {
		t.Fatalf("字符键应关闭列表")
	}
	if app.searchInput != "x" {
		t.Fatalf("字符应进入搜索框，实际 %q", app.searchInput)
	}
}

// 无历史时若意外进入列表模式，按键不应 panic，且列表应关闭。
func TestSearchHistPopupEmptyNoPanic(t *testing.T) {
	app := newTestApp()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}) // 进搜索
	app.searchHistMode = true                                      // 绕过 ctrl+r 守门，强制进入列表模式
	app.searchHistory = nil                                        // 空历史
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})                     // 之前会 panic，现在应被 guard 拦截
	if app.searchHistMode {
		t.Fatalf("空历史时 guard 应关闭列表")
	}
}
