import { describe, expect, it } from 'vitest'
import { effectiveFields } from './fieldvis'

// 列按解析器能力显隐(C 方案):configured ∩ parserFields + message 恒在,不随数据抖动
describe('effectiveFields', () => {
  it('parserFields 为 null(老后端/mock 未实现)→ 原样返回不裁剪', () => {
    expect(effectiveFields(['time', 'level', 'message'], null)).toEqual(['time', 'level', 'message'])
  })

  it('按能力集裁剪,保持 configured 序', () => {
    expect(effectiveFields(['time', 'level', 'thread', 'message'], ['level', 'message'])).toEqual(['level', 'message'])
  })

  it('message 恒显:能力集没有 message 也保留', () => {
    expect(effectiveFields(['time', 'message'], ['time'])).toEqual(['time', 'message'])
  })

  it('plain-text 能力集只剩 message → 只显示内容列(需求①核心场景)', () => {
    expect(effectiveFields(['time', 'level', 'thread', 'traceId', 'logger', 'message'], ['message'])).toEqual(['message'])
  })

  it('message 恒显兜底:configured 不含 message 时尾部补上(坏配置自救,spec 拍板项②)', () => {
    expect(effectiveFields(['time', 'level'], null)).toEqual(['time', 'level', 'message'])
    expect(effectiveFields(['time', 'level'], ['time', 'level'])).toEqual(['time', 'level', 'message'])
  })
})
