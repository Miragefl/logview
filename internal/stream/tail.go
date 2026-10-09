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
	tailLines   int
	seq         atomic.Uint64
}

func NewTailSource(paths []string, followLines int) *TailSource {
	return &TailSource{paths: paths, followLines: followLines}
}

// WithTailLines 设置非 follow 模式(followLines<=0)下的尾读行数;返回自身支持链式装配。
// 大文件全量读完后中间行终被 ring buffer 淘汰,尾读只搬最终会留下来的行。
func (t *TailSource) WithTailLines(n int) *TailSource {
	t.tailLines = n
	return t
}

func (t *TailSource) Label() string { return "tail" }

// Scope 本地 tail 属全局域(词频与全局历史共享)。
func (t *TailSource) Scope() string { return "" }

func (t *TailSource) Start(ctx context.Context) (<-chan []model.RawLine, error) {
	b := NewBatcher(ctx)
	go func() {
		defer b.Close()
		var wg sync.WaitGroup
		for _, p := range t.paths {
			wg.Add(1)
			go func(path string) {
				defer wg.Done()
				t.tailFile(ctx, b, path)
			}(p)
		}
		wg.Wait()
	}()
	return b.Out(), nil
}

// sendLine 批量通道发送的兼容薄层(保持既有调用点形态;seq 由 Batcher 维护)。
func (t *TailSource) sendLine(ctx context.Context, b *Batcher, text, source string) bool {
	return b.Send(text, source)
}

func (t *TailSource) tailFile(ctx context.Context, ch *Batcher, path string) {
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
		// 尾读(与 FileSource 同策略):大文件定位尾部起点后从起点顺序读;
		// start==0(行数不足)保持 br 从头读(头数据可能仍在 br 预读缓冲,不能重建)
		if t.tailLines > 0 {
			if start := seekTailLines(f, t.tailLines); start > 0 {
				if _, err := f.Seek(start, io.SeekStart); err == nil {
					if st, statErr := f.Stat(); statErr == nil {
						t.sendLine(ctx, ch, tailNoticeLine(t.tailLines, st.Size()), source)
					}
					reader = bufio.NewReader(f)
				}
			}
		}
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
			seekBack := min(int64(t.followLines)*1024, info.Size())
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
