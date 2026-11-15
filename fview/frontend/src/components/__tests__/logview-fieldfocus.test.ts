// 字段域光标测试(field-interact):h 在 message col0 跳最右字段栏、h/l 栏间
// 移动两端停、l 回 message col0、j/k 换行跟随、message motion 跳回、
// N=0 不跳、v 进 visual 清字段态、wrap 下 fcur 渲染。
// mock 头复刻 logview-mount.test.ts(mount 冒烟同源);vm 驱动后 await tick
// 再断言(渲染链异步,与现有字符光标用例同节奏)。
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import naive from 'naive-ui'

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
import type { LineDTO } from '../../lib/types'

function ln(seq: number, message: string, logger = 'a.B.C'): LineDTO {
  return { seq, time: '12:00:00.001', level: 'INFO', thread: 'main', traceId: '', logger, message, raw: message }
}

const PROPS = {
  hitSeqs: new Set<number>(),
  hitWords: [] as string[],
  levelMode: 'A' as const,
  fields: ['time', 'level', 'logger', 'message'],
}

const tick = () => new Promise((r) => setTimeout(r, 0))

/** 字段域从右数:logger=0(最右,紧邻 message)/level=1/time=2。 */
async function mountView(lines: LineDTO[], fields = PROPS.fields, wrap = false) {
  const tabs = useTabsStore()
  tabs.applyOpened({ tabId: 't1', label: 'a.log' })
  tabs.applyLines({ tabId: 't1', lines })
  useUiStore().wrap = wrap
  const w = mount(LogView, { props: { ...PROPS, fields }, global: { plugins: [naive] } })
  await tick()
  return w
}

type Vm = {
  moveCursor: (d: number) => void
  moveCursorCol: (m: string) => void
  startVisual: (l: string) => void
  yankLine: () => { n: number; unit: '行' | '字符' }
  cursorWord: () => string
}

describe('LogView 字段域光标(field-interact)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('f01: col0 再 h 跳最右字段栏(logger),h/l 栏间移动两端停,l 回 message col0', async () => {
    const w = await mountView([ln(1, 'abc')])
    const vm = w.vm as unknown as Vm
    vm.moveCursor(1)
    await tick()
    vm.moveCursorCol('left') // 未激活 → col0 激活
    await tick()
    expect(w.get('.char-cursor').text()).toBe('a')

    vm.moveCursorCol('left') // col0 再 h → 跳字段域最右栏 logger
    await tick()
    expect(w.find('.row.cur .cell.f-logger.fcur').exists()).toBe(true)
    expect(w.findAll('.char-cursor')).toHaveLength(0) // 块光标熄灭

    vm.moveCursorCol('left') // → level
    await tick()
    expect(w.find('.row.cur .cell.f-level.fcur').exists()).toBe(true)
    vm.moveCursorCol('left') // → time(最左)
    await tick()
    expect(w.find('.row.cur .cell.f-time.fcur').exists()).toBe(true)
    vm.moveCursorCol('left') // 最左停住
    await tick()
    expect(w.find('.row.cur .cell.f-time.fcur').exists()).toBe(true)

    vm.moveCursorCol('right') // time → level
    await tick()
    expect(w.find('.row.cur .cell.f-level.fcur').exists()).toBe(true)
    vm.moveCursorCol('right') // level → logger
    await tick()
    expect(w.find('.row.cur .cell.f-logger.fcur').exists()).toBe(true)
    vm.moveCursorCol('right') // logger → message col0
    await tick()
    expect(w.findAll('.fcur')).toHaveLength(0)
    expect(w.get('.char-cursor').text()).toBe('a')
  })

  it('f02: 未激活态首次 h 只激活 col0 不跳域(光标在最左侧"再按"才跳)', async () => {
    const w = await mountView([ln(1, 'abc')])
    const vm = w.vm as unknown as Vm
    vm.moveCursor(1)
    await tick()
    vm.moveCursorCol('left')
    await tick()
    // 仍在 message col0(未跳字段域)
    expect(w.get('.char-cursor').text()).toBe('a')
    expect(w.findAll('.fcur')).toHaveLength(0)
  })

  it('f03: j/k 换行字段光标跟随(栏位不变,行变)', async () => {
    const w = await mountView([ln(1, 'abc'), ln(2, 'xyz')])
    const vm = w.vm as unknown as Vm
    vm.moveCursor(1)
    await tick()
    vm.moveCursorCol('left')
    await tick()
    vm.moveCursorCol('left') // logger
    await tick()
    vm.moveCursorCol('left') // level(fieldFocus=1)
    await tick()
    vm.moveCursor(1) // j → 行2
    await tick()
    expect(w.find('.row.cur .cell.f-level.fcur').exists()).toBe(true)
    expect(w.get('.row.cur').text()).toContain('xyz')
  })

  it('f04: 0/^/$/w/b/e 从字段域跳回 message 对应位置', async () => {
    const w = await mountView([ln(1, 'abc def')])
    const vm = w.vm as unknown as Vm
    vm.moveCursor(1)
    await tick()
    vm.moveCursorCol('left')
    await tick()
    vm.moveCursorCol('left') // 进字段域 logger
    await tick()
    vm.moveCursorCol('end') // $ 跳回 message 行尾
    await tick()
    expect(w.findAll('.fcur')).toHaveLength(0)
    expect(w.get('.char-cursor').text()).toBe('f')

    vm.moveCursorCol('start') // 0 回行首
    await tick()
    vm.moveCursorCol('left') // col0 再 h → 字段域
    await tick()
    expect(w.find('.row.cur .cell.f-logger.fcur').exists()).toBe(true)
    vm.moveCursorCol('wordFwd') // w 跳回 message 下一词首
    await tick()
    expect(w.findAll('.fcur')).toHaveLength(0)
    expect(w.get('.char-cursor').text()).toBe('d')
  })

  it('f05: N=0(字段全隐只剩 message)col0 再 h 不跳不炸', async () => {
    const w = await mountView([ln(1, 'abc')], ['message'])
    const vm = w.vm as unknown as Vm
    vm.moveCursor(1)
    await tick()
    vm.moveCursorCol('left') // 激活 col0
    await tick()
    vm.moveCursorCol('left') // 无字段栏 → 停 col0
    await tick()
    expect(w.findAll('.fcur')).toHaveLength(0)
    expect(w.get('.char-cursor').text()).toBe('a')
  })

  it('f06: 字段域按 v 进 visual 清字段态(双态不并存)', async () => {
    const w = await mountView([ln(1, 'abc')])
    const vm = w.vm as unknown as Vm
    vm.moveCursor(1)
    await tick()
    vm.moveCursorCol('left')
    await tick()
    vm.moveCursorCol('left') // 字段域 logger
    await tick()
    expect(w.find('.row.cur .cell.f-logger.fcur').exists()).toBe(true)
    vm.startVisual('char')
    await tick()
    expect(w.findAll('.fcur')).toHaveLength(0)
    expect(w.get('.char-cursor').text()).toBe('a') // 锚回落 message col0
  })

  it('f08: 字段域鼠标按下(单击/拖选起点)清字段态(Review Focus 3 拖选半边)', async () => {
    const w = await mountView([ln(1, 'abc')])
    const vm = w.vm as unknown as Vm
    vm.moveCursor(1)
    await tick()
    vm.moveCursorCol('left')
    await tick()
    vm.moveCursorCol('left') // 字段域 logger
    await tick()
    expect(w.find('.row.cur .cell.f-logger.fcur').exists()).toBe(true)
    // 鼠标在 message 区按下(拖选/单击起点)必须退出字段域,防双光标并存
    await w.find('.rows .row .cell.f-msg').trigger('mousedown', { button: 0 })
    await tick()
    expect(w.findAll('.fcur')).toHaveLength(0)
  })

  it('f07: wrap 开启字段 cell 仍 fcur(字段列不 wrap)', async () => {
    const w = await mountView([ln(1, 'abc')], PROPS.fields, true)
    const vm = w.vm as unknown as Vm
    vm.moveCursor(1)
    await tick()
    vm.moveCursorCol('left')
    await tick()
    vm.moveCursorCol('left') // 字段域 logger
    await tick()
    expect(w.find('.row.cur .cell.f-logger.fcur').exists()).toBe(true)
  })

  // ---- 字段复制(field-interact Task6):yy 栏位原始值/双击字段复制/空值不进剪贴板 ----

  /** enterField 进指定字段栏(从右数 idx:0=logger,1=level,2=time)。 */
  async function enterField(w: Awaited<ReturnType<typeof mountView>>, idx: number) {
    const vm = w.vm as unknown as Vm
    vm.moveCursor(1)
    await tick()
    vm.moveCursorCol('left')
    await tick()
    for (let i = 0; i <= idx; i++) {
      vm.moveCursorCol('left')
      await tick()
    }
  }

  it('y01: 字段域 yy 复制原始值(屏显缩写不丢信息)', async () => {
    vi.mocked(ClipboardSetText).mockClear()
    const w = await mountView([ln(1, 'abc', 'com.example.a.LogService')])
    await enterField(w, 0) // logger 栏
    const r = (w.vm as unknown as Vm).yankLine()
    expect(vi.mocked(ClipboardSetText)).toHaveBeenCalledWith('com.example.a.LogService')
    expect(r).toEqual({ n: 24, unit: '字符' }) // com.example.a.LogService = 24 rune
  })

  it('y02: 空值字段栏 yy 不进剪贴板', async () => {
    vi.mocked(ClipboardSetText).mockClear()
    const empty = ln(1, 'abc')
    empty.time = ''
    const w = await mountView([empty])
    await enterField(w, 2) // time 栏
    const r = (w.vm as unknown as Vm).yankLine()
    expect(r).toEqual({ n: 0, unit: '字符', empty: true }) // 空值带标记,flash 闪提示
    expect(vi.mocked(ClipboardSetText)).not.toHaveBeenCalled()
  })

  it('y03: message 域 yy 仍复制整行 raw(不回归)', async () => {
    vi.mocked(ClipboardSetText).mockClear()
    const w = await mountView([ln(1, 'abc')])
    const vm = w.vm as unknown as Vm
    vm.moveCursor(1)
    await tick()
    const r = vm.yankLine()
    expect(vi.mocked(ClipboardSetText)).toHaveBeenCalledWith('abc') // raw=message
    expect(r).toEqual({ n: 1, unit: '行' })
  })

  it('y04: 双击字段 cell 复制原始值,不误触详情', async () => {
    vi.mocked(ClipboardSetText).mockClear()
    const w = await mountView([ln(1, 'abc', 'com.example.a.LogService')])
    await tick()
    await w.find('.rows .row .cell.f-logger').trigger('dblclick')
    await tick()
    expect(vi.mocked(ClipboardSetText)).toHaveBeenCalledWith('com.example.a.LogService')
    expect(w.emitted('copied')![0][0]).toEqual({ n: 24, unit: '字符' })
    // .stop 拦截 row 的 openDetail:详情浮层不弹
    expect(document.body.querySelector('.detail')).toBeNull()
  })

  it('y06: 双击空值字段 emit 空值提示(不静默),不进剪贴板', async () => {
    vi.mocked(ClipboardSetText).mockClear()
    const empty = ln(1, 'abc')
    empty.time = ''
    const w = await mountView([empty])
    await tick()
    await w.find('.rows .row .cell.f-time').trigger('dblclick')
    await tick()
    expect(vi.mocked(ClipboardSetText)).not.toHaveBeenCalled()
    expect(w.emitted('copied')![0][0]).toEqual({ n: 0, unit: '字符', empty: true })
  })

  it('y07: 双击 source chip 复制来源名,不误触详情(k8s 合流 source 在 gutter 不在字段列)', async () => {
    vi.mocked(ClipboardSetText).mockClear()
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyLines({ tabId: 't1', lines: [{ ...ln(1, 'abc'), source: 'pod-a-7f9' }] })
    const w = mount(LogView, { props: { ...PROPS, sourceCol: true }, global: { plugins: [naive] } })
    await tick()
    await w.find('.rows .row .src-chip').trigger('dblclick')
    await tick()
    expect(vi.mocked(ClipboardSetText)).toHaveBeenCalledWith('pod-a-7f9')
    expect(w.emitted('copied')![0][0]).toEqual({ n: 9, unit: '字符' }) // pod-a-7f9 = 9 rune
    expect(document.body.querySelector('.detail')).toBeNull() // .stop 不误触详情
  })

  it('y05: 双击 message 区仍打开行详情(不回归)', async () => {
    const w = await mountView([ln(1, 'abc')])
    await tick()
    await w.find('.row .cell.f-msg').trigger('dblclick')
    await tick()
    expect(document.body.querySelector('.detail')).toBeTruthy()
  })

  // ---- cursorWord 取词(field-interact Task7):visual 选区 > 字段域 > 光标词 ----

  it('cw01: visual char 选区取首行子串(跨行不拼接)', async () => {
    const w = await mountView([ln(1, 'abc def'), ln(2, 'xyz')])
    const vm = w.vm as unknown as Vm
    vm.moveCursor(1)
    await tick()
    vm.moveCursorCol('right') // col1
    await tick()
    vm.moveCursorCol('right') // col2(锚 c)
    await tick()
    vm.startVisual('char')
    await tick()
    vm.moveCursor(1) // 选区跨到行2
    await tick()
    expect(vm.cursorWord()).toBe('c def') // 首行 [2..] 子串
  })

  it('cw02: visual line 选区取锚行 message', async () => {
    const w = await mountView([ln(1, 'abc def'), ln(2, 'xyz')])
    const vm = w.vm as unknown as Vm
    vm.moveCursor(1)
    await tick()
    vm.startVisual('line')
    await tick()
    vm.moveCursor(1)
    await tick()
    expect(vm.cursorWord()).toBe('abc def') // 锚行(行1)全文
  })

  it('cw03: 字段域取整字段原始值', async () => {
    const w = await mountView([ln(1, 'abc', 'com.example.a.LogService')])
    await enterField(w, 0) // logger 栏
    expect((w.vm as unknown as Vm).cursorWord()).toBe('com.example.a.LogService')
  })

  it('cw04: message 光标词按 wordAt 口径', async () => {
    const w = await mountView([ln(1, 'timeout=3000')])
    const vm = w.vm as unknown as Vm
    vm.moveCursor(1)
    await tick()
    vm.moveCursorCol('right') // col0
    await tick()
    vm.moveCursorCol('right') // col1
    await tick()
    vm.moveCursorCol('right') // col2
    await tick()
    vm.moveCursorCol('right') // col3(词中)
    await tick()
    expect(vm.cursorWord()).toBe('timeout')
  })

  it('cw05: 无选区未激活/空白上取词返回空串', async () => {
    const w = await mountView([ln(1, 'a b c')])
    const vm = w.vm as unknown as Vm
    expect(vm.cursorWord()).toBe('') // 行光标未激活列
    vm.moveCursor(1)
    await tick()
    vm.moveCursorCol('left') // 未激活 left → col0(a)
    await tick()
    vm.moveCursorCol('right') // col1(空格;未激活 right 从 0 起步会跳到 1,先 left 锚定)
    await tick()
    expect(vm.cursorWord()).toBe('') // 空白无词
  })
})
