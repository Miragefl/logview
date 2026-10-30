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
