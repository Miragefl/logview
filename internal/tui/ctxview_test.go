package tui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/justfun/logview/internal/model"
	"github.com/muesli/termenv"
)

// ctxSetup:12 行 buffer(L01..L12,奇数行 ERROR/偶数行 INFO),真实过滤(levelFilter=ERROR)
// 得过滤视图 6 行(L01/L03/.../L11),cursor 落 L05(过滤视图 idx=2,buffer idx=4)。
// 用真实过滤而非手动视图:插入式混入靠 filterHit 判命中,setup 必须让 filterHit 语义真实。
func ctxSetup(t *testing.T) *App {
	t.Helper()
	app := newTestApp()
	app.width = 200
	app.height = 50
	app.buffer.Clear()
	app.levelCounts = map[string]int{}
	app.filteredView = nil
	for i := 1; i <= 12; i++ {
		msg := fmt.Sprintf("L%02d", i)
		lv := "INFO"
		if i%2 == 1 {
			lv = "ERROR" // 奇数行命中
		}
		app.buffer.Push(&model.ParsedLine{
			Raw:     model.RawLine{Text: "2026-09-14 10:00:00.000 [t] " + lv + "  c.x.Svc - " + msg, Source: "ctx.log"},
			Level:   lv,
			Message: msg,
		})
	}
	app.levelFilter = "ERROR"
	app.recomputeView() // 真实过滤:L01/L03/.../L11 共 6 行
	app.cursor = 2      // L05
	return app
}

// +3:过滤列表 6 行保留,触发行(L05)上方插入 buffer[L02,L04](未命中行,L03 命中不插)。
func TestCtxViewInsertPlus(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})

	vl := app.viewLines()
	if len(vl) != 8 { // 6 过滤行 + 2 插入行(L02/L04)
		t.Fatalf("+3 混合列表应 8 行(6+2), got %d", len(vl))
	}
	// 顺序:L01,L02(dim),L03,L04(dim),L05(anchor),L07,L09,L11
	wantDim := []bool{false, true, false, true, false, false, false, false}
	for i, e := range app.ctxLines {
		if e.dim != wantDim[i] {
			t.Fatalf("ctxLines[%d].dim=%v, want %v", i, e.dim, wantDim[i])
		}
	}
	if app.cursor != 4 { // anchor(L05)在混合列表的新位置
		t.Fatalf("anchor 光标应落 idx4, got %d", app.cursor)
	}
	// View 级:插入行与列表远端行同时可见(替换式回归网——曾整视图被换成小快照)
	out := app.View()
	for _, probe := range []string{"L02", "L04", "L05", "L11"} {
		if !strings.Contains(out, probe) {
			t.Fatalf("View 应含 %s(插入行+列表远端行共存)", probe)
		}
	}
	// 恢复
	app.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if len(app.ctxLines) != 0 || app.cursor != 2 || len(app.viewLines()) != 6 {
		t.Fatal("Esc 应恢复纯过滤视图与锚点")
	}
}

// -2:触发行(L05)下方插入 (idx, idx+2] = L06,L07;L07 命中(奇数)不插 → 只插 L06。
func TestCtxViewInsertMinus(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if len(app.ctxLines) != 7 { // 6 + L06
		t.Fatalf("-2 混合列表应 7 行, got %d", len(app.ctxLines))
	}
	// 顺序:L01,L03,L05(anchor),L06(dim),L07,L09,L11
	if !app.ctxLines[3].dim || app.ctxLines[3].pl != app.buffer.Get(5) {
		t.Fatal("idx3 应为插入行 L06(dim)")
	}
	// View 级:插入行与列表远端行同时可见
	out := app.View()
	for _, probe := range []string{"L06", "L01", "L11"} {
		if !strings.Contains(out, probe) {
			t.Fatalf("View 应含 %s(插入行+列表远端行共存)", probe)
		}
	}
}

// 回归:-x 窗口尾部未命中行必须按时间序插入(首个越过窗口的命中行之前),
// 不得甩到混合列表末尾(-3 曾把 L08 追加到 L11 之后)。
func TestCtxViewMinusTailOrder(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// 窗口 (4,7]:L06/L08 未命中插入 dim;L08 必须落在 L07 与 L09 之间
	want := []string{"L01", "L03", "L05", "L06d", "L07", "L08d", "L09", "L11"}
	var got []string
	for _, e := range app.ctxLines {
		m := e.pl.Message
		if e.dim {
			m += "d"
		}
		got = append(got, m)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("-3 混合顺序 = %v, want %v", got, want)
	}
}

// 回归:默认 - (n=5) 窗口 (4,9] 内未命中 L06/L08/L10 全部按时间序插入
// (L10 落在 L09 与 L11 之间,曾甩尾到 L11 之后)。
// 注:终审 brief 给的 want 漏了 L10d——手工推演 buffer idx9=L10 属窗口内 INFO,
// 按 spec((idx, idx+n] 全量未命中行)应插入 dim,want 已修正为含 L10d。
func TestCtxViewMinusDefaultOrder(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	want := []string{"L01", "L03", "L05", "L06d", "L07", "L08d", "L09", "L10d", "L11"}
	var got []string
	for _, e := range app.ctxLines {
		m := e.pl.Message
		if e.dim {
			m += "d"
		}
		got = append(got, m)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("- 默认混合顺序 = %v, want %v", got, want)
	}
}

// 回归:wrap 模式(w)下插入行同样暗色——buildWrapLines 非 cursor 路径补 ctxDim 分支。
// 探针策略:默认主题未加载时 DetailDimStyle 与 TimeStyle/HelpStyle 同为色 243,且
// 详情面板也用 DetailDimStyle——按行定位断言:临时把全局 DetailDimStyle 换成独占色
// (用毕还原),TrueColor profile 强制开启(无 TTY 测试环境会剥色),断言 L06 所在
// 渲染行带独占 SGR。
func TestCtxViewWrapDim(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prevProfile)
	prevStyle := DetailDimStyle
	DetailDimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ABCDEF"))
	defer func() { DetailDimStyle = prevStyle }()

	app := ctxSetup(t)
	app.wrapMode = true
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})

	var dimRow string
	for _, row := range strings.Split(app.View(), "\n") {
		if strings.Contains(stripANSI(row), "L06") {
			dimRow = row
			break
		}
	}
	if dimRow == "" {
		t.Fatal("wrap 模式 View 应渲染插入行 L06")
	}
	seq := strings.Split(DetailDimStyle.Render("PROBE"), "PROBE")[0]
	if seq == "" {
		t.Fatal("前置:TrueColor 下 DetailDimStyle 应产生 SGR 序列")
	}
	if !strings.Contains(dimRow, seq) {
		t.Fatalf("wrap 模式插入行 L06 应带 DetailDimStyle SGR %q, row=%q", seq, dimRow)
	}
}

// 边界 clamp:cursor 在列表首行(L01,buffer idx0)+默认5 → 上侧无行可插,视图=纯列表。
func TestCtxViewInsertClampTop(t *testing.T) {
	app := ctxSetup(t)
	app.cursor = 0 // L01
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(app.ctxLines) != 6 {
		t.Fatalf("顶部 +默认应无插入行, got %d", len(app.ctxLines))
	}
	for i, e := range app.ctxLines {
		if e.dim {
			t.Fatalf("顶部 clamp 后不应有 dim 行, ctxLines[%d].dim=%v", i, e.dim)
		}
	}
}

// 移动键恢复回锚;无过滤 no-op。
func TestCtxViewNavExitAndNoFilter(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app.Update(fakeKey("ctrl+j"))
	if len(app.ctxLines) != 0 || app.cursor != 2 {
		t.Fatal("移动键应恢复且回锚")
	}

	app2 := newTestApp()
	app2.searchInput = ""
	app2.levelFilter = ""
	app2.hides = nil
	app2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app2.ctxInput != "" || len(app2.ctxLines) != 0 {
		t.Fatal("无过滤时 + 应无操作")
	}
}

// 输入态取消/非数字忽略。
func TestCtxViewInputCancel(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if app.ctxInput != "+" {
		t.Fatalf("非数字应忽略, ctxInput=%q", app.ctxInput)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if app.ctxInput != "" || len(app.ctxLines) != 0 {
		t.Fatal("输入态 Esc 应取消")
	}
}

// 回归:混入态禁折叠——stGroups 是 filteredView 坐标,混入视图按混合列表索引查表会错位,
// 曾把快照行误渲染成 (N lines) 占位并吞行;混入态必须逐行显示。
// 插入式语义下混合列表=完整过滤列表(含窗口外命中行 Qux),无未命中行落在窗口内 → 4 行全 dim=false。
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
	if len(app.ctxLines) != 4 { // 窗口 [0,2] 内全是命中行,列表 4 行原样保留(含窗口外 Qux)
		t.Fatalf("+5 应保留完整过滤列表 4 行, got %d", len(app.ctxLines))
	}
	for i, e := range app.ctxLines {
		if e.dim {
			t.Fatalf("窗口内全命中,不应有插入行, ctxLines[%d].dim=%v", i, e.dim)
		}
	}
	if app.foldedGroup(1) != nil {
		t.Fatal("混入态应禁折叠查表")
	}
	view := stripANSI(app.View())
	if strings.Contains(view, "lines) [e") {
		t.Fatal("混入态渲染不得出现折叠占位 (N lines)")
	}
	// 4 行过滤行逐行可见(含窗口外的 Qux——插入式:过滤列表完整保留)
	for _, want := range []string{"boom", "Bar.run", "Baz.call", "Qux.work"} {
		if !strings.Contains(view, want) {
			t.Fatalf("过滤列表行 %q 应逐行可见", want)
		}
	}
	// INFO 行未命中过滤且不在上侧窗口,不得出现
	if strings.Contains(view, "nothing here") {
		t.Fatal("窗口外未命中行不应出现")
	}
}
