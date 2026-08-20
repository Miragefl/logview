package stream

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync/atomic"

	"github.com/justfun/logview/internal/model"
)

// SSHSource tails a remote file over the system ssh client.
// Auth is fully delegated to ssh (~/.ssh/config aliases, keys, agent, ProxyJump).
type SSHSource struct {
	host      string
	path      string
	tailLines int
	seq       atomic.Uint64
}

func NewSSHSource(host, path string, tailLines int) *SSHSource {
	return &SSHSource{host: host, path: path, tailLines: tailLines}
}

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
	args := []string{
		"-o", "ConnectTimeout=8",
		"-o", "BatchMode=yes", // 免交互：密钥/agent 认证，失败直接报错而非挂起等密码
		s.host,
	}
	if s.tailLines > 0 {
		args = append(args, fmt.Sprintf("tail -n %d -F %s", s.tailLines, shellQuote(s.path)))
	} else {
		args = append(args, fmt.Sprintf("tail -F %s", shellQuote(s.path)))
	}

	cmd := exec.CommandContext(ctx, "ssh", args...)
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

	// stderr 每行即时上屏（不等进程退出）
	go func() {
		escan := bufio.NewScanner(stderr)
		escan.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for escan.Scan() {
			s.emitErr(ctx, ch, escan.Text())
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

func (s *SSHSource) Cleanup() error { return nil }
