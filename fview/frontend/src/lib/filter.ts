// 级别过滤/隐藏词/高亮区间（语义逐条对齐 TUI：matchLevelFilter app.go:1152、
// matchHides search_model.go:86——E/W/I/D/A 档位与忽略大小写包含）

export type LevelFilterMode = 'E' | 'W' | 'I' | 'D' | 'A'

const ERR_LV = ['ERROR', 'ERR', 'FATAL']
const WARN_LV = ['WARN', 'WARNING']
const DEBUG_LV = ['DEBUG', 'DBG']

/** levelFilter 判断级别在档位下是否保留显示。 */
export function levelFilter(mode: LevelFilterMode, level: string): boolean {
  if (mode === 'D' || mode === 'A') return true
  const lv = level.toUpperCase()
  const isErr = ERR_LV.includes(lv)
  if (mode === 'E') return isErr
  const isWarn = WARN_LV.includes(lv)
  if (mode === 'W') return isErr || isWarn
  if (mode === 'I') return isErr || isWarn || lv === 'INFO'
  return true
}

/** hidesFilter 隐藏词过滤：raw 文本忽略大小写命中任一词即隐藏（返回 false）。 */
export function hidesFilter(hides: string[], raw: string): boolean {
  if (!hides.length) return true
  const lower = raw.toLowerCase()
  return !hides.some((kw) => kw && lower.includes(kw.toLowerCase()))
}

/** highlightRanges 找出各关键词在文本中的全部出现区间（升序，忽略大小写）。 */
export function highlightRanges(
  text: string,
  words: string[]
): Array<[number, number]> {
  const lower = text.toLowerCase()
  const ranges: Array<[number, number]> = []
  for (const w of words) {
    if (!w) continue
    const lw = w.toLowerCase()
    let i = lower.indexOf(lw)
    while (i >= 0) {
      ranges.push([i, i + w.length])
      i = lower.indexOf(lw, i + w.length)
    }
  }
  return ranges.sort((a, b) => a[0] - b[0])
}
