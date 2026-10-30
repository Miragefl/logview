package tui

import (
	"sort"
	"strings"
	"time"

	"github.com/justfun/logview/internal/usage"
)

// 热点数据通用逻辑（加载/衰减/Bump/持久化）在 internal/usage 共享包，
// TUI 与 fview 桌面端读写同一份 ~/.local/state/logview/usage.json。
// 本文件保留 TUI 专属的候选排序（★ 常用标记）与词条历史。

// key 前缀别名：TUI 内引用名保持不变，值与共享包常量对齐。
const (
	usageSSHHost   = usage.KeySSHHost
	usageK8sCtx    = usage.KeyK8sCtx
	usageK8sNS     = usage.KeyK8sNS
	usageK8sRes    = usage.KeyK8sRes
	usageFRPConn   = usage.KeyFRPConn
	usageHighlight = usage.KeyHighlight
	usageHide      = usage.KeyHide
)

// BumpUsage 记录一次使用（薄包装转共享包，调用面不动）。
func BumpUsage(key string) { usage.Bump(key) }

// sortCandidatesHot 候选按（衰减频次降序, 名称升序）排序；带 hot 标记（★ 前缀给常用项 label）。
func sortCandidatesHot(items []sourceCandidate, keyPrefix string, mark bool) []sourceCandidate {
	if len(items) == 0 {
		return items
	}
	m := usage.Load()
	now := time.Now()
	type rank struct {
		item  sourceCandidate
		score float64
	}
	ranks := make([]rank, len(items))
	for i, it := range items {
		ranks[i] = rank{item: it, score: m[keyPrefix+it.value].Decayed(now)}
	}
	sort.SliceStable(ranks, func(i, j int) bool {
		if ranks[i].score != ranks[j].score {
			return ranks[i].score > ranks[j].score
		}
		return ranks[i].item.label < ranks[j].item.label
	})
	out := make([]sourceCandidate, len(ranks))
	for i, r := range ranks {
		it := r.item
		// ★ 门槛取 0.99 而非 1:BumpUsage 落盘时 LastUsed 截断到整 Unix 秒,
		// 而读取用带亚秒的 now 惰性衰减,刚用过一次的词读回 0.9999x < 1
		// (7 天半衰期下的亚秒幻影衰减),严格 >= 1 会让 ★ 在新机器上永不点亮。
		// 0.99 只影响 ★ 点亮门槛,不影响排序本身。
		if mark && r.score >= 0.99 {
			it.label = "★ " + it.label
		}
		out[i] = it
	}
	return out
}

// keywordHistKey 高亮/隐藏词频 key:kind+scope+":"+词(scope 空即全局)。
func keywordHistKey(kind, scope, word string) string {
	return kind + scope + ":" + word
}

// sortedUsageWords 按 kind+scope 前缀捞词条,衰减分降序、同分 LastUsed 新→旧、
// 再同则词序升序,剥前缀返回词,截前 limit(C-r 高亮/隐藏历史列表数据源)。
func sortedUsageWords(kind, scope string, limit int) []string {
	m := usage.Load()
	prefix := kind + scope + ":"
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
	if len(rs) > limit {
		rs = rs[:limit]
	}
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.word
	}
	return out
}
