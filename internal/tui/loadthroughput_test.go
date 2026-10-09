package tui

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/justfun/logview/internal/model"
	"github.com/justfun/logview/internal/parser"
	"github.com/justfun/logview/internal/stream"
)

// 全链数据吞吐(手动跑,分离数据链与渲染占比):FileSource 块读→processBatch
// (真实 logback 正则解析+缓冲+DetectAppend),不经过 tea 渲染。
// go test ./internal/tui/ -run TestLoadThroughput -v -timeout 600s
func TestLoadThroughput(t *testing.T) {
	const log = "/tmp/logview-perf-1g.log"
	if _, err := os.Stat(log); err != nil {
		t.Skip("需 /tmp/logview-perf-1g.log")
	}
	p, err := parser.NewRegexParser("logback", `(?P<time>\S+ \S+) \[(?P<thread>[^\]]+)\] \[(?P<traceId>[^\]]+)\] (?P<level>\S+)\s+(?P<logger>\S+) - (?P<message>.*)`)
	if err != nil {
		t.Fatal(err)
	}
	ad := parser.NewAutoDetect([]parser.Parser{p})
	src := stream.NewFileSource([]string{log}).WithTailLines(7000000)
	ch, err := src.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	a := NewApp(&mockStream{}, ad, 7000000, nil)
	a.width, a.height = 120, 40
	t0 := time.Now()
	var lines []model.RawLine
	for batch := range ch {
		for _, raw := range batch {
			lines = append(lines, raw)
			if len(lines) >= 1000 {
				a.processBatch(lines)
				lines = lines[:0]
			}
		}
	}
	if len(lines) > 0 {
		a.processBatch(lines)
	}
	el := time.Since(t0)
	t.Logf("全链数据吞吐: %d 行 / %v (%.0f 万行/s)",
		a.buffer.Len(), el.Round(time.Millisecond), float64(a.buffer.Len())/el.Seconds()/10000)
}
