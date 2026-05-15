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

type rulesFile struct {
	Rules []RuleConfig `yaml:"rules"`
}

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
}

func NewAutoDetect(parsers []Parser) *AutoDetect {
	return &AutoDetect{
		parsers: parsers,
		chosen:  make(map[string]Parser),
	}
}

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