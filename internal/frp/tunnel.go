// internal/frp/tunnel.go
package frp

// frpc stcp visitor 隧道：自动分配本地端口，拉起 frpc 子进程打通隧道。
// 要求 frp ≥ v0.52（TOML 配置格式）。

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const tunnelReadyTimeout = 10 * time.Second

type Tunnel struct {
	port    int
	cmd     *exec.Cmd
	cfgFile string
}

func (t *Tunnel) LocalPort() int { return t.port }

// Cleanup 杀 frpc 进程并删临时配置（幂等）。
func (t *Tunnel) Cleanup() error {
	if t.cmd != nil && t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
	}
	if t.cfgFile != "" {
		os.RemoveAll(filepath.Dir(t.cfgFile))
	}
	return nil
}

// pickFreePort 取一个空闲本地端口（listen :0 后关闭；存在极小竞争窗口，可接受）。
func pickFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// splitHostPort 拆 addr；无端口/非法端口默认 7000。
func splitHostPort(addr string) (string, int) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil || portStr == "" {
		return addr, 7000
	}
	port := 0
	fmt.Sscanf(portStr, "%d", &port)
	if port == 0 {
		return addr, 7000
	}
	return host, port
}

// visitorTOML 生成 frpc visitor 配置（serverAddr/serverPort/token + stcp visitor 块）。
func visitorTOML(server Server, sk, proxyName string, bindPort int) string {
	host, port := splitHostPort(server.Addr)
	var b strings.Builder
	fmt.Fprintf(&b, "serverAddr = %q\n", host)
	fmt.Fprintf(&b, "serverPort = %d\n", port)
	if server.Token != "" {
		fmt.Fprintf(&b, "auth.token = %q\n", server.Token)
	}
	b.WriteString("\n[[visitors]]\n")
	fmt.Fprintf(&b, "name = %q\n", "logview-"+proxyName)
	b.WriteString("type = \"stcp\"\n")
	fmt.Fprintf(&b, "serverName = %q\n", proxyName)
	fmt.Fprintf(&b, "secretKey = %q\n", sk)
	b.WriteString("bindAddr = \"127.0.0.1\"\n")
	fmt.Fprintf(&b, "bindPort = %d\n", bindPort)
	return b.String()
}

// StartTunnel 启动 frpc visitor：自动挑端口、写临时配置、等待本地端口就绪。
func StartTunnel(server Server, sk, proxyName string) (*Tunnel, error) {
	if _, err := exec.LookPath("frpc"); err != nil {
		return nil, fmt.Errorf("未找到 frpc，请先安装 frp (https://github.com/fatedier/frp)")
	}
	port, err := pickFreePort()
	if err != nil {
		return nil, fmt.Errorf("分配本地端口失败: %w", err)
	}
	dir, err := os.MkdirTemp("", "logview-frpc-")
	if err != nil {
		return nil, err
	}
	cfg := filepath.Join(dir, "visitor.toml")
	if err := os.WriteFile(cfg, []byte(visitorTOML(server, sk, proxyName, port)), 0600); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	cmd := exec.Command("frpc", "-c", cfg)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("frpc 启动失败: %w", err)
	}
	t := &Tunnel{port: port, cmd: cmd, cfgFile: cfg}
	if err := waitReady(port, cmd, stderr, tunnelReadyTimeout); err != nil {
		t.Cleanup()
		return nil, err
	}
	return t, nil
}

// waitReady 轮询本地端口直到可连（frpc 绑定成功）；期间 frpc 退出则报 stderr 尾行。
func waitReady(port int, cmd *exec.Cmd, stderr io.Reader, timeout time.Duration) error {
	var mu sync.Mutex
	lastLine := ""
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			mu.Lock()
			lastLine = sc.Text()
			mu.Unlock()
		}
	}()
	exited := make(chan struct{})
	go func() {
		cmd.Wait()
		close(exited)
	}()
	target := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if conn, err := net.DialTimeout("tcp", target, 300*time.Millisecond); err == nil {
			conn.Close()
			return nil
		}
		select {
		case <-exited:
			mu.Lock()
			line := lastLine
			mu.Unlock()
			return fmt.Errorf("frpc 已退出: %s", line)
		default:
		}
		time.Sleep(200 * time.Millisecond)
	}
	mu.Lock()
	line := lastLine
	mu.Unlock()
	return fmt.Errorf("隧道就绪超时(%v)，frpc 最后输出: %s", timeout, line)
}
