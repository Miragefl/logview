package model

import (
	"testing"
	"time"
)

func TestParsedLineGet(t *testing.T) {
	now := time.Date(2026, 5, 15, 9, 27, 1, 130000000, time.UTC)
	p := ParsedLine{
		Raw: RawLine{Text: "raw", Source: "pod-1"},
		Fields: map[Field]string{
			FieldTime:    "09:27:01.130",
			FieldLevel:   "INFO",
			FieldThread:  "main",
			FieldTraceID: "abc123",
			FieldLogger:  "com.example.App",
			FieldMessage: "hello world",
		},
	}
	p.SetUnixMs(now.UnixMilli())

	tests := []struct {
		field Field
		want  string
	}{
		{FieldTime, "09:27:01.130"},
		{FieldLevel, "INFO"},
		{FieldThread, "main"},
		{FieldTraceID, "abc123"},
		{FieldLogger, "com.example.App"},
		{FieldMessage, "hello world"},
		{FieldSource, "pod-1"},
	}

	for _, tt := range tests {
		got := p.Get(tt.field)
		if got != tt.want {
			t.Errorf("Get(%s) = %q, want %q", tt.field, got, tt.want)
		}
	}
}

// base 去重驻留回归(迭代 10):InitBase 传入与 Raw.Text 同串(无 ANSI 行
// StripANSI 零拷贝产物)时不驻留 header(16B/行),std 取值经 Raw.Text 回落;
// 含 ANSI 行的独立清洗串仍须驻留并正确取值。
func TestBaseDedupFallback(t *testing.T) {
	raw := "2026-10-11 10:00:00 [main] INFO c.x.S - hello"
	p := ParsedLine{Raw: RawLine{Text: raw, Source: "f.log"}}
	p.InitBase(raw) // 同串:不应驻留
	if p.base != "" {
		t.Fatalf("同串 base 不应驻留, got %q", p.base)
	}
	p.SetStdSpan(FieldMessage, 40, 5) // 防御长度按 Raw.Text 判定("hello" 起始偏移 40)
	if got := p.Message(); got != "hello" {
		t.Fatalf("回落取值 Message() = %q, want hello", got)
	}

	// 含 ANSI 行:清洗串(≠原串)独立驻留,span 索引清洗串
	p2 := ParsedLine{Raw: RawLine{Text: "\x1b[31mhello\x1b[0m", Source: "f.log"}}
	p2.InitBase("hello")
	if p2.base != "hello" {
		t.Fatalf("独立清洗串应驻留, got %q", p2.base)
	}
	p2.SetStdSpan(FieldMessage, 0, 5)
	if got := p2.Message(); got != "hello" {
		t.Fatalf("清洗串取值 Message() = %q, want hello", got)
	}

	// 同内容不同实例:内容相等即等价(span 语义相同),同样免驻留
	p3 := ParsedLine{Raw: RawLine{Text: "abc", Source: "f.log"}}
	p3.InitBase("ab" + "c")
	if p3.base != "" || p3.Message() != "" {
		t.Fatal("内容相等的 base 应免驻留且取值正常")
	}
	p3.SetStdSpan(FieldMessage, 0, 3)
	if p3.Message() != "abc" {
		t.Fatalf("相等串免驻留后取值 = %q, want abc", p3.Message())
	}
}

func TestFieldMaskToggle(t *testing.T) {
	fm := DefaultFieldMask()
	if !fm.IsVisible(FieldTime) {
		t.Error("time should be visible by default")
	}
	fm.Toggle(FieldTime)
	if fm.IsVisible(FieldTime) {
		t.Error("time should be hidden after toggle")
	}
	fm.Toggle(FieldTime)
	if !fm.IsVisible(FieldTime) {
		t.Error("time should be visible after second toggle")
	}
}