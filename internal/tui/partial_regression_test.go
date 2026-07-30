package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justfun/logview/internal/model"
)

// 回归保护：v0.12.11 曾因 recomputeView 在 partial query 时 early-return（freeze）
// 导致输入 not 时视图卡在 "no" 的匹配数。v0.12.12 删掉 freeze 改用 strippedQuery。
// 这些测试锁死"输入/删除操作符后视图必须正确重算"，防止 freeze 复活。
func TestPartialQueryNoFreezeOnDelete(t *testing.T) {
	typeR := func(app *App, r rune) { app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}) }
	typeS := func(app *App, s string) { for _, r := range s { typeR(app, r) } }
	bs := func(app *App) { app.Update(tea.KeyMsg{Type: tea.KeyBackspace}) }
	enterSearch := func(app *App) { app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}) }

	const total = 20
	newApp := func() *App {
		app := NewApp(&mockStream{}, nil, 1000, nil)
		app.width = 120
		app.height = 40
		for i := 0; i < 11; i++ {
			app.processLine(model.RawLine{Text: "2026-05-15 09:27:01.130 [t] [abc] ERROR App - boom failure", Source: "p"})
		}
		for i := 0; i < 9; i++ {
			app.processLine(model.RawLine{Text: "2026-05-15 09:27:01.130 [t] [abc] INFO  App - all good", Source: "p"})
		}
		return app
	}

	// 空 → 输 not（partial，应剥离为空 → 全部）→ 删空 → 仍全部
	t.Run("empty_not_delete", func(t *testing.T) {
		app := newApp()
		enterSearch(app)
		typeS(app, "not")
		if len(app.filteredView) != total {
			t.Fatalf("输入 not 应显示全部 %d，实际 %d（freeze 复活？）", total, len(app.filteredView))
		}
		for i := 0; i < 3; i++ {
			bs(app)
		}
		if len(app.filteredView) != total {
			t.Fatalf("删除 not 后应恢复全部 %d，实际 %d", total, len(app.filteredView))
		}
	})

	// ERROR(11) → "ERROR not"(partial，剥离为 ERROR → 11) → 删回 ERROR → 仍 11，不能卡 0
	t.Run("prefix_not_delete", func(t *testing.T) {
		app := newApp()
		enterSearch(app)
		typeS(app, "ERROR")
		if len(app.filteredView) != 11 {
			t.Fatalf("ERROR 应匹配 11，实际 %d", len(app.filteredView))
		}
		typeS(app, " not") // partial：strippedQuery → "ERROR"
		if len(app.filteredView) != 11 {
			t.Fatalf("ERROR not 应剥离为 ERROR 匹配 11，实际 %d", len(app.filteredView))
		}
		for i := 0; i < 4; i++ { // 删 " not" 回到 "ERROR"
			bs(app)
		}
		if app.searchInput != "ERROR" {
			t.Fatalf("应删回 ERROR，实际 %q", app.searchInput)
		}
		if len(app.filteredView) != 11 {
			t.Fatalf("删回 ERROR 应为 11，实际 %d", len(app.filteredView))
		}
	})

	// not → ctrl+u 清空 → 全部
	t.Run("not_ctrlu", func(t *testing.T) {
		app := newApp()
		enterSearch(app)
		typeS(app, "not")
		app.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
		if app.searchInput != "" || len(app.filteredView) != total {
			t.Fatalf("ctrl+u 后应清空并显示全部，input=%q view=%d", app.searchInput, len(app.filteredView))
		}
	})
}
