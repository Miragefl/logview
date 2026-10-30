// Package fview 提供 fview 桌面端的 tab 会话管理：一个 tab 绑定一条 LogStream +
// AutoDetect + RingBuffer，行批经 EventsEmit 分批推送前端（≤emitBatch 行/批）。
// ingest 管线独立于 TUI（internal/tui 的 processBatch 是 TUI 侧实现），
// 解析/缓冲/日志源全部复用 internal 核心包，单源不分叉。
package fview

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/justfun/logview/internal/buffer"
	"github.com/justfun/logview/internal/model"
	"github.com/justfun/logview/internal/parser"
	"github.com/justfun/logview/internal/stream"
)

// emitBatch 单次 EventsEmit 行批上限（spec：10 万行初始灌入分批推送）。
const emitBatch = 1000

// flushInterval follow 低速追加时的兜底 flush 间隔（对齐 stream.Batcher 的 50ms）。
const flushInterval = 50 * time.Millisecond

// LineDTO 前端行视图（EventsEmit JSON 序列化结构）。
type LineDTO struct {
	Seq     uint64 `json:"seq"`
	Time    string `json:"time"`
	Level   string `json:"level"`
	Thread  string `json:"thread"`
	TraceID string `json:"traceId"`
	Logger  string `json:"logger"`
	Message string `json:"message"`
	Raw     string `json:"raw"`
}

// TabInfo tab 元信息快照。
type TabInfo struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Lines  int    `json:"lines"`
	Follow bool   `json:"follow"`
}

// 事件协议（emit 的 event 名与负载）。
type linesEvent struct {
	TabID    string    `json:"tabId"`
	Lines    []LineDTO `json:"lines"`
	FirstSeq uint64    `json:"firstSeq"`
}

type metaEvent struct {
	TabID      string `json:"tabId"`
	Label      string `json:"label"`
	ParserName string `json:"parserName"`
	Repaint    bool   `json:"repaint"`
	// Follow 非空时表示跟随态变更（SetFollow 广播；其余场景为 nil 不序列化）
	Follow *bool `json:"follow,omitempty"`
}

type openedEvent struct {
	TabID string `json:"tabId"`
	Label string `json:"label"`
}

type closedEvent struct {
	TabID string `json:"tabId"`
}

type tab struct {
	id     string
	label  string
	follow bool
	src    stream.LogStream
	pars   *parser.AutoDetect
	buf    *buffer.RingBuffer
	cancel context.CancelFunc
	done   chan struct{}

	mu         sync.Mutex
	parserName string
	pend       []LineDTO   // 待 flush 行
	flushTimer *time.Timer // 尾批兜底（流安静时 50ms 后必 flush）
	lastFlush  time.Time
}

func (t *tab) toDTO(pl *model.ParsedLine) LineDTO {
	tm := ""
	if ts := pl.Time(); !ts.IsZero() {
		tm = ts.Format("2006-01-02 15:04:05.000")
	}
	return LineDTO{
		Seq:     pl.Raw.Seq,
		Time:    tm,
		Level:   pl.Level(),
		Thread:  pl.Thread(),
		TraceID: pl.TraceID(),
		Logger:  pl.Logger(),
		Message: pl.Message(),
		Raw:     pl.Raw.Text,
	}
}

// Manager 管理 tab 会话集合；emit 即 Wails EventsEmit 注入（测试注入收集器）。
type Manager struct {
	emit    func(event string, data any)
	bufSize int

	mu      sync.Mutex
	parsers []parser.Parser // SetParsers 注入，OpenTab 时按快照构造 per-tab AutoDetect
	tabs    map[string]*tab
	order   []string
	nextID  int
}

// NewManager 构造会话管理器；bufSize ≤0 时取默认 10 万行。
func NewManager(emit func(event string, data any), bufSize int) *Manager {
	if bufSize <= 0 {
		bufSize = 100000
	}
	return &Manager{emit: emit, bufSize: bufSize, tabs: make(map[string]*tab)}
}

// SetParsers 注入解析规则编译产物（cmd 装配层调；nil = 不解析，全部兜底行）。
func (m *Manager) SetParsers(ps []parser.Parser) {
	m.mu.Lock()
	m.parsers = ps
	m.mu.Unlock()
}

// OpenTab 打开一个日志源 tab，返回 tabID；行流随即经 emit 推送。
func (m *Manager) OpenTab(src stream.LogStream) (string, error) {
	m.mu.Lock()
	id := fmt.Sprintf("tab-%d", m.nextID)
	m.nextID++
	pars := parser.NewAutoDetect(m.parsers)
	m.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	t := &tab{
		id:     id,
		label:  src.Label(),
		src:    src,
		pars:   pars,
		buf:    buffer.NewRingBuffer(m.bufSize),
		cancel: cancel,
		done:   make(chan struct{}),
	}

	ch, err := src.Start(ctx)
	if err != nil {
		cancel()
		return "", err
	}

	m.mu.Lock()
	m.tabs[id] = t
	m.order = append(m.order, id)
	m.mu.Unlock()

	m.emit("tab:opened", openedEvent{TabID: id, Label: t.label})
	go t.ingestLoop(ctx, ch, m)
	return id, nil
}

// CloseTab 关闭 tab：停 ingest、清理流资源、广播 tab:closed。
func (m *Manager) CloseTab(tabID string) error {
	m.mu.Lock()
	t, ok := m.tabs[tabID]
	if ok {
		delete(m.tabs, tabID)
		for i, id := range m.order {
			if id == tabID {
				m.order = append(m.order[:i], m.order[i+1:]...)
				break
			}
		}
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("tab %s 不存在", tabID)
	}
	t.cancel()
	<-t.done // 等 ingest 退出，避免与 Cleanup 竞态
	t.mu.Lock()
	if t.flushTimer != nil {
		t.flushTimer.Stop()
		t.flushTimer = nil
	}
	t.mu.Unlock()
	cleanupErr := t.src.Cleanup()
	// Cleanup 出错也必须广播 closed：tab 已从 map 删除，不广播则前端永久卡死（Final Review #8）
	m.emit("tab:closed", closedEvent{TabID: tabID})
	return cleanupErr
}

// SetFollow 设置 tab 跟随态（App 装配层调；gz 等归档源读满即止，follow 仅表用户意图）。
func (m *Manager) SetFollow(tabID string, follow bool) error {
	m.mu.Lock()
	t, ok := m.tabs[tabID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("tab %s 不存在", tabID)
	}
	t.mu.Lock()
	t.follow = follow
	label := t.label
	t.mu.Unlock()
	m.emit("tab:meta", metaEvent{TabID: tabID, Label: label, Follow: &follow})
	return nil
}

// Tabs 返回打开顺序的 tab 元信息快照。
func (m *Manager) Tabs() []TabInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]TabInfo, 0, len(m.order))
	for _, id := range m.order {
		t := m.tabs[id]
		if t == nil {
			continue
		}
		t.mu.Lock()
		out = append(out, TabInfo{ID: t.id, Label: t.label, Lines: t.buf.Len(), Follow: t.follow})
		t.mu.Unlock()
	}
	return out
}

// Lines 拉取 tab 行窗口 [start, end)；end<0 表示到末尾（计划 pre-flight 裁决）。
// RingBuffer 自身无锁：所有 buf 访问经 t.mu 与 ingest goroutine 串行化。
func (m *Manager) Lines(tabID string, start, end int) []LineDTO {
	m.mu.Lock()
	t, ok := m.tabs[tabID]
	m.mu.Unlock()
	if !ok {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	n := t.buf.Len()
	if start < 0 {
		start = 0
	}
	if end < 0 || end > n {
		end = n
	}
	if start >= end {
		return nil
	}
	slice := t.buf.Slice(start, end)
	out := make([]LineDTO, 0, len(slice))
	for _, pl := range slice {
		out = append(out, t.toDTO(pl))
	}
	return out
}

func (t *tab) ingestLoop(ctx context.Context, ch <-chan []model.RawLine, m *Manager) {
	defer close(t.done)
	for {
		select {
		case <-ctx.Done():
			return
		case batch, ok := <-ch:
			if !ok {
				// 源读毕（file EOF / 管道结束）：flush 尾批并广播最终 meta
				t.flush(m)
				m.emit("tab:meta", metaEvent{TabID: t.id, Label: t.label, ParserName: t.parserName})
				return
			}
			t.processBatch(batch, m)
		}
	}
}

func (t *tab) processBatch(batch []model.RawLine, m *Manager) {
	for _, raw := range batch {
		raw.Text = model.StripANSI(raw.Text)
		var pl *model.ParsedLine
		if t.pars != nil {
			if p := t.pars.Detect(raw); p != nil {
				pl = p.Parse(raw)
				t.onParserSwitch(p, m)
			}
		}
		if pl == nil {
			// 兜底行：Message span 覆盖全文（与 TUI ingestLine 一致）
			pl = &model.ParsedLine{Raw: raw}
			pl.InitBase(raw.Text)
			pl.SetStdSpan(model.FieldMessage, 0, len(raw.Text))
		}
		dto := t.toDTO(pl)
		t.mu.Lock()
		t.buf.Push(pl)
		t.mu.Unlock()
		t.appendEmit(dto, m)
	}
	t.maybeFlush(m)
}

// onParserSwitch 解析器切换处理：仅"已有解析器之后再切换"（plain→结构化等）触发
// 回灌重解析与 repaint 全量重发；首次选定（""→x）无视觉变化，不重发。
func (t *tab) onParserSwitch(p parser.Parser, m *Manager) {
	name := p.Name()
	prev := t.parserName
	t.parserName = name
	if prev == "" || prev == name {
		return
	}
	t.mu.Lock()
	n := t.buf.Len()
	if n == 0 {
		t.mu.Unlock()
		return
	}
	// 回灌：兜底行（Message()==Raw.Text，与 TUI 标识一致）重 Parse
	for i := 0; i < n; i++ {
		pl := t.buf.Get(i)
		if pl.Message() == pl.Raw.Text {
			if npl := p.Parse(pl.Raw); npl != nil {
				t.buf.Set(i, npl)
			}
		}
	}
	// 持锁快照全量 DTO 分片，解锁后再逐批 emit——10 万行 repaint 的
	// JSON 序列化期间不阻塞 Lines()/Tabs()（Final Review #7）
	batches := make([][]LineDTO, 0, n/emitBatch+1)
	for start := 0; start < n; start += emitBatch {
		end := min(start+emitBatch, n)
		dtos := make([]LineDTO, 0, end-start)
		for i := start; i < end; i++ {
			dtos = append(dtos, t.toDTO(t.buf.Get(i)))
		}
		batches = append(batches, dtos)
	}
	t.mu.Unlock()

	m.emit("tab:meta", metaEvent{TabID: t.id, Label: t.label, ParserName: name, Repaint: true})
	for _, dtos := range batches {
		m.emit("tab:lines", linesEvent{TabID: t.id, Lines: dtos, FirstSeq: dtos[0].Seq})
	}
}

func (t *tab) appendEmit(dto LineDTO, m *Manager) {
	t.mu.Lock()
	t.pend = append(t.pend, dto)
	full := len(t.pend) >= emitBatch
	t.mu.Unlock()
	if full {
		t.flush(m)
	}
}

// maybeFlush 尾批兜底：pend 非空且未安排兜底时，50ms 后强制 flush——
// 流安静（follow 低速/尾读最后一批）时不再依赖下一批到达才检查时间。
func (t *tab) maybeFlush(m *Manager) {
	t.mu.Lock()
	if len(t.pend) == 0 || t.flushTimer != nil {
		t.mu.Unlock()
		return
	}
	t.flushTimer = time.AfterFunc(flushInterval, func() {
		t.flush(m)
		t.mu.Lock()
		t.flushTimer = nil
		t.mu.Unlock()
	})
	t.mu.Unlock()
}

func (t *tab) flush(m *Manager) {
	t.mu.Lock()
	batch := t.pend
	t.pend = nil
	t.lastFlush = time.Now()
	t.mu.Unlock()
	if len(batch) == 0 {
		return
	}
	m.emit("tab:lines", linesEvent{TabID: t.id, Lines: batch, FirstSeq: batch[0].Seq})
}
