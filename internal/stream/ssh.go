package stream

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/justfun/logview/internal/model"
)

// writeAskpassScript 生成临时 askpass 脚本（回显 $SSH_PASSWORD），返回路径与清理函数。
func writeAskpassScript() (string, func(), error) {
	dir, err := os.MkdirTemp("", "logview-askpass-")
	if err != nil {
		return "", nil, err
	}
	path := filepath.Join(dir, "askpass.sh")
	script := "#!/bin/sh\necho \"$SSH_PASSWORD\"\n"
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		os.RemoveAll(dir)
		return "", nil, err
	}
	return path, func() { os.RemoveAll(dir) }, nil
}

// applySSHAuth 按认证方式装配 ssh 命令：password 非空走 askpass（返回清理函数，无需清理时为 nil）。
func applySSHAuth(cmd *exec.Cmd, password string) (func(), error) {
	if password == "" {
		// 免密：BatchMode 直接报错不挂起
		cmd.Args = insertSSHOpt(cmd.Args, "-o", "BatchMode=yes")
		return nil, nil
	}
	askpass, cleanup, err := writeAskpassScript()
	if err != nil {
		return nil, err
	}
	cmd.Args = insertSSHOpt(cmd.Args, "-o", "BatchMode=no", "-o", "StrictHostKeyChecking=no")
	cmd.Env = append(os.Environ(),
		"SSH_ASKPASS="+askpass,
		"SSH_ASKPASS_REQUIRE=force",
		"SSH_PASSWORD="+password,
		"DISPLAY=:0", // 部分 ssh 版本要求 DISPLAY 存在才走 askpass
	)
	return cleanup, nil
}

// insertSSHOpt 在 host 参数（最后一个非选项参数前的位置简化为索引 1 后）插入 ssh 选项。
// 约定：调用时 Args[0]="ssh" Args[1]="-o ConnectTimeout..." 系列，host 其后。
func insertSSHOpt(args []string, opts ...string) []string {
	// 找到第一个非 -o/-值 参数位置（host），选项插在其前
	i := 1
	for i < len(args) {
		if args[i] == "-o" {
			i += 2
			continue
		}
		break
	}
	out := make([]string, 0, len(args)+len(opts))
	out = append(out, args[:i]...)
	out = append(out, opts...)
	out = append(out, args[i:]...)
	return out
}

// sshCommandPrefix 组装 ssh 命令前缀（host 前的选项；port>0 时加 -p）。
// port>0 即 frp 隧道场景（127.0.0.1:bindPort）：端口每次随机分配，host key 永远
// 未知，BatchMode 下会直接 "Host key verification failed"（触发不了密码框）——
// 关闭 host key 校验且不写 known_hosts（随机端口条目只会污染文件）。
func sshCommandPrefix(host string, port int) []string {
	args := []string{"ssh", "-o", "ConnectTimeout=8"}
	if port > 0 {
		args = insertSSHOpt(args, "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null")
		args = insertSSHOpt(args, "-p", strconv.Itoa(port))
	}
	return append(args, host)
}

// SSHSource tails a remote file over the system ssh client.
// Auth is fully delegated to ssh (~/.ssh/config aliases, keys, agent, ProxyJump).
// 密码认证：SetPassword 后经 SSH_ASKPASS 临时脚本喂给 ssh（免交互）。
type SSHSource struct {
	host      string
	port      int
	path      string
	tailLines int
	password  string
	seq       atomic.Uint64
}

func NewSSHSource(host, path string, tailLines int) *SSHSource {
	return &SSHSource{host: host, path: path, tailLines: tailLines}
}

// SetPort 指定 ssh 端口（frp 隧道场景 127.0.0.1:bindPort）。
func (s *SSHSource) SetPort(p int) { s.port = p }

// SetPassword 设置密码认证（TUI 密码框输入后调用）。
func (s *SSHSource) SetPassword(pw string) { s.password = pw }

// Password 返回已设密码（重连时复用）。
func (s *SSHSource) Password() string { return s.password }

// Host / Path 返回连接目标（TUI 密码重连用）。
func (s *SSHSource) Host() string { return s.host }
func (s *SSHSource) Path() string { return s.path }

func (s *SSHSource) Label() string {
	return fmt.Sprintf("ssh://%s%s", s.host, s.path)
}

func (s *SSHSource) Start(ctx context.Context) (<-chan model.RawLine, error) {
	ch := make(chan model.RawLine, 256)
	go func() {
		defer close(ch)
		s.stream(ctx, ch)
	}()
	return ch, nil
}

func (s *SSHSource) stream(ctx context.Context, ch chan<- model.RawLine) {
	args := sshCommandPrefix(s.host, s.port)
	if s.tailLines > 0 {
		args = append(args, fmt.Sprintf("tail -n %d -F %s", s.tailLines, shellQuote(s.path)))
	} else {
		args = append(args, fmt.Sprintf("tail -F %s", shellQuote(s.path)))
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cleanup, err := applySSHAuth(cmd, s.password)
	if err != nil {
		s.emitErr(ctx, ch, fmt.Sprintf("askpass: %v", err))
		return
	}
	if cleanup != nil {
		defer cleanup()
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.emitErr(ctx, ch, fmt.Sprintf("ssh pipe error: %v", err))
		return
	}
	// 本地 ssh 错误（DNS/认证/超时）走 stderr，单独接管为错误行
	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.emitErr(ctx, ch, fmt.Sprintf("ssh stderr pipe error: %v", err))
		return
	}
	if err := cmd.Start(); err != nil {
		s.emitErr(ctx, ch, fmt.Sprintf("ssh %s failed to start: %v", s.host, err))
		return
	}
	defer cmd.Wait()

	// stderr 每行即时上屏（不等进程退出）；忽略已知 ssh 警告横幅
	go func() {
		escan := bufio.NewScanner(stderr)
		escan.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for escan.Scan() {
			text := escan.Text()
			if isSSHBannerNoise(text) {
				continue
			}
			s.emitErr(ctx, ch, text)
		}
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		text := scanner.Text()
		line := model.RawLine{
			Text:   text,
			Source: s.host,
			Seq:    s.seq.Add(1),
		}
		// surface common ssh failures as ERROR-flavored lines for visibility
		if isSSHErrorLine(text) {
			line.Text = fmt.Sprintf("ERROR %s", text)
		}
		select {
		case ch <- line:
		case <-ctx.Done():
			return
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		s.emitErr(ctx, ch, fmt.Sprintf("ssh %s stream error: %v", s.host, err))
	}
}

func (s *SSHSource) emitErr(ctx context.Context, ch chan<- model.RawLine, msg string) {
	line := model.RawLine{Text: "ERROR " + msg, Source: s.host, Seq: s.seq.Add(1)}
	select {
	case ch <- line:
	case <-ctx.Done():
	}
}

// isSSHBannerNoise 过滤 ssh 客户端的已知警告横幅（post-quantum 提示等），不算错误。
func isSSHBannerNoise(text string) bool {
	lower := strings.ToLower(text)
	for _, pat := range []string{
		"warning: connection is not using a post-quantum key exchange",
		"this session may be vulnerable to \"store now, decrypt later\"",
		"the server may need to be upgraded. see https://openssh.com",
		"permanently added '",
	} {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}

func isSSHErrorLine(text string) bool {
	lower := strings.ToLower(text)
	for _, pat := range []string{
		"ssh: connect to host",
		"connection refused",
		"connection reset",
		"permission denied",
		"host key verification failed",
		"could not resolve hostname",
		"no route to host",
		"timed out",
		"tail: cannot open",
		"tail: no such file",
	} {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}

// shellQuote quotes a path for safe inclusion in a remote shell command.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	// paths are user-provided; quote unless purely safe chars
	safe := true
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			r == '/' || r == '.' || r == '_' || r == '-') {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// SSHDirEntry 远程目录条目。
type SSHDirEntry struct {
	Name  string
	IsDir bool
}

// SSHListDir 列出远程目录内容（默认端口）。
func SSHListDir(host, path, password string) ([]SSHDirEntry, error) {
	return SSHListDirWithPort(host, 0, path, password)
}

// SSHListDirWithPort 列出远程目录内容（指定端口，frp 隧道场景）。
// 单发 ssh 命令；password 非空时走 askpass 密码认证。
func SSHListDirWithPort(host string, port int, path, password string) ([]SSHDirEntry, error) {
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		path = "/"
	}
	args := append(sshCommandPrefix(host, port),
		fmt.Sprintf("ls -1 -F %s 2>/dev/null", shellQuote(path)))
	// 远端 ls 错误已丢弃(2>/dev/null)；CombinedOutput 中 stderr 即 ssh 自身错误
	// （认证/连接），供 UI 判断是否弹密码框
	cmd := exec.Command(args[0], args[1:]...)
	cleanup, err := applySSHAuth(cmd, password)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer cleanup()
	}
	// stdout=目录条目；stderr=ssh 自身错误（认证/连接/banner），分离接管
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	var errBuf strings.Builder
	done := make(chan struct{})
	go func() {
		defer close(done)
		escan := bufio.NewScanner(stderr)
		for escan.Scan() {
			text := escan.Text()
			if isSSHBannerNoise(text) {
				continue
			}
			errBuf.WriteString(text)
			errBuf.WriteString("\n")
		}
	}()
	var entries []SSHDirEntry
	scan := bufio.NewScanner(stdout)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" {
			continue
		}
		if strings.HasSuffix(line, "/") {
			entries = append(entries, SSHDirEntry{Name: strings.TrimSuffix(line, "/"), IsDir: true})
		} else if strings.HasSuffix(line, "*") || strings.HasSuffix(line, "@") {
			continue // 可执行/符号链接跳过（日志场景无意义）
		} else {
			entries = append(entries, SSHDirEntry{Name: line})
		}
	}
	werr := cmd.Wait()
	<-done
	if werr != nil {
		return nil, fmt.Errorf("ssh ls %s: %s", host, strings.TrimSpace(errBuf.String()))
	}
	return entries, nil
}

func (s *SSHSource) Cleanup() error { return nil }
