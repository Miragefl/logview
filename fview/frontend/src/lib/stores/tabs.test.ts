import { describe, expect, it } from 'vitest'
import { addLines, type LineDTO } from './tabs'

function ln(seq: number, msg?: string): LineDTO {
  return { seq, time: '', level: '', thread: '', traceId: '', logger: '', message: msg ?? `m${seq}`, raw: `raw-${seq}` }
}

describe('addLines', () => {
  it('递增批次直接追加（快路径）', () => {
    const out = addLines([ln(1), ln(2)], [ln(3), ln(4)])
    expect(out.map((l) => l.seq)).toEqual([1, 2, 3, 4])
  })

  it('乱序批次按 Seq 插入正确位置（repaint 部分重发）', () => {
    const base = [ln(1), ln(2), ln(5), ln(6)]
    const out = addLines(base, [ln(3), ln(4)])
    expect(out.map((l) => l.seq)).toEqual([1, 2, 3, 4, 5, 6])
  })

  it('同 Seq 行替换而非重复（repaint 全量重发语义）', () => {
    const base = [ln(1, 'old'), ln(2, 'old')]
    const out = addLines(base, [ln(1, 'new'), ln(2, 'new')])
    expect(out).toHaveLength(2)
    expect(out[0].message).toBe('new')
    expect(out[1].message).toBe('new')
  })

  it('超过 maxLines 裁掉头部旧行（对齐后端 RingBuffer 容量）', () => {
    const base = Array.from({ length: 8 }, (_, i) => ln(i + 1)) // seq 1..8
    const batch = Array.from({ length: 5 }, (_, i) => ln(9 + i)) // seq 9..13
    const out = addLines(base, batch, 10)
    expect(out).toHaveLength(10)
    expect(out[0].seq).toBe(4) // 1..3 被淘汰
    expect(out[out.length - 1].seq).toBe(13)
  })
})
