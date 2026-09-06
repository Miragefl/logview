package stream

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
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

// Scope 本地 tail 属全局域(词频与全局历史共享)。
func (t *TailSource) Scope() string { return "" }

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

	// gz 归档:全量解压顺序读(解压读满,行进 ring buffer,容量天然封顶),
	// 读到 EOF 即止——归档不追加,follow 轮询无意义(followLines 同样不生效)。
	br := bufio.NewReader(f)
	if isGzipMagic(br) {
		gz, gzErr := gzip.NewReader(br)
		if gzErr != nil {
			t.sendLine(ctx, ch, fmt.Sprintf("[logview] 解压 %s 失败: %v", path, gzErr), source)
			return
		}
		defer gz.Close()
		reader := bufio.NewReader(gz)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			line, err := reader.ReadString('\n')
			if err != nil {
				if line != "" {
					if !t.sendLine(ctx, ch, trimNewline(line), source) {
						return
					}
				}
				if err != io.EOF {
					t.sendLine(ctx, ch, fmt.Sprintf("[logview] 读取 %s 出错(可能已损坏): %v", path, err), source)
				}
				return
			}
			if !t.sendLine(ctx, ch, trimNewline(line), source) {
				return
			}
		}
	}

	// 非 gz:现有逻辑(seek 取尾 + follow 轮询)零改动;全量读分支复用 br——
	// Peek 虽不消费 br 缓冲,但已触发底层预读推进 fd 偏移,重建 reader 会丢文件头。
	if t.followLines <= 0 {
		// read all existing content
		reader := br
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
