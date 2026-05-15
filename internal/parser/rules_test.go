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

	rules, _, err := LoadRules(fpath)
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