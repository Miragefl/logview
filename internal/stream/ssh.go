package stream

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// SSHSource tails a remote file over the system ssh client.
// Auth is fully delegated to ssh (~/.ssh/config aliases, keys, agent, ProxyJump).
// 密码认证：SetPassword 后经 SSH_ASKPASS 临时脚本喂给 ssh（免交互）。
type SSHSource struct {
	host      string
	path      string
	tailLines int
	password  string
	seq       atomic.Uint64
}

func NewSSHSource(host, path string, tailLines int) *SSHSource {
	return &SSHSource{host: host, path: path, tailLines: tailLines}
}

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
	opts := []string{"-o", "BatchMode=yes"} // 免交互：密钥/agent 认证
	if s.password != "" {
		// 密码认证：SSH_ASKPASS 脚本回显密码（环境变量注入，见 startCmd）
		opts = []string{"-o", "BatchMode=no", "-o", "StrictHostKeyChecking=no"}
	}
	args := append([]string{"-o", "ConnectTimeout=8"}, opts...)
	args = append(args, s.host)
	if s.tailLines > 0 {
		args = append(args, fmt.Sprintf("tail -n %d -F %s", s.tailLines, shellQuote(s.path)))
	} else {
		args = append(args, fmt.Sprintf("tail -F %s", shellQuote(s.path)))
	}

	cmd := exec.CommandContext(ctx, "ssh", args...)
	if s.password != "" {
		askpass, cleanup, err := writeAskpassScript()
		if err != nil {
			s.emitErr(ctx, ch, fmt.Sprintf("askpass: %v", err))
			return
		}
		defer cleanup()
		cmd.Env = append(os.Environ(),
			"SSH_ASKPASS="+askpass,
			"SSH_ASKPASS_REQUIRE=force",
			"SSH_PASSWORD="+s.password,
			"DISPLAY=:0", // 部分 ssh 版本要求 DISPLAY 存在才走 askpass
		)
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

// SSHListDir 列出远程目录内容（目录 / 后缀分类），每行一个条目。
// 单发 ssh 命令，BatchMode 保证不挂起。
func SSHListDir(host, path string) ([]SSHDirEntry, error) {
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		path = "/"
	}
	args := []string{"-o", "ConnectTimeout=8", "-o", "BatchMode=yes", host,
		fmt.Sprintf("ls -1 -F %s", shellQuote(path))}
	out, err := exec.Command("ssh", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("ssh ls %s: %s", host, strings.TrimSpace(err.Error()))
	}
	var entries []SSHDirEntry
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
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
	return entries, nil
}

func (s *SSHSource) Cleanup() error { return nil }
