// 字段列高亮/命中渲染测试(field-match-scope):高亮词与搜索命中底纹
// 从 message 扩到所有字段 cell(所见即所染——屏显截断/缩写文本上染)。
// mock 头复刻 logview-fieldfocus.test.ts。
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import naive from 'naive-ui'

vi.mock('../../lib/backend', () => ({
  getLine: vi.fn().mockResolvedValue({}),
  setFollow: vi.fn().mockResolvedValue(undefined),
  bumpKeyword: vi.fn().mockResolvedValue(undefined),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({
  ClipboardSetText: vi.fn().mockResolvedValue(undefined),
}))
vi.stubGlobal('ResizeObserver', class {
  observe() {}
  unobserve() {}
  disconnect() {}
})

import LogView from '../logview/LogView.vue'
import { useTabsStore } from '../../stores/tabs'
import { useAnnotationsStore } from '../../stores/annotations'
import type { LineDTO } from '../../lib/types'

function ln(seq: number, message: string, logger = 'OrderService'): LineDTO {
  return { seq, time: '12:00:00.001', level: 'INFO', thread: 'main', traceId: '', logger, message, raw: message }
}

const PROPS = {
  hitSeqs: new Set<number>(),
  hitWords: [] as string[],
  levelMode: 'A' as const,
  fields: ['time', 'level', 'logger', 'message'],
}

const tick = () => new Promise((r) => setTimeout(r, 0))

async function mountView(lines: LineDTO[], highlights: { word: string; color: string }[], hitWords: string[]) {
  const tabs = useTabsStore()
  tabs.applyOpened({ tabId: 't1', label: 'a.log' })
  tabs.applyLines({ tabId: 't1', lines })
  const ann = useAnnotationsStore()
  ann.highlights = highlights
  const w = mount(LogView, { props: { ...PROPS, hitWords }, global: { plugins: [naive] } })
  await tick()
  return w
}

describe('LogView 字段列高亮/命中渲染(field-match-scope)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('h01: 高亮词命中 logger 列 → 该列文字亮色标注', async () => {
    const w = await mountView([ln(1, 'plain msg')], [{ word: 'OrderService', color: '#ff0000' }], [])
    const hl = w.find('.rows .row .cell.f-logger .hl-w')
    expect(hl.exists()).toBe(true)
    expect(hl.text()).toBe('OrderService')
    expect(hl.attributes('style')).toContain('rgb(255, 0, 0)') // jsdom 将 #ff0000 序列化为 rgb()
  })

  it('h02: 搜索命中词落在 time 列 → 该列带命中底纹', async () => {
    const w = await mountView([ln(1, 'plain msg')], [], ['12:00'])
    const hit = w.find('.rows .row .cell.f-time .hit-w')
    expect(hit.exists()).toBe(true)
    expect(hit.text()).toBe('12:00')
  })

  it('h03: level 列命中染成内层 span(级别色之上叠加高亮)', async () => {
    const w = await mountView([ln(1, 'plain msg')], [{ word: 'INFO', color: '#00ff00' }], [])
    const hl = w.find('.rows .row .cell.f-level.lv-info .hl-w')
    expect(hl.exists()).toBe(true)
    expect(hl.text()).toBe('INFO')
  })

  it('h04: 无命中时字段列无 hl/hit span(普通文本路径不炸)', async () => {
    const w = await mountView([ln(1, 'plain msg')], [{ word: 'nomatch', color: '#ff0000' }], ['alsonomatch'])
    expect(w.findAll('.rows .row .cell:not(.f-msg) .hl-w')).toHaveLength(0)
    expect(w.findAll('.rows .row .cell:not(.f-msg) .hit-w')).toHaveLength(0)
  })

  it('h05: 表头不染(守护)', async () => {
    const w = await mountView([ln(1, 'plain msg')], [{ word: 'OrderService', color: '#ff0000' }], ['12:00'])
    expect(w.findAll('.row.header .hl-w')).toHaveLength(0)
    expect(w.findAll('.row.header .hit-w')).toHaveLength(0)
  })

  it('h06: 缩写屏显所见即所染——保留末段可染,被缩掉段不染(Focus 4 守护)', async () => {
    // com.example.order.OrderService → 缩写 c.e.o.OrderService(包名段取首字母,末段类名全保留)
    const w = await mountView([ln(1, 'plain msg', 'com.example.order.OrderService')], [{ word: 'OrderService', color: '#ff0000' }], [])
    const hl = w.find('.rows .row .cell.f-logger .hl-w')
    expect(hl.exists()).toBe(true)
    expect(hl.text()).toBe('OrderService') // 缩写保留的末段命中,屏显可见即可染

    const w2 = await mountView([ln(1, 'plain msg', 'com.example.order.OrderService')], [{ word: 'example', color: '#ff0000' }], [])
    expect(w2.findAll('.rows .row .cell.f-logger .hl-w')).toHaveLength(0) // example 已缩成 e.,屏显无此文本不染(防"先染原值再截断"式回归)
  })
})
