// 可视选择区间测试(验收反馈④ v/V/y):seq 域区间,锚/光标双向,折叠占位行天然不进区间。
import { describe, expect, it } from 'vitest'
import { selectedSeqRange } from './visualSel'

// seq 3-9 为折叠组成员,视图只剩 [1,2,10]
const SEQ = [1, 2, 10]

describe('selectedSeqRange', () => {
  it('锚 < 光标 → 正序区间', () => {
    expect(selectedSeqRange(SEQ, 1, 10)).toEqual({ lo: 1, hi: 10 })
  })

  it('锚 > 光标 → 逆序同样覆盖', () => {
    expect(selectedSeqRange(SEQ, 10, 1)).toEqual({ lo: 1, hi: 10 })
  })

  it('锚 = 光标 → 单行区间', () => {
    expect(selectedSeqRange(SEQ, 2, 2)).toEqual({ lo: 2, hi: 2 })
  })

  it('跨折叠组的两可见行:区间覆盖可见行,折叠占位行(seq 3-9)不在域内', () => {
    const r = selectedSeqRange(SEQ, 2, 10)
    expect(r).toEqual({ lo: 2, hi: 10 })
    // 区间内可见行只有 2 与 10(3-9 被折叠隐藏,不参与)
    expect(SEQ.filter((s) => s >= r!.lo && s <= r!.hi)).toEqual([2, 10])
  })

  it('锚或光标不在可见域 → null', () => {
    expect(selectedSeqRange(SEQ, 5, 10)).toBeNull()
    expect(selectedSeqRange(SEQ, 1, 99)).toBeNull()
  })
})
