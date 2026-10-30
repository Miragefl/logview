import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import naive from 'naive-ui'

vi.mock('../../lib/backend', () => ({
  matchLines: vi.fn().mockResolvedValue([0, 2]), // buffer 索引域
  parseQuery: vi.fn().mockResolvedValue({
    parseError: '',
    highlightKeywords: ['err'],
    timeRangeHint: '',
  }),
  getFields: vi.fn().mockResolvedValue([
    { name: 'time', visible: true },
    { name: 'level', visible: true },
    { name: 'thread', visible: false },
  ]),
  saveFields: vi.fn().mockResolvedValue(undefined),
  saveHides: vi.fn().mockResolvedValue(undefined),
  setFollow: vi.fn().mockResolvedValue(undefined),
  getLine: vi.fn().mockResolvedValue({ seq: '101', message: 'x' }),
  getMemStats: vi.fn().mockResolvedValue(0),
  subscribeTabEvents: vi.fn(),
  getHides: vi.fn().mockResolvedValue([]),
  currentTheme: vi.fn().mockResolvedValue(''),
  themeNames: vi.fn().mockResolvedValue([]),
  themeCSS: vi.fn().mockResolvedValue({}),
  setTheme: vi.fn().mockResolvedValue(undefined),
  usageRecent: vi.fn().mockResolvedValue([]),
  closeTab: vi.fn().mockResolvedValue(undefined),
  bumpKeyword: vi.fn().mockResolvedValue(undefined),
  usageKeywords: vi.fn().mockResolvedValue([]),
}))

import * as backend from '../../lib/backend'
import Toolbar from '../logview/Toolbar.vue'
import SearchPopup from '../logview/SearchPopup.vue'
import { useTabsStore } from '../../stores/tabs'
import { useSearchStore } from '../../stores/searchStore'

function mountC(c: Parameters<typeof mount>[0], props?: Record<string, unknown>) {
  return mount(c as never, { props: props as never, global: { plugins: [naive] } })
}

describe('Critical 修复回归', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    ;(backend.matchLines as ReturnType<typeof vi.fn>).mockResolvedValue([0, 2])
  })

  it('C1: 搜索命中索引域(buffer idx)转为镜像 seq 域下发', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    // 镜像 seq 从 100 起（模拟裁头后稳态：seq≠索引）
    tabs.applyLines({ tabId: 't1', lines: makeLines(100, 3) })

    // 搜索逻辑已迁统一弹窗 SearchPopup(验收反馈③);NModal teleport 到 body,
    // 直接从 body 取原生 input 驱动事件
    const search = useSearchStore()
    search.open(0)
    const w = mount(SearchPopup, { global: { plugins: [naive] } })
    await new Promise((r) => setTimeout(r, 0))

    const input = document.body.querySelector('input')
    expect(input).toBeTruthy()
    input!.value = 'err'
    input!.dispatchEvent(new Event('input', { bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))
    input!.dispatchEvent(new KeyboardEvent('keyup', { key: 'Enter', bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))

    const emitted = w.emitted('search')
    expect(emitted).toBeTruthy()
    // 索引 [0,2] → seq [100,102]（不得把索引当 seq 下发）
    expect(emitted![0][0]).toEqual({ hitSeqs: [100, 102], words: ['err'] })
  })

  it('C2: 字段显隐只写 fields 配置，不得污染 rules.yaml hides 键', async () => {
    const w = mountC(Toolbar, { levelMode: 'A', fields: ['time', 'level', 'message'] })
    await new Promise((r) => setTimeout(r, 0)) // getFields 加载
    const vm = w.vm as unknown as { toggleField: (name: string) => Promise<void> }
    await vm.toggleField('time')
    expect(backend.saveFields).toHaveBeenCalled()
    expect(backend.saveHides).not.toHaveBeenCalled() // 红线：字段名不得写 hides
  })
})

import type { LineDTO } from '../../lib/types'

function makeLines(startSeq: number, n: number): LineDTO[] {
  const out: LineDTO[] = []
  for (let i = 0; i < n; i++) {
    const seq = startSeq + i
    out.push({
      seq,
      time: '',
      level: '',
      thread: '',
      traceId: '',
      logger: '',
      message: `m${seq}`,
      raw: `raw-${seq}`,
    })
  }
  return out
}
