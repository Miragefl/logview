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

func (p *PipeSource) Start(ctx context.Context) (<-chan model.RawLine, error) {
	ch := make(chan model.RawLine, 256)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(p.reader)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}
			line := model.RawLine{
				Text:   scanner.Text(),
				Source: "pipe",
				Seq:    p.seq.Add(1),
			}
			select {
			case ch <- line:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

func (p *PipeSource) Cleanup() error { return nil }