package stream

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/justfun/logview/internal/model"
)

type FileSource struct {
	paths     []string
	tailLines int
	seq       atomic.Uint64
}

func NewFileSource(paths []string) *FileSource {
	return &FileSource{paths: paths}
}

// WithTailLines 设置尾读行数(0=保持全量读);返回自身支持链式装配。
// 大文件全量读完后中间行终被 ring buffer 淘汰,尾读只搬最终会留下来的行。
func (f *FileSource) WithTailLines(n int) *FileSource {
	f.tailLines = n
	return f
}

func (f *FileSource) Label() string { return "file" }

func (f *FileSource) Cleanup() error { return nil }

// Scope 本地文件属全局域(词频与全局历史共享)。
func (f *FileSource) Scope() string { return "" }

func (f *FileSource) Start(ctx context.Context) (<-chan []model.RawLine, error) {
	b := NewBatcher(ctx)
	go func() {
		defer b.Close()
		for _, p := range f.paths {
			f.readFile(ctx, b, p)
		}
	}()
	return b.Out(), nil
}

func (f *FileSource) readFile(ctx context.Context, b *Batcher, path string) {
	// 尾读路径:tailLines>0 的普通(非 gz)文件从尾部定位起点再顺序读;
	// 不适用(打开失败/gz)时回落下方全量路径统一处理错误与解压
	if f.tailLines > 0 && f.readFromTail(ctx, b, path) {
		return
	}
	// 普通文件全量也走块式(原文 arena 化);gz 回落 openMaybeGzip 逐行解压
	if f.readPlainChunked(ctx, b, path) {
		return
	}
	reader, closer, _, err := openMaybeGzip(path)
	if err != nil {
		// 打不开时向通道写一条错误提示行,避免用户面对空屏无反馈
		b.Send(fmt.Sprintf("[logview] 无法打开文件 %s: %v", path, err), filepath.Base(path))
		return
	}
	defer closer.Close()

	source := filepath.Base(path)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			// 末行无换行符时 ReadString 仍返回数据,先消费再退出
			if line != "" {
				b.Send(trimNewline(line), source)
			}
			// 非 EOF 的读错误(如中途损坏的 gz)提示用户,而非静默截断
			if err != io.EOF {
				b.Send(fmt.Sprintf("[logview] 读取 %s 出错(可能已损坏): %v", path, err), source)
			}
			break
		}
		b.Send(trimNewline(line), source)
	}
}

// readFromTail 尾读路径:非 gz 文件从尾部 tailLines 行起点顺序读到 EOF。
// 定位到起点(start>0)时先发提示行告知当前是尾部窗口;start==0(文件行数不足)
// 等价全量读,不发提示。返回 false 表示不适用(打开失败/gz),回落全量路径。
func (f *FileSource) readFromTail(ctx context.Context, b *Batcher, path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	if isGzipFile(file) {
		return false // gz 流无法 seek,回落全量解压读(ring 容量天然封顶)
	}
	source := filepath.Base(path)
	if st, err := file.Stat(); err == nil && st.Size() > 0 {
		if start := seekTailLines(file, f.tailLines); start > 0 {
			if _, err := file.Seek(start, io.SeekStart); err != nil {
				return false
			}
			b.Send(tailNoticeLine(f.tailLines, st.Size()), source)
		}
	}
	return f.sendChunkedLines(ctx, file, b, source)
}

// readPlainChunked 非 gz 普通文件的块式全量读;返回 false(打开失败/gz)回落 openMaybeGzip 路径。
func (f *FileSource) readPlainChunked(ctx context.Context, b *Batcher, path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	if isGzipFile(file) {
		return false
	}
	return f.sendChunkedLines(ctx, file, b, filepath.Base(path))
}

// sendChunkedLines 块式读取并按行发送:每 1MB 块一次 string 拷贝,
// 行以子串共享块 backing——替代逐行 ReadString 的独立分配,
// 消除每行原文的 size class 取整损耗并减半 GC 对象数(列存配套的原文 arena 化)。
// 跨块不完整行经 carry 拼接;读错误提示用户而非静默截断。
func (f *FileSource) sendChunkedLines(ctx context.Context, r io.Reader, b *Batcher, source string) bool {
	buf := make([]byte, 1<<20)
	var carry []byte
	for {
		n, rerr := r.Read(buf)
		if n > 0 {
			data := append(carry, buf[:n]...)
			chunk := string(data)
			carry = carry[:0]
			start := 0
			for i := 0; i < len(chunk); i++ {
				if chunk[i] == '\n' {
					if !b.Send(chunk[start:i], source) {
						return true
					}
					start = i + 1
				}
			}
			if start < len(chunk) {
				carry = append(carry, chunk[start:]...) // 块尾不完整行,留待下块拼接
			}
		}
		if rerr != nil {
			if len(carry) > 0 { // 末行无换行符,同样消费
				b.Send(string(carry), source)
			}
			if rerr != io.EOF {
				b.Send(fmt.Sprintf("[logview] 读取 %s 出错(可能已损坏): %v", source, rerr), source)
			}
			return true
		}
	}
}
