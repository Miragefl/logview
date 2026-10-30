// 换行引擎(验收反馈④ w):wrapSegments 按 displayWidth 切段(复用 columns 的
// CJK 计宽);HeightIndex 前缀和 + 二分,支撑变高行虚拟滚动。
// 渲染层用 CSS pre-wrap 实际折行,宽度按 charPx 留 1 字符余量防测量误差溢出。
import { displayWidth } from './columns'

/** wrapSegments 文本按 widthCh 显示宽切行;宽 2 字符不切半,整字下移。 */
export function wrapSegments(text: string, widthCh: number): string[] {
  if (widthCh <= 0) return [text]
  if (text === '') return ['']
  const segs: string[] = []
  let cur = ''
  let curW = 0
  for (const ch of text) {
    const w = displayWidth(ch)
    if (curW + w > widthCh) {
      segs.push(cur)
      cur = ch
      curW = w
    } else {
      cur += ch
      curW += w
    }
  }
  if (cur !== '' || segs.length === 0) segs.push(cur)
  return segs
}

/** HeightIndex 变高行索引:heights[i] 为第 i 行的视觉行数(单位行高 rowH)。 */
export class HeightIndex {
  readonly rowH: number
  private readonly heights: number[]
  private readonly prefix: number[] // prefix[i] = 前 i 行总高(px)

  constructor(heights: number[], rowH: number) {
    this.rowH = rowH
    this.heights = heights
    this.prefix = [0]
    for (const h of heights) this.prefix.push(this.prefix[this.prefix.length - 1] + h * rowH)
  }

  get count(): number {
    return this.heights.length
  }

  total(): number {
    return this.prefix[this.prefix.length - 1]
  }

  offsetOf(i: number): number {
    if (i <= 0) return 0
    if (i >= this.count) return this.total()
    return this.prefix[i]
  }

  /** rowsAt 第 i 行视觉行数(高度模型)。 */
  rowsAt(i: number): number {
    return this.heights[i] ?? 1
  }

  /** locate offsetPx 所在行索引(行顶 ≤ offsetPx < 行底),越界钳制。 */
  locate(offsetPx: number): number {
    if (this.count <= 0) return 0
    if (offsetPx <= 0) return 0
    let lo = 0
    let hi = this.count - 1
    let ans = 0
    while (lo <= hi) {
      const mid = (lo + hi) >> 1
      if (this.prefix[mid] <= offsetPx) {
        ans = mid
        lo = mid + 1
      } else {
        hi = mid - 1
      }
    }
    return ans
  }
}

/** visibleRows 变高可视区 [start, end):按高度覆盖 viewport + overscan 行。 */
export function visibleRows(
  hi: HeightIndex,
  offsetPx: number,
  viewportH: number,
  overscan = 10
): [number, number] {
  if (hi.count <= 0) return [0, 0]
  const start = Math.max(0, hi.locate(offsetPx) - overscan)
  const limit = offsetPx + viewportH + overscan * hi.rowH
  let end = start
  while (end < hi.count && hi.offsetOf(end) < limit) end++
  return [start, Math.max(end, start + 1)]
}
