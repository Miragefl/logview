package parser

import (
	"fmt"
	"os"

	"github.com/justfun/logview/internal/model"
	"gopkg.in/yaml.v3"
)

type RuleConfig struct {
	Name    string `yaml:"name"`
	Pattern string `yaml:"pattern"`
	Parse   string `yaml:"parse,omitempty"`
}

type FieldConfig struct {
	Name    string `yaml:"name"`
	Visible bool   `yaml:"visible"`
}

type rulesFile struct {
	Rules  []RuleConfig `yaml:"rules"`
	Fields []FieldConfig `yaml:"fields,omitempty"`
}

func LoadRules(path string) ([]RuleConfig, []FieldConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read rules: %w", err)
	}
	var rf rulesFile
	if err := yaml.Unmarshal(data, &rf); err != nil {
		return nil, nil, fmt.Errorf("parse rules yaml: %w", err)
	}
	return rf.Rules, rf.Fields, nil
}

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

// Detect returns the parser for a source. First N lines are tested against
// structured parsers only (skipping plain-text). Once a structured parser matches,
// it's cached. If no structured parser matches within maxPending lines, plain-text
// is used as fallback.
func (ad *AutoDetect) Detect(raw model.RawLine) Parser {
	if p, ok := ad.chosen[raw.Source]; ok {
		return p
	}

	// Only try structured parsers, skip plain-text catch-all
	for _, p := range ad.parsers {
		if p.Name() == "plain-text" {
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
			if p.Name() == "plain-text" {
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