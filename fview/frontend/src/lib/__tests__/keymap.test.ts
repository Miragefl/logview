// resolveKey 纯函数测试(vim-count):数字前缀累积/0 并入/移动键携带 count 发出/
// 非移动键与 Esc 清零/meta 分支清零并走 tabSwitch/clamp 1000/y 序列透传与不吃 count。
import { describe, expect, it } from 'vitest'
import { resolveKey, type KeyState } from '../keymap'

const k = (key: string, mod: Partial<{ ctrlKey: boolean; metaKey: boolean }> = {}) => ({
  key,
  ctrlKey: false,
  ...mod,
})
const st = (): KeyState => ({ pendingZ: false })

describe('resolveKey count 前缀', () => {
  it('数字累积 pending;count>0 时 0 并入(10w 的 10)', () => {
    const s = st()
    expect(resolveKey(k('1'), s)).toEqual({ action: 'pending' })
    expect(s.pendingCount).toBe(1)
    expect(resolveKey(k('0'), s)).toEqual({ action: 'pending' })
    expect(s.pendingCount).toBe(10)
  })

  it('3j → cursorDown 携带 count:3,发完清零', () => {
    const s = st()
    resolveKey(k('3'), s)
    expect(resolveKey(k('j'), s)).toEqual({ action: 'cursorDown', count: 3 })
    expect(s.pendingCount).toBe(0)
  })

  it('5w → wordFwd count:5(词运动吃 count)', () => {
    const s = st()
    resolveKey(k('5'), s)
    expect(resolveKey(k('w'), s)).toEqual({ action: 'wordFwd', count: 5 })
  })

  it('3h/3l → 字符移动吃 count(vim 惯例全量移动键含 h/l)', () => {
    const s = st()
    resolveKey(k('3'), s)
    expect(resolveKey(k('h'), s)).toEqual({ action: 'cursorLeft', count: 3 })
    resolveKey(k('2'), s)
    expect(resolveKey(k('l'), s)).toEqual({ action: 'cursorRight', count: 2 })
  })

  it('无 count 时 0 仍是行首(现状不变)', () => {
    const s = st()
    expect(resolveKey(k('0'), s)).toEqual({ action: 'lineStart' })
  })

  it('Esc 清 count 且不携带(防 count 态卡死)', () => {
    const s = st()
    resolveKey(k('3'), s)
    expect(resolveKey(k('Escape'), s)).toEqual({ action: 'escape' })
    expect(s.pendingCount).toBe(0)
  })

  it('动作键 d 不吃 count,发出即清', () => {
    const s = st()
    resolveKey(k('3'), s)
    expect(resolveKey(k('d'), s)).toEqual({ action: 'detail' })
    expect(s.pendingCount).toBe(0)
  })

  it('count 态下 meta+数字走 tabSwitch 且不带 count(⌘1 切 tab 不受 count 污染)', () => {
    const s = st()
    resolveKey(k('2'), s)
    expect(resolveKey(k('1', { metaKey: true }), s)).toEqual({ action: 'tabSwitch', arg: '1' })
    expect(s.pendingCount).toBe(0)
  })

  it('9999 clamp 1000', () => {
    const s = st()
    for (const d of ['9', '9', '9', '9']) resolveKey(k(d), s)
    expect(s.pendingCount).toBe(1000)
    expect(resolveKey(k('j'), s)).toEqual({ action: 'cursorDown', count: 1000 })
  })

  it('pendingY 透传:y 后数字照常累积,5w 仍生效', () => {
    const s = st()
    resolveKey(k('y'), s) // 进 y 序列
    resolveKey(k('5'), s) // 透传后进 count 累积
    expect(resolveKey(k('w'), s)).toEqual({ action: 'wordFwd', count: 5 })
    expect(s.pendingCount).toBe(0)
  })

  it('3 后 y:y 序列键不吃 count 发出即清,随后 yy 复制照旧', () => {
    const s = st()
    resolveKey(k('3'), s)
    expect(resolveKey(k('y'), s)).toEqual({ action: 'yank' })
    expect(s.pendingCount).toBe(0)
    expect(resolveKey(k('y'), s)).toEqual({ action: 'yankLine' })
  })
})

describe('Q 反向循环源面板', () => {
  it('Q → sourcesBack(反向循环,q 正向保持)', () => {
    const s = st()
    expect(resolveKey(k('Q'), s)).toEqual({ action: 'sourcesBack' })
    expect(resolveKey(k('q'), s)).toEqual({ action: 'sources' })
  })
})
