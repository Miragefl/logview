// 字符光标移动纯函数(vim-copy):message 消息段 rune 域移动/选区子串/wrap 段投影。
// 语义对齐 vim 简化版——字母/数字/下划线(含 CJK 等 unicode 字母数字)为词字符,
// 连续标点为一词,空白跳过;消息首尾停住(不跨行接续,spec 拍板)。
// 宽度(displayWidth/CJK 2 列)是渲染层的事,本模块只用 rune index。
// Go 侧 internal/tui/charnav.go 与本文件逐用例对齐(m*/s*/w* 同表)。
export type Motion =
  | 'left'
  | 'right'
  | 'wordFwd'
  | 'wordBack'
  | 'wordEnd'
  | 'start'
  | 'firstNonBlank'
  | 'end'

type RuneClass = 'word' | 'punct' | 'space'

/** classOf 字符三分类:词/标点/空白。 */
function classOf(r: string): RuneClass {
  if (/\s/.test(r)) return 'space'
  if (isWordRune(r)) return 'word'
  return 'punct'
}

/** isWordRune vim iskeyword 简化:字母/数字/下划线(unicode)。 */
export function isWordRune(r: string): boolean {
  return r === '_' || /[\p{L}\p{N}]/u.test(r)
}

/** applyMotion 消息内 rune 列移动;空消息返回 0,结果 clamp 到 [0, len-1]。 */
export function applyMotion(msg: string, col: number, m: Motion): number {
  const r = Array.from(msg)
  const n = r.length
  if (n === 0) return 0
  const c = Math.max(0, Math.min(n - 1, col))
  switch (m) {
    case 'left':
      return Math.max(0, c - 1)
    case 'right':
      return Math.min(n - 1, c + 1)
    case 'start':
      return 0
    case 'firstNonBlank': {
      for (let i = 0; i < n; i++) if (classOf(r[i]) !== 'space') return i
      return 0 // 全空白行回行首
    }
    case 'end':
      return n - 1
    case 'wordFwd': {
      // 跳过当前同类段(空白上则直接跳空白),落下一词首;到末尾停住
      let i = c
      const cls = classOf(r[i])
      if (cls !== 'space') while (i < n && classOf(r[i]) === cls) i++
      while (i < n && classOf(r[i]) === 'space') i++
      return Math.min(i, n - 1)
    }
    case 'wordBack': {
      // 词中(左邻同类)退当前词首;词首/空白退到前一词首
      let i = c
      if (i > 0 && classOf(r[i]) !== 'space' && classOf(r[i - 1]) === classOf(r[i])) {
        while (i > 0 && classOf(r[i - 1]) === classOf(r[i])) i--
        return i
      }
      while (i > 0 && classOf(r[i - 1]) === 'space') i--
      if (i === 0) return 0
      let j = i - 1
      const cls = classOf(r[j])
      while (j > 0 && classOf(r[j - 1]) === cls) j--
      return j
    }
    case 'wordEnd': {
      // 在词尾(下一字符异类/末尾)→ 跳过空白到下一词再取其尾;否则取当前词尾
      let i = c
      const cls = classOf(r[i])
      if (cls !== 'space' && (i === n - 1 || classOf(r[i + 1]) !== cls)) {
        i++
        while (i < n && classOf(r[i]) === 'space') i++
      }
      const c2 = i < n ? classOf(r[i]) : 'space'
      while (i < n - 1 && classOf(r[i + 1]) === c2) i++
      return Math.min(i, n - 1)
    }
  }
}

/** charSelText 字符选区子串:msgs 按行序(锚..光标已归一为首..尾),
 *  首行 [startCol..]、中间行全量、尾行 [..endCol](端点 inclusive,列 clamp 行内);
 *  chars 为 text 的 rune 计数(含换行)。 */
export function charSelText(
  msgs: string[],
  startCol: number,
  endCol: number
): { text: string; chars: number } {
  if (msgs.length === 0) return { text: '', chars: 0 }
  const sub = (s: string, from: number, to: number | undefined) => {
    const r = Array.from(s)
    const f = Math.max(0, Math.min(r.length - 1 < 0 ? 0 : r.length - 1, from))
    const t = to === undefined ? r.length - 1 : Math.max(f, Math.min(r.length - 1, to))
    return r.slice(f, t + 1).join('')
  }
  const parts = msgs.map((s, i) => {
    if (i === 0 && i === msgs.length - 1) return sub(s, startCol, endCol)
    if (i === 0) return sub(s, startCol, undefined)
    if (i === msgs.length - 1) return sub(s, 0, endCol)
    return s
  })
  const text = parts.join('\n')
  return { text, chars: Array.from(text).length }
}

/** colToSeg wrap 段投影:rune col → 第几段+段内 rune 偏移;col 越界 clamp 末段尾。 */
export function colToSeg(segs: string[], col: number): { segIdx: number; colInSeg: number } {
  if (segs.length === 0) return { segIdx: 0, colInSeg: 0 }
  let acc = 0
  for (let i = 0; i < segs.length; i++) {
    const len = Array.from(segs[i]).length
    if (i === segs.length - 1 || col < acc + len) {
      const inSeg = Math.max(0, Math.min(len - 1, col - acc))
      return { segIdx: i, colInSeg: inSeg }
    }
    acc += len
  }
  return { segIdx: segs.length - 1, colInSeg: 0 }
}
