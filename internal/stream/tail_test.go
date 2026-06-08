package stream

import (
	"context"
	"fmt"
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

	src := NewTailSource([]string{fpath}, 0)
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
	src := NewTailSource([]string{"/var/log/a.log", "/var/log/b.log"}, 0)
	if src.Label() != "tail" {
		t.Errorf("Label() = %q, want %q", src.Label(), "tail")
	}
}

func TestTailSourceTailsNewLines(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.log")

	if err := os.WriteFile(fpath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	src := NewTailSource([]string{fpath}, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	f, err := os.OpenFile(fpath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("new1\nnew2\nnew3\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

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
	src := NewTailSource([]string{"/tmp/test.log"}, 0)
	err := src.Cleanup()
	if err != nil {
		t.Errorf("Cleanup() error: %v", err)
	}
}

func TestTailSourceFollowSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.log")
	content := "old1\nold2\nold3\n"
	if err := os.WriteFile(fpath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	src := NewTailSource([]string{fpath}, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	var lines []string
	timeout := time.After(2 * time.Second)
	for len(lines) < 1 {
		select {
		case raw := <-ch:
			lines = append(lines, raw.Text)
		case <-timeout:
			t.Fatalf("timed out, got %d/1 lines: %v", len(lines), lines)
		}
	}

	if lines[0] != "old3" {
		t.Errorf("lines[0] = %q, want %q (should only get last 1 line)", lines[0], "old3")
	}
}

func TestTailSourceFollowLastNLines(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.log")
	var content string
	for i := 1; i <= 10; i++ {
		content += fmt.Sprintf("line%d\n", i)
	}
	if err := os.WriteFile(fpath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	src := NewTailSource([]string{fpath}, 3)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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
			t.Fatalf("timed out, got %d/3 lines: %v", len(lines), lines)
		}
	}

	if lines[0] != "line8" {
		t.Errorf("lines[0] = %q, want %q", lines[0], "line8")
	}
	if lines[1] != "line9" {
		t.Errorf("lines[1] = %q, want %q", lines[1], "line9")
	}
	if lines[2] != "line10" {
		t.Errorf("lines[2] = %q, want %q", lines[2], "line10")
	}
}

func TestTailSourceFollowThenAppend(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.log")
	content := "old1\nold2\nold3\n"
	if err := os.WriteFile(fpath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	src := NewTailSource([]string{fpath}, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	var lines []string
	timeout := time.After(2 * time.Second)
	for len(lines) < 2 {
		select {
		case raw := <-ch:
			lines = append(lines, raw.Text)
		case <-timeout:
			t.Fatalf("timed out reading initial, got %d/2 lines", len(lines))
		}
	}

	if lines[0] != "old2" || lines[1] != "old3" {
		t.Fatalf("initial lines = %v, want [old2, old3]", lines)
	}

	f, err := os.OpenFile(fpath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("new1\nnew2\n")
	f.Close()

	var appended []string
	for len(appended) < 2 {
		select {
		case raw := <-ch:
			appended = append(appended, raw.Text)
		case <-timeout:
			t.Fatalf("timed out reading appended, got %d/2 lines", len(appended))
		}
	}

	if appended[0] != "new1" || appended[1] != "new2" {
		t.Errorf("appended = %v, want [new1, new2]", appended)
	}
}
