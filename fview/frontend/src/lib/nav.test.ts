// 光标导航纯函数测试(验收反馈④):钳制移动 + scrolloff 可视性保证。
import { describe, expect, it } from 'vitest'
import { ensureVisible, moveClamp } from './nav'

describe('moveClamp', () => {
  it('越界钳制到 [0, total-1]', () => {
    expect(moveClamp(0, -5, 100)).toBe(0)
    expect(moveClamp(99, 5, 100)).toBe(99)
    expect(moveClamp(50, 3, 100)).toBe(53)
  })
})

describe('ensureVisible', () => {
  // 场景:total=100,first=40,可视 20 行 → 可视 [40,59],scrolloff=3 → 舒适区 [43,56]
  it('光标在舒适区内:不滚动', () => {
    expect(ensureVisible(50, 40, 20, 100, 3)).toBe(40)
    expect(ensureVisible(43, 40, 20, 100, 3)).toBe(40)
  })

  it('光标进入上 scrolloff 圈:上滚让出 3 行余量', () => {
    expect(ensureVisible(41, 40, 20, 100, 3)).toBe(38)
  })

  it('光标到下边缘:下滚让出 3 行余量', () => {
    expect(ensureVisible(59, 40, 20, 100, 3)).toBe(43)
  })

  it('滚动钳制到 [0, total-rowsInView]', () => {
    expect(ensureVisible(2, 40, 20, 100, 3)).toBe(0)
    expect(ensureVisible(98, 40, 20, 100, 3)).toBe(80)
  })
})
