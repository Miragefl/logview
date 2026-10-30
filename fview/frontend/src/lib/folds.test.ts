// 折叠视图模型测试(验收反馈④ e 键):折叠组渲染占位、过滤后近似展开、seq 键控。
import { describe, expect, it } from 'vitest'
import { buildViewItems } from './folds'
import type { LineDTO } from './types'
import type { FoldRange } from './folds'

function ln(seq: number, message: string, level = 'ERROR'): LineDTO {
  return { seq, time: '', level, thread: '', traceId: '', logger: '', message, raw: message }
}

const FOLD: FoldRange = { startSeq: 2, endSeq: 5, leader: 'java.lang.Exception: boom' }
// 组:seq2 leader,seq3-5 帧;组外 seq1/seq6 普通行
const LINES = [
  ln(1, 'INFO before', 'INFO'),
  ln(2, 'java.lang.Exception: boom'),
  ln(3, '\tat a.B(B.java:1)'),
  ln(4, '\tat a.C(C.java:2)'),
  ln(5, '\tat a.D(D.java:3)'),
  ln(6, 'INFO after', 'INFO'),
]

describe('buildViewItems', () => {
  it('未折叠:全行直出', () => {
    const items = buildViewItems(LINES, [FOLD], [])
    expect(items).toHaveLength(6)
    expect(items.every((i) => i.type === 'line')).toBe(true)
  })

  it('折叠组渲染单个占位项,组内成员不出现', () => {
    const items = buildViewItems(LINES, [FOLD], [2])
    expect(items).toHaveLength(3)
    expect(items[1]).toEqual({ type: 'fold', range: FOLD })
    const seqs = items.filter((i) => i.type === 'line').map((i) => (i as { ln: LineDTO }).ln.seq)
    expect(seqs).toEqual([1, 6])
  })

  it('组内可见行 ≤1(级别过滤后只剩 leader)→ 自然展开,不出占位', () => {
    const only = [LINES[0], LINES[1], LINES[5]] // 帧被过滤
    const items = buildViewItems(only, [FOLD], [2])
    expect(items.every((i) => i.type === 'line')).toBe(true)
    expect(items).toHaveLength(3)
  })

  it('leader 被过滤但成员 ≥2 可见 → 自然展开,成员直出不消失(Final #3)', () => {
    // leader(seq2)被隐藏词滤掉,帧 seq3/5 可见(seq4 也被滤)
    const filtered = [LINES[0], LINES[2], LINES[4], LINES[5]]
    const items = buildViewItems(filtered, [FOLD], [2])
    expect(items.every((i) => i.type === 'line')).toBe(true)
    expect(items).toHaveLength(4) // 成员一个不丢
    expect(items.some((i) => i.type === 'fold')).toBe(false)
  })

  it('repaint 场景:lines 换成同内容新数组,输出 seq 序列不变(Review Focus ②)', () => {
    const a = buildViewItems(LINES, [FOLD], [2])
    const b = buildViewItems(LINES.map((l) => ({ ...l })), [FOLD], [2])
    const seqs = (xs: typeof a) =>
      xs.map((i) => (i.type === 'line' ? i.ln.seq : `f${i.range.startSeq}`))
    expect(seqs(b)).toEqual(seqs(a))
  })
})
