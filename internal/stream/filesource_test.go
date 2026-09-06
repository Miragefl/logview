package stream

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileSourceReadsAllLines(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.log")
	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(fpath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	src := NewFileSource([]string{fpath})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	var lines []string
	timeout := time.After(2 * time.Second)
	for {
		select {
		case raw, ok := <-ch:
			if !ok {
				goto done
			}
			lines = append(lines, raw.Text)
		case <-timeout:
			t.Fatalf("timed out, got %d lines", len(lines))
		}
	}
done:
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "line1" || lines[1] != "line2" || lines[2] != "line3" {
		t.Errorf("lines = %v, want [line1 line2 line3]", lines)
	}
}

func TestFileSourceLastLineWithoutNewline(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "no-newline.log")
	if err := os.WriteFile(fpath, []byte("line1\nline2\nno-trailing-newline"), 0644); err != nil {
		t.Fatal(err)
	}

	src := NewFileSource([]string{fpath})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	var lines []string
	timeout := time.After(2 * time.Second)
	for {
		select {
		case raw, ok := <-ch:
			if !ok {
				goto done
			}
			lines = append(lines, raw.Text)
		case <-timeout:
			t.Fatalf("timed out, got %d lines", len(lines))
		}
	}
done:
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if lines[2] != "no-trailing-newline" {
		t.Errorf("last line = %q, want %q", lines[2], "no-trailing-newline")
	}
}

func TestFileSourceOpenErrorReportsLine(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.log")

	src := NewFileSource([]string{missing})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	var lines []string
	timeout := time.After(2 * time.Second)
	for {
		select {
		case raw, ok := <-ch:
			if !ok {
				goto done
			}
			lines = append(lines, raw.Text)
		case <-timeout:
			t.Fatalf("timed out, got %d lines", len(lines))
		}
	}
done:
	if len(lines) != 1 {
		t.Fatalf("expected 1 error line, got %d: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "missing.log") {
		t.Errorf("error line should mention the file, got %q", lines[0])
	}
}

func TestFileSourceChannelCloses(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.log")
	if err := os.WriteFile(fpath, []byte("only\n"), 0644); err != nil {
		t.Fatal(err)
	}

	src := NewFileSource([]string{fpath})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// read one line
	raw, ok := <-ch
	if !ok {
		t.Fatal("expected one line, channel was closed")
	}
	if raw.Text != "only" {
		t.Errorf("text = %q, want %q", raw.Text, "only")
	}

	// channel should close
	_, ok = <-ch
	if ok {
		t.Error("expected channel to be closed after reading all lines")
	}
}

func TestFileSourceLabel(t *testing.T) {
	src := NewFileSource([]string{"/tmp/test.log"})
	if src.Label() != "file" {
		t.Errorf("Label() = %q, want %q", src.Label(), "file")
	}
}

func TestFileSourceMultiFile(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.log")
	f2 := filepath.Join(dir, "b.log")
	os.WriteFile(f1, []byte("a1\na2\n"), 0644)
	os.WriteFile(f2, []byte("b1\nb2\n"), 0644)

	src := NewFileSource([]string{f1, f2})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	var lines []string
	timeout := time.After(2 * time.Second)
	for {
		select {
		case raw, ok := <-ch:
			if !ok {
				goto done
			}
			lines = append(lines, raw.Text)
		case <-timeout:
			t.Fatalf("timed out, got %d lines", len(lines))
		}
	}
done:
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d: %v", len(lines), lines)
	}
}

func TestFileSourceCleanup(t *testing.T) {
	src := NewFileSource([]string{"/tmp/test.log"})
	if err := src.Cleanup(); err != nil {
		t.Errorf("Cleanup() error: %v", err)
	}
}

// gz 文件透明解压;混装普通文件逐个独立嗅探;损坏 gz 出错误行。
func TestFileSourceReadsGzip(t *testing.T) {
	dir := t.TempDir()
	gzPath := filepath.Join(dir, "app.log.gz")
	writeGzip(t, gzPath, "g1\ng2\ng3\n")
	plainPath := filepath.Join(dir, "plain.log")
	os.WriteFile(plainPath, []byte("p1\n"), 0644)

	src := NewFileSource([]string{gzPath, plainPath})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	var lines []string
	timeout := time.After(2 * time.Second)
	for len(lines) < 4 {
		select {
		case raw := <-ch:
			lines = append(lines, raw.Text)
		case <-timeout:
			t.Fatalf("timed out, got %d/4 lines: %v", len(lines), lines)
		}
	}
	want := []string{"g1", "g2", "g3", "p1"}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("第 %d 行 = %q, want %q", i, lines[i], want[i])
		}
	}
}

func TestFileSourceCorruptGzip(t *testing.T) {
	dir := t.TempDir()
	gzPath := filepath.Join(dir, "bad.log.gz")
	writeGzip(t, gzPath, "line1\nline2\n")
	data, _ := os.ReadFile(gzPath)
	os.WriteFile(gzPath, data[:len(data)/2], 0644) // 截断一半:头合法,中途损坏

	src := NewFileSource([]string{gzPath})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ch, _ := src.Start(ctx)

	var lines []string
	timeout := time.After(2 * time.Second)
collect:
	for {
		select {
		case raw, ok := <-ch:
			if !ok {
				break collect
			}
			lines = append(lines, raw.Text)
		case <-timeout:
			break collect
		}
	}
	found := false
	for _, l := range lines {
		if strings.HasPrefix(l, "[logview]") && strings.Contains(l, "bad.log.gz") {
			found = true
		}
	}
	if !found {
		t.Fatalf("损坏 gz 应出 [logview] 错误行, got %v", lines)
	}
}
