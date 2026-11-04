// 上下文混入(移植自 TUI internal/tui buildCtxLines,2026-11-04 桌面版):
// 过滤视图 + 多锚集合 → 混合行序列。语义对齐 TUI 最终版(sticky 多锚):
// - +N 在锚行下方插入原始流 (ai, ai+N] 区间内未命中过滤的行,-N 在上方 [ai-N, ai);
//   方向语义 = TUI 2026-09-14 二修换向(+=往后/下侧,-=往前/上侧)
// - 锚 seq 键控(TUI 用 *ParsedLine 指针键;fview 行对象 repaint 会替换,指针键必漂移)
// - 命中过滤的行不插(它已在视图);窗口按 seq 距离,镜像环形裁头自然 clamp
// - 死锚(seq 不在镜像/不在过滤视图)静默跳过——过滤变化由调用方 watch 清锚兜底
import type { LineDTO } from './types'

/** CtxAnchor 单锚参数:before=true 往前(上侧),n 行数(调用方已钳 1..1000)。 */
export interface CtxAnchor {
  before: boolean
  n: number
}

/** buildCtxMixed 组装混入视图:filtered/all 均须 seq 升序(镜像天然有序)。
 * 返回归并后的升序行序列 + 混入行 seq 集合(dimSeqs,渲染层整行暗色+┆ 标记)。
 * 空锚走快速路径原样返回 filtered 引用(热路径每帧重算,不拷贝)。 */
export function buildCtxMixed(
  filtered: LineDTO[],
  all: LineDTO[],
  anchors: Record<number, CtxAnchor>
): { lines: LineDTO[]; dimSeqs: Set<number> } {
  const keys = Object.keys(anchors)
  if (keys.length === 0) return { lines: filtered, dimSeqs: new Set<number>() }

  const filteredSeqs = new Set(filtered.map((l) => l.seq))
  const idx = new Map<number, number>(all.map((l, i) => [l.seq, i]))
  const dim = new Set<number>()
  for (const k of keys) {
    const seq = Number(k)
    const ai = idx.get(seq)
    // 死锚/防御锚(不在镜像或不在过滤视图):跳过
    if (ai === undefined || !filteredSeqs.has(seq)) continue
    const a = anchors[seq]!
    const from = a.before ? Math.max(0, ai - a.n) : ai + 1
    const to = a.before ? ai : Math.min(all.length, ai + 1 + a.n)
    for (let i = from; i < to; i++) {
      const s = all[i].seq
      if (!filteredSeqs.has(s)) dim.add(s) // 命中行已在视图,不插不重复
    }
  }
  if (dim.size === 0) return { lines: filtered, dimSeqs: dim }

  // filtered 与 dim 行皆升序 → 双指针归并 O(m+d),不用 sort
  const dimLines = all.filter((l) => dim.has(l.seq))
  const lines: LineDTO[] = []
  let i = 0
  let j = 0
  while (i < filtered.length || j < dimLines.length) {
    if (j >= dimLines.length || (i < filtered.length && filtered[i].seq < dimLines[j].seq)) {
      lines.push(filtered[i++])
    } else {
      lines.push(dimLines[j++])
    }
  }
  return { lines, dimSeqs: dim }
}
