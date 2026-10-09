package tui

import (
	"fmt"
	"testing"

	"github.com/justfun/logview/internal/model"
	"github.com/justfun/logview/internal/stacktrace"
)

// 搜索全量刷新性能基准:10 万行 logback,单关键词命中
func BenchmarkFlushSearch100k(b *testing.B) {
	a := NewApp(&mockStream{}, nil, 100000, nil)
	a.width, a.height = 120, 40
	for i := 0; i < 100000; i++ {
		a.processLine(newBenchRaw(i))
	}
	a.searchMode = true
	a.searchInput = "order"
	a.searchPending = true
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.searchPending = true
		a.flushSearch()
	}
}

func newBenchRaw(i int) model.RawLine {
	return model.RawLine{
		Text:   fmt.Sprintf("2026-10-09 09:%02d:%02d.%03d [http-nio-8080-exec-%d] [%016x] INFO  c.j.o.OrderServiceImpl - createOrder userId=%d sku=%d status=PAID amount=9.99", i%60, i%60, i%1000, i%50+1, uint64(i)*2654435761, i, i*7),
		Source: "bench.log",
	}
}

func BenchmarkContainsIgnoreCase(b *testing.B) {
	s := "createOrder userId=123456 sku=789 status=PAID amount=9.99 with mixed Case Words"
	sub := "status=paid"
	for i := 0; i < b.N; i++ {
		containsIgnoreCase(s, sub)
	}
}

// 分帧刷新与同步全量结果一致(12 万行跨 3 片,含作废路径)
func TestChunkedRecomputeMatchesSync(t *testing.T) {
	a := NewApp(&mockStream{}, nil, 200000, nil)
	a.width, a.height = 120, 40
	for i := 0; i < 120000; i++ {
		a.processLine(newBenchRaw(i))
	}
	a.searchInput = "order"
	cmd := a.startChunkedRecompute()
	if cmd == nil {
		t.Fatal("12 万行(>recomputeChunkMin)应走分帧路径")
	}
	steps := 0
	for a.chunkActive && steps < 100 {
		msg := cmd()
		rc, ok := msg.(recomputeChunkMsg)
		if !ok {
			t.Fatalf("chunk 链应持续返回 recomputeChunkMsg, got %T", msg)
		}
		cmd = a.handleRecomputeChunk(rc)
		steps++
	}
	if a.chunkActive {
		t.Fatalf("chunk 链未完成(%d 步)", steps)
	}
	if steps != 3 { // 120000/50000 = 3 片(最后片收尾)
		t.Errorf("分片数 = %d, want 3", steps)
	}
	want := 0
	for i := 0; i < a.buffer.Len(); i++ {
		if a.matchLineForFilter(a.buffer.Get(i)) {
			want++
		}
	}
	if len(a.filteredView) != want {
		t.Fatalf("分帧结果 %d 行, 同步语义 %d 行", len(a.filteredView), want)
	}

	// 作废路径:新输入令牌自增后,在途分片(旧令牌)被丢弃
	a.searchInput = "zzz-not-exist"
	cmd2 := a.startChunkedRecompute()
	msg := cmd2()
	if rc, ok := msg.(recomputeChunkMsg); ok {
		a.chunkTok++ // 模拟新输入作废
		if got := a.handleRecomputeChunk(rc); got != nil {
			t.Fatal("已作废分片不应续跑")
		}
	}
}

// 回归:分帧清空视图后的首个渲染帧不越界——旧堆栈组/游标坐标(数百万级)
// 若未随清空作废,片间渲染帧会以旧坐标索引空 filteredView 直接 panic
func TestChunkedRecomputeRenderSafe(t *testing.T) {
	a := NewApp(&mockStream{}, nil, 200000, nil)
	a.width, a.height = 120, 40
	for i := 0; i < 120000; i++ {
		a.processLine(newBenchRaw(i))
	}
	a.searchInput = "order"
	a.recomputeView()
	// 模拟旧视图的大坐标状态
	a.cursor = len(a.filteredView) - 1
	a.offset = max(0, a.cursor-5)
	a.stGroups = []stacktrace.Group{{Start: 1, End: 100, Leader: "boom"}}
	a.startChunkedRecompute() // 清空+作废坐标
	_ = a.View()              // 渲染帧:旧代码此处 index out of range panic
}

// 纯消费路径基准:每行 processLine(解析+缓冲+滑窗),预生成行避免 fmt 污染
func BenchmarkProcessLine(b *testing.B) {
	const n = 200000
	lines := make([]model.RawLine, n)
	for i := range lines {
		lines[i] = newBenchRaw(i)
	}
	a := NewApp(&mockStream{}, nil, 1000000, nil)
	a.width, a.height = 120, 40
	a.processLine(lines[0]) // 预热
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.processLine(lines[i%n])
	}
}
