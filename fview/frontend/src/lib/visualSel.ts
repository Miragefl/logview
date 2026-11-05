// 可视选择区间(验收反馈④ v/V/y):锚与光标都在可见行域(seq 升序数组)时
// 返回覆盖区间;折叠占位行不在域内,天然不参与选择。
export interface SeqRange {
  lo: number
  hi: number
}

/** selectedSeqRange 可见 seq 数组内取锚/光标区间;任一不在域内返回 null。 */
export function selectedSeqRange(
  lineSeqs: number[],
  anchorSeq: number,
  cursorSeq: number
): SeqRange | null {
  const ai = lineSeqs.indexOf(anchorSeq)
  const ci = lineSeqs.indexOf(cursorSeq)
  if (ai < 0 || ci < 0) return null
  const lo = Math.min(ai, ci)
  const hi = Math.max(ai, ci)
  return { lo: lineSeqs[lo], hi: lineSeqs[hi] }
}

/** CharSelRow 字符选区单行 rune 列区间(from/to 含端点;to=-1 到行尾)。 */
export interface CharSelRow {
  from: number
  to: number
}

/** charSelRows 字符选区逐行区间(vim-copy):锚/光标带列,行序小端为首行;
 *  首行 [端列..]、中间行全量、尾行 [..端列];锚/光标行序任意(自动归一);
 *  任一端不在可见行域返回 null(折叠隐藏行不进选区,所见即所选)。 */
export function charSelRows(
  lineSeqs: number[],
  anchorSeq: number,
  anchorCol: number,
  cursorSeq: number,
  cursorCol: number
): Map<number, CharSelRow> | null {
  const ai = lineSeqs.indexOf(anchorSeq)
  const ci = lineSeqs.indexOf(cursorSeq)
  if (ai < 0 || ci < 0) return null
  const fwd = ai <= ci
  const first = { seq: lineSeqs[Math.min(ai, ci)], col: fwd ? anchorCol : cursorCol }
  const last = { seq: lineSeqs[Math.max(ai, ci)], col: fwd ? cursorCol : anchorCol }
  const m = new Map<number, CharSelRow>()
  for (let i = Math.min(ai, ci); i <= Math.max(ai, ci); i++) {
    const seq = lineSeqs[i]
    if (seq === first.seq && seq === last.seq) m.set(seq, { from: Math.min(first.col, last.col), to: Math.max(first.col, last.col) })
    else if (seq === first.seq) m.set(seq, { from: first.col, to: -1 })
    else if (seq === last.seq) m.set(seq, { from: 0, to: last.col })
    else m.set(seq, { from: 0, to: -1 })
  }
  return m
}
