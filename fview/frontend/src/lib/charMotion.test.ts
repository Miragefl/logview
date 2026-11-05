// 字符光标移动纯函数(vim-copy):Motion 语义对齐 vim 简化版——字母/数字/下划线
// 为词字符,连续标点为一词,空白跳过;消息首尾停住(不跨行接续)。
// 用例编号 m*/s*/w* 与 TUI internal/tui/charnav_test.go 逐条对齐(双端同表)。
import { describe, expect, it } from 'vitest'
import { applyMotion, charSelText, colToSeg, isWordRune } from './charMotion'

describe('applyMotion 词移动(m 系列)', () => {
  it('m01: hello world 基本词移动', () => {
    expect(applyMotion('hello world', 0, 'wordFwd')).toBe(6)
    expect(applyMotion('hello world', 0, 'wordEnd')).toBe(4)
    expect(applyMotion('hello world', 6, 'wordBack')).toBe(0)
  })

  it('m02: foo=bar, baz 标点词', () => {
    expect(applyMotion('foo=bar, baz', 0, 'wordFwd')).toBe(3) // → '=' 词首
    expect(applyMotion('foo=bar, baz', 3, 'wordFwd')).toBe(4) // '=' → bar 首
    // 光标在 '='(自身即词尾)按 e → 下一词 bar 的尾
    expect(applyMotion('foo=bar, baz', 3, 'wordEnd')).toBe(6)
  })

  it('m03: 空消息/单字符 clamp 不越界', () => {
    expect(applyMotion('', 0, 'wordFwd')).toBe(0)
    expect(applyMotion('', 0, 'end')).toBe(0)
    expect(applyMotion('a', 5, 'wordFwd')).toBe(0)
    expect(applyMotion('a', 0, 'left')).toBe(0)
    expect(applyMotion('a', 0, 'right')).toBe(0)
  })

  it('m04: CJK rune 域(宽度是渲染层的事)', () => {
    expect(applyMotion('中 文 log', 0, 'wordFwd')).toBe(2) // → '文'(idx 2)
    // '中' 即单词尾,e 跳下一词 '文' 尾
    expect(applyMotion('中 文 log', 0, 'wordEnd')).toBe(2)
  })

  it('m05: 越界输入 col clamp', () => {
    expect(applyMotion('hello', 99, 'left')).toBe(3)
    expect(applyMotion('hello', 99, 'start')).toBe(0)
    expect(applyMotion('hello', 99, 'end')).toBe(4)
  })

  it('m06: 0/^/$ 消息域', () => {
    expect(applyMotion('  leading blank', 4, 'start')).toBe(0)
    expect(applyMotion('  leading blank', 4, 'firstNonBlank')).toBe(2)
    expect(applyMotion('  leading blank', 0, 'end')).toBe(14)
    // 全空白行 ^ 回行首
    expect(applyMotion('   ', 2, 'firstNonBlank')).toBe(0)
  })

  it('m07: 消息首尾停住(不跨行接续)', () => {
    expect(applyMotion('abc', 0, 'left')).toBe(0)
    expect(applyMotion('abc', 2, 'right')).toBe(2)
    expect(applyMotion('abc def', 6, 'wordFwd')).toBe(6) // 末词尾按 w 停住
    expect(applyMotion('abc def', 6, 'wordEnd')).toBe(6)
    expect(applyMotion('abc def', 0, 'wordBack')).toBe(0)
  })

  it('m08: 词中按 w 跳下一词首(不落当前词内)', () => {
    expect(applyMotion('abc def', 1, 'wordFwd')).toBe(4)
    expect(applyMotion('abc def', 2, 'wordEnd')).toBe(6) // 已在词尾,e 跳下一词尾
    expect(applyMotion('abc def', 1, 'wordBack')).toBe(0) // 词中 b 回词首
  })
})

describe('isWordRune 词字符判定', () => {
  it('字母/数字/下划线/CJK 字母是词字符,标点空白不是', () => {
    expect(isWordRune('a')).toBe(true)
    expect(isWordRune('Z')).toBe(true)
    expect(isWordRune('5')).toBe(true)
    expect(isWordRune('_')).toBe(true)
    expect(isWordRune('中')).toBe(true)
    expect(isWordRune('=')).toBe(false)
    expect(isWordRune(' ')).toBe(false)
    expect(isWordRune(',')).toBe(false)
  })
})

describe('charSelText 字符选区子串(s 系列)', () => {
  it('s01: 跨行——首行[startCol..]、中间行全量、尾行[..endCol]', () => {
    const r = charSelText(['hello world', 'second line', 'tail'], 6, 2)
    expect(r.text).toBe('world\nsecond line\ntai')
    expect(r.chars).toBe(21) // 含换行 rune 计数
  })

  it('s02: 单行子串[startCol..endCol]', () => {
    expect(charSelText(['abcd'], 1, 4).text).toBe('bcd')
    expect(charSelText(['abcd'], 0, 0).text).toBe('a')
  })

  it('s03: 空消息行产出空串', () => {
    const r = charSelText(['a', '', 'b'], 0, 0)
    expect(r.text).toBe('a\n\nb')
    expect(r.chars).toBe(4) // a + \n + \n + b
  })

  it('s04: 列 clamp——startCol/endCol 越界压回行内', () => {
    expect(charSelText(['abc'], 0, 99).text).toBe('abc')
    expect(charSelText(['abc', 'def'], 99, 99).text).toBe('c\ndef')
  })
})

describe('colToSeg wrap 段投影(w 系列)', () => {
  it('w01: rune col → 段号+段内偏移,越界 clamp 末段尾', () => {
    expect(colToSeg(['hel', 'lo w', 'orld'], 0)).toEqual({ segIdx: 0, colInSeg: 0 })
    expect(colToSeg(['hel', 'lo w', 'orld'], 3)).toEqual({ segIdx: 1, colInSeg: 0 })
    expect(colToSeg(['hel', 'lo w', 'orld'], 10)).toEqual({ segIdx: 2, colInSeg: 3 })
    expect(colToSeg(['hel', 'lo w', 'orld'], 99)).toEqual({ segIdx: 2, colInSeg: 3 })
    // 段内中段:col 5 → 第 2 段(起于 3)内偏移 2
    expect(colToSeg(['hel', 'lo w', 'orld'], 5)).toEqual({ segIdx: 1, colInSeg: 2 })
  })

  it('w02: 空段数组与空消息安全', () => {
    expect(colToSeg([''], 0)).toEqual({ segIdx: 0, colInSeg: 0 })
  })
})
