import { describe, expect, it } from 'vitest'
import { levelFilter, hidesFilter, highlightRanges } from './filter'

describe('levelFilter（语义对齐 TUI matchLevelFilter）', () => {
  it('E 档仅错误系', () => {
    expect(levelFilter('E', 'ERROR')).toBe(true)
    expect(levelFilter('E', 'err')).toBe(true)
    expect(levelFilter('E', 'FATAL')).toBe(true)
    expect(levelFilter('E', 'WARN')).toBe(false)
    expect(levelFilter('E', 'INFO')).toBe(false)
  })
  it('W 档错误+警告', () => {
    expect(levelFilter('W', 'ERROR')).toBe(true)
    expect(levelFilter('W', 'WARNING')).toBe(true)
    expect(levelFilter('W', 'INFO')).toBe(false)
  })
  it('I 档去 DEBUG 保留其他', () => {
    expect(levelFilter('I', 'DEBUG')).toBe(false)
    expect(levelFilter('I', 'dbg')).toBe(false)
    expect(levelFilter('I', 'INFO')).toBe(true)
    expect(levelFilter('I', 'WARN')).toBe(true)
    expect(levelFilter('I', 'ERROR')).toBe(true)
  })
  it('D/A 档全部通过', () => {
    for (const lv of ['DEBUG', 'INFO', 'WARN', 'ERROR', '']) {
      expect(levelFilter('D', lv)).toBe(true)
      expect(levelFilter('A', lv)).toBe(true)
    }
  })
})

describe('hidesFilter（对齐 TUI matchHides：忽略大小写包含即隐藏）', () => {
  it('命中任一隐藏词即滤除', () => {
    expect(hidesFilter(['health', 'ping'], 'GET /Health check ok')).toBe(false)
    expect(hidesFilter(['health'], 'GET /ping')).toBe(true)
  })
  it('空隐藏词列表全保留', () => {
    expect(hidesFilter([], 'anything')).toBe(true)
  })
})

describe('highlightRanges', () => {
  it('返回词的全部出现区间（升序）', () => {
    expect(highlightRanges('foo bar foo', ['foo'])).toEqual([
      [0, 3],
      [8, 11],
    ])
  })
  it('多词多区间', () => {
    expect(highlightRanges('a error b warn', ['error', 'warn'])).toEqual([
      [2, 7],
      [10, 14],
    ])
  })
  it('忽略大小写与空词', () => {
    expect(highlightRanges('ERR err', ['err', ''])).toEqual([
      [0, 3],
      [4, 7],
    ])
  })
})
