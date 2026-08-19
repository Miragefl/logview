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
	paths       []string
	followLines int
	seq         atomic.Uint64
}

func NewTailSource(paths []string, followLines int) *TailSource {
	return &TailSource{paths: paths, followLines: followLines}
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

func (t *TailSource) sendLine(ctx context.Context, ch chan<- model.RawLine, text, source string) bool {
	raw := model.RawLine{
		Text:   text,
		Source: source,
		Seq:    t.seq.Add(1),
	}
	select {
	case ch <- raw:
		return true
	case <-ctx.Done():
		return false
	}
}

func (t *TailSource) tailFile(ctx context.Context, ch chan<- model.RawLine, path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	source := filepath.Base(path)

	if t.followLines <= 0 {
		// read all existing content
		reader := bufio.NewReader(f)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			line, err := reader.ReadString('\n')
			if err != nil {
				// 末行无换行符时 ReadString 仍返回数据，先消费再退出
				if line != "" {
					if !t.sendLine(ctx, ch, trimNewline(line), source) {
						return
					}
				}
				break
			}
			if !t.sendLine(ctx, ch, trimNewline(line), source) {
				return
			}
		}
	} else {
		// follow mode: read last N lines from end, then follow
		info, statErr := f.Stat()
		if statErr == nil && info.Size() > 0 {
			seekBack := int64(t.followLines) * 1024
			if seekBack > info.Size() {
				seekBack = info.Size()
			}
			start := info.Size() - seekBack
			f.Seek(start, 0)

			reader := bufio.NewReader(f)
			if start > 0 {
				reader.ReadString('\n')
			}

			ring := make([]string, 0, t.followLines)
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					if line != "" {
						ring = append(ring, trimNewline(line))
						if len(ring) > t.followLines {
							ring = ring[1:]
						}
					}
					break
				}
				ring = append(ring, trimNewline(line))
				if len(ring) > t.followLines {
					ring = ring[1:]
				}
			}

			for _, l := range ring {
				if !t.sendLine(ctx, ch, l, source) {
					return
				}
			}
		} else {
			f.Seek(0, 2)
		}
	}

	// follow new lines (both modes enter this loop)
	reader := bufio.NewReader(f)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			// 部分行也立即显示，避免丢行（与 GNU tail 行为一致）
			if line != "" {
				if !t.sendLine(ctx, ch, trimNewline(line), source) {
					return
				}
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if !t.sendLine(ctx, ch, trimNewline(line), source) {
			return
		}
	}
}

func trimNewline(line string) string {
	if len(line) > 0 && line[len(line)-1] == '\n' {
		return line[:len(line)-1]
	}
	return line
}

func (t *TailSource) Cleanup() error { return nil }
