package stream

import (
	"bufio"
	"context"
	"io"
	"sync/atomic"

	"github.com/justfun/logview/internal/model"
)

type PipeSource struct {
	reader io.Reader
	seq    atomic.Uint64
}

func NewPipeSource(r io.Reader) *PipeSource {
	return &PipeSource{reader: r}
}

func (p *PipeSource) Label() string { return "pipe" }

// Scope 管道属全局域(词频与全局历史共享)。
func (p *PipeSource) Scope() string { return "" }

func (p *PipeSource) Start(ctx context.Context) (<-chan []model.RawLine, error) {
	b := NewBatcher(ctx)
	go func() {
		defer b.Close()
		scanner := bufio.NewScanner(p.reader)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if !b.Send(scanner.Text(), "pipe") {
				return
			}
		}
	}()
	return b.Out(), nil
}

func (p *PipeSource) Cleanup() error { return nil }
