package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/justfun/logview/internal/logcore"
	"github.com/justfun/logview/internal/model"
	"github.com/justfun/logview/internal/parser"
)

// TestTimeFileModeMultiDay 原住 searchquery_time_test.go；searchquery 迁 internal/logcore 时留在
// tui——它测 App 层端到端过滤（NewApp + processLine + filteredView），非纯查询解析。
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
		if app.buffer.Get(i).Time().IsZero() {
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
					if strings.Contains(line.Message(), want) {
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

// TestCurrentQueryPartialStripsOperator 原住 searchquery_test.go；测 App.currentQuery()
// 的中间态剥离，属 App 层行为，searchquery 迁 internal/logcore 时留在 tui。
func TestCurrentQueryPartialStripsOperator(t *testing.T) {
	a := &App{}
	line := &model.ParsedLine{Raw: model.RawLine{Text: "hello world"}, Fields: map[model.Field]string{model.FieldMessage: "hello world"}}
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

// TestTimeFeedbackErrorContainsTime I3 错误反馈红字渲染段（原 TestTimeRangeHintNew 内嵌，
// 依赖 App.timeFeedback 与 stripANSI，随迁移留在 tui）。
func TestTimeFeedbackErrorContainsTime(t *testing.T) {
	anchor := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	q3 := logcore.ParseSearchQueryAt("time:>xx", anchor)
	a := &App{searchInput: "time:>xx"}
	if fb := stripANSI(a.timeFeedback(q3)); !strings.Contains(fb, "time") {
		t.Errorf("I3: 错误反馈红字应含 time, 实际 %q", fb)
	}
}
