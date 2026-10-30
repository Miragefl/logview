import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { addLines, useTabsStore } from './tabs'
import type { LineDTO } from '../lib/types'

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

describe('useTabsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('applyLines 合并行流并同步行数（tab:lines 事件）', () => {
    const s = useTabsStore()
    s.applyOpened({ tabId: 't1', label: 'app.log' })
    s.applyLines({ tabId: 't1', lines: [ln(1), ln(3)] })
    s.applyLines({ tabId: 't1', lines: [ln(2)] }) // 乱序补发
    const t = s.tabs.find((x) => x.info.id === 't1')
    expect(t?.lines.map((l) => l.seq)).toEqual([1, 2, 3])
    expect(t?.info.lines).toBe(3)
  })

  it('applyMeta 更新 label 与 follow；remove 关 tab 并修正 activeId', () => {
    const s = useTabsStore()
    s.applyOpened({ tabId: 'a', label: 'a.log' })
    s.applyOpened({ tabId: 'b', label: 'b.log' })
    s.applyMeta({ tabId: 'a', label: 'a.log', parserName: 'json', follow: true })
    expect(s.tabs[0].info.follow).toBe(true)

    s.activeId = 'b'
    s.remove('b')
    expect(s.tabs.map((t) => t.info.id)).toEqual(['a'])
    expect(s.activeId).toBe('a') // 关闭后回落到邻近 tab
  })

  it('folds/toggleCollapse:折叠态增删;applyFolds 清已消失组的孤儿折叠态', () => {
    const s = useTabsStore()
    s.applyOpened({ tabId: 't1', label: 'app.log' })
    s.applyFolds({ tabId: 't1', ranges: [{ startSeq: 2, endSeq: 5, leader: 'x' }] })
    const t = () => s.tabs.find((x) => x.info.id === 't1')!

    s.toggleCollapse(2)
    expect(t().collapsed).toEqual([2])
    s.toggleCollapse(2)
    expect(t().collapsed).toEqual([])

    s.toggleCollapse(2)
    // 组消失(重发空 ranges)→ 孤儿折叠态清除
    s.applyFolds({ tabId: 't1', ranges: [] })
    expect(t().collapsed).toEqual([])
  })
})
