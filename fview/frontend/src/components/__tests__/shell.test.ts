import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import naive from 'naive-ui'

// mock backend：App/Welcome/SearchBar 只依赖这几个封装（jsdom 下无真实 Wails 运行时）
vi.mock('../../lib/backend', () => ({
  subscribeTabEvents: vi.fn(),
  closeTab: vi.fn().mockResolvedValue(undefined),
  onShortcutCloseTab: vi.fn(),
  getHides: vi.fn().mockResolvedValue([]),
  getFields: vi.fn().mockResolvedValue([]),
  usageRecent: vi.fn().mockResolvedValue(['file:/tmp/app.log', 'file:/tmp/error.log']),
  currentTheme: vi.fn().mockResolvedValue(''),
  themeNames: vi.fn().mockResolvedValue([]),
  themeCSS: vi.fn().mockResolvedValue({}),
  setTheme: vi.fn().mockResolvedValue(undefined),
  getMemStats: vi.fn().mockResolvedValue(71 * 1024 * 1024),
  matchLines: vi.fn().mockResolvedValue([]),
  parseQuery: vi.fn().mockResolvedValue({ parseError: '', highlightKeywords: [], timeRangeHint: '' }),
  bumpKeyword: vi.fn().mockResolvedValue(undefined),
  usageKeywords: vi.fn().mockResolvedValue([]),
}))

import App from '../../App.vue'
import SearchBar from '../logview/SearchBar.vue'
import { closeTab, onShortcutCloseTab } from '../../lib/backend'
import { useTabsStore } from '../../stores/tabs'
import { useUiStore } from '../../stores/ui'
import type { LineDTO } from '../../lib/types'

// mount 用当前 active pinia（beforeEach 已 setActivePinia），保证与测试内 store 同源
function mountApp() {
  return mount(App, { global: { plugins: [naive] } })
}

describe('应用壳', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('无 tab 时渲染欢迎页（含最近源文件名）', async () => {
    const w = mountApp()
    await new Promise((r) => setTimeout(r, 0)) // 等 usageRecent promise
    expect(w.text()).toContain('fview')
    expect(w.text()).toContain('app.log')
    expect(w.text()).toContain('error.log')
  })

  it('有 tab 时渲染 TabBar 且 label 正确、active 高亮', () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'app.log' })
    tabs.applyOpened({ tabId: 't2', label: 'web1:/var/log/x.log' })
    tabs.activeId = 't2'
    const w = mountApp()
    expect(w.text()).toContain('app.log')
    expect(w.text()).toContain('web1:/var/log/x.log')
    const active = w.find('.tab.active')
    expect(active.exists()).toBe(true)
    expect(active.text()).toContain('web1')
  })

  it('TabBar 点击切换 activeId，点 × 关闭 tab', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyOpened({ tabId: 't2', label: 'b.log' })
    tabs.activeId = 't1'
    const w = mountApp()
    // 切换
    await w.findAll('.tab')[1].trigger('click')
    expect(useTabsStore().activeId).toBe('t2')
    // 关闭 t2
    await w.findAll('.tab')[1].find('.close').trigger('click')
    expect(useTabsStore().tabs.map((t) => t.info.id)).toEqual(['t1'])
  })

  it('切 tab 清空搜索过滤:旧 tab 命中集不得串扰新 tab(final review I4)', async () => {
    const mk = (seq: number, m: string): LineDTO => ({
      seq, time: '', level: 'INFO', thread: '', traceId: '', logger: '', message: m, raw: m,
    })
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: [mk(100, 'hit-x'), mk(101, 'miss-y')] })
    tabs.applyOpened({ tabId: 't2', label: 'b.log' })
    tabs.applyLines({ tabId: 't2', lines: [mk(100, 'm1'), mk(101, 'm2'), mk(102, 'm3')] })
    tabs.activeId = 't1'
    const w = mountApp()
    await new Promise((r) => setTimeout(r, 0))

    // tab A 搜索命中 seq=100 → 视图过滤为 1 行
    const bar = w.findComponent(SearchBar)
    expect(bar.exists()).toBe(true)
    bar.vm.$emit('search', { hitSeqs: [100], words: [] })
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))
    expect(w.findAll('.row:not(.header)')).toHaveLength(1)

    // 切到 tab B:命中集必须清空,不得用 A 的 seq 域过滤 B(seq 是 per-tab 域)
    tabs.activeId = 't2'
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))
    expect(w.findAll('.row:not(.header)')).toHaveLength(3)
  })
})

describe('⌘W 菜单快捷键(shortcut:closeTab 派遣)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(onShortcutCloseTab).mockClear()
    vi.mocked(closeTab).mockClear()
  })

  // 抓 App 挂载时注册的 ⌘W 回调——订阅接线本身就是要断言的目标
  function shortcutCb(): () => void {
    const calls = vi.mocked(onShortcutCloseTab).mock.calls
    if (!calls.length) throw new Error('App 未订阅 onShortcutCloseTab')
    return calls[0][0]
  }

  it('⌘W 关当前激活 tab', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyOpened({ tabId: 't2', label: 'b.log' })
    tabs.activeId = 't2'
    mountApp()
    shortcutCb()()
    await new Promise((r) => setTimeout(r, 0))
    expect(vi.mocked(closeTab)).toHaveBeenCalledWith('t2')
  })

  it('弹窗打开时 ⌘W 仍关 tab(不经 keydown 让位,浏览器惯例)', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.activeId = 't1'
    useUiStore().dialogOpen('settings')
    mountApp()
    shortcutCb()()
    await new Promise((r) => setTimeout(r, 0))
    expect(vi.mocked(closeTab)).toHaveBeenCalledWith('t1')
  })

  it('无 tab 时 ⌘W no-op 不抛错', async () => {
    mountApp()
    expect(() => shortcutCb()()).not.toThrow()
    await new Promise((r) => setTimeout(r, 0))
    expect(vi.mocked(closeTab)).not.toHaveBeenCalled()
  })

  it('最后一个 tab 关闭回欢迎页(空态不关窗)', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.activeId = 't1'
    const w = mountApp()
    shortcutCb()()
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))
    expect(vi.mocked(closeTab)).toHaveBeenCalledWith('t1')
    expect(useTabsStore().tabs).toHaveLength(0)
    // 欢迎页含最近源 error.log(tab label 是 a.log,不会与断言混淆)
    expect(w.text()).toContain('error.log')
  })
})
