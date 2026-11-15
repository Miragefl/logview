// 字段域导航测试(tui-word-field,对齐 fview field-interact):col0 再 h 跳最右栏/
// h/l 两端停/最右 l 回 message/message motion 回跳/j 复位/yy 复制栏位/渲染逆视频。
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Miragefl/logview/internal/model"
	"github.com/muesli/termenv"
)

// enrichFields fixture 是 plain-text 行(测试环境无 rules.yaml,AutoDetect 不解析),
// 字段域用例需要解析态:直接注入标准三字段(同包可写)。
func enrichFields(app *App) {
	for _, l := range app.viewLines() {
		if l.Fields == nil {
			l.Fields = map[model.Field]string{}
		}
		l.Fields[model.FieldLevel] = "INFO"
		l.Fields[model.FieldTime] = "2026-05-15 09:27:01.130"
		l.Fields[model.FieldSource] = "test-pod"
	}
}

// enterField 从 message col0 再按 h 进入字段域最右栏(默认 mask:0=time/1=source/2=level)。
func enterField(app *App) {
	enrichFields(app)
	app.cursor = 0
	app.Update(press('$'))
	app.Update(press('0'))
	app.Update(press('h'))
}

func TestFieldNav(t *testing.T) {
	t.Run("col0 再 h 跳最右栏,光标列熄灭", func(t *testing.T) {
		app := newTestApp()
		enterField(app)
		if app.fieldFocus != 2 {
			t.Fatalf("fieldFocus=%d want 2(level 最右栏)", app.fieldFocus)
		}
		if app.cursorCol != -1 {
			t.Fatalf("cursorCol=%d want -1(字段域熄灭)", app.cursorCol)
		}
	})
	t.Run("字段域 h/l 两端停", func(t *testing.T) {
		app := newTestApp()
		enterField(app)
		app.Update(press('h'))
		app.Update(press('h')) // 2→1→0,第三次停 0
		app.Update(press('h'))
		if app.fieldFocus != 0 {
			t.Fatalf("h 停最左:fieldFocus=%d want 0", app.fieldFocus)
		}
		app.Update(press('l'))
		app.Update(press('l')) // 0→1→2 停最右(第三次是回 message,另一用例锁)
		if app.fieldFocus != 2 {
			t.Fatalf("l 停最右:fieldFocus=%d want 2", app.fieldFocus)
		}
	})
	t.Run("最右栏 l 回 message col0", func(t *testing.T) {
		app := newTestApp()
		enterField(app)
		app.Update(press('l'))
		if app.fieldFocus != -1 || app.cursorCol != 0 {
			t.Fatalf("最右 l:fieldFocus=%d cursorCol=%d want -1/0", app.fieldFocus, app.cursorCol)
		}
	})
	t.Run("字段域 message motion 回跳(w 生效)", func(t *testing.T) {
		app := newTestApp()
		enterField(app)
		app.Update(press('w'))
		if app.fieldFocus != -1 {
			t.Fatalf("w 后 fieldFocus=%d want -1", app.fieldFocus)
		}
		if app.cursorCol <= 0 {
			t.Fatalf("w 应在 message 域前进,cursorCol=%d", app.cursorCol)
		}
	})
	t.Run("j 切行字段域保持(逐行扫一列,fview 同款)", func(t *testing.T) {
		app := newTestApp()
		enterField(app)
		app.Update(press('j'))
		if app.fieldFocus != 2 {
			t.Fatalf("j 后 fieldFocus=%d want 2(跨行保持)", app.fieldFocus)
		}
	})
	t.Run("yy 复制栏位原始值", func(t *testing.T) {
		app := newTestApp()
		enterField(app) // 最右栏=level
		app.Update(press('y'))
		app.Update(press('y'))
		if !strings.Contains(app.yankMsg, "INFO") {
			t.Fatalf("yy 应复制栏位值 INFO,yankMsg=%q", app.yankMsg)
		}
	})
	t.Run("字段域光标词=整字段原始值", func(t *testing.T) {
		app := newTestApp()
		enterField(app) // level=INFO
		if w := app.cursorWord(); w != "INFO" {
			t.Fatalf("字段域光标词=%q want INFO", w)
		}
	})
	t.Run("渲染逆视频(wrap 与非 wrap)", func(t *testing.T) {
		oldProfile := lipgloss.ColorProfile() // 测试无 tty,profile 默认 Ascii 剥 ANSI(charnav_test 同款设定)
		lipgloss.SetColorProfile(termenv.ANSI256)
		defer lipgloss.SetColorProfile(oldProfile)
		app := newTestApp()
		enterField(app)
		if v := app.View(); !strings.Contains(v, "\x1b[7m") {
			t.Fatalf("非 wrap 渲染应含逆视频 ANSI")
		}
		app.wrapMode = true
		if v := app.View(); !strings.Contains(v, "\x1b[7m") {
			t.Fatalf("wrap 渲染应含逆视频 ANSI")
		}
	})
}

// TestFieldNavFixes Final Review 修复轮:#1 v 进 visual 清字段域/#2 合并 yy 直达
// 分派字段域/#3 visual 下 /、C-h、C-x 透传弹窗(选区词预填可达)。
func TestFieldNavFixes(t *testing.T) {
	t.Run("v 进 visual 清字段域(双态不并存)", func(t *testing.T) {
		app := newTestApp()
		enterField(app)
		app.Update(press('v'))
		if app.fieldFocus != -1 {
			t.Fatalf("v 后 fieldFocus=%d want -1(清字段态)", app.fieldFocus)
		}
	})
	t.Run("合并输入 yy 直达也走字段域栏位", func(t *testing.T) {
		app := newTestApp()
		enterField(app)
		app.Update(press('y', 'y')) // 单条 KeyRunes 连按合并形态
		if !strings.Contains(app.yankMsg, "INFO") || strings.Contains(app.yankMsg, "2026-05-15 09:27") {
			t.Fatalf("合并 yy 应复制栏位值 INFO(非整行),yankMsg=%q", app.yankMsg)
		}
	})
	t.Run("visual 下 / 透传搜索弹窗且预填选区词", func(t *testing.T) {
		app := newTestApp()
		app.cursor = 0
		app.Update(press('$'))
		app.Update(press('0'))
		app.Update(press('v'))
		app.Update(press('e')) // 选区首词 2026
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
		if !app.searchMode {
			t.Fatalf("visual 下 / 应打开搜索弹窗")
		}
		if app.searchInput != "2026" {
			t.Fatalf("选区词预填 searchInput=%q want 2026", app.searchInput)
		}
	})
	t.Run("yy 空字段不进剪贴板报空值未复制", func(t *testing.T) {
		app := newTestApp()
		enterField(app)
		app.fieldFocus = 0 // time 栏(enrich 过)——换 thread 栏更纯:直接指不可见空栏不可得,用 Fields 删空
		vl := app.viewLines()
		delete(vl[0].Fields, model.FieldTime)
		app.Update(press('y'))
		app.Update(press('y'))
		if !strings.Contains(app.yankMsg, "未复制") {
			t.Fatalf("空值 yy 应提示未复制,yankMsg=%q", app.yankMsg)
		}
	})
}
