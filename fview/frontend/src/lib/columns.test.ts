// 分栏渲染纯函数测试(验收反馈②,TUI logview.go 固定列宽模型)。
import { describe, expect, it } from 'vitest'
import {
  DEFAULT_WIDTHS,
  abbreviateLogger,
  displayWidth,
  fitField,
  fieldsWidths,
  headerCells,
  layoutLine,
} from './columns'
import type { LineDTO } from './types'

describe('abbreviateLogger', () => {
  it('包名段一律取首字母,末段类名全保留(TUI logview.go:697)', () => {
    expect(abbreviateLogger('com.alibaba.foo.Bar')).toBe('c.a.f.Bar')
    expect(abbreviateLogger('com.a.b.Foo')).toBe('c.a.b.Foo')
  })

  it('两段/单段不变形(单段直接原样)', () => {
    expect(abbreviateLogger('com.Foo')).toBe('c.Foo')
    expect(abbreviateLogger('Root')).toBe('Root')
  })
})

describe('fitField', () => {
  it('time 超宽取尾部(保时分秒)', () => {
    expect(fitField('time', '2026-10-30 12:34:56.789', 12)).toBe('12:34:56.789')
  })

  it('logger 先缩写再截断加 …,最终显示宽 = width', () => {
    const out = fitField('logger', 'com.example.order.service.impl.OrderServiceImpl', 20)
    expect(out).toHaveLength(20)
    expect(out.endsWith('…')).toBe(true)
  })

  it("level 短补到 5、超长(如 WARNING)不截", () => {
    expect(fitField('level', 'INFO', 5)).toBe('INFO ')
    expect(fitField('level', 'WARNING', 5)).toBe('WARNING')
  })

  it('空值 = width 个空格占位(列对齐不塌)', () => {
    expect(fitField('thread', '', 10)).toBe('          ')
  })

  it('普通字段超宽截断加 …、右侧补空格到定宽', () => {
    expect(fitField('thread', 'http-nio-8080-exec-1', 10)).toBe('http-nio-…')
    expect(fitField('thread', 'main', 10)).toBe('main      ')
  })

  it('CJK 计宽 2:中文线程名按显示宽截断补齐,不撑破列', () => {
    // 主线程啊ABC 显示宽 8+3=11 > 10 → 头 9 显示宽(主线程啊A) + …
    expect(fitField('thread', '主线程啊ABC', 10)).toBe('主线程啊A…')
    // 线程池一 显示宽 8 < 10 → 补 2 空格
    expect(fitField('thread', '线程池一', 10)).toBe('线程池一  ')
  })
})

describe('layoutLine / headerCells', () => {
  const fields = ['time', 'level', 'thread', 'logger', 'message']
  const ln: LineDTO = {
    seq: 1,
    time: '2026-10-30 12:34:56.789',
    level: 'ERROR',
    thread: '',
    traceId: '',
    logger: 'com.alibaba.foo.Bar',
    message: 'boom',
    raw: 'raw',
  }

  it('空 thread 字段输出 10 空格占位,列对齐保持', () => {
    const cells = layoutLine(ln, fields, DEFAULT_WIDTHS)
    const thread = cells.find((c) => c.field === 'thread')
    expect(thread?.text).toBe(' '.repeat(10))
  })

  it('message 原文输出不定宽', () => {
    const cells = layoutLine(ln, fields, DEFAULT_WIDTHS)
    expect(cells.find((c) => c.field === 'message')?.text).toBe('boom')
  })

  it('表头与数据列宽度一一相等(对齐契约,固定列)', () => {
    const head = headerCells(fields, DEFAULT_WIDTHS)
    const body = layoutLine(ln, fields, DEFAULT_WIDTHS)
    expect(head.length).toBe(body.length)
    for (let i = 0; i < head.length; i++) {
      // level 允许数据行超宽(WARNING 不截);message 不定宽——契约只管其余固定列
      if (body[i].field === 'level' || body[i].field === 'message') continue
      expect(displayWidth(head[i].text)).toBe(displayWidth(body[i].text))
      expect(head[i].field).toBe(body[i].field)
    }
  })

  it('表头固定列显示宽与列宽严格相等(CJK 标签按显示宽补齐)', () => {
    const head = headerCells(['time', 'thread', 'traceId'], DEFAULT_WIDTHS)
    expect(displayWidth(head[0].text)).toBe(DEFAULT_WIDTHS.time)
    expect(displayWidth(head[1].text)).toBe(DEFAULT_WIDTHS.thread)
    expect(displayWidth(head[2].text)).toBe(DEFAULT_WIDTHS.traceId)
  })
})

describe('fieldsWidths', () => {
  it('rules.yaml fields[].width>0 覆盖默认列宽,0/缺省用默认', () => {
    const w = fieldsWidths(
      [
        { name: 'time', visible: true, width: 19 },
        { name: 'level', visible: true, width: 0 },
      ],
      ['time', 'level']
    )
    expect(w.time).toBe(19)
    expect(w.level).toBe(DEFAULT_WIDTHS.level)
  })
})
