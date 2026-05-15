package model

import (
	"testing"
	"time"
)

func TestParsedLineGet(t *testing.T) {
	now := time.Date(2026, 5, 15, 9, 27, 1, 130000000, time.UTC)
	p := ParsedLine{
		Raw:     RawLine{Text: "raw", Source: "pod-1"},
		Time:    now,
		Level:   "INFO",
		Thread:  "main",
		TraceID: "abc123",
		Logger:  "com.example.App",
		Message: "hello world",
	}

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

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  Level
	}{
		{"INFO", LevelInfo},
		{"error", LevelError},
		{"WARN", LevelWarn},
		{"debug", LevelDebug},
		{"FATAL", LevelError},
		{"unknown", LevelInfo},
	}
	for _, tt := range tests {
		got := ParseLevel(tt.input)
		if got != tt.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}