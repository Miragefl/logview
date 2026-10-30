import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { HL_COLORS, useAnnotationsStore } from './annotations'
import type { LineDTO } from '../lib/types'

function ln(message: string, raw = message): LineDTO {
  return { seq: 1, time: '', level: '', thread: '', traceId: '', logger: '', message, raw }
}

describe('useAnnotationsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('hidden：任一规则是 message 或 raw 子串即隐藏；无规则全不隐藏', () => {
    const s = useAnnotationsStore()
    expect(s.hidden(ln('everything visible'))).toBe(false)

    s.addHideRule('health-check')
    expect(s.hidden(ln('GET /health-check 200'))).toBe(true)
    expect(s.hidden(ln('INFO Server started'))).toBe(false)
  })

  it('hidden：规则匹配 raw 字段（message 为空的行也按 raw 判）', () => {
    const s = useAnnotationsStore()
    s.addHideRule('raw-token')
    expect(s.hidden(ln('', 'plain raw-token line'))).toBe(true)
  })

  it('addHighlight 10 色轮换：第 11 个词回到第 1 色；重复词不重复加', () => {
    const s = useAnnotationsStore()
    for (let i = 0; i < 11; i++) {
      s.addHighlight(`w${i}`)
    }
    expect(s.highlights).toHaveLength(11)
    expect(s.highlights[0].color).toBe(HL_COLORS[0])
    expect(s.highlights[10].color).toBe(HL_COLORS[10 % 10])

    s.addHighlight('w0') // 已存在，不重复
    expect(s.highlights).toHaveLength(11)
  })

  it('hideRules 去重与清空', () => {
    const s = useAnnotationsStore()
    s.addHideRule('x')
    s.addHideRule('x')
    expect(s.hideRules).toEqual(['x'])
    s.clearHideRules()
    expect(s.hideRules).toEqual([])
  })
})
