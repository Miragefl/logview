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
