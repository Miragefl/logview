package parser

import (
	"encoding/json"
	"time"

	"github.com/justfun/logview/internal/model"
)

type JSONParser struct {
	name string
}

func NewJSONParser(name string) *JSONParser {
	return &JSONParser{name: name}
}

func (p *JSONParser) Name() string { return p.name }

type jsonFields struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Thread  string `json:"thread"`
	TraceID string `json:"traceId"`
	Logger  string `json:"logger"`
	Message string `json:"message"`
}

func (p *JSONParser) Parse(raw model.RawLine) *model.ParsedLine {
	cleaned := ansiRe.ReplaceAllString(raw.Text, "")
	var f jsonFields
	if err := json.Unmarshal([]byte(cleaned), &f); err != nil {
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