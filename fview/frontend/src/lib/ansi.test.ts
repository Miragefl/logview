import { describe, expect, it } from 'vitest'
import { ansiToHtml } from './ansi'

describe('ansiToHtml', () => {
  it('基础前景色 + reset', () => {
    expect(ansiToHtml('\x1b[31mfoo\x1b[0m')).toBe('<span class="fg-red">foo</span>')
  })

  it('无转义文本原样返回', () => {
    expect(ansiToHtml('plain text')).toBe('plain text')
  })

  it('reset 后的文本不再着色', () => {
    expect(ansiToHtml('\x1b[31mfoo\x1b[0mbar')).toBe('<span class="fg-red">foo</span>bar')
  })

  it('组合码（加粗+颜色）', () => {
    expect(ansiToHtml('\x1b[1;32mok\x1b[0m')).toBe('<span class="bold fg-green">ok</span>')
  })

  it('未闭合 reset：尾段仍包裹当前样式（兜底）', () => {
    expect(ansiToHtml('\x1b[31munterminated')).toBe('<span class="fg-red">unterminated</span>')
  })

  it('亮色码（90-97）', () => {
    expect(ansiToHtml('\x1b[91mhot\x1b[0m')).toBe('<span class="fg-bright-red">hot</span>')
  })

  it('多段切换颜色', () => {
    expect(ansiToHtml('\x1b[31ma\x1b[0m\x1b[32mb\x1b[0m')).toBe(
      '<span class="fg-red">a</span><span class="fg-green">b</span>'
    )
  })
})
