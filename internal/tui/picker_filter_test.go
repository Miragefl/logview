// picker 无框打字过滤测试(tui-picker-unify):全层挂词/子序列过滤/切层清词/
// C-u 清词/q 守卫——fview panel-search-unify r2/r3 语义移植。
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/Miragefl/logview/internal/frp"
)

func keyMsg(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

// StripANSIForTest 剥 ANSI 转义(高亮断言用,原文可比对)。
func StripANSIForTest(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			inEsc = true
		case inEsc && (r == 'm' || r == 'K'):
			inEsc = false
		case !inEsc:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func newFilterApp(tab int) *App {
	app := newTestApp()
	app.sourcePickerMode = true
	app.sourceTab = tab
	return app
}

func TestPickerFilterAllLayers(t *testing.T) {
	t.Run("k8s ctx 层打字进词+子序列过滤", func(t *testing.T) {
		app := newFilterApp(0)
		app.pickerK8sLevel = 0
		app.pickerContexts = []sourceCandidate{
			{label: "prod-cluster", value: "prod-cluster"},
			{label: "dev-box", value: "dev-box"},
		}
		app.Update(keyMsg("p"))
		app.Update(keyMsg("c")) // pc → prod-cluster(p…c)命中,dev-box 无 p
		if app.pickerDirFilter != "pc" {
			t.Fatalf("词=%q want pc", app.pickerDirFilter)
		}
		cands := app.visiblePickerCandidates()
		if len(cands) != 1 || cands[0].value != "prod-cluster" {
			t.Fatalf("子序列过滤后应剩 prod-cluster, got %v", cands)
		}
	})
	t.Run("FRP L0 打字过滤连接", func(t *testing.T) {
		app := newFilterApp(3)
		app.pickerFRPLevel = 0
		frpStore().UpsertConn(frp.Conn{Name: "office-frp", Type: frp.TypeTCP, ServerAddr: "a.com", ServerPort: 7000, RemotePort: 6000})
		frpStore().UpsertConn(frp.Conn{Name: "home-nas", Type: frp.TypeTCP, ServerAddr: "b.com", ServerPort: 7000, RemotePort: 6001})
		app.Update(keyMsg("h"))
		app.Update(keyMsg("f")) // hf → 无命中(home-nas 无 f?h-o-m-e:无 f)→office-frp 也无 f…改断言
		cands := app.visiblePickerCandidates()
		if app.pickerDirFilter != "hf" {
			t.Fatalf("词=%q want hf", app.pickerDirFilter)
		}
		if len(cands) != 0 {
			t.Fatalf("hf 两名都无序命中,应空, got %v", cands)
		}
		app.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		app.Update(keyMsg("o")) // 词 ho:home-nas(h..o)命中,office-frp 无 h
		if cands := app.visiblePickerCandidates(); len(cands) != 1 || cands[0].value != "home-nas" {
			t.Fatalf("ho → home-nas, got %v", cands)
		}
	})
	t.Run("切 tab 清词", func(t *testing.T) {
		app := newFilterApp(0)
		app.pickerK8sLevel = 0
		app.pickerContexts = []sourceCandidate{{label: "x", value: "x"}}
		app.Update(keyMsg("k"))
		if app.pickerDirFilter != "k" {
			t.Fatalf("词未进")
		}
		app.Update(tea.KeyMsg{Type: tea.KeyTab})
		if app.pickerDirFilter != "" {
			t.Fatalf("切 tab 应清词,got %q", app.pickerDirFilter)
		}
	})
	t.Run("C-u 清词", func(t *testing.T) {
		app := newFilterApp(0)
		app.pickerK8sLevel = 0
		app.pickerContexts = []sourceCandidate{{label: "x", value: "x"}}
		app.Update(keyMsg("k"))
		app.Update(keyMsg("8"))
		app.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
		if app.pickerDirFilter != "" {
			t.Fatalf("C-u 应清词,got %q", app.pickerDirFilter)
		}
	})
	t.Run("词非空 q 进词不退出(q 守卫)", func(t *testing.T) {
		app := newFilterApp(0)
		app.pickerK8sLevel = 0
		app.pickerContexts = []sourceCandidate{{label: "qdemo", value: "qdemo"}}
		app.Update(keyMsg("d"))
		app.Update(keyMsg("q")) // 词非空:dq 连续进词,q 不得触发"再按退出"
		if app.pickerDirFilter != "dq" || !app.sourcePickerMode {
			t.Fatalf("词=%q mode=%v:词非空 q 应进词且不退出", app.pickerDirFilter, app.sourcePickerMode)
		}
	})
	t.Run("候选行命中字符高亮", func(t *testing.T) {
		oldProfile := lipgloss.ColorProfile() // 无 tty 默认 Ascii 剥 ANSI(fieldnav 同款设定)
		lipgloss.SetColorProfile(termenv.ANSI256)
		defer lipgloss.SetColorProfile(oldProfile)
		out := renderCandidateList([]sourceCandidate{{label: "hardware", value: "hardware"}}, 0, nil, 8, "hde")
		if !strings.Contains(out, "\x1b[") {
			t.Fatalf("命中段应有 ANSI 样式包裹,got %q", out)
		}
		if plain := StripANSIForTest(out); !strings.Contains(plain, "hardware") {
			t.Fatalf("原文应保留(剥 ANSI 后含 hardware), got %q", plain)
		}
	})
}
