// 字符光标移动纯函数测试(vim-copy):用例编号 m*/s*/w* 与 fview
// frontend/src/lib/charMotion.test.ts 逐条对齐(双端同表),数值以 vim 实语义为准。
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/justfun/logview/internal/model"
)

func TestCharNavApplyMotion(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		col  int
		m    Motion
		want int
	}{
		// m01: hello world 基本词移动
		{"m01 wordFwd", "hello world", 0, MotionWordFwd, 6},
		{"m01 wordEnd", "hello world", 0, MotionWordEnd, 4},
		{"m01 wordBack", "hello world", 6, MotionWordBack, 0},
		// m02: 标点词
		{"m02 wordFwd to punct", "foo=bar, baz", 0, MotionWordFwd, 3},
		{"m02 wordFwd punct to word", "foo=bar, baz", 3, MotionWordFwd, 4},
		{"m02 wordEnd on punct → next word tail", "foo=bar, baz", 3, MotionWordEnd, 6},
		// m03: 空/单字符 clamp
		{"m03 empty wordFwd", "", 0, MotionWordFwd, 0},
		{"m03 empty end", "", 0, MotionEnd, 0},
		{"m03 single wordFwd clamp", "a", 5, MotionWordFwd, 0},
		{"m03 single left", "a", 0, MotionLeft, 0},
		{"m03 single right", "a", 0, MotionRight, 0},
		// m04: CJK rune 域
		{"m04 cjk wordFwd", "中 文 log", 0, MotionWordFwd, 2},
		{"m04 cjk wordEnd", "中 文 log", 0, MotionWordEnd, 2},
		// m05: 越界输入 clamp
		{"m05 col 99 left", "hello", 99, MotionLeft, 3},
		{"m05 col 99 start", "hello", 99, MotionStart, 0},
		{"m05 col 99 end", "hello", 99, MotionEnd, 4},
		// m06: 0/^/$ 消息域
		{"m06 start", "  leading blank", 4, MotionStart, 0},
		{"m06 firstNonBlank", "  leading blank", 4, MotionFirstNonBlank, 2},
		{"m06 end", "  leading blank", 0, MotionEnd, 14},
		{"m06 all blank firstNonBlank", "   ", 2, MotionFirstNonBlank, 0},
		// m07: 消息首尾停住(不跨行接续)
		{"m07 left stop", "abc", 0, MotionLeft, 0},
		{"m07 right stop", "abc", 2, MotionRight, 2},
		{"m07 wordFwd stop at tail", "abc def", 6, MotionWordFwd, 6},
		{"m07 wordEnd stop at tail", "abc def", 6, MotionWordEnd, 6},
		{"m07 wordBack stop at head", "abc def", 0, MotionWordBack, 0},
		// m08: 词中移动
		{"m08 wordFwd mid word", "abc def", 1, MotionWordFwd, 4},
		{"m08 wordEnd at tail → next", "abc def", 2, MotionWordEnd, 6},
		{"m08 wordBack mid word", "abc def", 1, MotionWordBack, 0},
	}
	for _, c := range cases {
		if got := ApplyMotion(c.msg, c.col, c.m); got != c.want {
			t.Errorf("%s: ApplyMotion(%q,%d)=%d want %d", c.name, c.msg, c.col, got, c.want)
		}
	}
}

func TestCharNavCharSelText(t *testing.T) {
	// s01: 跨行——首行[startCol..]、中间全量、尾行[..endCol]
	text, chars := CharSelText([]string{"hello world", "second line", "tail"}, 6, 2)
	if text != "world\nsecond line\ntai" || chars != 21 {
		t.Errorf("s01: got %q/%d want world\\nsecond line\\ntai/21", text, chars)
	}
	// s02: 单行子串
	if text, _ := CharSelText([]string{"abcd"}, 1, 4); text != "bcd" {
		t.Errorf("s02a: got %q want bcd", text)
	}
	if text, _ := CharSelText([]string{"abcd"}, 0, 0); text != "a" {
		t.Errorf("s02b: got %q want a", text)
	}
	// s03: 空消息行产出空串
	if text, chars := CharSelText([]string{"a", "", "b"}, 0, 0); text != "a\n\nb" || chars != 4 {
		t.Errorf("s03: got %q/%d want a\\n\\nb/4", text, chars)
	}
	// s04: 列 clamp
	if text, _ := CharSelText([]string{"abc"}, 0, 99); text != "abc" {
		t.Errorf("s04a: got %q want abc", text)
	}
	if text, _ := CharSelText([]string{"abc", "def"}, 99, 99); text != "c\ndef" {
		t.Errorf("s04b: got %q want c\\ndef", text)
	}
}

func TestCharNavColToSeg(t *testing.T) {
	segs := []string{"hel", "lo w", "orld"}
	cases := []struct {
		name    string
		col     int
		segIdx  int
		colInSeg int
	}{
		{"w01 col0", 0, 0, 0},
		{"w01 col3", 3, 1, 0},
		{"w01 col10", 10, 2, 3},
		{"w01 col99 clamp", 99, 2, 3},
		{"w01 col5 mid seg2", 5, 1, 2},
	}
	for _, c := range cases {
		si, cs := ColToSeg(segs, c.col)
		if si != c.segIdx || cs != c.colInSeg {
			t.Errorf("%s: ColToSeg(%d)=(%d,%d) want (%d,%d)", c.name, c.col, si, cs, c.segIdx, c.colInSeg)
		}
	}
	// w02: 空段安全
	if si, cs := ColToSeg([]string{""}, 0); si != 0 || cs != 0 {
		t.Errorf("w02: ColToSeg empty=(%d,%d) want (0,0)", si, cs)
	}
}

// TestVimCopyKeys vim-copy 键位腾挪+移动状态机(handleNormalKeys 集成)。
// yy 完整消费路径不进单测(yankLines 真调 pbcopy 污染用户剪贴板),真机验收覆盖。
func TestVimCopyKeys(t *testing.T) {
	app := newTestApp()
	app.cursor = 0

	// 字符移动状态机:$ 激活光标列 → 0 回消息首 → w/l/e 前进 → b 回首
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'$'}})
	if app.cursorCol < 0 {
		t.Fatal("$ 应激活字符光标列")
	}
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})
	if app.cursorCol != 0 {
		t.Fatalf("0 应到消息首, got %d", app.cursorCol)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	if app.cursorCol <= 0 {
		t.Fatalf("w 应词前进, got %d", app.cursorCol)
	}
	wCol := app.cursorCol
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if app.cursorCol != wCol-1 {
		t.Fatalf("h 应回一字符(%d→%d)", wCol, app.cursorCol)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	eBefore := app.cursorCol
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	eAfter := app.cursorCol
	if eAfter <= eBefore {
		t.Fatalf("e 应前进到词尾, %d→%d", eBefore, eAfter)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if app.cursorCol >= eAfter || app.cursorCol < 0 {
		t.Fatalf("b 应回退到前一词首, %d→%d", eAfter, app.cursorCol)
	}

	// ctrl+w 换行腾挪(原 w)
	w0 := app.wrapMode
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	if app.wrapMode == w0 {
		t.Fatal("ctrl+w 应翻转 wrapMode")
	}
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlW})

	// ctrl+h 高亮弹窗(原 h)
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlH})
	if !app.searchMode || app.searchTab != 1 {
		t.Fatal("ctrl+h 应打开高亮 tab")
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEsc})

	// ctrl+x 隐藏弹窗(r2:裸 x 退役)
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if !app.searchMode || app.searchTab != 2 {
		t.Fatal("ctrl+x 应打开隐藏 tab")
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEsc})

	// y 序列:置位 → 第二键透传复位(vim 手感)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if app.pendingKey != "y" {
		t.Fatalf("y 应置 pendingKey, got %q", app.pendingKey)
	}
	c0 := app.cursor
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if app.pendingKey != "" {
		t.Fatal("y 后 j 应复位 pendingKey(透传)")
	}
	if app.cursor == c0 {
		t.Fatal("y 后 j 应正常下移")
	}

	// y 后 Esc 透传退出(不残留 pendingKey)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if app.pendingKey != "" {
		t.Fatal("y 后 Esc 应复位 pendingKey")
	}
}

// TestCharCursorRender 字符光标渲染探针(reverse SGR 出现在光标字符处)。
func TestCharCursorRender(t *testing.T) {
	app := newTestApp()
	app.cursor = 0
	// 未激活:View 无 reverse 标记
	if v := app.View(); strings.Contains(v, "\x1b[7m") {
		t.Fatal("未激活列光标时 View 不应含 reverse SGR")
	}
	// $ 激活:光标落 message 尾字符并带 reverse
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'$'}})
	msg := app.viewLines()[0].Message()
	want := "\x1b[7m" + string([]rune(msg)[app.cursorCol])
	if v := app.View(); !strings.Contains(v, want) {
		t.Fatalf("View 应含光标字符 reverse 标记 %q", want)
	}
	// 0:光标回 message 首
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})
	want0 := "\x1b[7m" + string([]rune(msg)[0])
	if v := app.View(); !strings.Contains(v, want0) {
		t.Fatalf("View 应含行首光标 reverse 标记 %q", want0)
	}

	// wrap 模式:切段后光标标记仍在(SGR replay 跟段)。注:窄屏 View 级断言受
	// 既有 visualStartFromWrap 段数估算偏差影响(底部光标行可能挤出视口,非本
	// 需求引入),故断言渲染函数级产物
	app.width = 40
	app.wrapMode = true
	bg := app.renderLineTextWithBg(app.viewLines()[app.cursor], SelectedBgColor, SelectedFgColor, app.cursor)
	segs := wrapAnsiText(bg, app.contentWidth())
	hit := false
	for _, s := range segs {
		if strings.Contains(s, "\x1b[7m") {
			hit = true
		}
	}
	if !hit {
		t.Fatal("wrap 切段后应仍有光标 reverse 标记(SGR replay)")
	}
	app.wrapMode = false
	app.width = 120

	// G 到底(短行):列记忆 clamp 到短行尾($ 拉满 colWish 再跳短行)
	app.processLine(model.RawLine{Text: "short", Source: "s"})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'$'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if app.cursorCol != 4 {
		t.Fatalf("G 到短行应 clamp colWish→4, got %d", app.cursorCol)
	}
	if v := app.View(); !strings.Contains(v, "\x1b[7mt") {
		t.Fatal("短行光标应落尾字符 t")
	}
}

// TestVisualCharSel 字符可视选择(vim-copy Task9):行集组装/锚列/渲染探针。
// y 提交路径不进单测(copyToClipboard 真调 pbcopy),真机验收覆盖。
func TestVisualCharSel(t *testing.T) {
	// 测试环境无 TTY,lipgloss 全局 profile=Ascii 会丢背景色——选区 SGR 探针
	// 需要 256 色渲染(用例内设置,结束还原防污染其他测试)
	oldProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(oldProfile)

	app := newTestApp()
	app.cursor = 0

	// 非选区态 ok=false
	if _, _, _, ok := app.charSelMsgs(); ok {
		t.Fatal("非选区态 charSelMsgs 应 ok=false")
	}

	// v(col0) + l×3:单行选区 [0..3]
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if !app.visualMode || !app.visualChar {
		t.Fatal("v 应进入字符可视模式")
	}
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	msgs, from, to, ok := app.charSelMsgs()
	if !ok || len(msgs) != 1 || from != 0 || to != 3 {
		t.Fatalf("单行选区应 [0..3], got msgs=%d from=%d to=%d ok=%v", len(msgs), from, to, ok)
	}

	// 渲染探针:选区背景 SGR(48;5 256色 / 48;2 真彩均可能,按主题配置)
	if v := app.View(); !strings.Contains(v, "\x1b[48;") {
		t.Fatal("字符选区行应有选区背景 SGR")
	}

	// 跨行:v(col4) + j:两行选区,首行 [4..]、尾行 [..colWish clamp]
	app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'$'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	msgs, from, to, ok = app.charSelMsgs()
	if !ok || len(msgs) != 2 || from != 4 {
		t.Fatalf("跨行选区应 2 行 from=4, got msgs=%d from=%d to=%d ok=%v", len(msgs), from, to, ok)
	}
	text, chars := CharSelText(msgs, from, to)
	if chars == 0 || !strings.Contains(text, string([]rune(msgs[0])[4:])) {
		t.Fatalf("跨行文本组装异常: %q", text)
	}

	// V 行级回归:visualChar=false
	app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'V'}})
	if !app.visualMode || app.visualChar {
		t.Fatal("V 应进入行级可视模式(非字符)")
	}
}

// TestVisualCharFreshV 直入 v(列光标未激活)锚列不得为 -1(Final Review I2):
// visualCol 残留 -1 会使锚行选区底纹不渲染(markSelRange from<0 no-op)。
func TestVisualCharFreshV(t *testing.T) {
	oldProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(oldProfile)

	app := newTestApp()
	app.cursor = 0
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}}) // 未按过任何列移动键
	if app.visualCol < 0 {
		t.Fatalf("直入 v 锚列应 clamp 0, got %d", app.visualCol)
	}
	from, _, ok := app.charSelRowRange(0)
	if !ok || from != 0 {
		t.Fatalf("单行选区区间应 (0,..), got (%d,ok=%v)", from, ok)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	v := app.View()
	if !strings.Contains(v, "\x1b[48;") {
		t.Fatal("直入 v 扩选后 View 应有选区底纹 SGR")
	}
}

// TestVisualCharSelNoTrunc 非 wrap 模式字符选区行强制 wrap 不截断(Final Review I3,
// spec"所见即所选"):锚行超宽时尾部须可见(普通路径被 MaxWidth 截断丢尾)。
func TestVisualCharSelNoTrunc(t *testing.T) {
	app := newTestApp() // 行 message 90 rune,前缀后显示宽超 contentWidth
	app.cursor = 0
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}) // 锚=行0(超宽),光标=行1
	v := app.View()
	if !strings.Contains(v, "message") {
		t.Fatal("非 wrap 选区行(锚行)超宽应强制 wrap,消息尾部可见")
	}
}
