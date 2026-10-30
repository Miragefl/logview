// 换行引擎纯函数测试(验收反馈④ w,变高行虚拟滚动):
// wrapSegments 按 displayWidth 切段(CJK 计 2);HeightIndex 前缀和+二分定位。
import { describe, expect, it } from 'vitest'
import { HeightIndex, visibleRows, wrapSegments } from './wrap'

describe('wrapSegments', () => {
  it('ASCII 60 字符 width 30 → 2 段', () => {
    expect(wrapSegments('a'.repeat(60), 30)).toEqual(['a'.repeat(30), 'a'.repeat(30)])
  })

  it('中文 30 字 width 30 → 2 段(每段 15 字,CJK 计宽 2)', () => {
    const out = wrapSegments('中'.repeat(30), 30)
    expect(out).toHaveLength(2)
    expect(out[0]).toBe('中'.repeat(15))
  })

  it('中英混排:宽 2 字符不切半,整字换行不溢出', () => {
    // abc=3,+中(2)=5 > 4 → 'abc' 成段,'中' 开新段
    expect(wrapSegments('abc中', 4)).toEqual(['abc', '中'])
  })

  it('空串 → [""];width≤0 原样不切', () => {
    expect(wrapSegments('', 30)).toEqual([''])
    expect(wrapSegments('abc', 0)).toEqual(['abc'])
  })
})

describe('HeightIndex', () => {
  const hi = new HeightIndex([1, 2, 1], 20) // 行高:20/40/20,total 80

  it('total/offsetOf 正确', () => {
    expect(hi.total()).toBe(80)
    expect(hi.offsetOf(0)).toBe(0)
    expect(hi.offsetOf(1)).toBe(20)
    expect(hi.offsetOf(2)).toBe(60)
  })

  it('locate:offsetPx 所在行;越界钳制', () => {
    expect(hi.locate(0)).toBe(0)
    expect(hi.locate(20)).toBe(1)
    expect(hi.locate(59)).toBe(1)
    expect(hi.locate(60)).toBe(2)
    expect(hi.locate(9999)).toBe(2)
  })

  it('offsetOf/locate 互逆:行顶 ≤ 命中 offset < 行底', () => {
    for (let px = 0; px < 80; px += 7) {
      const i = hi.locate(px)
      expect(hi.offsetOf(i)).toBeLessThanOrEqual(px)
      expect(hi.offsetOf(i) + hi.rowsAt(i) * 20).toBeGreaterThan(px)
    }
  })

  it('单行退化与空数组', () => {
    expect(new HeightIndex([1], 20).total()).toBe(20)
    const empty = new HeightIndex([], 20)
    expect(empty.total()).toBe(0)
    expect(empty.locate(0)).toBe(0)
  })
})

describe('visibleRows(变高可视区)', () => {
  it('按高度覆盖 viewport+overscan,含变高行', () => {
    const hi = new HeightIndex([1, 1, 1, 1, 1, 1], 10)
    const [s, e] = visibleRows(hi, 0, 25, 2)
    expect(s).toBe(0)
    expect(e).toBeGreaterThanOrEqual(4) // 3 行视口 + overscan
  })

  it('中部偏移定位正确', () => {
    const hi = new HeightIndex([1, 1, 1, 1], 10)
    const [s, e] = visibleRows(hi, 25, 20, 1)
    expect(s).toBeGreaterThanOrEqual(1)
    expect(hi.offsetOf(e - 1)).toBeLessThan(25 + 20 + 10)
  })
})
