package stream

import (
	"context"
	"os"
	"path/filepath"
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
