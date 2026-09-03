package tui

// time: 语法改造(v2)回归审计测试。
// 背景:timeTerm/errorTerm 取代 timeAfterTerm/timeBeforeTerm,fieldPrefixes 移除
// after:/before: 加入 time:,parser 全链路带 anchor。本文件验证既有搜索语法未被破坏:
// 布尔组合、字段匹配、高亮产词、中间态判定、tokenizer 字面量。
// 已知实现缺陷(NULL 传播仅一层、time: 悬空中间态空集)不在本文件断言,见审计报告。

import (
	"testing"
	"time"
)

// --- 1. 布尔组合完整性:(a OR b) AND (c OR d) 及其变体 ---

type comboRow struct {
	msg  string
	want bool
}

type comboCase struct {
	name  string
	query string
	rows  []comboRow
}

func runComboCases(t *testing.T, cases []comboCase) {
	t.Helper()
	for _, tc := range cases {
		q := parseSearchQuery(tc.query)
		if q.ParseError() != "" {
			t.Errorf("[%s] %q: 意外解析错误: %s", tc.name, tc.query, q.ParseError())
			continue
		}
		for _, r := range tc.rows {
			got := q.MatchLine(parsedLine("INFO", "", "", "", r.msg))
			if got != r.want {
				t.Errorf("[%s] %q vs message %q = %v, want %v", tc.name, tc.query, r.msg, got, r.want)
			}
		}
	}
}

func TestComboOrAndMatrix(t *testing.T) {
	runComboCases(t, []comboCase{
		{
			name:  "标准双括号OR-AND",
			query: "(x OR y) AND (z OR w)",
			rows: []comboRow{
				{"x z", true}, {"y w", true}, {"x", false}, {"z", false},
				{"x y z w", true}, {"", false}, {"x q z", true},
			},
		},
		{
			name:  "括号组隐式AND",
			query: "(x OR y) (z OR w)",
			rows: []comboRow{
				{"x z", true}, {"y w", true}, {"x", false}, {"z", false},
				{"x y z w", true}, {"", false},
			},
		},
		{
			name:  "括号优先于OR-右OR裸词",
			query: "(x OR y) AND z OR w", // ((x|y)&z)|w
			rows: []comboRow{
				{"x z", true}, {"y w", true}, {"x", false}, {"z", false},
				{"w", true}, {"x y z w", true}, {"", false},
			},
		},
		{
			name:  "AND优先于OR-无括号对照",
			query: "x OR y AND z OR w", // x|(y&z)|w
			rows: []comboRow{
				{"x z", true}, {"y w", true}, {"x", true}, {"z", false},
				{"w", true}, {"y", false}, {"y z", true}, {"", false},
			},
		},
		{
			name:  "左组三分支",
			query: "(x OR y OR q) AND (z OR w)",
			rows: []comboRow{
				{"x z", true}, {"q w", true}, {"y", false}, {"x z q", true}, {"q", false},
			},
		},
		{
			name:  "三组AND",
			query: "(x OR y) AND (z OR w) AND v",
			rows: []comboRow{
				{"x z v", true}, {"x z", false}, {"x v", false}, {"y w v", true},
			},
		},
		{
			name:  "NOT左组",
			query: "NOT (x OR y) AND (z OR w)",
			rows: []comboRow{
				{"q z", true}, {"x z", false}, {"y w", false}, {"z", true}, {"q", false},
			},
		},
		{
			name:  "NOT右组",
			query: "(x OR y) AND NOT (z OR w)",
			rows: []comboRow{
				{"x", true}, {"x z", false}, {"y w", false}, {"", false}, {"x q", true},
			},
		},
		{
			name:  "NOT在组内左支",
			query: "(NOT x OR y) AND (z OR w)",
			rows: []comboRow{
				{"x z", false}, {"y z", true}, {"q z", true}, {"q", false}, {"y w", true}, {"", false},
			},
		},
		{
			name:  "整体括号再OR",
			query: "((x OR y) AND (z OR w)) OR v",
			rows: []comboRow{
				{"x z", true}, {"v", true}, {"", false}, {"q z", false}, {"q v", true},
			},
		},
		{
			name:  "AND组OR对照形态",
			query: "(x AND q) OR (z AND w)",
			rows: []comboRow{
				{"x q", true}, {"z w", true}, {"x", false}, {"z", false},
				{"x z", false}, {"x q z w", true}, {"", false},
			},
		},
		{
			name:  "field混合双括号",
			query: "(level:ERROR OR level:WARN) AND (timeout OR retry)",
			rows:  []comboRow{
				// level 走 EqualFold、message 走 containsIgnoreCase(大小写混合验证)
			},
		},
		{
			name:  "嵌套括号",
			query: "a AND (b OR (c AND d))",
			rows: []comboRow{
				{"a b", true}, {"a c d", true}, {"a", false}, {"a b c d", true},
				{"b c d", false}, {"a c", false},
			},
		},
		{
			name:  "冗余双层括号",
			query: "((a))",
			rows: []comboRow{
				{"a", true}, {"b", false},
			},
		},
		{
			name:  "小写操作符",
			query: "(x or y) and (z or w)",
			rows: []comboRow{
				{"x z", true}, {"x", false},
			},
		},
		{
			name:  "隐式与显式AND混用",
			query: "a b AND c (d OR e)",
			rows: []comboRow{
				{"a b c d", true}, {"a b c e", true}, {"a c d", false}, {"a b d", false}, {"a b c", false},
			},
		},
	})
}

// field 混合双括号的真值表(与 runComboCases 分开:行需构造 level)。
func TestComboFieldOrAndMatrix(t *testing.T) {
	q := parseSearchQuery("(level:ERROR OR level:WARN) AND (timeout OR retry)")
	rows := []struct {
		level, msg string
		want       bool
	}{
		{"ERROR", "timeout", true},
		{"ERROR", "retry", true},
		{"WARN", "timeout x", true},
		{"WARN", "retry y", true},
		{"ERROR", "ok", false},
		{"WARN", "ok", false},
		{"INFO", "timeout", false},
		{"INFO", "ok", false},
		{"error", "TIMEOUT", true}, // level EqualFold + message containsIgnoreCase
		{"ERRORS", "timeout", false},
		{"INFO", "TIMEOUT occurred", false},
	}
	for _, r := range rows {
		got := q.MatchLine(parsedLine(r.level, "", "", "", r.msg))
		if got != r.want {
			t.Errorf("(level:ERROR OR level:WARN) AND (timeout OR retry) vs level=%s msg=%q = %v, want %v",
				r.level, r.msg, got, r.want)
		}
	}
}

// NOT 链(非 time):双 NOT 等价原条件。
func TestComboNotChain(t *testing.T) {
	runComboCases(t, []comboCase{
		{
			name:  "双NOT等价",
			query: "NOT NOT x",
			rows: []comboRow{
				{"x here", true}, {"y here", false},
			},
		},
		{
			name:  "三NOT取反",
			query: "NOT NOT NOT x",
			rows: []comboRow{
				{"x here", false}, {"y here", true},
			},
		},
		{
			name:  "NOT链与括号组合",
			query: "NOT (x AND y) OR z",
			rows: []comboRow{
				{"x y", false}, {"x", true}, {"z", true}, {"", true},
			},
		},
	})
}

// --- 2. 字段匹配语义 ---

func TestFieldSemanticsRegression(t *testing.T) {
	cases := []struct {
		name  string
		query string
		level string
		want  bool
	}{
		{"level精确大小写不敏感", "level:ERROR", "error", true},
		{"level混合大小写", "level:eRrOr", "ERROR", true},
		{"level精确不包含", "level:ERROR", "ERRORX", false},
		{"level前缀不算匹配", "level:ERR", "ERROR", false},
		{"level大小写混合查询", "level:WaRn", "wArN", true},
	}
	for _, tc := range cases {
		q := parseSearchQuery(tc.query)
		got := q.MatchLine(parsedLine(tc.level, "", "", "", "m"))
		if got != tc.want {
			t.Errorf("[%s] %q vs level=%s = %v, want %v", tc.name, tc.query, tc.level, got, tc.want)
		}
	}

	// 非关键字段 containsIgnoreCase(thread/traceId/logger/message/source)
	q := parseSearchQuery("thread:EXEC")
	if !q.MatchLine(parsedLine("INFO", "", "worker-exec-3", "", "m")) {
		t.Error("thread:EXEC 应包含匹配 worker-exec-3(大小写不敏感)")
	}
	q = parseSearchQuery("traceId:abc")
	if !q.MatchLine(parsedLine("INFO", "xyzabc789", "", "", "m")) {
		t.Error("traceId:abc 应包含匹配 xyzabc789")
	}
	q = parseSearchQuery("logger:example.App")
	if !q.MatchLine(parsedLine("INFO", "", "", "com.example.Application", "m")) {
		t.Error("logger:example.App 应包含匹配 com.example.Application")
	}
	pl := parsedLine("INFO", "", "", "", "m")
	pl.Raw.Source = "pod/api-7d8f-x9k2j"
	if !parseSearchQuery("source:api-7d8f").MatchLine(pl) {
		t.Error("source:ai-7d8f 应包含匹配 pod/api-7d8f-x9k2j")
	}
	if parseSearchQuery("source:nomatch").MatchLine(pl) {
		t.Error("source:nomatch 不应匹配")
	}

	// 字段缺失 → false
	if parseSearchQuery("traceId:abc").MatchLine(parsedLine("INFO", "", "", "", "abc in message")) {
		t.Error("traceId:abc 不应命中 message(字段缺失为 false,不回落 message)")
	}
}

func TestFieldQuotedAndSwallowRegression(t *testing.T) {
	// field:"带空格" 引号值
	q := parseSearchQuery(`message:"hello world"`)
	if !q.MatchLine(parsedLine("INFO", "", "", "", "say hello world now")) {
		t.Error(`message:"hello world" 应匹配含连续短语的行`)
	}
	if q.MatchLine(parsedLine("INFO", "", "", "", "hello there world")) {
		t.Error(`message:"hello world" 不应匹配被拆开的短语`)
	}
	// field: 空格吞词(不带引号)
	q2 := parseSearchQuery("message: hello")
	if !q2.MatchLine(parsedLine("INFO", "", "", "", "HELLO there")) {
		t.Error("message: hello(空格) 应吞词为值并大小写不敏感匹配")
	}
	// field: AND 不吞操作符(空值 message 匹配 message 非空行)
	q3 := parseSearchQuery("message: AND world")
	if !q3.MatchLine(parsedLine("INFO", "", "", "", "hello world")) {
		t.Error("message: AND world 应匹配含 world 的行(AND 不被吞)")
	}
	if q3.MatchLine(parsedLine("INFO", "", "", "", "hello")) {
		t.Error("message: AND world 不应匹配无 world 的行")
	}
	// field: 后跟括号不吞
	q4 := parseSearchQuery("message: (hello OR world)")
	if !q4.MatchLine(parsedLine("INFO", "", "", "", "hello")) {
		t.Error("message: (hello OR world) 中括号组应独立参与 AND")
	}
	// level: 空值 → EqualFold(val,"") 恒 false(level 非空时)
	if parseSearchQuery("level: AND world").MatchLine(parsedLine("ERROR", "", "", "", "world")) {
		t.Error("level: 空值对非空 level 行应恒 false")
	}
}

// --- 3. 高亮关键词提取 ---

func TestHighlightKeywordsRegression(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{"time条件不产词", "time:>09:00 AND level:ERROR", []string{"ERROR"}},
		{"time区间不产词", "time:10:00..11:00 OR timeout", []string{"timeout"}},
		{"NOT子树不产词-括号", "NOT (a OR b) AND c", []string{"c"}},
		{"NOT子树不产词-简单", "a AND NOT b", []string{"a"}},
		{"纯NOT查询无词", "NOT level:DEBUG", nil},
		{"引号keyword产词", `"quoted phrase" AND level:WARN`, []string{"quoted phrase", "WARN"}},
		{"引号field值产词", `message:"a b" OR z`, []string{"a b", "z"}},
		{"OR两侧都产词", "timeout OR level:WARN", []string{"timeout", "WARN"}},
	}
	for _, tc := range cases {
		anchor := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
		q := parseSearchQueryAt(tc.query, anchor)
		got := q.HighlightKeywords()
		if len(got) != len(tc.want) {
			t.Errorf("[%s] %q keywords = %v, want %v", tc.name, tc.query, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("[%s] %q keywords[%d] = %q, want %q", tc.name, tc.query, i, got[i], tc.want[i])
			}
		}
	}
	// errorTerm 不产词,同查询其余词照常产
	q := parseSearchQuery("level:ERROR AND time:>xx")
	if kw := q.HighlightKeywords(); len(kw) != 1 || kw[0] != "ERROR" {
		t.Errorf("errorTerm 应不产词, got %v", kw)
	}
	if q.ParseError() == "" {
		t.Error("time:>xx 应产生解析错误")
	}
}

// --- 4. isPartialQuery / strippedQuery ---

func TestIsPartialQueryRegression(t *testing.T) {
	trues := []string{
		// 原有中间态语义保留
		"and", "AND", "Or", "not", "NOT",
		"ERROR and", "a or", "level:ERROR NOT",
		"(", "(ERROR", "(ERROR OR WARN", "(a OR (b",
		`"hello`, `message:"a b`,
		// time: 悬空(值解析失败但只含合法字符 → 仍在输入)
		"time:", "time:>", "time:>09", "time:>09:0",
		"time:>2026-08-2", "x AND time:>-1", "time:10:00..",
	}
	falses := []string{
		// 原有完整查询
		"", "ERROR", "ERROR and timeout", "n", "no",
		"(ERROR)", "(ERROR OR WARN)", `"hello world"`, `message:"a b"`,
		"after:09:00", "before:10:00", "level:ERROR",
		// time: 完整合法值不是中间态
		"time:>09:00", "time:>=15:00:30", "time:<10:00:30",
		"time:>-10m", "time:-1h", "time:>-1h30m",
		"time:>2026-08-26", "time:>2026-08-26T09:00:30", "time:>=2026-08-26_09:00:30",
		"time:10:00..11:00", "time:9:00..10:00",
		// 非法字符 → 确定错误(红字提示路径),非中间态。
		// 设计取舍:时长单位 h/m/s/d 在合法字符白名单内(修复中途闪错),
		// "time:-1h3"(想敲 -1h30m 的中间帧)判中间态;含其它字母才是确定错误。
		"time:>abc", "time:>-10x",
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

func TestStrippedQueryRegression(t *testing.T) {
	cases := map[string]string{
		// 原有剥离行为保留
		"not": "", "NOT": "", "and": "", "or": "",
		"ERROR not": "ERROR", "error and": "error", "ERROR OR": "ERROR",
		"ERROR timeout": "ERROR timeout",
		"n":             "n", "no": "no",
		"(a OR b":   "(a OR b", // 括号中间态不剥离(由调用方保持上次结果)
		`"unclosed`: `"unclosed`,
	}
	for in, want := range cases {
		if got := strippedQuery(in); got != want {
			t.Errorf("strippedQuery(%q)=%q, want %q", in, got, want)
		}
	}
}

// --- 5. tokenizer 回归:裸字面量不被劫持 ---

func TestTokenizerLiteralRegression(t *testing.T) {
	type tk struct {
		kind  tokenKind
		field string
		value string
	}
	cases := []struct {
		name  string
		input string
		want  []tk
	}{
		{"裸大于时间", ">09:00", []tk{{tokKeyword, "", ">09:00"}}},
		{"裸Java初始化线程", "<init>", []tk{{tokKeyword, "", "<init>"}}},
		{"裸完整时间戳尖括号", "<2026-08-26T09:00>", []tk{{tokKeyword, "", "<2026-08-26T09:00>"}}},
		{"after前缀已移除", "after:09:00", []tk{{tokKeyword, "", "after:09:00"}}},
		{"before前缀已移除", "before:10:00", []tk{{tokKeyword, "", "before:10:00"}}},
		{"字面量与操作符混排", "( >09:00 OR <init> )", []tk{
			{tokLParen, "", ""}, {tokKeyword, "", ">09:00"}, {tokOr, "", "OR"},
			{tokKeyword, "", "<init>"}, {tokRParen, "", ""},
		}},
		{"time引号值", `time:">09:00"`, []tk{{tokField, "time", ">09:00"}}},
		{"time空格值", "time: >09:00", []tk{{tokField, "time", ">09:00"}}},
		{"message引号带空格", `message:"a b"`, []tk{{tokField, "message", "a b"}}},
		{"level空格吞词", "level: ERROR", []tk{{tokField, "level", "ERROR"}}},
		{"AND不被吞为值", "level: AND x", []tk{
			{tokField, "level", ""}, {tokAnd, "", "AND"}, {tokKeyword, "", "x"},
		}},
		{"括号不被吞为值", "message:(x)", []tk{
			{tokField, "message", ""}, {tokLParen, "", ""}, {tokKeyword, "", "x"}, {tokRParen, "", ""},
		}},
	}
	for _, tc := range cases {
		toks := tokenize(tc.input)
		if len(toks) != len(tc.want) {
			t.Errorf("[%s] tokenize(%q) 得 %d 个 token, want %d", tc.name, tc.input, len(toks), len(tc.want))
			continue
		}
		for i, w := range tc.want {
			if toks[i].kind != w.kind || toks[i].field != w.field || toks[i].value != w.value {
				t.Errorf("[%s] tokenize(%q)[%d] = {kind=%d field=%q value=%q}, want {kind=%d field=%q value=%q}",
					tc.name, tc.input, i, toks[i].kind, toks[i].field, toks[i].value, w.kind, w.field, w.value)
			}
		}
	}
}

// 字面量在 parse 层不被劫持:作为 keyword 搜索 message,不报错,且能参与组合。
func TestLiteralKeywordParseRegression(t *testing.T) {
	anchor := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	for _, lit := range []string{">09:00", "<init>", "<2026-08-26T09:00>", "after:09:00", "before:10:00"} {
		q := parseSearchQueryAt(lit, anchor)
		if q.ParseError() != "" {
			t.Errorf("%q 不应产生解析错误: %s", lit, q.ParseError())
		}
		if !q.MatchLine(parsedLine("INFO", "", "", "", "seen "+lit+" here")) {
			t.Errorf("%q 应作为字面量 keyword 命中 message", lit)
		}
		if q.MatchLine(parsedLine("INFO", "", "", "", "nothing here")) {
			t.Errorf("%q 不应命中不含它的行", lit)
		}
	}
	// 字面量混入布尔组合仍按 keyword 参与运算
	q := parseSearchQueryAt("<init> AND level:ERROR", anchor)
	if !q.MatchLine(parsedLine("ERROR", "", "main", "", "in <init> thread")) {
		t.Error("<init> AND level:ERROR 应命中 ERROR 行 message 含 <init>")
	}
	if q.MatchLine(parsedLine("INFO", "", "main", "", "in <init> thread")) {
		t.Error("<init> AND level:ERROR 不应命中 INFO 行")
	}
	// after: 带引号值也按普通 keyword 处理(不进时间语义)
	q2 := parseSearchQueryAt(`after:"09:00"`, anchor)
	if q2.ParseError() != "" || !q2.MatchLine(parsedLine("INFO", "", "", "", "ran after:09:00 sharp")) {
		t.Error(`after:"09:00" 应按普通 keyword 处理`)
	}
}

// --- 6. time: 与布尔组合(锚定固定时刻,含 README 承诺形态) ---

// timeRow:time 组合真值表行;t 为零值表示该行无时间字段。
type timeRow struct {
	level string
	msg   string
	t     time.Time
	want  bool
}

func TestTimeBooleanCompositionRegression(t *testing.T) {
	anchor := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	at := func(h, m int) time.Time { return time.Date(2026, 5, 15, h, m, 0, 0, time.UTC) }

	tRun := func(t *testing.T, name, query string, rows []timeRow) {
		t.Helper()
		q := parseSearchQueryAt(query, anchor)
		if q.ParseError() != "" {
			t.Errorf("[%s] %q 意外解析错误: %s", name, query, q.ParseError())
			return
		}
		for _, r := range rows {
			pl := parsedLine(r.level, "", "", "", r.msg)
			if !r.t.IsZero() {
				pl.Time = r.t
			}
			if got := q.MatchLine(pl); got != r.want {
				t.Errorf("[%s] %q vs (level=%s msg=%q time=%v) = %v, want %v",
					name, query, r.level, r.msg, r.t, got, r.want)
			}
		}
	}

	// README:多时段 time:10:00..11:00 OR time:13:00..14:00
	tRun(t, "多时段OR", "time:10:00..11:00 OR time:13:00..14:00", []timeRow{
		{"INFO", "x", at(10, 30), true},
		{"INFO", "x", at(10, 0), true}, // 闭区间含端
		{"INFO", "x", at(14, 0), true}, // 闭区间含端
		{"INFO", "x", at(11, 30), false},
		{"INFO", "x", at(12, 0), false},
		{"INFO", "x", at(13, 30), true},
	})

	// README:与其它条件自由组合 level:ERROR AND time:>-30s
	tRun(t, "相对时间AND字段", "level:ERROR AND time:>-30s", []timeRow{
		{"ERROR", "x", anchor.Add(-10 * time.Second), true},
		{"ERROR", "x", anchor.Add(-time.Minute), false},
		{"INFO", "x", anchor.Add(-10 * time.Second), false},
		{"ERROR", "x", time.Time{}, false}, // 无时间行不参与
	})

	// time 组进括号组参与布尔
	tRun(t, "time在OR组内", "(x OR y) AND (time:>09:00 OR time:<08:00)", []timeRow{
		{"INFO", "x", at(9, 30), true},
		{"INFO", "y", at(7, 30), true},
		{"INFO", "x", at(8, 30), false}, // 既不 >09:00 也不 <08:00
		{"INFO", "x", time.Time{}, false},
		{"INFO", "z", at(9, 30), false},
	})

	// README:NOT time: 不复活无时间行(NULL 语义)
	tRun(t, "NOT-time-NULL传播", "NOT time:>09:00", []timeRow{
		{"INFO", "x", at(8, 0), true},
		{"INFO", "x", at(9, 30), false},
		{"INFO", "x", time.Time{}, false},
	})

	// 括号包裹的 time: 在 NOT 下同样不复活(child 折叠为 termNode)
	tRun(t, "NOT-括号-time-NULL传播", "NOT (time:>09:00)", []timeRow{
		{"INFO", "x", at(8, 0), true},
		{"INFO", "x", at(9, 30), false},
		{"INFO", "x", time.Time{}, false},
	})

	// time 窗口 AND 字段 AND 关键词三重组合
	tRun(t, "time+字段+关键词", "time:>09:00 AND time:<10:00 AND level:ERROR timeout", []timeRow{
		{"ERROR", "timeout hit", at(9, 30), true},
		{"ERROR", "timeout hit", at(10, 30), false},
		{"INFO", "timeout hit", at(9, 30), false},
		{"ERROR", "all good", at(9, 30), false},
	})
}
