package parser

import (
	"regexp"
	"time"

	"github.com/justfun/logview/internal/model"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

type RegexParser struct {
	name   string
	re     *regexp.Regexp
	groups []string
}

func NewRegexParser(name, pattern string) (*RegexParser, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &RegexParser{
		name:   name,
		re:     re,
		groups: re.SubexpNames()[1:],
	}, nil
}

func (p *RegexParser) Name() string { return p.name }

func (p *RegexParser) Parse(raw model.RawLine) *model.ParsedLine {
	cleaned := ansiRe.ReplaceAllString(raw.Text, "")
	matches := p.re.FindStringSubmatch(cleaned)
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