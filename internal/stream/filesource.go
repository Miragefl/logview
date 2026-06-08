package stream

import (
	"bufio"
	"context"
	"os"
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
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	source := filepath.Base(path)
	reader := bufio.NewReader(file)
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
			Source: source,
			Seq:    f.seq.Add(1),
		}
		select {
		case ch <- raw:
		case <-ctx.Done():
			return
		}
	}
}

func (f *FileSource) Cleanup() error { return nil }
