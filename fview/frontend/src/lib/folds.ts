// 折叠视图模型(验收反馈④ e 键):过滤后行 + 折叠组(seq 域)→ 视图项序列。
// 语义(设计既定近似):组折叠时渲染单个占位项替代全组;组内可见行 ≤1
// (leader 被过滤剩孤行)时自然展开;全部按 seq 键控,repaint 同 seq 替换不漂移。
import type { LineDTO } from './types'

export interface FoldRange {
  startSeq: number
  endSeq: number
  leader: string
}

export type ViewItem =
  | { type: 'line'; ln: LineDTO }
  | { type: 'fold'; range: FoldRange }

/**
 * buildViewItems 组装视图项:
 * - lines 必须升序(过滤视图天然有序);folds/collapsed 均按 seq 键控
 * - 单遍扫描:折叠组按 startSeq 排序后指针推进,O(lines + folds)
 */
export function buildViewItems(
  lines: LineDTO[],
  folds: FoldRange[],
  collapsed: number[]
): ViewItem[] {
  if (!folds.length || !collapsed.length) {
    return lines.map((ln) => ({ type: 'line', ln }) as ViewItem)
  }
  const col = new Set(collapsed)
  const active = folds.filter((f) => col.has(f.startSeq)).sort((a, b) => a.startSeq - b.startSeq)
  if (!active.length) {
    return lines.map((ln) => ({ type: 'line', ln }) as ViewItem)
  }
  // 第一遍:数每个折叠组在过滤视图中的可见行数(决定折叠 or 自然展开)
  const counts = new Map<number, number>()
  let fi = 0
  for (const ln of lines) {
    while (fi < active.length && ln.seq > active[fi].endSeq) fi++
    const f = active[fi]
    if (f && ln.seq >= f.startSeq) {
      counts.set(f.startSeq, (counts.get(f.startSeq) ?? 0) + 1)
    }
  }
  // 第二遍:leader 位置出占位(可见行>1 时),成员跳过;其余行直出
  const out: ViewItem[] = []
  let fj = 0
  for (const ln of lines) {
    while (fj < active.length && ln.seq > active[fj].endSeq) fj++
    const f = active[fj]
    if (!f || ln.seq < f.startSeq) {
      out.push({ type: 'line', ln })
      continue
    }
    if ((counts.get(f.startSeq) ?? 0) <= 1) {
      out.push({ type: 'line', ln }) // 孤行自然展开
      continue
    }
    if (ln.seq === f.startSeq) {
      out.push({ type: 'fold', range: f }) // 占位替代全组
    }
    // 组内成员跳过
  }
  return out
}
