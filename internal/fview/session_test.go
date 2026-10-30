package fview

import (
	"context"
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
