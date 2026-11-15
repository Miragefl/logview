// hotSorted/isHot 纯函数测试(panel-hot-sort):分降序+同分/无分名称升序/缺失与 0
// 同为无热度/stable/isHot 0.99 阈值(对齐 TUI usage.go 亚秒幻影衰减口径)。
import { describe, expect, it } from 'vitest'
import { hotSorted, isHot } from '../hotSort'

interface It {
  k: string
  tag?: string
}
const keyOf = (it: It) => it.k

describe('hotSorted 热度排序', () => {
  it('有分在前无分在后', () => {
    const items: It[] = [{ k: 'a' }, { k: 'b' }, { k: 'c' }]
    const out = hotSorted(items, keyOf, { b: 3 })
    expect(out.map((x) => x.k)).toEqual(['b', 'a', 'c'])
  })

  it('同分/无分按 key 名称升序', () => {
    const items: It[] = [{ k: 'delta' }, { k: 'alpha' }, { k: 'charlie' }]
    const out = hotSorted(items, keyOf, { charlie: 2, alpha: 2 })
    expect(out.map((x) => x.k)).toEqual(['alpha', 'charlie', 'delta'])
  })

  it('分数缺失(undefined)与 0 同为无热度', () => {
    const items: It[] = [{ k: 'a' }, { k: 'b' }]
    const out = hotSorted(items, keyOf, { a: 0 })
    expect(out.map((x) => x.k)).toEqual(['a', 'b']) // 同级按名称
    const out2 = hotSorted(items, keyOf, { b: 0.5 })
    expect(out2.map((x) => x.k)).toEqual(['b', 'a'])
  })

  it('stable:同 key 附加字段不同,保持原相对序', () => {
    const items: It[] = [
      { k: 'same', tag: 'x' },
      { k: 'same', tag: 'y' },
    ]
    const out = hotSorted(items, keyOf, { same: 2 })
    expect(out.map((x) => x.tag)).toEqual(['x', 'y'])
  })

  it('isHot 0.99 阈值(TUI 同款亚秒衰减口径)', () => {
    expect(isHot(0.9899)).toBe(false)
    expect(isHot(0.99)).toBe(true)
    expect(isHot(0)).toBe(false)
  })
})
