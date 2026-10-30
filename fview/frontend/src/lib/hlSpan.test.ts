import { describe, expect, it } from 'vitest'
import { splitSpans } from './hlSpan'

describe('splitSpans', () => {
  it('无关键词：原样单段无标记', () => {
    expect(splitSpans('hello world', [], new Set())).toEqual([{ text: 'hello world' }])
  })

  it('高亮词分段：带对应颜色（亮色粗体下划线域）', () => {
    const out = splitSpans('error in payment', [{ word: 'error', color: '#e57373' }], new Set())
    expect(out).toEqual([
      { text: 'error', hl: '#e57373' },
      { text: ' in payment' },
    ])
  })

  it('搜索命中词：带 hit 标记（暗色底纹域），与高亮互不混淆', () => {
    const out = splitSpans('timeout after retry', [], new Set(['retry']))
    expect(out).toEqual([{ text: 'timeout after ' }, { text: 'retry', hit: true }])
  })

  it('高亮与命中同现：各归各域', () => {
    const out = splitSpans(
      'error caused by timeout',
      [{ word: 'error', color: '#e57373' }],
      new Set(['timeout'])
    )
    expect(out).toEqual([
      { text: 'error', hl: '#e57373' },
      { text: ' caused by ' },
      { text: 'timeout', hit: true },
    ])
  })

  it('重叠取先：更早开始的词占位，后词被截断', () => {
    // "ab" 高亮 与 "bc" 命中重叠在 "abc" 上："ab" 先开始占前两字符
    const out = splitSpans('abc', [{ word: 'ab', color: '#e57373' }], new Set(['bc']))
    expect(out).toEqual([{ text: 'ab', hl: '#e57373' }, { text: 'c' }])
  })

  it('同位置高亮词优先于命中词（双域区分优先展示用户标注）', () => {
    const out = splitSpans('err', [{ word: 'err', color: '#e57373' }], new Set(['err']))
    expect(out).toEqual([{ text: 'err', hl: '#e57373' }])
  })

  it('一词多次出现全部分段', () => {
    const out = splitSpans('a b a', [{ word: 'a', color: '#7ee29a' }], new Set())
    expect(out).toEqual([
      { text: 'a', hl: '#7ee29a' },
      { text: ' b ' },
      { text: 'a', hl: '#7ee29a' },
    ])
  })
})
