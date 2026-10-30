import { describe, expect, it } from 'vitest'
import { visibleRange } from './virtual'

describe('visibleRange', () => {
  it('常规窗口：按滚动位置计算可视行区间（含 overscan）', () => {
    // rowH=20，scrollTop=100 → 首行 5；viewport 400px → 20 行；overscan 10 上下扩
    expect(visibleRange(100, 400, 20, 1000, 10)).toEqual([0, 35])
  })

  it('scrollTop=0 从第 0 行开始（start 不为负）', () => {
    expect(visibleRange(0, 400, 20, 1000, 10)).toEqual([0, 30])
  })

  it('末尾 overscan 越界钳制（end ≤ total）', () => {
    // 首行 45 + 20 行 + overscan 10 = 75 > total 50 → 钳到 50
    expect(visibleRange(900, 400, 20, 50, 10)).toEqual([35, 50])
  })

  it('裁头后 total 变小：越界 scrollTop 映射出界给空区间（滚动层负责修正 scrollTop）', () => {
    // total=50 但 scrollTop 巨大（裁剪后数组变短）：start/end 均钳到 total → 空区间
    expect(visibleRange(100000, 400, 20, 50, 10)).toEqual([50, 50])
  })

  it('total=0 返回空区间', () => {
    expect(visibleRange(0, 400, 20, 0, 10)).toEqual([0, 0])
  })

  it('rowH 异常（0/负）防御返回全量', () => {
    expect(visibleRange(0, 400, 0, 5, 10)).toEqual([0, 5])
  })
})
