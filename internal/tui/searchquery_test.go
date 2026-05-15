package tui

import (
	"testing"
	"time"

	"github.com/justfun/logview/internal/model"
)

func parsedLine(level, traceID, thread, logger, message string) *model.ParsedLine {
	return &model.ParsedLine{
		Level:   level,
		TraceID: traceID,
		Thread:  thread,
		Logger:  logger,
		Message: message,
		Raw:     model.RawLine{Text: level + " " + traceID + " " + thread + " " + logger + " " + message},
	}
}

func parsedLineWithTime(level, traceID, thread, logger, message string, t time.Time) *model.ParsedLine {
	pl := parsedLine(level, traceID, thread, logger, message)
	pl.Time = t
	return pl
}

func TestSimpleKeyword(t *testing.T) {
	q := parseSearchQuery("ERROR")
	line := parsedLine("ERROR", "abc", "main", "com.example.App", "something broke")
	if !q.MatchLine(line) {
		t.Error("should match line with ERROR level")
	}
	line2 := parsedLine("INFO", "abc", "main", "com.example.App", "all good")
	if q.MatchLine(line2) {
		t.Error("should not match line without ERROR")
	}
}

func TestKeywordSearchInMessage(t *testing.T) {
	q := parseSearchQuery("timeout")
	line := parsedLine("ERROR", "abc", "main", "com.example.App", "connection timeout after 30s")
	if !q.MatchLine(line) {
		t.Error("should match message containing 'timeout'")
	}
}

func TestAndOperator(t *testing.T) {
	q := parseSearchQuery("ERROR AND timeout")
	line1 := parsedLine("ERROR", "abc", "main", "com.example.App", "connection timeout")
	if !q.MatchLine(line1) {
		t.Error("should match ERROR + timeout")
	}
	line2 := parsedLine("ERROR", "abc", "main", "com.example.App", "all good")
	if q.MatchLine(line2) {
		t.Error("should not match ERROR without timeout")
	}
	line3 := parsedLine("INFO", "abc", "main", "com.example.App", "connection timeout")
	if q.MatchLine(line3) {
		t.Error("should not match timeout without ERROR")
	}
}

func TestOrOperator(t *testing.T) {
	q := parseSearchQuery("ERROR OR WARN")
	line1 := parsedLine("ERROR", "abc", "main", "com.example.App", "broke")
	if !q.MatchLine(line1) {
		t.Error("should match ERROR")
	}
	line2 := parsedLine("WARN", "abc", "main", "com.example.App", "careful")
	if !q.MatchLine(line2) {
		t.Error("should match WARN")
	}
	line3 := parsedLine("INFO", "abc", "main", "com.example.App", "all good")
	if q.MatchLine(line3) {
		t.Error("should not match INFO")
	}
}

func TestImplicitAnd(t *testing.T) {
	q := parseSearchQuery("ERROR timeout")
	// implicit AND: both ERROR and timeout must be present
	line1 := parsedLine("ERROR", "abc", "main", "com.example.App", "connection timeout")
	if !q.MatchLine(line1) {
		t.Error("implicit AND should match both terms")
	}
	line2 := parsedLine("ERROR", "abc", "main", "com.example.App", "all good")
	if q.MatchLine(line2) {
		t.Error("implicit AND should not match only one term")
	}
}

func TestAndOrPrecedence(t *testing.T) {
	// ERROR AND timeout OR WARN → (ERROR AND timeout) OR WARN
	q := parseSearchQuery("ERROR AND timeout OR WARN")
	line1 := parsedLine("WARN", "abc", "main", "com.example.App", "careful")
	if !q.MatchLine(line1) {
		t.Error("WARN alone should match (OR branch)")
	}
	line2 := parsedLine("ERROR", "abc", "main", "com.example.App", "connection timeout")
	if !q.MatchLine(line2) {
		t.Error("ERROR + timeout should match (AND branch)")
	}
	line3 := parsedLine("ERROR", "abc", "main", "com.example.App", "all good")
	if q.MatchLine(line3) {
		t.Error("ERROR without timeout should not match AND branch")
	}
}

func TestFieldMatchTraceID(t *testing.T) {
	q := parseSearchQuery("traceId:abc123")
	line1 := parsedLine("ERROR", "abc123", "main", "com.example.App", "broke")
	if !q.MatchLine(line1) {
		t.Error("should match exact traceId")
	}
	line2 := parsedLine("ERROR", "xyz789", "main", "com.example.App", "broke")
	if q.MatchLine(line2) {
		t.Error("should not match different traceId")
	}
}

func TestFieldMatchLevel(t *testing.T) {
	q := parseSearchQuery("level:ERROR")
	line1 := parsedLine("ERROR", "abc", "main", "com.example.App", "broke")
	if !q.MatchLine(line1) {
		t.Error("should match level ERROR")
	}
	line2 := parsedLine("error", "abc", "main", "com.example.App", "broke")
	if !q.MatchLine(line2) {
		t.Error("should match level 'error' case-insensitively")
	}
	line3 := parsedLine("INFO", "abc", "main", "com.example.App", "all good")
	if q.MatchLine(line3) {
		t.Error("should not match level INFO")
	}
}

func TestTimeRange(t *testing.T) {
	q := parseSearchQuery("after:09:00 before:10:00")
	t0930 := time.Date(2026, 5, 15, 9, 30, 0, 0, time.Local)
	t1001 := time.Date(2026, 5, 15, 10, 1, 0, 0, time.Local)
	t0830 := time.Date(2026, 5, 15, 8, 30, 0, 0, time.Local)

	line1 := parsedLineWithTime("INFO", "abc", "main", "com.example.App", "test", t0930)
	if !q.MatchLine(line1) {
		t.Error("9:30 should be within 9:00~10:00")
	}
	line2 := parsedLineWithTime("INFO", "abc", "main", "com.example.App", "test", t1001)
	if q.MatchLine(line2) {
		t.Error("10:01 should be outside 9:00~10:00")
	}
	line3 := parsedLineWithTime("INFO", "abc", "main", "com.example.App", "test", t0830)
	if q.MatchLine(line3) {
		t.Error("8:30 should be outside 9:00~10:00")
	}
}

func TestFieldAndKeyword(t *testing.T) {
	q := parseSearchQuery("traceId:abc123 AND ERROR")
	line1 := parsedLine("ERROR", "abc123", "main", "com.example.App", "broke")
	if !q.MatchLine(line1) {
		t.Error("should match traceId:abc123 AND ERROR")
	}
	line2 := parsedLine("INFO", "abc123", "main", "com.example.App", "all good")
	if q.MatchLine(line2) {
		t.Error("should not match INFO even with correct traceId")
	}
}

func TestTimeRangeWithKeyword(t *testing.T) {
	q := parseSearchQuery("after:09:00 ERROR OR WARN")
	t0930 := time.Date(2026, 5, 15, 9, 30, 0, 0, time.Local)
	t0800 := time.Date(2026, 5, 15, 8, 0, 0, 0, time.Local)

	line1 := parsedLineWithTime("ERROR", "abc", "main", "com.example.App", "broke", t0930)
	if !q.MatchLine(line1) {
		t.Error("9:30 ERROR should match after:09:00 ERROR OR WARN")
	}
	line2 := parsedLineWithTime("WARN", "abc", "main", "com.example.App", "careful", t0930)
	if !q.MatchLine(line2) {
		t.Error("9:30 WARN should match")
	}
	line3 := parsedLineWithTime("ERROR", "abc", "main", "com.example.App", "broke", t0800)
	if q.MatchLine(line3) {
		t.Error("8:00 ERROR should not match after:09:00")
	}
}

func TestEmptyQuery(t *testing.T) {
	q := parseSearchQuery("")
	if !q.IsEmpty() {
		t.Error("empty query should be IsEmpty")
	}
	line := parsedLine("INFO", "abc", "main", "com.example.App", "test")
	if !q.MatchLine(line) {
		t.Error("empty query should match everything")
	}
}

func TestTimeRangeHint(t *testing.T) {
	q1 := parseSearchQuery("after:09:00 before:10:00 ERROR")
	hint := q1.TimeRangeHint()
	if hint != "09:00~10:00" {
		t.Errorf("expected '09:00~10:00', got %q", hint)
	}
	q2 := parseSearchQuery("ERROR")
	if q2.TimeRangeHint() != "" {
		t.Errorf("expected empty hint, got %q", q2.TimeRangeHint())
	}
}

func TestHighlightKeywords(t *testing.T) {
	q := parseSearchQuery("ERROR AND traceId:abc123")
	kw := q.HighlightKeywords()
	if len(kw) != 2 {
		t.Fatalf("expected 2 keywords, got %d: %v", len(kw), kw)
	}
	if kw[0] != "ERROR" || kw[1] != "abc123" {
		t.Errorf("expected [ERROR, abc123], got %v", kw)
	}
}

func TestHighlightKeywordsOr(t *testing.T) {
	q := parseSearchQuery("timeout OR WARN")
	kw := q.HighlightKeywords()
	if len(kw) != 2 {
		t.Fatalf("expected 2 keywords, got %d: %v", len(kw), kw)
	}
	found := map[string]bool{}
	for _, k := range kw {
		found[k] = true
	}
	if !found["timeout"] || !found["WARN"] {
		t.Errorf("expected timeout and WARN, got %v", kw)
	}
}
