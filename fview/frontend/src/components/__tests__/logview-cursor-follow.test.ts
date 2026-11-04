// 光标跟随滚动回归测试(2026-11-04):j 底部钉住态按 k,视口必须静止、光标回爬
// (scrolloff 舒适区语义)。根因:setCursor 等高分支读 lagging 的 range[0](源自
// 异步 scroll 事件更新的 scrollTop.value),键盘连按时 first 滞后——j 方向
// 碰巧无害(下滚分支只依赖 idx),k 方向 stale first 偏小使光标永落 stale 底部圈,
// 被反复拽回倒数第 4 行。jsdom 不派发 scroll 事件,天然复现最坏 stale 情形。
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
import { useUiStore } from '../../stores/ui'
import type { LineDTO } from '../../lib/types'

function ln(seq: number): LineDTO {
  return { seq, time: '', level: 'INFO', thread: '', traceId: '', logger: '', message: `l${seq}`, raw: '' }
}

const wrappers: ReturnType<typeof mount>[] = []
let protoH: PropertyDescriptor | undefined

/** mountView attachTo 真实 DOM。布局属性 hack(jsdom 恒 0/赋值 no-op):
 *  clientHeight 走原型级 getter(=600,onMounted 读的就是 .logview 元素,
 *  外层容器 hack 不生效);scrollTop 在实例上 defineProperty 成受控存储。
 *  scroll 事件不派发 = 本测试要的 stale 最坏情形(真实浏览器连按时同病)。 */
function mountView(rows: number) {
  void rows
  protoH = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'clientHeight')
  Object.defineProperty(HTMLElement.prototype, 'clientHeight', {
    get() {
      return 600
    },
    configurable: true,
  })
  const div = document.createElement('div')
  document.body.appendChild(div)
  const w = mount(LogView, {
    props: { hitSeqs: new Set<number>(), hitWords: [], levelMode: 'A' as never, fields: ['message'] },
    attachTo: div,
  })
  wrappers.push(w)
  const el = div.querySelector('.logview') as HTMLElement
  let st = 0
  Object.defineProperty(el, 'scrollTop', {
    get: () => st,
    set: (v: number) => {
      st = v
    },
    configurable: true,
  })
  return { w, el: () => el }
}

describe('LogView 光标跟随滚动', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: Array.from({ length: 100 }, (_, i) => ln(i)) })
  })
  afterEach(() => {
    wrappers.forEach((w) => w.unmount())
    wrappers.length = 0
    document.body.innerHTML = ''
    if (protoH) Object.defineProperty(HTMLElement.prototype, 'clientHeight', protoH)
  })

  it('j 底部钉住态按 k:视口静止,光标回爬不被拽回(scrolloff 舒适区)', async () => {
    // ui 默认 fontSize13×lineHeight1.5 → rowH=20,clientHeight 600 → rowsInView=30
    const { w } = mountView(100)
    await new Promise((r) => setTimeout(r, 0))
    const lv = w.vm as unknown as { moveCursor: (d: number) => void }
    const scrollTopOf = () => (document.body.querySelector('.logview') as HTMLElement).scrollTop

    // 60 次 j:光标 0→60,底部钉住(倒数第 4 行=last-3),每步下滚一行
    for (let i = 0; i < 60; i++) lv.moveCursor(1)
    await new Promise((r) => setTimeout(r, 0))
    // 钉住态 scrollTop=(idx-rows+1+scrolloff)×rowH=(60-30+1+3)×20=34 行
    // (若此断言已红,说明钉住态本身漂移,先修钉住再看回爬)
    expect(scrollTopOf()).toBe(34 * 20)

    // k 一步:光标 59 相对真实视口 first=34/last=63 的舒适区 [37,60] → 视口必须静止
    lv.moveCursor(-1)
    await new Promise((r) => setTimeout(r, 0))
    expect(scrollTopOf()).toBe(34 * 20) // ← 回归点:修复前 stale first(=0)使光标仍落底部圈,再滚到 660

    // 连按 k 20 次:光标 59→39,全程在舒适区 [37,60] 内爬,视口静止
    for (let i = 0; i < 20; i++) lv.moveCursor(-1)
    await new Promise((r) => setTimeout(r, 0))
    expect(scrollTopOf()).toBe(34 * 20)
  })

  it('kbNav 键盘导航态:kb-nav 类随 ui store 响应翻转(藏指针/悬停抑制的渲染开关)', async () => {
    const ui = useUiStore()
    mountView(10)
    await new Promise((r) => setTimeout(r, 0))
    const el = () => document.body.querySelector('.logview') as HTMLElement

    expect(el().classList.contains('kb-nav')).toBe(false)
    ui.kbNav = true
    await new Promise((r) => setTimeout(r, 0))
    expect(el().classList.contains('kb-nav')).toBe(true)
    ui.kbNav = false // mousemove 复位
    await new Promise((r) => setTimeout(r, 0))
    expect(el().classList.contains('kb-nav')).toBe(false)
  })
})
