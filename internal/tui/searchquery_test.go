package tui

import (
	"strings"
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
	anchor := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	q := parseSearchQueryAt("time:>09:00 time:<10:00", anchor)
	t0930 := time.Date(2026, 5, 15, 9, 30, 0, 0, time.UTC)
	t1001 := time.Date(2026, 5, 15, 10, 1, 0, 0, time.UTC)
	t0830 := time.Date(2026, 5, 15, 8, 30, 0, 0, time.UTC)

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
	// 跨天：昨天 23:50 不被 time:>09:00（今天锚定）命中
	yesterday := time.Date(2026, 5, 14, 23, 50, 0, 0, time.UTC)
	line4 := parsedLineWithTime("INFO", "abc", "main", "com.example.App", "test", yesterday)
	if q.MatchLine(line4) {
		t.Error("yesterday 23:50 should not match today-anchored >09:00")
	}
}

func TestTimeRangeSugar(t *testing.T) {
	anchor := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	q := parseSearchQueryAt("time:9:00..10:00", anchor) // 前导零宽容 + 区间闭区间
	inLow := parsedLineWithTime("INFO", "", "", "", "x", time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC))
	inHigh := parsedLineWithTime("INFO", "", "", "", "x", time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC))
	below := parsedLineWithTime("INFO", "", "", "", "x", time.Date(2026, 5, 15, 8, 59, 59, 0, time.UTC))
	above := parsedLineWithTime("INFO", "", "", "", "x", time.Date(2026, 5, 15, 10, 0, 1, 0, time.UTC))
	if !q.MatchLine(inLow) || !q.MatchLine(inHigh) {
		t.Error("区间糖应含两端（闭区间）")
	}
	if q.MatchLine(below) || q.MatchLine(above) {
		t.Error("区间糖两端之外不命中")
	}
}

func TestTimeRelative(t *testing.T) {
	anchor := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	q := parseSearchQueryAt("time:>-10m", anchor)
	in := parsedLineWithTime("INFO", "", "", "", "x", anchor.Add(-5*time.Minute))
	out := parsedLineWithTime("INFO", "", "", "", "x", anchor.Add(-11*time.Minute))
	if !q.MatchLine(in) || q.MatchLine(out) {
		t.Error("time:>-10m 应命中 anchor 前 10 分钟内")
	}
	// 简写 time:-1h 等价 >-1h
	q2 := parseSearchQueryAt("time:-1h", anchor)
	in2 := parsedLineWithTime("INFO", "", "", "", "x", anchor.Add(-30*time.Minute))
	if !q2.MatchLine(in2) {
		t.Error("time:-1h 简写应等价 >-1h")
	}
	// 复合时长
	q3 := parseSearchQueryAt("time:>-1h30m", anchor)
	in3 := parsedLineWithTime("INFO", "", "", "", "x", anchor.Add(-90*time.Minute+time.Second))
	if !q3.MatchLine(in3) {
		t.Error("time:>-1h30m 应命中 anchor 前 90 分钟内")
	}
}

func TestTimeUTCAnchor(t *testing.T) {
	// 日期字面量 + 秒级 + RFC3339 T 与 _ 别名
	anchor := time.Date(2026, 8, 28, 23, 0, 0, 0, time.UTC)
	q := parseSearchQueryAt("time:>2026-08-26T09:00:30", anchor)
	in := parsedLineWithTime("INFO", "", "", "", "x", time.Date(2026, 8, 26, 9, 0, 31, 0, time.UTC))
	out := parsedLineWithTime("INFO", "", "", "", "x", time.Date(2026, 8, 26, 9, 0, 30, 0, time.UTC))
	if !q.MatchLine(in) || q.MatchLine(out) {
		t.Error("time:> 应为严格大于")
	}
	q2 := parseSearchQueryAt("time:>=2026-08-26_09:00:30", anchor) // _ 别名 + 含边界
	boundary := parsedLineWithTime("INFO", "", "", "", "x", time.Date(2026, 8, 26, 9, 0, 30, 0, time.UTC))
	if !q2.MatchLine(boundary) {
		t.Error("time:>= 应含边界行（cron 整点不丢）")
	}
}

func TestTimeErrors(t *testing.T) {
	// 非法值必须显式报错（不静默降级），查询匹配空集
	for _, bad := range []string{
		"time:09:00",     // 缺比较符
		"time:>8-26",     // MM-DD 已删
		"time:>9:0",      // 格式错
		"time:>",         // 空值
		"time:>10:00..",  // 区间缺右值
		"time:-10x",      // 非法单位
		"time:>+10m",     // 正号拒绝
		"time:>2026-02-30", // 日期非法
	} {
		q := parseSearchQuery(bad)
		if q.ParseError() == "" {
			t.Errorf("%q 应产生解析错误", bad)
		}
		if q.MatchLine(parsedLine("INFO", "", "", "", "anything")) {
			t.Errorf("%q 解析错误时查询应为空集", bad)
		}
	}
	// 错误经 OR 右支传播：整体空集
	q := parseSearchQuery("level:ERROR OR time:>xx")
	if q.ParseError() == "" || q.MatchLine(parsedLine("ERROR", "", "", "", "boom")) {
		t.Error("OR 右支 time 错误应使整体空集")
	}
}

func TestTimeNullPropagation(t *testing.T) {
	anchor := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	noTime := parsedLine("INFO", "abc", "main", "App", "msg") // 无时间字段
	q := parseSearchQueryAt("time:>09:00", anchor)
	if q.MatchLine(noTime) {
		t.Error("无时间行不参与 time 条件")
	}
	q2 := parseSearchQueryAt("NOT time:>09:00", anchor)
	if q2.MatchLine(noTime) {
		t.Error("NOT time 也不得复活无时间行（NULL 传播）")
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
	anchor := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	q := parseSearchQueryAt("time:>09:00 (level:ERROR OR level:WARN)", anchor)
	t0930 := time.Date(2026, 5, 15, 9, 30, 0, 0, time.UTC)
	t0800 := time.Date(2026, 5, 15, 8, 0, 0, 0, time.UTC)

	line1 := parsedLineWithTime("ERROR", "abc", "main", "com.example.App", "broke", t0930)
	if !q.MatchLine(line1) {
		t.Error("9:30 ERROR should match time:>09:00 (level:ERROR OR level:WARN)")
	}
	line2 := parsedLineWithTime("WARN", "abc", "main", "com.example.App", "careful", t0930)
	if !q.MatchLine(line2) {
		t.Error("9:30 WARN should match")
	}
	line3 := parsedLineWithTime("ERROR", "abc", "main", "com.example.App", "broke", t0800)
	if q.MatchLine(line3) {
		t.Error("8:00 ERROR should not match time:>09:00")
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
	anchor := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	q1 := parseSearchQueryAt("time:>09:00 time:<10:00 ERROR", anchor)
	hint := q1.TimeRangeHint()
	want := "05-15 09:00:00~05-15 10:00:00"
	if hint != want {
		t.Errorf("expected %q, got %q", want, hint)
	}
	// 相对时间显示本地视角锚点 + 原始写法
	q2 := parseSearchQueryAt("time:>-10m", anchor)
	h2 := q2.TimeRangeHint()
	want2 := anchor.Add(-10 * time.Minute).Format("01-02 15:04:05")
	if !strings.Contains(h2, want2) || !strings.Contains(h2, "-10m") {
		t.Errorf("relative hint 应含本地锚点 %s 与原始写法, got %q", want2, h2)
	}
	// 无 time 条件 → 空
	q3 := parseSearchQuery("ERROR")
	if q3.TimeRangeHint() != "" {
		t.Errorf("expected empty hint, got %q", q3.TimeRangeHint())
	}
	// OR 语境不显示
	q4 := parseSearchQueryAt("time:>09:00 OR level:ERROR", anchor)
	if q4.TimeRangeHint() != "" {
		t.Errorf("OR 语境不应显示区间 hint, got %q", q4.TimeRangeHint())
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

// --- time: 为时间条件入口（历史：曾作为普通前缀移除，v2 起为时间语义） ---

func TestTimeFieldPresent(t *testing.T) {
	found := false
	for _, p := range fieldPrefixes {
		if p == "time:" {
			found = true
		}
	}
	if !found {
		t.Error("time: 应在 fieldPrefixes 中")
	}
	// 裸 >X 不进时间语义（字面量关键字）
	q := parseSearchQuery(">09:00")
	line := parsedLine("INFO", "", "", "", "value >09:00 here")
	if !q.MatchLine(line) {
		t.Error("裸 >09:00 应为字面量关键字搜索")
	}
	if q.ParseError() != "" {
		t.Error("裸 >09:00 不应产生解析错误")
	}
	// after:/before: 前缀移除后按未知前缀 → 关键字
	q2 := parseSearchQuery("after:09:00")
	if q2.ParseError() != "" {
		t.Error("after: 移除后应按普通词处理（不报错）")
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

func TestCurrentQueryPartialStripsOperator(t *testing.T) {
	a := &App{}
	line := &model.ParsedLine{Message: "hello world", Raw: model.RawLine{Text: "hello world"}}
	// 纯操作符中间态：剥离为空 → 显示全部
	for _, in := range []string{"not", "NOT", "and", "or"} {
		a.searchInput = in
		q := a.currentQuery()
		if !q.IsEmpty() {
			t.Errorf("input %q: 纯操作符应剥离为空", in)
		}
		if !q.MatchLine(line) {
			t.Errorf("input %q: 剥离空应匹配全部", in)
		}
	}
	// "ERROR not" → 剥离末尾 not → "ERROR" → 搜 message 含 ERROR（hello world 不含）
	a.searchInput = "ERROR not"
	if a.currentQuery().MatchLine(line) {
		t.Error(`"ERROR not" 应剥离为搜 ERROR，hello world 不含`)
	}
	// 非中间态正常搜
	a.searchInput = "hello"
	if a.currentQuery().IsEmpty() {
		t.Error("非中间态应正常搜")
	}
}

func TestStrippedQuery(t *testing.T) {
	cases := map[string]string{
		"not": "",
		"NOT": "",
		"and": "",
		"or": "",
		"ERROR not": "ERROR",
		"error and": "error",
		"ERROR OR": "ERROR",
		"ERROR timeout": "ERROR timeout",
		"n": "n",
		"no": "no",
	}
	for in, want := range cases {
		if got := strippedQuery(in); got != want {
			t.Errorf("strippedQuery(%q)=%q, want %q", in, got, want)
		}
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
