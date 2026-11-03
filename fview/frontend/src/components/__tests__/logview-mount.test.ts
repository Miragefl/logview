// LogView 虚拟滚动 mount 冒烟(Final Review #1/#6 回归):
// 行定位不得双重补偿;traceId 列类名与样式选择器必须配对。
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import naive from 'naive-ui'
// vite ?raw 资源(SFC 源码字符串,断言样式选择器配对)
import logViewSrc from '../logview/LogView.vue?raw'

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
import { useUiStore } from '../../stores/ui'
import { useAnnotationsStore } from '../../stores/annotations'
import type { LineDTO } from '../../lib/types'

function ln(seq: number, message: string, traceId = ''): LineDTO {
  return { seq, time: '12:00:00.001', level: 'INFO', thread: 'main', traceId, logger: 'a.B.C', message, raw: message }
}

const PROPS = {
  hitSeqs: new Set<number>(),
  hitWords: [] as string[],
  levelMode: 'A' as const,
  fields: ['time', 'level', 'thread', 'traceId', 'logger', 'message'],
}

describe('LogView mount 冒烟', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('行定位无双重补偿:.rows 不带 translateY,行 top 为全局偏移(Final #1)', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: [ln(1, 'a'), ln(2, 'b'), ln(3, 'c')] })
    const ui = useUiStore()

    const w = mount(LogView, { props: PROPS, global: { plugins: [naive] } })
    await new Promise((r) => setTimeout(r, 0))

    const rows = w.find('.rows').element as HTMLElement
    expect(rows.style.transform).toBe('') // translateY 与全局 top 不得叠加
    const rowEls = w.findAll('.row:not(.header)')
    expect(rowEls.length).toBeGreaterThanOrEqual(3)
    const px = (el: Element) => (el as HTMLElement).style.top
    expect(px(rowEls[0].element)).toBe('0px')
    expect(px(rowEls[1].element)).toBe(`${ui.rowPx()}px`)
    expect(px(rowEls[2].element)).toBe(`${ui.rowPx() * 2}px`)
  })

  it('traceId 列类名 f-traceId 与样式选择器配对,--trace 生效(Final #6)', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: [ln(1, 'a', 'abc123')] })
    const w = mount(LogView, { props: PROPS, global: { plugins: [naive] } })
    await new Promise((r) => setTimeout(r, 0))

    // 表头也带 f-traceId 类,取数据行的
    const traceCells = w.findAll('.row:not(.header) .f-traceId')
    expect(traceCells.length).toBeGreaterThanOrEqual(1)
    expect(traceCells[0].text()).toContain('abc123')
    // 样式表必须有对应选择器(类名拼错则 --trace 永不生效)
    expect(logViewSrc).toMatch(/\.f-traceId\s*\{/)
    expect(logViewSrc).toMatch(/--trace/)
  })

  it('搜索过滤:hitSeqs 非空时只渲染命中行,空集恢复全量(第三轮验收⑤,对齐 TUI recomputeView)', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: [ln(1, 'alpha'), ln(2, 'beta'), ln(3, 'alpha-2')] })

    const w = mount(LogView, {
      props: { ...PROPS, hitSeqs: new Set([2]) },
      global: { plugins: [naive] },
    })
    await new Promise((r) => setTimeout(r, 0))
    let rows = w.findAll('.row:not(.header)')
    expect(rows).toHaveLength(1)
    expect(rows[0].text()).toContain('beta')

    await w.setProps({ hitSeqs: new Set<number>() })
    await new Promise((r) => setTimeout(r, 0))
    rows = w.findAll('.row:not(.header)')
    expect(rows).toHaveLength(3)
  })

  it('source 字段可见时行内容不消失——fitField 无宽字段不再 RangeError(第三轮验收⑥)', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({
      tabId: 't1',
      lines: [{ ...ln(1, 'hello-msg'), source: 'local:a.log' } as LineDTO],
    })
    const w = mount(LogView, {
      props: { ...PROPS, fields: ['time', 'source', 'message'] },
      global: { plugins: [naive] },
    })
    await new Promise((r) => setTimeout(r, 0))
    const rows = w.findAll('.row:not(.header)')
    expect(rows).toHaveLength(1)
    // message 不丢 + source 列正常渲染原文
    expect(rows[0].text()).toContain('hello-msg')
    expect(rows[0].text()).toContain('local:a.log')
  })

  it('表头带 gutter 占位,与数据行列起点对齐(第三轮验收⑦)', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: [ln(1, 'a')] })
    const w = mount(LogView, { props: PROPS, global: { plugins: [naive] } })
    await new Promise((r) => setTimeout(r, 0))
    const header = w.find('.row.header')
    expect(header.exists()).toBe(true)
    // 表头第一个子元素必须是 gutter(与数据行同构,否则列起点错开 gutter 宽)
    expect(header.element.firstElementChild?.classList.contains('gutter')).toBe(true)
    expect(logViewSrc).toMatch(/visualActive/)
    expect(logViewSrc).toMatch(/hitSeqs\.has\(l\.seq\)/)
  })

  it('列宽拖拽:表头把手拖动调宽,mouseup 一次性 emit 落盘(第七轮验收②)', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: [ln(1, 'a')] })
    const w = mount(LogView, { props: PROPS, global: { plugins: [naive] } })
    await new Promise((r) => setTimeout(r, 0))

    // 固定列(非 message)各有把手:PROPS.fields = time/level/thread/traceId/logger/message → 5 把
    const handles = w.findAll('.row.header .col-handle')
    expect(handles.length).toBe(5)

    // 拖 time 列 +77px(charPx jsdom 下默认 7 → +11ch):12 → 23
    await handles[0].trigger('mousedown', { clientX: 100, button: 0 })
    document.dispatchEvent(new MouseEvent('mousemove', { clientX: 177, bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))
    // 拖拽中表头已按新宽渲染(fitField 填充)
    const headTime = w.findAll('.row.header .cell')[0]
    expect(headTime.text().endsWith('时间') || headTime.text().trim().length).toBeTruthy()
    document.dispatchEvent(new MouseEvent('mouseup', { clientX: 177, bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))

    const emitted = w.emitted('update:widths')
    expect(emitted).toBeTruthy()
    const newW = emitted![0][0] as Record<string, number>
    expect(newW.time).toBe(23)
    expect(newW.level).toBe(5)
    expect(newW.logger).toBe(20)
  })

  it('n/N 高亮兜底:无搜索命中集时跳高亮词所在行(第五轮验收)', async () => {
    const tabs = useTabsStore()
    const ann = useAnnotationsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({
      tabId: 't1',
      lines: [ln(1, 'plain-one'), ln(2, 'has alpha inside'), ln(3, 'plain-two')],
    })
    ann.addHighlight('alpha')

    const w = mount(LogView, { props: PROPS, global: { plugins: [naive] } })
    await new Promise((r) => setTimeout(r, 0))

    w.vm.cursorToHit(1) // n:从可视首行(seq 1)向下找,应跳到 seq 2
    await new Promise((r) => setTimeout(r, 0))
    const cur = w.find('.row.cur')
    expect(cur.exists()).toBe(true)
    expect(cur.text()).toContain('has alpha inside')

    w.vm.cursorToHit(-1) // N:seq 2 之上无命中,光标不动(不回绕,对齐 TUI 单向找)
    await new Promise((r) => setTimeout(r, 0))
    expect(w.find('.row.cur').text()).toContain('has alpha inside')
  })
})
