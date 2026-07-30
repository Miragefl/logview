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
	// 裸词只搜 message：裸 ERROR 不再命中 level=ERROR
	q := parseSearchQuery("ERROR")
	// message 含 ERROR → 匹配
	if !q.MatchLine(parsedLine("INFO", "", "", "", "ERROR happened")) {
		t.Error("bare ERROR should match message containing ERROR")
	}
	// level=ERROR 但 message 不含 → 不匹配
	if q.MatchLine(parsedLine("ERROR", "abc", "main", "com.example.App", "something broke")) {
		t.Error("bare ERROR should NOT match level=ERROR without ERROR in message")
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
	q := parseSearchQuery("level:ERROR AND timeout")
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
	q := parseSearchQuery("level:ERROR OR level:WARN")
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
	q := parseSearchQuery("level:ERROR timeout")
	// implicit AND: both level:ERROR and timeout must be present
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
	// level:ERROR AND timeout OR level:WARN → (level:ERROR AND timeout) OR level:WARN
	q := parseSearchQuery("level:ERROR AND timeout OR level:WARN")
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
	q := parseSearchQuery("traceId:abc123 AND level:ERROR")
	line1 := parsedLine("ERROR", "abc123", "main", "com.example.App", "broke")
	if !q.MatchLine(line1) {
		t.Error("should match traceId:abc123 AND level:ERROR")
	}
	line2 := parsedLine("INFO", "abc123", "main", "com.example.App", "all good")
	if q.MatchLine(line2) {
		t.Error("should not match INFO even with correct traceId")
	}
}

func TestTimeRangeWithKeyword(t *testing.T) {
	q := parseSearchQuery("after:09:00 level:ERROR OR level:WARN")
	t0930 := time.Date(2026, 5, 15, 9, 30, 0, 0, time.Local)
	t0800 := time.Date(2026, 5, 15, 8, 0, 0, 0, time.Local)

	line1 := parsedLineWithTime("ERROR", "abc", "main", "com.example.App", "broke", t0930)
	if !q.MatchLine(line1) {
		t.Error("9:30 ERROR should match after:09:00 level:ERROR OR level:WARN")
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
	q := parseSearchQuery("level:ERROR AND traceId:abc123")
	kw := q.HighlightKeywords()
	if len(kw) != 2 {
		t.Fatalf("expected 2 keywords, got %d: %v", len(kw), kw)
	}
	if kw[0] != "ERROR" || kw[1] != "abc123" {
		t.Errorf("expected [ERROR, abc123], got %v", kw)
	}
}

func TestHighlightKeywordsOr(t *testing.T) {
	q := parseSearchQuery("timeout OR level:WARN")
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

// --- 新增：tokenizer 索引扫描识别 NOT/括号/引号 ---

func TestTokenizeNewTokens(t *testing.T) {
	cases := map[string][]tokenKind{
		"NOT level:ERROR":  {tokNot, tokField},
		"(ERROR OR WARN)":  {tokLParen, tokKeyword, tokOr, tokKeyword, tokRParen},
		`message:"a b"`:    {tokField},
		`"hello world"`:    {tokKeyword},
	}
	for input, want := range cases {
		toks := tokenize(input)
		if len(toks) != len(want) {
			t.Errorf("tokenize(%q) got %d tokens %v, want %d", input, len(toks), toks, len(want))
			continue
		}
		for i, w := range want {
			if toks[i].kind != w {
				t.Errorf("tokenize(%q)[%d] = %v, want %v", input, i, toks[i].kind, w)
			}
		}
	}
	toks := tokenize(`message:"a b"`)
	if toks[0].value != "a b" {
		t.Errorf(`message:"a b" value = %q, want "a b"`, toks[0].value)
	}
}

// --- time: 字段已移除 ---

func TestTimeFieldRemoved(t *testing.T) {
	q := parseSearchQuery("time:09:30")
	line := parsedLine("INFO", "", "", "", "some message")
	if q.MatchLine(line) {
		t.Error("time:09:30 as keyword should not match message without it")
	}
	for _, p := range fieldPrefixes {
		if p == "time:" {
			t.Error("time: should be removed from fieldPrefixes")
		}
	}
}

// --- 裸词只搜 message（breaking） ---

func TestBareWordMessageOnly(t *testing.T) {
	q := parseSearchQuery("abc123")
	if q.MatchLine(parsedLine("INFO", "abc123", "", "", "no match here")) {
		t.Error("bare word should not search traceId")
	}
	if q.MatchLine(parsedLine("INFO", "", "", "abc123", "no match here")) {
		t.Error("bare word should not search logger")
	}
	if !q.MatchLine(parsedLine("INFO", "", "", "", "trace abc123 found")) {
		t.Error("bare word should match message")
	}
}

// --- field: 后空格 + 大小写不敏感操作符（回归） ---

func TestFieldSpaceAndCaseInsensitiveOps(t *testing.T) {
	q := parseSearchQuery("message: JF350 and message: JF351")
	if kw := q.HighlightKeywords(); len(kw) != 2 || kw[0] != "JF350" || kw[1] != "JF351" {
		t.Errorf("expected [JF350 JF351], got %v", kw)
	}
	if !q.MatchLine(parsedLine("INFO", "", "", "", "任务 JF350 关联 JF351 处理")) {
		t.Error("message 同时含 JF350+JF351 的行应匹配")
	}
	if q.MatchLine(parsedLine("INFO", "", "", "", "只有 JF350")) {
		t.Error("只含 JF350 的行不应匹配 AND 查询")
	}
	qOr := parseSearchQuery("message: JF350 or message: JF351")
	if !qOr.MatchLine(parsedLine("INFO", "", "", "", "只有 JF350")) {
		t.Error("or 查询应匹配含 JF350 的行")
	}
	if !qOr.MatchLine(parsedLine("INFO", "", "", "", "只有 JF351")) {
		t.Error("or 查询应匹配含 JF351 的行")
	}
	qOp := parseSearchQuery("message: AND timeout")
	if !qOp.MatchLine(parsedLine("ERROR", "", "", "", "connection timeout")) {
		t.Error("message: AND timeout 应匹配含 timeout 的行")
	}
}

// --- NOT 操作符 ---

func TestNotOperator(t *testing.T) {
	q := parseSearchQuery("message:ERROR NOT timeout")
	if !q.MatchLine(parsedLine("ERROR", "", "", "", "[ERROR] boom")) {
		t.Error("ERROR NOT timeout should match ERROR without timeout")
	}
	if q.MatchLine(parsedLine("ERROR", "", "", "", "[ERROR] connection timeout")) {
		t.Error("ERROR NOT timeout should not match when timeout present")
	}
	q2 := parseSearchQuery("NOT level:DEBUG")
	if !q2.MatchLine(parsedLine("INFO", "", "", "", "x")) {
		t.Error("NOT level:DEBUG should match INFO")
	}
	if q2.MatchLine(parsedLine("DEBUG", "", "", "", "x")) {
		t.Error("NOT level:DEBUG should not match DEBUG")
	}
	q3 := parseSearchQuery("NOT NOT message:timeout")
	if !q3.MatchLine(parsedLine("INFO", "", "", "", "connection timeout")) {
		t.Error("NOT NOT timeout should match timeout")
	}
}

// --- 括号分组 ---

func TestParenGrouping(t *testing.T) {
	q := parseSearchQuery("(ERROR OR WARN) AND timeout")
	if !q.MatchLine(parsedLine("INFO", "", "", "", "ERROR timeout occurred")) {
		t.Error("(ERROR OR WARN) AND timeout should match ERROR+timeout")
	}
	if q.MatchLine(parsedLine("INFO", "", "", "", "timeout occurred")) {
		t.Error("should not match timeout without ERROR/WARN")
	}
}

func TestNestedParens(t *testing.T) {
	q := parseSearchQuery("ERROR OR (WARN AND timeout)")
	if !q.MatchLine(parsedLine("INFO", "", "", "", "ERROR here")) {
		t.Error("should match ERROR")
	}
	if !q.MatchLine(parsedLine("INFO", "", "", "", "WARN timeout here")) {
		t.Error("should match WARN AND timeout")
	}
	if q.MatchLine(parsedLine("INFO", "", "", "", "WARN something else")) {
		t.Error("WARN without timeout should not match second branch")
	}
}

// --- 中间态查询判定（输入操作符/括号/引号时不立即搜索） ---

func TestCurrentQueryPartialShowsAll(t *testing.T) {
	a := &App{}
	line := &model.ParsedLine{Message: "hello world", Raw: model.RawLine{Text: "hello world"}}
	for _, in := range []string{"not", "NOT", "and", "or", "(ERR", `"hello`, "ERROR not"} {
		a.searchInput = in
		q := a.currentQuery()
		if !q.IsEmpty() {
			t.Errorf("input %q: partial query should be empty (show all)", in)
		}
		if !q.MatchLine(line) {
			t.Errorf("input %q: partial query should match all lines", in)
		}
	}
	// 非中间态正常搜
	a.searchInput = "hello"
	if a.currentQuery().IsEmpty() {
		t.Error("non-partial query should not be empty")
	}
}

func TestIsPartialQuery(t *testing.T) {
	trues := []string{
		"and", "AND", "Or", "not",
		"ERROR and", "a or", "level:ERROR NOT",
		"(", "(ERROR", "(ERROR OR WARN",
		`"hello`, `message:"a b`,
	}
	falses := []string{
		"", "ERROR", "ERROR and timeout",
		"(ERROR)", "(ERROR OR WARN)",
		`"hello world"`, `message:"a b"`,
		"after:09:00", "level:ERROR",
	}
	for _, s := range trues {
		if !isPartialQuery(s) {
			t.Errorf("isPartialQuery(%q) = false, want true", s)
		}
	}
	for _, s := range falses {
		if isPartialQuery(s) {
			t.Errorf("isPartialQuery(%q) = true, want false", s)
		}
	}
}
