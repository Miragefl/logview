// internal/frp/tunnel_test.go
package frp

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestPickFreePort(t *testing.T) {
	p1, err := pickFreePort()
	if err != nil {
		t.Fatal(err)
	}
	if p1 < 1 || p1 > 65535 {
		t.Fatalf("端口越界: %d", p1)
	}
}

func TestVisitorTOML(t *testing.T) {
	got := visitorTOML(Server{Addr: "frps.example.com:7000", Token: "tk"}, "sk1", "ssh-a", 6022)
	for _, want := range []string{
		`serverAddr = "frps.example.com"`,
		"serverPort = 7000",
		`auth.token = "tk"`,
		`type = "stcp"`,
		`serverName = "ssh-a"`,
		`secretKey = "sk1"`,
		`bindAddr = "127.0.0.1"`,
		"bindPort = 6022",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("visitor TOML 缺少 %s:\n%s", want, got)
		}
	}
}

func TestVisitorTOMLDefaultPort(t *testing.T) {
	got := visitorTOML(Server{Addr: "frps.example.com"}, "", "p", 1)
	if !strings.Contains(got, "serverPort = 7000") {
		t.Errorf("无端口地址应默认 7000:\n%s", got)
	}
}

func TestWaitReadyPortOpen(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port

	cmd := exec.Command("sleep", "30")
	r, w, _ := os.Pipe()
	defer w.Close()
	cmd.Stderr = w
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	tail := newLogTail()
	tail.attach(r)
	if err := waitReady(port, cmd, tail, 2*time.Second); err != nil {
		t.Fatalf("端口可连应就绪: %v", err)
	}
}

func TestWaitReadyProcessExit(t *testing.T) {
	cmd := exec.Command("sh", "-c", "echo boom >&2; exit 1")
	r, w, _ := os.Pipe()
	defer w.Close()
	cmd.Stderr = w
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	tail := newLogTail()
	tail.attach(r)
	err := waitReady(1, cmd, tail, 2*time.Second) // 端口 1 无人监听，靠进程退出报错
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("进程退出应携带日志尾行，实际: %v", err)
	}
}

func TestWaitReadyTimeout(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	r, w, _ := os.Pipe()
	defer w.Close()
	cmd.Stderr = w
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	tail := newLogTail()
	tail.attach(r)
	err := waitReady(1, cmd, tail, 300*time.Millisecond) // 端口 1 无人监听 → 超时
	if err == nil || !strings.Contains(err.Error(), "超时") {
		t.Fatalf("无人监听应超时报错，实际: %v", err)
	}
}

// logTail：环形截断 + last 取末条非空 + recent 取尾部 n 行（RecentLog 诊断用）。
func TestLogTailRingAndRecent(t *testing.T) {
	tail := newLogTail()
	for i := 0; i < tunnelLogCap+5; i++ {
		tail.write(fmt.Sprintf("line-%d", i))
	}
	if got := tail.last(); got != fmt.Sprintf("line-%d", tunnelLogCap+4) {
		t.Fatalf("last 应为末行，实际 %q", got)
	}
	want := fmt.Sprintf("line-%d | line-%d | line-%d", tunnelLogCap+2, tunnelLogCap+3, tunnelLogCap+4)
	if got := tail.recent(3); got != want {
		t.Fatalf("recent(3) 应为末 3 行，实际 %q", got)
	}
	tail.mu.Lock()
	n := len(tail.lines)
	tail.mu.Unlock()
	if n != tunnelLogCap {
		t.Fatalf("缓冲应截断到 %d 行，实际 %d", tunnelLogCap, n)
	}
	tail.write("")
	if got := tail.last(); got != fmt.Sprintf("line-%d", tunnelLogCap+4) {
		t.Fatalf("last 应跳过空行，实际 %q", got)
	}
}

// lastErr：跳过 [I] 信息行取末条 W/E（RecentLog 诊断用，信息行只会刷屏）。
func TestLogTailLastErr(t *testing.T) {
	tail := newLogTail()
	tail.write("2026-08-25 23:19:07.628 [I] [client/service.go:332] login to server success")
	tail.write("2026-08-25 23:19:07.628 [I] [visitor/visitor_manager.go:135] start visitor success")
	want := "2026-08-25 23:19:07.785 [W] [visitor/stcp.go:70] dialRawVisitorConn error: custom listener for [x] doesn't exist"
	tail.write(want)
	if got := tail.lastErr(); got != want {
		t.Fatalf("lastErr 应取末条 W 行，实际 %q", got)
	}
	// 尾部只有 [I] 行时继续向前找
	tail.write("2026-08-25 23:19:07.9 [I] [x] y")
	if got := tail.lastErr(); got != want {
		t.Fatalf("应跳过尾部 [I] 行，实际 %q", got)
	}
	// 全是 [I] 行 → 空串
	allInfo := newLogTail()
	allInfo.write("a [I] b")
	if got := allInfo.lastErr(); got != "" {
		t.Fatalf("全信息行应返回空串，实际 %q", got)
	}
}

// resetRegistryForTest 测试隔离：清空注册表与 closed 标记。
func resetRegistryForTest() {
	regMu.Lock()
	registry = map[string]*Tunnel{}
	regClosed = false
	regMu.Unlock()
}

// AcquireTunnel 注册表：同 key 复用（不重复建）、引用计数（归零才杀）、
// 死隧道驱逐、不同参数隔离、KillAllTunnels 兜底。
func TestAcquireTunnelReuseAndRefcount(t *testing.T) {
	resetRegistryForTest()
	t.Cleanup(resetRegistryForTest)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port

	calls := 0
	orig := startTunnelFn
	startTunnelFn = func(Server, string, string) (*Tunnel, error) {
		calls++
		return &Tunnel{port: port, tail: newLogTail()}, nil
	}
	t.Cleanup(func() { startTunnelFn = orig })

	s := Server{Addr: "frps.example.com:7000", Token: "tk"}

	// 同参数两次 Acquire → 同一对象，只建一次
	t1, err := AcquireTunnel(s, "sk", "p")
	if err != nil {
		t.Fatal(err)
	}
	t2, err := AcquireTunnel(s, "sk", "p")
	if err != nil {
		t.Fatal(err)
	}
	if t1 != t2 || calls != 1 {
		t.Fatalf("同参数应复用同一隧道: same=%v calls=%d", t1 == t2, calls)
	}

	// 释放一次（refs 2→1）：frpc 不杀、注册表保留，仍可复用
	t1.Cleanup()
	t3, err := AcquireTunnel(s, "sk", "p")
	if err != nil {
		t.Fatal(err)
	}
	if t3 != t2 || calls != 1 {
		t.Fatalf("引用未归零应继续复用: same=%v calls=%d", t3 == t2, calls)
	}

	// 释放到 0：注册表清除，下次新建
	t2.Cleanup()
	t3.Cleanup()
	t4, err := AcquireTunnel(s, "sk", "p")
	if err != nil {
		t.Fatal(err)
	}
	if t4 == t1 || calls != 2 {
		t.Fatalf("引用归零后应重新建隧道: calls=%d", calls)
	}

	// 不同参数（sk 不同）→ 不同隧道
	t5, err := AcquireTunnel(s, "sk2", "p")
	if err != nil {
		t.Fatal(err)
	}
	if t5 == t4 {
		t.Fatal("不同 sk 不应复用")
	}

	// 死隧道驱逐：端口关闭后复用失败 → 新建
	t4.Cleanup()
	t5.Cleanup()
	l.Close() // 模拟 frpc 死亡（bind 端口关闭）
	t6, err := AcquireTunnel(s, "sk", "p")
	if err != nil {
		t.Fatal(err)
	}
	if t6 == t4 || calls != 4 {
		t.Fatalf("死隧道应驱逐重建: calls=%d", calls)
	}

	// KillAllTunnels：清空注册表 + 强杀
	KillAllTunnels()
	regMu.Lock()
	n := len(registry)
	closed := regClosed
	regMu.Unlock()
	if n != 0 || !closed {
		t.Fatalf("KillAll 后注册表应空且 closed，n=%d closed=%v", n, closed)
	}
	// closed 后不再接受新隧道
	if _, err := AcquireTunnel(s, "sk", "p"); err == nil {
		t.Fatal("closed 后 Acquire 应报错")
	}
}
