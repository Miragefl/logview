import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useUiStore } from './ui'

describe('useUiStore', () => {
  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia())
  })

  it('默认值：字号 13 / 行高 1.5 / 等宽字体', () => {
    const s = useUiStore()
    expect(s.fontSize).toBe(13)
    expect(s.lineHeight).toBe(1.5)
    expect(s.fontFamily).toBeTruthy()
  })

  it('persist 落 localStorage（fview-ui 键），新 store 实例恢复', () => {
    const s = useUiStore()
    s.fontSize = 15
    s.lineHeight = 1.8
    s.persist()

    const raw = localStorage.getItem('fview-ui')
    expect(raw).toBeTruthy()
    expect(JSON.parse(raw!).fontSize).toBe(15)

    // 模拟重启：新 pinia 实例从盘恢复
    setActivePinia(createPinia())
    const s2 = useUiStore()
    expect(s2.fontSize).toBe(15)
    expect(s2.lineHeight).toBe(1.8)
  })

  it('弹窗栈:open/close 维护 modalOpen 与 topDialog,重复 open 不叠(第三轮验收②)', () => {
    const s = useUiStore()
    expect(s.modalOpen).toBe(false)
    expect(s.topDialog).toBe('')
    s.dialogOpen('search')
    s.dialogOpen('settings')
    s.dialogOpen('search') // 重复登记幂等
    expect(s.dialogs).toHaveLength(2)
    expect(s.modalOpen).toBe(true)
    expect(s.topDialog).toBe('settings')
    s.dialogClose('settings')
    expect(s.topDialog).toBe('search')
    s.dialogClose('search')
    expect(s.modalOpen).toBe(false)
  })
})

// uiFontSize 界面字号(第九轮验收③:非日志区文字可调,默认 12)
describe('uiFontSize', () => {
  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia()) // 新实例:loadPersisted 才会重读 localStorage
  })

  it('默认 12;persist 往返保留', () => {
    const s = useUiStore()
    expect(s.uiFontSize).toBe(12)
    s.uiFontSize = 15
    s.persist()
    const s2 = useUiStore()
    expect(s2.uiFontSize).toBe(15)
  })

  it('旧数据无 uiFontSize 字段回落 12', () => {
    localStorage.setItem('fview-ui', JSON.stringify({ fontFamily: 'x', fontSize: 13, lineHeight: 1.5 }))
    const s = useUiStore()
    expect(s.uiFontSize).toBe(12)
  })
})
