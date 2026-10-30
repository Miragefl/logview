package fview

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/justfun/logview/internal/model"
	"github.com/justfun/logview/internal/parser"
)

// feedStream 预置行批的测试流：Start 一次性灌入后关闭（模拟文件读毕/管道 EOF）。
type feedStream struct {
	label   string
	batches [][]model.RawLine
	cleaned bool
}

func (f *feedStream) Start(_ context.Context) (<-chan []model.RawLine, error) {
	ch := make(chan []model.RawLine, len(f.batches))
	for _, b := range f.batches {
		ch <- b
	}
	close(ch)
	return ch, nil
}
func (f *feedStream) Label() string { return f.label }
func (f *feedStream) Scope() string { return "" }
func (f *feedStream) Cleanup() error {
	f.cleaned = true
	return nil
}

// collector 并发安全的事件收集器（emit 注入 Manager）。
type collector struct {
	mu     sync.Mutex
	events []collectedEvent
}

type collectedEvent struct {
	name string
	data any
}

func (c *collector) emit(event string, data any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, collectedEvent{event, data})
}

// linesTotal 累计 tab:lines 事件携带的行数。
func (c *collector) linesTotal() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	total := 0
	for _, e := range c.events {
		if e.name == "tab:lines" {
			total += len(e.data.(linesEvent).Lines)
		}
	}
	return total
}

// repaintMeta 返回首个 repaint=true 的 meta 事件（未出现返回 false）。
func (c *collector) repaintMeta() (metaEvent, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.events {
		if e.name == "tab:meta" {
			if m, ok := e.data.(metaEvent); ok && m.Repaint {
				return m, true
			}
		}
	}
	return metaEvent{}, false
}

// hasClosed 是否收到指定 tab 的 tab:closed 事件。
func (c *collector) hasClosed(tabID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.events {
		if e.name == "tab:closed" {
			if ev, ok := e.data.(closedEvent); ok && ev.TabID == tabID {
				return true
			}
		}
	}
	return false
}

// seqMonotonic tab:lines 事件流（按到达序）的 Seq 是否严格递增。
func (c *collector) seqMonotonic() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	prev := uint64(0)
	for _, e := range c.events {
		if e.name != "tab:lines" {
			continue
		}
		for _, ln := range e.data.(linesEvent).Lines {
			if ln.Seq <= prev {
				return false
			}
			prev = ln.Seq
		}
	}
	return true
}

func maxBatchSize(c *collector) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	max := 0
	for _, e := range c.events {
		if e.name == "tab:lines" {
			if n := len(e.data.(linesEvent).Lines); n > max {
				max = n
			}
		}
	}
	return max
}

// firstSeqSeen 是否存在 FirstSeq==seq 的 tab:lines 批（repaint 全量重发判断）。
func (c *collector) firstSeqSeen(seq uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.events {
		if e.name == "tab:lines" && e.data.(linesEvent).FirstSeq == seq {
			return true
		}
	}
	return false
}

// waitLines 轮询等待 tab 缓冲行数与 emit 行数双达标（超时 5s 失败）。
func waitLines(t *testing.T, m *Manager, c *collector, tabID string, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ok := false
		for _, ti := range m.Tabs() {
			if ti.ID == tabID && ti.Lines >= want && c.linesTotal() >= want {
				ok = true
			}
		}
		if ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("tab %s 行数未达 %d（当前 tabs=%v emit=%d）", tabID, want, m.Tabs(), c.linesTotal())
}

func mkBatch(startSeq int, n int, text func(i int) string) []model.RawLine {
	out := make([]model.RawLine, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, model.RawLine{Text: text(i), Source: "s", Seq: uint64(startSeq + i)})
	}
	return out
}

// TestManagerIngestBatches 多批行经解析入环形缓冲，分批推送（计划 Task 5 Step 1）。
func TestManagerIngestBatches(t *testing.T) {
	c := &collector{}
	m := NewManager(c.emit, 1000)
	src := &feedStream{label: "t.log", batches: [][]model.RawLine{
		mkBatch(1, 100, func(i int) string { return fmt.Sprintf("line-a-%d", i) }),
		mkBatch(101, 100, func(i int) string { return fmt.Sprintf("line-b-%d", i) }),
		mkBatch(201, 100, func(i int) string { return fmt.Sprintf("line-c-%d", i) }),
	}}
	id, err := m.OpenTab(src)
	if err != nil {
		t.Fatalf("OpenTab: %v", err)
	}
	waitLines(t, m, c, id, 300)

	if got := c.linesTotal(); got != 300 {
		t.Errorf("emit 行总数 = %d, 期望 300", got)
	}
	if !c.seqMonotonic() {
		t.Error("emit 流 Seq 应严格递增")
	}
	if got := maxBatchSize(c); got > emitBatch {
		t.Errorf("单批行数 %d 超上限 %d", got, emitBatch)
	}
	if got := len(m.Lines(id, 0, -1)); got != 300 {
		t.Errorf("Lines(0,-1) = %d 行, 期望 300", got)
	}
}

// TestManagerLargeIngest 10 万行初始灌入不丢行、末行 Seq 连续（Review Focus 2）。
func TestManagerLargeIngest(t *testing.T) {
	c := &collector{}
	m := NewManager(c.emit, 100000)
	src := &feedStream{label: "big.log", batches: [][]model.RawLine{
		mkBatch(1, 100000, func(i int) string { return fmt.Sprintf("big-%d", i) }),
	}}
	id, err := m.OpenTab(src)
	if err != nil {
		t.Fatalf("OpenTab: %v", err)
	}
	waitLines(t, m, c, id, 100000)

	if got := c.linesTotal(); got != 100000 {
		t.Errorf("emit 行总数 = %d, 期望 100000（分批不丢行）", got)
	}
	lines := m.Lines(id, 0, -1)
	if last := lines[len(lines)-1]; last.Seq != 100000 {
		t.Errorf("末行 Seq = %d, 期望 100000", last.Seq)
	}
}

// logbackTestParser 构造 java-logback 测试解析器。
func logbackTestParser(t *testing.T) parser.Parser {
	t.Helper()
	p, err := parser.NewRegexParser("java-logback",
		`(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}) \[(?P<thread>[^\]]+)\] \[(?P<traceId>[^\]]+)\] (?P<level>\w+)\s+(?P<logger>\S+) - (?P<message>.*)`)
	if err != nil {
		t.Fatalf("构造 logback 解析器: %v", err)
	}
	return p
}

// plainTestParser 兜底解析器（Name 必须为 "plain-text"，AutoDetect 据此识别降级目标；
// 对齐 CompileRules 对 Fallback 规则的编译产物）。
func plainTestParser(t *testing.T) parser.Parser {
	t.Helper()
	p, err := parser.NewRegexParser("plain-text", `(?P<message>.*)`)
	if err != nil {
		t.Fatalf("构造 plain 解析器: %v", err)
	}
	return p
}

// TestManagerRepaintOnParserUpgrade 解析器升级触发 repaint 信号与全量重发（Review Focus 3）。
func TestManagerRepaintOnParserUpgrade(t *testing.T) {
	c := &collector{}
	m := NewManager(c.emit, 1000)
	m.SetParsers([]parser.Parser{logbackTestParser(t), plainTestParser(t)})

	logbackLine := func(i int) string {
		return fmt.Sprintf("2026-10-30 12:00:%02d.000 [main] [t-1] ERROR com.example.F - err-%d", i%60, i)
	}
	batches := [][]model.RawLine{
		mkBatch(1, 50, func(i int) string { return fmt.Sprintf("plain-%d", i) }), // 先 50 行 plain
		mkBatch(51, 50, logbackLine), // 后 50 行 logback → 升级
	}
	id, err := m.OpenTab(&feedStream{label: "mixed.log", batches: batches})
	if err != nil {
		t.Fatalf("OpenTab: %v", err)
	}
	waitLines(t, m, c, id, 100)

	// 升级信号：repaint=true 的 meta 事件
	deadline := time.Now().Add(5 * time.Second)
	var meta metaEvent
	for time.Now().Before(deadline) {
		if mm, ok := c.repaintMeta(); ok {
			meta = mm
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if meta.TabID == "" {
		t.Fatal("未收到 repaint=true 的 tab:meta 事件")
	}
	if meta.ParserName != "java-logback" {
		t.Errorf("repaint 后 parserName = %q, 期望 java-logback", meta.ParserName)
	}
	// 全量重发：repaint 发生在第 51 行（缓冲 50 行时），重发首批 FirstSeq==1；
	// 总量 = 初始 100 + 重发 50 = 150
	if !c.firstSeqSeen(1) {
		t.Error("repaint 全量重发应存在 FirstSeq==1 的批次")
	}
	if got := c.linesTotal(); got != 150 {
		t.Errorf("emit 行总数 = %d, 期望 150（初始 100 + repaint 重发 50）", got)
	}
	// 升级后 logback 行解析出级别
	lines := m.Lines(id, 50, 51)
	if lines[0].Level != "ERROR" {
		t.Errorf("升级后 logback 行 Level = %q, 期望 ERROR", lines[0].Level)
	}
}

// TestManagerCloseTabCleansUp 关 tab 清理流资源（Review Focus 4）。
func TestManagerCloseTabCleansUp(t *testing.T) {
	c := &collector{}
	m := NewManager(c.emit, 1000)
	src := &feedStream{label: "c.log", batches: [][]model.RawLine{mkBatch(1, 10, func(i int) string { return "x" })}}
	id, err := m.OpenTab(src)
	if err != nil {
		t.Fatalf("OpenTab: %v", err)
	}
	waitLines(t, m, c, id, 10)

	if err := m.CloseTab(id); err != nil {
		t.Fatalf("CloseTab: %v", err)
	}
	if !src.cleaned {
		t.Error("CloseTab 后 stream.Cleanup 未被调用")
	}
	if tabs := m.Tabs(); len(tabs) != 0 {
		t.Errorf("CloseTab 后 Tabs() = %v, 期望空", tabs)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !c.hasClosed(id) {
		time.Sleep(10 * time.Millisecond)
	}
	if !c.hasClosed(id) {
		t.Error("未收到 tab:closed 事件")
	}
}

// slowStream 发批 A（满批触发即时 flush）后延迟发批 B（不满批、不关闭通道），
// 复现 follow 场景尾批滞留 pend 的时序（Final Review Important #3）。
type slowStream struct {
	label string
	ch    chan []model.RawLine
}

func (s *slowStream) Start(_ context.Context) (<-chan []model.RawLine, error) {
	s.ch = make(chan []model.RawLine, 2)
	return s.ch, nil
}
func (s *slowStream) Label() string { return s.label }
func (s *slowStream) Scope() string { return "" }
func (s *slowStream) Cleanup() error {
	return nil
}

// errCleanupStream Cleanup 恒错（Final Review Important #8：Cleanup 出错也要广播 closed）。
type errCleanupStream struct {
	feedStream
}

func (e *errCleanupStream) Cleanup() error { return errors.New("boom") }

// TestOpenTabEmitsOpened OpenTab 后广播 tab:opened（前端 registerTab 唯一入口，Final Review Critical #1）。
func TestOpenTabEmitsOpened(t *testing.T) {
	c := &collector{}
	m := NewManager(c.emit, 100)
	id, err := m.OpenTab(&feedStream{label: "opened.log", batches: [][]model.RawLine{mkBatch(1, 5, func(i int) string { return "x" })}})
	if err != nil {
		t.Fatalf("OpenTab: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	ok := false
	for time.Now().Before(deadline) && !ok {
		c.mu.Lock()
		for _, e := range c.events {
			if e.name == "tab:opened" {
				if ev, alright := e.data.(openedEvent); alright && ev.TabID == id && ev.Label == "opened.log" {
					ok = true
				}
			}
		}
		c.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	if !ok {
		t.Fatal("OpenTab 后未广播 tab:opened {tabID, label}")
	}
}

// TestFlushTimerDrainsPendingTail 批 B 到达于 flush 间隔窗口内且流安静时，
// 兜底定时器必须在时限内把尾批推出（Final Review Important #3）。
func TestFlushTimerDrainsPendingTail(t *testing.T) {
	c := &collector{}
	m := NewManager(c.emit, 1000)
	src := &slowStream{label: "slow.log"}
	id, err := m.OpenTab(src)
	if err != nil {
		t.Fatalf("OpenTab: %v", err)
	}
	// 批 A：满批 1000 行（appendEmit 即时 flush，lastFlush=t0）
	src.ch <- mkBatch(1, 1000, func(i int) string { return "a" })
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && c.linesTotal() < 1000 {
		time.Sleep(10 * time.Millisecond)
	}
	if c.linesTotal() < 1000 {
		t.Fatal("批 A 未到达")
	}
	time.Sleep(80 * time.Millisecond) // 距上次 flush ≥50ms 后再发批 B，触发"窗口内不 flush+流安静"
	// 批 B：10 行不满批，流保持开放
	src.ch <- mkBatch(1001, 10, func(i int) string { return "b" })

	deadline = time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) && c.linesTotal() < 1010 {
		time.Sleep(10 * time.Millisecond)
	}
	if got := c.linesTotal(); got < 1010 {
		t.Errorf("尾批滞留 pend：emit 行数 %d（期望 ≥1010），flush 无定时器兜底", got)
	}
	// RingBuffer 容量 1000：累计 1010 行后缓冲保持 1000（淘汰语义）
	if got := len(m.Lines(id, 0, -1)); got != 1000 {
		t.Errorf("缓冲行数 = %d, 期望 1000（容量淘汰）", got)
	}
}

// TestCloseTabBroadcastsClosedDespiteCleanupError Cleanup 出错仍广播 tab:closed（Final Review Important #8）。
func TestCloseTabBroadcastsClosedDespiteCleanupError(t *testing.T) {
	c := &collector{}
	m := NewManager(c.emit, 1000)
	src := &errCleanupStream{feedStream{label: "e.log", batches: [][]model.RawLine{mkBatch(1, 5, func(i int) string { return "x" })}}}
	id, err := m.OpenTab(src)
	if err != nil {
		t.Fatalf("OpenTab: %v", err)
	}
	waitLines(t, m, c, id, 5)
	if err := m.CloseTab(id); err == nil {
		t.Fatal("Cleanup 恒错时 CloseTab 应返回错误")
	}
	if !c.hasClosed(id) {
		t.Error("Cleanup 出错时 tab:closed 未广播——前端 tab 将永久卡死")
	}
}

// TestSetFollowEmitsMeta SetFollow 广播带 follow 的 meta（前端跟随按钮显示态同步）。
func TestSetFollowEmitsMeta(t *testing.T) {
	c := &collector{}
	m := NewManager(c.emit, 1000)
	id, err := m.OpenTab(&feedStream{label: "f.log", batches: [][]model.RawLine{mkBatch(1, 5, func(i int) string { return "x" })}})
	if err != nil {
		t.Fatalf("OpenTab: %v", err)
	}
	waitLines(t, m, c, id, 5)
	if err := m.SetFollow(id, true); err != nil {
		t.Fatalf("SetFollow: %v", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	found := false
	for _, e := range c.events {
		if e.name == "tab:meta" {
			if ev, ok := e.data.(metaEvent); ok && ev.TabID == id && ev.Follow != nil && *ev.Follow {
				found = true
			}
		}
	}
	if !found {
		t.Error("SetFollow 后未广播 Follow=true 的 tab:meta")
	}
}
