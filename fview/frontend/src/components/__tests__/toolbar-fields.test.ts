import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { NDropdown } from 'naive-ui'
import Toolbar from '../logview/Toolbar.vue'
import { useTabsStore } from '../../stores/tabs'
import { useFieldsStore } from '../../stores/fields'
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
    return mount(Toolbar, { props: { configured } })
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
})
