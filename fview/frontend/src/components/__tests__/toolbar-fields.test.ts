import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { NDropdown } from 'naive-ui'
import Toolbar from '../logview/Toolbar.vue'
import { useTabsStore } from '../../stores/tabs'
import { useFieldsStore } from '../../stores/fields'
import { useUiStore } from '../../stores/ui'
import { saveFields } from '../../lib/backend'

vi.mock('../../lib/backend', () => ({
  setFollow: vi.fn(),
  saveFields: vi.fn(),
}))

interface Opt {
  key?: string
  label: string
  disabled?: boolean
  type?: string
}

// F 勾选只写当前日志覆盖(emit 给 App 落 localStorage),不再碰 rules.yaml(2026-11-03 spec 需求②)
describe('Toolbar 字段下拉(覆盖层)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    vi.mocked(saveFields).mockClear()
  })

  function mountToolbar(configured: string[]) {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'app.log', info: '/tmp/app.log' })
    const fieldsStore = useFieldsStore()
    fieldsStore.all = [
      { name: 'time', visible: true, width: 12 },
      { name: 'level', visible: true, width: 5 },
      { name: 'thread', visible: true, width: 10 },
      { name: 'traceId', visible: true, width: 12 },
      { name: 'logger', visible: true, width: 20 },
      { name: 'message', visible: true, width: 0 },
    ]
    // attachTo body:默认 detached 容器冒泡止步于容器根,到不了 window(evtd 层)
    return mount(Toolbar, { props: { configured }, attachTo: document.body })
  }

  function opts(w: ReturnType<typeof mountToolbar>): Opt[] {
    return w.findComponent(NDropdown).props('options') as Opt[]
  }

  it('勾选 time → emit override-fields 为去掉 time 后按配置序的数组;不写 rules.yaml', () => {
    const w = mountToolbar(['time', 'level', 'message'])
    w.findComponent(NDropdown).vm.$emit('select', 'time')
    expect(w.emitted('override-fields')![0]).toEqual([['level', 'message']])
    expect(saveFields).not.toHaveBeenCalled()
  })

  it('未勾字段再勾 → 加回列表并按配置序重排', () => {
    const w = mountToolbar(['level', 'message'])
    w.findComponent(NDropdown).vm.$emit('select', 'time')
    expect(w.emitted('override-fields')![0]).toEqual([['time', 'level', 'message']])
  })

  it('message 恒显不可取消', () => {
    const w = mountToolbar(['time', 'level', 'message'])
    w.findComponent(NDropdown).vm.$emit('select', 'message')
    expect(w.emitted('override-fields')).toBeUndefined()
  })

  it('解析器不产出的字段置灰标「未解析」;message 勾死;可产出字段不置灰', async () => {
    const tabs = useTabsStore()
    const w = mountToolbar(['time', 'message'])
    tabs.applyMeta({ tabId: 't1', label: 'app.log', parserName: 'p', parserFields: ['time', 'message'] })
    await nextTick()
    const o = opts(w)
    const byKey = Object.fromEntries(o.map((x) => [x.key ?? x.type, x]))
    expect(byKey['time'].disabled).toBeFalsy()
    expect(byKey['level'].disabled).toBe(true)
    expect(byKey['level'].label).toContain('未解析')
    expect(byKey['thread'].disabled).toBe(true)
    expect(byKey['traceId'].disabled).toBe(true)
    expect(byKey['logger'].disabled).toBe(true)
    expect(byKey['message'].disabled).toBe(true)
    expect(byKey['message'].label).not.toContain('未解析')
  })

  it('下拉尾有分隔线与「恢复默认」项,选中 emit null', () => {
    const w = mountToolbar(['time', 'message'])
    const o = opts(w)
    expect(o[o.length - 2]).toEqual({ type: 'divider' })
    expect(o[o.length - 1]).toMatchObject({ key: '__reset__', label: '恢复默认' })
    w.findComponent(NDropdown).vm.$emit('select', '__reset__')
    expect(w.emitted('override-fields')![0]).toEqual([null])
  })

  // r4 验收:字段下拉键盘流——⌃j/⌃k 中继方向键 + 弹窗栈让位 + 关闭恢复
  // r5 教训:evtd 委托监听挂 window(非 document),spy 必须挂 window 才是 naive 真实
  // 消费层——首版挂 document 全绿但真机 evtd 收不到(bubbles 缺失),断言错层即假绿
  it('打开登记弹窗栈(全局 vim 键让位);⌃j/⌃k 中继为 ArrowDown/ArrowUp 且掐断传播;关闭出栈停中继', async () => {
    const w = mountToolbar(['time', 'level', 'message'])
    const ui = useUiStore()
    // spy 挂 window:relayFieldNav(capture,document)先执行,合成方向键 bubbles 到
    // window 在此断言——与 naive/evtd 同层;原生 ctrl+j 被掐不应出现在此
    const got: string[] = []
    const spy = (e: KeyboardEvent) => got.push(e.key)
    window.addEventListener('keydown', spy)
    // 真机路径:按键 target 是焦点元素(按钮),非 document
    const btn = w.find('button').element
    const press = (k: string) =>
      btn.dispatchEvent(new KeyboardEvent('keydown', { key: k, ctrlKey: true, bubbles: true, cancelable: true }))
    try {
      // 未打开:⌃j 不中继、原样冒泡到达
      press('j')
      expect(got).toEqual(['j'])
      // 打开:让位登记 + 中继翻译(naive pending 高亮应出现——组件级端到端)
      w.vm.toggleFields()
      await nextTick()
      await nextTick()
      expect(ui.modalOpen).toBe(true)
      press('j')
      expect(got).toEqual(['j', 'ArrowDown'])
      // naive 真实消费层端到端:中继后 pending 高亮必须出现(r5 实证——
      // dispatch 到 document 本体时 evtd 匹配不到,必须走焦点元素原生路径)
      await nextTick()
      expect(document.querySelector('.n-dropdown-option-body--pending')).toBeTruthy()
      press('k')
      expect(got).toEqual(['j', 'ArrowDown', 'ArrowUp'])
      // 关闭:出栈恢复 + 中继停
      w.vm.toggleFields()
      await nextTick()
      expect(ui.modalOpen).toBe(false)
      press('j')
      expect(got).toEqual(['j', 'ArrowDown', 'ArrowUp', 'j'])
    } finally {
      window.removeEventListener('keydown', spy)
      w.unmount()
    }
  })

  // r6:Space 勾选(naive 键盘表无 Space,中继 Enter)+ Esc 关闭(manual 受控
  // 未接 update:show,自己关)+ 连续勾选不关(naive doUpdateShow 无人接,歪打正着)
  it('Space 中继为 Enter 勾选 pending 项且下拉不关;Esc 直接关闭并出栈', async () => {
    const w = mountToolbar(['time', 'level', 'message'])
    const ui = useUiStore()
    const btn = w.find('button').element
    const press = (k: string) =>
      btn.dispatchEvent(new KeyboardEvent('keydown', { key: k, ctrlKey: k === 'j' || k === 'k', bubbles: true, cancelable: true }))
    try {
      w.vm.toggleFields()
      await nextTick()
      await nextTick()
      // ⌃j 移 pending 到首项 time(已勾),Space 中继 Enter → toggle 掉 time
      press('j')
      press(' ')
      await nextTick()
      expect(w.emitted('override-fields')!.at(-1)).toEqual([['level', 'message']])
      // 连续勾选:勾完下拉不关、弹窗栈仍在
      expect(ui.modalOpen).toBe(true)
      // Esc:关闭 + 出栈
      press('Escape')
      await nextTick()
      expect(ui.modalOpen).toBe(false)
      expect(w.findComponent(NDropdown).props('show')).toBe(false)
    } finally {
      w.unmount()
    }
  })
})
