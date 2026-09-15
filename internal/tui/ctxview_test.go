package tui

import (
	"fmt"
	"reflect"
	"regexp"
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
// 方向语义(二修换向后):+ 往后(下侧窗口 (idx, idx+n]),- 往前(上侧窗口 [idx-n, idx))。
// Seq=i 唯一:书签按 Seq 键控,鉴别力依赖唯一 Seq(全 0 时 bookmarks 断言无法区分取错行)。
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
			Raw:     model.RawLine{Text: "2026-09-14 10:00:00.000 [t] " + lv + "  c.x.Svc - " + msg, Source: "ctx.log", Seq: uint64(i)},
			Level:   lv,
			Message: msg,
		})
	}
	app.levelFilter = "ERROR"
	app.recomputeView() // 真实过滤:L01/L03/.../L11 共 6 行
	app.cursor = 2      // L05
	return app
}

// +3(往后):下方插入 (4,7] 未命中行 L06,L08 在 L07/L09 之间。
func TestCtxViewPlusAfter(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	var got []string
	for _, e := range app.ctxLines {
		m := e.pl.Message
		if e.dim {
			m += "d"
		}
		got = append(got, m)
	}
	want := []string{"L01", "L03", "L05", "L06d", "L07", "L08d", "L09", "L11"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("+3(往后)顺序 = %v, want %v", got, want)
	}
	if app.cursor != 2 { // anchor L05 位置不变(上方无插入)
		t.Fatalf("anchor 光标应仍在 idx2, got %d", app.cursor)
	}
}

// -3(往前):上方插入 [1,4) 未命中行 L02,L04。
func TestCtxViewMinusBefore(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	var got []string
	for _, e := range app.ctxLines {
		m := e.pl.Message
		if e.dim {
			m += "d"
		}
		got = append(got, m)
	}
	want := []string{"L01", "L02d", "L03", "L04d", "L05", "L07", "L09", "L11"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("-3(往前)顺序 = %v, want %v", got, want)
	}
	if app.cursor != 4 { // anchor L05 因上方插入后移到 idx4
		t.Fatalf("anchor 光标应落 idx4, got %d", app.cursor)
	}
}

// +2(往后):触发行(L05)下方插入 (idx, idx+2] = L06;L07 命中(奇数)不插 → 只插 L06。
func TestCtxViewInsertPlus(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if len(app.ctxLines) != 7 { // 6 + L06
		t.Fatalf("+2 混合列表应 7 行, got %d", len(app.ctxLines))
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

// -3(往前):过滤列表 6 行保留,触发行(L05)上方插入 buffer[L02,L04](未命中行,L03 命中不插)。
func TestCtxViewInsertMinus(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})

	vl := app.viewLines()
	if len(vl) != 8 { // 6 过滤行 + 2 插入行(L02/L04)
		t.Fatalf("-3 混合列表应 8 行(6+2), got %d", len(vl))
	}
	// 顺序:L01,L02(dim),L03,L04(dim),L05(anchor),L07,L09,L11
	wantDim := []bool{false, true, false, true, false, false, false, false}
	for i, e := range app.ctxLines {
		if e.dim != wantDim[i] {
			t.Fatalf("ctxLines[%d].dim=%v, want %v", i, e.dim, wantDim[i])
		}
	}
	if app.cursor != 4 { // anchor(L05)在混合列表的新位置(上方插入后移)
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

// 回归(换向适配,+ 现在是下侧):+x 窗口尾部未命中行必须按时间序插入
// (首个越过窗口的命中行之前),不得甩到混合列表末尾(曾把 L08 追加到 L11 之后)。
func TestCtxViewPlusTailOrder(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
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
		t.Fatalf("+3 混合顺序 = %v, want %v", got, want)
	}
}

// 回归(换向适配,+ 现在是下侧):默认 + (n=5) 窗口 (4,9] 内未命中 L06/L08/L10 全部按时间序插入
// (L10 落在 L09 与 L11 之间,曾甩尾到 L11 之后)。
// 注:终审 brief 给的 want 漏了 L10d——手工推演 buffer idx9=L10 属窗口内 INFO,
// 按 spec((idx, idx+n] 全量未命中行)应插入 dim,want 已修正为含 L10d。
func TestCtxViewPlusDefaultOrder(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
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
		t.Fatalf("+ 默认混合顺序 = %v, want %v", got, want)
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
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
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

// 回归:插入行视觉强化——整行剥内层列色(时间/级别/来源 SGR 不残留),行首 ┆ 标记。
// 探针策略(参照 wrap-dim 测试):临时把全局 DetailDimStyle 换成独占色(用毕还原),
// TrueColor profile 强制开启(无 TTY 测试环境会剥色);渲染行由 FrameStyle 的
// "│ "/" │" 边框包裹(app.go renderLogs),故断言:插入行 SGR 集合 ⊆ {DetailDim,
// FrameStyle, reset}——任何内层列色残留即红;┆ 须为边框后日志内容的第一位。
// 判别力:若移除 stripANSI 实现,内层列色(级别 61/来源色等)不在白名单 → 红;
// 若丢 ┆ 前缀则找不到插入行 → 红;DetailDim 探针 SGR 必现则排除 TrueColor 空转。
func TestCtxViewDimStripped(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prevProfile)
	prevStyle := DetailDimStyle
	DetailDimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ABCDEF"))
	defer func() { DetailDimStyle = prevStyle }()

	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app.wrapMode = false
	out := app.View()

	var dimRow string
	for _, row := range strings.Split(out, "\n") {
		s := stripANSI(row)
		if strings.Contains(s, "┆") && strings.Contains(s, "L06") {
			dimRow = row
			break
		}
	}
	if dimRow == "" {
		t.Fatal("非 wrap 模式插入行 L06 应渲染且带 ┆ 标记")
	}
	if plain := stripANSI(dimRow); !strings.HasPrefix(plain, "│ ┆") {
		t.Fatalf("插入行 ┆ 应为边框后日志内容的第一位, row=%q", plain)
	}
	dimSeq := strings.Split(DetailDimStyle.Render("PROBE"), "PROBE")[0]
	if dimSeq == "" {
		t.Fatal("前置:TrueColor 下 DetailDimStyle 应产生 SGR 序列")
	}
	if !strings.Contains(dimRow, dimSeq) {
		t.Fatalf("前置:插入行应带 DetailDimStyle SGR %q(排除 TrueColor 空转), row=%q", dimSeq, dimRow)
	}
	// 整行仅允许 DetailDim 与边框(FrameStyle)的 SGR
	sgrRe := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	allowed := map[string]bool{"\x1b[0m": true}
	for _, probe := range []string{DetailDimStyle.Render("PROBE"), FrameStyle.Render("│ ")} {
		for _, seq := range sgrRe.FindAllString(probe, -1) {
			allowed[seq] = true
		}
	}
	for _, seq := range sgrRe.FindAllString(dimRow, -1) {
		if !allowed[seq] {
			t.Fatalf("插入行不得残留内层列色 SGR(应 stripANSI 统一暗色), 非法 SGR %q, row=%q", seq, dimRow)
		}
	}
}

// 边界 clamp(换向后 - 是上侧):cursor 在列表首行(L01,buffer idx0)+默认5 →
// 上侧无行可插,视图=纯列表。
func TestCtxViewInsertClampTop(t *testing.T) {
	app := ctxSetup(t)
	app.cursor = 0 // L01
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(app.ctxLines) != 6 {
		t.Fatalf("顶部 -默认应无插入行, got %d", len(app.ctxLines))
	}
	for i, e := range app.ctxLines {
		if e.dim {
			t.Fatalf("顶部 clamp 后不应有 dim 行, ctxLines[%d].dim=%v", i, e.dim)
		}
	}
}

// sticky:混入态移动不退出,光标可落插入行;z 序列正常锚定;v 拦截;无过滤时 +/- 无操作。
func TestCtxViewStickyNav(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// -Enter(空数字=5):上侧窗口 [0,4) 未命中行 L02,L04 插在 L03 前后
	if len(app.ctxLines) == 0 {
		t.Fatal("混入态应激活")
	}
	app.Update(fakeKey("j"))
	if len(app.ctxLines) == 0 {
		t.Fatal("移动键不得退出混入")
	}
	if !app.ctxDim(app.cursor) && app.cursor == 2 {
		t.Fatalf("j 后光标应离开触发行可落插入行, cursor=%d", app.cursor)
	}

	// z 序列:锚定生效不退出
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if len(app.ctxLines) == 0 || app.scrollAnchor != 1 {
		t.Fatalf("混入态 zt 应锚定不退出, ctxLines=%d anchor=%d", len(app.ctxLines), app.scrollAnchor)
	}

	// v 拦截
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if app.visualMode {
		t.Fatal("混入态禁可视选择")
	}

	// 无过滤时 + 无操作(原有断言保留)
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

// 混入态 m 书签落在插入行上(Seq 取自 viewLines 而非 filteredView)。
func TestCtxViewBookmarkOnCtxLine(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app.Update(fakeKey("j")) // L06 插入行
	if !app.ctxDim(app.cursor) {
		t.Fatalf("光标应落插入行 L06, cursor=%d", app.cursor)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if len(app.bookmarks) != 1 {
		t.Fatalf("插入行上书签应生效, bookmarks=%d", len(app.bookmarks))
	}
	// 鉴别力(Task1 收紧):书签必须打在插入行 L06(Seq=6)上——
	// 旧实现读 filteredView[cursor] 会错打在 L07(Seq=7)上,len==1 对两者都成立。
	if !app.bookmarks[6] {
		t.Fatalf("书签应落在插入行 L06(Seq=6) 上, bookmarks=%v", app.bookmarks)
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

// 插入行上退出:回最近过滤行(前后取近,相等取前)。L06 前后等距 L05/L07,取前 L05(过滤 idx2)。
func TestCtxViewExitNearest(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app.Update(fakeKey("j")) // L06 插入行
	app.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if len(app.ctxLines) != 0 {
		t.Fatal("Esc 应退出混入")
	}
	if app.cursor != 2 {
		t.Fatalf("应回最近过滤行 L05(等距取前), cursor=%d", app.cursor)
	}

	// L04 插入行(-5 场景):最近过滤行是 L03(idx1),不是锚 L05(idx2)
	app3 := ctxSetup(t)
	app3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	app3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	app3.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// -5:ctxLines = L01 L02(dim) L03 L04(dim) L05 L07 L09 L11,cursor 初始在 L05(idx4)
	app3.Update(fakeKey("k")) // L04 插入行(idx3)
	app3.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if app3.cursor != 1 {
		t.Fatalf("L04 上退出应回最近过滤行 L03(idx1), cursor=%d", app3.cursor)
	}
}

// --/++ 关闭:任一符号连按即关,无需 Enter;纯输入态(未混入)第二个符号=取消。
func TestCtxViewDoubleSignClose(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	if len(app.ctxLines) != 0 {
		t.Fatal("-- 应关闭混入(任一符号)")
	}
	// 纯输入态:首符号后按另一符号=取消,不构建
	app2 := ctxSetup(t)
	app2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	if app2.ctxInput != "" || len(app2.ctxLines) != 0 {
		t.Fatalf("输入态第二符号应取消, ctxInput=%q ctxLines=%d", app2.ctxInput, len(app2.ctxLines))
	}
}

// 调数重建:混入态 +8 Enter 以光标所在最近过滤行为新锚,插入行上重建不漂移。
func TestCtxViewRebuildAnchor(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app.Update(fakeKey("j")) // L06 插入行(混入坐标 idx3)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'8'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(app.ctxLines) == 0 {
		t.Fatal("重建后混入态应保持")
	}
	// 锚=距 L06 最近的过滤行 L05;buildCtxLines 末尾 cursor 指向新列表中 L05 的位置(idx2)
	if app.cursor != 2 {
		t.Fatalf("重建锚应为 L05(idx2), cursor=%d", app.cursor)
	}
}

// 回归:混入态禁折叠——stGroups 是 filteredView 坐标,混入视图按混合列表索引查表会错位,
// 曾把快照行误渲染成 (N lines) 占位并吞行;混入态必须逐行显示。
// 插入式语义下混合列表=完整过滤列表(含窗口外命中行 Qux),无未命中行落在窗口内 → 4 行全 dim=false。
// 换向后上侧窗口由 - 触发(cursor=2,窗口 [0,2) 全命中)。
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
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(app.ctxLines) != 4 { // 窗口 [0,2) 内全是命中行,列表 4 行原样保留(含窗口外 Qux)
		t.Fatalf("-5 应保留完整过滤列表 4 行, got %d", len(app.ctxLines))
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

// 混入态 d 详情:光标在插入行上时详情面板显示插入行本体。
func TestCtxViewDetailOnCtxLine(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app.Update(fakeKey("j")) // L06 插入行
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if !app.detailMode {
		t.Fatal("混入态 d 应打开详情")
	}
	view := stripANSI(app.View())
	// 详情面板 row 独有特征:L06 是 INFO,面板含 level INFO 行且不含 level ERROR 行
	// (触发行 L05 是 ERROR;背景层只有日志行,无 "level" 标签的面板行)
	if !strings.Contains(view, "level   INFO") || strings.Contains(view, "level   ERROR") {
		t.Fatalf("详情面板应显示插入行 L06 本体(INFO), view=%q", view)
	}
	if len(app.ctxLines) == 0 {
		t.Fatal("d 不得退出混入")
	}
}

// 混入态 n/N:跳到目标命中行,光标换算到 ctxLines 位置,混入保持。
// 用 highlights 路径(jumpSearchMatch 的 else 分支:searchInput 空时按高亮词跳转),
// 避免 searchInput 变化触发 recomputeView 退出混入——那是 Task 4 的语义。
func TestCtxViewJumpSearchSticky(t *testing.T) {
	app := ctxSetup(t)
	app.highlights = []string{"L11"} // 高亮词唯一命中 L11(过滤视图末行 idx5)
	app.cursor = 2
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if len(app.ctxLines) == 0 {
		t.Fatal("n 不得退出混入")
	}
	// +3 后 ctxLines = L01 L03 L05 L06(dim) L07 L08(dim) L09 L11,L11 在 idx7;
	// 旧代码把 cursor 设为 filteredView 索引 5(指向 L08),新代码应设 7。
	if got := app.ctxLines[app.cursor].pl.Message; got != "L11" {
		t.Fatalf("n 应跳到 L11 的 ctxLines 位置(idx7), cursor=%d msg=%s", app.cursor, got)
	}
}

// 语义性自动退出:过滤变化(recomputeView)、清屏(C)、源重置(resetViewState)均清混入。
func TestCtxViewAutoExit(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(app.ctxLines) == 0 {
		t.Fatal("前置:混入态应激活")
	}
	app.levelFilter = "INFO"
	app.recomputeView()
	if len(app.ctxLines) != 0 {
		t.Fatal("过滤变化应自动退出混入")
	}

	// 清屏键 C
	app2 := ctxSetup(t)
	app2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	if len(app2.ctxLines) != 0 {
		t.Fatal("清屏应清混入")
	}

	// 源重置(ReplaceStream 共用路径)
	app3 := ctxSetup(t)
	app3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app3.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app3.resetViewState()
	if len(app3.ctxLines) != 0 {
		t.Fatal("源重置应清混入")
	}
}

// ctxToFv:插入行起点取其后最近命中行;fvToCtx:过滤列表全保留,必命中。
// +3 后 ctxLines = L01 L03 L05 L06(dim) L07 L08(dim) L09 L11;
// filteredView = L01 L03 L05 L07 L09 L11(idx:L01=0 L03=1 L05=2 L07=3 L09=4 L11=5)。
func TestCtxViewIndexConv(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := app.ctxToFv(3); got != 3 { // L06(dim,idx3) → 其后最近命中 L07(filteredView idx3)
		t.Fatalf("ctxToFv(3)=%d, want 3(L06→L07)", got)
	}
	if got := app.ctxToFv(5); got != 4 { // L08(dim,idx5) → 其后最近命中 L09(filteredView idx4)
		t.Fatalf("ctxToFv(5)=%d, want 4(L08→L09)", got)
	}
	if got := app.fvToCtx(3); got != 4 { // L07 在 ctxLines idx4
		t.Fatalf("fvToCtx(3)=%d, want 4(L07)", got)
	}
	if got := app.fvToCtx(5); got != 7 { // L11 在 ctxLines idx7
		t.Fatalf("fvToCtx(5)=%d, want 7(L11)", got)
	}
}

// 状态栏持久指示:混入态显示 混入 ×1(锚数),退出后消失。
func TestCtxViewStatusIndicator(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	view := stripANSI(app.View())
	if !strings.Contains(view, "混入 ×1") {
		t.Fatal("混入态状态栏应显示 混入 ×1(锚数)")
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEscape})
	view = stripANSI(app.View())
	if strings.Contains(view, "混入 ×1") {
		t.Fatal("退出后指示应消失")
	}
}

// 无幽灵指示:换源(resetViewState)/清屏(clearScreen)直接清空混入快照时,
// ctxN/ctxAnchors 必须同步清零——否则状态栏残留"混入 ×1",且 Esc/-- 走 exitCtxView
// 早退分支永远清不掉。
func TestCtxViewNoGhostIndicator(t *testing.T) {
	// 场景①:换源 resetViewState
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.ctxN != 1 {
		t.Fatalf("前置失败:ctxN=%d, want 1(锚数)", app.ctxN)
	}
	app.resetViewState()
	if app.ctxN != 0 || len(app.ctxAnchors) != 0 {
		t.Fatalf("resetViewState 后应清零:ctxN=%d ctxAnchors=%d", app.ctxN, len(app.ctxAnchors))
	}
	// 场景②:清屏 clearScreen
	app2 := ctxSetup(t)
	app2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app2.clearScreen()
	if app2.ctxN != 0 || len(app2.ctxAnchors) != 0 {
		t.Fatalf("clearScreen 后应清零:ctxN=%d ctxAnchors=%d", app2.ctxN, len(app2.ctxAnchors))
	}
}

// 多锚累积(迭代 2):不同行分别 +3 互不清除;同行重复按键参数覆盖;++ 清全部。
// buffer idx:L05=4 L07=6;+3 后 ctxLines = L01 L03 L05 L06d L07 L08d L09 L11(L07 在 idx4)。
func TestCtxViewMultiAnchor(t *testing.T) {
	app := ctxSetup(t)
	// 行 L05(过滤 idx2)+3:窗口 (4,7] → 插 L06/L08
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// 移到 L07 再 +3:L05 锚保留,L07 新增锚(窗口 (6,9])
	app.Update(fakeKey("j")) // L06(dim,ctx idx3)
	app.Update(fakeKey("j")) // L07(ctx idx4)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.ctxN != 2 {
		t.Fatalf("应有 2 锚, ctxN=%d", app.ctxN)
	}
	// 两锚窗口合并 (4,7]∪(6,9] = {4..9}:L06/L08/L10 三个未命中行都插入 dim
	// (L07 窗口含 buffer idx9=L10,不在窗口外);命中行 L07/L09 不插
	dims := 0
	for _, e := range app.ctxLines {
		if e.dim {
			dims++
		}
	}
	if dims != 3 { // L06 + L08 + L10
		t.Fatalf("两个锚的插入行都应在(L06/L08/L10), dims=%d", dims)
	}
	// 同行覆盖:L07 上改按 -3 → L07 窗口变前向 [3,6) 插 L04;L05 锚不动,
	// 其窗口 (4,7] 仍覆盖 L08 → L08 保留(多锚语义:各行上下文互不清除)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.ctxN != 2 {
		t.Fatalf("同行覆盖后锚仍应 2, ctxN=%d", app.ctxN)
	}
	msgs := map[string]bool{}
	for _, e := range app.ctxLines {
		if e.dim {
			msgs[e.pl.Message] = true
		}
	}
	// 合并窗口 {3..7}:dim = L04(L07 前向)、L06/L08(L05 窗口);L02 不在任何窗口
	if !msgs["L04"] || !msgs["L06"] || !msgs["L08"] || msgs["L02"] {
		t.Fatalf("同行覆盖:应保留 L05 锚的 L06/L08、L07 改前向插 L04、无 L02, got %v", msgs)
	}
	// ++ 清全部锚退出
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	if len(app.ctxLines) != 0 || app.ctxN != 0 || len(app.ctxAnchors) != 0 {
		t.Fatal("++ 应清全部锚退出")
	}
}

// ---- 终审修复回归(follow 增量路径,2026-09-14)----
// 混入态 cursor 是 ctxLines 坐标(快照定长),filteredView 随 follow 新行增量增长——
// 以下测试构建混入后用 processBatch 追加新行模拟 follow 增量,钉死各路径不得
// 按"cursor 与 filteredView 同坐标系"的假设行事。

// C1 回归:混入态 follow 批次不得按 filteredView 长度钳 ctx 坐标光标。
// 过滤 6 行 +3 混入 8 行,光标在末行 L11(ctx idx7);follow 新批次到达(新行未命中
// 过滤,filteredView 仍 6 行)后光标必须原地不动——旧 processBatch clamp 用 fv 长度
// 钳 ctx 坐标,7>=6 把光标钳到 5(L08 dim 插入行),每个批次跳一次行。
func TestCtxViewFollowBatchNoJump(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app.cursor = len(app.viewLines()) - 1 // 末行 L11(ctx idx7;G 被混入守卫压掉,直接设)
	if app.viewLines()[app.cursor].Message != "L11" {
		t.Fatalf("前置:光标应在末行 L11, got %s", app.viewLines()[app.cursor].Message)
	}
	// follow 批次:新行 INFO 未命中过滤,filteredView 仍 6 行,clamp 无条件执行
	app.processBatch([]model.RawLine{{Text: "2026-09-14 10:01:00.000 [t] INFO  c.x.Svc - L13", Source: "ctx.log", Seq: 13}})
	if app.cursor != 7 {
		t.Fatalf("follow 批次不得移动混入态光标(sticky), cursor=%d", app.cursor)
	}
	if msg := app.viewLines()[app.cursor].Message; msg != "L11" {
		t.Fatalf("光标应仍在 L11, got %s", msg)
	}
	if len(app.ctxLines) != 8 {
		t.Fatalf("follow 新行不得进混入快照(快照语义), ctxLines=%d", len(app.ctxLines))
	}
}

// followFollowLine 模拟 follow 增量追加一条命中过滤的新行 L13:等价复现 processLine
// 对命中行的增量效果(buffer.Push + filteredView append);不走 processBatch 是因为
// 测试环境 parsers 为 nil,构造行无 Level 字段过不了 levelFilter,而 follow 增量本身
// 不重跑 recomputeView(语义性退出只由过滤变化触发)。
func followFollowLine(app *App) {
	pl := &model.ParsedLine{
		Raw:     model.RawLine{Text: "2026-09-14 10:01:00.000 [t] ERROR  c.x.Svc - L13", Source: "ctx.log", Seq: 13},
		Level:   "ERROR",
		Message: "L13",
	}
	app.buffer.Push(pl)
	app.filteredView = append(app.filteredView, pl) // idx6,不在 8 行混入快照内
}

// I1 回归(搜索跳转):n/N 跳向 follow 追加的命中行(不在混入快照)时,必须退出混入后
// 直接用 filteredView 索引落点——旧 fvToCtx 兜底裸返 fv 索引,光标停在快照的 dim 行上
// (ctxLines[6]=L09),高亮与后续 y/d/m 全按错误行生效。
func TestCtxViewJumpToFollowLineExits(t *testing.T) {
	app := ctxSetup(t)
	app.highlights = []string{"L13"} // 高亮词路径:searchInput 变化会退出混入,故用 highlights
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	followFollowLine(app)
	if len(app.filteredView) != 7 {
		t.Fatalf("前置:follow 命中行应增量进 filteredView, got %d", len(app.filteredView))
	}
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if len(app.ctxLines) != 0 {
		t.Fatal("目标行不在快照,n 应先退出混入再落点(语义诚实:要看的行不在快照就得退出看)")
	}
	if msg := app.filteredView[app.cursor].Message; msg != "L13" {
		t.Fatalf("n 应落在 follow 追加行 L13 上, got %s(cursor=%d)", msg, app.cursor)
	}
}

// I1 回归(书签跳转):jumpBookmark 落点不在快照时同样退出混入后直接用 fv 索引。
func TestCtxViewJumpBookmarkFollowLineExits(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	followFollowLine(app)
	// 书签打在 follow 追加行 L13 上(Seq=13,不在混入快照内)
	app.bookmarks[uint64(13)] = true
	app.bookmarkSeq = append(app.bookmarkSeq, uint64(13))
	app.jumpBookmark()
	if len(app.ctxLines) != 0 {
		t.Fatal("书签目标行不在快照,jumpBookmark 应先退出混入再落点")
	}
	if msg := app.filteredView[app.cursor].Message; msg != "L13" {
		t.Fatalf("书签跳转应落在 follow 追加行 L13 上, got %s(cursor=%d)", msg, app.cursor)
	}
}

// M1 回归:混入态 scrollPercent 分母必须用 viewLines()(快照行数)——旧实现用
// filteredView 长度,ctx 坐标光标除以 fv 长度会给出超 100% 的百分比(idx7/6 → 133%)。
func TestCtxViewScrollPercentMixed(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app.cursor = 7 // 末行 L11:ctx 坐标 7 > fvLen 6
	if got := app.scrollPercent(); got != " ─ 100%" {
		t.Fatalf("混入态末行百分比应为 100%%, got %q", got)
	}
	app.cursor = 3 // L06 dim:8 行中第 4 行 → 50%(旧实现 4*100/6=66%)
	if got := app.scrollPercent(); got != " ─ 50%" {
		t.Fatalf("混入态 idx3 百分比应为 50%%, got %q", got)
	}
}

// M2 回归:混入态 updateSearchStats 的"第 m/N"序号须先把 ctx 坐标光标换算回
// filteredView 再比较——旧实现拿 ctx 坐标与 fv 索引直接比较,dim 行偏移使序号偏大
// (末尾段恒计满)。用 level:ERROR 命中全部 6 过滤行、不命中 dim 行(L06/L08 为 INFO)。
func TestCtxViewSearchStatsMixedCursor(t *testing.T) {
	app := ctxSetup(t)
	app.searchInput = "level:ERROR"
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app.Update(fakeKey("j")) // L06 dim(ctx idx3)
	app.Update(fakeKey("j")) // L07(ctx idx4,filteredView idx3)
	app.updateSearchStats()
	if app.searchMatchCount != 6 {
		t.Fatalf("命中数应为 6, got %d", app.searchMatchCount)
	}
	if app.searchMatchIdx != 4 { // L07 是第 4 个命中;旧实现 ctx 坐标 4 直接比较得 5
		t.Fatalf("光标在 L07 时序号应为第 4 个, got %d", app.searchMatchIdx)
	}
}

// M3 回归:混入态打开搜索弹窗,星标字段须取光标实际所在行(viewLines)——旧实现查
// filteredView[cursor],ctx 坐标下取到别行字段(idx3 本是 L06 却取到 L07)。
func TestCtxViewStarFieldsOnCtxLine(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app.Update(fakeKey("j")) // L06 dim(ctx idx3)
	app.populateSearchFields()
	has := func(v string) bool {
		for _, sf := range app.starFields {
			if sf.Value == v {
				return true
			}
		}
		return false
	}
	if !has("L06") {
		t.Fatalf("星标字段应来自插入行 L06 本体, starFields=%v", app.starFields)
	}
	if has("L07") {
		t.Fatalf("星标字段不得取到 filteredView 错位的 L07, starFields=%v", app.starFields)
	}
}

// M4 回归:buildCtxLines 校验先行、换算后置——旧实现先 nearestFvCursor 把光标换算成
// fv 坐标再 early-return,若旧快照仍在,光标(已变 fv 坐标)与视图(仍按混入坐标渲染)
// 错位。正常流程过滤变化会经 recomputeView 自动退出、此状态不可达,测试直接构造钉死不变量。
func TestCtxViewRebuildValidateBeforeConvert(t *testing.T) {
	app := ctxSetup(t)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app.Update(fakeKey("j")) // L06 dim(ctx idx3)
	app.levelFilter = ""          // 绕过 recomputeView 的自动退出,构造"校验失败但快照仍在"
	app.searchInput = ""
	app.hides = nil
	app.buildCtxLines(false, 3)
	if app.cursor != 3 {
		t.Fatalf("校验失败的 early-return 不得改写光标坐标(否则与旧快照错位), cursor=%d", app.cursor)
	}
	if len(app.ctxLines) == 0 || !app.ctxDim(app.cursor) {
		t.Fatal("旧混入快照应原样保留(光标仍指 L06 dim)")
	}
}
