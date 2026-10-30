import { describe, expect, it } from 'vitest'
import { detectBase, toNaiveOverrides } from './index'

describe('detectBase', () => {
  it('深色背景判 dark', () => {
    expect(detectBase({ '--bg': '#10161f' })).toBe('dark')
    expect(detectBase({ '--bg': '#1a2230' })).toBe('dark')
  })

  it('浅色背景判 light', () => {
    expect(detectBase({ '--bg': '#f5f5f5' })).toBe('light')
    expect(detectBase({ '--bg': '#ffffff' })).toBe('light')
  })

  it('缺失/非法/空值回落 dark（后端保证非空，此处兜底）', () => {
    expect(detectBase({})).toBe('dark')
    expect(detectBase({ '--bg': '' })).toBe('dark')
    expect(detectBase({ '--bg': 'not-a-color' })).toBe('dark')
    expect(detectBase({ '--bg': '#xyz' })).toBe('dark')
  })

  it('临界值：中等亮度按暗处理（宁可暗底配浅字也不刺眼）', () => {
    expect(detectBase({ '--bg': '#808080' })).toBe('dark')
  })
})

describe('toNaiveOverrides', () => {
  const vars: Record<string, string> = {
    '--bg': '#10161f',
    '--fg': '#e6e6e6',
    '--level-error': '#e57373',
    '--level-warn': '#e2c77e',
    '--level-info': '#7ee29a',
    '--level-debug': '#8b98a9',
    '--time': '#5d7292',
    '--accent': '#4a8fe0',
    '--dim': '#5d7292',
    '--highlight': '#7dc4e4',
    '--popup-border': '#2a3a4f',
    '--popup-bg': '#16202e',
  }

  it('七个关键键映射到 Naive UI overrides', () => {
    const o = toNaiveOverrides(vars)
    expect(o.common?.bodyColor).toBe('#10161f')
    expect(o.common?.textColorBase).toBe('#e6e6e6')
    expect(o.common?.primaryColor).toBe('#4a8fe0')
    expect(o.common?.popoverColor).toBe('#16202e')
    expect(o.common?.cardColor).toBe('#16202e')
    expect(o.common?.borderColor).toBe('#2a3a4f')
    expect(o.common?.textColorDisabled).toBe('#5d7292')
    expect(o.common?.primaryColorHover).toBe('#7dc4e4')
  })

  it('缺变量不产出 undefined 色值（跳过缺失键）', () => {
    const o = toNaiveOverrides({ '--bg': '#10161f' })
    expect(o.common?.bodyColor).toBe('#10161f')
    expect(o.common?.primaryColor).toBeUndefined()
  })
})
