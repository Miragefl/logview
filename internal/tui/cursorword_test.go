// wordAt/cursorWord 测试(tui-word-field):wordAt 与 fview charMotion.test.ts
// wordAt 同表对齐(标点/空白返回空——验收 #6 定案,非 vim * 取邻词);cursorWord
// 四级优先(visual char 选区/visual line/字段域/message 光标词)。
package tui

import (
	"strings"
	"testing"
)

func TestWordAt(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		col  int
		want string
	}{
		{"词中取全词", "hello world", 7, "world"},
		{"词首同词", "hello world", 6, "world"},
		{"词尾同词", "hello world", 4, "hello"},
		{"标点返回空", "foo=bar", 3, ""},
		{"空白返回空", "a b", 1, ""},
		{"col 越界 clamp", "abc", 99, "abc"},
		{"col 负 clamp", "abc", -5, "abc"},
		{"空串", "", 0, ""},
		{"下划线词内", "err_timeout", 5, "err_timeout"},
		{"CJK 词", "中 文 log", 2, "文"},
	}
	for _, c := range cases {
		if got := wordAt(c.msg, c.col); got != c.want {
			t.Errorf("%s: wordAt(%q,%d)=%q want %q", c.name, c.msg, c.col, got, c.want)
		}
	}
}

func TestCursorWord(t *testing.T) {
	t.Run("message 光标词", func(t *testing.T) {
		app := newTestApp()
		app.cursor = 0
		app.Update(press('$')) // 激活光标列到 message 尾
		if w := app.cursorWord(); w != "message" {
			t.Fatalf("光标词=%q want message", w)
		}
	})
	t.Run("visual char 单行选区词", func(t *testing.T) {
		app := newTestApp()
		app.cursor = 0
		app.Update(press('$'))
		app.Update(press('0')) // 回 message 首
		app.Update(press('v'))
		app.Update(press('e')) // 词尾扩选:fixture 首词 2026(数字词)
		if w := app.cursorWord(); w != "2026" {
			t.Fatalf("char 选区词=%q want 2026", w)
		}
	})
	t.Run("visual char 跨行选区取首行子串", func(t *testing.T) {
		app := newTestApp()
		app.cursor = 0
		app.Update(press('$'))
		app.Update(press('0'))
		app.Update(press('v'))
		app.Update(press('j')) // 跨行扩选(同 fixture 行,消息相同;断言语义=锚行选区)
		w := app.cursorWord()
		if w == "" {
			t.Fatalf("跨行选区词不得为空")
		}
	})
	t.Run("visual line 选区词=锚行 message", func(t *testing.T) {
		app := newTestApp()
		app.cursor = 0
		app.Update(press('V'))
		if w := app.cursorWord(); !strings.Contains(w, "test message") {
			t.Fatalf("line 选区词应含锚行 message 全文, got %q", w)
		}
	})
	t.Run("无光标无选区返回空", func(t *testing.T) {
		app := newTestApp()
		app.cursorCol = -1
		if w := app.cursorWord(); w != "" {
			t.Fatalf("无光标词=%q want 空", w)
		}
	})
}

func TestPopupSeed(t *testing.T) {
	newApp := func() *App {
		app := newTestApp()
		app.cursor = 0
		app.Update(press('$')) // 光标尾词 message
		return app
	}
	t.Run("tab0 光标词预填搜索", func(t *testing.T) {
		app := newApp()
		app.openUnifiedPopup(0)
		if app.searchInput != "message" {
			t.Fatalf("searchInput=%q want message", app.searchInput)
		}
		if app.searchCursor != len([]rune(app.searchInput)) {
			t.Fatalf("searchCursor=%d want %d", app.searchCursor, len([]rune(app.searchInput)))
		}
	})
	t.Run("tab1 光标词优先于已有词 join", func(t *testing.T) {
		app := newApp()
		app.highlights = []string{"err", "warn"}
		app.openUnifiedPopup(1)
		if app.highlightInput != "message" {
			t.Fatalf("highlightInput=%q want message(光标词优先)", app.highlightInput)
		}
	})
	t.Run("无光标词回落已有词 join(现状)", func(t *testing.T) {
		app := newTestApp()
		app.cursorCol = -1
		app.highlights = []string{"err", "warn"}
		app.openUnifiedPopup(1)
		if app.highlightInput != "err, warn" {
			t.Fatalf("highlightInput=%q want err, warn(回落 join)", app.highlightInput)
		}
	})
	t.Run("输入非空不覆盖", func(t *testing.T) {
		app := newApp()
		app.searchInput = "keepme"
		app.openUnifiedPopup(0)
		if app.searchInput != "keepme" {
			t.Fatalf("searchInput=%q want keepme(非空不覆盖)", app.searchInput)
		}
	})
	t.Run("tab2 隐藏光标词预填", func(t *testing.T) {
		app := newApp()
		app.openUnifiedPopup(2)
		if app.hideInput != "message" {
			t.Fatalf("hideInput=%q want message", app.hideInput)
		}
	})
}

// TestFRPPwdPrompt frp-pwd-focus(TUI 侧):user 空的 FRP 连接(合法配置,默认 root)
// 认证失败后密码框必须能弹——promptFRPPw 的 user=="" 守卫曾吞掉弹框,
// 错误上屏但无重输入口,即"登录失败不能重新输入密码"。
func TestFRPPwdPrompt(t *testing.T) {
	app := newTestApp()
	// FRP 浏览态:user 留空、隧道端口就绪
	app.pickerFRPLevel = 2
	app.pickerFRPPort = 12345
	app.pickerFRPHost = "127.0.0.1"
	app.pickerFRPUser = ""
	app.promptFRPPw()
	if !app.sshPwMode {
		t.Fatalf("user 空的 FRP 认证失败应弹密码框(sshPwMode=false 被守卫吞掉)")
	}
	if app.sshPwFRPPort != 12345 {
		t.Fatalf("隧道端口应暂存供 confirmSSHPw 回浏览,got %d", app.sshPwFRPPort)
	}
}
