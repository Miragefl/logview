// vim-count 数字前缀集成测试:3j/5w/10G/2C-d/连按合并"10j"/Esc 清零/0 并入/
// visual 扩选/yankMsg 反馈。用例语义与 fview keymap.test.ts + shell.test.ts 对齐(双端同表)。
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func press(r ...rune) tea.Msg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: r}
}

// colReset 光标列回消息首(0 键),供词移动用例重置起点。
func colReset(app *App) {
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})
}

func TestCountPrefix(t *testing.T) {
	t.Run("3j 光标下移 3 行", func(t *testing.T) {
		app := newTestApp()
		app.cursor = 0
		app.Update(press('3'))
		app.Update(press('j'))
		if app.cursor != 3 {
			t.Fatalf("3j 后 cursor=%d, want 3", app.cursor)
		}
	})

	t.Run("5w 词前进 5 次(与 5 次单步 w 等价)", func(t *testing.T) {
		app := newTestApp()
		app.cursor = 0
		app.Update(press('$')) // 激活字符光标列
		app.Update(press('0')) // 回消息首
		for i := 0; i < 5; i++ {
			app.Update(press('w'))
		}
		want := app.cursorCol
		colReset(app)
		app.Update(press('5'))
		app.Update(press('w'))
		if app.cursorCol != want {
			t.Fatalf("5w 后 cursorCol=%d, want %d(5 次单步)", app.cursorCol, want)
		}
	})

	t.Run("10G/10g 跳视图第 10 行;100G clamp 尾行", func(t *testing.T) {
		app := newTestApp()
		app.cursor = 0
		app.Update(press('1'))
		app.Update(press('0'))
		app.Update(press('G'))
		if app.cursor != 9 {
			t.Fatalf("10G 后 cursor=%d, want 9", app.cursor)
		}
		app.cursor = 0
		app.Update(press('1'))
		app.Update(press('0'))
		app.Update(press('g'))
		if app.cursor != 9 {
			t.Fatalf("10g 后 cursor=%d, want 9", app.cursor)
		}
		app.cursor = 0
		app.Update(press('1'))
		app.Update(press('0'))
		app.Update(press('0'))
		app.Update(press('G'))
		if app.cursor != 19 {
			t.Fatalf("100G 后 cursor=%d, want 19(clamp)", app.cursor)
		}
	})

	t.Run("连按合并:单条 KeyRunes 10j 等效 10 行", func(t *testing.T) {
		app := newTestApp()
		app.cursor = 0
		app.Update(press('1', '0', 'j'))
		if app.cursor != 10 {
			t.Fatalf("合并 10j 后 cursor=%d, want 10", app.cursor)
		}
	})

	t.Run("Esc/不相关键清 count,后续 j 单步", func(t *testing.T) {
		app := newTestApp()
		app.cursor = 0
		app.Update(press('3'))
		app.Update(tea.KeyMsg{Type: tea.KeyEscape})
		app.Update(press('j'))
		if app.cursor != 1 {
			t.Fatalf("Esc 清 count 后 j cursor=%d, want 1", app.cursor)
		}
		app.Update(press('3'))
		app.Update(press('?')) // help 等非移动键不吃 count
		app.Update(press('?')) // 关 help
		if app.countN != 0 {
			t.Fatalf("? 后 countN=%d, want 0", app.countN)
		}
		app.Update(press('j'))
		if app.cursor != 2 {
			t.Fatalf("count 已清后 j cursor=%d, want 2", app.cursor)
		}
	})

	t.Run("count>0 时 0 并入:10w 等效 10 次单步", func(t *testing.T) {
		app := newTestApp()
		app.cursor = 0
		app.Update(press('$'))
		app.Update(press('0'))
		for i := 0; i < 10; i++ {
			app.Update(press('w'))
		}
		want := app.cursorCol
		colReset(app)
		app.Update(press('1'))
		app.Update(press('0'))
		app.Update(press('w'))
		if app.cursorCol != want {
			t.Fatalf("10w 后 cursorCol=%d, want %d", app.cursorCol, want)
		}
	})

	t.Run("9999 clamp 1000", func(t *testing.T) {
		app := newTestApp()
		for _, r := range []rune{'9', '9', '9', '9'} {
			app.Update(press(r))
		}
		if app.countN != 1000 {
			t.Fatalf("9999 后 countN=%d, want 1000", app.countN)
		}
	})

	t.Run("yankMsg 反馈 count 输入中", func(t *testing.T) {
		app := newTestApp()
		app.cursor = 0
		app.Update(press('3'))
		if !strings.Contains(app.yankMsg, "3") {
			t.Fatalf("count 输入中 yankMsg=%q, 应含 3", app.yankMsg)
		}
		app.Update(press('j'))
		if strings.Contains(app.yankMsg, "3") {
			t.Fatalf("动作后 yankMsg=%q, count 提示应消失", app.yankMsg)
		}
	})
}

func TestCountVisual(t *testing.T) {
	app := newTestApp()
	app.cursor = 0
	app.Update(press('j')) // 激活行光标
	app.Update(press('$'))
	app.Update(press('0'))
	app.Update(press('v')) // 字符可视
	// 基准:两次单步 w
	for i := 0; i < 2; i++ {
		app.Update(press('w'))
	}
	want := app.cursorCol
	app.Update(tea.KeyMsg{Type: tea.KeyEscape})
	// 重来:2w count
	app.Update(press('v'))
	colReset(app)
	app.Update(press('2'))
	app.Update(press('w'))
	if app.cursorCol != want {
		t.Fatalf("visual 2w 后 cursorCol=%d, want %d", app.cursorCol, want)
	}
	if app.countN != 0 {
		t.Fatalf("visual 移动后 countN=%d, want 0", app.countN)
	}
}

// TestCountGRenderLock Final Review Critical#1:count+G 的光标必须经得起
// View() 渲染——buildLogLines 的 autoscroll 钉尾(autoscroll 时 cursor=末行)
// 会当帧摧毁跳行结果,字段级断言对此是盲区。
func TestCountGRenderLock(t *testing.T) {
	app := newTestApp()
	app.cursor = 0
	app.Update(press('1'))
	app.Update(press('0'))
	app.Update(press('G'))
	if app.cursor != 9 {
		t.Fatalf("10G 后 cursor=%d, want 9", app.cursor)
	}
	_ = app.View() // 渲染层钉尾在此触发
	if app.cursor != 9 {
		t.Fatalf("View() 后 cursor=%d, want 9(autoscroll 钉尾摧毁跳行)", app.cursor)
	}
}

// TestCountHL Final Review Important#2:3h/3l 双端对齐(fview keymap 白名单
// 含 cursorLeft/cursorRight,TUI 同步吃 count)。
func TestCountHL(t *testing.T) {
	app := newTestApp()
	app.cursor = 0
	app.Update(press('$')) // 激活字符光标并到消息尾
	tail := app.cursorCol
	app.Update(press('3'))
	app.Update(press('h'))
	if app.cursorCol != tail-3 {
		t.Fatalf("3h 后 cursorCol=%d, want %d", app.cursorCol, tail-3)
	}
	app.Update(press('2'))
	app.Update(press('l'))
	if app.cursorCol != tail-1 {
		t.Fatalf("2l 后 cursorCol=%d, want %d", app.cursorCol, tail-1)
	}
}

// TestPgCount tui-picker-unify 顺手清账(deferred#3):pgup/pgdown 乘 count 对齐 fview。
func TestPgCount(t *testing.T) {
	app := newTestApp()
	app.cursor = 10
	app.Update(press('2'))
	app.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	want := min(19, 10+app.visibleLines()*2)
	if app.cursor != want {
		t.Fatalf("2PgDn 后 cursor=%d want %d", app.cursor, want)
	}
}
