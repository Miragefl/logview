import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
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
import LogView from '../logview/LogView.vue'
import { closeTab, onShortcutCloseTab } from '../../lib/backend'
import { useTabsStore } from '../../stores/tabs'
import { useSearchStore } from '../../stores/searchStore'
import { useUiStore } from '../../stores/ui'
import type { LineDTO } from '../../lib/types'

// mount 用当前 active pinia（beforeEach 已 setActivePinia），保证与测试内 store 同源
const mounted: ReturnType<typeof mount>[] = []
function mountApp() {
  const w = mount(App, { global: { plugins: [naive] } })
  mounted.push(w)
  return w
}

// 顶层 afterEach:App 实例必须卸载——document 级 keydown listener 与 NModal
// teleport 会跨用例/跨 describe 残留(dispatch 时旧实例并发开框,body 里堆
// 多个 wordspopup,querySelector 误中旧空框——field-interact 真机路径踩坑)
afterEach(() => {
  mounted.forEach((w) => w.unmount())
  mounted.length = 0
  document.body.innerHTML = ''
})

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

  // ---- 带词分发(field-interact Task7):光标词/选区词直达输入框 ----

  it('/ 键带光标词聚焦搜索条(field-interact)', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({
      tabId: 't1',
      lines: [{ seq: 1, time: '', level: 'INFO', thread: '', traceId: '', logger: '', message: 'w1 timeout w2', raw: '' }],
    })
    const w = mountApp()
    await new Promise((r) => setTimeout(r, 0))
    const lv = w.findComponent(LogView).vm as unknown as {
      moveCursor: (d: number) => void
      moveCursorCol: (m: string) => void
    }
    lv.moveCursor(1)
    await new Promise((r) => setTimeout(r, 0))
    // 光标移到 timeout 词中(col4)
    lv.moveCursorCol('right')
    lv.moveCursorCol('right')
    lv.moveCursorCol('right')
    lv.moveCursorCol('right')
    lv.moveCursorCol('right')
    await new Promise((r) => setTimeout(r, 0))

    document.dispatchEvent(new KeyboardEvent('keydown', { key: '/', bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))
    const search = useSearchStore()
    expect(search.barFocused).toBe(true)
    // App 挂真 SearchBar:seed 已被消费进输入框(端到端断言 value;
    // App mount 在内存树不进 document,走 wrapper 查询)
    const input = w.find('.searchbar input').element as HTMLInputElement
    expect(input.value).toBe('timeout')
  })

  it('C-h 带光标词打开高亮弹窗(field-interact)', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({
      tabId: 't1',
      lines: [{ seq: 1, time: '', level: 'INFO', thread: '', traceId: '', logger: '', message: 'w1 timeout w2', raw: '' }],
    })
    const w = mountApp()
    await new Promise((r) => setTimeout(r, 0))
    const lv = w.findComponent(LogView).vm as unknown as {
      moveCursor: (d: number) => void
      moveCursorCol: (m: string) => void
    }
    lv.moveCursor(1)
    await new Promise((r) => setTimeout(r, 0))
    lv.moveCursorCol('right')
    lv.moveCursorCol('right')
    lv.moveCursorCol('right')
    lv.moveCursorCol('right')
    lv.moveCursorCol('right')
    await new Promise((r) => setTimeout(r, 0))

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'h', ctrlKey: true, bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))
    const search = useSearchStore()
    expect(search.wordsShow).toBe('hl')
    // App 挂真 WordsPopup:seed 已被消费进输入框(端到端断言 value)
    const input = document.body.querySelector('.wordspopup input') as HTMLInputElement
    expect(input.value).toBe('timeout')
  })
})

describe('vim-count 数字前缀(组件链路)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  const key = (k: string, mod: KeyboardEventInit = {}) =>
    document.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true, ...mod }))
  const mk = (seq: number, m: string): LineDTO => ({
    seq, time: '', level: 'INFO', thread: '', traceId: '', logger: '', message: m, raw: m,
  })
  const seed = (n: number): LineDTO[] => Array.from({ length: n }, (_, i) => mk(100 + i, `m${i + 1}`))
  const tick = () => new Promise((r) => setTimeout(r, 0))
  /** curRow 光标行可见序(0 基,-1=无光标)与文本 */
  function curRow(w: ReturnType<typeof mountApp>) {
    const rows = w.findAll('.row:not(.header)')
    const i = rows.findIndex((r) => r.classes().includes('cur'))
    return { i, text: i >= 0 ? rows[i].text() : '' }
  }

  it('3j:无光标首按激活+走满 3 步(=第 4 行,vim 语义 count 优先于激活落子)', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: seed(5) })
    const w = mountApp()
    await tick()
    key('3')
    key('j')
    await tick()
    expect(curRow(w).text).toContain('m4')
  })

  it('2w:词前进 2 次(char-cursor 落 cc 首字符)', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: [mk(100, 'aa bb cc dd')] })
    const w = mountApp()
    await tick()
    key('2')
    key('w')
    await tick()
    const cur = w.find('.char-cursor')
    expect(cur.exists()).toBe(true)
    expect(cur.text()).toBe('c')
  })

  it('5G/10g:跳视图第 N 行;过滤态 clamp 到末可见行', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: seed(10) })
    const w = mountApp()
    await tick()
    key('5')
    key('G')
    await tick()
    expect(curRow(w).text).toContain('m5')
    key('1')
    key('0')
    key('g')
    await tick()
    expect(curRow(w).text).toContain('m10')
    // 过滤剩 3 可见行:5G clamp 第 3 可见行
    const bar = w.findComponent(SearchBar)
    bar.vm.$emit('search', { hitSeqs: [100, 102, 104], words: [] })
    await tick()
    await tick()
    expect(w.findAll('.row:not(.header)')).toHaveLength(3)
    key('5')
    key('G')
    await tick()
    expect(curRow(w).i).toBe(2)
  })

  it('2C-d:两倍半页(jsdom 视口保底 1 行/半页 → 下移 2 行)', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: seed(6) })
    const w = mountApp()
    await tick()
    key('2')
    key('d', { ctrlKey: true })
    await tick()
    expect(curRow(w).text).toContain('m3')
  })

  it('count 态被不吃 count 的键(#/Esc)清零,后续 j 单步', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: seed(5) })
    const w = mountApp()
    await tick()
    key('3')
    key('#')
    await tick()
    key('j')
    await tick()
    expect(curRow(w).text).toContain('m1')
    key('3')
    key('Escape')
    key('j')
    await tick()
    // Esc 后 j:上一 j 已激活 m1,单步 → m2
    expect(curRow(w).text).toContain('m2')
  })

  it('StatusBar 显示 pending count(showcmd 式反馈),动作后消失', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: seed(5) })
    const w = mountApp()
    await tick()
    key('3')
    await tick()
    const hint = w.find('.statusbar .count-hint')
    expect(hint.exists()).toBe(true)
    expect(hint.text()).toBe('3')
    key('j')
    await tick()
    expect(w.find('.statusbar .count-hint').exists()).toBe(false)
  })

  it('visual 态 3w 扩选:光标走 3 词,VISUAL 徽标在', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: [mk(100, 'aa bb cc dd ee')] })
    const w = mountApp()
    await tick()
    key('j') // 激活光标(startVisual 需光标行)
    await tick()
    key('v')
    await tick()
    expect(w.find('.visual-badge').exists()).toBe(true)
    key('3')
    key('w')
    await tick()
    expect(w.find('.char-cursor').text()).toBe('d') // dd 首字符
  })
})
