// 行文本高亮/命中分段（纯函数）：把一行文本切成带标记的片段数组，
// 高亮词（hl=颜色，亮色粗体下划线域）与搜索命中词（hit=true，暗色底纹域）
// 双域视觉区分（沿用 TUI v0.19 视觉语言）。

export interface Span {
  text: string
  /** hl 高亮词颜色（仅高亮域） */
  hl?: string
  /** hit 搜索命中标记（仅命中域） */
  hit?: boolean
}

interface Marker {
  start: number
  end: number
  hl?: string
  hit?: boolean
}

/**
 * splitSpans 把 text 按 keywords/hits 切段：
 * - 高亮词带各自颜色；命中词带 hit 标记
 * - 重叠取先（更早开始的占位，后者被截断）；同位置高亮词优先
 * - 命中词多形态匹配：命中词含空白时按原文 indexOf，否则大小写不敏感
 */
export function splitSpans(
  text: string,
  highlights: { word: string; color: string }[],
  hits: Set<string>
): Span[] {
  if (text === '') return []
  const markers: Marker[] = []
  for (const h of highlights) {
    if (!h.word) continue
    pushAll(markers, text, h.word, { hl: h.color }, false)
  }
  for (const w of hits) {
    if (!w) continue
    pushAll(markers, text, w, { hit: true }, true)
  }
  if (markers.length === 0) return [{ text }]
  // 按 (start, 高亮优先) 排序后贪心取不重叠段
  markers.sort((a, b) => a.start - b.start || (b.hl ? 1 : 0) - (a.hl ? 1 : 0))
  const taken: Marker[] = []
  let cursor = 0
  for (const m of markers) {
    if (m.start >= cursor) {
      taken.push(m)
      cursor = m.end
    }
  }
  // 输出分段：未标记间隙 + 标记段
  const out: Span[] = []
  let pos = 0
  for (const m of taken) {
    if (m.start > pos) out.push({ text: text.slice(pos, m.start) })
    const span: Span = { text: text.slice(m.start, m.end) }
    if (m.hl) span.hl = m.hl
    if (m.hit) span.hit = true
    out.push(span)
    pos = m.end
  }
  if (pos < text.length) out.push({ text: text.slice(pos) })
  return out
}

/** pushAll 收集 word 在 text 中的全部出现（大小写不敏感时用降级 indexOf 扫描）。 */
function pushAll(
  markers: Marker[],
  text: string,
  word: string,
  tag: Pick<Marker, 'hl' | 'hit'>,
  caseInsensitive: boolean
): void {
  const t = caseInsensitive ? text.toLowerCase() : text
  const w = caseInsensitive ? word.toLowerCase() : word
  let i = t.indexOf(w)
  while (i !== -1) {
    markers.push({ start: i, end: i + w.length, ...tag })
    i = t.indexOf(w, i + w.length)
  }
}
