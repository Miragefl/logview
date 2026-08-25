package tui

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justfun/logview/internal/frp"
)

// fakeFRPTunnel 测试用隧道句柄（记录 Cleanup 调用）。
type fakeFRPTunnel struct {
	port    int
	cleaned bool
}

func (f *fakeFRPTunnel) LocalPort() int { return f.port }
func (f *fakeFRPTunnel) Cleanup() error { f.cleaned = true; return nil }

// setupFRPStore 注入临时 frp store；隔离 HOME 防历史 usage.json 频次影响排序断言。
// cleanup 时清 usage 缓存：避免本测试的临时 HOME 缓存残留，导致后续测试 BumpUsage
// 基于空缓存写盘、覆盖丢失全局 usage.json 的历史频次（曾使 TestSSHCandidatesHotSorted flakes）。
func setupFRPStore(t *testing.T, conns ...frp.Conn) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	resetUsageForTest()
	t.Cleanup(resetUsageForTest)
	frp.SetStoreFileForTest(filepath.Join(t.TempDir(), "frp.json"))
	t.Cleanup(frp.ResetStoreForTest)
	t.Cleanup(func() { frpStoreRef = nil })
	st := frp.LoadStore()
	st.UpsertServer(frp.Server{Name: "s1", Addr: "frps.example.com:7000", Token: "tk"})
	for _, c := range conns {
		st.UpsertConn(c)
	}
	SetFRPStore(st)
}

// FRP tab L0：+ 新建连接恒在首位，已存记录随后（无频次时按名称序）；搜索过滤。
func TestFRPTabConnectionList(t *testing.T) {
	setupFRPStore(t,
		frp.Conn{Name: "client-a", Server: "s1", SK: "k", Proxy: "ssh-a", User: "root", Path: "/var/log/a.log"},
		frp.Conn{Name: "client-b", Server: "s1", SK: "k", Proxy: "ssh-b", User: "root", Path: "/var/log/b.log"},
	)
	app := newTestApp()
	app.openSourcePicker(3)

	cands := app.visiblePickerCandidates()
	if len(cands) != 3 || cands[0].value != "+new" || cands[1].value != "client-a" {
		t.Fatalf("L0 应为 [+new, client-a, client-b]，实际 %v", cands)
	}
	// 搜索过滤
	app.pickerFRPInput = "client-b"
	cands = app.visiblePickerCandidates()
	if len(cands) != 2 || cands[1].value != "client-b" {
		t.Fatalf("过滤后应只剩 client-b，实际 %v", cands)
	}
	app.pickerFRPInput = ""
}

// 4 tab 循环：Tab 4 次回原位，再 1 次到下一个 tab（% 4 生效）。
func TestSourcePickerFourTabs(t *testing.T) {
	app := newTestApp()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	for i := 0; i < 4; i++ {
		app.Update(tea.KeyMsg{Type: tea.KeyTab})
	}
	if app.sourceTab != 0 {
		t.Fatalf("四次 Tab 应回到 0，实际 %d", app.sourceTab)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyTab})
	if app.sourceTab != 1 {
		t.Fatalf("五次 Tab 应到 1（说明 %% 4 生效），实际 %d", app.sourceTab)
	}
}

// 关闭选择器时应清理未移交的 frp 隧道。
func TestCloseSourcePickerCleansFRPTunnel(t *testing.T) {
	app := newTestApp()
	app.openSourcePicker(3)
	fake := &fakeFRPTunnel{port: 6022}
	app.pickerFRPTunnel = fake
	app.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if app.sourcePickerMode {
		t.Fatal("Esc 应关闭选择器")
	}
	if !fake.cleaned {
		t.Fatal("关闭选择器应清理未移交的隧道")
	}
}
