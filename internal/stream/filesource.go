package stream

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sync/atomic"

	"github.com/justfun/logview/internal/model"
)

type FileSource struct {
	paths []string
	seq   atomic.Uint64
}

func NewFileSource(paths []string) *FileSource {
	return &FileSource{paths: paths}
}

func (f *FileSource) Label() string { return "file" }

// Scope 本地文件属全局域(词频与全局历史共享)。
func (f *FileSource) Scope() string { return "" }

func (f *FileSource) Start(ctx context.Context) (<-chan model.RawLine, error) {
	ch := make(chan model.RawLine, 256)
	go func() {
		defer close(ch)
		for _, p := range f.paths {
			f.readFile(ctx, ch, p)
		}
	}()
	return ch, nil
}

func (f *FileSource) readFile(ctx context.Context, ch chan<- model.RawLine, path string) {
	reader, closer, _, err := openMaybeGzip(path)
	if err != nil {
		// 打不开时向通道写一条错误提示行，避免用户面对空屏无反馈
		raw := model.RawLine{
			Text:   fmt.Sprintf("[logview] 无法打开文件 %s: %v", path, err),
			Source: filepath.Base(path),
			Seq:    f.seq.Add(1),
		}
		select {
		case ch <- raw:
		case <-ctx.Done():
		}
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
			// 末行无换行符时 ReadString 仍返回数据，先消费再退出
			if line != "" {
				f.sendLine(ctx, ch, trimNewline(line), source)
			}
			// 非 EOF 的读错误(如中途损坏的 gz)提示用户,而非静默截断
			if err != io.EOF {
				f.sendLine(ctx, ch, fmt.Sprintf("[logview] 读取 %s 出错(可能已损坏): %v", path, err), source)
			}
			break
		}
		f.sendLine(ctx, ch, trimNewline(line), source)
	}
}

func (f *FileSource) sendLine(ctx context.Context, ch chan<- model.RawLine, text, source string) bool {
	raw := model.RawLine{
		Text:   text,
		Source: source,
		Seq:    f.seq.Add(1),
	}
	select {
	case ch <- raw:
		return true
	case <-ctx.Done():
		return false
	}
}

func (f *FileSource) Cleanup() error { return nil }
