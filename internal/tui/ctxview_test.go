package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justfun/logview/internal/model"
)

// ctxSetup 灌 12 行(L01..L12,message 含序号),过滤只留偶数行(6 行)。
func ctxSetup(t *testing.T) *App {
	t.Helper()
	app := newTestApp()
	app.buffer.Clear()
	app.levelCounts = map[string]int{}
	app.filteredView = nil
	for i := 1; i <= 12; i++ {
		app.processLine(model.RawLine{
			Text:   "2026-09-13 10:00:00.000 [t] INFO  c.x.Svc - L" + string(rune('0'+i)),
			Source: "ctx.log",
		})
	}
	app.recomputeView()
	app.searchInput = "L2 L4 L6 L8" // 占位,真实过滤用下面(hasActiveFilter 依据)
	// 直接构造过滤视图:偶数行(用 search 语法不可表达奇偶,手动过滤)
	var view []*model.ParsedLine
	for i := 0; i < app.buffer.Len(); i++ {
		pl := app.buffer.Get(i)
		if i%2 == 1 { // L02/L04/.../L12
			view = append(view, pl)
		}
	}
	app.filteredView = view
	app.cursor = 1 // L04
	return app
}

// +3:当前行(L04)+ 原始上 3 行(L01..L03),时间序,anchor=L04。
func TestCtxViewPlus(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(app.ctxLines) != 4 {
		t.Fatalf("+3 应混入 4 行(上3+当前), got %d", len(app.ctxLines))
	}
	if app.ctxLines[0].pl != app.buffer.Get(0) || app.ctxLines[3].pl != app.buffer.Get(3) {
		t.Fatal("混入行应为原始 buffer 指针 L01..L04")
	}
	if !app.ctxLines[3].anchor {
		t.Fatal("最后一行应为 anchor(触发行)")
	}
	// dim 语义:anchor 行不暗;上文行暗 —— ctxDim(idx) 以 viewLines 索引为准
	if app.ctxDim(3) {
		t.Fatal("anchor 行不应 dim(此处 anchor 在 idx 3)")
	}
	if !app.ctxDim(0) {
		t.Fatal("非 anchor 上文行应 dim(此处 idx 0)")
	}
	// 恢复:Esc
	app.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if len(app.ctxLines) != 0 || app.cursor != 1 {
		t.Fatalf("Esc 应恢复纯过滤视图与锚点光标, lines=%d cursor=%d", len(app.ctxLines), app.cursor)
	}
}

// -2:当前行(L04)+ 下 2 行(L05/L06),anchor 在首位。
func TestCtxViewMinus(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(app.ctxLines) != 3 || !app.ctxLines[0].anchor {
		t.Fatalf("-2 应 3 行且 anchor 在首位, got %d 行 anchor0=%v", len(app.ctxLines), app.ctxLines[0].anchor)
	}
}

// Enter 空数字默认 5(边界 clamp 到 buffer 头)。
func TestCtxViewDefaultAndClamp(t *testing.T) {
	app := ctxSetup(t)
	app.cursor = 0 // L02
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(app.ctxLines) != 2 { // idx=1,上取 clamp 到 0:[0,1] 两行
		t.Fatalf("+默认应 clamp 到 2 行, got %d", len(app.ctxLines))
	}
}

// 输入态 Esc 取消;数字外字符忽略。
func TestCtxViewInputCancel(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}) // 非数字忽略
	if app.ctxInput != "+" {
		t.Fatalf("非数字应忽略, ctxInput=%q", app.ctxInput)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if app.ctxInput != "" || len(app.ctxLines) != 0 {
		t.Fatal("输入态 Esc 应取消")
	}
}

// 无过滤时 +/- 无操作;混入态导航键恢复。
func TestCtxViewNoFilterAndNavExit(t *testing.T) {
	app := newTestApp()
	app.searchInput = ""
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.ctxInput != "" || len(app.ctxLines) != 0 {
		t.Fatal("无过滤时 + 应无操作")
	}

	app2 := ctxSetup(t)
	app2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	app2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app2.Update(fakeKey("ctrl+j")) // 移动键 → 恢复且不移动
	if len(app2.ctxLines) != 0 || app2.cursor != 1 {
		t.Fatalf("移动键应恢复且光标回锚, lines=%d cursor=%d", len(app2.ctxLines), app2.cursor)
	}
}

// 回归:混入态禁折叠——stGroups 是 filteredView 坐标,混入视图按快照索引查表会错位,
// 曾把快照行误渲染成 (N lines) 占位并吞行;混入态必须逐行显示原始快照。
func TestCtxViewMixedNoFold(t *testing.T) {
	app := newTestApp()
	app.buffer.Clear()
	app.levelCounts = map[string]int{}
	app.filteredView = nil
	app.stGroups = nil
	for _, txt := range []string{
		"java.lang.RuntimeException: com.foo boom", // 组首(含 Exception,自身不折叠)
		"\tat com.foo.Bar.run(Bar.java:10)",
		"\tat com.foo.Baz.call(Baz.java:20)",
		"\tat com.foo.Qux.work(Qux.java:30)",
		"INFO all done, nothing here",
	} {
		app.processLine(model.RawLine{Text: txt, Source: "st.log"})
	}
	app.searchInput = "com.foo" // 过滤留组首+3 帧行(INFO 行不含 com.foo)
	app.recomputeView()
	if len(app.filteredView) != 4 || len(app.stGroups) != 1 {
		t.Fatalf("前置:过滤视图应 4 行且检出 1 个堆栈组, view=%d groups=%d",
			len(app.filteredView), len(app.stGroups))
	}
	if app.foldedGroup(1) == nil {
		t.Fatal("前置:非混入态组内帧行应可折叠")
	}

	app.cursor = 2 // filteredView[2] = at com.foo.Baz(buffer idx 2)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(app.ctxLines) != 3 { // buffer[0..2]:组首+Bar+Baz,+5 clamp 到头
		t.Fatalf("+5 应混入 3 行, got %d", len(app.ctxLines))
	}
	if app.foldedGroup(1) != nil {
		t.Fatal("混入态应禁折叠查表")
	}
	view := stripANSI(app.View())
	if strings.Contains(view, "lines) [e") {
		t.Fatal("混入态渲染不得出现折叠占位 (N lines)")
	}
	// 快照 3 行(组首/Bar/Baz)逐行可见;Qux 不在快照内
	for _, want := range []string{"boom", "Bar.run", "Baz.call"} {
		if !strings.Contains(view, want) {
			t.Fatalf("混入快照行 %q 应逐行可见", want)
		}
	}
	if strings.Contains(view, "Qux.work") {
		t.Fatal("快照外的行不应出现")
	}
}
