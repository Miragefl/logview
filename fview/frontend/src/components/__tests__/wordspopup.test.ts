// WordsPopup 高亮/隐藏词管理框测试(第六轮 SLS 式改造):
// 模式驱动/加词/词表键盘流(ctrl+j/k 选中,d 删词)/Esc 关框/弹窗栈登记。
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import naive, { NModal } from 'naive-ui'

vi.mock('../../lib/backend', () => ({
  usageKeywords: vi.fn().mockResolvedValue(['freq-word']),
  bumpKeyword: vi.fn().mockResolvedValue(undefined),
}))

import WordsPopup from '../logview/WordsPopup.vue'
import { useSearchStore } from '../../stores/searchStore'
import { useAnnotationsStore } from '../../stores/annotations'
import { useUiStore } from '../../stores/ui'
import { useTabsStore } from '../../stores/tabs'

function key(k: string, opts: KeyboardEventInit = {}) {
  document.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true, ...opts }))
}

function setWord(v: string) {
  const input = document.body.querySelector('.wordspopup input')
  if (!input) throw new Error('wordspopup input 不存在')
  ;(input as HTMLInputElement).value = v
  input.dispatchEvent(new Event('input', { bubbles: true }))
}

describe('WordsPopup 词管理框', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    useTabsStore().applyOpened({ tabId: 't1', label: 'local:a.log' })
  })
  const wrappers: ReturnType<typeof mount>[] = []
  afterEach(() => {
    wrappers.forEach((w) => w.unmount())
    wrappers.length = 0
    document.body.innerHTML = ''
  })

  it('hl 模式:Enter 加词,词表 ctrl+j 选中 + d 删除', async () => {
    const search = useSearchStore()
    const ann = useAnnotationsStore()
    search.openWords('hl')
    wrappers.push(mount(WordsPopup, { global: { plugins: [naive] } }))
    await new Promise((r) => setTimeout(r, 0))

    setWord('alpha')
    key('Enter')
    await new Promise((r) => setTimeout(r, 0))
    expect(ann.highlights.map((h) => h.word)).toEqual(['alpha'])

    // 键盘选中第一条再删
    key('j', { ctrlKey: true })
    await new Promise((r) => setTimeout(r, 0))
    expect(document.querySelectorAll('.wordspopup .words li.sel').length).toBe(1)
    key('d')
    await new Promise((r) => setTimeout(r, 0))
    expect(ann.highlights).toHaveLength(0)
  })

  it('hide 模式:加词进 hideRules', async () => {
    const search = useSearchStore()
    const ann = useAnnotationsStore()
    search.openWords('hide')
    wrappers.push(mount(WordsPopup, { global: { plugins: [naive] } }))
    await new Promise((r) => setTimeout(r, 0))

    setWord('noise')
    key('Enter')
    await new Promise((r) => setTimeout(r, 0))
    expect(ann.hideRules).toEqual(['noise'])
  })

  it('Esc 关框:wordsShow 清空', async () => {
    const search = useSearchStore()
    search.openWords('hl')
    wrappers.push(mount(WordsPopup, { global: { plugins: [naive] } }))
    await new Promise((r) => setTimeout(r, 0))

    key('Escape')
    await new Promise((r) => setTimeout(r, 0))
    expect(search.wordsShow).toBe('')
  })

  it('弹窗栈登记:开时入栈,关时出栈(全局 vim 键让位)', async () => {
    const search = useSearchStore()
    const ui = useUiStore()
    search.openWords('hl')
    wrappers.push(mount(WordsPopup, { global: { plugins: [naive] } }))
    await new Promise((r) => setTimeout(r, 0))
    expect(ui.dialogs).toContain('words')
    expect(ui.modalOpen).toBe(true)

    search.closeWords()
    await new Promise((r) => setTimeout(r, 0))
    expect(ui.dialogs).not.toContain('words')
  })

  it('输入框聚焦时打字不被吞:d 进输入框,不误删选中词(C2)', async () => {
    const search = useSearchStore()
    const ann = useAnnotationsStore()
    search.openWords('hl')
    wrappers.push(mount(WordsPopup, { global: { plugins: [naive] } }))
    await new Promise((r) => setTimeout(r, 0))

    setWord('alpha')
    key('Enter')
    await new Promise((r) => setTimeout(r, 0))
    // ctrl+j 选中词表第一条
    key('j', { ctrlKey: true })
    await new Promise((r) => setTimeout(r, 0))

    // 焦点回输入框打 "debug":d/x 字符必须进框,不得拦截删词
    const input = document.body.querySelector('.wordspopup input') as HTMLInputElement
    input.focus()
    setWord('debug')
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'd', bubbles: true, cancelable: true }))
    await new Promise((r) => setTimeout(r, 0))
    expect(ann.highlights.map((h) => h.word)).toEqual(['alpha'])
  })

  it('NModal 关闭原生 esc/自动聚焦,Esc 分层与初始焦点自理(I1)', async () => {
    const search = useSearchStore()
    search.openWords('hl')
    const w = mount(WordsPopup, { global: { plugins: [naive] } })
    wrappers.push(w)
    await new Promise((r) => setTimeout(r, 0))
    const modal = w.findComponent(NModal)
    expect(modal.exists()).toBe(true)
    expect(modal.props('closeOnEsc')).toBe(false)
    expect(modal.props('autoFocus')).toBe(false)
  })

  it('卸载清 wordsShow:关 tab 后重挂不自动弹(I2)', async () => {
    const search = useSearchStore()
    search.openWords('hl')
    const w = mount(WordsPopup, { global: { plugins: [naive] } })
    await new Promise((r) => setTimeout(r, 0))
    // 模拟"管理框开着时关掉最后一个 tab"(App v-if 卸载组件)
    w.unmount()
    expect(search.wordsShow).toBe('')
  })
})
