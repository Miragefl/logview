// useTypeFilter composable 测试(r2 打字过滤):字母进词/修饰键互斥/退格删尾/Esc 清词消费。
// 词存 ui.panelFilter(会话级,SidebarRail Esc 逐级依据)——需 pinia。
// useLocalTypeFilter(browse-typeahead):BrowseDialog 弹窗本地词版——不进面板共享词。
import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { typeFilterKey, useLocalTypeFilter, useTypeFilter } from '../sources/useTypeFilter'
import { useUiStore } from '../../stores/ui'

function ev(key: string, opts: KeyboardEventInit = {}) {
  return new KeyboardEvent('keydown', { key, ...opts })
}

describe('useTypeFilter 打字过滤', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('可打印无修饰字符依次进词并消费', () => {
    const { q, typeKey } = useTypeFilter()
    expect(typeKey(ev('d'))).toBe(true)
    expect(typeKey(ev('e'))).toBe(true)
    expect(typeKey(ev('v'))).toBe(true)
    expect(q.value).toBe('dev')
    // 空格是路径/主机名合法字符,照进词
    expect(typeKey(ev(' '))).toBe(true)
    expect(q.value).toBe('dev ')
  })

  it('ctrl/meta/alt 修饰字符一律不进词不消费(与 ⌃j/k/a/d/e 天然互斥)', () => {
    const { q, typeKey } = useTypeFilter()
    expect(typeKey(ev('d', { ctrlKey: true }))).toBe(false)
    expect(typeKey(ev('e', { metaKey: true }))).toBe(false)
    expect(typeKey(ev('a', { altKey: true }))).toBe(false)
    expect(q.value).toBe('')
  })

  it('Backspace 删尾;词空时 Backspace 不消费', () => {
    const { q, typeKey } = useTypeFilter()
    typeKey(ev('a'))
    typeKey(ev('b'))
    expect(typeKey(ev('Backspace'))).toBe(true)
    expect(q.value).toBe('a')
    typeKey(ev('Backspace'))
    expect(q.value).toBe('')
    expect(typeKey(ev('Backspace'))).toBe(false)
  })

  it('Esc 词非空清空并消费;词空不消费(归 SidebarRail 关面板)', () => {
    const { q, typeKey } = useTypeFilter()
    typeKey(ev('x'))
    expect(typeKey(ev('Escape'))).toBe(true)
    expect(q.value).toBe('')
    expect(typeKey(ev('Escape'))).toBe(false)
  })

  it('多字符键(Enter/方向键/Tab)不进词不消费', () => {
    const { q, typeKey } = useTypeFilter()
    expect(typeKey(ev('Enter'))).toBe(false)
    expect(typeKey(ev('ArrowDown'))).toBe(false)
    expect(typeKey(ev('Tab'))).toBe(false)
    expect(q.value).toBe('')
  })
})

describe('typeFilterKey 纯函数', () => {
  it('可打印字符拼接;Backspace 去尾;Esc 清空;均返回新词', () => {
    expect(typeFilterKey(ev('l'), '')).toBe('l')
    expect(typeFilterKey(ev('o'), 'l')).toBe('lo')
    expect(typeFilterKey(ev('Backspace'), 'lo')).toBe('l')
    expect(typeFilterKey(ev('Escape'), 'lo')).toBe('')
  })

  it('修饰键与多字符键不消费返回 null', () => {
    expect(typeFilterKey(ev('j', { ctrlKey: true }), '')).toBeNull()
    expect(typeFilterKey(ev('ArrowUp'), 'lo')).toBeNull()
    expect(typeFilterKey(ev('F5'), '')).toBeNull()
    // 词空时 Backspace/Esc 也不消费
    expect(typeFilterKey(ev('Backspace'), '')).toBeNull()
    expect(typeFilterKey(ev('Escape'), '')).toBeNull()
  })
})

describe('useLocalTypeFilter 本地词', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('打字进词/退格删尾/Esc 清空不消费关窗键——与面板版同语义', () => {
    const { q, typeKey } = useLocalTypeFilter()
    expect(typeKey(ev('l'))).toBe(true)
    expect(typeKey(ev('o'))).toBe(true)
    expect(typeKey(ev('g'))).toBe(true)
    expect(q.value).toBe('log')
    expect(typeKey(ev('Backspace'))).toBe(true)
    expect(q.value).toBe('lo')
    expect(typeKey(ev('Escape'))).toBe(true)
    expect(q.value).toBe('')
    expect(typeKey(ev('Escape'))).toBe(false)
  })

  it('本地词不串面板共享词:打词后 store.panelFilter 仍空', () => {
    const ui = useUiStore()
    const { q, typeKey } = useLocalTypeFilter()
    typeKey(ev('a'))
    typeKey(ev('b'))
    expect(q.value).toBe('ab')
    expect(ui.panelFilter).toBe('')
  })
})
