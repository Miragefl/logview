package tui

import (
	"fmt"
	"testing"
	"time"

	"github.com/justfun/logview/internal/model"
	"github.com/justfun/logview/internal/parser"
)

// perfFill 向 app 灌 n 行日志(n ≤ 容量时流式统计精确,> 容量后发生淘汰)。
func perfFill(app *App, n int) {
	for i := 0; i < n; i++ {
		app.processLine(model.RawLine{
			Text:   fmt.Sprintf("2026-09-09 10:%02d:%02d.%03d [exec-%d] INFO  com.example.OrderService - order id=%d", i/60%60, i%60, i%1000, i%32, i),
			Source: "perf.log",
		})
	}
}

// 性能 guard:10 万行 + 搜索词,单次 recomputeView < 60ms(宽松阈,探针本机 ~33ms)。
func TestRecomputeViewPerfGuard(t *testing.T) {
	if testing.Short() {
		t.Skip("perf guard")
	}
	app := NewApp(&mockStream{}, nil, 200000, nil)
	app.width = 200
	app.height = 50
	perfFill(app, 100000)
	app.searchInput = "order"
	t0 := time.Now()
	app.recomputeView()
	if d := time.Since(t0); d > 60*time.Millisecond {
		t.Fatalf("10 万行 recomputeView = %v, 超过 60ms guard", d)
	}
	if len(app.filteredView) != 100000 {
		t.Fatalf("过滤结果应 10 万行, got %d", len(app.filteredView))
	}
}

// 满环淘汰后 levelCounts 修正:流式计数因淘汰失真,statsDirty 触发 recompute 全量重算修正。
func TestLevelCountsRingEviction(t *testing.T) {
	// perfFill 的行是单中括号 java 格式,需真实 parser 提取 level(nil parsers 时无 Level 字段,流式计数恒 0)
	p, _ := parser.NewRegexParser("java-logback",
		`(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}) \[(?P<thread>[^\]]+)\] (?P<level>\w+)\s+(?P<logger>\S+) - (?P<message>.*)`)
	app := NewApp(&mockStream{}, parser.NewAutoDetect([]parser.Parser{p}), 1000, nil) // 容量 1000
	perfFill(app, 1500)                                                               // 淘汰 500 行
	if app.buffer.Len() != 1000 {
		t.Fatalf("buffer 应满 1000, got %d", app.buffer.Len())
	}
	app.recomputeView()
	// 手工全量重算对照
	want := map[string]int{}
	for i := 0; i < app.buffer.Len(); i++ {
		if lv := app.buffer.Get(i).Get(model.FieldLevel); lv != "" {
			want[lv]++
		}
	}
	if got := app.levelCounts["INFO"]; got != want["INFO"] {
		t.Fatalf("满环后 levelCounts[INFO] = %d, want %d(淘汰修正)", got, want["INFO"])
	}
	// 清空重置后 dirty 复位,流式重新精确
	app.resetViewState()
	perfFill(app, 10)
	app.recomputeView()
	if app.levelCounts["INFO"] != 10 {
		t.Fatalf("重置后流式计数 = %d, want 10", app.levelCounts["INFO"])
	}
}
