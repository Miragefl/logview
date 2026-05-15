package stream

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPipeSourceReadsLines(t *testing.T) {
	input := "line1\nline2\nline3\n"
	r := strings.NewReader(input)
	src := NewPipeSource(r)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	var lines []string
	for i := 0; i < 3; i++ {
		select {
		case raw := <-ch:
			lines = append(lines, raw.Text)
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for line")
		}
	}

	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	if lines[0] != "line1" {
		t.Errorf("lines[0] = %q, want %q", lines[0], "line1")
	}
	if lines[2] != "line3" {
		t.Errorf("lines[2] = %q, want %q", lines[2], "line3")
	}
	if src.Label() != "pipe" {
		t.Errorf("Label() = %q, want %q", src.Label(), "pipe")
	}
}

func TestPipeSourceContextCancel(t *testing.T) {
	r := strings.NewReader("line1\n")
	src := NewPipeSource(r)

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	<-ch // read the one line
	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("channel should be closed after cancel")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for channel close")
	}
}