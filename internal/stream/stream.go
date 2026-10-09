package stream

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/justfun/logview/internal/model"
)

// LogStream produces raw log lines from a data source.
// 通道元素为行批:大文件加载的 channel 锁操作从每行一次(数百万次
// pthread_cond_signal,实测占加载 CPU 的 ~53%)降为每批一次。
type LogStream interface {
	Start(ctx context.Context) (<-chan []model.RawLine, error)
	Label() string
	// Scope 返回源的词频隔离域(关键词历史/频次按此分域:FRP=连接名、SSH=主机、k8s="k8s"、本地/管道="")。
	Scope() string
	Cleanup() error
}

const batchSize = 4096

// Batcher 逐行→攒批的公共发送件(各源共用):满批即发,另有 50ms ticker 兜底
// flush——follow 场景低速追加行不满批也能及时到达(纯满批会把 tail -f 的新行
// 滞留在缓冲直到关闭)。Seq 由 Batcher 内部维护(每源一个,保持源内单调递增)。
// 多 goroutine 并发发送安全(多 pod/多路径场景),批量后锁开销可忽略。
type Batcher struct {
	mu   sync.Mutex
	ch   chan []model.RawLine
	buf  []model.RawLine
	ctx  context.Context
	done chan struct{}
	seq  atomic.Uint64
}

func NewBatcher(ctx context.Context) *Batcher {
	b := &Batcher{
		ch:   make(chan []model.RawLine, 64),
		buf:  make([]model.RawLine, 0, batchSize),
		ctx:  ctx,
		done: make(chan struct{}),
	}
	go b.tickFlush()
	return b
}

// tickFlush 50ms 兜底 flush:高速灌入由满批触发,低速 follow 由时间触发。
func (b *Batcher) tickFlush() {
	t := time.NewTicker(50 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-b.done:
			return
		case <-b.ctx.Done():
			return
		case <-t.C:
			b.mu.Lock()
			if len(b.buf) > 0 {
				b.flushBatchLocked()
			}
			b.mu.Unlock()
		}
	}
}

// Out 返回批通道(消费端)。
func (b *Batcher) Out() <-chan []model.RawLine { return b.ch }

// Send 追加一行,满批即发;ctx 取消返回 false(调用方停止读取)。
func (b *Batcher) Send(text, source string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, model.RawLine{Text: text, Source: source, Seq: b.seq.Add(1)})
	if len(b.buf) >= batchSize {
		return b.flushBatchLocked()
	}
	return true
}

func (b *Batcher) flushBatchLocked() bool {
	if len(b.buf) == 0 {
		return true
	}
	select {
	case b.ch <- b.buf:
		b.buf = make([]model.RawLine, 0, batchSize)
		return true
	case <-b.ctx.Done():
		return false
	}
}

// Close 发送剩余尾批并关闭通道(源读取结束后必须调用)。
func (b *Batcher) Close() {
	b.mu.Lock()
	b.flushBatchLocked()
	close(b.ch)
	b.mu.Unlock()
	close(b.done) // 停兜底 ticker
}
