// 热度排序纯函数(panel-hot-sort):排序语义对齐 TUI internal/tui/usage.go
// sortCandidatesHot——衰减分降序、同分名称升序;isHot ★ 阈值 0.99(TUI 亚秒
// 幻影衰减口径,Bump 落盘截整秒而读取带亚秒,严格 ≥1 会永不点亮)。

/** hotSorted 返回热度排序新数组:分降序,同分/无分(缺失或 0)按 key 升序,stable。 */
export function hotSorted<T>(items: readonly T[], keyOf: (it: T) => string, scores: Record<string, number>): T[] {
  return [...items].sort((a, b) => {
    const sa = scores[keyOf(a)] ?? 0
    const sb = scores[keyOf(b)] ?? 0
    if (sa !== sb) return sb - sa
    return keyOf(a) < keyOf(b) ? -1 : keyOf(a) > keyOf(b) ? 1 : 0
  })
}

/** isHot ★ 常用标记判定(与 TUI sortCandidatesHot 同阈值)。 */
export function isHot(score: number | undefined): boolean {
  return (score ?? 0) >= 0.99
}
