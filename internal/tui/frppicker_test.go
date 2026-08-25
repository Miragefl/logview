package tui

import (
	"fmt"
	"os"
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

// typeRunes 逐字符向 app 发送 KeyRunes（模拟键盘输入）。
func typeRunes(app *App, s string) {
	for _, r := range s {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

// 表单 step0：+manual 恒在首位，已存服务器随后；选已存服务器直达 step3(sk)，
// Backspace 逐级返回（未走过的 step1/2 跳过）。
func TestFRPFormServerSelect(t *testing.T) {
	setupFRPStore(t)
	app := newTestApp()
	app.openSourcePicker(3)
	// L0 Enter（光标 0 = + 新建连接）→ 表单 step 0
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerFRPLevel != 1 || app.pickerFRPStep != 0 {
		t.Fatalf("应进入表单 step0，实际 level=%d step=%d", app.pickerFRPLevel, app.pickerFRPStep)
	}
	cands := app.visiblePickerCandidates()
	if len(cands) != 2 || cands[0].value != "+manual" || cands[1].value != "s1" {
		t.Fatalf("step0 候选应为 [+manual, s1]，实际 %v", cands)
	}
	// 选 s1（光标 1）→ 直接跳 sk（step 3）
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerFRPStep != 3 || app.pickerFRPServerName != "s1" {
		t.Fatalf("选已存服务器应到 step3(sk)，实际 step=%d server=%q", app.pickerFRPStep, app.pickerFRPServerName)
	}
	// Backspace 回 step0，再 Backspace 回 L0
	app.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if app.pickerFRPStep != 0 {
		t.Fatalf("Backspace 应回 step0，实际 %d", app.pickerFRPStep)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if app.pickerFRPLevel != 0 {
		t.Fatalf("step0 Backspace 应回 L0，实际 %d", app.pickerFRPLevel)
	}
}

// 表单手动输入路径：地址 → token（保存服务器）→ sk → proxy → user（空值不提交）。
func TestFRPFormNewServerAndFields(t *testing.T) {
	setupFRPStore(t)
	app := newTestApp()
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // +new → step0
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 光标 0 = +manual → step1
	if app.pickerFRPStep != 1 {
		t.Fatalf("应到 step1(地址)，实际 %d", app.pickerFRPStep)
	}
	typeRunes(app, "frps2.example.com:7000")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step2 token
	typeRunes(app, "tk2")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step3，保存服务器
	sv, ok := frpStore().FindServer("frps2.example.com:7000")
	if !ok || sv.Addr != "frps2.example.com:7000" || sv.Token != "tk2" {
		t.Fatalf("新服务器应已保存，实际 %+v ok=%v", sv, ok)
	}
	typeRunes(app, "sk1")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // step4
	typeRunes(app, "ssh-x")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // step5
	if app.pickerFRPSK != "sk1" || app.pickerFRPProxy != "ssh-x" {
		t.Fatalf("字段未暂存: sk=%q proxy=%q", app.pickerFRPSK, app.pickerFRPProxy)
	}
	// user 留空 Enter：不提交
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerFRPStep != 5 {
		t.Fatalf("user 为空不应提交，实际 step=%d", app.pickerFRPStep)
	}
}

// 手动输入地址中途返回 L0 后再选已存服务器：残留 ServerAddr 会使 step3 回退
// 跳级失效（回退走到 step2），空 token Enter 会用残留地址覆盖已存服务器记录。
func TestFRPFormBackspaceAfterManual(t *testing.T) {
	setupFRPStore(t)
	app := newTestApp()
	app.openSourcePicker(3)
	// Enter(+new)→step0，Enter(+manual)→step1，输入地址→step2（ServerAddr 残留）
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	typeRunes(app, "frps2.example.com:7000")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerFRPStep != 2 {
		t.Fatalf("应到 step2(token)，实际 %d", app.pickerFRPStep)
	}
	// Backspace×3：step2→step1→step0→L0
	for i := 0; i < 3; i++ {
		app.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	if app.pickerFRPLevel != 0 {
		t.Fatalf("三次 Backspace 应回 L0，实际 level=%d step=%d", app.pickerFRPLevel, app.pickerFRPStep)
	}
	// 重进表单，选已存服务器 s1 → 直达 step3
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // +new → step0
	app.Update(tea.KeyMsg{Type: tea.KeyDown})  // 光标 → s1
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerFRPStep != 3 || app.pickerFRPServerName != "s1" {
		t.Fatalf("选 s1 应到 step3(sk)，实际 step=%d server=%q", app.pickerFRPStep, app.pickerFRPServerName)
	}
	// step3 由选已存服务器直达（未走 step1/2）：Backspace 应跳级回 step0，而非 step2
	app.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if app.pickerFRPLevel != 1 || app.pickerFRPStep != 0 {
		t.Fatalf("step3 Backspace 应跳级回 step0，实际 level=%d step=%d", app.pickerFRPLevel, app.pickerFRPStep)
	}
	// 再走一遍同路径：s1 记录应保持 setup 写入的原值，未被残留地址覆盖
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 选 s1 → step3
	app.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	sv, ok := frpStore().FindServer("s1")
	if !ok || sv.Addr != "frps.example.com:7000" || sv.Token != "tk" {
		t.Fatalf("s1 记录应保持原值(frps.example.com:7000/tk)，实际 %+v ok=%v", sv, ok)
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

func TestFRPTunnelMsgBrowseEntersDirLevel(t *testing.T) {
	setupFRPStore(t)
	app := newTestApp()
	app.openSourcePicker(3)
	fake := &fakeFRPTunnel{port: 6022}
	conn := frp.Conn{Name: "ssh-x", Server: "s1", SK: "sk1", Proxy: "ssh-x", User: "root"}
	app.Update(frpTunnelMsg{conn: conn, tunnel: fake, browse: true})
	if app.pickerFRPLevel != 2 || app.pickerFRPTunnel != fake || !app.pickerLoading {
		t.Fatalf("browse=true 应进 L2 且 loading，实际 level=%d loading=%v", app.pickerFRPLevel, app.pickerLoading)
	}
	if app.pickerFRPUser != "root" || app.pickerFRPDir != "/" {
		t.Fatalf("user/dir 应就位: %q %q", app.pickerFRPUser, app.pickerFRPDir)
	}

	// 目录候选回填
	app.Update(candidatesMsg{tab: 3, kind: "frpdir", ns: "frp:/",
		items: []sourceCandidate{{label: "app.log", value: "app.log"}, {label: "sub/", value: "sub", dir: true}}})
	app.pickerLoading = false
	cands := app.visiblePickerCandidates()
	if len(cands) != 2 || cands[0].value != "app.log" {
		t.Fatalf("L2 候选不符: %v", cands)
	}

	// 进子目录
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerFRPDir != "/sub" {
		t.Fatalf("Enter 应进 /sub，实际 %s", app.pickerFRPDir)
	}
}

func TestFRPTunnelMsgErrorSurfaces(t *testing.T) {
	setupFRPStore(t)
	app := newTestApp()
	app.openSourcePicker(3)
	app.pickerLoading = true
	app.Update(frpTunnelMsg{err: fmt.Errorf("未找到 frpc")})
	if app.pickerLoading {
		t.Fatal("失败应清 loading")
	}
}

func TestFRPFormSubmitStartsTunnel(t *testing.T) {
	setupFRPStore(t)
	app := newTestApp()
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // +new → step0
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 选 s1 → step3
	typeRunes(app, "sk1")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	typeRunes(app, "ssh-x")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	typeRunes(app, "root")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // step5 提交
	if !app.pickerLoading {
		t.Fatal("表单提交后应进入 loading 等隧道")
	}
}

func fakeSSHBin(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := dir + "/ssh"
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 30\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
}

func TestFRPConfirmSavesRecordAndSwitchesStream(t *testing.T) {
	setupFRPStore(t)
	fakeSSHBin(t) // 阻塞式假 ssh，避免真连 127.0.0.1
	app := newTestApp()
	app.openSourcePicker(3)
	fake := &fakeFRPTunnel{port: 6022}
	app.pickerFRPLevel = 2
	app.pickerFRPTunnel = fake
	app.pickerFRPServerName = "s1"
	app.pickerFRPSK = "sk1"
	app.pickerFRPProxy = "ssh-x"
	app.pickerFRPUser = "root"
	app.pickerFRPConnName = "ssh-x"
	app.pickerFRPDir = "/var/log"
	app.pickerCandidates = []sourceCandidate{{label: "app.log", value: "app.log"}}
	app.pickerLoading = false

	app.Update(tea.KeyMsg{Type: tea.KeyDown}) // 光标跳过首位的 ../（非根目录时列表前置返回上级项）
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 选中 app.log → confirmFRPPicker
	if app.sourcePickerMode {
		t.Fatal("确认后应关闭选择器")
	}
	if fake.cleaned {
		t.Fatal("隧道应移交 FRPSource，不应被清理")
	}
	label := app.stream.Label()
	if label != "frp://ssh-x/var/log/app.log" {
		t.Fatalf("应切到 FRP 源，实际 %s", label)
	}
	// 记录已保存（含 Path）
	c, ok := frpStore().FindConn("ssh-x")
	if !ok || c.Path != "/var/log/app.log" || c.Server != "s1" || c.SK != "sk1" {
		t.Fatalf("记录应整体保存，实际 %+v ok=%v", c, ok)
	}
}

func TestFRPDirectRecordTail(t *testing.T) {
	setupFRPStore(t, frp.Conn{Name: "client-a", Server: "s1", SK: "sk1", Proxy: "ssh-a", User: "root", Path: "/var/log/a.log"})
	fakeSSHBin(t)
	app := newTestApp()
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyDown}) // 光标到 client-a
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !app.pickerLoading {
		t.Fatal("直达应进入 loading 等隧道")
	}
	fake := &fakeFRPTunnel{port: 6022}
	app.Update(frpTunnelMsg{
		conn:   frp.Conn{Name: "client-a", Server: "s1", SK: "sk1", Proxy: "ssh-a", User: "root", Path: "/var/log/a.log"},
		tunnel: fake, browse: false,
	})
	if app.sourcePickerMode {
		t.Fatal("直达建流后应关闭选择器")
	}
	if got := app.stream.Label(); got != "frp://client-a/var/log/a.log" {
		t.Fatalf("应直达 tail，实际 %s", got)
	}
}
