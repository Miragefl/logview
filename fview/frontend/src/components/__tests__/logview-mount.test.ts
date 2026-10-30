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
})
