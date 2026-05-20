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

	rules, _, _, _, _, _, err := LoadRules(fpath)
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

func TestLoadRulesWithPatterns(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `
patterns:
  time: '(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}[.,]\d{3})'
  thread: '(?P<thread>[^\]]+)'
  traceId: '(?P<traceId>[^\]]+)'
  level: '(?P<level>\w+)'
  logger: '(?P<logger>\S+)'
  message: '(?P<message>.*)'

rules:
  - name: java-logback
    pattern: '{time} \[{thread}\] \[{traceId}\] {level}\s+{logger} - {message}'
  - name: java-logback-notrace
    pattern: '{time} \[{thread}\] {level}\s+{logger} - {message}'
  - name: plain-text
    pattern: '{message}'
`
	fpath := filepath.Join(dir, "rules.yaml")
	os.WriteFile(fpath, []byte(yamlContent), 0644)

	rules, _, _, _, _, _, err := LoadRules(fpath)
	if err != nil {
		t.Fatalf("LoadRules() error: %v", err)
	}
	if len(rules) != 3 {
		t.Fatalf("got %d rules, want 3", len(rules))
	}

	// verify the patterns were expanded correctly
	parsers := MustCompileRules(rules)
	line := "2026-05-18 12:40:35.676 [park-pool-1-thread2] INFO  c.p.h.manager.CheckDeviceSchedule:78 - LK CAM 192.168.0.226 can not ping"
	parsed := parsers[1].Parse(model.RawLine{Text: line})
	if parsed == nil {
		t.Fatal("java-logback-notrace should match the test line")
	}
	if parsed.Get("level") != "INFO" {
		t.Errorf("level = %q, want INFO", parsed.Get("level"))
	}
	if parsed.Get("thread") != "park-pool-1-thread2" {
		t.Errorf("thread = %q, want park-pool-1-thread2", parsed.Get("thread"))
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