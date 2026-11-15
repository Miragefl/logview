package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Miragefl/logview/internal/frp"
	"github.com/Miragefl/logview/internal/stream"
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

// stcpConn 测试常用 stcp 连接。
func stcpConn(name string) frp.Conn {
	return frp.Conn{Name: name, Type: frp.TypeSTCP, ServerAddr: "frps.example.com", ServerPort: 7000,
		Token: "tk", ServerName: name, SecretKey: "sk1", User: "root"}
}

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
	for _, c := range conns {
		st.UpsertConn(c)
	}
	SetFRPStore(st)
	return p
}

// FRP tab L0：+ 新建连接恒在首位，已存记录随后（无频次时按名称序）；搜索过滤。
func TestFRPTabConnectionList(t *testing.T) {
	setupFRPStore(t,
		frp.Conn{Name: "client-a", Type: frp.TypeSTCP, ServerAddr: "frps.example.com", ServerPort: 7000,
			ServerName: "ssh-a", SecretKey: "k", User: "root", Path: "/var/log/a.log"},
		frp.Conn{Name: "client-b", Type: frp.TypeTCP, ServerAddr: "frps.example.com", ServerPort: 7000,
			RemotePort: 6000, User: "root", Path: "/var/log/b.log"},
	)
	app := newTestApp()
	app.openSourcePicker(3)

	cands := app.visiblePickerCandidates()
	if len(cands) != 3 || cands[0].value != "+new" || cands[1].value != "client-a" {
		t.Fatalf("L0 应为 [+new, client-a, client-b]，实际 %v", cands)
	}
	// label：连接名 + [类型 关键参数（官方字段）]——frp-path-hide:日志路径不显示
	if cands[1].label != "client-a  [stcp ssh-a@frps.example.com]" {
		t.Fatalf("stcp label 应含 serverName@serverAddr，实际 %q", cands[1].label)
	}
	if cands[2].label != "client-b  [tcp frps.example.com:6000]" {
		t.Fatalf("tcp label 应含 serverAddr:remotePort，实际 %q", cands[2].label)
	}
	// 搜索过滤（client-b 是 tcp 连接，按 remotePort 摘要过滤）
	app.pickerDirFilter = "6000" // tui-picker-unify:词源统一 pickerDirFilter
	cands = app.visiblePickerCandidates()
	if len(cands) != 1 || cands[0].value != "client-b" {
		t.Fatalf("过滤后应只剩 client-b(+new 词态隐藏,tui-picker-unify)，实际 %v", cands)
	}
	// frp-path-hide：无 Path 记录不再显示占位，与有 Path 记录同款渲染
	setupFRPStore(t, stcpConn("x"))
	app2 := newTestApp()
	app2.openSourcePicker(3)
	if c := app2.visiblePickerCandidates(); strings.Contains(c[1].label, "（未选过日志）") {
		t.Fatalf("无 Path 不应显示占位，实际 %q", c[1].label)
	}
}

// 别名展示：L0 label 优先 alias（空回落 name）；value（主键）仍是 name。
func TestFRPConnAliasDisplay(t *testing.T) {
	c := stcpConn("client-a")
	c.Alias = "办公隧道"
	setupFRPStore(t, c)
	app := newTestApp()
	app.openSourcePicker(3)
	cands := app.visiblePickerCandidates()
	if !strings.HasPrefix(cands[1].label, "办公隧道  [") {
		t.Fatalf("label 应以别名开头，实际 %q", cands[1].label)
	}
	if cands[1].value != "client-a" {
		t.Fatalf("主键应仍是 name，实际 %q", cands[1].value)
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

// 表单 step0：连接类型候选（stcp/tcp），选择后进 serverAddr；Backspace 逐级返回。
func TestFRPFormTypeSelect(t *testing.T) {
	setupFRPStore(t)
	app := newTestApp()
	app.openSourcePicker(3)
	// L0 Enter（光标 0 = + 新建连接）→ 表单 step0（type 选择）
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerFRPLevel != 1 || app.pickerFRPStep != 0 {
		t.Fatalf("应进入表单 step0，实际 level=%d step=%d", app.pickerFRPLevel, app.pickerFRPStep)
	}
	cands := app.visiblePickerCandidates()
	if len(cands) != 2 || cands[0].value != frp.TypeSTCP || cands[1].value != frp.TypeTCP {
		t.Fatalf("step0 候选应为 [stcp, tcp]，实际 %v", cands)
	}
	// 选 tcp（光标 1）→ step1(serverAddr)
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerFRPStep != 1 || app.pickerFRPFormType != frp.TypeTCP {
		t.Fatalf("选 tcp 应到 step1 且记录类型，实际 step=%d type=%q", app.pickerFRPStep, app.pickerFRPFormType)
	}
	// Backspace 逐级返回：step1→step0→L0（线性，无 v1 的跳级特判）
	app.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if app.pickerFRPStep != 0 {
		t.Fatalf("Backspace 应回 step0，实际 %d", app.pickerFRPStep)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if app.pickerFRPLevel != 0 {
		t.Fatalf("step0 Backspace 应回 L0，实际 %d", app.pickerFRPLevel)
	}
}

// 表单 stcp 全流程：type → serverAddr → token(可空) → secretKey → serverName → user（空值不提交）。
func TestFRPFormSTCPFields(t *testing.T) {
	setupFRPStore(t)
	app := newTestApp()
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // +new → step0(type)
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 光标 0 = stcp → step1
	if app.pickerFRPStep != 1 {
		t.Fatalf("应到 step1(serverAddr)，实际 %d", app.pickerFRPStep)
	}
	typeRunes(app, "frps2.example.com:7000")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step2 token
	typeRunes(app, "tk2")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step3 secretKey
	typeRunes(app, "sk1")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step4 serverName
	typeRunes(app, "ssh-x")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step5 user
	if app.pickerFRPSecretKey != "sk1" || app.pickerFRPSName != "ssh-x" {
		t.Fatalf("字段未暂存: secretKey=%q serverName=%q", app.pickerFRPSecretKey, app.pickerFRPSName)
	}
	// user 留空 Enter：不提交
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerFRPStep != 5 {
		t.Fatalf("user 为空不应提交，实际 step=%d", app.pickerFRPStep)
	}
}

// 表单 tcp 流程：step3=remotePort（非数字/越界不前进），step4=user → step5=alias（可空，末步提交）。
func TestFRPFormTCPFields(t *testing.T) {
	setupFRPStore(t)
	app := newTestApp()
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // +new → step0
	app.Update(tea.KeyMsg{Type: tea.KeyDown})  // 光标 → tcp
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step1
	typeRunes(app, "frps.example.com")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step2 token
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 空 token → step3 remotePort
	if app.pickerFRPStep != 3 {
		t.Fatalf("tcp 应到 step3(remotePort)，实际 %d", app.pickerFRPStep)
	}
	// 非数字不前进
	typeRunes(app, "abc")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerFRPStep != 3 {
		t.Fatalf("非数字端口不应前进，实际 step=%d", app.pickerFRPStep)
	}
	// 输入框内容需清掉再输入：Backspace 逐字符删 "abc"
	for i := 0; i < 3; i++ {
		app.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	typeRunes(app, "6000")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step4 user
	if app.pickerFRPStep != 4 || app.pickerFRPRemotePort != 6000 {
		t.Fatalf("端口 6000 应进 step4，实际 step=%d port=%d", app.pickerFRPStep, app.pickerFRPRemotePort)
	}
	// user 空 Enter 不提交；填 user 进 step5(alias)（tcp 末步）
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerFRPStep != 4 || app.pickerLoading {
		t.Fatalf("user 为空不应提交，实际 step=%d loading=%v", app.pickerFRPStep, app.pickerLoading)
	}
	typeRunes(app, "root")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step5 alias
	if app.pickerFRPStep != 5 {
		t.Fatalf("tcp user 后应进 step5(alias)，实际 %d", app.pickerFRPStep)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 空别名提交 → loading
	if !app.pickerLoading {
		t.Fatal("tcp 表单提交后应进入 loading")
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

// stcp frpTunnelMsg：进 L2、隧道/host=127.0.0.1/隧道端口落位。
func TestFRPTunnelMsgBrowseEntersDirLevel(t *testing.T) {
	setupFRPStore(t)
	app := newTestApp()
	app.openSourcePicker(3)
	fake := &fakeFRPTunnel{port: 6022}
	conn := stcpConn("ssh-x")
	app.Update(frpTunnelMsg{conn: conn, tunnel: fake})
	if app.pickerFRPLevel != 2 || app.pickerFRPTunnel != fake || !app.pickerLoading {
		t.Fatalf("隧道建立应进 L2 且 loading，实际 level=%d loading=%v", app.pickerFRPLevel, app.pickerLoading)
	}
	if app.pickerFRPUser != "root" || app.pickerFRPDir != "/" {
		t.Fatalf("user/dir 应就位（无 Path 起始根目录）: %q %q", app.pickerFRPUser, app.pickerFRPDir)
	}
	if app.pickerFRPHost != "127.0.0.1" || app.pickerFRPPort != 6022 {
		t.Fatalf("stcp 浏览目标应为 127.0.0.1:6022，实际 %s:%d", app.pickerFRPHost, app.pickerFRPPort)
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

// tcp frpTunnelMsg（无隧道）：host=serverAddr、port=remotePort 落位，隧道为 nil。
func TestFRPTunnelMsgTCPDirect(t *testing.T) {
	setupFRPStore(t)
	app := newTestApp()
	app.openSourcePicker(3)
	conn := frp.Conn{Name: "nas", Type: frp.TypeTCP, ServerAddr: "frps.example.com", ServerPort: 7000,
		RemotePort: 6000, User: "root", Path: "/var/log/nas.log"}
	app.Update(frpTunnelMsg{conn: conn})
	if app.pickerFRPLevel != 2 || app.pickerFRPTunnel != nil || !app.pickerLoading {
		t.Fatalf("tcp 应进 L2（无隧道）且 loading，实际 level=%d tunnel=%v loading=%v",
			app.pickerFRPLevel, app.pickerFRPTunnel != nil, app.pickerLoading)
	}
	if app.pickerFRPHost != "frps.example.com" || app.pickerFRPPort != 6000 {
		t.Fatalf("tcp 浏览目标应为 frps.example.com:6000，实际 %s:%d", app.pickerFRPHost, app.pickerFRPPort)
	}
	if app.pickerFRPDir != "/var/log" { // Path 父目录
		t.Fatalf("起始目录应为 /var/log，实际 %s", app.pickerFRPDir)
	}
	if app.pickerFRPCur == nil || app.pickerFRPCur.Name != "nas" {
		t.Fatal("连接快照应落位")
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

// stcp 表单提交后进入 loading 等隧道。
func TestFRPFormSubmitStartsTunnel(t *testing.T) {
	setupFRPStore(t)
	app := newTestApp()
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // +new → step0
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // stcp → step1
	typeRunes(app, "frps.example.com")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step2
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 空 token → step3
	typeRunes(app, "sk1")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	typeRunes(app, "ssh-x")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	typeRunes(app, "root")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step6 alias
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 空别名提交
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

// enterFRPBrowseState 构造 stcp L2 浏览态（快照/host/port 就位，模拟 frpTunnelMsg 落位后的现场）。
// 隧道参数收接口：fakeFRPTunnel/fakeFRPTunnelLog 均可传（后者保留 RecentLog 供诊断断言）。
func enterFRPBrowseState(app *App, t frpTunnelHandle, conn frp.Conn) {
	app.openSourcePicker(3)
	cur := conn
	app.pickerFRPLevel = 2
	app.pickerFRPCur = &cur
	app.pickerFRPTunnel = t
	app.pickerFRPHost = "127.0.0.1"
	app.pickerFRPPort = t.LocalPort()
	app.pickerFRPUser = conn.User
	app.pickerFRPConnName = conn.Name
	app.pickerFRPDir = "/var/log"
}

func TestFRPConfirmSavesRecordAndSwitchesStream(t *testing.T) {
	setupFRPStore(t)
	fakeSSHBin(t) // 阻塞式假 ssh，避免真连 127.0.0.1
	app := newTestApp()
	fake := &fakeFRPTunnel{port: 6022}
	conn := stcpConn("ssh-x")
	conn.Alias = "办公隧道" // stcp 侧 SetAlias 接线的端到端锁定（Final Review Minor#2）
	enterFRPBrowseState(app, fake, conn)
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
	if label != "frp://办公隧道/var/log/app.log" {
		t.Fatalf("应切到 FRP 源（label 用别名），实际 %s", label)
	}
	// 记录已保存（含 Path，官方字段整体）
	c, ok := frpStore().FindConn("ssh-x")
	if !ok || c.Path != "/var/log/app.log" || c.ServerAddr != "frps.example.com" ||
		c.ServerName != "ssh-x" || c.SecretKey != "sk1" || c.Type != frp.TypeSTCP {
		t.Fatalf("记录应整体保存，实际 %+v ok=%v", c, ok)
	}
}

// tcp 确认建流：FRPDirectSource（无隧道），记录整体保存；label 用别名（端到端接线）。
func TestFRPConfirmTCPDirectSource(t *testing.T) {
	setupFRPStore(t)
	fakeSSHBin(t)
	app := newTestApp()
	conn := frp.Conn{Name: "nas", Type: frp.TypeTCP, ServerAddr: "frps.example.com", ServerPort: 7000,
		RemotePort: 6000, User: "root", Alias: "家里 NAS"}
	cur := conn
	app.openSourcePicker(3)
	app.pickerFRPLevel = 2
	app.pickerFRPCur = &cur
	app.pickerFRPHost = conn.ServerAddr
	app.pickerFRPPort = conn.RemotePort
	app.pickerFRPUser = conn.User
	app.pickerFRPConnName = conn.Name
	app.pickerFRPDir = "/var/log"
	app.pickerCandidates = []sourceCandidate{{label: "app.log", value: "app.log"}}
	app.pickerLoading = false

	app.Update(tea.KeyMsg{Type: tea.KeyDown}) // 跳过 ../
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // confirm → FRPDirectSource
	if app.sourcePickerMode {
		t.Fatal("确认后应关闭选择器")
	}
	if label := app.stream.Label(); label != "frp://家里 NAS/var/log/app.log" {
		t.Fatalf("应切到 FRP 直连源（label 用别名），实际 %s", label)
	}
	c, ok := frpStore().FindConn("nas")
	if !ok || c.Type != frp.TypeTCP || c.RemotePort != 6000 || c.Path != "/var/log/app.log" {
		t.Fatalf("tcp 记录应整体保存，实际 %+v ok=%v", c, ok)
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

// 表单最后一步提交即保存记录（此前仅选文件确认时保存，中途放弃配置就丢）。
func TestFRPFormSubmitSavesConnImmediately(t *testing.T) {
	p := setupFRPStore(t)
	app := newTestApp()
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // +new → step0
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // stcp → step1
	typeRunes(app, "frps.example.com:7000")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step2
	typeRunes(app, "tk")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step3
	typeRunes(app, "sk1")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	typeRunes(app, "ssh-z")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	typeRunes(app, "root")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step6 alias
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 空别名提交

	c, ok := frpStore().FindConn("ssh-z")
	if !ok || c.Type != frp.TypeSTCP || c.ServerAddr != "frps.example.com" || c.ServerPort != 7000 ||
		c.Token != "tk" || c.SecretKey != "sk1" || c.ServerName != "ssh-z" || c.User != "root" {
		t.Fatalf("提交后应立即保存记录（官方字段平铺），实际 %+v ok=%v", c, ok)
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
	setupFRPStore(t, func() frp.Conn {
		c := stcpConn("client-a")
		c.Path = "/var/log/a.log"
		return c
	}())
	fakeSSHBin(t)
	app := newTestApp()
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyDown}) // 光标到 client-a
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !app.pickerLoading {
		t.Fatal("选已存记录应进入 loading 等隧道")
	}
	fake := &fakeFRPTunnel{port: 6022}
	c := stcpConn("client-a")
	c.Path = "/var/log/a.log"
	app.Update(frpTunnelMsg{conn: c, tunnel: fake})
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
// 连接快照/host/port 模拟 L0 选已存记录直达时就位（真实时序：浏览前已定）。
func enterFRPPwState(app *App, fake *fakeFRPTunnel) {
	enterFRPBrowseState(app, fake, stcpConn("ssh-x"))
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
	enterFRPBrowseState(app, fake, stcpConn("ssh-x"))
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
	fake := &fakeFRPTunnelLog{log: "E proxy [ssh-a] not found"}
	fake.port = 6022
	enterFRPBrowseState(app, fake, stcpConn("ssh-x"))
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
	enterFRPBrowseState(app2, &fakeFRPTunnel{port: 6022}, stcpConn("ssh-x"))
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
	fake := &fakeFRPTunnelLog{log: "[W] dialRawVisitorConn error: start new visitor connection error: custom listener for [8C32230C66ED] doesn't exist"}
	fake.port = 6022
	enterFRPBrowseState(app, fake, stcpConn("ssh-x"))
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

// frp 密码确认后回目录层：连接快照应保留，confirmFRPPicker 保存的记录字段完整。
func TestFRPPwConfirmKeepsConnForRecord(t *testing.T) {
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
	if app.pickerFRPCur == nil || app.pickerFRPCur.ServerAddr != "frps.example.com" || app.pickerFRPCur.SecretKey != "sk1" {
		t.Fatalf("确认后应保留连接快照，实际 %+v", app.pickerFRPCur)
	}

	// 回目录层后选文件确认 → 记录整体保存（官方字段非空）
	app.pickerCandidates = []sourceCandidate{{label: "app.log", value: "app.log"}}
	app.pickerLoading = false
	app.Update(tea.KeyMsg{Type: tea.KeyDown})  // 跳过首位 ../
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 选中 app.log → confirmFRPPicker
	c, ok := frpStore().FindConn("ssh-x")
	if !ok || c.ServerAddr != "frps.example.com" || c.SecretKey != "sk1" || c.ServerName != "ssh-x" {
		t.Fatalf("记录应含官方字段，实际 %+v ok=%v", c, ok)
	}
}

// 回归（终审 F1）：表单建连 A 浏览后退回 L0 再 "+new" 建连 B，残留 ConnName 必须作废。
// 否则：① frpPwKey 错位（B 的密码缓存走 A 的 key）；② confirmFRPPicker 以旧名 UpsertConn，
// ssh-x 旧记录被 B 的参数整体覆盖；③ 迟到 frpTunnelMsg 的 conn.Name 不生效。
func TestFRPNewFormClearsStaleConnName(t *testing.T) {
	connA := stcpConn("ssh-x")
	connA.Path = "/var/log/x.log"
	setupFRPStore(t, connA)
	fakeSSHBin(t) // confirmFRPPicker 建 FRPSource，避免真连
	app := newTestApp()
	app.openSourcePicker(3)

	// 第一次建连 A（表单 serverName=ssh-x）：提交后隧道建立，进 L2 落位 ConnName
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // +new → step0(type)
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // stcp → step1
	typeRunes(app, "frps.example.com")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step2
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 空 token → step3
	typeRunes(app, "sk1")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	typeRunes(app, "ssh-x")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	typeRunes(app, "root")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step6 alias
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 空别名提交 → loading
	fake1 := &fakeFRPTunnel{port: 6022}
	app.Update(frpTunnelMsg{conn: stcpConn("ssh-x"), tunnel: fake1})
	if app.pickerFRPConnName != "ssh-x" {
		t.Fatalf("前置：建连 A 后 ConnName 应为 ssh-x，实际 %q", app.pickerFRPConnName)
	}
	// 目录列表回填（清 loading）→ 根目录 Backspace → closeFRPBrowse 回 L0
	app.Update(candidatesMsg{tab: 3, kind: "frpdir", ns: "frp:/",
		items: []sourceCandidate{{label: "app.log", value: "app.log"}}})
	app.pickerLoading = false
	app.pickerFRPDir = "/" // 起始已是根（无 Path 记录），直接 Backspace 即退
	app.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if app.pickerFRPLevel != 0 || app.pickerFRPTunnel != nil {
		t.Fatalf("Backspace 应回 L0 并清隧道，实际 level=%d tunnel=%v", app.pickerFRPLevel, app.pickerFRPTunnel != nil)
	}

	// "+new" Enter 进表单：残留 ConnName 必须清空（旧记录名作废）
	app.pickerFRPAlias = "残留别名" // 模拟 C-e 编辑后的残值：+new 必须清零
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.pickerFRPLevel != 1 || app.pickerFRPStep != 0 {
		t.Fatalf("应进表单 step0，实际 level=%d step=%d", app.pickerFRPLevel, app.pickerFRPStep)
	}
	if app.pickerFRPConnName != "" {
		t.Fatalf("+new 进表单应清除残留 ConnName，实际 %q", app.pickerFRPConnName)
	}
	if app.pickerFRPAlias != "" {
		t.Fatalf("+new 进表单应清除残留别名，实际 %q", app.pickerFRPAlias)
	}

	// 走表单建连 B（serverName=ssh-y）
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // stcp → step1
	typeRunes(app, "frps.example.com")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step2
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 空 token → step3
	typeRunes(app, "sk2")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	typeRunes(app, "ssh-y")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	typeRunes(app, "root")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step6 alias
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 空别名提交 → loading
	if !app.pickerLoading {
		t.Fatal("表单提交后应进入 loading 等隧道")
	}

	// 隧道建立：conn.Name 应无条件落位，密码 key 跟随 B
	fake2 := &fakeFRPTunnel{port: 6023}
	connB := stcpConn("ssh-y")
	connB.SecretKey = "sk2"
	app.Update(frpTunnelMsg{conn: connB, tunnel: fake2})
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
	if !ok || old.ServerName != "ssh-x" || old.SecretKey != "sk1" || old.Path != "/var/log/x.log" {
		t.Fatalf("ssh-x 旧记录不应被建连 B 覆盖，实际 %+v ok=%v", old, ok)
	}
	rec, ok := frpStore().FindConn("ssh-y")
	if !ok || rec.ServerAddr != "frps.example.com" || rec.SecretKey != "sk2" || rec.ServerName != "ssh-y" ||
		rec.User != "root" || rec.Path != "/var/log/app.log" {
		t.Fatalf("ssh-y 应保存为新记录，实际 %+v ok=%v", rec, ok)
	}
}

func TestFRPBrowsePasswordPrompt(t *testing.T) {
	setupFRPStore(t)
	app := newTestApp()
	fake := &fakeFRPTunnel{port: 6022}
	enterFRPBrowseState(app, fake, stcpConn("ssh-x"))

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

// C-x 删除：L0 删连接记录（+new 不可删、删除后落盘光标收敛）。
func TestSourcePickerFRPDelete(t *testing.T) {
	p := setupFRPStore(t,
		func() frp.Conn { c := stcpConn("client-a"); c.Path = "/var/log/a.log"; return c }(),
		func() frp.Conn { c := stcpConn("client-b"); c.Path = "/var/log/b.log"; return c }(),
	)
	app := newTestApp()
	app.openSourcePicker(3)

	// 光标在 +new：C-x 不删
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if len(frpStore().Conns) != 2 {
		t.Fatal("+new 上 C-x 不应删除")
	}
	// 下移到 client-a：C-d 删除并落盘(tui-picker-unify:C-x 退役改 C-d)
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	if _, ok := frpStore().FindConn("client-a"); ok {
		t.Fatal("C-d 应删除 client-a")
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
}

// 新建表单必填字段首字母 q 不得退出 TUI:q-退出仅限列表态(过滤框空=纯列表导航),
// 表单每步字段从空开始,空输入框按 q 是打字常态——serverAddr 填 qq.example.com
// 第一个字母就把整个 TUI 杀了(2026-11-04 验收 bug)。
func TestFRPFormQNotQuit(t *testing.T) {
	setupFRPStore(t)
	app := newTestApp()
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // +new 首位 → 新建表单
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // stcp 首位 → step 1 serverAddr
	if app.pickerFRPLevel != 1 || app.pickerFRPStep != 1 {
		t.Fatalf("应在新建表单 step1,实际 level=%d step=%d", app.pickerFRPLevel, app.pickerFRPStep)
	}
	typeRunes(app, "q")
	if !app.sourcePickerMode || app.pickerFRPInput != "q" {
		t.Fatalf("q 应进输入框且不退出选择器,实际 input=%q mode=%v", app.pickerFRPInput, app.sourcePickerMode)
	}
}

// stcp 表单 step6=alias（可空）：user 后进别名步，输入提交后入库；Backspace 回 user。
func TestFRPFormAliasSTCP(t *testing.T) {
	setupFRPStore(t)
	app := newTestApp()
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // +new → step0
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // stcp → step1
	typeRunes(app, "frps.example.com:7000")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step2 token
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 空 token → step3
	typeRunes(app, "sk1")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step4 serverName
	typeRunes(app, "ssh-x")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step5 user
	typeRunes(app, "root")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step6 alias（新）
	if app.pickerFRPStep != 6 {
		t.Fatalf("user 后应进 step6(alias)，实际 %d", app.pickerFRPStep)
	}
	// Backspace 逐级回退：step6 → step5（回退后输入框从空起，user 需重输）
	app.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if app.pickerFRPStep != 5 {
		t.Fatalf("alias 步 Backspace 应回 user，实际 %d", app.pickerFRPStep)
	}
	typeRunes(app, "root")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // user 重提交 → step6
	typeRunes(app, "生产网关")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 提交
	if !app.pickerLoading {
		t.Fatal("alias 填值提交后应 loading")
	}
	if c, ok := frpStore().FindConn("ssh-x"); !ok || c.Alias != "生产网关" {
		t.Fatalf("入库应带别名，实际 %+v", c)
	}
}

// tcp 表单 step5=alias：user 后进别名步；空回车提交不受阻、入库别名为空串。
func TestFRPFormAliasTCP(t *testing.T) {
	setupFRPStore(t)
	app := newTestApp()
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app.Update(tea.KeyMsg{Type: tea.KeyDown}) // 光标 → tcp
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step1
	typeRunes(app, "frps.example.com")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step2
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 空 token → step3
	typeRunes(app, "6000")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step4 user
	typeRunes(app, "root")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step5 alias（新）
	if app.pickerFRPStep != 5 {
		t.Fatalf("tcp user 后应进 step5(alias)，实际 %d", app.pickerFRPStep)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 空别名直接提交
	if !app.pickerLoading {
		t.Fatal("tcp 空别名提交应 loading")
	}
	if c, ok := frpStore().FindConn("frps.example.com"); !ok || c.Alias != "" {
		t.Fatalf("空别名入库应为空串，实际 %+v", c)
	}
}

// C-e 编辑回填 alias：每步种子带原值，全程 Enter 直通即保留（frp-edit-prefill
// 改写：预填语义下不再重打字段）；Path 一并保留。
func TestFRPEditKeepsAlias(t *testing.T) {
	setupFRPStore(t, frp.Conn{
		Name: "ssh-x", Type: frp.TypeSTCP, ServerAddr: "frps.example.com", ServerPort: 7000,
		ServerName: "ssh-x", SecretKey: "sk1", User: "root", Alias: "生产网关", Path: "/var/log/app.log",
	})
	app := newTestApp()
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyDown}) // 光标 → ssh-x（0 是 +new）
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	if app.pickerFRPLevel != 1 || app.pickerFRPAlias != "生产网关" {
		t.Fatalf("编辑应回填别名，实际 level=%d alias=%q", app.pickerFRPLevel, app.pickerFRPAlias)
	}
	// stcp 步 0-6：每步输入框已带原值，7 次 Enter 全程直通提交（零打字）
	for range 7 {
		app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	}
	c, ok := frpStore().FindConn("ssh-x")
	if !ok || c.Alias != "生产网关" {
		t.Fatalf("编辑直通别名应保留，实际 %+v", c)
	}
	if c.Path != "/var/log/app.log" {
		t.Fatalf("编辑保存应保留 Path，实际 %q", c.Path)
	}
}

// 编辑 alias 步 C-u 清空种子后 Enter = 删除别名（回落 Name 展示）。
func TestFRPEditClearsAlias(t *testing.T) {
	setupFRPStore(t, frp.Conn{
		Name: "ssh-x", Type: frp.TypeSTCP, ServerAddr: "frps.example.com", ServerPort: 7000,
		ServerName: "ssh-x", SecretKey: "sk1", User: "root", Alias: "生产网关", Path: "/var/log/app.log",
	})
	app := newTestApp()
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	// 前 6 步种子直通（type/addr/token/secretKey/serverName/user）
	for range 6 {
		app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	}
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlU}) // 清空 alias 种子
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 空 Enter = 删别名，提交
	c, ok := frpStore().FindConn("ssh-x")
	if !ok || c.Alias != "" {
		t.Fatalf("C-u 清空后回车应删除别名，实际 %+v", c)
	}
}

// 表单回退一步再前进：已提交到表单字段的值回到输入框、光标置 runes 尾
// （frp-edit-prefill；新表单同修——此前回退清空输入框，再前进必输字段卡死）。
// Esc 序列：step2 输入为空时"词→层→关"第一步无词可清，一次 Esc 即回退一步。
func TestFRPFormBackReseed(t *testing.T) {
	setupFRPStore(t)
	app := newTestApp()
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlA}) // C-a 新表单
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // step0 选 stcp → step1
	typeRunes(app, "frps.example.com")
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step2 token（addr 已提交字段）
	app.Update(tea.KeyMsg{Type: tea.KeyEscape}) // 输入空，直接回退 → step1
	if app.pickerFRPStep != 1 {
		t.Fatalf("Esc 应回退到 step1，实际 %d", app.pickerFRPStep)
	}
	if app.pickerFRPInput != "frps.example.com" {
		t.Fatalf("回退后输入框应带出已输 addr，实际 %q", app.pickerFRPInput)
	}
	if app.pickerFRPCursor != len([]rune("frps.example.com")) {
		t.Fatalf("光标应置 runes 尾，实际 %d", app.pickerFRPCursor)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 前进回 step2
	if app.pickerFRPStep != 2 || app.pickerFRPInput != "" {
		t.Fatalf("新表单空字段种子应为空，实际 step=%d input=%q", app.pickerFRPStep, app.pickerFRPInput)
	}
}

// C-e 编辑逐步种子：每步进入输入框带原值（addr 种子含 :port）。
func TestFRPEditSeedsEachStep(t *testing.T) {
	setupFRPStore(t, frp.Conn{
		Name: "ssh-x", Type: frp.TypeSTCP, ServerAddr: "frps.example.com", ServerPort: 7500,
		Token: "tk1", ServerName: "ssh-x", SecretKey: "sk1", User: "root", Alias: "生产网关",
	})
	app := newTestApp()
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyDown}) // → ssh-x
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	if app.pickerCursor != 0 { // stcp 行（frpTypeCandidates 顺序 stcp=0）
		t.Fatalf("type 光标应落原类型行，实际 %d", app.pickerCursor)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // step1
	if app.pickerFRPInput != "frps.example.com:7500" {
		t.Fatalf("addr 种子应含 :port，实际 %q", app.pickerFRPInput)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // step2
	if app.pickerFRPInput != "tk1" {
		t.Fatalf("token 种子，实际 %q", app.pickerFRPInput)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // step3
	if app.pickerFRPInput != "sk1" {
		t.Fatalf("secretKey 种子，实际 %q", app.pickerFRPInput)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // step4
	if app.pickerFRPInput != "ssh-x" {
		t.Fatalf("serverName 种子，实际 %q", app.pickerFRPInput)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // step5
	if app.pickerFRPInput != "root" {
		t.Fatalf("user 种子，实际 %q", app.pickerFRPInput)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // step6
	if app.pickerFRPInput != "生产网关" {
		t.Fatalf("alias 种子（中文），实际 %q", app.pickerFRPInput)
	}
	if app.pickerFRPCursor != len([]rune("生产网关")) {
		t.Fatalf("中文种子光标应按 runes 尾，实际 %d", app.pickerFRPCursor)
	}
}

// 编辑全程零打字直通：非默认端口/中文别名/全部字段完整保留（把原信息带出来的终极验收）。
func TestFRPEditAllEnterKeepsPort(t *testing.T) {
	setupFRPStore(t, frp.Conn{
		Name: "ssh-x", Type: frp.TypeSTCP, ServerAddr: "frps.example.com", ServerPort: 7500,
		Token: "tk1", ServerName: "ssh-x", SecretKey: "sk1", User: "root", Alias: "生产网关", Path: "/var/log/app.log",
	})
	app := newTestApp()
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	for range 7 { // stcp 步 0-6 全程 Enter 直通（零打字）
		app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	}
	c, ok := frpStore().FindConn("ssh-x")
	if !ok {
		t.Fatal("编辑直通应落库")
	}
	if c.ServerPort != 7500 {
		t.Fatalf("非默认端口应保留，实际 %d", c.ServerPort)
	}
	if c.Alias != "生产网关" || c.Token != "tk1" || c.SecretKey != "sk1" || c.User != "root" || c.ServerName != "ssh-x" {
		t.Fatalf("全字段应保留，实际 %+v", c)
	}
	if c.Path != "/var/log/app.log" {
		t.Fatalf("Path 应保留，实际 %q", c.Path)
	}
}

// tcp 连接编辑：type 光标落 tcp 行（1），Enter 保留 tcp 不误改 stcp。
func TestFRPEditTCPCursorOnTypeRow(t *testing.T) {
	setupFRPStore(t, frp.Conn{
		Name: "gw", Type: frp.TypeTCP, ServerAddr: "frps.example.com", ServerPort: 7000,
		RemotePort: 6000, User: "root",
	})
	app := newTestApp()
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	if app.pickerCursor != 1 { // tcp 行
		t.Fatalf("tcp 连接 type 光标应落 1，实际 %d", app.pickerCursor)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // Enter 保留 tcp → step1
	if app.pickerFRPFormType != frp.TypeTCP {
		t.Fatalf("Enter 应保留 tcp，实际 %q", app.pickerFRPFormType)
	}
}

// remotePort=0 异常存量编辑：种子空，Enter 卡 step3 不提交（防呆保持）。
func TestFRPEditZeroRemotePortStays(t *testing.T) {
	setupFRPStore(t, frp.Conn{
		Name: "gw", Type: frp.TypeTCP, ServerAddr: "frps.example.com", ServerPort: 7000,
		RemotePort: 0, User: "root",
	})
	app := newTestApp()
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // type → step1
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // addr → step2
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // token → step3
	if app.pickerFRPInput != "" {
		t.Fatalf("remotePort=0 种子应为空，实际 %q", app.pickerFRPInput)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 空输入 → 卡 step3
	if app.pickerFRPStep != 3 {
		t.Fatalf("空 remotePort 应卡 step3，实际 %d", app.pickerFRPStep)
	}
}

// conn.User==""（空默认 root 语义）编辑：user 步种子 "root"，保存显式化为 root（语义等价）。
func TestFRPEditUserEmptySeedsRoot(t *testing.T) {
	setupFRPStore(t, frp.Conn{
		Name: "ssh-x", Type: frp.TypeSTCP, ServerAddr: "frps.example.com", ServerPort: 7000,
		ServerName: "ssh-x", SecretKey: "sk1",
	})
	app := newTestApp()
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	for range 5 { // type/addr/token/sk/sname → step5 user
		app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	}
	if app.pickerFRPInput != "root" {
		t.Fatalf("user 空应种子 root（默认显式化），实际 %q", app.pickerFRPInput)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // user → step6
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // alias 空 → 提交
	c, ok := frpStore().FindConn("ssh-x")
	if !ok || c.User != "root" {
		t.Fatalf("编辑保存 User 应为 root，实际 %+v", c)
	}
}

// 表单层 C-a/C-e 转发输入编辑（行首/行尾）——预填后光标常态在尾，改前缀/端口
// 需跳行首（Final Review Important-1：此前两键在表单层被吞成无操作）。
func TestFRPFormCtrlAHomeCtrlEEnd(t *testing.T) {
	setupFRPStore(t, frp.Conn{
		Name: "ssh-x", Type: frp.TypeSTCP, ServerAddr: "frps.example.com", ServerPort: 7500,
		ServerName: "ssh-x", SecretKey: "sk1", User: "root",
	})
	app := newTestApp()
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlE}) // C-e 编辑
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step1，种子 frps.example.com:7500 光标尾
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlA}) // 行首
	if app.pickerFRPCursor != 0 {
		t.Fatalf("C-a 应回行首，实际 %d", app.pickerFRPCursor)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlE}) // 行尾
	if app.pickerFRPCursor != len([]rune("frps.example.com:7500")) {
		t.Fatalf("C-e 应回行尾，实际 %d", app.pickerFRPCursor)
	}
}

// 编辑 stcp 连接 step0 切 tcp 提交：对侧字段不残留（submitFRPForm 按型清空，
// 对齐 fview saveForm isTCP 语义；名派生 host=新记录，旧 stcp 记录不动是既有命名语义）。
func TestFRPEditSwitchTypeNoResidue(t *testing.T) {
	setupFRPStore(t, frp.Conn{
		Name: "ssh-x", Type: frp.TypeSTCP, ServerAddr: "frps.example.com", ServerPort: 7000,
		ServerName: "ssh-x", SecretKey: "sk1", User: "root", Alias: "别名",
	})
	app := newTestApp()
	app.openSourcePicker(3)
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	app.Update(tea.KeyMsg{Type: tea.KeyDown})  // type 行 → tcp
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // 切 tcp → step1（addr 种子 "frps.example.com"）
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // addr → step2
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // token → step3
	typeRunes(app, "6000")                     // remotePort（stcp 原值 0 种子空，必输）
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → step4 user
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // user 种子 root → step5 alias
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // alias 种子直通提交
	c, ok := frpStore().FindConn("frps.example.com") // tcp 名 = host（派生命名）
	if !ok {
		t.Fatal("切型提交应落库（名=host）")
	}
	if c.Type != frp.TypeTCP || c.RemotePort != 6000 {
		t.Fatalf("切型后应为 tcp/6000，实际 %+v", c)
	}
	if c.SecretKey != "" || c.ServerName != "" {
		t.Fatalf("stcp 字段不得残留进 tcp 连接，实际 %+v", c)
	}
}
