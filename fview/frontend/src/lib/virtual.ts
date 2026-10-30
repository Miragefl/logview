// 虚拟滚动可视区间计算（纯函数）：固定行高模型，上下各扩 overscan 行。
// 越界语义：start/end 均钳到 [0, total]；裁头导致 scrollTop 出界时输出空区间，
// 由滚动层（watch total）修正 scrollTop。

/**
 * visibleRange 返回可视行区间 [start, end)（含 overscan 缓冲）。
 * - scrollTop: 当前滚动偏移（px）
 * - viewportH: 视口高（px）
 * - rowH: 固定单行高（px）；≤0 视为异常返回全量
 * - total: 当前行总数（镜像裁头后以当前数组为准）
 */
export function visibleRange(
  scrollTop: number,
  viewportH: number,
  rowH: number,
  total: number,
  overscan = 10
): [number, number] {
  if (total <= 0) return [0, 0]
  if (rowH <= 0) return [0, total]
  const start = Math.min(Math.max(0, Math.floor(scrollTop / rowH) - overscan), total)
  const rawEnd = Math.ceil((scrollTop + viewportH) / rowH) + overscan
  const end = Math.min(total, Math.max(rawEnd, start))
  return [start, end]
}
