// LogView 虚拟滚动 mount 冒烟(Final Review #1/#6 回归):
// 行定位不得双重补偿;traceId 列类名与样式选择器必须配对。
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import naive from 'naive-ui'
// vite ?raw 资源(SFC 源码字符串,断言样式选择器配对)
import logViewSrc from '../logview/LogView.vue?raw'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

vi.mock('../../lib/backend', () => ({
  getLine: vi.fn().mockResolvedValue({}),
  setFollow: vi.fn().mockResolvedValue(undefined),
  bumpKeyword: vi.fn().mockResolvedValue(undefined),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({
  ClipboardSetText: vi.fn().mockResolvedValue(undefined),
}))
vi.stubGlobal('ResizeObserver', class {
  observe() {}
  unobserve() {}
  disconnect() {}
})

import LogView from '../logview/LogView.vue'
import { ClipboardSetText } from '../../../wailsjs/runtime/runtime'
import { useTabsStore } from '../../stores/tabs'
import { useUiStore } from '../../stores/ui'
import { useAnnotationsStore } from '../../stores/annotations'
import type { LineDTO } from '../../lib/types'

function ln(seq: number, message: string, traceId = ''): LineDTO {
  return { seq, time: '12:00:00.001', level: 'INFO', thread: 'main', traceId, logger: 'a.B.C', message, raw: message }
}

const PROPS = {
  hitSeqs: new Set<number>(),
  hitWords: [] as string[],
  levelMode: 'A' as const,
  fields: ['time', 'level', 'thread', 'traceId', 'logger', 'message'],
}

describe('LogView mount 冒烟', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('行定位无双重补偿:.rows 不带 translateY,行 top 为全局偏移(Final #1)', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: [ln(1, 'a'), ln(2, 'b'), ln(3, 'c')] })
    const ui = useUiStore()

    const w = mount(LogView, { props: PROPS, global: { plugins: [naive] } })
    await new Promise((r) => setTimeout(r, 0))

    const rows = w.find('.rows').element as HTMLElement
    expect(rows.style.transform).toBe('') // translateY 与全局 top 不得叠加
    const rowEls = w.findAll('.row:not(.header)')
    expect(rowEls.length).toBeGreaterThanOrEqual(3)
    const px = (el: Element) => (el as HTMLElement).style.top
    expect(px(rowEls[0].element)).toBe('0px')
    expect(px(rowEls[1].element)).toBe(`${ui.rowPx()}px`)
    expect(px(rowEls[2].element)).toBe(`${ui.rowPx() * 2}px`)
  })

  it('traceId 列类名 f-traceId 与样式选择器配对,--trace 生效(Final #6)', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: [ln(1, 'a', 'abc123')] })
    const w = mount(LogView, { props: PROPS, global: { plugins: [naive] } })
    await new Promise((r) => setTimeout(r, 0))

    // 表头也带 f-traceId 类,取数据行的
    const traceCells = w.findAll('.row:not(.header) .f-traceId')
    expect(traceCells.length).toBeGreaterThanOrEqual(1)
    expect(traceCells[0].text()).toContain('abc123')
    // 样式表必须有对应选择器(类名拼错则 --trace 永不生效)
    expect(logViewSrc).toMatch(/\.f-traceId\s*\{/)
    expect(logViewSrc).toMatch(/--trace/)
  })

  it('搜索过滤:hitSeqs 非空时只渲染命中行,空集恢复全量(第三轮验收⑤,对齐 TUI recomputeView)', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: [ln(1, 'alpha'), ln(2, 'beta'), ln(3, 'alpha-2')] })

    const w = mount(LogView, {
      props: { ...PROPS, hitSeqs: new Set([2]) },
      global: { plugins: [naive] },
    })
    await new Promise((r) => setTimeout(r, 0))
    let rows = w.findAll('.row:not(.header)')
    expect(rows).toHaveLength(1)
    expect(rows[0].text()).toContain('beta')

    await w.setProps({ hitSeqs: new Set<number>() })
    await new Promise((r) => setTimeout(r, 0))
    rows = w.findAll('.row:not(.header)')
    expect(rows).toHaveLength(3)
  })

  it('source 字段可见时行内容不消失——fitField 无宽字段不再 RangeError(第三轮验收⑥)', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({
      tabId: 't1',
      lines: [{ ...ln(1, 'hello-msg'), source: 'local:a.log' } as LineDTO],
    })
    const w = mount(LogView, {
      props: { ...PROPS, fields: ['time', 'source', 'message'] },
      global: { plugins: [naive] },
    })
    await new Promise((r) => setTimeout(r, 0))
    const rows = w.findAll('.row:not(.header)')
    expect(rows).toHaveLength(1)
    // message 不丢 + source 列正常渲染原文
    expect(rows[0].text()).toContain('hello-msg')
    expect(rows[0].text()).toContain('local:a.log')
  })

  it('表头带 gutter 占位,与数据行列起点对齐(第三轮验收⑦)', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: [ln(1, 'a')] })
    const w = mount(LogView, { props: PROPS, global: { plugins: [naive] } })
    await new Promise((r) => setTimeout(r, 0))
    const header = w.find('.row.header')
    expect(header.exists()).toBe(true)
    // 表头第一个子元素必须是 gutter(与数据行同构,否则列起点错开 gutter 宽)
    expect(header.element.firstElementChild?.classList.contains('gutter')).toBe(true)
    expect(logViewSrc).toMatch(/visualActive/)
    expect(logViewSrc).toMatch(/hitSeqs\.has\(l\.seq\)/)
  })

  it('列宽拖拽:表头把手拖动调宽,mouseup 一次性 emit 落盘(第七轮验收②)', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: [ln(1, 'a')] })
    const w = mount(LogView, { props: PROPS, global: { plugins: [naive] } })
    await new Promise((r) => setTimeout(r, 0))

    // 固定列(非 message)各有把手:PROPS.fields = time/level/thread/traceId/logger/message → 5 把
    const handles = w.findAll('.row.header .col-handle')
    expect(handles.length).toBe(5)

    // 拖 time 列 +77px(charPx jsdom 下默认 7 → +11ch):12 → 23
    await handles[0].trigger('mousedown', { clientX: 100, button: 0 })
    document.dispatchEvent(new MouseEvent('mousemove', { clientX: 177, bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))
    // 拖拽中表头已按新宽渲染(fitField 填充)
    const headTime = w.findAll('.row.header .cell')[0]
    expect(headTime.text().endsWith('时间') || headTime.text().trim().length).toBeTruthy()
    document.dispatchEvent(new MouseEvent('mouseup', { clientX: 177, bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))

    const emitted = w.emitted('update:widths')
    expect(emitted).toBeTruthy()
    const newW = emitted![0][0] as Record<string, number>
    expect(newW.time).toBe(23)
    expect(newW.level).toBe(5)
    expect(newW.logger).toBe(20)
  })

  it('n/N 高亮兜底:无搜索命中集时跳高亮词所在行(第五轮验收)', async () => {
    const tabs = useTabsStore()
    const ann = useAnnotationsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({
      tabId: 't1',
      lines: [ln(1, 'plain-one'), ln(2, 'has alpha inside'), ln(3, 'plain-two')],
    })
    ann.addHighlight('alpha')

    const w = mount(LogView, { props: PROPS, global: { plugins: [naive] } })
    await new Promise((r) => setTimeout(r, 0))

    w.vm.cursorToHit(1) // n:从可视首行(seq 1)向下找,应跳到 seq 2
    await new Promise((r) => setTimeout(r, 0))
    const cur = w.find('.row.cur')
    expect(cur.exists()).toBe(true)
    expect(cur.text()).toContain('has alpha inside')

    w.vm.cursorToHit(-1) // N:seq 2 之上无命中,光标不动(不回绕,对齐 TUI 单向找)
    await new Promise((r) => setTimeout(r, 0))
    expect(w.find('.row.cur').text()).toContain('has alpha inside')
  })
})

// 复制三件套(第九轮验收④):y 当前行/鼠标拖选行范围/⌘C 通道
describe('复制与拖选', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(ClipboardSetText).mockClear()
  })

  function mountWith3(): ReturnType<typeof mount> {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: [ln(1, 'm1'), ln(2, 'm2'), ln(3, 'm3')] })
    return mount(LogView, { props: PROPS, global: { plugins: [naive] } })
  }

  it('yy 复制当前光标行 raw;单按 y 无选区不再复制(vim-copy 语义)', async () => {
    const w = mountWith3()
    await new Promise((r) => setTimeout(r, 0))
    const vm = w.vm as unknown as {
      gotoTop: () => void
      yank: () => { n: number }
      yankLine: () => { n: number }
    }
    vm.gotoTop() // 光标定位首行 seq1(cursorSeq 同步 ref,无需等帧)
    expect(vm.yank().n).toBe(0) // 单按 y 无选区 no-op
    expect(ClipboardSetText).not.toHaveBeenCalled()
    expect(vm.yankLine().n).toBe(1) // yy → 复制整行 raw
    expect(ClipboardSetText).toHaveBeenCalledWith('m1')
  })

  it('鼠标拖选字符级:松手即复制选区收起并 emit 反馈(vim-copy)', async () => {
    const w = mountWith3()
    await new Promise((r) => setTimeout(r, 0))
    const rows = w.findAll('.row:not(.header)')
    await rows[0].trigger('mousedown', { button: 0 }) // 锚 (1,0)(测试环境 rect=0 → 列 0)
    await rows[2].trigger('mousemove', { buttons: 1 }) // 端点 (3,0)
    await new Promise((r) => setTimeout(r, 0))
    // 选区高亮 .char-sel(字符级,行级 .vis 不再出现)
    expect(w.findAll('.char-sel').length).toBeGreaterThan(0)
    await rows[2].trigger('mouseup')
    await new Promise((r) => setTimeout(r, 0))
    // 松手即复制:首行 [0..]、中间全量、尾行 [..0];选区收起
    expect(ClipboardSetText).toHaveBeenCalledWith('m1\nm2\nm')
    expect(w.findAll('.char-sel')).toHaveLength(0)
    // App 反馈通道:emit('copied', {n, unit})——'m1\nm2\nm' 含换行 7 rune
    expect(w.emitted('copied')?.at(-1)).toEqual([{ n: 7, unit: '字符' }])
  })

  it('鼠标单击定位行+列光标,不复制;双击详情不受影响', async () => {
    const w = mountWith3()
    await new Promise((r) => setTimeout(r, 0))
    const rows = w.findAll('.row:not(.header)')
    await rows[1].trigger('mousedown', { button: 0 })
    await rows[1].trigger('mouseup')
    await new Promise((r) => setTimeout(r, 0))
    expect(ClipboardSetText).not.toHaveBeenCalled()
    expect(w.findAll('.char-cursor')).toHaveLength(1) // 列光标落在行2 col0
    expect(w.get('.char-cursor').text()).toBe('m')
  })

  it('hasSelection 无选区 false/有选区 true(⌘C 拦截判定)', async () => {
    const w = mountWith3()
    await new Promise((r) => setTimeout(r, 0))
    const vm = w.vm as unknown as { hasSelection: () => boolean; startVisual: (l: string) => void; moveCursor: (d: number) => void; gotoTop: () => void }
    expect(vm.hasSelection()).toBe(false)
    vm.gotoTop()
    vm.startVisual('line')
    vm.moveCursor(2)
    await new Promise((r) => setTimeout(r, 0))
    expect(vm.hasSelection()).toBe(true)
  })
})


// 终审修复回归:Important1 拖选窗外松手防线 / Important2 :root 变量环
describe('终审修复回归', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(ClipboardSetText).mockClear()
  })

  it('拖选在窗外松手(buttons=0 移动)即复制收起,终点=最后 move 位(Final Review I4)', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: [ln(1, 'm1'), ln(2, 'm2'), ln(3, 'm3')] })
    const w = mount(LogView, { props: PROPS, global: { plugins: [naive] } })
    await new Promise((r) => setTimeout(r, 0))
    const rows = w.findAll('.row:not(.header)')
    await rows[0].trigger('mousedown', { button: 0 })
    await rows[2].trigger('mousemove', { buttons: 1 }) // 按住拖到第 3 行
    await rows[1].trigger('mousemove', { buttons: 0 }) // 窗外松手:复制收起,终点仍=行3
    await new Promise((r) => setTimeout(r, 0))
    expect(ClipboardSetText).toHaveBeenCalledWith('m1\nm2\nm') // 锚..最后 move 位(行3),非回窗落点
    expect(w.findAll('.char-sel')).toHaveLength(0)
    vi.mocked(ClipboardSetText).mockClear()
    await rows[0].trigger('mousemove', { buttons: 0 }) // 拖拽态已清,不复活
    expect(ClipboardSetText).not.toHaveBeenCalled()
    expect(w.findAll('.char-sel')).toHaveLength(0)
  })

  it(':root --ui-font-size 为字面量 12px(与派生档不得循环引用)', () => {
    // css?raw 在 vitest 走 css 管线拿不到源串,node:fs 直读正本
    const styleCss = readFileSync(resolve(process.cwd(), 'src/style.css'), 'utf8')
    expect(styleCss).toMatch(/--ui-font-size:\s*12px/)
    expect(styleCss).not.toMatch(/--ui-font-size:\s*var\(/)
  })

  // ---- 字符光标(vim-copy Task3):按需出现/移动/列记忆/wrap 投影 ----
  async function setupCursor(lines: LineDTO[], wrap = false) {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines })
    const ui = useUiStore()
    ui.wrap = wrap
    const w = mount(LogView, { props: PROPS, global: { plugins: [naive] } })
    await new Promise((r) => setTimeout(r, 0))
    return w
  }

  it('字符光标:未按移动键不渲染;h/l 按需出现并包住字符', async () => {
    const w = await setupCursor([ln(1, 'hello world')])
    const vm = w.vm as unknown as { moveCursor: (d: number) => void; moveCursorCol: (m: string) => void }
    vm.moveCursor(1) // 行光标,列未激活
    await new Promise((r) => setTimeout(r, 0))
    expect(w.findAll('.char-cursor')).toHaveLength(0)

    vm.moveCursorCol('left') // col0(激活)
    await new Promise((r) => setTimeout(r, 0))
    expect(w.get('.char-cursor').text()).toBe('h')

    vm.moveCursorCol('right')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.get('.char-cursor').text()).toBe('e')
  })

  it('字符光标:0/$/w/b/e 光标字符与 applyMotion 一致', async () => {
    const w = await setupCursor([ln(1, 'hello world')])
    const vm = w.vm as unknown as { moveCursorCol: (m: string) => void }
    const cur = () => w.get('.char-cursor').text()
    vm.moveCursorCol('end')
    await new Promise((r) => setTimeout(r, 0))
    expect(cur()).toBe('d') // $ → 尾
    vm.moveCursorCol('start')
    await new Promise((r) => setTimeout(r, 0))
    expect(cur()).toBe('h') // 0 → 首
    vm.moveCursorCol('wordFwd')
    await new Promise((r) => setTimeout(r, 0))
    expect(cur()).toBe('w') // w → 词首
    vm.moveCursorCol('wordBack')
    await new Promise((r) => setTimeout(r, 0))
    expect(cur()).toBe('h') // b → 词首
    vm.moveCursorCol('wordEnd')
    await new Promise((r) => setTimeout(r, 0))
    expect(cur()).toBe('o') // e → hello 尾
  })

  it('字符光标:j/k 列记忆——长行 $ 后 k 到短行 clamp,再 j 恢复', async () => {
    const w = await setupCursor([ln(1, 'ab'), ln(2, 'long line here')])
    const vm = w.vm as unknown as { moveCursor: (d: number) => void; moveCursorCol: (m: string) => void }
    vm.moveCursor(1) // 光标行1 'ab'
    vm.moveCursor(1) // 行2
    vm.moveCursorCol('end') // col13 'e'
    await new Promise((r) => setTimeout(r, 0))
    expect(w.get('.char-cursor').text()).toBe('e')
    vm.moveCursor(-1) // k 到 'ab' clamp col1
    await new Promise((r) => setTimeout(r, 0))
    expect(w.get('.char-cursor').text()).toBe('b')
    vm.moveCursor(1) // j 回长行恢复 col13
    await new Promise((r) => setTimeout(r, 0))
    expect(w.get('.char-cursor').text()).toBe('e')
  })

  it('字符光标:wrap 模式光标投影到正确段', async () => {
    // mount 环境 msgWidthCh=max(10,..)=10,'hello world'(11ch) → 2 段 ['hello worl','d']
    const w = await setupCursor([ln(1, 'hello world')], true)
    const vm = w.vm as unknown as { moveCursorCol: (m: string) => void }
    vm.moveCursorCol('end')
    await new Promise((r) => setTimeout(r, 0))
    const wlines = w.findAll('.wline')
    expect(wlines.length).toBe(2)
    // 光标 'd' 应在第 2 段内
    const cur = w.get('.char-cursor')
    expect(cur.text()).toBe('d')
    expect(cur.element.closest('.wline')).toBe(wlines[1].element)
  })

  it('字符光标:非 wrap 模式光标行超宽临时 wrap(所见即光标,Final Review C1)', async () => {
    // mount 环境 msgWidthCh=10,'hello world'(11ch) 超宽——列光标激活后该行
    // 应临时按段渲染(.wline≥2),块光标可见;未激活时保持单行截断
    const w = await setupCursor([ln(1, 'hello world')])
    expect(w.findAll('.wline')).toHaveLength(0) // 未激活不 wrap
    const vm = w.vm as unknown as { moveCursorCol: (m: string) => void }
    vm.moveCursorCol('end')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.findAll('.wline').length).toBeGreaterThanOrEqual(2)
    expect(w.get('.char-cursor').text()).toBe('d')
  })

  it('字符光标:样式选择器配对(防类名拼错)', () => {
    expect(logViewSrc).toMatch(/\.char-cursor\s*\{/)
  })
})

// ---- 字符可视选择(vim-copy Task4):v 字符级锚列、y 复制子串并收起、⌘C 通道 ----
describe('字符可视选择', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(ClipboardSetText).mockClear()
  })

  async function setupSel(lines: LineDTO[]) {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines })
    const w = mount(LogView, { props: PROPS, global: { plugins: [naive] } })
    await new Promise((r) => setTimeout(r, 0))
    return w
  }

  it('v 后移动扩选:.char-sel 覆盖锚..光标列;y 复制子串并收起', async () => {
    const w = await setupSel([ln(1, 'hello world')])
    const vm = w.vm as unknown as {
      moveCursor: (d: number) => void
      moveCursorCol: (m: string) => void
      startVisual: (mode: string) => void
      yank: () => { n: number }
    }
    vm.moveCursor(1) // 行光标 seq1
    vm.moveCursorCol('start') // col0
    vm.startVisual('char') // 锚 (1,0)
    vm.moveCursorCol('right')
    vm.moveCursorCol('right')
    vm.moveCursorCol('right') // 光标列 3
    await new Promise((r) => setTimeout(r, 0))
    // 选区底纹 + 光标块(端点字符被光标反色块压住)合并覆盖锚..光标列
    const covered = [...w.findAll('.char-sel'), ...w.findAll('.char-cursor')]
      .map((s) => s.text())
      .join('')
    expect(covered).toBe('hell')

    expect(vm.yank().n).toBe(4) // 字符数
    expect(ClipboardSetText).toHaveBeenCalledWith('hell')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.findAll('.char-sel')).toHaveLength(0) // 复制后选区收起
  })

  it('v 跨行:首行尾段+中间全量+尾行首段', async () => {
    const w = await setupSel([ln(1, 'hello world'), ln(2, 'second line'), ln(3, 'tail')])
    const vm = w.vm as unknown as {
      moveCursor: (d: number) => void
      moveCursorCol: (m: string) => void
      startVisual: (mode: string) => void
      yank: () => { n: number }
    }
    vm.moveCursor(1)
    vm.moveCursorCol('wordFwd') // col6
    vm.startVisual('char') // 锚 (1,6)
    vm.moveCursor(1) // 行2(列记忆 col6)
    vm.moveCursor(1) // 行3(列记忆 clamp col3)
    vm.moveCursorCol('start') // 归零再选(列记忆是设计行为,测试显式起步)
    vm.moveCursorCol('right')
    vm.moveCursorCol('right') // (3,2)
    expect(vm.yank().n).toBe(21)
    expect(ClipboardSetText).toHaveBeenCalledWith('world\nsecond line\ntai')
  })

  it('V 行级现状回归:V+j+y 复制 raw 行区间', async () => {
    const w = await setupSel([ln(1, 'm1'), ln(2, 'm2')])
    const vm = w.vm as unknown as {
      moveCursor: (d: number) => void
      startVisual: (mode: string) => void
      yank: () => { n: number }
    }
    vm.moveCursor(1)
    vm.startVisual('line')
    vm.moveCursor(1)
    expect(vm.yank().n).toBe(2)
    expect(ClipboardSetText).toHaveBeenCalledWith('m1\nm2')
  })

  it('v 后 Esc 退出选区;⌘C 通道 yankSelection 优先字符选区', async () => {
    const w = await setupSel([ln(1, 'hello')])
    const vm = w.vm as unknown as {
      moveCursor: (d: number) => void
      moveCursorCol: (m: string) => void
      startVisual: (mode: string) => void
      exitVisual: () => void
      yankSelection: () => { n: number }
      hasSelection: () => boolean
    }
    vm.moveCursor(1)
    vm.moveCursorCol('start')
    vm.startVisual('char')
    vm.moveCursorCol('end') // col4
    expect(vm.hasSelection()).toBe(true)
    expect(vm.yankSelection().n).toBe(5) // ⌘C 通道:字符选区走 char 子串
    expect(ClipboardSetText).toHaveBeenCalledWith('hello')
    await new Promise((r) => setTimeout(r, 0))

    // 重建选区再 Esc 退出
    vm.startVisual('char')
    vm.exitVisual()
    await new Promise((r) => setTimeout(r, 0))
    expect(w.findAll('.char-sel')).toHaveLength(0)
    expect(vm.hasSelection()).toBe(false)
  })
})
