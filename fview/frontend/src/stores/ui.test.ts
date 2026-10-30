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
})
