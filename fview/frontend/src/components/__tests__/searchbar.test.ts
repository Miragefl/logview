// SearchBar 常驻搜索条测试(第六轮 SLS 式改造):
// Enter 执行(C1 命中域换算迁移)/清空恢复全量/Esc 失焦不清词/parseError 拦截/级别切换。
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import naive, { NSelect } from 'naive-ui'

vi.mock('../../lib/backend', () => ({
  matchLines: vi.fn().mockResolvedValue([0, 2]),
  parseQuery: vi.fn().mockResolvedValue({ parseError: '', highlightKeywords: ['err'], timeRangeHint: '' }),
  usageKeywords: vi.fn().mockResolvedValue([]),
  bumpKeyword: vi.fn().mockResolvedValue(undefined),
}))

import SearchBar from '../logview/SearchBar.vue'
import { useTabsStore } from '../../stores/tabs'
import { useSearchStore } from '../../stores/searchStore'
import { matchLines, parseQuery } from '../../lib/backend'

function key(k: string, opts: KeyboardEventInit = {}) {
  const input = document.body.querySelector('.searchbar input')
  input?.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true, ...opts }))
}

function setQ(v: string) {
  const input = document.body.querySelector('.searchbar input') as HTMLInputElement
  input.value = v
  input.dispatchEvent(new Event('input', { bubbles: true }))
}

/** mountBar 挂到真实 DOM(SearchBar 无 Teleport,jsdom querySelector 需 attachTo)。 */
/** wrappers 跨用例收集(afterEach 统一卸载,防 jsdom document 串扰)。 */
const wrappers: ReturnType<typeof mount>[] = []

function mountBar(props: { hitCount: number; levelMode: string }) {
  const div = document.createElement('div')
  document.body.appendChild(div)
  const w = mount(SearchBar, {
    props: props as never,
    attachTo: div,
    global: { plugins: [naive] },
  })
  wrappers.push(w)
  return w
}

describe('SearchBar 常驻搜索条', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(parseQuery).mockResolvedValue({
      parseError: '',
      highlightKeywords: ['err'],
      timeRangeHint: '',
    })
  })
  afterEach(() => {
    wrappers.forEach((w) => w.unmount())
    wrappers.length = 0
    document.body.innerHTML = ''
  })

  it('Enter 执行:matchLines 索引域(buffer idx)换算镜像 seq 域下发(C1 迁移)', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    // 镜像 seq 从 100 起(模拟裁头稳态:seq≠索引)
    tabs.applyLines({ tabId: 't1', lines: [
      { seq: 100, time: '', level: 'INFO', thread: '', traceId: '', logger: '', message: 'err one', raw: '' },
      { seq: 101, time: '', level: 'INFO', thread: '', traceId: '', logger: '', message: 'mid', raw: '' },
      { seq: 102, time: '', level: 'INFO', thread: '', traceId: '', logger: '', message: 'err two', raw: '' },
    ] })

    const w = mountBar({ hitCount: 0, levelMode: 'A' })
    await new Promise((r) => setTimeout(r, 0))

    setQ('err')
    await new Promise((r) => setTimeout(r, 0))
    key('Enter')
    await new Promise((r) => setTimeout(r, 0))

    const emitted = w.emitted('search')
    expect(emitted).toBeTruthy()
    // 索引 [0,2] → seq [100,102](不得把索引当 seq 下发)
    expect(emitted![0][0]).toEqual({ hitSeqs: [100, 102], words: ['err'] })
    // 成功执行入搜索历史
    expect(useSearchStore().history[0]).toBe('err')
  })

  it('清空回车恢复全量:emit 空 hitSeqs 且不调 matchLines', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: [] })

    const w = mountBar({ hitCount: 3, levelMode: 'A' })
    await new Promise((r) => setTimeout(r, 0))

    vi.mocked(matchLines).mockClear()
    setQ('')
    key('Enter')
    await new Promise((r) => setTimeout(r, 0))

    expect(w.emitted('search')![0][0]).toEqual({ hitSeqs: [], words: [] })
    expect(matchLines).not.toHaveBeenCalled()
  })

  it('Esc 失焦不清词:输入保留,再次 Enter 仍按原词执行', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: [] })

    const w = mountBar({ hitCount: 0, levelMode: 'A' })
    await new Promise((r) => setTimeout(r, 0))

    const input = document.body.querySelector('.searchbar input') as HTMLInputElement
    input.focus()
    setQ('keep-me')
    key('Escape')
    await new Promise((r) => setTimeout(r, 0))
    // 失焦但词保留(vim:Esc 只退出输入态)
    expect(document.activeElement).not.toBe(input)
    expect(input.value).toBe('keep-me')

    input.focus()
    key('Enter')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.emitted('search')!.length).toBe(1)
  })

  it('Enter 执行成功即失焦:焦点交还日志区,/ 键可再聚焦(vim:执行完退出输入态)', async () => {
    const search = useSearchStore()
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: [
      { seq: 100, time: '', level: 'INFO', thread: '', traceId: '', logger: '', message: 'err one', raw: '' },
    ] })

    const w = mountBar({ hitCount: 0, levelMode: 'A' })
    await new Promise((r) => setTimeout(r, 0))

    const input = document.body.querySelector('.searchbar input') as HTMLInputElement
    input.focus()
    setQ('err')
    key('Enter')
    await new Promise((r) => setTimeout(r, 0))
    // 执行成功即失焦:焦点不得留在搜索框(要再搜按 / 或点击)
    expect(document.activeElement).not.toBe(input)
    expect(w.emitted('search')!.length).toBe(1)
    // / 键重聚焦回归:失焦后 store 已复位,再 focusBar 仍能聚焦
    search.focusBar()
    await new Promise((r) => setTimeout(r, 0))
    expect(document.activeElement).toBe(input)
  })

  it('清空回车恢复全量同样失焦', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: [] })

    mountBar({ hitCount: 3, levelMode: 'A' })
    await new Promise((r) => setTimeout(r, 0))

    const input = document.body.querySelector('.searchbar input') as HTMLInputElement
    input.focus()
    setQ('')
    key('Enter')
    await new Promise((r) => setTimeout(r, 0))
    expect(document.activeElement).not.toBe(input)
  })

  it('parseError:不执行搜索,input 进入 error 态', async () => {
    vi.mocked(parseQuery).mockResolvedValue({
      parseError: 'time: 语法错误',
      highlightKeywords: [],
      timeRangeHint: '',
    })
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: [] })

    const w = mountBar({ hitCount: 0, levelMode: 'A' })
    await new Promise((r) => setTimeout(r, 0))

    setQ('time:bad')
    key('Enter')
    await new Promise((r) => setTimeout(r, 0))

    expect(w.emitted('search')).toBeUndefined()
    // NInput error 态(naive status=error 渲染 error-status 类;jsdom 下拉交互不稳,断言类名)
    expect(document.body.querySelector('.searchbar .n-input--error-status')).toBeTruthy()
    // 报错不失焦:留在输入框让用户改词,blur 会把错误提示带离视线
    const input = document.body.querySelector('.searchbar input') as HTMLInputElement
    input.focus()
    key('Enter')
    await new Promise((r) => setTimeout(r, 0))
    expect(document.activeElement).toBe(input)
  })

  it('级别下拉切换 emit update:levelMode', async () => {
    const w = mountBar({ hitCount: 0, levelMode: 'A' })
    await new Promise((r) => setTimeout(r, 0))

    // naive select 在 jsdom 的浮层交互不稳,直接驱动契约:NSelect update:value → 组件 emit
    const sel = w.findComponent(NSelect)
    expect(sel.exists()).toBe(true)
    sel.vm.$emit('update:value', 'I')
    await new Promise((r) => setTimeout(r, 0))
    const emitted = w.emitted('update:levelMode')
    expect(emitted).toBeTruthy()
    expect(emitted![0][0]).toBe('I')
  })

  it('blur 回写聚焦态:失焦后 store 复位,再 focusBar 仍能聚焦(C1 死锁回归)', async () => {
    const search = useSearchStore()
    useTabsStore().applyOpened({ tabId: 't1', label: 'a.log' })
    mountBar({ hitCount: 0, levelMode: 'A' })
    await new Promise((r) => setTimeout(r, 0))

    const input = document.body.querySelector('.searchbar input') as HTMLInputElement
    search.focusBar()
    await new Promise((r) => setTimeout(r, 0))
    expect(document.activeElement).toBe(input)

    // 模拟点击别处失焦:store 必须回写 false,否则同值赋值 watch 不触发 → / 键死锁
    input.blur()
    await new Promise((r) => setTimeout(r, 0))
    expect(search.barFocused).toBe(false)

    // 死锁回归:失焦后再按 / 仍能聚焦
    search.focusBar()
    await new Promise((r) => setTimeout(r, 0))
    expect(document.activeElement).toBe(input)
  })

  it('历史下拉失焦关闭:ctrl+R 开历史后 blur,浮层收起(I3)', async () => {
    const search = useSearchStore()
    search.addSearchHistory('w1')
    useTabsStore().applyOpened({ tabId: 't1', label: 'a.log' })
    mountBar({ hitCount: 0, levelMode: 'A' })
    await new Promise((r) => setTimeout(r, 0))

    const input = document.body.querySelector('.searchbar input') as HTMLInputElement
    input.focus()
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'r', ctrlKey: true, bubbles: true, cancelable: true }))
    await new Promise((r) => setTimeout(r, 0))
    expect(document.body.querySelector('.searchbar .hist')).toBeTruthy()

    input.blur() // 点日志区 → 浮层必须收起,不得残留遮挡
    await new Promise((r) => setTimeout(r, 0))
    expect(document.body.querySelector('.searchbar .hist')).toBeNull()
  })
})
