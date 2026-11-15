// 搜索态 store 测试:常驻条聚焦/词管理框 + 会话级搜索历史
// (cap 20、去重提升到最上、最新在上——与 TUI addSearchHistory 语义一致)。
import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useSearchStore } from './searchStore'

describe('useSearchStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('常驻条聚焦态:focusBar/blurBar 翻转(第六轮 SLS 式改造)', () => {
    const s = useSearchStore()
    expect(s.barFocused).toBe(false)
    s.focusBar()
    expect(s.barFocused).toBe(true)
    s.blurBar()
    expect(s.barFocused).toBe(false)
  })

  it('词管理框态:openWords/closeWords 模式驱动', () => {
    const s = useSearchStore()
    expect(s.wordsShow).toBe('')
    s.openWords('hl')
    expect(s.wordsShow).toBe('hl')
    s.openWords('hide')
    expect(s.wordsShow).toBe('hide')
    s.closeWords()
    expect(s.wordsShow).toBe('')
  })

  it('addSearchHistory:最新在最上,重复词提到最上', () => {
    const s = useSearchStore()
    s.addSearchHistory('error')
    s.addSearchHistory('timeout')
    expect(s.history).toEqual(['timeout', 'error'])
    s.addSearchHistory('error')
    expect(s.history).toEqual(['error', 'timeout'])
  })

  it('addSearchHistory:cap 20,超限丢最旧;空串不入', () => {
    const s = useSearchStore()
    for (let i = 0; i < 23; i++) {
      s.addSearchHistory(`q${i}`)
    }
    expect(s.history).toHaveLength(20)
    expect(s.history[0]).toBe('q22')
    expect(s.history[19]).toBe('q3') // q0/q1/q2 被裁
    s.addSearchHistory('   ')
    expect(s.history[0]).toBe('q22')
  })

  it('seed 通道(field-interact):focusBar 带词写 seedQ,无 seed 不清已有值', () => {
    const s = useSearchStore()
    expect(s.seedQ).toBe('')
    s.focusBar('timeout')
    expect(s.seedQ).toBe('timeout')
    expect(s.barFocused).toBe(true)
    // 无 seed 调用不清已有 seed(只聚焦)
    s.blurBar()
    s.seedQ = 'keep'
    s.focusBar()
    expect(s.seedQ).toBe('keep')
    expect(s.barFocused).toBe(true)
  })

  it('seed 通道(field-interact):openWords 带词写 wordsSeed,无 seed 不清已有值', () => {
    const s = useSearchStore()
    expect(s.wordsSeed).toBe('')
    s.openWords('hl', 'foo')
    expect(s.wordsShow).toBe('hl')
    expect(s.wordsSeed).toBe('foo')
    // 无 seed 调用不清已有 seed(只切模式)
    s.wordsSeed = 'keep'
    s.openWords('hide')
    expect(s.wordsShow).toBe('hide')
    expect(s.wordsSeed).toBe('keep')
  })
})
