package parser

import (
	"reflect"
	"testing"

	"github.com/justfun/logview/internal/model"
)

func TestJSONParser(t *testing.T) {
	p := NewJSONParser("json-log")

	input := `{"time":"2026-05-15T09:27:01.130Z","level":"INFO","thread":"main","traceId":"abc123","logger":"com.example.App","message":"hello world"}`
	raw := model.RawLine{Text: input, Source: "pod-1"}
	result := p.Parse(raw)

	if result == nil {
		t.Fatal("Parse() returned nil")
	}
	if result.Level() != "INFO" {
		t.Errorf("Level = %q, want INFO", result.Level())
	}
	if result.Message() != "hello world" {
		t.Errorf("Message = %q, want 'hello world'", result.Message())
	}
	if result.TraceID() != "abc123" {
		t.Errorf("TraceID = %q, want 'abc123'", result.TraceID())
	}
}

func TestJSONParserInvalidJSON(t *testing.T) {
	p := NewJSONParser("json-log")
	raw := model.RawLine{Text: "not json at all"}
	result := p.Parse(raw)
	if result != nil {
		t.Error("expected nil for invalid JSON")
	}
}

func TestJSONParserName(t *testing.T) {
	p := NewJSONParser("my-json")
	if p.Name() != "my-json" {
		t.Errorf("Name() = %q, want 'my-json'", p.Name())
	}
}
func TestJSONParserFields(t *testing.T) {
	// JSON 解析可产出字段集固定六个(与 jsonFields 的 json key 一致)
	p := NewJSONParser("json-log")
	want := []string{"time", "level", "thread", "traceId", "logger", "message"}
	if !reflect.DeepEqual(p.Fields(), want) {
		t.Fatalf("Fields() = %v, want %v", p.Fields(), want)
	}
}
