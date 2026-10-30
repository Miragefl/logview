// 统一搜索弹窗 store 测试(验收反馈③):弹窗态 + 会话级搜索历史
// (cap 20、去重提升到最上、最新在上——与 TUI addSearchHistory 语义一致)。
import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useSearchStore } from './searchStore'

describe('useSearchStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('弹窗态:open 切分区/close 收起/tab 轮换', () => {
    const s = useSearchStore()
    expect(s.show).toBe(false)
    s.open(1)
    expect(s.show).toBe(true)
    expect(s.tab).toBe(1)
    s.close()
    expect(s.show).toBe(false)
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
})
