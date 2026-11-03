import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import naive from 'naive-ui'
import { NDropdown } from 'naive-ui'
import { useFieldsStore } from '../../stores/fields'

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

function mountC(c: Parameters<typeof mount>[0], props?: Record<string, unknown>) {
  return mount(c as never, { props: props as never, global: { plugins: [naive] } })
}

describe('Critical 修复回归', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    ;(backend.matchLines as ReturnType<typeof vi.fn>).mockResolvedValue([0, 2])
  })

  // C1(搜索命中索引域→seq 域换算)已随 SearchPopup 退役迁至 searchbar.test.ts
  // (第六轮 SLS 式常驻条改造),同断言覆盖。

  it('C2: 字段勾选只走覆盖层 emit,不写 rules.yaml(含 hides 红线)', async () => {
    // 2026-11-03 字段显示重构:F 勾选改写单日志覆盖(localStorage),rules.yaml 默认层只在设置中心
    const fieldsStore = useFieldsStore()
    fieldsStore.all = [
      { name: 'time', visible: true, width: 12 },
      { name: 'level', visible: true, width: 5 },
      { name: 'message', visible: true, width: 0 },
    ]
    const w = mountC(Toolbar, { configured: ['time', 'level', 'message'] })
    w.findComponent(NDropdown).vm.$emit('select', 'time')
    expect(w.emitted('override-fields')![0]).toEqual([['level', 'message']])
    expect(backend.saveFields).not.toHaveBeenCalled() // 覆盖层不碰 rules.yaml
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
