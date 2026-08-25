# frp stcp 日志源实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在源选择器弹窗新增 FRP tab：frp stcp 隧道自动建连（端口自动分配），参数+日志路径持久化记忆，旧记录搜索直达 tail。

**Architecture:** 新增 `internal/frp` 包（store：frp.json 读写；tunnel：frpc visitor 子进程管理）。`internal/stream` 增加 `FRPSource`（组合隧道 + 复用 `SSHSource`，SSHSource 增加 port 支持）。TUI 层 sourcepicker 扩为 4 tab，frp tab 三级交互（连接列表 / 逐字段表单 / 远程目录浏览），隧道建立异步化（`frpTunnelMsg`）。

**Tech Stack:** Go 1.26、bubbletea TUI、无新依赖（frpc 为系统外部命令，要求 frp ≥ v0.52 TOML 配置）。

**Spec:** `docs/superpowers/specs/2026-08-25-frp-stcp-source-design.md`

## Global Constraints

- 不引入任何新的 Go 依赖（go.mod 不变）
- frp 参数不写入 rules.yaml、不写入 `~/.ssh/config`；持久化仅 `~/.local/state/logview/frp.json`
- ssh 密码仅内存（`a.sshPasswords`），不落盘
- 代码注释用中文，与现有风格一致
- 测试命令：`go test ./...`；构建：`make build`
- tab 切换取模两处：`sourcepicker_ui.go` 的 KeyTab `% 3` 与 KeyShiftTab `% 3`，均改 `% 4`；tabs 数组改 4 元素
- 现有 8 处 `switch a.sourceTab`（case 0/1/2）全部需适配 case 3

---

### Task 1: frp store（internal/frp/store.go）

**Files:**
- Create: `internal/frp/store.go`
- Test: `internal/frp/store_test.go`

**Interfaces:**
- Produces: `frp.Server{Name,Addr,Token string}`、`frp.Conn{Name,Server,SK,Proxy,User,Path string}`、`frp.Store{Servers []Server; Conns []Conn}`；`frp.LoadStore() *Store`（全局单例）、`(s *Store) Save() error`、`(s *Store) UpsertServer(Server)`、`(s *Store) UpsertConn(Conn)`、`(s *Store) FindServer(name string) (Server, bool)`、`(s *Store) FindConn(name string) (Conn, bool)`、`frp.SetStoreFileForTest(path string)`、`frp.ResetStoreForTest()`

- [ ] **Step 1: 写失败测试**

```go
// internal/frp/store_test.go
package frp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRoundtripAndUpsert(t *testing.T) {
	p := filepath.Join(t.TempDir(), "frp.json")
	SetStoreFileForTest(p)
	defer ResetStoreForTest()

	st := LoadStore()
	st.UpsertServer(Server{Name: "prod", Addr: "frps.example.com:7000", Token: "tk"})
	st.UpsertServer(Server{Name: "prod", Addr: "frps2.example.com:7000", Token: "tk2"}) // 覆盖同名
	st.UpsertConn(Conn{Name: "a", Server: "prod", SK: "sk1", Proxy: "ssh-a", User: "root", Path: "/var/log/a.log"})
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}

	ResetStoreForTest()
	SetStoreFileForTest(p)
	st2 := LoadStore()
	if len(st2.Servers) != 1 || st2.Servers[0].Addr != "frps2.example.com:7000" {
		t.Fatalf("UpsertServer 应按 Name 覆盖，实际 %+v", st2.Servers)
	}
	c, ok := st2.FindConn("a")
	if !ok || c.Proxy != "ssh-a" || c.Path != "/var/log/a.log" {
		t.Fatalf("FindConn 应取回记录，实际 %+v ok=%v", c, ok)
	}
	if _, ok := st2.FindConn("nope"); ok {
		t.Fatal("不存在的记录应返回 false")
	}
}

func TestStoreCorruptFileDegradesToEmpty(t *testing.T) {
	p := filepath.Join(t.TempDir(), "frp.json")
	SetStoreFileForTest(p)
	defer ResetStoreForTest()
	if err := os.WriteFile(p, []byte("not json {"), 0644); err != nil {
		t.Fatal(err)
	}
	st := LoadStore()
	if len(st.Servers) != 0 || len(st.Conns) != 0 {
		t.Fatalf("损坏文件应降级为空 store，实际 %+v", st)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/frp/ -run TestStore -v`
Expected: FAIL（包/函数未定义）

- [ ] **Step 3: 最小实现**

```go
// internal/frp/store.go
package frp

// frp stcp 连接参数持久化：~/.local/state/logview/frp.json（与 usage.json/session.json 同目录）。
// 存两类数据：frps 服务器（地址+token）、连接记录（frps 引用+sk+proxy+用户+日志路径）。
// 文件缺失/损坏 → 空 store 降级，不报错。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Server struct {
	Name  string `json:"name"`
	Addr  string `json:"addr"` // host:port
	Token string `json:"token"`
}

type Conn struct {
	Name   string `json:"name"` // 记录名（默认 = proxy 名）
	Server string `json:"server"`
	SK     string `json:"sk"`
	Proxy  string `json:"proxy"`
	User   string `json:"user"`
	Path   string `json:"path"` // 远程日志路径（直达 tail 用）
}

type Store struct {
	Servers []Server `json:"servers"`
	Conns   []Conn   `json:"connections"`
}

var (
	storeMu   sync.Mutex
	storeData *Store
	storeFile string // 测试可覆盖
)

func storePath() string {
	if storeFile != "" {
		return storeFile
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".local", "state", "logview")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "frp.json")
}

// LoadStore 全局单例：读失败/损坏 → 空 store。
func LoadStore() *Store {
	storeMu.Lock()
	defer storeMu.Unlock()
	if storeData != nil {
		return storeData
	}
	storeData = &Store{}
	p := storePath()
	if p == "" {
		return storeData
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return storeData
	}
	json.Unmarshal(data, storeData) // 损坏 → 保持空 store
	return storeData
}

func (s *Store) Save() error {
	storeMu.Lock()
	defer storeMu.Unlock()
	p := storePath()
	if p == "" {
		return os.ErrNotExist
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

// UpsertServer 按 Name 去重覆盖。
func (s *Store) UpsertServer(v Server) {
	for i := range s.Servers {
		if s.Servers[i].Name == v.Name {
			s.Servers[i] = v
			return
		}
	}
	s.Servers = append(s.Servers, v)
}

// UpsertConn 按 Name 去重覆盖（确认日志文件时更新 Path 也走这里）。
func (s *Store) UpsertConn(c Conn) {
	for i := range s.Conns {
		if s.Conns[i].Name == c.Name {
			s.Conns[i] = c
			return
		}
	}
	s.Conns = append(s.Conns, c)
}

func (s *Store) FindServer(name string) (Server, bool) {
	for _, v := range s.Servers {
		if v.Name == name {
			return v, true
		}
	}
	return Server{}, false
}

func (s *Store) FindConn(name string) (Conn, bool) {
	for _, c := range s.Conns {
		if c.Name == name {
			return c, true
		}
	}
	return Conn{}, false
}

// SetStoreFileForTest / ResetStoreForTest 测试专用（同包测试直接调用）。
func SetStoreFileForTest(p string) {
	storeMu.Lock()
	defer storeMu.Unlock()
	storeFile = p
	storeData = nil
}

func ResetStoreForTest() {
	storeMu.Lock()
	defer storeMu.Unlock()
	storeFile = ""
	storeData = nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/frp/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/frp/
git commit -m "feat(frp): frp 连接参数存储（frp.json 读写）"
```

---

### Task 2: 隧道管理（internal/frp/tunnel.go）

**Files:**
- Create: `internal/frp/tunnel.go`
- Create: `internal/frp/tunnel_integration_test.go`（build tag，默认跳过）
- Test: `internal/frp/tunnel_test.go`

**Interfaces:**
- Consumes: Task 1 的 `Server`
- Produces: `frp.StartTunnel(server Server, sk, proxyName string) (*Tunnel, error)`；`(*Tunnel).LocalPort() int`；`(*Tunnel).Cleanup() error`

- [ ] **Step 1: 写失败测试**

```go
// internal/frp/tunnel_test.go
package frp

import (
	"net"
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
	r, w, err := osPipe()
	if err != nil {
		t.Fatal(err)
	}
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
	r, w, _ := osPipe()
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
	r, w, _ := osPipe()
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
```

注：`osPipe` 为测试辅助（封装 `os.Pipe()`），在测试文件底部定义：

```go
func osPipe() (*os.File, *os.File, error) { return os.Pipe() }
```
（import 增加 `"os"`；如嫌多余可直接用 `os.Pipe()`，返回值忽略 err 用 `r, w, _ := os.Pipe()`）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/frp/ -run 'PickFree|Visitor|WaitReady' -v`
Expected: FAIL

- [ ] **Step 3: 实现**

```go
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
```

```go
// internal/frp/tunnel_integration_test.go
//go:build frpintegration

package frp

// 真实 frpc 集成测试（需本机 frpc + 可达 frps）：
//   go test -tags frpintegration ./internal/frp/ -run TestStartTunnelReal -v
// 环境变量：FRPS_ADDR（host:port）、FRPS_TOKEN、FRP_SK、FRP_PROXY。
import (
	"os"
	"testing"
)

func TestStartTunnelReal(t *testing.T) {
	sv := Server{Addr: os.Getenv("FRPS_ADDR"), Token: os.Getenv("FRPS_TOKEN")}
	tun, err := StartTunnel(sv, os.Getenv("FRP_SK"), os.Getenv("FRP_PROXY"))
	if err != nil {
		t.Fatal(err)
	}
	defer tun.Cleanup()
	if tun.LocalPort() <= 0 {
		t.Fatalf("端口非法: %d", tun.LocalPort())
	}
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/frp/ -v && go vet ./internal/frp/`
Expected: PASS（frpintegration 文件默认不编译）

- [ ] **Step 5: Commit**

```bash
git add internal/frp/
git commit -m "feat(frp): frpc visitor 隧道管理（自动端口分配+就绪探测）"
```

---

### Task 3: SSHSource 端口支持（internal/stream/ssh.go）

**Files:**
- Modify: `internal/stream/ssh.go`
- Test: `internal/stream/ssh_test.go`（追加）

**Interfaces:**
- Produces: `(*SSHSource).SetPort(p int)`；`stream.SSHListDirWithPort(host string, port int, path, password string) ([]SSHDirEntry, error)`；内部 `sshCommandPrefix(host string, port int) []string`

- [ ] **Step 1: 写失败测试（追加到 ssh_test.go）**

```go
func TestSSHCommandPrefix(t *testing.T) {
	got := sshCommandPrefix("user@127.0.0.1", 6022)
	want := "ssh -o ConnectTimeout=8 -p 6022 user@127.0.0.1"
	if strings.Join(got, " ") != want {
		t.Errorf("got %v", got)
	}
	got = sshCommandPrefix("host-a", 0)
	if strings.Join(got, " ") != "ssh -o ConnectTimeout=8 host-a" {
		t.Errorf("port=0 应无 -p，got %v", got)
	}
}

func TestSSHSourceWithPortStreamsLines(t *testing.T) {
	bin := fakeSSHScript(t, "frp line one\nfrp line two")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	src := NewSSHSource("root@127.0.0.1", "/var/log/app.log", 100)
	src.SetPort(6022)
	ch, err := src.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	line, ok := <-ch
	if !ok || line.Text != "frp line one" {
		t.Fatalf("带端口应正常出流: %+v", line)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/stream/ -run 'TestSSHCommandPrefix|TestSSHSourceWithPort' -v`
Expected: FAIL

- [ ] **Step 3: 实现**

ssh.go 修改点：

1. 新增函数（insertSSHOpt 之后）：

```go
// sshCommandPrefix 组装 ssh 命令前缀（host 前的选项；port>0 时加 -p）。
func sshCommandPrefix(host string, port int) []string {
	args := []string{"ssh", "-o", "ConnectTimeout=8"}
	if port > 0 {
		args = insertSSHOpt(args, "-p", strconv.Itoa(port))
	}
	return append(args, host)
}
```

2. `SSHSource` 结构体加字段 `port int`（`host` 下方），新增方法：

```go
// SetPort 指定 ssh 端口（frp 隧道场景 127.0.0.1:bindPort）。
func (s *SSHSource) SetPort(p int) { s.port = p }
```

3. `stream()` 中 `args := []string{"ssh", "-o", "ConnectTimeout=8", s.host}` 替换为：

```go
args := sshCommandPrefix(s.host, s.port)
```

4. `SSHListDir` 原函数体整体移入新函数并替换 args 行，原函数改委托：

```go
// SSHListDir 列出远程目录内容（默认端口）。
func SSHListDir(host, path, password string) ([]SSHDirEntry, error) {
	return SSHListDirWithPort(host, 0, path, password)
}

// SSHListDirWithPort 列出远程目录内容（指定端口，frp 隧道场景）。
func SSHListDirWithPort(host string, port int, path, password string) ([]SSHDirEntry, error) {
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		path = "/"
	}
	args := append(sshCommandPrefix(host, port),
		fmt.Sprintf("ls -1 -F %s 2>/dev/null", shellQuote(path)))
	// ……以下原 SSHListDir 逻辑不变（cmd 构建起，最后一行返回）
}
```

5. import 增加 `"strconv"`

- [ ] **Step 4: 运行确认通过 + 回归**

Run: `go test ./internal/stream/ -v`
Expected: 全部 PASS

- [ ] **Step 5: Commit**

```bash
git add internal/stream/ssh.go internal/stream/ssh_test.go
git commit -m "feat(stream): SSHSource/SSHListDir 支持指定端口（frp 隧道用）"
```

---

### Task 4: FRPSource（internal/stream/frp.go）

**Files:**
- Create: `internal/stream/frp.go`
- Test: `internal/stream/frp_test.go`

**Interfaces:**
- Consumes: Task 3 的 `NewSSHSource/SetPort/SetPassword/Password/Path`
- Produces: `stream.NewFRPSource(name string, t frpTunnelHandle, user, path string, tailLines int) *FRPSource`（`frpTunnelHandle` 为本文件定义的接口，`*frp.Tunnel` 天然满足）；`FRPSource` 实现 `LogStream` + `SetPassword/Password/Host/Path`（`Host()` 返回 `"frp:"+name`）

- [ ] **Step 1: 写失败测试**

```go
// internal/stream/frp_test.go
package stream

import (
	"context"
	"testing"
	"time"
)

type fakeTunnel struct {
	port    int
	cleaned bool
}

func (f *fakeTunnel) LocalPort() int { return f.port }
func (f *fakeTunnel) Cleanup() error { f.cleaned = true; return nil }

func TestFRPSourceStreamsAndCleansTunnel(t *testing.T) {
	bin := fakeSSHScript(t, "frp tail line")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	tun := &fakeTunnel{port: 6022}
	src := NewFRPSource("client-a", tun, "root", "/var/log/a.log", 100)

	if got := src.Label(); got != "frp://client-a/var/log/a.log" {
		t.Fatalf("label 应为 frp://client-a/var/log/a.log，实际 %s", got)
	}
	if src.Host() != "frp:client-a" || src.Path() != "/var/log/a.log" {
		t.Fatalf("Host/Path 不符: %s %s", src.Host(), src.Path())
	}

	ch, err := src.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	select {
	case l, ok := <-ch:
		if !ok || l.Text != "frp tail line" {
			t.Fatalf("应流出 frp tail line: %+v", l)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}

	src.SetPassword("pw")
	if src.Password() != "pw" {
		t.Fatal("SetPassword 应委托 inner")
	}
	if err := src.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if !tun.cleaned {
		t.Fatal("Cleanup 应清理隧道")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/stream/ -run TestFRPSource -v`
Expected: FAIL

- [ ] **Step 3: 实现**

```go
// internal/stream/frp.go
package stream

// FRPSource：frp stcp 隧道 + SSH tail。隧道由 TUI 侧建好后移交进来，
// Cleanup 时连带杀 frpc visitor。认证/错误上屏逻辑全部复用 SSHSource。

import (
	"context"
	"fmt"

	"github.com/justfun/logview/internal/model"
)

// frpTunnelHandle 隧道能力（*frp.Tunnel 实现；测试用 fake）。
type frpTunnelHandle interface {
	LocalPort() int
	Cleanup() error
}

type FRPSource struct {
	tunnel frpTunnelHandle
	inner  *SSHSource
	name   string // 记录名（label 与密码缓存 key）
}

func NewFRPSource(name string, t frpTunnelHandle, user, path string, tailLines int) *FRPSource {
	inner := NewSSHSource(user+"@127.0.0.1", path, tailLines)
	inner.SetPort(t.LocalPort())
	return &FRPSource{tunnel: t, inner: inner, name: name}
}

func (s *FRPSource) Label() string { return fmt.Sprintf("frp://%s%s", s.name, s.inner.path) }

func (s *FRPSource) Start(ctx context.Context) (<-chan model.RawLine, error) {
	return s.inner.Start(ctx)
}

func (s *FRPSource) Cleanup() error {
	s.inner.Cleanup()
	return s.tunnel.Cleanup()
}

func (s *FRPSource) SetPassword(pw string) { s.inner.SetPassword(pw) }
func (s *FRPSource) Password() string      { return s.inner.Password() }
func (s *FRPSource) Host() string          { return "frp:" + s.name }
func (s *FRPSource) Path() string          { return s.inner.Path() }
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/stream/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/stream/frp.go internal/stream/frp_test.go
git commit -m "feat(stream): FRPSource（frp 隧道 + SSH tail 组合源）"
```

---

### Task 5: TUI 基础——tab 扩容 + FRP 状态字段 + L0 连接列表

**Files:**
- Modify: `internal/tui/app.go`（状态字段 113-144 行区域、closeSourcePicker）
- Modify: `internal/tui/sourcepicker_ui.go`（tabs/取模/openSourcePicker/visiblePickerCandidates/pickerInputRef/buildSourcePickerLines L0）
- Modify: `internal/tui/sourcepicker.go`（frpConnCandidates/SetFRPStore）
- Modify: `internal/tui/usage.go`（usageFRPConn key）
- Test: `internal/tui/frppicker_test.go`

**Interfaces:**
- Consumes: Task 1 的 `frp.Store/LoadStore/Conn/Server`
- Produces: `tui.SetFRPStore(s *frp.Store)`；`frpConnCandidates(filter string) []sourceCandidate`；App 字段 `pickerFRPLevel/pickerFRPStep/pickerFRPInput/pickerFRPCursor/pickerFRPServerName/pickerFRPServerAddr/pickerFRPSK/pickerFRPProxy/pickerFRPUser/pickerFRPDir/pickerFRPConnName/pickerFRPTunnel`；`const usageFRPConn = "frp:"`；tui 侧接口 `frpTunnelHandle`；`var startFRPTunnel`（Task 7 使用）

- [ ] **Step 1: 写失败测试**

```go
// internal/tui/frppicker_test.go
package tui

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justfun/logview/internal/frp"
)

type fakeFRPTunnel struct {
	port    int
	cleaned bool
}

func (f *fakeFRPTunnel) LocalPort() int { return f.port }
func (f *fakeFRPTunnel) Cleanup() error { f.cleaned = true; return nil }

func setupFRPStore(t *testing.T, conns ...frp.Conn) {
	t.Helper()
	frp.SetStoreFileForTest(filepath.Join(t.TempDir(), "frp.json"))
	t.Cleanup(frp.ResetStoreForTest)
	st := frp.LoadStore()
	st.UpsertServer(frp.Server{Name: "s1", Addr: "frps.example.com:7000", Token: "tk"})
	for _, c := range conns {
		st.UpsertConn(c)
	}
	SetFRPStore(st)
}

func typeRunes(app *App, s string) {
	for _, r := range s {
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

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

func TestSourcePickerFourTabs(t *testing.T) {
	app := newTestApp()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	for i := 0; i < 3; i++ {
		app.Update(tea.KeyMsg{Type: tea.KeyTab})
	}
	if app.sourceTab != 0 {
		t.Fatalf("三次 Tab 应回到 0，实际 %d", app.sourceTab)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyTab})
	if app.sourceTab != 1 {
		t.Fatalf("四次 Tab 应到 1（说明 %% 4 生效），实际 %d", app.sourceTab)
	}
}

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
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/tui/ -run 'FRP|FourTabs' -v`
Expected: FAIL（字段/函数未定义）

- [ ] **Step 3: 实现**

**usage.go**（key 前缀块追加）：
```go
	usageFRPConn = "frp:"
```

**sourcepicker.go** 追加（import 增加 frp 包）：
```go
// frpStoreRef 全局注入（loadConfig 时 set；测试用 SetFRPStore 覆盖）。
var frpStoreRef *frp.Store

// SetFRPStore 注入 frp 连接存储。
func SetFRPStore(s *frp.Store) { frpStoreRef = s }

func frpStore() *frp.Store {
	if frpStoreRef == nil {
		return frp.LoadStore()
	}
	return frpStoreRef
}

// frpTunnelHandle tui 侧隧道句柄（*frp.Tunnel / 测试 fake 均实现）。
type frpTunnelHandle interface {
	LocalPort() int
	Cleanup() error
}

// frpConnCandidates FRP L0 候选：+ 新建连接 恒在首位，其后已存记录（频次降序）。
func frpConnCandidates(filter string) []sourceCandidate {
	var conns []sourceCandidate
	for _, c := range frpStore().Conns {
		conns = append(conns, sourceCandidate{
			label: c.Name + "  " + c.Proxy + " " + c.Path,
			value: c.Name,
		})
	}
	conns = sortCandidatesHot(conns, usageFRPConn, true)
	out := []sourceCandidate{{label: "+ 新建连接", value: "+new"}}
	f := strings.ToLower(strings.TrimSpace(filter))
	for _, c := range conns {
		if f == "" || strings.Contains(strings.ToLower(c.label), f) {
			out = append(out, c)
		}
	}
	return out
}
```

**app.go**：
- `sourceTab` 注释改 `// 0=K8s 1=本地 2=SSH 3=FRP`
- SSH picker 字段块后追加：

```go
	pickerFRPLevel      int           // FRP 层级：0=连接列表 1=新建表单 2=远程目录
	pickerFRPStep       int           // 表单步骤：0=选服务器 1=新地址 2=token 3=sk 4=proxy 5=user
	pickerFRPInput      string        // FRP 搜索/表单输入
	pickerFRPCursor     int
	pickerFRPServerName string        // 已选/新建的服务器名
	pickerFRPServerAddr string        // 新服务器地址（表单暂存）
	pickerFRPSK         string
	pickerFRPProxy      string
	pickerFRPUser       string
	pickerFRPDir        string        // 远程浏览当前目录
	pickerFRPConnName   string        // 直达场景选中的记录名（空=新建）
	pickerFRPTunnel     frpTunnelHandle // 浏览期间常驻隧道（确认后移交 FRPSource）
```

- `closeSourcePicker` 改为：
```go
func (a *App) closeSourcePicker() {
	a.sourcePickerMode = false
	a.pickerCandidates = nil
	a.pickerChecked = nil
	// 未移交的 frp 隧道随弹窗关闭清理
	if a.pickerFRPTunnel != nil {
		a.pickerFRPTunnel.Cleanup()
		a.pickerFRPTunnel = nil
	}
}
```

**sourcepicker_ui.go**：
- `openSourcePicker` 末尾追加（**不重置 pickerFRPTunnel**——密码流程重开 picker 需保留；清理统一走 closeSourcePicker）：
```go
	a.pickerFRPLevel = 0
	a.pickerFRPStep = 0
	a.pickerFRPInput = ""
	a.pickerFRPCursor = 0
	a.pickerFRPServerName = ""
	a.pickerFRPServerAddr = ""
	a.pickerFRPSK = ""
	a.pickerFRPProxy = ""
	a.pickerFRPUser = ""
	a.pickerFRPDir = ""
	a.pickerFRPConnName = ""
```
- `handleSourcePickerKeys`：KeyTab `% 3` → `% 4`；KeyShiftTab `(a.sourceTab + 2) % 3` → `(a.sourceTab + 3) % 4`
- `visiblePickerCandidates` switch 追加：
```go
	case 3: // FRP
		return frpConnCandidates(a.pickerFRPInput)
```
- `pickerInputRef` switch 追加：
```go
	case 3:
		return inputRef{&a.pickerFRPInput, &a.pickerFRPCursor}
```
- `buildSourcePickerLines`：tabs 数组改 `[]string{"K8s", "本地", "SSH", "FRP"}`；switch 追加 case 3（本任务只渲染 L0）：
```go
	case 3:
		content.WriteString(a.inputLine(a.pickerFRPInput, a.pickerFRPCursor, "搜索连接（名称/proxy/路径）…") + "\n\n")
		if len(cands) == 0 {
			content.WriteString(PopupTabStyle.Render(" 无保存的连接") + "\n")
		} else {
			content.WriteString(renderCandidateList(cands, a.pickerCursor, nil, 10))
		}
		content.WriteString("\n" + PopupTabStyle.Render(" Enter打开/新建 C-j/k移动 Esc取消"))
```
- `pickerTabEnterCmd` / `pickerBackspace` / `pickerEnter` / `confirmSourcePicker` 本任务不加 case 3（Task 6/7/8 处理；Go switch 无匹配 case 不 panic）

- [ ] **Step 4: 运行确认通过 + 回归**

Run: `go test ./internal/tui/ -v`
Expected: 全部 PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/
git commit -m "feat(tui): 源选择器扩为 4 tab，FRP L0 连接列表（搜索+频次排序）"
```

---

### Task 6: TUI 新建表单（step 状态机 + 服务器新增保存）

**Files:**
- Modify: `internal/tui/sourcepicker_ui.go`（pickerEnter case 3、pickerBackspace case 3、visiblePickerCandidates/buildSourcePickerLines 的 level 1 分支）
- Modify: `internal/tui/sourcepicker.go`（frpServerCandidates）
- Test: `internal/tui/frppicker_test.go`（追加）

**Interfaces:**
- Consumes: Task 5 的字段与 `frpStore()`
- Produces: `frpServerCandidates(filter string) []sourceCandidate`；`func (a *App) pickerFRPFormEnter(cand sourceCandidate) tea.Cmd`

- [ ] **Step 1: 写失败测试（追加）**

```go
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
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/tui/ -run 'TestFRPForm' -v`
Expected: FAIL

- [ ] **Step 3: 实现**

**sourcepicker.go** 追加：
```go
// frpServerCandidates 表单 step0 候选：手动输入 恒在首位，其后已存服务器。
func frpServerCandidates(filter string) []sourceCandidate {
	out := []sourceCandidate{{label: "手动输入新服务器…", value: "+manual"}}
	f := strings.ToLower(strings.TrimSpace(filter))
	for _, s := range frpStore().Servers {
		label := s.Name + "  " + s.Addr
		if f == "" || strings.Contains(strings.ToLower(label), f) {
			out = append(out, sourceCandidate{label: label, value: s.Name})
		}
	}
	return out
}
```

**sourcepicker_ui.go**：
- `visiblePickerCandidates` case 3 改为：
```go
	case 3: // FRP
		switch a.pickerFRPLevel {
		case 0:
			return frpConnCandidates(a.pickerFRPInput)
		case 1:
			if a.pickerFRPStep == 0 {
				return frpServerCandidates(a.pickerFRPInput)
			}
			return nil
		default:
			return nil // L2 目录浏览（Task 7 实现）
		}
```
- `pickerEnter` switch 追加 case 3（level 0/1；level 2 留 Task 7）：
```go
	case 3: // FRP
		switch a.pickerFRPLevel {
		case 0:
			if cand.value == "+new" {
				a.pickerFRPLevel = 1
				a.pickerFRPStep = 0
				a.pickerFRPInput = ""
				a.pickerCursor = 0
				return nil
			}
			return nil // 旧记录直达（Task 8 实现）
		case 1:
			return a.pickerFRPFormEnter(cand)
		}
```
- 新增表单提交函数：
```go
// pickerFRPFormEnter 表单各步骤 Enter 提交（step5 提交建隧道，Task 7 接管）。
func (a *App) pickerFRPFormEnter(cand sourceCandidate) tea.Cmd {
	input := strings.TrimSpace(a.pickerFRPInput)
	next := func(step int) {
		a.pickerFRPStep = step
		a.pickerFRPInput = ""
		a.pickerCursor = 0
	}
	switch a.pickerFRPStep {
	case 0: // 选服务器
		if cand.value == "+manual" {
			next(1)
		} else {
			a.pickerFRPServerName = cand.value
			next(3) // 已存服务器有 token，直接到 sk
		}
	case 1: // 新服务器地址
		if input == "" {
			return nil
		}
		a.pickerFRPServerAddr = input
		a.pickerFRPServerName = input // 默认名 = 地址
		next(2)
	case 2: // token（可空）
		frpStore().UpsertServer(frp.Server{
			Name:  a.pickerFRPServerName,
			Addr:  a.pickerFRPServerAddr,
			Token: input,
		})
		if err := frpStore().Save(); err != nil {
			a.appendErrorLine(fmt.Sprintf("frp 服务器保存失败: %v", err))
		}
		next(3)
	case 3: // sk
		if input == "" {
			return nil
		}
		a.pickerFRPSK = input
		next(4)
	case 4: // proxy
		if input == "" {
			return nil
		}
		a.pickerFRPProxy = input
		next(5)
	case 5: // user → 提交建隧道（Task 7 接管实际拉起）
		if input == "" {
			return nil
		}
		a.pickerFRPUser = input
		a.pickerFRPInput = ""
		return nil // Task 7 将替换为 fetchFRPTunnelCmd
	}
	return nil
}
```
- `pickerBackspace` switch 追加：
```go
	case 3:
		if a.pickerFRPLevel == 1 {
			a.pickerFRPInput = ""
			if a.pickerFRPStep > 0 {
				a.pickerFRPStep--
			} else {
				a.pickerFRPLevel = 0
			}
		}
```
- `buildSourcePickerLines` case 3 扩为 switch（L0 分支保留 Task 5 内容，追加）：
```go
		case 1:
			prompts := []string{
				"选择 frps 服务器", "新服务器地址 host:port", "token（可空）",
				"sk (secret key)", "proxy 名称", "ssh 用户名",
			}
			content.WriteString(DetailLabelStyle.Render(prompts[a.pickerFRPStep]+": ") +
				a.inputLine(a.pickerFRPInput, a.pickerFRPCursor, "") + "\n\n")
			if a.pickerFRPStep == 0 {
				if len(cands) == 0 {
					content.WriteString(PopupTabStyle.Render(" 无保存的服务器") + "\n")
				} else {
					content.WriteString(renderCandidateList(cands, a.pickerCursor, nil, 8))
				}
			}
			content.WriteString("\n" + PopupTabStyle.Render(" Enter下一步 C-j/k移动 Backspace返回 Esc取消"))
```
（import 增加 frp 包）

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/tui/ -run 'TestFRP' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/
git commit -m "feat(tui): FRP 新建表单（服务器选择/新增保存 + 逐字段输入）"
```

---

### Task 7: 隧道异步建立 + L2 远程目录浏览

**Files:**
- Modify: `internal/tui/sourcepicker.go`（startFRPTunnel/frpTunnelMsg/fetchFRPTunnelCmd/fetchFRPDirCmd）
- Modify: `internal/tui/app.go`（Update 的 frpTunnelMsg case、candidatesMsg 追加 frpdir 分支）
- Modify: `internal/tui/sourcepicker_ui.go`（step5 提交、pickerEnter case 3 level 2 + 路径直达、pickerBackspace case 3 level 2、visiblePickerCandidates L2、pickerInputRef case 3、buildSourcePickerLines L2、辅助函数）
- Test: `internal/tui/frppicker_test.go`（追加）

**Interfaces:**
- Consumes: Task 2 `frp.StartTunnel`、Task 3 `stream.SSHListDirWithPort`、Task 6 `pickerFRPFormEnter` 的 step5
- Produces: `var startFRPTunnel func(frp.Server, string, string) (frpTunnelHandle, error)`；`type frpTunnelMsg struct{conn frp.Conn; tunnel frpTunnelHandle; browse bool; err error}`；`fetchFRPTunnelCmd(server frp.Server, conn frp.Conn, browse bool) tea.Cmd`；`fetchFRPDirCmd(user string, port int, path, password string) tea.Cmd`；`(a *App) closeFRPBrowse()`；`(a *App) frpCurName() string`；`(a *App) frpPwKey() string`；`(a *App) filteredFRPCands() []sourceCandidate`

- [ ] **Step 1: 写失败测试（追加）**

```go
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
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/tui/ -run 'TestFRPTunnel|TestFRPFormSubmit' -v`
Expected: FAIL

- [ ] **Step 3: 实现**

**sourcepicker.go** 追加：
```go
// startFRPTunnel 可注入的隧道启动（测试替换；生产包装 frp.StartTunnel）。
var startFRPTunnel = func(server frp.Server, sk, proxy string) (frpTunnelHandle, error) {
	return frp.StartTunnel(server, sk, proxy)
}

type frpTunnelMsg struct {
	conn   frp.Conn
	tunnel frpTunnelHandle
	browse bool // true=进目录浏览 false=直接 tail
	err    error
}

// fetchFRPTunnelCmd 异步建隧道（阻塞最长 10s，必须异步）。
func fetchFRPTunnelCmd(server frp.Server, conn frp.Conn, browse bool) tea.Cmd {
	return func() tea.Msg {
		t, err := startFRPTunnel(server, conn.SK, conn.Proxy)
		return frpTunnelMsg{conn: conn, tunnel: t, browse: browse, err: err}
	}
}

// fetchFRPDirCmd 异步拉取 frp 隧道远端目录。
func fetchFRPDirCmd(user string, port int, path, password string) tea.Cmd {
	return func() tea.Msg {
		entries, err := stream.SSHListDirWithPort(user+"@127.0.0.1", port, path, password)
		if err != nil {
			return candidatesMsg{tab: 3, kind: "frpdir", ns: "frp:" + path, err: err}
		}
		var items []sourceCandidate
		for _, e := range entries {
			if e.IsDir {
				items = append(items, sourceCandidate{label: e.Name + "/", value: e.Name, dir: true})
			} else {
				items = append(items, sourceCandidate{label: e.Name, value: e.Name})
			}
		}
		return candidatesMsg{tab: 3, kind: "frpdir", ns: "frp:" + path, items: items}
	}
}
```

**app.go**：
- `case candidatesMsg:` 的 `switch msg.kind` 追加：
```go
		case "frpdir":
			if a.pickerFRPLevel == 2 && msg.ns == "frp:"+a.pickerFRPDir {
				a.pickerCandidates = msg.items
				// 认证失败 → frp 密码弹窗（Task 9 实现）
			}
```
- Update 追加 case（candidatesMsg 之后）：
```go
	case frpTunnelMsg:
		if msg.err != nil {
			a.pickerLoading = false
			a.appendErrorLine(fmt.Sprintf("frp 隧道建立失败: %v", msg.err))
			return a, nil
		}
		if msg.browse {
			// 进目录浏览（表单提交后 picker 仍开着；直达重开场景同样覆盖）
			a.sourcePickerMode = true
			a.sourceTab = 3
			a.pickerFRPLevel = 2
			a.pickerFRPTunnel = msg.tunnel
			a.pickerFRPUser = msg.conn.User
			a.pickerFRPProxy = msg.conn.Proxy
			if a.pickerFRPConnName == "" {
				a.pickerFRPConnName = msg.conn.Name
			}
			a.pickerFRPDir = "/"
			a.pickerCandidates = nil
			a.pickerCursor = 0
			a.pickerLoading = true
			return fetchFRPDirCmd(msg.conn.User, msg.tunnel.LocalPort(), "/", a.sshPasswords["frp:"+msg.conn.Name])
		}
		// 旧记录直达：直接 tail（Task 8 实现）
		a.pickerLoading = false
		return a, nil
```

**sourcepicker_ui.go**：
- `pickerFRPFormEnter` 的 case 5 替换为：
```go
	case 5: // user → 提交建隧道
		if input == "" {
			return nil
		}
		a.pickerFRPUser = input
		a.pickerFRPInput = ""
		server, ok := frpStore().FindServer(a.pickerFRPServerName)
		if !ok {
			a.appendErrorLine(fmt.Sprintf("frp 服务器 %s 不存在", a.pickerFRPServerName))
			return nil
		}
		conn := frp.Conn{Name: a.pickerFRPProxy, Server: a.pickerFRPServerName,
			SK: a.pickerFRPSK, Proxy: a.pickerFRPProxy, User: input}
		a.pickerLoading = true
		return fetchFRPTunnelCmd(server, conn, true)
```
- 辅助函数（pickerBackspace 之前）：
```go
// frpCurName 当前 frp 连接名（记录名或 proxy 名）。
func (a *App) frpCurName() string {
	if a.pickerFRPConnName != "" {
		return a.pickerFRPConnName
	}
	return a.pickerFRPProxy
}

// frpPwKey frp 密码内存缓存 key。
func (a *App) frpPwKey() string { return "frp:" + a.frpCurName() }

// closeFRPBrowse 退出目录浏览：清隧道回连接列表。
func (a *App) closeFRPBrowse() {
	if a.pickerFRPTunnel != nil {
		a.pickerFRPTunnel.Cleanup()
		a.pickerFRPTunnel = nil
	}
	a.pickerFRPLevel = 0
	a.pickerFRPDir = ""
	a.pickerCandidates = nil
	a.pickerCursor = 0
}

// filteredFRPCands FRP 远程目录层候选（同 SSH：无过滤首位 ../，根目录除外）。
func (a *App) filteredFRPCands() []sourceCandidate {
	filter := strings.ToLower(a.pickerDirFilter)
	var items []sourceCandidate
	if filter == "" && a.pickerFRPDir != "/" {
		items = append(items, sourceCandidate{label: "../", value: "..", dir: true})
	}
	for _, c := range a.pickerCandidates {
		if filter == "" || strings.Contains(strings.ToLower(c.label), filter) {
			items = append(items, c)
		}
	}
	return items
}
```
- `visiblePickerCandidates` case 3 的 `default` 分支替换为 `return a.filteredFRPCands()`
- `pickerInputRef` case 3 改为：
```go
	case 3:
		if a.pickerFRPLevel == 2 {
			return inputRef{&a.pickerDirFilter, &a.pickerFilterCursor}
		}
		return inputRef{&a.pickerFRPInput, &a.pickerFRPCursor}
```
- `pickerEnter`：
  - case 3 的 level 1 之后追加 level 2 分支：
```go
		default: // 远程目录浏览
			if a.pickerFRPTunnel == nil {
				return nil
			}
			if cand.value == ".." {
				if a.pickerFRPDir == "/" {
					a.closeFRPBrowse()
					return nil
				}
				a.pickerFRPDir = parentPath(a.pickerFRPDir)
				a.pickerCandidates = nil
				a.pickerCursor = 0
				a.pickerLoading = true
				return fetchFRPDirCmd(a.pickerFRPUser, a.pickerFRPTunnel.LocalPort(), a.pickerFRPDir, a.sshPasswords[a.frpPwKey()])
			}
			if cand.dir {
				a.pickerFRPDir = strings.TrimSuffix(a.pickerFRPDir, "/") + "/" + cand.value
				a.pickerCandidates = nil
				a.pickerCursor = 0
				a.pickerLoading = true
				return fetchFRPDirCmd(a.pickerFRPUser, a.pickerFRPTunnel.LocalPort(), a.pickerFRPDir, a.sshPasswords[a.frpPwKey()])
			}
			a.pickerRemotePath = strings.TrimSuffix(a.pickerFRPDir, "/") + "/" + cand.value
			return a.confirmFRPPicker() // Task 8 实现
```
  - SSH 路径直达块之后追加 FRP 路径直达（pickerEnter 顶部区域）：
```go
	// FRP 目录层：过滤框输入以 / 开头 → 路径直达（同 SSH）
	if a.sourceTab == 3 && a.pickerFRPLevel == 2 && a.pickerFRPTunnel != nil && strings.HasPrefix(a.pickerDirFilter, "/") {
		path := strings.TrimSuffix(a.pickerDirFilter, "/")
		if path == "" {
			path = "/"
		}
		if _, err := stream.SSHListDirWithPort(a.pickerFRPUser+"@127.0.0.1", a.pickerFRPTunnel.LocalPort(), path, a.sshPasswords[a.frpPwKey()]); err != nil {
			// 路径不存在或不可读：当作文件尝试打开
			a.pickerRemotePath = path
			a.pickerDirFilter = ""
			return a.confirmFRPPicker()
		}
		a.pickerFRPDir = path
		a.pickerCandidates = nil
		a.pickerCursor = 0
		a.pickerDirFilter = ""
		a.pickerLoading = true
		return fetchFRPDirCmd(a.pickerFRPUser, a.pickerFRPTunnel.LocalPort(), path, a.sshPasswords[a.frpPwKey()])
	}
```
- `pickerBackspace` case 3 扩展（level 2 返回逻辑）：
```go
	case 3:
		switch a.pickerFRPLevel {
		case 1:
			a.pickerFRPInput = ""
			if a.pickerFRPStep > 0 {
				a.pickerFRPStep--
			} else {
				a.pickerFRPLevel = 0
			}
		case 2:
			if a.pickerFRPTunnel == nil {
				a.pickerFRPLevel = 0
				return nil
			}
			if a.pickerFRPDir != "/" {
				a.pickerFRPDir = parentPath(a.pickerFRPDir)
				a.pickerCandidates = nil
				a.pickerLoading = true
				return fetchFRPDirCmd(a.pickerFRPUser, a.pickerFRPTunnel.LocalPort(), a.pickerFRPDir, a.sshPasswords[a.frpPwKey()])
			}
			a.closeFRPBrowse()
		}
```
- `buildSourcePickerLines` case 3 的 switch 追加 default（L2 渲染）：
```go
		default:
			content.WriteString(DetailLabelStyle.Render(fmt.Sprintf(" frp:%s %s", a.frpCurName(), a.pickerFRPDir)) + "\n")
			content.WriteString(a.inputLine(a.pickerDirFilter, a.pickerFilterCursor, "输入过滤…") + "\n\n")
			if a.pickerLoading && len(cands) == 0 {
				content.WriteString(PopupTabStyle.Render(" 加载中…") + "\n")
			} else if len(cands) == 0 {
				content.WriteString(PopupTabStyle.Render(" 目录为空或不可读") + "\n")
			} else {
				content.WriteString(renderCandidateList(cands, a.pickerCursor, nil, 10))
			}
			content.WriteString("\n" + PopupTabStyle.Render(" 进目录:Enter 选文件:Enter C-j/k移动 Backspace返回 Esc取消"))
```
- 本任务先加 `confirmFRPPicker` 的最小占位实现让编译通过（Task 8 替换）：
```go
// confirmFRPPicker FRP 确认建流（Task 8 完整实现）。
func (a *App) confirmFRPPicker() tea.Cmd { return nil }
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/tui/ -run 'TestFRP' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/
git commit -m "feat(tui): FRP 隧道异步建立 + 远程目录浏览"
```

---

### Task 8: 确认建流 + 记录保存 + 旧记录直达

**Files:**
- Modify: `internal/tui/sourcepicker_ui.go`（confirmFRPPicker 完整实现、pickerEnter case 3 level 0 旧记录直达）
- Modify: `internal/tui/app.go`（frpTunnelMsg browse=false 分支）
- Test: `internal/tui/frppicker_test.go`（追加）

**Interfaces:**
- Consumes: Task 4 `stream.NewFRPSource`、Task 7 的 `fetchFRPTunnelCmd/frpTunnelMsg`
- Produces: `func (a *App) confirmFRPPicker() tea.Cmd`（完整版）

- [ ] **Step 1: 写失败测试（追加）**

```go
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
```
（import 增加 `"os"`）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/tui/ -run 'TestFRPConfirm|TestFRPDirect' -v`
Expected: FAIL

- [ ] **Step 3: 实现**

**sourcepicker_ui.go** — `confirmFRPPicker` 占位替换为完整实现：
```go
// confirmFRPPicker FRP 确认建流：保存记录（含 Path）→ 隧道移交 FRPSource。
func (a *App) confirmFRPPicker() tea.Cmd {
	tunnel := a.pickerFRPTunnel
	if tunnel == nil {
		return nil
	}
	path := strings.TrimSpace(a.pickerRemotePath)
	if path == "" && a.pickerFRPDir != "" {
		path = a.pickerFRPDir
	}
	if path == "" {
		return nil
	}
	name := a.pickerFRPConnName
	if name == "" {
		name = a.pickerFRPProxy
	}
	user := a.pickerFRPUser
	frpStore().UpsertConn(frp.Conn{
		Name: name, Server: a.pickerFRPServerName, SK: a.pickerFRPSK,
		Proxy: a.pickerFRPProxy, User: user, Path: path,
	})
	if err := frpStore().Save(); err != nil {
		a.appendErrorLine(fmt.Sprintf("frp 记录保存失败: %v", err))
	}
	BumpUsage(usageFRPConn + name)
	a.pickerFRPTunnel = nil // 隧道移交 FRPSource，防 closeSourcePicker 清理
	a.closeSourcePicker()
	src := stream.NewFRPSource(name, tunnel, user, path, 200)
	if pw := a.sshPasswords["frp:"+name]; pw != "" {
		src.SetPassword(pw)
	}
	return a.ReplaceStream(src)
}
```

**sourcepicker_ui.go** — `pickerEnter` case 3 level 0 的 `return nil // 旧记录直达` 替换：
```go
			conn, ok := frpStore().FindConn(cand.value)
			if !ok {
				return nil
			}
			server, ok := frpStore().FindServer(conn.Server)
			if !ok {
				a.appendErrorLine(fmt.Sprintf("frp 记录 %s 引用的服务器 %s 不存在", conn.Name, conn.Server))
				return nil
			}
			a.pickerFRPConnName = conn.Name
			a.pickerFRPUser = conn.User
			a.pickerFRPSK = conn.SK
			a.pickerFRPProxy = conn.Proxy
			a.pickerFRPServerName = conn.Server
			a.pickerLoading = true
			return fetchFRPTunnelCmd(server, conn, false)
```

**app.go** — frpTunnelMsg 的 `// 旧记录直达：直接 tail（Task 8 实现）` 块替换：
```go
		// 旧记录直达：直接 tail
		a.pickerLoading = false
		a.closeSourcePicker() // 隧道在 msg 上，close 不会误清
		BumpUsage(usageFRPConn + msg.conn.Name)
		src := stream.NewFRPSource(msg.conn.Name, msg.tunnel, msg.conn.User, msg.conn.Path, 200)
		if pw := a.sshPasswords["frp:"+msg.conn.Name]; pw != "" {
			src.SetPassword(pw)
		}
		return a.ReplaceStream(src)
```

- [ ] **Step 4: 运行确认通过 + 回归**

Run: `go test ./internal/tui/ -v`
Expected: 全部 PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/
git commit -m "feat(tui): FRP 确认建流（记录保存+旧记录直达 tail）"
```

---

### Task 9: 密码流程适配

**Files:**
- Modify: `internal/tui/sshpw.go`（maybePromptSSHPassword 接口化、promptFRPPw、confirmSSHPw/closeSSHPw frp 分支）
- Modify: `internal/tui/app.go`（sshPwUser/sshPwFRPPort 字段、resetViewState/restartCurrentStream 重构）
- Test: `internal/tui/frppicker_test.go`（追加）

**Interfaces:**
- Consumes: Task 4 `FRPSource.SetPassword/Host/Path`、Task 7 `fetchFRPDirCmd`
- Produces: `func (a *App) restartCurrentStream() tea.Cmd`；`func (a *App) resetViewState()`；`func (a *App) promptFRPPw()`；App 字段 `sshPwUser string`、`sshPwFRPPort int`

- [ ] **Step 1: 写失败测试（追加）**

```go
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
```
（import 增加 `"github.com/justfun/logview/internal/stream"`、`"fmt"`）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/tui/ -run 'TestFRPTail|TestFRPBrowse' -v`
Expected: FAIL

- [ ] **Step 3: 实现**

**app.go**：
- 密码字段区追加：
```go
	sshPwUser    string // frp 场景的 ssh 用户名（密码确认后重拉目录用）
	sshPwFRPPort int    // >0 = frp 浏览来源的密码框（端口随隧道，需单独记）
```
- `ReplaceStream` 中视图重置块抽为 `resetViewState()`，`ReplaceStream` 调用之：
```go
// resetViewState 清屏重置全部视图状态（ReplaceStream/restartCurrentStream 共用）。
func (a *App) resetViewState() {
	a.buffer.Clear()
	a.filteredView = nil
	a.stGroups = nil
	a.expanded = make(map[int]bool)
	a.levelCounts = make(map[string]int)
	a.bookmarks = make(map[uint64]bool)
	a.bookmarkSeq = nil
	a.cursor = 0
	a.offset = 0
	a.autoscroll = true
}

// restartCurrentStream 当前源带新参数重启（frp 密码重连；不 Cleanup 旧源，保留隧道）。
func (a *App) restartCurrentStream() tea.Cmd {
	if a.cancelFunc != nil {
		a.cancelFunc()
	}
	a.resetViewState()
	ctx, cancel := context.WithCancel(context.Background())
	a.cancelFunc = cancel
	ch, err := a.stream.Start(ctx)
	if err != nil {
		cancel()
		a.cancelFunc = nil
		a.appendErrorLine(fmt.Sprintf("重启源失败 %s: %v", a.stream.Label(), err))
		return nil
	}
	a.streamCh = ch
	a.yankMsg = ""
	return waitForStream(ch)
}
```
（`ReplaceStream` 原来那 10 行重置代码删除，改为调用 `a.resetViewState()`，其余不变）

**sshpw.go**：
- `maybePromptSSHPassword` 的类型断言接口化：
```go
	// 从当前 SSH/FRP 源取 host/path 重连
	src, ok := a.stream.(interface {
		Host() string
		Path() string
	})
```
- `closeSSHPw` 扩展：
```go
func (a *App) closeSSHPw() {
	a.sshPwMode = false
	a.sshPwInput = ""
	a.sshPwCursor = 0
	a.sshPwFromPicker = false
	if a.sshPwFRPPort > 0 {
		a.sshPwFRPPort = 0
		// 取消：浏览中的隧道一并清理（picker 已关，无人接管）
		if a.pickerFRPTunnel != nil {
			a.pickerFRPTunnel.Cleanup()
			a.pickerFRPTunnel = nil
		}
	}
}
```
- `confirmSSHPw` 重写：
```go
// confirmSSHPw 密码确认分流：frp 浏览 → 存密码回 FRP 目录层；
// picker 目录浏览来源 → 存密码回到 SSH 目录浏览；tail 流来源 → 带密码重连。
func (a *App) confirmSSHPw() tea.Cmd {
	host, path, pw := a.sshPwHost, a.sshPwPath, a.sshPwInput
	fromPicker := a.sshPwFromPicker
	frpPort, frpUser := a.sshPwFRPPort, a.sshPwUser
	a.sshPwFRPPort = 0 // 先清标记，防 closeSSHPw 误杀隧道
	a.closeSSHPw()
	if pw == "" || host == "" {
		return nil
	}
	// 密码入内存缓存（后续同主机连接/浏览免重输）
	if a.sshPasswords == nil {
		a.sshPasswords = make(map[string]string)
	}
	a.sshPasswords[host] = pw

	if frpPort > 0 {
		// frp 浏览来源：重开 FRP 目录层，带密码重拉（隧道复用）
		t := a.pickerFRPTunnel
		a.openSourcePicker(3)
		a.pickerFRPLevel = 2
		a.pickerFRPTunnel = t
		a.pickerFRPDir = path
		a.pickerFRPUser = frpUser
		a.pickerFRPProxy = strings.TrimPrefix(host, "frp:")
		a.pickerFRPConnName = strings.TrimPrefix(host, "frp:")
		a.pickerCandidates = nil
		a.pickerCursor = 0
		a.pickerLoading = true
		return fetchFRPDirCmd(frpUser, frpPort, path, pw)
	}
	if fromPicker {
		// 回到 picker 的远程目录层，带密码重拉目录列表（原 SSH 逻辑不变）
		a.openSourcePicker(2)
		a.pickerSSHHost = host
		a.pickerSSHDir = path
		a.pickerSSHRoot = path
		a.pickerCandidates = nil
		a.pickerCursor = 0
		a.pickerLoading = true
		return fetchSSHDirCmd(host, path, pw)
	}
	// frp tail 来源：原源带密码重启（隧道复用，不能走 ReplaceStream 的 Cleanup）
	if strings.HasPrefix(host, "frp:") {
		if src, ok := a.stream.(*stream.FRPSource); ok {
			src.SetPassword(pw)
			return a.restartCurrentStream()
		}
		return nil
	}
	src := stream.NewSSHSource(host, path, 200)
	src.SetPassword(pw)
	return a.ReplaceStream(src)
}
```
- 新增：
```go
// promptFRPPw frp 目录浏览认证失败：弹密码框（隧道保留，确认后继续浏览）。
func (a *App) promptFRPPw() {
	if a.pickerFRPTunnel == nil || a.pickerFRPUser == "" {
		return
	}
	t := a.pickerFRPTunnel
	a.pickerFRPTunnel = nil // 暂摘下，防 closeSourcePicker 清理
	a.closeSourcePicker()
	a.pickerFRPTunnel = t
	a.sshPwHost = a.frpPwKey()
	a.sshPwPath = a.pickerFRPDir
	a.sshPwUser = a.pickerFRPUser
	a.sshPwFRPPort = t.LocalPort()
	a.sshPwFromPicker = true
	a.sshPwMode = true
	a.sshPwInput = ""
	a.sshPwCursor = 0
}
```
- `candidatesMsg` 的 frpdir 分支（app.go）补上提示触发：
```go
		case "frpdir":
			if a.pickerFRPLevel == 2 && msg.ns == "frp:"+a.pickerFRPDir {
				a.pickerCandidates = msg.items
				if msg.err != nil && len(msg.items) == 0 &&
					strings.Contains(strings.ToLower(msg.err.Error()), "permission denied") {
					a.promptFRPPw()
				}
			}
```

- [ ] **Step 4: 运行确认通过 + 回归（重点：原 SSH 密码测试）**

Run: `go test ./internal/tui/ -v`
Expected: 全部 PASS（含原有 sshpw_test.go）

- [ ] **Step 5: Commit**

```bash
git add internal/tui/
git commit -m "feat(tui): FRP 密码认证流程（浏览/尾随场景，隧道复用）"
```

---

### Task 10: root.go 注入 + 全量验证

**Files:**
- Modify: `cmd/root.go`（loadConfig 注入 frp store）
- Test: 手动验收（无自动化，见清单）

**Interfaces:**
- Consumes: Task 5 `tui.SetFRPStore`、Task 1 `frp.LoadStore`

- [ ] **Step 1: 实现**

`cmd/root.go` 的 `loadConfig()` 中，`tui.SetSSHHosts(...)` 之后追加（import 增加 `"github.com/justfun/logview/internal/frp"`）：
```go
	// frp 连接存储注入（~/.local/state/logview/frp.json，缺失自动降级空 store）
	tui.SetFRPStore(frp.LoadStore())
```

- [ ] **Step 2: 全量自动化验证**

Run: `go test ./... && go vet ./... && make build`
Expected: 全部 PASS、构建成功

- [ ] **Step 3: 手动验收（需要真实 frps/frpc 环境，用户执行）**

```text
1. ./logview picker → Tab 切到 FRP tab
2. 新建连接：选服务器（或手动输入新地址+token）→ sk → proxy → user → 应自动建隧道并进入远程目录浏览
3. 浏览选日志文件 → tail；重启 logview → FRP tab 应能看到刚存的记录（★ 热点标记）
4. 选中旧记录 Enter → 直接 tail（不再浏览目录）
5. 搜索框输入关键字 → 过滤记录
6. 同时打开两个 frp 记录（先开 A，o 重新打开选 B）→ 两条隧道并存，A 的流被正常清理
7. PATH 无 frpc 的机器上新建连接 → 明确报错「未找到 frpc」
8. 远端 ssh 需密码的记录：目录浏览阶段 → 应弹密码框，输入后继续浏览；tail 阶段认证失败 → 弹密码框重连
9. cat ~/.local/state/logview/frp.json → 检查 servers/connections 内容
```

- [ ] **Step 4: Commit**

```bash
git add cmd/root.go
git commit -m "feat: 启动时注入 frp 连接存储"
```

---

## Self-Review 记录

- Spec 覆盖：表单参数（frps 选择/新增+token/sk/proxy/user）→ Task 6/7；端口自动分配 → Task 2；首连浏览+整体记忆 → Task 7/8；旧记录搜索直达 → Task 5/8；独立于 ~/.ssh/config → 全局约束；错误处理四场景 → Task 2（frpc 缺失/隧道失败）、Task 1（json 损坏降级）、Task 9（认证失败密码框）
- 类型一致性：`frpTunnelHandle` 在 stream（Task 4）与 tui（Task 5）各定义一份（结构化接口，`*frp.Tunnel` 均满足）；`fetchFRPDirCmd(user, port, path, password)` 签名在 Task 7 定义、Task 9 使用一致
- 已知简化：记录删除不提供（可直接编辑 frp.json）；同名 proxy 记录会被 Upsert 覆盖（视为更新）

