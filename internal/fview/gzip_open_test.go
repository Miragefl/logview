package fview

import (
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/justfun/logview/internal/parser"
	"github.com/justfun/logview/internal/stream"
)

// TestGzipOpenEOF gzip 归档经 Manager 完整管线：自动解压读满、EOF 即止、
// 非 follow 语义（Review Focus 5；.gz 的 follow 由 stream 层内部降级为读满）。
func TestGzipOpenEOF(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "app.log.gz")
	fw, err := os.Create(p)
	if err != nil {
		t.Fatalf("创建临时 gz: %v", err)
	}
	zw := gzip.NewWriter(fw)
	for i := 1; i <= 100; i++ {
		fmt.Fprintf(zw, "2026-10-30 12:00:%02d.000 [main] [t-1] INFO com.example.Gz - gz-line-%d\n", i%60, i)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("关闭 gzip writer: %v", err)
	}
	if err := fw.Close(); err != nil {
		t.Fatalf("关闭文件: %v", err)
	}

	c := &collector{}
	m := NewManager(c.emit, 1000)
	m.SetParsers([]parser.Parser{logbackTestParser(t), plainTestParser(t)})
	id, err := m.OpenTab(stream.NewFileSource([]string{p}).WithTailLines(100000), "", "")
	if err != nil {
		t.Fatalf("OpenTab(gz): %v", err)
	}
	waitLines(t, m, c, id, 100)

	tabs := m.Tabs()
	if len(tabs) != 1 || tabs[0].Follow {
		t.Errorf("gz tab Follow 应为 false，实际 %v", tabs)
	}
	if got := len(m.Lines(id, 0, -1)); got != 100 {
		t.Errorf("gz 解压行数 = %d, 期望 100", got)
	}
	if got := m.Lines(id, 99, -1)[0].Message; got != "gz-line-100" {
		t.Errorf("末行 message = %q, 期望 gz-line-100", got)
	}
}
