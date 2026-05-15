package parser

import "github.com/justfun/logview/internal/model"

type JSONParser struct {
	name string
}

func NewJSONParser(name string) *JSONParser {
	return &JSONParser{name: name}
}

func (p *JSONParser) Name() string { return p.name }
func (p *JSONParser) Parse(raw model.RawLine) *model.ParsedLine { return nil }