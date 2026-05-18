package stream

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/justfun/logview/internal/model"
)

type TailSource struct {
	paths      []string
	followOnly bool
	seq        atomic.Uint64
}

func NewTailSource(paths []string, followOnly bool) *TailSource {
	return &TailSource{paths: paths, followOnly: followOnly}
}

func (t *TailSource) Label() string { return "tail" }

func (t *TailSource) Start(ctx context.Context) (<-chan model.RawLine, error) {
	ch := make(chan model.RawLine, 256)
	go func() {
		defer close(ch)
		var wg sync.WaitGroup
		for _, p := range t.paths {
			wg.Add(1)
			go func(path string) {
				defer wg.Done()
				t.tailFile(ctx, ch, path)
			}(p)
		}
		wg.Wait()
	}()
	return ch, nil
}

func (t *TailSource) tailFile(ctx context.Context, ch chan<- model.RawLine, path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	reader := bufio.NewReader(f)

	if !t.followOnly {
		// Read existing content first
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			if len(line) > 0 && line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
			}

			raw := model.RawLine{
				Text:   line,
				Source: filepath.Base(path),
				Seq:    t.seq.Add(1),
			}
			select {
			case ch <- raw:
			case <-ctx.Done():
				return
			}
		}
	} else {
		// Follow mode: seek to end, skip all existing content
		f.Seek(0, 2)
	}

	// Then tail for new lines
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if len(line) > 0 && line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}

		raw := model.RawLine{
			Text:   line,
			Source: filepath.Base(path),
			Seq:    t.seq.Add(1),
		}
		select {
		case ch <- raw:
		case <-ctx.Done():
			return
		}
	}
}

func (t *TailSource) Cleanup() error { return nil }