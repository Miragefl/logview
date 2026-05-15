package stream

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTailSourceReadsFile(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.log")
	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(fpath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	src := NewTailSource([]string{fpath})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	var lines []string
	timeout := time.After(2 * time.Second)
	for len(lines) < 3 {
		select {
		case raw := <-ch:
			lines = append(lines, raw.Text)
		case <-timeout:
			t.Fatalf("timed out, got %d/3 lines", len(lines))
		}
	}

	if lines[0] != "line1" {
		t.Errorf("lines[0] = %q, want %q", lines[0], "line1")
	}
	if lines[1] != "line2" {
		t.Errorf("lines[1] = %q, want %q", lines[1], "line2")
	}
	if lines[2] != "line3" {
		t.Errorf("lines[2] = %q, want %q", lines[2], "line3")
	}
}

func TestTailSourceLabel(t *testing.T) {
	src := NewTailSource([]string{"/var/log/a.log", "/var/log/b.log"})
	if src.Label() != "tail" {
		t.Errorf("Label() = %q, want %q", src.Label(), "tail")
	}
}

func TestTailSourceTailsNewLines(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.log")

	// Start with empty file
	if err := os.WriteFile(fpath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	src := NewTailSource([]string{fpath})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Wait a bit for the source to start reading
	time.Sleep(100 * time.Millisecond)

	// Write new lines
	f, err := os.OpenFile(fpath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("new1\nnew2\nnew3\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Wait for lines to be read
	time.Sleep(100 * time.Millisecond)

	var lines []string
	timeout := time.After(2 * time.Second)
	for len(lines) < 3 {
		select {
		case raw := <-ch:
			lines = append(lines, raw.Text)
		case <-timeout:
			t.Fatalf("timed out, got %d/3 lines", len(lines))
		}
	}

	if lines[0] != "new1" {
		t.Errorf("lines[0] = %q, want %q", lines[0], "new1")
	}
	if lines[1] != "new2" {
		t.Errorf("lines[1] = %q, want %q", lines[1], "new2")
	}
	if lines[2] != "new3" {
		t.Errorf("lines[2] = %q, want %q", lines[2], "new3")
	}
}

func TestTailSourceCleanup(t *testing.T) {
	src := NewTailSource([]string{"/tmp/test.log"})
	err := src.Cleanup()
	if err != nil {
		t.Errorf("Cleanup() error: %v", err)
	}
}