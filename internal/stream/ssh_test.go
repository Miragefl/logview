package stream

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/justfun/logview/internal/model"
)

// fakeSSHScript writes a shell script masquerading as ssh (PATH injected),
// printing lines then optionally an error line.
func fakeSSHScript(t *testing.T, output string) string {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "ssh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ncat <<'EOF'\n"+output+"\nEOF\n"), 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSSHSourceStreamsLines(t *testing.T) {
	bin := fakeSSHScript(t, "line one\nline two\nERROR ssh: connect to host x refused")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	src := NewSSHSource("fakehost", "/var/log/app.log", 100)
	ch, err := src.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	var lines []model.RawLine
	timeout := time.After(3 * time.Second)
	for len(lines) < 3 {
		select {
		case l, ok := <-ch:
			if !ok {
				goto done
			}
			lines = append(lines, l)
		case <-timeout:
			t.Fatalf("timeout, got %d lines", len(lines))
		}
	}
done:
	if lines[0].Text != "line one" || lines[1].Text != "line two" {
		t.Errorf("unexpected lines: %v", lines)
	}
	if lines[2].Source != "fakehost" {
		t.Errorf("source should be host, got %s", lines[2].Source)
	}
	if !strings.HasPrefix(lines[2].Text, "ERROR") {
		t.Errorf("ssh error line should be ERROR-flavored: %q", lines[2].Text)
	}
	if src.Label() != "ssh://fakehost/var/log/app.log" {
		t.Errorf("unexpected label: %s", src.Label())
	}
}

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"/var/log/app.log": "/var/log/app.log",
		"":                 "''",
		"/path/with space": "'/path/with space'",
		"/path/it's":       `'/path/it'\''s'`,
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSSHCommandPrefix(t *testing.T) {
	got := sshCommandPrefix("user@127.0.0.1", 6022)
	want := "ssh -o ConnectTimeout=8 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -p 6022 user@127.0.0.1"
	if strings.Join(got, " ") != want {
		t.Errorf("got %v", got)
	}
	got = sshCommandPrefix("host-a", 0)
	if strings.Join(got, " ") != "ssh -o ConnectTimeout=8 host-a" {
		t.Errorf("port=0 应无 -p（非 frp 场景不动 host key 策略），got %v", got)
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

func TestIsSSHErrorLine(t *testing.T) {
	for _, s := range []string{"ssh: connect to host x port 22: Connection refused", "tail: cannot open '/x' for reading"} {
		if !isSSHErrorLine(s) {
			t.Errorf("should be error: %q", s)
		}
	}
	if isSSHErrorLine("normal log line 200 OK") {
		t.Error("normal line misclassified")
	}
}

// 远端命令拼装:gz 走解压管道(无 -F,行数优先 bufferLines);普通文件零变化。
func TestRemoteTailCommand(t *testing.T) {
	cases := []struct {
		name string
		src  *SSHSource
		want string
	}{
		{"普通+行数", &SSHSource{path: "/var/log/a.log", tailLines: 200},
			"tail -n 200 -F /var/log/a.log"},
		{"普通带空格路径", &SSHSource{path: "/var/log/my app.log", tailLines: 10},
			"tail -n 10 -F '/var/log/my app.log'"},
		{"普通无行数", &SSHSource{path: "/var/log/a.log"},
			"tail -F /var/log/a.log"},
		{"gz+bufferLines", &SSHSource{path: "/var/log/a.log.gz", tailLines: 200, bufferLines: 100000},
			"gzip -dc /var/log/a.log.gz | tail -n 100000"},
		{"gz 兜底 tailLines", &SSHSource{path: "/var/log/a.log.2026-09-05.gz", tailLines: 200},
			"gzip -dc /var/log/a.log.2026-09-05.gz | tail -n 200"},
		{"gz 双零全量", &SSHSource{path: "/var/log/x.gz"},
			"gzip -dc /var/log/x.gz"},
	}
	for _, c := range cases {
		if got := c.src.remoteTailCommand(); got != c.want {
			t.Errorf("%s: remoteTailCommand() = %q, want %q", c.name, got, c.want)
		}
	}
}

// SetBufferLines 注入。
func TestSSHSourceSetBufferLines(t *testing.T) {
	s := NewSSHSource("h", "/x.gz", 200)
	if s.bufferLines != 0 {
		t.Fatalf("默认 bufferLines 应为 0, got %d", s.bufferLines)
	}
	s.SetBufferLines(5000)
	if s.bufferLines != 5000 {
		t.Fatalf("SetBufferLines 后 = %d, want 5000", s.bufferLines)
	}
	// FRPSource 转发注入(inner 拿到同一值)。
	// (NewFRPSource 对 nil 隧道会在构造期调 t.LocalPort() 触发 nil panic,故字面量构造)
	fs := &FRPSource{inner: NewSSHSource("u@127.0.0.1", "/x.gz", 200)}
	fs.SetBufferLines(7777)
	if fs.inner.bufferLines != 7777 {
		t.Fatalf("FRPSource 转发后 inner.bufferLines = %d, want 7777", fs.inner.bufferLines)
	}
}
