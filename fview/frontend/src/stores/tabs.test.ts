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

  it('applyCleared:清镜像/行数/折叠态,activeId 不变', () => {
    const s = useTabsStore()
    s.applyOpened({ tabId: 't1', label: 'app.log' })
    s.applyLines({ tabId: 't1', lines: [ln(1), ln(2)] })
    s.applyFolds({ tabId: 't1', ranges: [{ startSeq: 1, endSeq: 2, leader: 'x' }] })
    s.toggleCollapse(1)

    s.applyCleared('t1')
    const t = s.tabs.find((x) => x.info.id === 't1')!
    expect(t.lines).toEqual([])
    expect(t.info.lines).toBe(0)
    expect(t.folds).toEqual([])
    expect(t.collapsed).toEqual([])
    expect(s.activeId).toBe('t1')
  })

  it('toggleBookmark:活动 tab 书签增删(seq 域);applyCleared 一并清', () => {
    const s = useTabsStore()
    s.applyOpened({ tabId: 't1', label: 'a.log' })
    s.toggleBookmark(5)
    s.toggleBookmark(9)
    s.toggleBookmark(5) // 再按删除
    expect(s.tabs[0].bookmarks).toEqual([9])

    s.applyOpened({ tabId: 't2', label: 'b.log' })
    s.toggleBookmark(1)
    expect(s.tabs[0].bookmarks).toEqual([9]) // tab 隔离
    expect(s.tabs[1].bookmarks).toEqual([1])

    s.applyCleared('t2')
    expect(s.tabs[1].bookmarks).toEqual([])
  })

  it('applyOpened 透传 display/info(顶栏文件名+源信息条,验收⑥)', () => {
    const s = useTabsStore()
    s.applyOpened({ tabId: 't1', label: 'file', display: 'app.log', info: '/tmp/app.log' })
    expect(s.tabs[0].info.display).toBe('app.log')
    expect(s.tabs[0].info.info).toBe('/tmp/app.log')
    // display 空:保持空,TabBar 回落 label
    s.applyOpened({ tabId: 't2', label: 'ssh://h/p', display: '', info: '' })
    expect(s.tabs[1].info.display).toBe('')
  })
})

describe('parserFields(tab:meta 透传)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('applyMeta 带 parserFields 存入 tab;register 初始为 null', () => {
    const s = useTabsStore()
    s.applyOpened({ tabId: 't1', label: 'app.log' })
    expect(s.tabs[0].parserFields).toBeNull()
    s.applyMeta({ tabId: 't1', label: 'app.log', parserName: 'java-logback', parserFields: ['time', 'message'] })
    expect(s.tabs[0].parserFields).toEqual(['time', 'message'])
  })

  it('follow-only meta 不带 parserFields 时不清已有值(omitempty→undefined 跳过)', () => {
    const s = useTabsStore()
    s.applyOpened({ tabId: 't1', label: 'app.log' })
    s.applyMeta({ tabId: 't1', label: 'app.log', parserName: 'java-logback', parserFields: ['time'] })
    s.applyMeta({ tabId: 't1', label: 'app.log', parserName: 'java-logback', follow: false })
    expect(s.tabs[0].parserFields).toEqual(['time'])
  })
})
