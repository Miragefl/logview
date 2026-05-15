package parser

import (
	"testing"

	"github.com/justfun/logview/internal/model"
)

func TestANSILogParsing(t *testing.T) {
	// kubectl 输出的带ANSI颜色码的真实日志
	raw := "\x1b[31m2026-05-15 10:21:59.305\x1b[0;39m \x1b[32m[http-nio-80-exec-10]\x1b[0;39m [f502fb023dc44603] \x1b[34mINFO \x1b[0;39m \x1b[1;35mcom.ydcloud.smart.parking.calc.service.CappingService\x1b[0;39m - [自然日封顶] 分组结果: 分组数=1"
	
	rule := RuleConfig{
		Name:    "java-logback",
		Pattern: `(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}) \[(?P<thread>[^\]]+)\] \[(?P<traceId>[^\]]+)\] (?P<level>\w+)\s+(?P<logger>\S+) - (?P<message>.*)`,
	}
	parsers := MustCompileRules([]RuleConfig{rule})
	ad := NewAutoDetect(parsers)

	rl := model.RawLine{Text: raw, Source: "billing-rule"}
	p := ad.Detect(rl)
	if p == nil {
		t.Fatal("没有匹配到解析器")
	}

	result := p.Parse(rl)
	if result == nil {
		t.Fatal("解析返回nil")
	}

	if result.Level != "INFO" {
		t.Errorf("Level = %q, want INFO", result.Level)
	}
	if result.Thread != "http-nio-80-exec-10" {
		t.Errorf("Thread = %q, want 'http-nio-80-exec-10'", result.Thread)
	}
	if result.TraceID != "f502fb023dc44603" {
		t.Errorf("TraceID = %q, want 'f502fb023dc44603'", result.TraceID)
	}
	if result.Logger != "com.ydcloud.smart.parking.calc.service.CappingService" {
		t.Errorf("Logger = %q, want 'com.ydcloud.smart.parking.calc.service.CappingService'", result.Logger)
	}
	if result.Message != "[自然日封顶] 分组结果: 分组数=1" {
		t.Errorf("Message = %q, want '[自然日封顶] 分组结果: 分组数=1'", result.Message)
	}
}

func TestCleanLogParsing(t *testing.T) {
	// 不带ANSI码的干净日志
	raw := "2026-05-15 10:21:59.305 [http-nio-80-exec-10] [f502fb023dc44603] INFO  com.ydcloud.smart.parking.calc.service.CappingService - [自然日封顶] 分组结果: 分组数=1"
	
	rule := RuleConfig{
		Name:    "java-logback",
		Pattern: `(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}) \[(?P<thread>[^\]]+)\] \[(?P<traceId>[^\]]+)\] (?P<level>\w+)\s+(?P<logger>\S+) - (?P<message>.*)`,
	}
	parsers := MustCompileRules([]RuleConfig{rule})
	ad := NewAutoDetect(parsers)

	rl := model.RawLine{Text: raw, Source: "billing-rule"}
	p := ad.Detect(rl)
	if p == nil {
		t.Fatal("没有匹配到解析器")
	}

	result := p.Parse(rl)
	if result == nil {
		t.Fatal("解析返回nil")
	}
	if result.Level != "INFO" {
		t.Errorf("Level = %q, want INFO", result.Level)
	}
	if result.TraceID != "f502fb023dc44603" {
		t.Errorf("TraceID = %q, want 'f502fb023dc44603'", result.TraceID)
	}
}
