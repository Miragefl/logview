// 光标导航纯函数(验收反馈④):钳制移动 + scrolloff 可视性保证(TUI 手感)。

/** moveClamp 光标移动并钳制到 [0, total-1];total≤0 返回 -1(空列表)。 */
export function moveClamp(idx: number, delta: number, total: number): number {
  if (total <= 0) return -1
  return Math.min(total - 1, Math.max(0, idx + delta))
}

/**
 * ensureVisible 光标所在行保持在可视区 scrolloff 圈内,返回应有的首行索引:
 * - 光标在 [first+scrolloff, first+rows-1-scrolloff] 舒适区内 → 不动
 * - 侵入上圈 → first = idx - scrolloff;触下边缘 → first = idx - rows + 1 + scrolloff
 * - 结果钳制到 [0, max(0, total-rowsInView)]
 */
export function ensureVisible(
  idx: number,
  first: number,
  rowsInView: number,
  total: number,
  scrolloff = 3
): number {
  if (total <= 0 || rowsInView <= 0) return 0
  const last = first + rowsInView - 1
  if (idx >= first + scrolloff && idx <= last - scrolloff) return first
  let nf: number
  if (idx < first + scrolloff) {
    nf = idx - scrolloff
  } else {
    nf = idx - rowsInView + 1 + scrolloff
  }
  const maxFirst = Math.max(0, total - rowsInView)
  return Math.min(maxFirst, Math.max(0, nf))
}
