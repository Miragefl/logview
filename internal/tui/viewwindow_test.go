package tui

import (
	"fmt"
	"testing"
	"time"

	"github.com/justfun/logview/internal/model"
)

// 视图滑窗:filteredView 与 ring buffer 同容量边界,超限滑掉头部,
// 视图内容始终是最近的 bufSize 行(滚动窗口语义)。
func TestViewSlideWindowBounds(t *testing.T) {
	app := NewApp(&mockStream{}, nil, 3, nil)
	app.width = 120
	app.height = 40
	for i := 0; i < 5; i++ {
		app.processLine(model.RawLine{Text: fmt.Sprintf("line-%d", i), Source: "s"})
	}
	if len(app.filteredView) != 3 {
		t.Fatalf("filteredView len = %d, want 3(=bufSize 滑窗边界)", len(app.filteredView))
	}
	for i, want := range []string{"line-2", "line-3", "line-4"} {
		if got := app.filteredView[i].Raw.Text; got != want {
			t.Errorf("filteredView[%d] = %q, want %q", i, got, want)
		}
	}
}

// 非混入态非 autoscroll 下,滑窗淘汰头部行时光标跟随行内容前移(不跳行)。
func TestViewSlideWindowCursorFollows(t *testing.T) {
	app := NewApp(&mockStream{}, nil, 3, nil)
	app.width = 120
	app.height = 40
	app.autoscroll = false
	for i := 0; i < 3; i++ {
		app.processLine(model.RawLine{Text: fmt.Sprintf("line-%d", i), Source: "s"})
	}
	app.cursor = 1 // 指向 line-1
	app.processLine(model.RawLine{Text: "line-3", Source: "s"}) // 滑掉 line-0,line-1 前移到索引 0
	if app.cursor != 0 || app.filteredView[app.cursor].Raw.Text != "line-1" {
		t.Fatalf("cursor = %d(%q), want 0(line-1)", app.cursor, app.filteredView[app.cursor].Raw.Text)
	}
	app.processLine(model.RawLine{Text: "line-4", Source: "s"}) // 滑掉 line-1
	if app.cursor != 0 || app.filteredView[app.cursor].Raw.Text != "line-2" {
		t.Fatalf("cursor = %d(%q), want 0(line-2)", app.cursor, app.filteredView[app.cursor].Raw.Text)
	}
}

// 滑窗淘汰时堆栈组索引同步迁移(整组滑出丢弃),DetectAppend 续扫不断裂。
func TestViewSlideWindowStackGroups(t *testing.T) {
	app := NewApp(&mockStream{}, nil, 6, nil)
	app.width = 120
	app.height = 40
	lines := []string{
		"filler-0",
		"filler-1",
		"java.lang.NullPointerException",
		"  at a.B.c(B.java:1)",
		"  at a.B.d(B.java:2)",
		"filler-2",
	}
	for _, l := range lines {
		app.processLine(model.RawLine{Text: l, Source: "s"})
	}
	app.processBatch(nil) // 触发 DetectAppend,组 {2,4}
	if len(app.stGroups) != 1 || app.stGroups[0].Start != 2 || app.stGroups[0].End != 4 {
		t.Fatalf("stGroups = %+v, want [{2 4}]", app.stGroups)
	}
	app.processLine(model.RawLine{Text: "filler-3", Source: "s"}) // 滑掉 filler-0 → 组 {1,3}
	if g := app.stGroups[0]; g.Start != 1 || g.End != 3 {
		t.Fatalf("after 1st evict stGroups = %+v, want [{1 3}]", app.stGroups)
	}
	app.processLine(model.RawLine{Text: "filler-4", Source: "s"}) // 滑掉 filler-1 → 组 {0,2}
	if g := app.stGroups[0]; g.Start != 0 || g.End != 2 {
		t.Fatalf("after 2nd evict stGroups = %+v, want [{0 2}]", app.stGroups)
	}
	// 组头(NPE 行)也被滑出:残组先迁移(Start 钳 0),重扫后整组消亡——头没了组不成立
	app.processLine(model.RawLine{Text: "filler-5", Source: "s"})
	app.processBatch(nil)
	if len(app.stGroups) != 0 {
		t.Fatalf("residual group should be cleared after leader evicted: %+v", app.stGroups)
	}
}

// 状态栏加载徽章:流结束(EOF)定格总时长,未结束按静默间隔区分加载中/实时
func TestLoadBadge(t *testing.T) {
	a := NewApp(&mockStream{}, nil, 10, nil)
	if a.loadBadge() != "" {
		t.Fatalf("首行未到应无徽章, got %q", a.loadBadge())
	}
	// 显式时间戳模拟:首行 10s 前,最近一行距今 <3s → 活跃接收
	base := time.Now().Add(-10 * time.Second)
	a.firstLineAt = base
	a.lastLineAt = time.Now().Add(-1 * time.Second)
	a.lastLineAt = a.firstLineAt.Add(2500 * time.Millisecond) // span=2.5s 且距今 >3s
	// span 2.5s 距今 7.5s:静默 → 实时
	if got := a.loadBadge(); got != "[实时: 2.5s]" {
		t.Fatalf("静默态徽章 = %q, want [实时: 2.5s]", got)
	}
	// 最近一行回到此刻附近 → 加载中
	a.lastLineAt = time.Now().Add(-500 * time.Millisecond)
	a.firstLineAt = a.lastLineAt.Add(-2500 * time.Millisecond) // span=2.5s
	if got := a.loadBadge(); got != "[加载中: 2.5s]" {
		t.Fatalf("活跃接收期徽章 = %q, want [加载中: 2.5s]", got)
	}
	a.loadDur = 45600 * time.Millisecond // 流结束定格
	if got := a.loadBadge(); got != "[加载: 45.6s]" {
		t.Fatalf("定格徽章 = %q, want [加载: 45.6s]", got)
	}
}

// 时长与字节格式化
func TestFmtDurAndHumanBytes(t *testing.T) {
	if fmtDur(45600*time.Millisecond) != "45.6s" {
		t.Errorf("fmtDur 45.6s 失败")
	}
	if fmtDur(93700*time.Millisecond) != "1m33s" {
		t.Errorf("fmtDur 1m33s 失败")
	}
	if humanBytes(94<<20) != "94MB" {
		t.Errorf("humanBytes 94MB 失败")
	}
	if humanBytes(2608<<20) != "2.5GB" {
		t.Errorf("humanBytes 2.5GB 失败")
	}
}
