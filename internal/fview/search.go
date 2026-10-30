// 搜索绑定：logcore.SearchQuery 单源执行（784 行解析器不重写不分叉，spec 决策表）。
// 前端输入 query 字符串，Go 侧解析 + 批量匹配返回命中索引。
package fview

import (
	"github.com/justfun/logview/internal/logcore"
	"github.com/justfun/logview/internal/model"
)

// QueryMeta 搜索语法元信息（前端校验错误/高亮词/时间范围提示）。
type QueryMeta struct {
	ParseError        string   `json:"parseError"`
	HighlightKeywords []string `json:"highlightKeywords"`
	TimeRangeHint     string   `json:"timeRangeHint"`
}

// ParseQuery 解析搜索语法（logcore 单源），语法错误经 ParseError 透出。
func ParseQuery(input string) QueryMeta {
	q := logcore.ParseSearchQuery(input)
	return QueryMeta{
		ParseError:        q.ParseError(),
		HighlightKeywords: q.HighlightKeywords(),
		TimeRangeHint:     q.TimeRangeHint(),
	}
}

// MatchLines 全量匹配 tab 行返回命中索引（行序 0..n-1）；
// 空查询或语法错误返回 nil。持锁快照行引用后锁外匹配，不阻塞 ingest。
func (m *Manager) MatchLines(tabID, query string) []int {
	q := logcore.ParseSearchQuery(query)
	if q.IsEmpty() || q.ParseError() != "" {
		return nil
	}
	m.mu.Lock()
	t, ok := m.tabs[tabID]
	m.mu.Unlock()
	if !ok {
		return nil
	}
	t.mu.Lock()
	n := t.buf.Len()
	lines := make([]*model.ParsedLine, n)
	for i := 0; i < n; i++ {
		lines[i] = t.buf.Get(i)
	}
	t.mu.Unlock()

	var idx []int
	for i, pl := range lines {
		if q.MatchLine(pl) {
			idx = append(idx, i)
		}
	}
	return idx
}
