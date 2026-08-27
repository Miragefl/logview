package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justfun/logview/internal/frp"
	"github.com/justfun/logview/internal/stream"
)

// fakeFRPTunnel 测试用隧道句柄（记录 Cleanup 调用）。
type fakeFRPTunnel struct {
	port    int
	cleaned bool
}

func (f *fakeFRPTunnel) LocalPort() int { return f.port }
func (f *fakeFRPTunnel) Cleanup() error { f.cleaned = true; return nil }

// fakeFRPTunnelLog 可选实现 RecentLog（连接层错误诊断）。
type fakeFRPTunnelLog struct {
	fakeFRPTunnel
	log string
}

func (f *fakeFRPTunnelLog) RecentLog() string { return f.log }

// setupFRPStore 注入临时 frp store，返回 store 文件路径（落盘断言用）；
// 隔离 HOME 防历史 usage.json 频次影响排序断言。
// cleanup 时清 usage 缓存：避免本测试的临时 HOME 缓存残留，导致后续测试 BumpUsage
// 基于空缓存写盘、覆盖丢失全局 usage.json 的历史频次（曾使 TestSSHCandidatesHotSorted flakes）。
func setupFRPStore(t *testing.T, conns ...frp.Conn) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	resetUsageForTest()
	t.Cleanup(resetUsageForTest)
	p := filepath.Join(t.TempDir(), "frp.json")
	frp.SetStoreFileForTest(p)
	t.Cleanup(frp.ResetStoreForTest)
	t.Cleanup(func() { frpStoreRef = nil })
	st := frp.LoadStore()
	st.UpsertServer(frp.Server{Name: "s1", Addr: "frps.example.com:7000", Token: "tk"})
	for _, c := range conns {
		st.UpsertConn(c)
	}
	SetFRPStore(st)
	return p
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
	// label 只展示 proxy + 上次访问目录（Name 重复是噪音）
	if cands[1].label != "ssh-a  /var/log/a.log" {
		t.Fatalf("记录 label 应为 proxy + 目录，实际 %q", cands[1].label)
	}
	// 搜索过滤（按 proxy 名）
	app.pickerFRPInput = "ssh-b"
	cands = app.visiblePickerCandidates()
	if len(cands) != 2 || cands[1].value != "client-b" {
		t.Fatalf("过滤后应只剩 client-b，实际 %v", cands)
	}
	// 无 Path 记录占位提示
	setupFRPStore(t, frp.Conn{Name: "x", Server: "s1", SK: "k", Proxy: "ssh-x", User: "root"})
	app2 := newTestApp()
	app2.openSourcePicker(3)
	if c := app2.visiblePickerCandidates(); c[1].label != "ssh-x  （未选过日志）" {
		t.Fatalf("无 Path 应显示占位，实际 %q", c[1].label)
	}
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
	if cands[1].label != "s1  frps.example.com:7000" {
		t.Fatalf("命名服务器 label 应为 名 + 地址，实际 %q", cands[1].label)
	}
	// Name = Addr（手动输入创建）时同行不重复
	frpStore().UpsertServer(frp.Server{Name: "frps.x.com:7000", Addr: "frps.x.com:7000"})
	if c := app.visiblePickerCandidates(); c[len(c)-1].label != "frps.x.com:7000" {
		t.Fatalf("Name=Addr 应只显示一个，实际 %q", c[len(c)-1].label)
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
	app.Update(frpTunnelMsg{conn: conn, tunnel: fake})
	if app.pickerFRPLevel != 2 || app.pickerFRPTunnel != fake || !app.pickerLoading {
		t.Fatalf("隧道建立应进 L2 且 loading，实际 level=%d loading=%v", app.pickerFRPLevel, app.pickerLoading)
	}
	if app.pickerFRPUser != "root" || app.pickerFRPDir != "/" {
		t.Fatalf("user/dir 应就位（无 Path 起始根目录）: %q %q", app.pickerFRPUser, app.pickerFRPDir)
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

// frpBrowseRoot 起始目录：已存 Path 取父目录；Path 空/根/单段 从根开始。
func TestFRPBrowseRoot(t *testing.T) {
	cases := map[string]string{
		"/var/log/a.log": "/var/log",
		"/a.log":         "/",
		"/var/log/":      "/var",
		"":               "/",
		"/":              "/",
	}
	for in, want := range cases {
		if got := frpBrowseRoot(in); got != want {
			t.Errorf("frpBrowseRoot(%q) = %q, want %q", in, got, want)
		}
	}
}

// 表单 step5 提交即保存记录（此前仅选文件确认时保存，中途放弃配置就丢）。
func TestFRPFormSubmitSavesConnImmediately(t *testing.T) {
	p := setupFRPStore(t)
	app := newTestApp()
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // +new → step0
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 选 s1 → step3
	typeRunes(app, "sk1")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	typeRunes(app, "ssh-z")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	typeRunes(app, "root")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // step5 提交

	c, ok := frpStore().FindConn("ssh-z")
	if !ok || c.Server != "s1" || c.SK != "sk1" || c.Proxy != "ssh-z" || c.User != "root" {
		t.Fatalf("提交后应立即保存记录，实际 %+v ok=%v", c, ok)
	}
	// 落盘验证：重载 store 仍在
	frp.ResetStoreForTest()
	frp.SetStoreFileForTest(p)
	if _, ok := frp.LoadStore().FindConn("ssh-z"); !ok {
		t.Fatal("记录应已落盘")
	}
}

// L0 选已存记录：建隧道后统一进目录浏览，起始目录 = 已存 Path 父目录。
func TestFRPDirectRecordBrowses(t *testing.T) {
	setupFRPStore(t, frp.Conn{Name: "client-a", Server: "s1", SK: "sk1", Proxy: "ssh-a", User: "root", Path: "/var/log/a.log"})
	fakeSSHBin(t)
	app := newTestApp()
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyDown}) // 光标到 client-a
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !app.pickerLoading {
		t.Fatal("选已存记录应进入 loading 等隧道")
	}
	fake := &fakeFRPTunnel{port: 6022}
	app.Update(frpTunnelMsg{
		conn:   frp.Conn{Name: "client-a", Server: "s1", SK: "sk1", Proxy: "ssh-a", User: "root", Path: "/var/log/a.log"},
		tunnel: fake,
	})
	if !app.sourcePickerMode || app.pickerFRPLevel != 2 {
		t.Fatalf("应进 L2 目录浏览，实际 mode=%v level=%d", app.sourcePickerMode, app.pickerFRPLevel)
	}
	if app.pickerFRPDir != "/var/log" {
		t.Fatalf("起始目录应为已存 Path 父目录 /var/log，实际 %s", app.pickerFRPDir)
	}
	if fake.cleaned {
		t.Fatal("浏览隧道不应被清理")
	}
}

func TestFRPTailPasswordPromptAndRestart(t *testing.T) {
	setupFRPStore(t)
	fakeSSHBin(t)
	app := newTestApp()
	fake := &fakeFRPTunnel{port: 6022}
	src := stream.NewFRPSource("client-a", fake, "root", "/var/log/a.log", 100)
	app.stream = src

	// tail 流认证失败 → 弹密码框
	app.appendErrorLine("ERROR ssh: permission denied")
	if !app.sshPwMode {
		t.Fatal("Permission denied 应弹密码框")
	}
	if app.sshPwHost != "frp:client-a" {
		t.Fatalf("密码框主机应为 frp:client-a，实际 %s", app.sshPwHost)
	}
	// 输入密码 → 原源重启（隧道不清理）
	app.sshPwInput = "pw123"
	app.confirmSSHPw()
	if src.Password() != "pw123" {
		t.Fatal("密码应写入 FRPSource")
	}
	if fake.cleaned {
		t.Fatal("密码重连不应清理隧道")
	}
	if app.sshPasswords["frp:client-a"] != "pw123" {
		t.Fatal("密码应入内存缓存")
	}
}

// enterFRPPwState 构造 frp 目录浏览认证失败 → 密码弹窗态（隧道保留）。
// ServerName/SK 模拟 L0 选已存记录直达时就位（真实时序：浏览前已定）。
func enterFRPPwState(app *App, fake *fakeFRPTunnel) {
	app.openSourcePicker(3)
	app.pickerFRPLevel = 2
	app.pickerFRPTunnel = fake
	app.pickerFRPServerName = "s1"
	app.pickerFRPSK = "sk1"
	app.pickerFRPUser = "root"
	app.pickerFRPDir = "/var/log"
	app.pickerFRPProxy = "ssh-x"
	app.pickerFRPConnName = "ssh-x"
	app.Update(candidatesMsg{tab: 3, kind: "frpdir", ns: "frp:/var/log",
		err: fmt.Errorf("ssh: permission denied, please try again")})
}

// 防回归：frp 端口随机分配导致 host key 校验失败（非 permission denied）——
// 错误必须上屏（曾静默显示"目录为空"），不弹密码框；成功回填清空错误。
func TestFRPDirErrorSurfacesWithoutPwPrompt(t *testing.T) {
	setupFRPStore(t)
	app := newTestApp()
	app.openSourcePicker(3)
	app.pickerFRPLevel = 2
	fake := &fakeFRPTunnel{port: 6022}
	app.pickerFRPTunnel = fake
	app.pickerFRPUser = "root"
	app.pickerFRPDir = "/"

	app.Update(candidatesMsg{tab: 3, kind: "frpdir", ns: "frp:/",
		err: fmt.Errorf("ssh ls root@127.0.0.1: Host key verification failed.")})
	if app.sshPwMode {
		t.Fatal("host key 失败不应弹密码框")
	}
	if app.pickerFRPErr != "ssh ls root@127.0.0.1: Host key verification failed." {
		t.Fatalf("错误应上屏，实际 %q", app.pickerFRPErr)
	}
	// 多行 stderr 只取首行
	app.Update(candidatesMsg{tab: 3, kind: "frpdir", ns: "frp:/",
		err: fmt.Errorf("x: line1\nline2\nline3")})
	if app.pickerFRPErr != "x: line1" {
		t.Fatalf("多行错误取首行，实际 %q", app.pickerFRPErr)
	}
	// 成功回填清空
	app.Update(candidatesMsg{tab: 3, kind: "frpdir", ns: "frp:/",
		items: []sourceCandidate{{label: "app.log", value: "app.log"}}})
	if app.pickerFRPErr != "" {
		t.Fatalf("成功回填应清空错误，实际 %q", app.pickerFRPErr)
	}
}

// 连接层错误（reset/kex）：附带 frpc 日志尾（真因在 frpc 输出，ssh 只见 reset）。
func TestFRPDirConnErrAppendsFRPCLog(t *testing.T) {
	setupFRPStore(t)
	app := newTestApp()
	app.openSourcePicker(3)
	app.pickerFRPLevel = 2
	fake := &fakeFRPTunnelLog{log: "E proxy [ssh-a] not found"}
	fake.port = 6022
	app.pickerFRPTunnel = fake
	app.pickerFRPUser = "root"
	app.pickerFRPDir = "/"

	app.Update(candidatesMsg{tab: 3, kind: "frpdir", ns: "frp:/",
		err: fmt.Errorf("ssh ls root@127.0.0.1: kex_exchange_identification: read: Connection reset by peer")})
	want := "ssh ls root@127.0.0.1: kex_exchange_identification: read: Connection reset by peer | frpc: E proxy [ssh-a] not found"
	if app.pickerFRPErr != want {
		t.Fatalf("连接层错误应附带 frpc 日志，实际:\n%q\nwant:\n%q", app.pickerFRPErr, want)
	}
	// 非连接层错误（如命令失败）不附带
	app.Update(candidatesMsg{tab: 3, kind: "frpdir", ns: "frp:/",
		err: fmt.Errorf("ssh ls root@127.0.0.1: some other failure")})
	if strings.Contains(app.pickerFRPErr, "frpc:") {
		t.Fatalf("非连接层错误不应附带 frpc 日志，实际 %q", app.pickerFRPErr)
	}
	// 隧道未实现 RecentLog（fakeFRPTunnel）：只显示原始错误，不崩
	app2 := newTestApp()
	app2.openSourcePicker(3)
	app2.pickerFRPLevel = 2
	app2.pickerFRPTunnel = &fakeFRPTunnel{port: 6022}
	app2.pickerFRPUser = "root"
	app2.pickerFRPDir = "/"
	app2.Update(candidatesMsg{tab: 3, kind: "frpdir", ns: "frp:/",
		err: fmt.Errorf("ssh ls root@127.0.0.1: Connection reset by peer")})
	if want := "ssh ls root@127.0.0.1: Connection reset by peer"; app2.pickerFRPErr != want {
		t.Fatalf("无 RecentLog 实现应只显示原始错误，实际 %q", app2.pickerFRPErr)
	}
}

// frpc 典型真因翻译：custom listener 不存在 → 附人话提示（proxy 名核对/远端在线）。
func TestFRPDirErrHintsProxyNotFound(t *testing.T) {
	setupFRPStore(t)
	app := newTestApp()
	app.openSourcePicker(3)
	app.pickerFRPLevel = 2
	fake := &fakeFRPTunnelLog{log: "[W] dialRawVisitorConn error: start new visitor connection error: custom listener for [8C32230C66ED] doesn't exist"}
	fake.port = 6022
	app.pickerFRPTunnel = fake
	app.pickerFRPUser = "root"
	app.pickerFRPDir = "/"
	app.Update(candidatesMsg{tab: 3, kind: "frpdir", ns: "frp:/",
		err: fmt.Errorf("ssh ls root@127.0.0.1: kex_exchange_identification: read: Connection reset by peer")})
	if !strings.Contains(app.pickerFRPErr, "custom listener") || !strings.Contains(app.pickerFRPErr, "proxy 名在 frps 上不存在") {
		t.Fatalf("应附 proxy 不存在提示，实际 %q", app.pickerFRPErr)
	}
}

// frp 密码框 Esc 取消：浏览隧道一并清理，frp 端口标记复位（不劫持后续密码流程）。
func TestFRPPwEscCleansTunnel(t *testing.T) {
	setupFRPStore(t)
	app := newTestApp()
	fake := &fakeFRPTunnel{port: 6022}
	enterFRPPwState(app, fake)
	if !app.sshPwMode || app.sshPwFRPPort != 6022 {
		t.Fatalf("应进入 frp 密码弹窗态: mode=%v port=%d", app.sshPwMode, app.sshPwFRPPort)
	}

	app.Update(tea.KeyMsg{Type: tea.KeyEscape}) // Esc 取消 → closeSSHPw
	if app.sshPwMode {
		t.Fatal("Esc 应关闭密码框")
	}
	if !fake.cleaned {
		t.Fatal("Esc 取消应清理浏览中的 frp 隧道")
	}
	if app.pickerFRPTunnel != nil {
		t.Fatal("隧道句柄应复位")
	}
	if app.sshPwFRPPort != 0 {
		t.Fatalf("sshPwFRPPort 应清零，实际 %d", app.sshPwFRPPort)
	}
}

// frp 密码确认后回目录层：ServerName/SK 应保留，confirmFRPPicker 保存的记录不缺服务器/SK。
func TestFRPPwConfirmKeepsServerForRecord(t *testing.T) {
	setupFRPStore(t)
	fakeSSHBin(t) // confirmFRPPicker 建 FRPSource，避免真连
	app := newTestApp()
	fake := &fakeFRPTunnel{port: 6022}
	enterFRPPwState(app, fake)
	if !app.sshPwMode {
		t.Fatal("应进入 frp 密码弹窗态")
	}

	app.sshPwInput = "pw789"
	if cmd := app.confirmSSHPw(); cmd == nil {
		t.Fatal("确认密码应返回重拉目录的 cmd")
	}
	if app.pickerFRPServerName != "s1" || app.pickerFRPSK != "sk1" {
		t.Fatalf("确认后应保留服务器/SK: server=%q sk=%q", app.pickerFRPServerName, app.pickerFRPSK)
	}

	// 回目录层后选文件确认 → 记录整体保存（Server/SK 非空）
	app.pickerCandidates = []sourceCandidate{{label: "app.log", value: "app.log"}}
	app.pickerLoading = false
	app.Update(tea.KeyMsg{Type: tea.KeyDown})  // 跳过首位 ../
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 选中 app.log → confirmFRPPicker
	c, ok := frpStore().FindConn("ssh-x")
	if !ok || c.Server != "s1" || c.SK != "sk1" {
		t.Fatalf("记录应含服务器/SK，实际 %+v ok=%v", c, ok)
	}
}

// 回归（终审 F1）：表单建连 A 浏览后退回 L0 再 "+new" 建连 B，残留 ConnName 必须作废。
// 否则：① frpPwKey 错位（B 的密码缓存走 A 的 key）；② confirmFRPPicker 以旧名 UpsertConn，
// ssh-x 旧记录被 B 的参数整体覆盖；③ 迟到 frpTunnelMsg 的 conn.Name 不生效。
func TestFRPNewFormClearsStaleConnName(t *testing.T) {
	setupFRPStore(t, frp.Conn{Name: "ssh-x", Server: "s1", SK: "sk1", Proxy: "ssh-x", User: "root", Path: "/var/log/x.log"})
	fakeSSHBin(t) // confirmFRPPicker 建 FRPSource，避免真连
	app := newTestApp()
	app.openSourcePicker(3)

	// 第一次建连 A（表单 proxy=ssh-x）：提交后隧道建立，browse=true 进 L2 落位 ConnName
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // +new → step0
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 选 s1 → step3
	typeRunes(app, "sk1")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	typeRunes(app, "ssh-x")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	typeRunes(app, "root")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // step5 提交 → loading
	fake1 := &fakeFRPTunnel{port: 6022}
	app.Update(frpTunnelMsg{conn: frp.Conn{Name: "ssh-x", Server: "s1", SK: "sk1", Proxy: "ssh-x", User: "root"},
		tunnel: fake1})
	if app.pickerFRPConnName != "ssh-x" {
		t.Fatalf("前置：建连 A 后 ConnName 应为 ssh-x，实际 %q", app.pickerFRPConnName)
	}
	// 目录列表回填（清 loading）→ 根目录 Backspace → closeFRPBrowse 回 L0
	app.Update(candidatesMsg{tab: 3, kind: "frpdir", ns: "frp:/",
		items: []sourceCandidate{{label: "app.log", value: "app.log"}}})
	app.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if app.pickerFRPLevel != 0 || app.pickerFRPTunnel != nil {
		t.Fatalf("Backspace 应回 L0 并清隧道，实际 level=%d tunnel=%v", app.pickerFRPLevel, app.pickerFRPTunnel)
	}

	// "+new" Enter 进表单：残留 ConnName 必须清空（旧记录名作废）
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerFRPLevel != 1 || app.pickerFRPStep != 0 {
		t.Fatalf("应进表单 step0，实际 level=%d step=%d", app.pickerFRPLevel, app.pickerFRPStep)
	}
	if app.pickerFRPConnName != "" {
		t.Fatalf("+new 进表单应清除残留 ConnName，实际 %q", app.pickerFRPConnName)
	}

	// 走表单建连 B（proxy=ssh-y）：选 s1 → sk → proxy → user 提交
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 选 s1 → step3
	typeRunes(app, "sk2")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	typeRunes(app, "ssh-y")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	typeRunes(app, "root")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // step5 提交 → loading
	if !app.pickerLoading {
		t.Fatal("表单提交后应进入 loading 等隧道")
	}

	// 隧道建立（browse=true）：conn.Name 应无条件落位，密码 key 跟随 B
	fake2 := &fakeFRPTunnel{port: 6023}
	app.Update(frpTunnelMsg{conn: frp.Conn{Name: "ssh-y", Server: "s1", SK: "sk2", Proxy: "ssh-y", User: "root"},
		tunnel: fake2})
	if app.pickerFRPConnName != "ssh-y" {
		t.Fatalf("建连 B 后 ConnName 应为 ssh-y，实际 %q", app.pickerFRPConnName)
	}
	if got := app.frpPwKey(); got != "frp:ssh-y" {
		t.Fatalf("frpPwKey 应为 frp:ssh-y，实际 %q", got)
	}

	// 确认路径：选中文件 → confirmFRPPicker 应新建 ssh-y 记录，ssh-x 旧记录不被覆盖
	app.pickerFRPDir = "/var/log"
	app.pickerCandidates = []sourceCandidate{{label: "app.log", value: "app.log"}}
	app.pickerLoading = false
	app.Update(tea.KeyMsg{Type: tea.KeyDown})  // 跳过首位 ../
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 选中 app.log → confirmFRPPicker
	if app.sourcePickerMode {
		t.Fatal("确认后应关闭选择器")
	}
	old, ok := frpStore().FindConn("ssh-x")
	if !ok || old.Server != "s1" || old.SK != "sk1" || old.Proxy != "ssh-x" || old.Path != "/var/log/x.log" {
		t.Fatalf("ssh-x 旧记录不应被建连 B 覆盖，实际 %+v ok=%v", old, ok)
	}
	rec, ok := frpStore().FindConn("ssh-y")
	if !ok || rec.Server != "s1" || rec.SK != "sk2" || rec.Proxy != "ssh-y" || rec.User != "root" || rec.Path != "/var/log/app.log" {
		t.Fatalf("ssh-y 应保存为新记录，实际 %+v ok=%v", rec, ok)
	}
}

func TestFRPBrowsePasswordPrompt(t *testing.T) {
	setupFRPStore(t)
	app := newTestApp()
	fake := &fakeFRPTunnel{port: 6022}
	app.openSourcePicker(3)
	app.pickerFRPLevel = 2
	app.pickerFRPTunnel = fake
	app.pickerFRPUser = "root"
	app.pickerFRPDir = "/var/log"
	app.pickerFRPProxy = "ssh-x"
	app.pickerFRPConnName = "ssh-x"

	// 目录拉取认证失败 → 弹密码框（隧道保留）
	app.Update(candidatesMsg{tab: 3, kind: "frpdir", ns: "frp:/var/log",
		err: fmt.Errorf("ssh: permission denied, please try again")})
	if !app.sshPwMode {
		t.Fatal("frp 目录认证失败应弹密码框")
	}
	if app.pickerFRPTunnel != fake {
		t.Fatal("密码流程应保留隧道")
	}
	// 确认密码 → 回到 FRP 目录层
	app.sshPwInput = "pw456"
	cmd := app.confirmSSHPw()
	if cmd == nil {
		t.Fatal("确认密码应返回重拉目录的 cmd")
	}
	if app.pickerFRPLevel != 2 || app.pickerFRPTunnel != fake || !app.pickerLoading {
		t.Fatalf("应回到 L2 且 loading: level=%d loading=%v", app.pickerFRPLevel, app.pickerLoading)
	}
}

// C-x 删除：L0 删连接记录（+new 不可删、删除后落盘光标收敛）；
// L1 step0 删服务器（被引用时拒绝、+manual 不可删、无引用时可删）。
func TestSourcePickerFRPDelete(t *testing.T) {
	p := setupFRPStore(t,
		frp.Conn{Name: "client-a", Server: "s1", SK: "k", Proxy: "ssh-a", User: "root", Path: "/var/log/a.log"},
		frp.Conn{Name: "client-b", Server: "s1", SK: "k", Proxy: "ssh-b", User: "root", Path: "/var/log/b.log"},
	)
	app := newTestApp()
	app.openSourcePicker(3)

	// 光标在 +new：C-x 不删
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if len(frpStore().Conns) != 2 {
		t.Fatal("+new 上 C-x 不应删除")
	}
	// 下移到 client-a：C-x 删除并落盘
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if _, ok := frpStore().FindConn("client-a"); ok {
		t.Fatal("C-x 应删除 client-a")
	}
	frp.ResetStoreForTest()
	frp.SetStoreFileForTest(p)
	frpStoreRef = frp.LoadStore()
	if _, ok := frpStore().FindConn("client-a"); ok {
		t.Fatal("删除应已落盘")
	}
	if _, ok := frpStore().FindConn("client-b"); !ok {
		t.Fatal("client-b 应保留")
	}

	// L1 step0：s1 被 client-b 引用 → C-x 拒绝（先 C-k 回 +new 进表单）
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // +new → L1 step0
	if app.pickerFRPLevel != 1 || app.pickerFRPStep != 0 {
		t.Fatalf("应进 L1 step0，level=%d step=%d", app.pickerFRPLevel, app.pickerFRPStep)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlJ}) // 光标到 s1
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if len(frpStore().Servers) != 1 {
		t.Fatal("被引用的服务器不应被删除")
	}

	// 删掉 client-b 后：s1 可删，列表只剩 +manual
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlJ}) // client-b
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // +new → L1 step0
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlJ}) // 光标到 s1
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if len(frpStore().Servers) != 0 {
		t.Fatal("无引用的服务器应可删除")
	}
	cands := app.visiblePickerCandidates()
	if len(cands) != 1 || cands[0].value != "+manual" {
		t.Fatalf("删空后应只剩 +manual，实际 %v", cands)
	}
}
