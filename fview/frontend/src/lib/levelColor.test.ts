import { describe, expect, it } from 'vitest'
import { levelClass } from './levelColor'

describe('levelClass', () => {
  it('错误系映射 lv-error', () => {
    expect(levelClass('ERROR')).toBe('lv-error')
    expect(levelClass('err')).toBe('lv-error')
    expect(levelClass('FATAL')).toBe('lv-error')
  })
  it('警告系映射 lv-warn', () => {
    expect(levelClass('WARN')).toBe('lv-warn')
  })
  it('INFO/DEBUG 各自映射', () => {
    expect(levelClass('INFO')).toBe('lv-info')
    expect(levelClass('DEBUG')).toBe('lv-debug')
  })
  it('空与未知级别无样式', () => {
    expect(levelClass('')).toBe('')
    expect(levelClass('TRACE')).toBe('')
  })
})
