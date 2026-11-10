// 高亮/隐藏关键词历史(验收反馈③):fview 侧词条词频读写,与 TUI 共享
// ~/.local/state/logview/usage.json 的 hl:/hide: 前缀数据(key 格式与 TUI
// keywordHistKey 一致:kind+scope+":"+词,scope 空即全局)。排序逻辑对齐 TUI
// sortedUsageWords 语义(衰减分降序/LastUsed 新→旧/词序升序),不 import internal/tui。
package fview

import (
	"sort"
	"strings"
	"time"

	"github.com/Miragefl/logview/internal/usage"
)

// kindPrefix 前端短名 → usage key 前缀。
var kindPrefix = map[string]string{
	"hl":   usage.KeyHighlight,
	"hide": usage.KeyHide,
}

// keywordLimit 历史列表长度上限(对齐 TUI C-r 列表数据源 top20)。
const keywordLimit = 20

// BumpKeyword 记一次关键词使用(7 天半衰期;非法 kind/空词静默忽略)。
func BumpKeyword(scope, kind, word string) {
	p, ok := kindPrefix[kind]
	if !ok || word == "" {
		return
	}
	usage.Bump(p + scope + ":" + word)
}

// Keywords 按 kind+scope 捞词频 top20(衰减分降序)。
func Keywords(scope, kind string) []string {
	p, ok := kindPrefix[kind]
	if !ok {
		return nil
	}
	prefix := p + scope + ":"
	m := usage.Load()
	now := time.Now()
	type ranked struct {
		word  string
		score float64
		last  int64
	}
	var rs []ranked
	for k, e := range m {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		rs = append(rs, ranked{word: k[len(prefix):], score: e.Decayed(now), last: e.LastUsed})
	}
	sort.Slice(rs, func(i, j int) bool {
		if rs[i].score != rs[j].score {
			return rs[i].score > rs[j].score
		}
		if rs[i].last != rs[j].last {
			return rs[i].last > rs[j].last
		}
		return rs[i].word < rs[j].word
	})
	if len(rs) > keywordLimit {
		rs = rs[:keywordLimit]
	}
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.word
	}
	return out
}
