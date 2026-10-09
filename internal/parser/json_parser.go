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
	cleaned := model.StripANSI(raw.Text)
	var f jsonFields
	if err := json.Unmarshal([]byte(cleaned), &f); err != nil {
		return nil
	}

	// json 字段值由解码产生、不在原文中,无法以 span 引用——全量进 Fields map
	// (stdOrMap/Get 均优先查 map,消费端无感);时间另存 unixMs 供比较运算
	result := &model.ParsedLine{
		Raw: raw,
		Fields: map[model.Field]string{
			model.FieldLevel:   f.Level,
			model.FieldThread:  f.Thread,
			model.FieldTraceID: f.TraceID,
			model.FieldLogger:  f.Logger,
			model.FieldMessage: f.Message,
		},
	}

	if f.Time != "" {
		result.Fields[model.FieldTime] = f.Time
		for _, layout := range []string{
			"2006-01-02T15:04:05.000Z",
			"2006-01-02T15:04:05Z",
			"2006-01-02 15:04:05.000",
			"2006-01-02 15:04:05",
			time.RFC3339,
			time.RFC3339Nano,
			"15:04:05.000", // 无日期：0000-01-01，匹配层按时分窗口
			"15:04:05",
			"15:04",
		} {
			if t, err := time.ParseInLocation(layout, f.Time, time.Local); err == nil {
				result.SetUnixMs(t.UnixMilli())
				break
			}
		}
	}

	return result
}