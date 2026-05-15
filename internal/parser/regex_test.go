package parser

import (
	"testing"

	"github.com/justfun/logview/internal/model"
)

func TestRegexParserJavaLogback(t *testing.T) {
	pattern := `(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}) \[(?P<thread>[^\]]+)\] \[(?P<traceId>[^\]]+)\] (?P<level>\w+)\s+(?P<logger>\S+) - (?P<message>.*)`
	p, err := NewRegexParser("java-logback", pattern)
	if err != nil {
		t.Fatalf("NewRegexParser() error: %v", err)
	}

	input := "2026-05-15 09:27:01.130 [MQTT Call: sit-mqtt-client-984169] [NA] INFO  com.ydcloud.smart.parking.mqtt.DefaultMqttCallback - ==========deliveryComplete=true=========="
	raw := model.RawLine{Text: input, Source: "pod-1"}
	result := p.Parse(raw)

	if result == nil {
		t.Fatal("Parse() returned nil")
	}
	if result.Level != "INFO" {
		t.Errorf("Level = %q, want %q", result.Level, "INFO")
	}
	if result.Thread != "MQTT Call: sit-mqtt-client-984169" {
		t.Errorf("Thread = %q, want %q", result.Thread, "MQTT Call: sit-mqtt-client-984169")
	}
	if result.TraceID != "NA" {
		t.Errorf("TraceID = %q, want %q", result.TraceID, "NA")
	}
	if result.Message != "==========deliveryComplete=true==========" {
		t.Errorf("Message = %q, want %q", result.Message, "==========deliveryComplete=true==========")
	}
	if result.Logger != "com.ydcloud.smart.parking.mqtt.DefaultMqttCallback" {
		t.Errorf("Logger = %q, want %q", result.Logger, "com.ydcloud.smart.parking.mqtt.DefaultMqttCallback")
	}
}

func TestRegexParserNoMatch(t *testing.T) {
	p, _ := NewRegexParser("test", `^(?P<level>\w+) (?P<message>.*)$`)
	raw := model.RawLine{Text: "no-separator-here"}
	result := p.Parse(raw)
	if result != nil {
		t.Error("expected nil for non-matching input")
	}
}

func TestRegexParserName(t *testing.T) {
	p, _ := NewRegexParser("my-rule", `(?P<message>.*)`)
	if p.Name() != "my-rule" {
		t.Errorf("Name() = %q, want %q", p.Name(), "my-rule")
	}
}