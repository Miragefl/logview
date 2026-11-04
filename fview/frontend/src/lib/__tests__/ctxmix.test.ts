// 上下文混入纯函数测试(移植自 TUI grep-context buildCtxLines):
// 方向(+往后/-往前)/裁头 clamp/多锚去重/同锚覆盖/死锚跳过/命中行不插/空锚 identity。
import { describe, expect, it } from 'vitest'
import { buildCtxMixed } from '../ctxmix'
import type { LineDTO } from '../types'

function ln(seq: number): LineDTO {
  return { seq, time: '', level: 'INFO', thread: '', traceId: '', logger: '', message: `l${seq}`, raw: '' }
}

/** rng 造 seq 升序行数组 [from, to)。 */
function rng(from: number, to: number): LineDTO[] {
  return Array.from({ length: to - from }, (_, i) => ln(from + i))
}

const seqs = (ls: LineDTO[]) => ls.map((l) => l.seq)

describe('buildCtxMixed 上下文混入', () => {
  it('+N 往后:锚行下方插入原始流未命中行,dim 标记,整体升序', () => {
    // all 0..10,过滤只剩 {5},锚 5 +2 → 5 下面插 6/7
    const r = buildCtxMixed(rng(0, 11).filter((l) => l.seq === 5), rng(0, 11), { 5: { before: false, n: 2 } })
    expect(seqs(r.lines)).toEqual([5, 6, 7])
    expect([...r.dimSeqs]).toEqual([6, 7])
  })

  it('-N 往前:锚行上方插入(TUI 二修换向语义:+=往后/下,-=往前/上)', () => {
    const r = buildCtxMixed(rng(0, 11).filter((l) => l.seq === 5), rng(0, 11), { 5: { before: true, n: 2 } })
    expect(seqs(r.lines)).toEqual([3, 4, 5])
    expect([...r.dimSeqs]).toEqual([3, 4])
  })

  it('裁头 clamp:锚靠头部窗口不足 N 行,取到多少插多少', () => {
    const r = buildCtxMixed(rng(0, 11).filter((l) => l.seq === 1), rng(0, 11), { 1: { before: true, n: 5 } })
    // [1-5,1) 负区间 clamp 到 0 → 只插 seq 0
    expect(seqs(r.lines)).toEqual([0, 1])
  })

  it('多锚累积:不同行各自窗口互不清除,重叠 dim 行去重', () => {
    // 过滤剩 {5,9},锚 5 +3(窗口 6/7/8)、锚 9 -3(窗口 6/7/8)→ 重叠只插一次
    const r = buildCtxMixed(
      rng(0, 11).filter((l) => l.seq === 5 || l.seq === 9),
      rng(0, 11),
      { 5: { before: false, n: 3 }, 9: { before: true, n: 3 } }
    )
    expect(seqs(r.lines)).toEqual([5, 6, 7, 8, 9])
    expect(r.dimSeqs.size).toBe(3)
  })

  it('命中过滤的行不插:窗口内 seq 已在过滤视图 → 不 dim 不重复', () => {
    // 过滤剩 {5,7},锚 5 +3 窗口 6/7/8 → 7 已可见不插,只插 6/8
    const r = buildCtxMixed(
      rng(0, 11).filter((l) => l.seq === 5 || l.seq === 7),
      rng(0, 11),
      { 5: { before: false, n: 3 } }
    )
    expect(seqs(r.lines)).toEqual([5, 6, 7, 8])
    expect([...r.dimSeqs]).toEqual([6, 8])
  })

  it('死锚跳过:锚 seq 不在镜像(裁剪/换源)→ 静默无操作', () => {
    const r = buildCtxMixed([ln(5)], rng(0, 3), { 99: { before: false, n: 2 } })
    expect(seqs(r.lines)).toEqual([5])
    expect(r.dimSeqs.size).toBe(0)
  })

  it('锚不在过滤视图(过滤已变的防御)→ 跳过该锚', () => {
    const r = buildCtxMixed([ln(5)], rng(0, 11), { 3: { before: false, n: 2 } })
    expect(seqs(r.lines)).toEqual([5])
    expect(r.dimSeqs.size).toBe(0)
  })

  it('空锚:lines 原样返回,dimSeqs 空', () => {
    const filtered = rng(2, 5)
    const r = buildCtxMixed(filtered, rng(0, 11), {})
    expect(r.lines).toBe(filtered) // 引用不拷贝(热路径每帧重算)
    expect(r.dimSeqs.size).toBe(0)
  })
})
