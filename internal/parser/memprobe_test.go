package parser

import (
	"bufio"
	"os"
	"runtime"
	"testing"

	"github.com/justfun/logview/internal/model"
)

// TestMemoryFootprintPerLine 内存足迹探针(手动跑):纯 Parse+驻留,统计每行堆字节。
// go test ./internal/parser/ -run TestMemoryFootprintPerLine -v -timeout 300s
func TestMemoryFootprintPerLine(t *testing.T) {
	path := os.Getenv("MEMPROBE_LOG")
	if path == "" {
		t.Skip("设 MEMPROBE_LOG 指向测试日志后运行")
	}
	p, err := NewRegexParser("logback", `(?P<time>\S+ \S+) \[(?P<thread>[^\]]+)\] \[(?P<traceId>[^\]]+)\] (?P<level>\S+)\s+(?P<logger>\S+) - (?P<message>.*)`)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	br := bufio.NewReaderSize(f, 1<<20)
	lines := make([]*model.ParsedLine, 0, 6000000)
	matched := 0
	for {
		line, rerr := br.ReadString('\n')
		if len(line) > 0 && line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}
		if line != "" {
			raw := model.RawLine{Text: line, Source: "perf.log"}
			pl := p.Parse(raw)
			if pl == nil {
				pl = &model.ParsedLine{Raw: raw, Fields: map[model.Field]string{model.FieldMessage: line}}
			} else {
				matched++
			}
			lines = append(lines, pl)
		}
		if rerr != nil {
			break
		}
	}
	// 强制解引用保活:只 len() 的话 liveness 会把 backing array 判死,GC 全回收导致测不到
	runtime.GC()
	var keep int
	for _, pl := range lines {
		keep += len(pl.Message())
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	t.Logf("lines=%d matched=%d keep=%d HeapAlloc=%dMB (%.0f B/line)",
		len(lines), matched, keep, ms.HeapAlloc>>20, float64(ms.HeapAlloc)/float64(len(lines)))
}
