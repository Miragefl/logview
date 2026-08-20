package parser

import (
	"fmt"
	"os"
	"strings"

	"github.com/justfun/logview/internal/model"
	"gopkg.in/yaml.v3"
)

type RuleConfig struct {
	Name    string `yaml:"name"`
	Pattern string `yaml:"pattern"`
	Parse   string `yaml:"parse,omitempty"`
	// Fallback 显式声明该规则为兜底解析器（默认规则名为 plain-text 时也视为兜底）。
	Fallback bool `yaml:"fallback,omitempty"`
}

// plainTextName 兜底解析器的固定名称：AutoDetect 依赖它识别"无结构化规则命中时"的降级目标。
const plainTextName = "plain-text"

type FieldConfig struct {
	Name    string `yaml:"name"`
	Visible bool   `yaml:"visible"`
}

type KeyBindingConfig struct {
	Action string `yaml:"action"`
	Key    string `yaml:"key"`
}

type rulesFile struct {
	Patterns    map[string]string `yaml:"patterns,omitempty"`
	Rules       []RuleConfig      `yaml:"rules"`
	Fields      []FieldConfig     `yaml:"fields,omitempty"`
	History     int               `yaml:"history,omitempty"`
	Theme       string            `yaml:"theme,omitempty"`
	ThemeColors map[string]string `yaml:"theme_colors,omitempty"`
	Hides       []string          `yaml:"hides,omitempty"`
	KeyBindings map[string]string `yaml:"keybindings,omitempty"`
	SSHHosts    []string          `yaml:"ssh_hosts,omitempty"`
}

// RulesResult 承载 rules.yaml 的全部加载结果，避免多返回值膨胀。
type RulesResult struct {
	Rules       []RuleConfig
	Fields      []FieldConfig
	History     int
	Theme       string
	ThemeColors map[string]string
	Hides       []string
	KeyBindings map[string]string
	SSHHosts    []string
}

func LoadRules(path string) (*RulesResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read rules: %w", err)
	}
	var rf rulesFile
	if err := yaml.Unmarshal(data, &rf); err != nil {
		return nil, fmt.Errorf("parse rules yaml: %w", err)
	}
	if len(rf.Patterns) > 0 {
		for i := range rf.Rules {
			rf.Rules[i].Pattern = expandPatterns(rf.Rules[i].Pattern, rf.Patterns)
		}
	}
	return &RulesResult{
		Rules:       rf.Rules,
		Fields:      rf.Fields,
		History:     rf.History,
		Theme:       rf.Theme,
		ThemeColors: rf.ThemeColors,
		Hides:       rf.Hides,
		KeyBindings: rf.KeyBindings,
		SSHHosts:    rf.SSHHosts,
	}, nil
}

func expandPatterns(pattern string, vars map[string]string) string {
	for key, val := range vars {
		pattern = strings.ReplaceAll(pattern, "{"+key+"}", val)
	}
	return pattern
}

// CompileRules 把规则配置编译为解析器列表；坏正则返回错误而非 panic。
func CompileRules(rules []RuleConfig) ([]Parser, error) {
	parsers := make([]Parser, 0, len(rules))
	for _, r := range rules {
		if r.Fallback {
			r.Name = plainTextName
		}
		if r.Parse == "json" {
			parsers = append(parsers, NewJSONParser(r.Name))
			continue
		}
		p, err := NewRegexParser(r.Name, r.Pattern)
		if err != nil {
			return nil, fmt.Errorf("rule %q: invalid regex: %w", r.Name, err)
		}
		parsers = append(parsers, p)
	}
	return parsers, nil
}

type AutoDetect struct {
	parsers []Parser
	chosen  map[string]Parser
	pending map[string][]model.RawLine // lines buffered before parser chosen
}

func NewAutoDetect(parsers []Parser) *AutoDetect {
	return &AutoDetect{
		parsers: parsers,
		chosen:  make(map[string]Parser),
		pending: make(map[string][]model.RawLine),
	}
}

// maxPending is the max lines buffered per source before forcing plain-text.
const maxPending = 50

// Detect returns the parser for a source. It tries structured parsers on every
// line (skipping plain-text). Once a structured parser matches, it's cached.
// If no structured parser matches within maxPending lines, plain-text is used
// as fallback, but subsequent lines still try structured parsers — if one matches
// later, the source is upgraded and pending lines are re-parsed.
func (ad *AutoDetect) Detect(raw model.RawLine) Parser {
	// Fast path: structured parser already chosen
	if p, ok := ad.chosen[raw.Source]; ok {
		if p.Name() != plainTextName {
			return p
		}
		// plain-text fallback: keep trying structured parsers
		for _, sp := range ad.parsers {
			if sp.Name() == plainTextName {
				continue
			}
			if sp.Parse(raw) != nil {
				ad.chosen[raw.Source] = sp
				delete(ad.pending, raw.Source)
				return sp
			}
		}
		return p
	}

	// No parser chosen yet: try structured parsers
	for _, p := range ad.parsers {
		if p.Name() == plainTextName {
			continue
		}
		if p.Parse(raw) != nil {
			ad.chosen[raw.Source] = p
			delete(ad.pending, raw.Source)
			return p
		}
	}

	// Buffer this line
	ad.pending[raw.Source] = append(ad.pending[raw.Source], raw)

	// Force plain-text if we've buffered too many lines without a match
	if len(ad.pending[raw.Source]) >= maxPending {
		for _, p := range ad.parsers {
			if p.Name() == plainTextName {
				ad.chosen[raw.Source] = p
				delete(ad.pending, raw.Source)
				return p
			}
		}
	}

	return nil
}

// DrainPending returns and clears buffered pending lines for all sources.
func (ad *AutoDetect) DrainPending() []model.RawLine {
	var all []model.RawLine
	for src, lines := range ad.pending {
		all = append(all, lines...)
		delete(ad.pending, src)
	}
	return all
}