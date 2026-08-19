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

	rules, _, _, _, _, _, _, err := LoadRules(fpath)
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

	rules, _, _, _, _, _, _, err := LoadRules(fpath)
	if err != nil {
		t.Fatalf("LoadRules() error: %v", err)
	}
	if len(rules) != 3 {
		t.Fatalf("got %d rules, want 3", len(rules))
	}

	// verify the patterns were expanded correctly
	parsers, _ := CompileRules(rules)
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
	parsers, _ := CompileRules(rules)
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

func TestCompileRulesInvalidRegexReturnsError(t *testing.T) {
	rules := []RuleConfig{{Name: "bad-rule", Pattern: `(?P<broken>[`}}
	if _, err := CompileRules(rules); err == nil {
		t.Fatal("CompileRules should return error for invalid regex, got nil")
	}
}

func TestCompileRulesFallbackMarker(t *testing.T) {
	// fallback: true 的规则无论叫什么名字都应被编译为兜底解析器
	rules := []RuleConfig{
		{Name: "java", Pattern: `(?P<level>\w+) (?P<message>.*)`},
		{Name: "my-custom-name", Pattern: `(?P<message>.*)`, Fallback: true},
	}
	parsers, err := CompileRules(rules)
	if err != nil {
		t.Fatalf("CompileRules() error: %v", err)
	}
	if parsers[1].Name() != "plain-text" {
		t.Errorf("fallback rule name = %q, want %q", parsers[1].Name(), "plain-text")
	}
}

func TestAutoDetectBuffersPendingLines(t *testing.T) {
	rules := []RuleConfig{
		{Name: "java", Pattern: `^INFO `},
		{Name: "plain-text", Pattern: `(?P<message>.*)`},
	}
	parsers, _ := CompileRules(rules)
	ad := NewAutoDetect(parsers)

	// 不匹配结构化规则的行：前 maxPending-1 行返回 nil 并缓冲
	for i := 0; i < maxPending-1; i++ {
		if p := ad.Detect(model.RawLine{Text: "plain line", Source: "s1"}); p != nil {
			t.Fatalf("Detect() should buffer line %d, got parser %q", i, p.Name())
		}
	}
	if got := len(ad.DrainPending()); got != maxPending-1 {
		t.Errorf("pending = %d, want %d", got, maxPending-1)
	}
}

func TestAutoDetectFallsBackAfterMaxPending(t *testing.T) {
	rules := []RuleConfig{
		{Name: "java", Pattern: `^INFO `},
		{Name: "plain-text", Pattern: `(?P<message>.*)`},
	}
	parsers, _ := CompileRules(rules)
	ad := NewAutoDetect(parsers)

	var p Parser
	for i := 0; i < maxPending; i++ {
		p = ad.Detect(model.RawLine{Text: "plain line", Source: "s1"})
	}
	if p == nil || p.Name() != "plain-text" {
		t.Fatalf("after %d unmatched lines, want plain-text fallback, got %v", maxPending, p)
	}
}

func TestAutoDetectUpgradesFromFallback(t *testing.T) {
	rules := []RuleConfig{
		{Name: "java", Pattern: `^INFO `},
		{Name: "plain-text", Pattern: `(?P<message>.*)`},
	}
	parsers, _ := CompileRules(rules)
	ad := NewAutoDetect(parsers)

	// 先降级到 plain-text
	for i := 0; i < maxPending; i++ {
		ad.Detect(model.RawLine{Text: "plain line", Source: "s1"})
	}
	// 之后结构化行到达：应升级为 java 并清空 pending
	p := ad.Detect(model.RawLine{Text: "INFO structured now", Source: "s1"})
	if p == nil || p.Name() != "java" {
		t.Fatalf("fallback should upgrade to java, got %v", p)
	}
	if got := len(ad.DrainPending()); got != 0 {
		t.Errorf("pending after upgrade = %d, want 0", got)
	}
}