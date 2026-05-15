package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/justfun/logview/internal/buffer"
	"github.com/justfun/logview/internal/model"
	"github.com/justfun/logview/internal/parser"
	"github.com/justfun/logview/internal/stacktrace"
	"github.com/justfun/logview/internal/stream"
	"github.com/justfun/logview/testutil"
)

func TestPipeToBuffer(t *testing.T) {
	lines := testutil.JavaLogbackLines()
	input := strings.Join(lines, "\n") + "\n"
	r := strings.NewReader(input)

	src := stream.NewPipeSource(r)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}

	rules := []parser.RuleConfig{
		{
			Name:    "java-logback",
			Pattern: `(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}) \[(?P<thread>[^\]]+)\] \[(?P<traceId>[^\]]+)\] (?P<level>\w+)\s+(?P<logger>\S+) - (?P<message>.*)`,
		},
	}
	parsers := parser.MustCompileRules(rules)
	ad := parser.NewAutoDetect(parsers)
	buf := buffer.NewRingBuffer(100)

	for i := 0; i < len(lines); i++ {
		select {
		case raw := <-ch:
			if p := ad.Detect(raw); p != nil {
				pl := p.Parse(raw)
				if pl == nil {
					// Create minimal ParsedLine for unmatched lines (e.g., stack traces)
					// Extract the actual message part by looking for "at " or using full text
					msg := raw.Text
					if idx := strings.Index(raw.Text, " at "); idx >= 0 {
						msg = " " + raw.Text[idx+1:] // preserve leading space for stack frame detection
					}
					pl = &model.ParsedLine{
						Raw:     raw,
						Message: msg,
					}
				}
				buf.Push(pl)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out at line %d", i)
		}
	}

	if buf.Len() != 6 {
		t.Fatalf("buffer has %d lines, want 6", buf.Len())
	}

	first := buf.Get(0)
	if first.Level != "INFO" {
		t.Errorf("first line level = %q, want INFO", first.Level)
	}
	if first.TraceID != "abc123" {
		t.Errorf("first line traceId = %q, want abc123", first.TraceID)
	}

	var parsed []*model.ParsedLine
	for i := 0; i < buf.Len(); i++ {
		parsed = append(parsed, buf.Get(i))
	}
	groups := stacktrace.Detect(parsed)
	if len(groups) != 1 {
		t.Fatalf("got %d stack trace groups, want 1", len(groups))
	}
	if groups[0].Start != 2 || groups[0].End != 4 {
		t.Errorf("group = [%d,%d], want [2,4]", groups[0].Start, groups[0].End)
	}
}