// internal/frp/tunnel_test.go
package frp

import (
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
	if err := waitReady(port, cmd, r, 2*time.Second); err != nil {
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
	err := waitReady(1, cmd, r, 2*time.Second) // 端口 1 无人监听，靠进程退出报错
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("进程退出应携带 stderr 尾行，实际: %v", err)
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
	err := waitReady(1, cmd, r, 300*time.Millisecond) // 端口 1 无人监听 → 超时
	if err == nil || !strings.Contains(err.Error(), "超时") {
		t.Fatalf("无人监听应超时报错，实际: %v", err)
	}
}
