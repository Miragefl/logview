// 上下文混入集成测试(2026-11-04 移植 TUI grep-context):ctxKey 键序列状态机 +
// 锚落 tab store + dim 渲染 + 三路清除 + 过滤变化语义性退出 + 无过滤 no-op。
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('../../lib/backend', () => ({
  getLine: vi.fn().mockResolvedValue(null),
  setFollow: vi.fn().mockResolvedValue(undefined),
  bumpKeyword: vi.fn().mockResolvedValue(undefined),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({
  ClipboardSetText: vi.fn().mockResolvedValue(undefined),
}))

import LogView from '../logview/LogView.vue'
import { useTabsStore } from '../../stores/tabs'
import type { LineDTO } from '../../lib/types'

function ln(seq: number): LineDTO {
  return { seq, time: '', level: 'INFO', thread: '', traceId: '', logger: '', message: `l${seq}`, raw: `raw${seq}` }
}
/** key 合成键事件(ctxKey 只读 key/ctrl/meta/alt)。 */
function key(k: string, opts: KeyboardEventInit = {}) {
  return new KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true, ...opts })
}

const wrappers: ReturnType<typeof mount>[] = []

/** mountView 命中集 {5,9} 挂载(12 行镜像);返回组件实例类型句柄。 */
function mountView(hitSeqs: Set<number>) {
  const w = mount(LogView, {
    props: { hitSeqs, hitWords: [], levelMode: 'A' as never, fields: ['time', 'level', 'message'] },
  })
  wrappers.push(w)
  return w.vm as unknown as {
    ctxKey: (e: KeyboardEvent) => boolean
    cursorToSeq: (seq: number) => void
    hasCtxMix: () => number
  }
}

describe('LogView 上下文混入', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: Array.from({ length: 12 }, (_, i) => ln(i)) })
  })
  afterEach(() => {
    wrappers.forEach((w) => w.unmount())
    wrappers.length = 0
  })

  it('+ Enter 默认 5:锚落光标行,窗口未命中行 dim 渲染,命中行不插', async () => {
    const tabs = useTabsStore()
    const lv = mountView(new Set([5, 9]))
    lv.cursorToSeq(5)

    expect(lv.ctxKey(key('+'))).toBe(true)
    expect(lv.ctxKey(key('Enter'))).toBe(true)
    await new Promise((r) => setTimeout(r, 0))

    expect(tabs.active?.ctxAnchors).toEqual({ 5: { before: false, n: 5 } })
    // 窗口 (5,10]:6/7/8 dim,9 命中不插,10 dim → 4 行 .ctx;行首 ┆ 同数
    const w = wrappers[0]!
    expect(w.findAll('.row.ctx').length).toBe(4)
    expect(w.findAll('.ctx-mark').length).toBe(4)
  })

  it('+ 3 Enter:数字累积 n=3', async () => {
    const tabs = useTabsStore()
    const lv = mountView(new Set([5, 9]))
    lv.cursorToSeq(9)
    for (const k of ['+', '3', 'Enter']) lv.ctxKey(key(k))
    await new Promise((r) => setTimeout(r, 0))
    // 窗口 (9,12] clamp 到尾:10/11
    expect(tabs.active?.ctxAnchors).toEqual({ 9: { before: false, n: 3 } })
    expect(wrappers[0]!.findAll('.row.ctx').length).toBe(2)
  })

  it('- 2 Enter:往前混入(上侧窗口)', async () => {
    const lv = mountView(new Set([5, 9]))
    lv.cursorToSeq(5)
    for (const k of ['-', '2', 'Enter']) lv.ctxKey(key(k))
    await new Promise((r) => setTimeout(r, 0))
    // 窗口 [3,5):3/4 dim
    expect(wrappers[0]!.findAll('.row.ctx').length).toBe(2)
  })

  it('++ 第二符号关闭:锚清空,光标回最近过滤行', async () => {
    const tabs = useTabsStore()
    const lv = mountView(new Set([5, 9]))
    lv.cursorToSeq(5)
    for (const k of ['+', '2', 'Enter']) lv.ctxKey(key(k))
    await new Promise((r) => setTimeout(r, 0))
    expect(tabs.active?.ctxAnchors![5]).toBeTruthy()

    // 光标挪到 dim 行(4)再 ++:清锚且光标回最近过滤行 5
    lv.cursorToSeq(4)
    expect(lv.ctxKey(key('+'))).toBe(true)
    expect(lv.ctxKey(key('+'))).toBe(true)
    await new Promise((r) => setTimeout(r, 0))
    expect(tabs.active?.ctxAnchors).toEqual({})
    expect(wrappers[0]!.findAll('.row.ctx').length).toBe(0)
    expect(wrappers[0]!.find('.row.cur')!.text()).toContain('l5')
  })

  it('Esc 有锚先清锚(消费);无锚不吞', async () => {
    const tabs = useTabsStore()
    const lv = mountView(new Set([5, 9]))
    lv.cursorToSeq(5)
    lv.ctxKey(key('+'))
    lv.ctxKey(key('Enter')) // 输入态 Enter=生效(Esc 在输入态=取消,两码事)
    await new Promise((r) => setTimeout(r, 0))
    expect(Object.keys(tabs.active!.ctxAnchors).length).toBe(1)

    expect(lv.ctxKey(key('Escape'))).toBe(true) // 混入态 Esc=清锚
    await new Promise((r) => setTimeout(r, 0))
    expect(tabs.active?.ctxAnchors).toEqual({})
    expect(lv.ctxKey(key('Escape'))).toBe(false) // 无锚不吞,交回全局(visual 退出等)
  })

  it('输入态 Esc 取消不生效 + 输入态吞其他键', async () => {
    const tabs = useTabsStore()
    const lv = mountView(new Set([5, 9]))
    lv.cursorToSeq(5)
    lv.ctxKey(key('+'))
    expect(lv.ctxKey(key('j'))).toBe(true) // 输入态吞键(TUI:其余忽略)
    expect(lv.ctxKey(key('Escape'))).toBe(true) // 取消
    await new Promise((r) => setTimeout(r, 0))
    expect(tabs.active?.ctxAnchors).toEqual({})
  })

  it('过滤变化(hitSeqs 换引用)自动清锚', async () => {
    const tabs = useTabsStore()
    const lv = mountView(new Set([5, 9]))
    lv.cursorToSeq(5)
    lv.ctxKey(key('+'))
    lv.ctxKey(key('Enter'))
    await new Promise((r) => setTimeout(r, 0))
    expect(Object.keys(tabs.active!.ctxAnchors).length).toBe(1)

    await wrappers[0]!.setProps({ hitSeqs: new Set([5, 9, 11]) })
    await new Promise((r) => setTimeout(r, 0))
    expect(tabs.active?.ctxAnchors).toEqual({})
  })

  it('无过滤 + 不进输入态(ctxKey 不消费)', () => {
    const lv = mountView(new Set()) // hitSeqs 空 + level A + 无隐藏 = 全量视图
    expect(lv.ctxKey(key('+'))).toBe(false)
  })

  it('多锚累积:不同行锚互不清除,状态栏计数=锚数', async () => {
    const tabs = useTabsStore()
    const lv = mountView(new Set([5, 9]))
    lv.cursorToSeq(5)
    for (const k of ['+', '2', 'Enter']) lv.ctxKey(key(k))
    lv.cursorToSeq(9)
    for (const k of ['-', '2', 'Enter']) lv.ctxKey(key(k))
    await new Promise((r) => setTimeout(r, 0))
    expect(Object.keys(tabs.active!.ctxAnchors).sort().map(Number)).toEqual([5, 9])
    expect(lv.hasCtxMix()).toBe(2)
    // 锚5 窗口 6/7 + 锚9 窗口 7/8 → dim {6,7,8} 去重
    expect(wrappers[0]!.findAll('.row.ctx').length).toBe(3)
  })
})
