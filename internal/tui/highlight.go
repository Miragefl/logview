// 子序列匹配与高亮切分(tui-picker-unify):从 fview frontend/src/lib/highlight.ts
// 逐用例移植(r3 算法)——fzf 式:hde→hardware 命中(h…d…e 按序出现);子串优先
// (完整子串命中→整段连续下标,防贪心把 'my app' 的 m 吃到错位);大小写不敏感;
// 空词恒真/identity、不 trim(空格是路径/主机名合法字符)。双端同表测试
// (highlight_test.go ↔ highlight.test.ts)锁语义零漂移。
package tui

import "strings"

// HLSeg 高亮切分段:Hit 命中(渲染层包 accent 样式)。
type HLSeg struct {
	T   string
	Hit bool
}

// matchPositions 命中位置(rune 下标;nil=不命中;空词/全空格词返回空切片=恒真)。
// isSubseq 与 splitHighlight 共用——过滤与高亮见到的命中位置必须一致。
func matchPositions(text, kw string) []int {
	k := strings.ToLower(kw)
	if strings.TrimSpace(k) == "" {
		return []int{}
	}
	lower := strings.ToLower(text)
	// 子串优先(字节 Index 定位→换算 rune 下标):连续命中是子集,视觉不变
	if at := strings.Index(lower, k); at >= 0 {
		kr := []rune(k)
		base := len([]rune(lower[:at]))
		pos := make([]int, len(kr))
		for i := range pos {
			pos[i] = base + i
		}
		return pos
	}
	// 子序列贪心(fzf 式非连续)
	tr, kr := []rune(lower), []rune(k)
	var pos []int
	i := 0
	for j := 0; j < len(tr) && i < len(kr); j++ {
		if tr[j] == kr[i] {
			pos = append(pos, j)
			i++
		}
	}
	if i == len(kr) {
		return pos
	}
	return nil
}

// isSubseq 子序列判定(候选过滤谓词):kw 各字符按序出现即命中;空词恒真。
func isSubseq(text, kw string) bool {
	return matchPositions(text, kw) != nil
}

// splitHighlight 子序列高亮切分(保留原文大小写):命中字符标 Hit,相邻命中合并
// 成段;词空 identity;不命中整体 miss。
func splitHighlight(text, kw string) []HLSeg {
	pos := matchPositions(text, kw)
	tr := []rune(text)
	if pos == nil {
		return []HLSeg{{T: text, Hit: false}}
	}
	hits := make([]bool, len(tr))
	for _, j := range pos {
		hits[j] = true
	}
	var segs []HLSeg
	start := 0
	for j := 1; j <= len(tr); j++ {
		if j == len(tr) || hits[j] != hits[start] {
			segs = append(segs, HLSeg{T: string(tr[start:j]), Hit: hits[start]})
			start = j
		}
	}
	return segs
}
