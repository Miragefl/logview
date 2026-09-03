package tui

// time: 时间范围搜索 — 行为规格测试（矩阵第一、三节）。
// 用例来源:docs/superpowers/plans/2026-08-28-time-range-test-matrix.md
// 翻译约定:anchor 注入固定时刻 A0 = 2026-08-28T10:00:00Z；行时间一律
// time.Date(..., time.UTC) 构造；无时间字段行用 parsedLine（Time IsZero）。
//
// 裁决落地（与矩阵正文差异处已在对应用例旁注明）:
//   - H4/H7:实现采用分支级 NULL 传播（OR 其它分支可救活 IsZero 行；
//     notNode 特判仅限直接 time 型子节点），按矩阵裁决注记的该分支落地。
//   - E11:矩阵行写 level=WARN 与其断言要点"OR 右支救活"自相矛盾
//     （WARN 不命中 level:ERROR，右支无法救活），判定矩阵笔误，修正为 level=ERROR。
//   - I2:实现 hint 排版为 "01-02 15:04:05"（无年份，现有 TestTimeRangeHint
//     已定案），矩阵翻译约定明示"具体排版以实现为准"，故断言子串不含年份。
//   - I3:错误红字经 ParseError() 通道渲染（searchbar.go timeFeedback），
//     TimeRangeHint() 错误时返回空，故按双通道断言。
//   - 第三节 fixture:矩阵样本行缺 [thread] [traceId] logger 结构，无法命中
//     生产 java-logback 规则（会落 plain-text，Time 全零），修正为完整
//     logback 结构，时间与 message 语义不变。

import (
	"strings"
	"testing"
	"time"

	"github.com/justfun/logview/internal/model"
	"github.com/justfun/logview/internal/parser"
)

// anchorA0 矩阵默认锚点:A0 = 2026-08-28T10:00:00Z。
var anchorA0 = time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)

// utc 构造 UTC 时刻的简写（矩阵行记法 "08-28 09:30" → utc(8,28,9,30,0)）。
func utc(month time.Month, day, hour, min, sec int) time.Time {
	return time.Date(2026, month, day, hour, min, sec, 0, time.UTC)
}

// timeTC 矩阵通用表骨架:query + 行 + 期望布尔。
type timeTC struct {
	query string
	line  *model.ParsedLine
	want  bool
}

// runTimeTC 表驱动执行:anchor 注入 A0。
func runTimeTC(t *testing.T, name string, cases []timeTC) {
	t.Helper()
	for i, c := range cases {
		q := parseSearchQueryAt(c.query, anchorA0)
		if got := q.MatchLine(c.line); got != c.want {
			t.Errorf("%s[%d] %q 期望 %v, 实际 %v", name, i+1, c.query, c.want, got)
		}
	}
}

// tl 带时刻的普通行（level=INFO，message 任意）。
func tl(t time.Time) *model.ParsedLine {
	return parsedLineWithTime("INFO", "", "", "", "msg", t)
}

// tlLevel 带时刻与 level 的行。
func tlLevel(level string, t time.Time) *model.ParsedLine {
	return parsedLineWithTime(level, "", "", "", "msg", t)
}

// --- 1.1 值形态 × 方向符（A1-A23，23 条）---

func TestTimeValueFormsAndOps(t *testing.T) {
	runTimeTC(t, "A", []timeTC{
		// A1/A2 前导零宽容:9:00 ≡ 09:00
		{"time:>9:00", tl(utc(8, 28, 9, 30, 0)), true},
		{"time:>09:00", tl(utc(8, 28, 9, 30, 0)), true},
		// A3/A4 > 严格，恰等不含
		{"time:>9:00", tl(utc(8, 28, 9, 0, 0)), false},
		{"time:>09:00", tl(utc(8, 28, 9, 0, 0)), false},
		// A5/A6 >= 含边界，叠加前导零宽容
		{"time:>=9:00", tl(utc(8, 28, 9, 0, 0)), true},
		{"time:>=09:00", tl(utc(8, 28, 9, 0, 0)), true},
		// A7-A9 </<= 边界
		{"time:<10:00", tl(utc(8, 28, 9, 59, 0)), true},
		{"time:<10:00", tl(utc(8, 28, 10, 0, 0)), false},
		{"time:<=10:00", tl(utc(8, 28, 10, 0, 0)), true},
		// A10-A12 秒粒度
		{"time:<10:00:30", tl(utc(8, 28, 10, 0, 29)), true},
		{"time:<10:00:30", tl(utc(8, 28, 10, 0, 30)), false},
		{"time:<=10:00:30", tl(utc(8, 28, 10, 0, 30)), true},
		// A13/A14 时分秒形态
		{"time:>09:00:30", tl(utc(8, 28, 9, 0, 31)), true},
		{"time:>09:00:30", tl(utc(8, 28, 9, 0, 30)), false},
		// A15-A18 纯日期 → 当天 00:00:00
		{"time:>2026-08-26", tl(utc(8, 26, 8, 0, 0)), true},
		{"time:>2026-08-26", tl(utc(8, 26, 0, 0, 0)), false},
		{"time:>=2026-08-26", tl(utc(8, 26, 0, 0, 0)), true},
		{"time:>2026-08-26", tl(utc(8, 25, 23, 59, 0)), false},
		// A19/A20 日期+时分（分精度秒补 0）
		{"time:>2026-08-26T09:00", tl(utc(8, 26, 9, 0, 30)), true},
		{"time:>2026-08-26T09:00", tl(utc(8, 26, 9, 0, 0)), false},
		// A21 完整形态 + >=
		{"time:>=2026-08-26T09:00:30", tl(utc(8, 26, 9, 0, 30)), true},
		// A22/A23 _ 别名 ≡ T
		{"time:>2026-08-26_09:00:30", tl(utc(8, 26, 9, 0, 31)), true},
		{"time:>2026-08-26_09:00:30", tl(utc(8, 26, 9, 0, 29)), false},
	})
}

// --- 1.2 相对时长（B1-B12 匹配 + B13 节点断言，13 条）---

func TestTimeRelativeDurations(t *testing.T) {
	runTimeTC(t, "B", []timeTC{
		// B1-B3 锚点 = A0-10m = 09:50
		{"time:>-10m", tl(utc(8, 28, 9, 51, 0)), true},
		{"time:>-10m", tl(utc(8, 28, 9, 50, 0)), false},
		{"time:>=-10m", tl(utc(8, 28, 9, 50, 0)), true},
		// B4/B5 简写 time:-10m ≡ time:>-10m（默认 >，非 >=）
		{"time:-10m", tl(utc(8, 28, 9, 55, 0)), true},
		{"time:-10m", tl(utc(8, 28, 9, 45, 0)), false},
		// B6/B7 d 单位；锚点 = 08-26 10:00
		{"time:>-2d", tl(utc(8, 26, 10, 0, 1)), true},
		{"time:>-2d", tl(utc(8, 26, 9, 59, 0)), false},
		// B8-B10 复合时长 1h30m ≡ 90m；锚点 = 08:30
		{"time:>-1h30m", tl(utc(8, 28, 8, 31, 0)), true},
		{"time:>-90m", tl(utc(8, 28, 8, 31, 0)), true},
		{"time:>-90m", tl(utc(8, 28, 8, 29, 0)), false},
		// B11/B12 s 单位；锚点 = 09:59:30
		{"time:>-30s", tl(utc(8, 28, 9, 59, 31)), true},
		{"time:>-30s", tl(utc(8, 28, 9, 59, 30)), false},
	})
	// B13 anchor 减法正确性:解析产物的绝对时刻 == A0.Add(-90m) = 08:30:00Z
	q := parseSearchQueryAt("time:>-90m", anchorA0)
	tn, ok := q.root.(*termNode)
	if !ok {
		t.Fatalf("B13: 根节点应为 *termNode, 实际 %T", q.root)
	}
	want := anchorA0.Add(-90 * time.Minute)
	if tn.time == nil || !tn.time.Equal(want) {
		t.Errorf("B13: 锚点应为 %v, 实际 %v", want, tn.time)
	}
	// B13 附:hint 同步显示锚点时刻(展示跟随 anchor 的 location,时区无关断言)
	wantHint := anchorA0.Add(-90 * time.Minute).Format("15:04:05")
	if h := q.TimeRangeHint(); !strings.Contains(h, wantHint) {
		t.Errorf("B13: hint 应含 %s, 实际 %q", wantHint, h)
	}
}

// --- 1.3 区间糖（C1-C12，12 条）---

func TestTimeRangeSugarClosedInterval(t *testing.T) {
	runTimeTC(t, "C", []timeTC{
		// C1-C5 闭区间:>=下界 AND <=上界，端点含
		{"time:10:00..11:00", tl(utc(8, 28, 10, 0, 0)), true},
		{"time:10:00..11:00", tl(utc(8, 28, 11, 0, 0)), true},
		{"time:10:00..11:00", tl(utc(8, 28, 10, 30, 0)), true},
		{"time:10:00..11:00", tl(utc(8, 28, 9, 59, 59)), false},
		{"time:10:00..11:00", tl(utc(8, 28, 11, 0, 1)), false},
		// C6-C8 带日期区间（不跨日）
		{"time:2026-08-26T09:00..2026-08-26T11:00", tl(utc(8, 26, 10, 0, 0)), true},
		{"time:2026-08-26T09:00..2026-08-26T11:00", tl(utc(8, 26, 8, 59, 0)), false},
		{"time:2026-08-26T09:00..2026-08-26T11:00", tl(utc(8, 27, 10, 0, 0)), false},
		// C9-C11 日期区间:上界补全为 08-27 00:00:00
		{"time:2026-08-26..2026-08-27", tl(utc(8, 26, 15, 0, 0)), true},
		{"time:2026-08-26..2026-08-27", tl(utc(8, 27, 0, 0, 0)), true},
		{"time:2026-08-26..2026-08-27", tl(utc(8, 27, 23, 59, 0)), false},
		// C12 混合粒度:左纯时分锚 A0 当天，右带日期，两侧各自补全
		{"time:09:00..2026-08-29T10:00", tl(utc(8, 28, 9, 30, 0)), true},
	})
}

// --- 1.4 显式双条件 AND = 开区间（D1-D3，3 条）---

func TestTimeExplicitRangeOpenInterval(t *testing.T) {
	runTimeTC(t, "D", []timeTC{
		{"time:>09:00 time:<10:00", tl(utc(8, 28, 9, 30, 0)), true},
		// 开区间端点不含（对照区间糖 C1/C2 闭端点含）
		{"time:>09:00 time:<10:00", tl(utc(8, 28, 9, 0, 0)), false},
		{"time:>09:00 time:<10:00", tl(utc(8, 28, 10, 0, 0)), false},
	})
}

// --- 1.5 多时段与正交组合（E1-E11，11 条）---

func TestTimeMultiSegmentAndOrthogonal(t *testing.T) {
	runTimeTC(t, "E", []timeTC{
		// E1-E5 双时段 OR
		{"time:10:00..11:00 OR time:13:00..14:00", tl(utc(8, 28, 10, 30, 0)), true},
		{"time:10:00..11:00 OR time:13:00..14:00", tl(utc(8, 28, 13, 30, 0)), true},
		{"time:10:00..11:00 OR time:13:00..14:00", tl(utc(8, 28, 12, 0, 0)), false},
		{"time:10:00..11:00 OR time:13:00..14:00", tl(utc(8, 28, 11, 0, 0)), true},
		{"time:10:00..11:00 OR time:13:00..14:00", tl(utc(8, 28, 14, 0, 0)), true},
		// E6-E8 time: 与字段语法正交组合（锚点 = 09:59:30）
		{"level:ERROR AND time:>-30s", tlLevel("ERROR", utc(8, 28, 9, 59, 31)), true},
		{"level:ERROR AND time:>-30s", tlLevel("INFO", utc(8, 28, 9, 59, 31)), false},
		{"level:ERROR AND time:>-30s", tlLevel("ERROR", utc(8, 28, 9, 0, 0)), false},
		// E9/E10 有时间行的 NOT 常规取反
		{"NOT time:>09:00", tl(utc(8, 28, 8, 30, 0)), true},
		{"NOT time:>09:00", tl(utc(8, 28, 9, 30, 0)), false},
		// E11 OR 右支救活。注:矩阵原文 level=WARN 与其断言要点"OR 右支救活"
		// 自相矛盾（WARN 不命中 level:ERROR），判定矩阵笔误，修正为 level=ERROR。
		{"time:>09:00 OR level:ERROR", tlLevel("ERROR", utc(8, 28, 8, 0, 0)), true},
	})
}

// --- 1.6 非法输入 → errorNode（F1-F16，16 条）---
// 断言三件套:① 不 panic 且 ParseError 非空；② 空集（任意行 false）；
// ③ 不降级为关键字（message 恰含错误子串的行也不命中）。

func TestTimeParseErrors(t *testing.T) {
	normalLine := tl(utc(8, 28, 9, 30, 0))
	cases := []struct {
		query     string
		degradeMsg string // 防降级行的 message（空则只验普通行）
		checkLine *model.ParsedLine
	}{
		{"time:09:00", "job at 09:00 started", nil},          // F1 缺方向符
		{"time:>8-26", "backup 8-26 done", nil},               // F2 MM-DD 已删
		{"time:>9:0", "delay 9:0 ms", nil},                    // F3 分钟须两位
		{"time:>", "", nil},                                   // F4 方向符后空值
		{"time:>10:00..", "range 10:00.. open", nil},          // F5 方向符后区间缺右值
		{"time:10:00..", "range 10:00.. open", nil},           // F6 区间缺右值
		{"time:-10x", "took -10x retries", nil},               // F7 非法单位
		{"time:>+10m", "shift +10m later", nil},               // F8 正号拒绝
		{"time:>-10", "delta -10 units", nil},                 // F9 纯数字缺单位
		{"time:>09:00:3", "clock 09:00:3 weird", nil},         // F10 秒一位
		{"time:>2026-8-26", "on 2026-8-26 note", nil},         // F11 月份须两位
		{"time:>24:00", "open 24:00 hours", nil},              // F12 小时越界（防归一化）
		{"time:>09:60", "leap 09:60 min", nil},                // F13 分钟越界
		{"time:>xx OR level:ERROR", "", parsedLine("ERROR", "", "", "", "boom")},           // F14
		{"NOT time:>xx", "", tl(utc(8, 28, 8, 30, 0))},                                      // F15
		{"level:ERROR AND time:>xx", "", parsedLine("ERROR", "", "", "", "boom")},           // F16
	}
	for i, c := range cases {
		q := parseSearchQueryAt(c.query, anchorA0) // ① 不 panic
		if q.ParseError() == "" {
			t.Errorf("F%d %q 应产生解析错误", i+1, c.query)
		}
		if q.MatchLine(normalLine) { // ② 空集
			t.Errorf("F%d %q 解析错误时应对任意行不命中", i+1, c.query)
		}
		if c.degradeMsg != "" { // ③ 不降级为关键字
			if q.MatchLine(parsedLine("INFO", "", "", "", c.degradeMsg)) {
				t.Errorf("F%d %q 不应降级为关键字（message 含 %q 也不命中）", i+1, c.query, c.degradeMsg)
			}
		}
		if c.checkLine != nil && q.MatchLine(c.checkLine) {
			t.Errorf("F%d %q 错误应整体传播为空集（OR/AND/NOT 均不救活）", i+1, c.query)
		}
	}
}

// --- 1.7 跨天锚定（G1-G6，6 条）---
// 纯时分只锚 anchor 当天，比较统一为绝对时刻（非每日窗口）。

func TestTimeCrossDayAnchoring(t *testing.T) {
	runTimeTC(t, "G", []timeTC{
		{"time:>09:00", tl(utc(8, 27, 23, 50, 0)), false}, // G1 昨天 23:50 不算今天 09:00 之后
		{"time:>09:00", tl(utc(8, 28, 9, 30, 0)), true},   // G2 今天命中
		{"time:>09:00", tl(utc(8, 29, 8, 0, 0)), true},    // G3 明日绝对时刻更晚 → 命中
		{"time:>09:00", tl(utc(8, 28, 9, 0, 0)), false},   // G4 > 严格边界
		{"time:>=09:00", tl(utc(8, 28, 9, 0, 0)), true},   // G5 >= 含
		{"time:<09:00", tl(utc(8, 27, 23, 50, 0)), true},  // G6 < 绝对比较，昨天更早命中
	})
}

// --- 1.8 IsZero 行 NULL 传播（H1-H8，8 条）---
// H4/H7 按裁决落地的分支级语义:OR 其它分支可救活；notNode 特判仅限直接 time 子节点。

func TestTimeNullPropagationIsZero(t *testing.T) {
	noTime := parsedLine("INFO", "", "", "", "no time field")
	noTimeErr := parsedLine("ERROR", "", "", "", "no time field")
	noTimeInfo := noTime // H7 复用（level=INFO）
	runTimeTC(t, "H", []timeTC{
		{"time:>09:00", noTime, false},                                        // H1 基础 NULL
		{"NOT time:>09:00", noTime, false},                                    // H2 notNode 特判防翻转
		{"level:ERROR AND time:>09:00", noTimeErr, false},                     // H3 AND 传播
		{"time:>09:00 OR level:ERROR", noTimeErr, true},                       // H4 分支级 NULL（裁决定案）
		{"time:10:00..11:00", noTime, false},                                  // H5 区间糖 = 两 time AND
		{"time:>09:00 time:<10:00", noTime, false},                            // H6 双条件同理
		{"NOT (time:>09:00 OR level:ERROR)", noTimeInfo, false},               // H7 深层 NULL 传播（bug 修复后:子树含 time 且行无时间,NOT 不得复活）
		{"time:>09:00 OR NOT time:<08:00", noTime, false},                     // H8 OR 右支 NOT time 特判 false
	})
}

// --- 1.9 TimeRangeHint 新格式（I1-I6，6 条）---

func TestTimeRangeHintNew(t *testing.T) {
	// I1 相对值:显示解析后绝对时刻 + 原始时长（展示跟随 anchor 的 location,时区无关断言）
	h := parseSearchQueryAt("time:>-10m", anchorA0).TimeRangeHint()
	wantI1 := anchorA0.Add(-10 * time.Minute).Format("15:04:05")
	if !strings.Contains(h, wantI1) || !strings.Contains(h, "-10m") {
		t.Errorf("I1: hint 应含 %s 与 -10m, 实际 %q", wantI1, h)
	}
	// I2 区间显示补全后的绝对区间。注:实现排版为 "01-02 15:04:05"（无年份），
	// 矩阵翻译约定"具体排版以实现为准"，断言子串相应调整。
	h = parseSearchQueryAt("time:10:00..11:00", anchorA0).TimeRangeHint()
	if !strings.Contains(h, "08-28") || !strings.Contains(h, "10:00") || !strings.Contains(h, "11:00") {
		t.Errorf("I2: hint 应含 08-28 / 10:00 / 11:00, 实际 %q", h)
	}
	// I3 解析错误:红字经 ParseError 通道（searchbar 的 timeFeedback 渲染
	// "✗ time: <错误>"），TimeRangeHint 此时为空。
	q3 := parseSearchQueryAt("time:>xx", anchorA0)
	if q3.ParseError() == "" {
		t.Error("I3: time:>xx 应产生解析错误")
	}
	if q3.TimeRangeHint() != "" {
		t.Errorf("I3: 解析错误时 TimeRangeHint 应为空, 实际 %q", q3.TimeRangeHint())
	}
	a := &App{searchInput: "time:>xx"}
	if fb := stripANSI(a.timeFeedback(q3)); !strings.Contains(fb, "time") {
		t.Errorf("I3: 错误反馈红字应含 time, 实际 %q", fb)
	}
	// I4 纯关键字无 hint
	if got := parseSearchQueryAt("ERROR", anchorA0).TimeRangeHint(); got != "" {
		t.Errorf("I4: 纯关键字 hint 应为空, 实际 %q", got)
	}
	// I5 OR 语境不显示区间 hint
	if got := parseSearchQueryAt("time:10:00..11:00 OR level:ERROR", anchorA0).TimeRangeHint(); got != "" {
		t.Errorf("I5: OR 语境 hint 应为空, 实际 %q", got)
	}
	// I6 NOT 语境不显示区间 hint
	if got := parseSearchQueryAt("NOT time:>09:00", anchorA0).TimeRangeHint(); got != "" {
		t.Errorf("I6: NOT 语境 hint 应为空, 实际 %q", got)
	}
}

// --- 三、集成用例（file 模式多日，P1-P4，4 条）---
// fixture 为 java-logback 多日样本（时间经 parser 解析为无时区即 UTC 语义）。
// 纯时分形态不进集成（依赖真实 Now 锚点，不稳定），只测带完整日期形态（anchor 无关）。
// 注:矩阵样本行缺 [thread] [traceId] logger 结构，无法命中生产 logback 规则，
// 此处修正为完整结构（时间与 message 语义不变）。

var timeMultiDayFixture = []string{
	"2026-08-26 08:00:00.000 [main] [t-2601] INFO  com.example.Day - day26 early",
	"2026-08-26 12:00:00.000 [main] [t-2602] ERROR com.example.Day - day26 error",
	"2026-08-26 23:59:00.000 [main] [t-2603] WARN  com.example.Day - day26 late",
	"2026-08-27 08:59:00.000 [main] [t-2701] INFO  com.example.Day - day27 before-boundary",
	"2026-08-27 09:00:00.000 [main] [t-2702] INFO  com.example.Day - day27 boundary-exact",
	"2026-08-27 09:00:01.000 [main] [t-2703] INFO  com.example.Day - day27 boundary-plus-1s",
	"2026-08-27 10:30:00.000 [main] [t-2704] ERROR com.example.Day - day27 error",
	"2026-08-27 23:00:00.000 [main] [t-2705] INFO  com.example.Day - day27 late",
}

func TestTimeFileModeMultiDay(t *testing.T) {
	p, err := parser.NewRegexParser("java-logback",
		`(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}) \[(?P<thread>[^\]]+)\] \[(?P<traceId>[^\]]+)\] (?P<level>\w+)\s+(?P<logger>\S+) - (?P<message>.*)`)
	if err != nil {
		t.Fatalf("构造 logback 解析器失败: %v", err)
	}
	app := NewApp(&mockStream{}, parser.NewAutoDetect([]parser.Parser{p}), 1000, nil)
	for _, ln := range timeMultiDayFixture {
		app.processLine(model.RawLine{Text: ln, Source: "file-test"})
	}
	// 前置:8 行全部解析出时间（否则后续断言无意义）
	if app.buffer.Len() != len(timeMultiDayFixture) {
		t.Fatalf("缓冲行数 = %d, 期望 %d", app.buffer.Len(), len(timeMultiDayFixture))
	}
	for i := 0; i < app.buffer.Len(); i++ {
		if app.buffer.Get(i).Time.IsZero() {
			t.Fatalf("第 %d 行未解析出时间: %q", i, app.buffer.Get(i).Raw.Text)
		}
	}

	cases := []struct {
		name     string
		query    string
		wantMsgs []string // 期望命中集合（message 子串）
	}{
		{"P1", "time:>2026-08-27T09:00", []string{
			"day27 boundary-plus-1s", "day27 error", "day27 late",
		}},
		{"P2", "time:2026-08-26..2026-08-27", []string{
			"day26 early", "day26 error", "day26 late",
		}},
		{"P3", "level:ERROR AND time:>2026-08-27T09:00", []string{
			"day27 error",
		}},
		{"P4", "time:<2026-08-27T09:00", []string{
			"day26 early", "day26 error", "day26 late", "day27 before-boundary",
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			app.searchInput = c.query
			app.recomputeView()
			if got := len(app.filteredView); got != len(c.wantMsgs) {
				t.Fatalf("%s %q 命中 %d 行, 期望 %d 行", c.name, c.query, got, len(c.wantMsgs))
			}
			for _, want := range c.wantMsgs {
				found := false
				for _, line := range app.filteredView {
					if strings.Contains(line.Message, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("%s %q 应命中 message 含 %q 的行", c.name, c.query, want)
				}
			}
		})
	}
}

// --- 无日期时间戳（HH:mm:ss.SSS 日志）的时分窗口退化 ---
func TestTimeNoDateTimestamps(t *testing.T) {
	anchor := time.Date(2026, 9, 3, 22, 33, 0, 0, time.UTC)
	// parser 对无日期时间戳产出 0000-01-01（Year()<=0）
	noDate := func(h, m, s int) *model.ParsedLine {
		return parsedLineWithTime("INFO", "", "", "", "x", time.Date(0, 1, 1, h, m, s, 0, time.UTC))
	}
	run := func(query string, line *model.ParsedLine, want bool) {
		t.Helper()
		q := parseSearchQueryAt(query, anchor)
		if got := q.MatchLine(line); got != want {
			t.Errorf("%q 对 %s = %v, want %v", query, line.Time.Format("15:04:05"), got, want)
		}
	}
	run("time:>09:00", noDate(22, 31, 49), true)  // 用户实测场景
	run("time:>09:00", noDate(8, 31, 49), false)
	run("time:<23:00", noDate(22, 31, 49), true)
	run("time:22:00..23:00", noDate(22, 31, 49), true)
	run("time:22:00..22:32", noDate(22, 31, 49), true)   // 上界 22:32:00,行 22:31:49 在内
	run("time:22:00..22:31", noDate(22, 31, 0), true)    // 闭区间含端点
	run("time:22:00..22:30", noDate(22, 31, 49), false)
	// 相对时间退化:锚点取本地视角时分(行按 anchor 本地时刻 ± 偏移构造,时区无关)
	noDateFromLocal := func(anchor time.Time, d time.Duration) *model.ParsedLine {
		loc := anchor.Add(d)
		return parsedLineWithTime("INFO", "", "", "", "x",
			time.Date(0, 1, 1, loc.Hour(), loc.Minute(), loc.Second(), 0, time.UTC))
	}
	run("time:>-10m", noDateFromLocal(anchor, 2*time.Minute), true)   // 本地 now+2m 必在窗口内
	run("time:>-10m", noDateFromLocal(anchor, -20*time.Minute), false) // 本地 now-20m 必在窗口外
	run("time:>-3m", noDateFromLocal(anchor, 1*time.Minute), true)
	run("time:>-3m", noDateFromLocal(anchor, -5*time.Minute), false)
	// 完整日期查询对无日期行:取查询值的时分
	run("time:>2026-09-03T09:00", noDate(22, 31, 49), true)
}
