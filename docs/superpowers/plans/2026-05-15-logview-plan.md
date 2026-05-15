# LogView Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a terminal TUI tool for real-time log viewing with search, field filtering, multi-source support (k8s/tail/pipe), and stack trace folding.

**Architecture:** Four-layer pipeline: LogStream (data source) → LogParser (regex/json) → RingBuffer (fixed-capacity buffer + search index) → TUI (bubbletea virtual scroll). All layers communicate via Go channels. Each data source implements a common LogStream interface.

**Tech Stack:** Go 1.22+, bubbletea, lipgloss, bubbles, cobra (CLI)

---

## File Structure

```
logview/
├── main.go                          # Entry point
├── cmd/
│   └── root.go                      # cobra CLI commands (k8s/tail/pipe)
├── internal/
│   ├── model/
│   │   └── line.go                  # RawLine, ParsedLine, FieldMask types
│   ├── stream/
│   │   ├── stream.go                # LogStream interface
│   │   ├── pipe.go                  # PipeSource (stdin)
│   │   ├── tail.go                  # TailSource (local files)
│   │   └── k8s.go                   # K8sSource (kubectl wrapper + pod discovery)
│   ├── parser/
│   │   ├── parser.go                # Parser interface + auto-detect
│   │   ├── regex.go                 # RegexParser
│   │   ├── json_parser.go           # JSONParser
│   │   └── rules.go                 # Rule config loading from YAML
│   ├── buffer/
│   │   ├── ring.go                  # RingBuffer
│   │   └── index.go                 # SearchIndex
│   ├── stacktrace/
│   │   └── detector.go              # Stack trace detection & grouping
│   ├── export/
│   │   └── export.go                # Export to file (raw/json)
│   └── tui/
│       ├── app.go                   # Root tea.Model, orchestration
│       ├── keymap.go                # Key bindings
│       ├── style.go                 # lipgloss styles & colors
│       ├── logview.go               # Virtual scroll log list
│       ├── searchbar.go             # Search input bar
│       ├── panel.go                 # Bottom panel (fields/levels/filter tabs)
│       ├── export_dlg.go            # Export dialog overlay
│       └── helpbar.go               # Keyboard shortcuts bar
├── testutil/
│   └── testutil.go                  # Test helpers (golden files, etc.)
├── go.mod
└── go.sum
```

---

### Task 1: Project Scaffold + Core Types

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `internal/model/line.go`
- Test: `internal/model/line_test.go`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/viscum/Documents/code/justfun/ai/log
go mod init github.com/justfun/logview
```

- [ ] **Step 2: Install core dependencies**

```bash
go get github.com/charmbracelet/bubbletea
go get github.com/charmbracelet/lipgloss
go get github.com/charmbracelet/bubbles
go get github.com/spf13/cobra
go get gopkg.in/yaml.v3
```

- [ ] **Step 3: Create core types**

Create `internal/model/line.go`:

```go
package model

import "time"

// RawLine is an unparsed log line from any data source.
type RawLine struct {
	Text  string    // raw text
	Source string   // origin label, e.g. "pod/api-7d8f6-x9k2j"
	Seq   uint64    // monotonic sequence number
}

// Field represents a parsed log field name.
type Field string

const (
	FieldTime    Field = "time"
	FieldLevel   Field = "level"
	FieldThread  Field = "thread"
	FieldTraceID Field = "traceId"
	FieldLogger  Field = "logger"
	FieldMessage Field = "message"
	FieldSource  Field = "source"
)

// AllFields lists all possible parsed fields in display order.
var AllFields = []Field{FieldTime, FieldLevel, FieldThread, FieldTraceID, FieldLogger, FieldMessage, FieldSource}

// ParsedLine is a structured log line produced by a parser.
type ParsedLine struct {
	Raw      RawLine
	Time     time.Time
	Level    string
	Thread   string
	TraceID  string
	Logger   string
	Message  string
	Fields   map[Field]string // all extracted fields
}

// Get returns the value for a given field.
func (p *ParsedLine) Get(f Field) string {
	if p.Fields != nil {
		return p.Fields[f]
	}
	switch f {
	case FieldTime:
		return p.Time.Format("15:04:05.000")
	case FieldLevel:
		return p.Level
	case FieldThread:
		return p.Thread
	case FieldTraceID:
		return p.TraceID
	case FieldLogger:
		return p.Logger
	case FieldMessage:
		return p.Message
	case FieldSource:
		return p.Raw.Source
	}
	return ""
}

// FieldMask controls which fields are visible in the TUI.
type FieldMask map[Field]bool

// DefaultFieldMask returns the default visible fields.
func DefaultFieldMask() FieldMask {
	return FieldMask{
		FieldTime:    true,
		FieldLevel:   true,
		FieldThread:  false,
		FieldTraceID: false,
		FieldLogger:  false,
		FieldMessage: true,
		FieldSource:  true,
	}
}

// IsVisible returns whether a field should be displayed.
func (fm FieldMask) IsVisible(f Field) bool {
	visible, ok := fm[f]
	return ok && visible
}

// Toggle flips the visibility of a field.
func (fm FieldMask) Toggle(f Field) {
	fm[f] = !fm[f]
}

// Level represents a log severity level.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// ParseLevel parses a level string (case-insensitive).
func ParseLevel(s string) Level {
	switch s {
	case "DEBUG", "debug", "DBG":
		return LevelDebug
	case "INFO", "info":
		return LevelInfo
	case "WARN", "warn", "WARNING", "warning":
		return LevelWarn
	case "ERROR", "error", "ERR", "FATAL", "fatal":
		return LevelError
	}
	return LevelInfo
}
```

- [ ] **Step 4: Write tests for core types**

Create `internal/model/line_test.go`:

```go
package model

import (
	"testing"
	"time"
)

func TestParsedLineGet(t *testing.T) {
	now := time.Date(2026, 5, 15, 9, 27, 1, 130000000, time.UTC)
	p := ParsedLine{
		Raw:     RawLine{Text: "raw", Source: "pod-1"},
		Time:    now,
		Level:   "INFO",
		Thread:  "main",
		TraceID: "abc123",
		Logger:  "com.example.App",
		Message: "hello world",
	}

	tests := []struct {
		field Field
		want  string
	}{
		{FieldTime, "09:27:01.130"},
		{FieldLevel, "INFO"},
		{FieldThread, "main"},
		{FieldTraceID, "abc123"},
		{FieldLogger, "com.example.App"},
		{FieldMessage, "hello world"},
		{FieldSource, "pod-1"},
	}

	for _, tt := range tests {
		got := p.Get(tt.field)
		if got != tt.want {
			t.Errorf("Get(%s) = %q, want %q", tt.field, got, tt.want)
		}
	}
}

func TestFieldMaskToggle(t *testing.T) {
	fm := DefaultFieldMask()
	if !fm.IsVisible(FieldTime) {
		t.Error("time should be visible by default")
	}
	fm.Toggle(FieldTime)
	if fm.IsVisible(FieldTime) {
		t.Error("time should be hidden after toggle")
	}
	fm.Toggle(FieldTime)
	if !fm.IsVisible(FieldTime) {
		t.Error("time should be visible after second toggle")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  Level
	}{
		{"INFO", LevelInfo},
		{"error", LevelError},
		{"WARN", LevelWarn},
		{"debug", LevelDebug},
		{"FATAL", LevelError},
		{"unknown", LevelInfo},
	}
	for _, tt := range tests {
		got := ParseLevel(tt.input)
		if got != tt.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/model/ -v
```

Expected: all PASS

- [ ] **Step 6: Create minimal main.go**

Create `main.go`:

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: logview <command> [args]")
		fmt.Println("Commands: k8s, tail, pipe")
		os.Exit(1)
	}
	fmt.Printf("logview: unknown command %q\n", os.Args[1])
	os.Exit(1)
}
```

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum main.go internal/model/
git commit -m "feat: project scaffold with core types"
```

---

### Task 2: Stream Interface + PipeSource

**Files:**
- Create: `internal/stream/stream.go`
- Create: `internal/stream/pipe.go`
- Test: `internal/stream/pipe_test.go`

- [ ] **Step 1: Write the test**

Create `internal/stream/pipe_test.go`:

```go
package stream

import (
	"bufio"
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

	// read the one line
	<-ch

	// cancel context
	cancel()

	// channel should close shortly
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("channel should be closed after cancel")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for channel close")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/stream/ -v
```

Expected: FAIL — package doesn't exist yet

- [ ] **Step 3: Create LogStream interface**

Create `internal/stream/stream.go`:

```go
package stream

import "context"

// LogStream produces raw log lines from a data source.
type LogStream interface {
	// Start begins producing lines. Returns a read-only channel.
	// The channel is closed when the context is cancelled or the source ends.
	Start(ctx context.Context) (<-chan RawLine, error)
	// Label returns a human-readable name for this source.
	Label() string
	// Cleanup releases resources.
	Cleanup() error
}

// RawLine is an unparsed log line with source metadata.
// (Aliased here to avoid circular import; the canonical definition is in model.)
// We re-export from model.
type RawLine struct {
	Text   string
	Source string
	Seq    uint64
}
```

Wait — we have RawLine in both `model` and `stream`. Let me fix this. `stream` should import from `model`. Let me revise:

Create `internal/stream/stream.go`:

```go
package stream

import (
	"context"
	"github.com/justfun/logview/internal/model"
)

// LogStream produces raw log lines from a data source.
type LogStream interface {
	Start(ctx context.Context) (<-chan model.RawLine, error)
	Label() string
	Cleanup() error
}
```

- [ ] **Step 4: Create PipeSource**

Create `internal/stream/pipe.go`:

```go
package stream

import (
	"bufio"
	"context"
	"io"
	"sync/atomic"

	"github.com/justfun/logview/internal/model"
)

// PipeSource reads log lines from an io.Reader (stdin).
type PipeSource struct {
	reader io.Reader
	seq    atomic.Uint64
}

// NewPipeSource creates a PipeSource from any io.Reader.
func NewPipeSource(r io.Reader) *PipeSource {
	return &PipeSource{reader: r}
}

func (p *PipeSource) Label() string { return "pipe" }

func (p *PipeSource) Start(ctx context.Context) (<-chan model.RawLine, error) {
	ch := make(chan model.RawLine, 256)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(p.reader)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}
			line := model.RawLine{
				Text:   scanner.Text(),
				Source: "pipe",
				Seq:    p.seq.Add(1),
			}
			select {
			case ch <- line:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

func (p *PipeSource) Cleanup() error { return nil }
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/stream/ -v
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/stream/
git commit -m "feat: LogStream interface + PipeSource"
```

---

### Task 3: TailSource

**Files:**
- Create: `internal/stream/tail.go`
- Test: `internal/stream/tail_test.go`

- [ ] **Step 1: Write the test**

Create `internal/stream/tail_test.go`:

```go
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
	if lines[2] != "line3" {
		t.Errorf("lines[2] = %q, want %q", lines[2], "line3")
	}
}

func TestTailSourceLabel(t *testing.T) {
	src := NewTailSource([]string{"/var/log/a.log", "/var/log/b.log"})
	label := src.Label()
	if label != "tail" {
		t.Errorf("Label() = %q, want %q", label, "tail")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/stream/ -run TestTail -v
```

Expected: FAIL

- [ ] **Step 3: Implement TailSource**

Create `internal/stream/tail.go`:

```go
package stream

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/justfun/logview/internal/model"
)

// TailSource tails one or more local files.
type TailSource struct {
	paths []string
	seq   atomic.Uint64
}

// NewTailSource creates a TailSource for the given file paths.
func NewTailSource(paths []string) *TailSource {
	return &TailSource{paths: paths}
}

func (t *TailSource) Label() string { return "tail" }

func (t *TailSource) Start(ctx context.Context) (<-chan model.RawLine, error) {
	ch := make(chan model.RawLine, 256)

	go func() {
		defer close(ch)
		var wg sync.WaitGroup
		for _, p := range t.paths {
			wg.Add(1)
			go func(path string) {
				defer wg.Done()
				t.tailFile(ctx, ch, path)
			}(p)
		}
		wg.Wait()
	}()

	return ch, nil
}

func (t *TailSource) tailFile(ctx context.Context, ch chan<- model.RawLine, path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	// seek to end for tail -f behavior
	f.Seek(0, 2)

	reader := bufio.NewReader(f)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			// no new data, brief sleep then retry
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// trim newline
		if len(line) > 0 && line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}

		raw := model.RawLine{
			Text:   line,
			Source: filepath.Base(path),
			Seq:    t.seq.Add(1),
		}
		select {
		case ch <- raw:
		case <-ctx.Done():
			return
		}
	}
}

func (t *TailSource) Cleanup() error { return nil }
```

Wait, I used `time` and `sync` and `filepath` without importing. Let me fix:

```go
package stream

import (
	"bufio"
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/justfun/logview/internal/model"
)

// TailSource tails one or more local files.
type TailSource struct {
	paths []string
	seq   atomic.Uint64
}

func NewTailSource(paths []string) *TailSource {
	return &TailSource{paths: paths}
}

func (t *TailSource) Label() string { return "tail" }

func (t *TailSource) Start(ctx context.Context) (<-chan model.RawLine, error) {
	ch := make(chan model.RawLine, 256)
	go func() {
		defer close(ch)
		var wg sync.WaitGroup
		for _, p := range t.paths {
			wg.Add(1)
			go func(path string) {
				defer wg.Done()
				t.tailFile(ctx, ch, path)
			}(p)
		}
		wg.Wait()
	}()
	return ch, nil
}

func (t *TailSource) tailFile(ctx context.Context, ch chan<- model.RawLine, path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	f.Seek(0, 2) // seek to end

	reader := bufio.NewReader(f)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if len(line) > 0 && line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}

		raw := model.RawLine{
			Text:   line,
			Source: filepath.Base(path),
			Seq:    t.seq.Add(1),
		}
		select {
		case ch <- raw:
		case <-ctx.Done():
			return
		}
	}
}

func (t *TailSource) Cleanup() error { return nil }
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/stream/ -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/stream/tail.go internal/stream/tail_test.go
git commit -m "feat: TailSource for local file tailing"
```

---

### Task 4: Parser — Regex + Rules Loading

**Files:**
- Create: `internal/parser/parser.go`
- Create: `internal/parser/regex.go`
- Create: `internal/parser/rules.go`
- Test: `internal/parser/regex_test.go`
- Test: `internal/parser/rules_test.go`

- [ ] **Step 1: Write parser interface + test**

Create `internal/parser/parser.go`:

```go
package parser

import "github.com/justfun/logview/internal/model"

// Parser converts raw text into structured log lines.
type Parser interface {
	// Parse attempts to parse a raw line. Returns nil if the format doesn't match.
	Parse(raw model.RawLine) *model.ParsedLine
	// Name returns the rule name.
	Name() string
}
```

Create `internal/parser/regex_test.go`:

```go
package parser

import (
	"testing"

	"github.com/justfun/logview/internal/model"
)

func TestRegexParserJavaLogback(t *testing.T) {
	pattern := `(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}) \[(?P<thread>[^\]]+)\] \[(?P<traceId>[^\]]+)\] (?P<level>\w+)\s+(?P<logger>\S+) - (?P<message>.*)`
	p, err := NewRegexParser("java-logback", pattern)
	if err != nil {
		t.Fatalf("NewRegexParser() error: %v", err)
	}

	input := "2026-05-15 09:27:01.130 [MQTT Call: sit-mqtt-client-984169] [NA] INFO  com.ydcloud.smart.parking.mqtt.DefaultMqttCallback - ==========deliveryComplete=true=========="
	raw := model.RawLine{Text: input, Source: "pod-1"}
	result := p.Parse(raw)

	if result == nil {
		t.Fatal("Parse() returned nil")
	}
	if result.Level != "INFO" {
		t.Errorf("Level = %q, want %q", result.Level, "INFO")
	}
	if result.Thread != "MQTT Call: sit-mqtt-client-984169" {
		t.Errorf("Thread = %q, want %q", result.Thread, "MQTT Call: sit-mqtt-client-984169")
	}
	if result.TraceID != "NA" {
		t.Errorf("TraceID = %q, want %q", result.TraceID, "NA")
	}
	if result.Message != "==========deliveryComplete=true==========" {
		t.Errorf("Message = %q, want %q", result.Message, "==========deliveryComplete=true==========")
	}
	if result.Logger != "com.ydcloud.smart.parking.mqtt.DefaultMqttCallback" {
		t.Errorf("Logger = %q, want %q", result.Logger, "com.ydcloud.smart.parking.mqtt.DefaultMqttCallback")
	}
}

func TestRegexParserNoMatch(t *testing.T) {
	p, err := NewRegexParser("test", `(?P<level>\w+) (?P<message>.*)`)
	if err != nil {
		t.Fatal(err)
	}
	raw := model.RawLine{Text: "12345 no level here"}
	result := p.Parse(raw)
	if result != nil {
		t.Error("expected nil for non-matching input")
	}
}

func TestRegexParserName(t *testing.T) {
	p, _ := NewRegexParser("my-rule", `(?P<message>.*)`)
	if p.Name() != "my-rule" {
		t.Errorf("Name() = %q, want %q", p.Name(), "my-rule")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/parser/ -v
```

Expected: FAIL

- [ ] **Step 3: Implement RegexParser**

Create `internal/parser/regex.go`:

```go
package parser

import (
	"regexp"
	"time"

	"github.com/justfun/logview/internal/model"
)

// RegexParser parses log lines using a regex with named capture groups.
type RegexParser struct {
	name    string
	re      *regexp.Regexp
	groups  []string
}

// NewRegexParser creates a parser from a named-capture-group regex.
func NewRegexParser(name, pattern string) (*RegexParser, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &RegexParser{
		name:   name,
		re:     re,
		groups: re.SubexpNames()[1:], // index 0 is empty
	}, nil
}

func (p *RegexParser) Name() string { return p.name }

func (p *RegexParser) Parse(raw model.RawLine) *model.ParsedLine {
	matches := p.re.FindStringSubmatch(raw.Text)
	if matches == nil {
		return nil
	}

	result := &model.ParsedLine{
		Raw:    raw,
		Fields: make(map[model.Field]string),
	}

	for i, name := range p.groups {
		if i+1 >= len(matches) {
			break
		}
		val := matches[i+1]
		result.Fields[model.Field(name)] = val

		switch model.Field(name) {
		case model.FieldTime:
			if t, err := time.Parse("2006-01-02 15:04:05.000", val); err == nil {
				result.Time = t
			}
		case model.FieldLevel:
			result.Level = val
		case model.FieldThread:
			result.Thread = val
		case model.FieldTraceID:
			result.TraceID = val
		case model.FieldLogger:
			result.Logger = val
		case model.FieldMessage:
			result.Message = val
		}
	}

	return result
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/parser/ -v
```

Expected: all PASS

- [ ] **Step 5: Write rules loading tests**

Create `internal/parser/rules_test.go`:

```go
package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRulesFromYAML(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `
rules:
  - name: java-logback
    pattern: '(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}) \[(?P<thread>[^\]]+)\] \[(?P<traceId>[^\]]+)\] (?P<level>\w+)\s+(?P<logger>\S+) - (?P<message>.*)'
  - name: plain-text
    pattern: '(?P<message>.*)'
`
	fpath := filepath.Join(dir, "rules.yaml")
	os.WriteFile(fpath, []byte(yamlContent), 0644)

	rules, err := LoadRules(fpath)
	if err != nil {
		t.Fatalf("LoadRules() error: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("got %d rules, want 2", len(rules))
	}
	if rules[0].Name != "java-logback" {
		t.Errorf("rules[0].Name = %q", rules[0].Name)
	}
	if rules[1].Name != "plain-text" {
		t.Errorf("rules[1].Name = %q", rules[1].Name)
	}
}

func TestAutoDetectParser(t *testing.T) {
	rules := []RuleConfig{
		{Name: "java", Pattern: `(?P<level>\w+) (?P<message>.*)`},
		{Name: "fallback", Pattern: `(?P<message>.*)`},
	}

	parsers := MustCompileRules(rules)
	ad := NewAutoDetect(parsers)

	// "INFO hello" should match first rule
	raw := RawLine{Text: "INFO hello world"}
	p := ad.Detect(raw)
	if p == nil {
		t.Fatal("Detect() returned nil")
	}
	if p.Name() != "java" {
		t.Errorf("matched %q, want %q", p.Name(), "java")
	}
}
```

Wait, I'm using `RawLine` without the model prefix in the test. Let me fix. Also `AutoDetect` needs to be defined. Let me restructure.

- [ ] **Step 5 (revised): Write rules loading tests**

Create `internal/parser/rules_test.go`:

```go
package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/justfun/logview/internal/model"
)

func TestLoadRulesFromYAML(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `
rules:
  - name: java-logback
    pattern: '(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}) \[(?P<thread>[^\]]+)\] \[(?P<traceId>[^\]]+)\] (?P<level>\w+)\s+(?P<logger>\S+) - (?P<message>.*)'
  - name: plain-text
    pattern: '(?P<message>.*)'
`
	fpath := filepath.Join(dir, "rules.yaml")
	os.WriteFile(fpath, []byte(yamlContent), 0644)

	rules, err := LoadRules(fpath)
	if err != nil {
		t.Fatalf("LoadRules() error: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("got %d rules, want 2", len(rules))
	}
	if rules[0].Name != "java-logback" {
		t.Errorf("rules[0].Name = %q", rules[0].Name)
	}
}

func TestAutoDetect(t *testing.T) {
	rules := []RuleConfig{
		{Name: "java", Pattern: `(?P<level>\w+) (?P<message>.*)`},
		{Name: "fallback", Pattern: `(?P<message>.*)`},
	}
	parsers := MustCompileRules(rules)
	ad := NewAutoDetect(parsers)

	raw := model.RawLine{Text: "INFO hello world"}
	p := ad.Detect(raw)
	if p == nil {
		t.Fatal("Detect() returned nil")
	}
	if p.Name() != "java" {
		t.Errorf("matched %q, want %q", p.Name(), "java")
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

```bash
go test ./internal/parser/ -run TestLoad -v
```

Expected: FAIL

- [ ] **Step 7: Implement rules loading + auto-detect**

Create `internal/parser/rules.go`:

```go
package parser

import (
	"fmt"
	"os"

	"github.com/justfun/logview/internal/model"
	"gopkg.in/yaml.v3"
)

// RuleConfig is a parsed rule from the YAML config.
type RuleConfig struct {
	Name    string `yaml:"name"`
	Pattern string `yaml:"pattern"`
	Parse   string `yaml:"parse,omitempty"` // "json" for JSON parsing
}

// rulesFile represents the YAML config file structure.
type rulesFile struct {
	Rules []RuleConfig `yaml:"rules"`
}

// LoadRules loads rule definitions from a YAML file.
func LoadRules(path string) ([]RuleConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read rules: %w", err)
	}
	var rf rulesFile
	if err := yaml.Unmarshal(data, &rf); err != nil {
		return nil, fmt.Errorf("parse rules yaml: %w", err)
	}
	return rf.Rules, nil
}

// MustCompileRules compiles all rule configs into Parser instances.
// Panics on invalid regex — config should be validated at startup.
func MustCompileRules(rules []RuleConfig) []Parser {
	parsers := make([]Parser, 0, len(rules))
	for _, r := range rules {
		if r.Parse == "json" {
			parsers = append(parsers, NewJSONParser(r.Name))
			continue
		}
		p, err := NewRegexParser(r.Name, r.Pattern)
		if err != nil {
			panic(fmt.Sprintf("invalid regex in rule %q: %v", r.Name, err))
		}
		parsers = append(parsers, p)
	}
	return parsers
}

// AutoDetect tries each parser in order and returns the first that matches.
type AutoDetect struct {
	parsers []Parser
	chosen  map[string]Parser // cache: source → parser
}

// NewAutoDetect creates an auto-detection wrapper.
func NewAutoDetect(parsers []Parser) *AutoDetect {
	return &AutoDetect{
		parsers: parsers,
		chosen:  make(map[string]Parser),
	}
}

// Detect returns the matching parser for a given raw line.
// Caches the result per source so subsequent lines skip detection.
func (ad *AutoDetect) Detect(raw model.RawLine) Parser {
	if p, ok := ad.chosen[raw.Source]; ok {
		return p
	}
	for _, p := range ad.parsers {
		if p.Parse(raw) != nil {
			ad.chosen[raw.Source] = p
			return p
		}
	}
	return nil
}
```

- [ ] **Step 8: Run all parser tests**

```bash
go test ./internal/parser/ -v
```

Expected: all PASS

- [ ] **Step 9: Commit**

```bash
git add internal/parser/
git commit -m "feat: LogParser with regex, rules YAML, and auto-detect"
```

---

### Task 5: JSON Parser

**Files:**
- Create: `internal/parser/json_parser.go`
- Test: `internal/parser/json_parser_test.go`

- [ ] **Step 1: Write the test**

Create `internal/parser/json_parser_test.go`:

```go
package parser

import (
	"testing"

	"github.com/justfun/logview/internal/model"
)

func TestJSONParser(t *testing.T) {
	p := NewJSONParser("json-log")

	input := `{"time":"2026-05-15T09:27:01.130Z","level":"INFO","thread":"main","traceId":"abc123","logger":"com.example.App","message":"hello world"}`
	raw := model.RawLine{Text: input, Source: "pod-1"}
	result := p.Parse(raw)

	if result == nil {
		t.Fatal("Parse() returned nil")
	}
	if result.Level != "INFO" {
		t.Errorf("Level = %q, want INFO", result.Level)
	}
	if result.Message != "hello world" {
		t.Errorf("Message = %q, want 'hello world'", result.Message)
	}
	if result.TraceID != "abc123" {
		t.Errorf("TraceID = %q, want 'abc123'", result.TraceID)
	}
}

func TestJSONParserInvalidJSON(t *testing.T) {
	p := NewJSONParser("json-log")
	raw := model.RawLine{Text: "not json at all"}
	result := p.Parse(raw)
	if result != nil {
		t.Error("expected nil for invalid JSON")
	}
}

func TestJSONParserName(t *testing.T) {
	p := NewJSONParser("my-json")
	if p.Name() != "my-json" {
		t.Errorf("Name() = %q, want 'my-json'", p.Name())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/parser/ -run TestJSON -v
```

Expected: FAIL

- [ ] **Step 3: Implement JSONParser**

Create `internal/parser/json_parser.go`:

```go
package parser

import (
	"encoding/json"
	"time"

	"github.com/justfun/logview/internal/model"
)

// JSONParser parses log lines that are JSON objects.
type JSONParser struct {
	name string
}

// NewJSONParser creates a JSON-based parser.
func NewJSONParser(name string) *JSONParser {
	return &JSONParser{name: name}
}

func (p *JSONParser) Name() string { return p.name }

// jsonFields maps JSON keys to ParsedLine fields.
type jsonFields struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Thread  string `json:"thread"`
	TraceID string `json:"traceId"`
	Logger  string `json:"logger"`
	Message string `json:"message"`
}

func (p *JSONParser) Parse(raw model.RawLine) *model.ParsedLine {
	var f jsonFields
	if err := json.Unmarshal([]byte(raw.Text), &f); err != nil {
		return nil
	}

	result := &model.ParsedLine{
		Raw:     raw,
		Level:   f.Level,
		Thread:  f.Thread,
		TraceID: f.TraceID,
		Logger:  f.Logger,
		Message: f.Message,
		Fields: map[model.Field]string{
			model.FieldLevel:   f.Level,
			model.FieldThread:  f.Thread,
			model.FieldTraceID: f.TraceID,
			model.FieldLogger:  f.Logger,
			model.FieldMessage: f.Message,
		},
	}

	if f.Time != "" {
		for _, layout := range []string{
			"2006-01-02T15:04:05.000Z",
			"2006-01-02T15:04:05Z",
			"2006-01-02 15:04:05.000",
			"2006-01-02 15:04:05",
			time.RFC3339,
			time.RFC3339Nano,
		} {
			if t, err := time.Parse(layout, f.Time); err == nil {
				result.Time = t
				result.Fields[model.FieldTime] = f.Time
				break
			}
		}
	}

	return result
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/parser/ -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/parser/json_parser.go internal/parser/json_parser_test.go
git commit -m "feat: JSON log parser"
```

---

### Task 6: Ring Buffer

**Files:**
- Create: `internal/buffer/ring.go`
- Test: `internal/buffer/ring_test.go`

- [ ] **Step 1: Write the test**

Create `internal/buffer/ring_test.go`:

```go
package buffer

import (
	"testing"

	"github.com/justfun/logview/internal/model"
)

func TestRingBufferAppendAndGet(t *testing.T) {
	rb := NewRingBuffer(5)
	for i := 0; i < 7; i++ {
		rb.Push(&model.ParsedLine{Message: string(rune('a' + i))})
	}

	// capacity 5, pushed 7, so oldest 2 are evicted
	if rb.Len() != 5 {
		t.Fatalf("Len() = %d, want 5", rb.Len())
	}

	// should have entries c(2), d(3), e(4), f(5), g(6)
	first := rb.Get(0)
	if first.Message != "c" {
		t.Errorf("Get(0).Message = %q, want 'c'", first.Message)
	}

	last := rb.Get(4)
	if last.Message != "g" {
		t.Errorf("Get(4).Message = %q, want 'g'", last.Message)
	}
}

func TestRingBufferGetOutOfRange(t *testing.T) {
	rb := NewRingBuffer(3)
	rb.Push(&model.ParsedLine{Message: "a"})
	result := rb.Get(5)
	if result != nil {
		t.Error("expected nil for out-of-range Get")
	}
}

func TestRingBufferSlice(t *testing.T) {
	rb := NewRingBuffer(10)
	for i := 0; i < 5; i++ {
		rb.Push(&model.ParsedLine{Message: string(rune('a' + i))})
	}

	slice := rb.Slice(1, 4)
	if len(slice) != 3 {
		t.Fatalf("Slice(1,4) len = %d, want 3", len(slice))
	}
	if slice[0].Message != "b" {
		t.Errorf("slice[0].Message = %q, want 'b'", slice[0].Message)
	}
}

func TestRingBufferTotalReceived(t *testing.T) {
	rb := NewRingBuffer(3)
	rb.Push(&model.ParsedLine{Message: "a"})
	rb.Push(&model.ParsedLine{Message: "b"})
	rb.Push(&model.ParsedLine{Message: "c"})
	rb.Push(&model.ParsedLine{Message: "d"})

	if rb.TotalReceived() != 4 {
		t.Errorf("TotalReceived() = %d, want 4", rb.TotalReceived())
	}
	if rb.Len() != 3 {
		t.Errorf("Len() = %d, want 3", rb.Len())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/buffer/ -v
```

Expected: FAIL

- [ ] **Step 3: Implement RingBuffer**

Create `internal/buffer/ring.go`:

```go
package buffer

import "github.com/justfun/logview/internal/model"

// RingBuffer is a fixed-capacity circular buffer for ParsedLines.
// Thread-safe for single-writer (Push) + single-reader (Get/Slice).
type RingBuffer struct {
	buf    []*model.ParsedLine
	cap    int
	head   int // index of oldest entry
	len    int
	total  uint64
}

// NewRingBuffer creates a ring buffer with the given capacity.
func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		buf: make([]*model.ParsedLine, capacity),
		cap: capacity,
	}
}

// Push adds a parsed line to the buffer.
// If the buffer is full, the oldest entry is overwritten.
func (rb *RingBuffer) Push(line *model.ParsedLine) {
	idx := (rb.head + rb.len) % rb.cap
	rb.buf[idx] = line
	if rb.len < rb.cap {
		rb.len++
	} else {
		rb.head = (rb.head + 1) % rb.cap
	}
	rb.total++
}

// Len returns the number of entries currently in the buffer.
func (rb *RingBuffer) Len() int { return rb.len }

// TotalReceived returns the total number of lines pushed (including evicted).
func (rb *RingBuffer) TotalReceived() uint64 { return rb.total }

// Get returns the entry at logical index i (0 = oldest).
// Returns nil if i is out of range.
func (rb *RingBuffer) Get(i int) *model.ParsedLine {
	if i < 0 || i >= rb.len {
		return nil
	}
	idx := (rb.head + i) % rb.cap
	return rb.buf[idx]
}

// Slice returns entries in the range [start, end).
func (rb *RingBuffer) Slice(start, end int) []*model.ParsedLine {
	if start < 0 {
		start = 0
	}
	if end > rb.len {
		end = rb.len
	}
	if start >= end {
		return nil
	}
	result := make([]*model.ParsedLine, end-start)
	for i := range result {
		result[i] = rb.Get(start + i)
	}
	return result
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/buffer/ -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/buffer/
git commit -m "feat: ring buffer for log storage"
```

---

### Task 7: Search Index

**Files:**
- Create: `internal/buffer/index.go`
- Test: `internal/buffer/index_test.go`

- [ ] **Step 1: Write the test**

Create `internal/buffer/index_test.go`:

```go
package buffer

import (
	"testing"
)

func TestSearchIndexBasicMatch(t *testing.T) {
	idx := NewSearchIndex()

	// "hello" appears in buffer positions 0, 2
	idx.Add(0, "hello world")
	idx.Add(1, "foo bar")
	idx.Add(2, "say hello again")

	hits := idx.Search("hello")
	if len(hits) != 2 {
		t.Fatalf("Search('hello') returned %d hits, want 2", len(hits))
	}
	if hits[0] != 0 || hits[1] != 2 {
		t.Errorf("hits = %v, want [0, 2]", hits)
	}
}

func TestSearchIndexNoMatch(t *testing.T) {
	idx := NewSearchIndex()
	idx.Add(0, "hello world")

	hits := idx.Search("xyz")
	if len(hits) != 0 {
		t.Errorf("Search('xyz') returned %d hits, want 0", len(hits))
	}
}

func TestSearchIndexCaseInsensitive(t *testing.T) {
	idx := NewSearchIndex()
	idx.Add(0, "Hello World")

	hits := idx.Search("hello")
	if len(hits) != 1 {
		t.Fatalf("case-insensitive search returned %d hits, want 1", len(hits))
	}
}

func TestSearchIndexClear(t *testing.T) {
	idx := NewSearchIndex()
	idx.Add(0, "hello")
	idx.Clear()

	hits := idx.Search("hello")
	if len(hits) != 0 {
		t.Errorf("after Clear, Search returned %d hits, want 0", len(hits))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/buffer/ -run TestSearch -v
```

Expected: FAIL

- [ ] **Step 3: Implement SearchIndex**

Create `internal/buffer/index.go`:

```go
package buffer

import (
	"sort"
	"strings"
)

// SearchIndex provides case-insensitive keyword search over buffer positions.
// Not thread-safe — caller must synchronize.
type SearchIndex struct {
	// Inverted index: lowercased token → set of buffer positions
	index map[string]map[int]struct{}
}

// NewSearchIndex creates an empty search index.
func NewSearchIndex() *SearchIndex {
	return &SearchIndex{
		index: make(map[string]map[int]struct{}),
	}
}

// Add indexes the text at the given buffer position.
func (si *SearchIndex) Add(pos int, text string) {
	tokens := tokenize(text)
	for _, tok := range tokens {
		if _, ok := si.index[tok]; !ok {
			si.index[tok] = make(map[int]struct{})
		}
		si.index[tok][pos] = struct{}{}
	}
}

// Search returns buffer positions matching the query (case-insensitive substring).
// For multi-word queries, returns positions matching ALL words.
func (si *SearchIndex) Search(query string) []int {
	words := tokenize(query)
	if len(words) == 0 {
		return nil
	}

	var result map[int]struct{}
	for _, w := range words {
		positions := make(map[int]struct{})
		for tok, posSet := range si.index {
			if strings.Contains(tok, w) {
				for p := range posSet {
					positions[p] = struct{}{}
				}
			}
		}
		if result == nil {
			result = positions
		} else {
			// intersect
			for p := range result {
				if _, ok := positions[p]; !ok {
					delete(result, p)
				}
			}
		}
		if len(result) == 0 {
			return nil
		}
	}

	hits := make([]int, 0, len(result))
	for p := range result {
		hits = append(hits, p)
	}
	sort.Ints(hits)
	return hits
}

// Clear removes all indexed data.
func (si *SearchIndex) Clear() {
	si.index = make(map[string]map[int]struct{})
}

// tokenize splits text into lowercase tokens.
func tokenize(text string) []string {
	text = strings.ToLower(text)
	return strings.Fields(text)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/buffer/ -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/buffer/index.go internal/buffer/index_test.go
git commit -m "feat: search index with inverted index"
```

---

### Task 8: Stack Trace Detector

**Files:**
- Create: `internal/stacktrace/detector.go`
- Test: `internal/stacktrace/detector_test.go`

- [ ] **Step 1: Write the test**

Create `internal/stacktrace/detector_test.go`:

```go
package stacktrace

import (
	"testing"

	"github.com/justfun/logview/internal/model"
)

func TestDetectStackTrace(t *testing.T) {
	lines := []*model.ParsedLine{
		{Message: "something happened"},
		{Message: "java.lang.NullPointerException"},
		{Message: "  at com.example.App.doThing(App.java:42)"},
		{Message: "  at com.example.App.run(App.java:10)"},
		{Message: "Caused by: java.lang.IllegalArgumentException"},
		{Message: "  at com.example.Util.check(Util.java:5)"},
		{Message: "normal log again"},
	}

	groups := Detect(lines)
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}
	g := groups[0]
	if g.Start != 1 {
		t.Errorf("Start = %d, want 1", g.Start)
	}
	if g.End != 5 {
		t.Errorf("End = %d, want 5", g.End)
	}
	if g.Leader != "java.lang.NullPointerException" {
		t.Errorf("Leader = %q", g.Leader)
	}
}

func TestNoStackTrace(t *testing.T) {
	lines := []*model.ParsedLine{
		{Message: "hello"},
		{Message: "world"},
	}
	groups := Detect(lines)
	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/stacktrace/ -v
```

Expected: FAIL

- [ ] **Step 3: Implement detector**

Create `internal/stacktrace/detector.go`:

```go
package stacktrace

import (
	"strings"

	"github.com/justfun/logview/internal/model"
)

// Group represents a contiguous range of stack trace lines in the buffer.
type Group struct {
	Start  int    // buffer index of the exception line
	End    int    // buffer index of the last stack frame (inclusive)
	Leader string // first line text (the exception name)
}

// Detect scans parsed lines and returns stack trace groups.
func Detect(lines []*model.ParsedLine) []Group {
	var groups []Group
	i := 0
	for i < len(lines) {
		if isExceptionLine(lines[i].Message) {
			start := i
			leader := lines[i].Message
			i++
			// consume stack frames
			for i < len(lines) && isStackFrame(lines[i].Message) {
				i++
			}
			// consume "Caused by" chains
			for i < len(lines) && isCausedBy(lines[i].Message) {
				i++
				for i < len(lines) && isStackFrame(lines[i].Message) {
					i++
				}
			}
			groups = append(groups, Group{
				Start:  start,
				End:    i - 1,
				Leader: leader,
			})
		} else {
			i++
		}
	}
	return groups
}

func isExceptionLine(msg string) bool {
	trimmed := strings.TrimSpace(msg)
	// Java exceptions typically contain a dot and no spaces (class pattern)
	// e.g. "java.lang.NullPointerException" or "java.io.IOException: msg"
	if strings.Contains(trimmed, "Exception") ||
		strings.Contains(trimmed, "Error") ||
		strings.Contains(trimmed, "Throwable") {
		return !strings.HasPrefix(trimmed, "at ") &&
			!strings.HasPrefix(trimmed, "Caused by")
	}
	return false
}

func isStackFrame(msg string) bool {
	trimmed := strings.TrimSpace(msg)
	return strings.HasPrefix(trimmed, "at ")
}

func isCausedBy(msg string) bool {
	trimmed := strings.TrimSpace(msg)
	return strings.HasPrefix(trimmed, "Caused by:")
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/stacktrace/ -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/stacktrace/
git commit -m "feat: stack trace detector for Java exceptions"
```

---

### Task 9: Export

**Files:**
- Create: `internal/export/export.go`
- Test: `internal/export/export_test.go`

- [ ] **Step 1: Write the test**

Create `internal/export/export_test.go`:

```go
package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justfun/logview/internal/model"
)

func TestExportRaw(t *testing.T) {
	lines := []*model.ParsedLine{
		{Raw: model.RawLine{Text: "line1"}, Message: "line1"},
		{Raw: model.RawLine{Text: "line2"}, Message: "line2"},
	}

	dir := t.TempDir()
	fpath := filepath.Join(dir, "export.log")
	n, err := ToFile(lines, fpath, FormatRaw)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("exported %d lines, want 2", n)
	}

	data, _ := os.ReadFile(fpath)
	content := string(data)
	if !strings.Contains(content, "line1\n") || !strings.Contains(content, "line2\n") {
		t.Errorf("unexpected content: %q", content)
	}
}

func TestExportJSON(t *testing.T) {
	lines := []*model.ParsedLine{
		{Level: "INFO", Message: "hello"},
	}

	dir := t.TempDir()
	fpath := filepath.Join(dir, "export.json")
	n, err := ToFile(lines, fpath, FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("exported %d lines, want 1", n)
	}

	data, _ := os.ReadFile(fpath)
	content := string(data)
	if !strings.Contains(content, `"level":"INFO"`) || !strings.Contains(content, `"message":"hello"`) {
		t.Errorf("unexpected JSON: %q", content)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/export/ -v
```

Expected: FAIL

- [ ] **Step 3: Implement export**

Create `internal/export/export.go`:

```go
package export

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/justfun/logview/internal/model"
)

// Format controls the export output format.
type Format int

const (
	FormatRaw  Format = iota // original log text
	FormatJSON               // structured JSON
)

// exportEntry is the JSON structure for a single exported line.
type exportEntry struct {
	Time    string `json:"time,omitempty"`
	Level   string `json:"level,omitempty"`
	Thread  string `json:"thread,omitempty"`
	TraceID string `json:"traceId,omitempty"`
	Logger  string `json:"logger,omitempty"`
	Message string `json:"message,omitempty"`
	Source  string `json:"source,omitempty"`
}

// ToFile writes parsed lines to a file in the given format.
// Returns the number of lines written.
func ToFile(lines []*model.ParsedLine, path string, format Format) (int, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf("create export file: %w", err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	count := 0

	for _, line := range lines {
		switch format {
		case FormatRaw:
			fmt.Fprintln(w, line.Raw.Text)
		case FormatJSON:
			entry := exportEntry{
				Time:    line.Get(model.FieldTime),
				Level:   line.Level,
				Thread:  line.Thread,
				TraceID: line.TraceID,
				Logger:  line.Logger,
				Message: line.Message,
				Source:  line.Raw.Source,
			}
			data, _ := json.Marshal(entry)
			fmt.Fprintln(w, string(data))
		}
		count++
	}

	if err := w.Flush(); err != nil {
		return count, fmt.Errorf("flush export: %w", err)
	}
	return count, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/export/ -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/export/
git commit -m "feat: log export to raw text and JSON"
```

---

### Task 10: Cobra CLI Setup

**Files:**
- Create: `cmd/root.go`
- Modify: `main.go`

- [ ] **Step 1: Create cobra commands**

Create `cmd/root.go`:

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	ruleName   string
	bufferSize int
)

var rootCmd = &cobra.Command{
	Use:   "logview",
	Short: "Terminal log viewer with real-time search and filtering",
}

var k8sCmd = &cobra.Command{
	Use:   "k8s <resource> [flags]",
	Short: "View logs from Kubernetes pods",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resource := args[0]
		namespace, _ := cmd.Flags().GetString("namespace")
		fmt.Printf("k8s: resource=%s namespace=%s\n", resource, namespace)
		// TODO: wire up stream + TUI (Task 15)
		return nil
	},
}

var tailCmd = &cobra.Command{
	Use:   "tail <file> [file...] [flags]",
	Short: "View logs from local files",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("tail: files=%v\n", args)
		// TODO: wire up stream + TUI (Task 15)
		return nil
	},
}

var pipeCmd = &cobra.Command{
	Use:   "pipe",
	Short: "View logs from stdin (pipe)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("pipe: reading from stdin")
		// TODO: wire up stream + TUI (Task 15)
		return nil
	},
}

func init() {
	k8sCmd.Flags().StringP("namespace", "n", "default", "Kubernetes namespace")
	rootCmd.PersistentFlags().StringVar(&ruleName, "rule", "", "parser rule name (auto-detect if empty)")
	rootCmd.PersistentFlags().IntVar(&bufferSize, "buffer-size", 100000, "ring buffer capacity")
	rootCmd.AddCommand(k8sCmd)
	rootCmd.AddCommand(tailCmd)
	rootCmd.AddCommand(pipeCmd)
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Update main.go**

```go
package main

import "github.com/justfun/logview/cmd"

func main() {
	cmd.Execute()
}
```

- [ ] **Step 3: Build and verify**

```bash
go build -o logview . && ./logview --help
```

Expected: help text showing k8s, tail, pipe commands

- [ ] **Step 4: Commit**

```bash
git add cmd/ main.go
git commit -m "feat: cobra CLI with k8s/tail/pipe commands"
```

---

### Task 11: TUI — Key Bindings + Styles

**Files:**
- Create: `internal/tui/keymap.go`
- Create: `internal/tui/style.go`

- [ ] **Step 1: Create key bindings**

Create `internal/tui/keymap.go`:

```go
package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all keyboard shortcuts.
type KeyMap struct {
	Search   key.Binding
	Enter    key.Binding
	Tab      key.Binding
	Fields   key.Binding
	Expand   key.Binding
	Export   key.Binding
	Jump     key.Binding
	Quit     key.Binding
	Escape   key.Binding
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "搜索"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("Enter", "提取"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("Tab", "面板"),
		),
		Fields: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "字段"),
		),
		Expand: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "堆栈"),
		),
		Export: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "导出"),
		),
		Jump: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "跳转"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "退出"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("Esc", "取消"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "上移"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "下移"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown"),
		),
	}
}

// HelpEntries returns key bindings for the help bar.
func (km KeyMap) HelpEntries() []key.Binding {
	return []key.Binding{
		km.Search, km.Tab, km.Enter, km.Fields,
		km.Expand, km.Export, km.Jump, km.Quit,
	}
}
```

- [ ] **Step 2: Create styles**

Create `internal/tui/style.go`:

```go
package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Title bar
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62"))

	// Log levels
	LevelDebug = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	LevelInfo  = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	LevelWarn  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	LevelError = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)

	// Search bar
	SearchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62"))

	// Search highlight
	HighlightStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("227")).
			Foreground(lipgloss.Color("0"))

	// Bottom panel
	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false, false, false)

	// Selected line
	SelectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("15"))

	// Help bar
	HelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243"))

	HelpKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86"))

	// Checkbox
	CheckboxChecked   = "☑"
	CheckboxUnchecked = "☐"

	// Folded stack indicator
	FoldedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).Italic(true)

	// New logs indicator
	NewLogStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).Bold(true)
)

// LevelStyle returns the style for a given log level.
func LevelStyle(level string) lipgloss.Style {
	switch level {
	case "DEBUG", "DBG":
		return LevelDebug
	case "INFO":
		return LevelInfo
	case "WARN", "WARNING":
		return LevelWarn
	case "ERROR", "ERR", "FATAL":
		return LevelError
	}
	return lipgloss.NewStyle()
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/tui/keymap.go internal/tui/style.go
git commit -m "feat: TUI key bindings and styles"
```

---

### Task 12: TUI — Root Model + Virtual Scroll Log View

**Files:**
- Create: `internal/tui/app.go`
- Create: `internal/tui/logview.go`
- Create: `internal/tui/helpbar.go`

This is the biggest task. The root model orchestrates all components and manages the bubbletea event loop.

- [ ] **Step 1: Create the root model**

Create `internal/tui/app.go`:

```go
package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/justfun/logview/internal/buffer"
	"github.com/justfun/logview/internal/model"
	"github.com/justfun/logview/internal/parser"
	"github.com/justfun/logview/internal/stacktrace"
	"github.com/justfun/logview/internal/stream"
)

// App is the root bubbletea model.
type App struct {
	// config
	stream    stream.LogStream
	parsers   *parser.AutoDetect
	buffer    *buffer.RingBuffer
	searchIdx *buffer.SearchIndex
	keymap    KeyMap

	// state
	lines        []*model.ParsedLine // filtered/sorted view
	filteredView []*model.ParsedLine // after applying filters
	stGroups     []stacktrace.Group
	expanded     map[int]bool // stack trace groups that are expanded

	// UI state
	width       int
	height      int
	cursor      int // position in filteredView
	offset      int // scroll offset (first visible line)
	autoscroll  bool
	newLogs     int

	// search
	searchMode  bool
	searchInput string

	// filter
	fieldMask model.FieldMask
	levelMask map[string]bool // level → visible
	filterTraceID string
	filterThread  string

	// panel
	activePanel int // 0=fields, 1=levels, 2=filters

	// export
	exportMode bool

	// parser name
	parserName string

	// tick
	lastTick time.Time
}

// NewApp creates a new TUI application.
func NewApp(src stream.LogStream, parsers *parser.AutoDetect, bufSize int) *App {
	return &App{
		stream:     src,
		parsers:    parsers,
		buffer:     buffer.NewRingBuffer(bufSize),
		searchIdx:  buffer.NewSearchIndex(),
		keymap:     DefaultKeyMap(),
		fieldMask:  model.DefaultFieldMask(),
		levelMask:  map[string]bool{"DEBUG": false, "INFO": true, "WARN": true, "ERROR": true},
		expanded:   make(map[int]bool),
		autoscroll: true,
	}
}

// streamMsg wraps a raw line received from the stream.
type streamMsg struct{ line model.RawLine }

// tickMsg is sent at ~30fps for rendering.
type tickMsg struct{}

func waitForStream(ctx context.Context, ch <-chan model.RawLine) tea.Cmd {
	return func() tea.Msg {
		select {
		case line, ok := <-ch:
			if !ok {
				return nil
			}
			return streamMsg{line: line}
		case <-ctx.Done():
			return nil
		}
	}
}

func tickEvery() tea.Cmd {
	return tea.Tick(33*time.Millisecond, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (a *App) Init() tea.Cmd {
	ctx := context.Background()
	ch, err := a.stream.Start(ctx)
	if err != nil {
		return nil
	}
	return tea.Batch(waitForStream(ctx, ch), tickEvery())
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case streamMsg:
		a.processLine(msg.line)
		return a, waitForStream(context.Background(), nil) // simplified

	case tickMsg:
		return a, tickEvery()

	case tea.KeyMsg:
		if a.exportMode {
			return a.handleExportKeys(msg)
		}
		if a.searchMode {
			return a.handleSearchKeys(msg)
		}
		return a.handleNormalKeys(msg)
	}

	return a, nil
}

func (a *App) processLine(raw model.RawLine) {
	var pl *model.ParsedLine
	if a.parsers != nil {
		if p := a.parsers.Detect(raw); p != nil {
			pl = p.Parse(raw)
			a.parserName = p.Name()
		}
	}
	if pl == nil {
		pl = &model.ParsedLine{
			Raw:     raw,
			Message: raw.Text,
			Fields:  map[model.Field]string{model.FieldMessage: raw.Text},
		}
	}

	a.buffer.Push(pl)
	a.searchIdx.Add(int(a.buffer.TotalReceived()-1), raw.Text)

	if !a.autoscroll {
		a.newLogs++
	}

	// recompute filtered view periodically (on tick)
	a.recomputeView()
}

func (a *App) recomputeView() {
	// rebuild filtered view from buffer
	// apply level filter, traceId filter, thread filter, search
	var view []*model.ParsedLine
	for i := 0; i < a.buffer.Len(); i++ {
		line := a.buffer.Get(i)
		if line == nil {
			continue
		}
		if !a.levelMask[line.Level] && line.Level != "" {
			continue
		}
		if a.filterTraceID != "" && line.TraceID != a.filterTraceID {
			continue
		}
		if a.filterThread != "" && line.Thread != a.filterThread {
			continue
		}
		if a.searchInput != "" {
			// check if message contains search term
			// simplified: just substring match
			// TODO: use searchIdx for better performance
			if !containsIgnoreCase(line.Message, a.searchInput) &&
				!containsIgnoreCase(line.Raw.Text, a.searchInput) {
				continue
			}
		}
		view = append(view, line)
	}
	a.filteredView = view

	// update stack trace groups
	a.stGroups = stacktrace.Detect(view)
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > 0 && len(substr) > 0 &&
				containsSubstr(s, substr)))
}

func containsSubstr(s, sub string) bool {
	// simple case-insensitive substring
	sl, sbl := len(s), len(sub)
	for i := 0; i <= sl-sbl; i++ {
		match := true
		for j := 0; j < sbl; j++ {
			sc, tc := s[i+j], sub[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if tc >= 'A' && tc <= 'Z' {
				tc += 32
			}
			if sc != tc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func (a *App) handleNormalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return a, tea.Quit
	case "/":
		a.searchMode = true
		a.searchInput = ""
		return a, nil
	case "tab":
		a.activePanel = (a.activePanel + 1) % 3
		return a, nil
	case "f":
		a.activePanel = 0
		return a, nil
	case "e":
		// toggle expand current stack trace group
		for _, g := range a.stGroups {
			if a.cursor >= g.Start && a.cursor <= g.End {
				a.expanded[g.Start] = !a.expanded[g.Start]
				break
			}
		}
		return a, nil
	case "s":
		a.exportMode = true
		return a, nil
	case "g":
		if a.cursor == 0 {
			a.cursor = len(a.filteredView) - 1
		} else {
			a.cursor = 0
		}
		a.autoscroll = (a.cursor == len(a.filteredView)-1)
		return a, nil
	case "enter":
		// extract traceId/thread from current line
		if a.cursor < len(a.filteredView) {
			line := a.filteredView[a.cursor]
			if line.TraceID != "" && line.TraceID != "NA" {
				a.filterTraceID = line.TraceID
			}
			if line.Thread != "" {
				a.filterThread = line.Thread
			}
			a.activePanel = 2
			a.recomputeView()
		}
		return a, nil
	case "up", "k":
		if a.cursor > 0 {
			a.cursor--
			a.autoscroll = false
		}
		return a, nil
	case "down", "j":
		if a.cursor < len(a.filteredView)-1 {
			a.cursor++
		}
		a.autoscroll = (a.cursor == len(a.filteredView)-1)
		return a, nil
	case "pgup":
		pageSize := a.visibleLines()
		a.cursor -= pageSize
		if a.cursor < 0 {
			a.cursor = 0
		}
		a.autoscroll = false
		return a, nil
	case "pgdown":
		pageSize := a.visibleLines()
		a.cursor += pageSize
		if a.cursor >= len(a.filteredView) {
			a.cursor = len(a.filteredView) - 1
		}
		a.autoscroll = (a.cursor == len(a.filteredView)-1)
		return a, nil
	}
	return a, nil
}

func (a *App) handleSearchKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.searchMode = false
		return a, nil
	case "enter":
		a.searchMode = false
		a.recomputeView()
		return a, nil
	case "backspace":
		if len(a.searchInput) > 0 {
			a.searchInput = a.searchInput[:len(a.searchInput)-1]
			a.recomputeView()
		}
		return a, nil
	default:
		if len(msg.String()) == 1 {
			a.searchInput += msg.String()
			a.recomputeView()
		}
		return a, nil
	}
}

func (a *App) handleExportKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		a.exportMode = false
		return a, nil
	}
	return a, nil
}

func (a *App) visibleLines() int {
	// total height minus title bar, search bar, bottom panel, help bar
	return a.height - 6
}

func (a *App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	// title
	title := TitleStyle.Render(
		fmt.Sprintf(" LogView ─ %s [%s] ─ %d条 ", a.stream.Label(), a.parserName, a.buffer.Len()),
	)

	// log view
	logView := a.renderLogView()

	// search bar
	searchBar := a.renderSearchBar()

	// bottom panel
	panel := a.renderPanel()

	// help bar
	helpBar := a.renderHelpBar()

	// compose
	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s", title, logView, searchBar, panel, helpBar)
}
```

- [ ] **Step 2: Create log view renderer**

Create `internal/tui/logview.go`:

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/justfun/logview/internal/model"
	"github.com/justfun/logview/internal/stacktrace"
)

func (a *App) renderLogView() string {
	visibleLines := a.visibleLines()
	if visibleLines < 1 {
		visibleLines = 1
	}

	// auto-scroll: keep cursor at bottom
	if a.autoscroll && len(a.filteredView) > 0 {
		a.cursor = len(a.filteredView) - 1
	}

	// calculate visible range
	start := a.cursor - visibleLines/2
	if start < 0 {
		start = 0
	}
	end := start + visibleLines
	if end > len(a.filteredView) {
		end = len(a.filteredView)
	}
	a.offset = start

	var b strings.Builder
	for i := start; i < end; i++ {
		line := a.filteredView[i]

		// check if this line is part of a folded stack trace
		folded := false
		for _, g := range a.stGroups {
			if i > g.Start && i <= g.End && !a.expanded[g.Start] {
				if i == g.Start+1 {
					// render fold indicator instead of first frame
					count := g.End - g.Start
					b.WriteString(FoldedStyle.Render(
						fmt.Sprintf("  (%d lines folded) [→展开]", count),
					))
					b.WriteByte('\n')
				}
				folded = true
				break
			}
		}
		if folded {
			continue
		}

		// render the line
		rendered := a.renderLine(line, i == a.cursor)
		b.WriteString(rendered)
		b.WriteByte('\n')
	}

	// new logs indicator
	if a.newLogs > 0 && !a.autoscroll {
		indicator := NewLogStyle.Render(fmt.Sprintf(" [新日志: %d条] ↓按g跳转", a.newLogs))
		b.WriteString(indicator)
	}

	return b.String()
}

func (a *App) renderLine(line *model.ParsedLine, selected bool) string {
	var parts []string

	for _, f := range model.AllFields {
		if !a.fieldMask.IsVisible(f) {
			continue
		}
		val := line.Get(f)
		if val == "" {
			continue
		}

		switch f {
		case model.FieldLevel:
			parts = append(parts, LevelStyle(val).Render(val))
		case model.FieldSource:
			parts = append(parts, fmt.Sprintf("[%s]", val))
		default:
			parts = append(parts, val)
		}
	}

	text := strings.Join(parts, "  ")

	// highlight search term
	if a.searchInput != "" {
		text = highlightText(text, a.searchInput)
	}

	if selected {
		return SelectedStyle.Render(text)
	}
	return text
}

func highlightText(text, query string) string {
	if query == "" {
		return text
	}
	idx := containsSubstr(text, query)
	if !idx {
		return text
	}
	// find actual position
	lower := strings.ToLower(text)
	lowerQ := strings.ToLower(query)
	result := ""
	for i := 0; i < len(text); {
		if i <= len(text)-len(query) && lower[i:i+len(query)] == lowerQ {
			result += HighlightStyle.Render(text[i : i+len(query)])
			i += len(query)
		} else {
			result += string(text[i])
			i++
		}
	}
	return result
}
```

Wait, `containsSubstr` returns `bool`, not an index. The highlightText function has a bug. Let me fix this properly.

- [ ] **Step 2 (revised): Create log view renderer**

Create `internal/tui/logview.go`:

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/justfun/logview/internal/model"
)

func (a *App) renderLogView() string {
	visibleLines := a.visibleLines()
	if visibleLines < 1 {
		visibleLines = 1
	}

	if a.autoscroll && len(a.filteredView) > 0 {
		a.cursor = len(a.filteredView) - 1
	}

	start := a.cursor - visibleLines/2
	if start < 0 {
		start = 0
	}
	end := start + visibleLines
	if end > len(a.filteredView) {
		end = len(a.filteredView)
	}
	a.offset = start

	var b strings.Builder
	for i := start; i < end; i++ {
		line := a.filteredView[i]

		folded := false
		for _, g := range a.stGroups {
			if i > g.Start && i <= g.End && !a.expanded[g.Start] {
				if i == g.Start+1 {
					count := g.End - g.Start
					b.WriteString(FoldedStyle.Render(
						fmt.Sprintf("  (%d lines folded) [→展开]", count),
					))
					b.WriteByte('\n')
				}
				folded = true
				break
			}
		}
		if folded {
			continue
		}

		rendered := a.renderLine(line, i == a.cursor)
		b.WriteString(rendered)
		b.WriteByte('\n')
	}

	if a.newLogs > 0 && !a.autoscroll {
		indicator := NewLogStyle.Render(fmt.Sprintf(" [新日志: %d条] ↓按g跳转", a.newLogs))
		b.WriteString(indicator)
	}

	return b.String()
}

func (a *App) renderLine(line *model.ParsedLine, selected bool) string {
	var parts []string

	for _, f := range model.AllFields {
		if !a.fieldMask.IsVisible(f) {
			continue
		}
		val := line.Get(f)
		if val == "" {
			continue
		}

		switch f {
		case model.FieldLevel:
			parts = append(parts, LevelStyle(val).Render(val))
		case model.FieldSource:
			parts = append(parts, fmt.Sprintf("[%s]", val))
		default:
			parts = append(parts, val)
		}
	}

	text := strings.Join(parts, "  ")

	if a.searchInput != "" {
		text = highlightText(text, a.searchInput)
	}

	if selected {
		return SelectedStyle.Render(text)
	}
	return text
}

func highlightText(text, query string) string {
	if query == "" {
		return text
	}

	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(query)
	qLen := len(lowerQuery)

	var result strings.Builder
	i := 0
	for i <= len(lowerText)-qLen {
		if lowerText[i:i+qLen] == lowerQuery {
			result.WriteString(HighlightStyle.Render(text[i : i+qLen]))
			i += qLen
		} else {
			result.WriteByte(text[i])
			i++
		}
	}
	// remaining chars
	for ; i < len(text); i++ {
		result.WriteByte(text[i])
	}
	return result.String()
}
```

- [ ] **Step 3: Create search bar + help bar renderers**

Create `internal/tui/searchbar.go`:

```go
package tui

import "fmt"

func (a *App) renderSearchBar() string {
	if a.searchMode {
		return SearchStyle.Render(fmt.Sprintf(" 搜索: %s█", a.searchInput))
	}
	if a.searchInput != "" {
		return SearchStyle.Render(fmt.Sprintf(" 搜索: %s", a.searchInput))
	}
	return SearchStyle.Render(" 按 / 搜索")
}
```

Create `internal/tui/helpbar.go`:

```go
package tui

import (
	"fmt"
	"strings"
)

func (a *App) renderHelpBar() string {
	var parts []string
	for _, b := range a.keymap.HelpEntries() {
		key := b.Help().Key
		desc := b.Help().Desc
		parts = append(parts, fmt.Sprintf("%s%s", HelpKeyStyle.Render(key), HelpStyle.Render(desc)))
	}
	return strings.Join(parts, "  ")
}
```

- [ ] **Step 4: Create bottom panel renderer**

Create `internal/tui/panel.go`:

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/justfun/logview/internal/model"
)

func (a *App) renderPanel() string {
	switch a.activePanel {
	case 0:
		return a.renderFieldsPanel()
	case 1:
		return a.renderLevelsPanel()
	case 2:
		return a.renderFilterPanel()
	}
	return ""
}

func (a *App) renderFieldsPanel() string {
	var items []string
	for _, f := range model.AllFields {
		checkBox := CheckboxUnchecked
		if a.fieldMask.IsVisible(f) {
			checkBox = CheckboxChecked
		}
		items = append(items, fmt.Sprintf("%s %s", checkBox, f))
	}
	return PanelStyle.Render("字段: " + strings.Join(items, "  "))
}

func (a *App) renderLevelsPanel() string {
	levels := []string{"DEBUG", "INFO", "WARN", "ERROR"}
	var items []string
	for _, l := range levels {
		checkBox := CheckboxUnchecked
		if a.levelMask[l] {
			checkBox = CheckboxChecked
		}
		items = append(items, fmt.Sprintf("%s %s", checkBox, LevelStyle(l).Render(l)))
	}
	return PanelStyle.Render("级别: " + strings.Join(items, "  "))
}

func (a *App) renderFilterPanel() string {
	traceDisplay := a.filterTraceID
	if traceDisplay == "" {
		traceDisplay = "(空)"
	}
	threadDisplay := a.filterThread
	if threadDisplay == "" {
		threadDisplay = "(空)"
	}
	return PanelStyle.Render(fmt.Sprintf("筛选: traceId=%s  thread=%s", traceDisplay, threadDisplay))
}
```

- [ ] **Step 5: Build and verify compilation**

```bash
go build -o logview . 2>&1
```

Expected: clean build, no errors

- [ ] **Step 6: Commit**

```bash
git add internal/tui/
git commit -m "feat: TUI root model, virtual scroll, search, panels, help bar"
```

---

### Task 13: Wire CLI to TUI (Tail + Pipe)

**Files:**
- Modify: `cmd/root.go`

- [ ] **Step 1: Wire tail and pipe commands to TUI**

Update `cmd/root.go` — replace the RunE functions for tail and pipe:

```go
var tailCmd = &cobra.Command{
	Use:   "tail <file> [file...] [flags]",
	Short: "View logs from local files",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		parsers, err := loadParsers()
		if err != nil {
			return err
		}
		src := stream.NewTailSource(args)
		app := tui.NewApp(src, parsers, bufferSize)
		p := tea.NewProgram(app, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

var pipeCmd = &cobra.Command{
	Use:   "pipe",
	Short: "View logs from stdin (pipe)",
	RunE: func(cmd *cobra.Command, args []string) error {
		parsers, err := loadParsers()
		if err != nil {
			return err
		}
		src := stream.NewPipeSource(os.Stdin)
		app := tui.NewApp(src, parsers, bufferSize)
		p := tea.NewProgram(app, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

func loadParsers() (*parser.AutoDetect, error) {
	homeDir, _ := os.UserHomeDir()
	rulesPath := filepath.Join(homeDir, ".logview", "rules.yaml")

	var rules []parser.RuleConfig
	if data, err := os.ReadFile(rulesPath); err == nil {
		rules, _ = parser.LoadRules(rulesPath)
	}
	if len(rules) == 0 {
		// built-in defaults
		rules = defaultRules()
	}
	parsers := parser.MustCompileRules(rules)
	return parser.NewAutoDetect(parsers), nil
}

func defaultRules() []parser.RuleConfig {
	return []parser.RuleConfig{
		{
			Name:    "java-logback",
			Pattern: `(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}) \[(?P<thread>[^\]]+)\] \[(?P<traceId>[^\]]+)\] (?P<level>\w+)\s+(?P<logger>\S+) - (?P<message>.*)`,
		},
		{
			Name:  "json-log",
			Parse: "json",
		},
		{
			Name:    "plain-text",
			Pattern: `(?P<message>.*)`,
		},
	}
}
```

Make sure imports include `"os"`, `"path/filepath"`, and the internal packages.

- [ ] **Step 2: Build and test**

```bash
go build -o logview .
echo -e "2026-05-15 09:27:01.130 [main] [abc123] INFO  com.example.App - hello\n2026-05-15 09:27:02.000 [main] [abc123] ERROR com.example.App - oops" | ./logview pipe
```

Expected: TUI opens showing parsed log lines with level coloring

- [ ] **Step 3: Commit**

```bash
git add cmd/root.go
git commit -m "feat: wire tail and pipe commands to TUI"
```

---

### Task 14: K8s Source + Multi-Pod Aggregation

**Files:**
- Create: `internal/stream/k8s.go`
- Test: `internal/stream/k8s_test.go`
- Modify: `cmd/root.go` (wire k8s command)

- [ ] **Step 1: Write the test**

Create `internal/stream/k8s_test.go`:

```go
package stream

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/justfun/logview/internal/model"
)

// mockKubectlCmd replaces exec.Command for testing.
func TestK8sSourceLabel(t *testing.T) {
	src := NewK8sSource("deploy/parking-api", "default", nil)
	label := src.Label()
	if label != "k8s/deploy/parking-api" {
		t.Errorf("Label() = %q, want 'k8s/deploy/parking-api'", label)
	}
}

func TestK8sSourceParseResource(t *testing.T) {
	tests := []struct {
		input   string
		want    K8sResource
		wantErr bool
	}{
		{"deploy/parking-api", K8sResource{Kind: "deployment", Name: "parking-api"}, false},
		{"pod/api-7d8f6-x9k2j", K8sResource{Kind: "pod", Name: "api-7d8f6-x9k2j"}, false},
		{"sts/data-store", K8sResource{Kind: "statefulset", Name: "data-store"}, false},
		{"invalid", K8sResource{}, true},
	}
	for _, tt := range tests {
		got, err := ParseK8sResource(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseK8sResource(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ParseK8sResource(%q) = %+v, want %+v", tt.input, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/stream/ -run TestK8s -v
```

Expected: FAIL

- [ ] **Step 3: Implement K8sSource**

Create `internal/stream/k8s.go`:

```go
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

// K8sResource represents a parsed k8s resource reference.
type K8sResource struct {
	Kind string // deployment, statefulset, pod
	Name string
}

// ParseK8sResource parses a resource string like "deploy/name" or "pod/name".
func ParseK8sResource(s string) (K8sResource, error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 || parts[1] == "" {
		return K8sResource{}, fmt.Errorf("invalid resource format: %q (expected kind/name)", s)
	}
	kind := strings.ToLower(parts[0])
	switch kind {
	case "deploy", "deployment":
		kind = "deployment"
	case "sts", "statefulset":
		kind = "statefulset"
	case "po", "pod":
		kind = "pod"
	default:
		return K8sResource{}, fmt.Errorf("unsupported resource kind: %q", parts[0])
	}
	return K8sResource{Kind: kind, Name: parts[1]}, nil
}

// K8sSource reads logs from Kubernetes pods via kubectl.
type K8sSource struct {
	resource  K8sResource
	namespace string
	podNames  []string // override: if set, skip discovery
	seq       atomic.Uint64
}

// NewK8sSource creates a k8s log source.
// podNames can be nil — they'll be auto-discovered for deployments.
func NewK8sSource(resource, namespace string, podNames []string) *K8sSource {
	res, _ := ParseK8sResource(resource)
	return &K8sSource{
		resource:  res,
		namespace: namespace,
		podNames:  podNames,
	}
}

func (k *K8sSource) Label() string {
	return fmt.Sprintf("k8s/%s/%s", k.resource.Kind, k.resource.Name)
}

func (k *K8sSource) Start(ctx context.Context) (<-chan model.RawLine, error) {
	pods := k.podNames
	var err error

	if len(pods) == 0 && k.resource.Kind != "pod" {
		pods, err = k.discoverPods(ctx)
		if err != nil {
			return nil, fmt.Errorf("discover pods: %w", err)
		}
	} else if k.resource.Kind == "pod" {
		pods = []string{k.resource.Name}
	}

	if len(pods) == 0 {
		return nil, fmt.Errorf("no pods found for %s", k.resource.Name)
	}

	ch := make(chan model.RawLine, 256)
	go func() {
		defer close(ch)
		var wg sync.WaitGroup
		for _, pod := range pods {
			wg.Add(1)
			go func(podName string) {
				defer wg.Done()
				k.streamPod(ctx, ch, podName)
			}(pod)
		}
		wg.Wait()
	}()

	return ch, nil
}

func (k *K8sSource) discoverPods(ctx context.Context) ([]string, error) {
	args := []string{"get", "pods",
		"-l", fmt.Sprintf("app=%s", k.resource.Name),
		"-n", k.namespace,
		"-o", "jsonpath={.items[*].metadata.name}",
	}
	out, err := exec.CommandContext(ctx, "kubectl", args...).Output()
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, fmt.Errorf("no pods found")
	}
	return strings.Fields(raw), nil
}

func (k *K8sSource) streamPod(ctx context.Context, ch chan<- model.RawLine, podName string) {
	args := []string{"logs", "-f", podName, "-n", k.namespace}
	cmd := exec.CommandContext(ctx, "kubectl", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}
	defer cmd.Wait()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := model.RawLine{
			Text:   scanner.Text(),
			Source: podName,
			Seq:    k.seq.Add(1),
		}
		select {
		case ch <- line:
		case <-ctx.Done():
			return
		}
	}
}

func (k *K8sSource) Cleanup() error { return nil }
```

Wait, I need to import `sync` for `sync.WaitGroup`. Let me add it.

- [ ] **Step 3 (revised): Implement K8sSource with correct imports**

The file above needs `"sync"` added to the imports. The corrected import block:

```go
import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/justfun/logview/internal/model"
)
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/stream/ -run TestK8s -v
```

Expected: PASS for Label and ParseK8sResource tests

- [ ] **Step 5: Wire k8s command**

Update the k8sCmd RunE in `cmd/root.go`:

```go
var k8sCmd = &cobra.Command{
	Use:   "k8s <resource> [flags]",
	Short: "View logs from Kubernetes pods",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		parsers, err := loadParsers()
		if err != nil {
			return err
		}
		namespace, _ := cmd.Flags().GetString("namespace")
		src := stream.NewK8sSource(args[0], namespace, nil)
		app := tui.NewApp(src, parsers, bufferSize)
		p := tea.NewProgram(app, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}
```

- [ ] **Step 6: Build and verify**

```bash
go build -o logview .
```

Expected: clean build

- [ ] **Step 7: Commit**

```bash
git add internal/stream/k8s.go internal/stream/k8s_test.go cmd/root.go
git commit -m "feat: K8s source with pod discovery and multi-pod aggregation"
```

---

### Task 15: Export Dialog + Async Export

**Files:**
- Create: `internal/tui/export_dlg.go`
- Modify: `internal/tui/app.go`

- [ ] **Step 1: Create export dialog**

Create `internal/tui/export_dlg.go`:

```go
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/justfun/logview/internal/export"
	"github.com/justfun/logview/internal/model"
)

// ExportState tracks the export dialog state.
type ExportState struct {
	Scope      int    // 0 = filtered, 1 = all
	Format     int    // 0 = raw, 1 = json
	FilePath   string
	Cursor     int    // which field is focused (0=scope, 1=format, 2=path)
	Exporting  bool
	Done       bool
	Exported   int
}

func newExportState() ExportState {
	return ExportState{
		FilePath: fmt.Sprintf("./logview-export-%s.log", time.Now().Format("20060102")),
	}
}

func (a *App) renderExportDialog() string {
	s := a.exportState
	dlgStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2)

	var b strings.Builder
	b.WriteString("导出日志\n\n")

	// scope
	scopeMarker := " "
	if s.Cursor == 0 {
		scopeMarker = "▸"
	}
	scopeOpts := []string{
		fmt.Sprintf("○ 当前筛选结果(%d条)", len(a.filteredView)),
		fmt.Sprintf("○ 全部缓冲区(%d条)", a.buffer.Len()),
	}
	scopeOpts[s.Scope] = strings.Replace(scopeOpts[s.Scope], "○", "●", 1)
	b.WriteString(fmt.Sprintf("%s 范围: %s  %s\n", scopeMarker, scopeOpts[0], scopeOpts[1]))

	// format
	formatMarker := " "
	if s.Cursor == 1 {
		formatMarker = "▸"
	}
	formatOpts := []string{"○ 原始日志", "○ 结构化(JSON)"}
	formatOpts[s.Format] = strings.Replace(formatOpts[s.Format], "○", "●", 1)
	b.WriteString(fmt.Sprintf("%s 格式: %s  %s\n", formatMarker, formatOpts[0], formatOpts[1]))

	// path
	pathMarker := " "
	if s.Cursor == 2 {
		pathMarker = "▸"
	}
	b.WriteString(fmt.Sprintf("%s 路径: %s\n", pathMarker, s.FilePath))

	if s.Done {
		b.WriteString(fmt.Sprintf("\n✓ 已导出 %d 条 → %s", s.Exported, s.FilePath))
	}

	b.WriteString("\n\n[Enter 确认]  [Esc 取消]")

	return dlgStyle.Render(b.String())
}

func (a *App) handleExportKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := &a.exportState
	switch msg.String() {
	case "esc", "q":
		a.exportMode = false
		return a, nil
	case "up", "k":
		if s.Cursor > 0 {
			s.Cursor--
		}
	case "down", "j":
		if s.Cursor < 2 {
			s.Cursor++
		}
	case "left", "h":
		switch s.Cursor {
		case 0:
			s.Scope = 0
		case 1:
			s.Format = 0
		}
	case "right", "l":
		switch s.Cursor {
		case 0:
			s.Scope = 1
		case 1:
			s.Format = 1
		}
	case "enter":
		a.doExport()
	}
	return a, nil
}

func (a *App) doExport() {
	s := &a.exportState
	var lines []*model.ParsedLine
	if s.Scope == 0 {
		lines = a.filteredView
	} else {
		for i := 0; i < a.buffer.Len(); i++ {
			if l := a.buffer.Get(i); l != nil {
				lines = append(lines, l)
			}
		}
	}

	format := export.FormatRaw
	if s.Format == 1 {
		format = export.FormatJSON
	}

	n, err := export.ToFile(lines, s.FilePath, format)
	if err != nil {
		s.Done = true
		s.Exported = 0
		return
	}
	s.Done = true
	s.Exported = n
}
```

- [ ] **Step 2: Add ExportState to App and update View**

Add to `App` struct in `app.go`:

```go
exportState ExportState
```

Initialize in `NewApp`:

```go
exportState: newExportState(),
```

Update the `View()` method to overlay the export dialog:

```go
func (a *App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	title := TitleStyle.Render(
		fmt.Sprintf(" LogView ─ %s [%s] ─ %d条 ", a.stream.Label(), a.parserName, a.buffer.Len()),
	)
	logView := a.renderLogView()
	searchBar := a.renderSearchBar()
	panel := a.renderPanel()
	helpBar := a.renderHelpBar()

	base := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", title, logView, searchBar, panel, helpBar)

	if a.exportMode {
		return base + "\n" + a.renderExportDialog()
	}
	return base
}
```

- [ ] **Step 3: Build**

```bash
go build -o logview .
```

Expected: clean build

- [ ] **Step 4: Commit**

```bash
git add internal/tui/export_dlg.go internal/tui/app.go
git commit -m "feat: export dialog with scope/format selection"
```

---

### Task 16: Integration Test + Default Config File

**Files:**
- Create: `testutil/testutil.go`
- Create: `internal/integration/pipe_test.go`

- [ ] **Step 1: Create test helper**

Create `testutil/testutil.go`:

```go
package testutil

// JavaLogbackLines returns sample log lines in Java Logback format.
func JavaLogbackLines() []string {
	return []string{
		"2026-05-15 09:27:01.130 [main] [abc123] INFO  com.example.App - hello world",
		"2026-05-15 09:27:02.000 [main] [abc123] WARN  com.example.App - something odd",
		"2026-05-15 09:27:03.000 [worker-1] [def456] ERROR com.example.App - java.lang.NullPointerException",
		"2026-05-15 09:27:03.001 [worker-1] [def456]   at com.example.App.run(App.java:42)",
		"2026-05-15 09:27:03.002 [worker-1] [def456]   at com.example.App.process(App.java:10)",
		"2026-05-15 09:27:04.000 [main] [abc123] INFO  com.example.App - done",
	}
}
```

- [ ] **Step 2: Write integration test for pipe source → parser → buffer**

Create `internal/integration/pipe_test.go`:

```go
package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/justfun/logview/internal/buffer"
	"github.com/justfun/logview/internal/model"
	"github.com/justfun/logview/internal/parser"
	"github.com/justfun/logview/internal/stream"
	"github.com/justfun/logview/internal/stacktrace"
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
					t.Fatalf("failed to parse line %d: %q", i, raw.Text)
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

	// verify parsed fields
	first := buf.Get(0)
	if first.Level != "INFO" {
		t.Errorf("first line level = %q, want INFO", first.Level)
	}
	if first.TraceID != "abc123" {
		t.Errorf("first line traceId = %q, want abc123", first.TraceID)
	}

	// verify stack trace detection
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
```

- [ ] **Step 3: Run integration test**

```bash
go test ./internal/integration/ -v
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add testutil/ internal/integration/
git commit -m "feat: integration test for pipe → parser → buffer pipeline"
```

---

### Task 17: Final Polish + README

**Files:**
- Create: `README.md`

- [ ] **Step 1: Create README**

Create `README.md`:

```markdown
# LogView

终端实时日志查看器，支持实时搜索、字段过滤、多Pod聚合。

## 安装

```bash
go build -o logview .
```

## 使用

```bash
# Kubernetes
logview k8s deploy/parking-api
logview k8s deploy/parking-api -n production

# 本地文件
logview tail /var/log/app.log

# 管道
kubectl logs -f deploy/parking-api | logview pipe
```

## 自定义解析规则

创建 `~/.logview/rules.yaml`：

```yaml
rules:
  - name: java-logback
    pattern: '(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}) \[(?P<thread>[^\]]+)\] \[(?P<traceId>[^\]]+)\] (?P<level>\w+)\s+(?P<logger>\S+) - (?P<message>.*)'
  - name: json-log
    pattern: '(?P<raw>.*)'
    parse: json
  - name: plain-text
    pattern: '(?P<message>.*)'
```

## 快捷键

| 按键 | 功能 |
|------|------|
| `/` | 搜索 |
| `Tab` | 切换面板 |
| `Enter` | 提取traceId/线程 |
| `f` | 字段显示控制 |
| `e` | 折叠/展开堆栈 |
| `s` | 导出 |
| `g` | 跳转顶/底 |
| `q` | 退出 |
```

- [ ] **Step 2: Run all tests**

```bash
go test ./... -v
```

Expected: all PASS

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: add README with usage and keyboard shortcuts"
```

---

## Spec Coverage Check

| Spec Requirement | Task |
|---|---|
| LogStream interface (k8s/tail/pipe) | Task 2, 3, 14 |
| 自定义正则解析 + 命名捕获组 | Task 4 |
| JSON 解析 | Task 5 |
| Rules YAML 加载 + 自动检测 | Task 4 |
| RingBuffer 环形缓冲区 | Task 6 |
| SearchIndex 搜索索引 | Task 7 |
| 虚拟滚动 | Task 12 (logview.go) |
| 搜索栏 + 实时搜索 | Task 12 (searchbar.go) |
| 字段显示控制 | Task 12 (panel.go) |
| 级别过滤 | Task 12 (panel.go) |
| TraceId/线程筛选 | Task 12 (panel.go) |
| 两步筛选 (Enter提取) | Task 12 (handleNormalKeys) |
| 异常堆栈折叠 | Task 8 + Task 12 |
| 多Pod聚合 | Task 14 |
| 导出 (raw/JSON) | Task 9 + Task 15 |
| 快捷键提示栏 | Task 12 (helpbar.go) |
| 回看暂停 + 新日志提示 | Task 12 (renderLogView) |
| Cobra CLI (k8s/tail/pipe) | Task 10 |
