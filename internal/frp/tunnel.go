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

// logTail frpc 输出环形缓冲：waitReady 判活 + 运行期错误诊断（连接被 reset 时
// frpc 日志里才有真因：proxy 不存在/sk 错/远端掉线）。
type logTail struct {
	mu    sync.Mutex
	lines []string // 尾部最多 tunnelLogCap 行
}

const tunnelLogCap = 30

func newLogTail() *logTail { return &logTail{} }

// attach 持续读取 r 进缓冲（goroutine，r 关闭/EOF 后自然退出）。
func (lt *logTail) attach(r io.Reader) {
	go func() {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			lt.write(sc.Text())
		}
	}()
}

func (lt *logTail) write(line string) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	lt.lines = append(lt.lines, line)
	if len(lt.lines) > tunnelLogCap {
		lt.lines = lt.lines[len(lt.lines)-tunnelLogCap:]
	}
}

// last 最后一条非空日志（waitReady 报错用）。
func (lt *logTail) last() string {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	for i := len(lt.lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lt.lines[i]) != "" {
			return lt.lines[i]
		}
	}
	return ""
}

// lastErr 最后一条告警/错误日志（跳过 [I] 信息行与空行；诊断只需 W/E）。
func (lt *logTail) lastErr() string {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	for i := len(lt.lines) - 1; i >= 0; i-- {
		line := lt.lines[i]
		if strings.TrimSpace(line) == "" || strings.Contains(line, " [I] ") {
			continue
		}
		return line
	}
	return ""
}

// recent 最近 n 行（" | " 连接，空缓冲返回空串）。
func (lt *logTail) recent(n int) string {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	if len(lt.lines) == 0 {
		return ""
	}
	if n > len(lt.lines) {
		n = len(lt.lines)
	}
	return strings.Join(lt.lines[len(lt.lines)-n:], " | ")
}

type Tunnel struct {
	port    int
	cmd     *exec.Cmd
	cfgFile string
	tail    *logTail
	key     string // 注册表 key（复用）；未注册为空
	refs    int    // 引用计数（注册表锁保护）
}

func (t *Tunnel) LocalPort() int { return t.port }

// RecentLog 最近一条 frpc 告警/错误日志（[I] 信息行剔除；ssh 层只见 reset，真因在 W/E 行）。
func (t *Tunnel) RecentLog() string { return t.tail.lastErr() }

// kill 杀 frpc 进程并删临时配置（幂等；引用归零或强制退出时调用）。
func (t *Tunnel) kill() {
	if t.cmd != nil && t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
	}
	if t.cfgFile != "" {
		os.RemoveAll(filepath.Dir(t.cfgFile))
	}
}

// Cleanup 释放引用：归零才真正杀 frpc（复用场景其他持有者还在用）。
func (t *Tunnel) Cleanup() error {
	regMu.Lock()
	t.refs--
	dead := t.refs <= 0
	if dead && t.key != "" && registry[t.key] == t {
		delete(registry, t.key)
	}
	regMu.Unlock()
	if dead {
		t.kill()
	}
	return nil
}

// alive 本地端口可连即隧道存活（frpc 死则 bind 端口关闭）。
func (t *Tunnel) alive() bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", t.port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// 隧道注册表：同参数（服务器+token+proxy+sk）的存活隧道复用，引用计数管理生命周期。
var (
	regMu     sync.Mutex
	registry  = map[string]*Tunnel{}
	regClosed bool // KillAllTunnels 后不再接受新隧道（进程退出竞态兜底）
)

// startTunnelFn 可注入的隧道构建（测试替换真实 frpc 启动）。
var startTunnelFn = StartTunnel

func tunnelKey(server Server, sk, proxy string) string {
	return server.Addr + "\x00" + server.Token + "\x00" + proxy + "\x00" + sk
}

// AcquireTunnel 获取隧道：同 key 存活则复用（引用 +1，省 10s 建连等待），
// 否则新建并注册。Cleanup 释放引用，归零自动杀 frpc。
func AcquireTunnel(server Server, sk, proxy string) (*Tunnel, error) {
	key := tunnelKey(server, sk, proxy)
	regMu.Lock()
	if t := registry[key]; t != nil && t.alive() {
		t.refs++
		regMu.Unlock()
		return t, nil
	}
	regMu.Unlock()

	t, err := startTunnelFn(server, sk, proxy)
	if err != nil {
		return nil, err
	}
	t.key = key
	regMu.Lock()
	if regClosed {
		regMu.Unlock()
		t.kill()
		return nil, fmt.Errorf("logview 已退出，隧道未建立")
	}
	t.refs = 1
	registry[key] = t
	regMu.Unlock()
	return t, nil
}

// KillAllTunnels 强杀全部存活隧道（进程退出兜底：选择器持有/在途消息等漏网场景）。
func KillAllTunnels() {
	regMu.Lock()
	regClosed = true
	ts := make([]*Tunnel, 0, len(registry))
	for _, t := range registry {
		ts = append(ts, t)
	}
	registry = map[string]*Tunnel{}
	regMu.Unlock()
	for _, t := range ts {
		t.kill()
	}
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
	// stdout 一并接管：防 frpc 输出污染 TUI 终端，且部分版本错误走 stdout
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("frpc 启动失败: %w", err)
	}
	tail := newLogTail()
	tail.attach(stderr)
	tail.attach(stdout)
	t := &Tunnel{port: port, cmd: cmd, cfgFile: cfg, tail: tail}
	if err := waitReady(port, cmd, tail, tunnelReadyTimeout); err != nil {
		t.kill()
		return nil, err
	}
	return t, nil
}

// waitReady 轮询本地端口直到可连（frpc 绑定成功）；期间 frpc 退出则报日志尾行。
func waitReady(port int, cmd *exec.Cmd, tail *logTail, timeout time.Duration) error {
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
			return fmt.Errorf("frpc 已退出: %s", tail.last())
		default:
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("隧道就绪超时(%v)，frpc 最后输出: %s", timeout, tail.last())
}
