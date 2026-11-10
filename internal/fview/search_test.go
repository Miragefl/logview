package fview

import (
	"fmt"
	"testing"

	"github.com/Miragefl/logview/internal/model"
	"github.com/Miragefl/logview/internal/parser"
)

// TestMatchLines 全量匹配返回命中索引（logcore 单源执行；语法错误返回 nil）。
func TestMatchLines(t *testing.T) {
	c := &collector{}
	m := NewManager(c.emit, 1000)
	m.SetParsers([]parser.Parser{logbackTestParser(t), plainTestParser(t)})
	src := &feedStream{label: "s.log", batches: [][]model.RawLine{
		mkBatch(1, 100, func(i int) string {
			lv := "INFO"
			if i%2 == 0 {
				lv = "ERROR"
			}
			return fmt.Sprintf("2026-10-30 12:00:%02d.000 [main] [t-1] %s com.example.S - msg-%d", i%60, lv, i)
		}),
	}}
	id, err := m.OpenTab(src, "", "")
	if err != nil {
		t.Fatalf("OpenTab: %v", err)
	}
	waitLines(t, m, c, id, 100)

	// level:ERROR 命中偶数下标 50 行
	hits := m.MatchLines(id, "level:ERROR")
	if len(hits) != 50 {
		t.Fatalf("level:ERROR 命中 %d 行, 期望 50", len(hits))
	}
	if hits[0] != 0 || hits[1] != 2 {
		t.Errorf("命中索引应从 0 开始的偶数，实际前两个 %v", hits[:2])
	}

	// 关键词匹配
	hits = m.MatchLines(id, "msg-55")
	if len(hits) != 1 || hits[0] != 55 {
		t.Errorf("msg-55 应命中索引 55，实际 %v", hits)
	}

	// 语法错误 → nil
	if got := m.MatchLines(id, "time:>xx"); got != nil {
		t.Errorf("语法错误应返回 nil，实际 %v", got)
	}

	// 空查询 → nil
	if got := m.MatchLines(id, ""); got != nil {
		t.Errorf("空查询应返回 nil，实际 %v", got)
	}
}
