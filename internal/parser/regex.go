package parser

import (
	"regexp"
	"time"

	"github.com/justfun/logview/internal/model"
)

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
	cleaned := model.StripANSI(raw.Text)
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
			// 布局按精度降级尝试；无日期格式落到 0000-01-01（匹配层按时分窗口比较）
			if t, err := time.ParseInLocation("2006-01-02 15:04:05.000", val, time.Local); err == nil {
				result.Time = t
			} else if t, err := time.ParseInLocation("2006-01-02 15:04:05", val, time.Local); err == nil {
				result.Time = t
			} else if t, err := time.ParseInLocation("2006-01-02T15:04:05", val, time.Local); err == nil {
				result.Time = t
			} else if t, err := time.ParseInLocation("15:04:05.000", val, time.Local); err == nil {
				result.Time = t
			} else if t, err := time.ParseInLocation("15:04:05", val, time.Local); err == nil {
				result.Time = t
			} else if t, err := time.ParseInLocation("15:04:05,000", val, time.Local); err == nil {
				result.Time = t
			} else if t, err := time.ParseInLocation("15:04", val, time.Local); err == nil {
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