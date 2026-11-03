import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { NSelect } from 'naive-ui'
import SettingsAppearance from '../dialogs/SettingsAppearance.vue'
import { useFieldsStore } from '../../stores/fields'
import { saveFields } from '../../lib/backend'

const themeApply = vi.fn()

vi.mock('../../themes', () => ({
  useThemeStore: () => ({
    nameList: ['dark', 'dracula', 'nord'],
    name: 'dark',
    apply: themeApply,
    init: vi.fn(),
    naiveTheme: null,
    overrides: {},
  }),
}))

vi.mock('../../lib/backend', () => ({
  systemFonts: vi.fn().mockResolvedValue(['SF Mono']),
  getFields: vi.fn(),
  saveFields: vi.fn().mockResolvedValue(undefined),
}))

// 需求③主题下拉 + 需求②默认字段显示层入口(2026-11-03 spec)
describe('SettingsAppearance', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    themeApply.mockClear()
    vi.mocked(saveFields).mockClear()
  })

  function mountAppearance() {
    const fieldsStore = useFieldsStore()
    fieldsStore.all = [
      { name: 'time', visible: true, width: 12 },
      { name: 'level', visible: false, width: 5 },
      { name: 'message', visible: true, width: 0 },
    ]
    fieldsStore.loaded = true
    return mount(SettingsAppearance)
  }

  it('主题为 NSelect 下拉,options=主题列表,选择触发 theme.apply', async () => {
    const w = mountAppearance()
    const selects = w.findAllComponents(NSelect)
    const themeSel = selects[0]
    expect(themeSel.props('options')).toEqual([
      { label: 'dark', value: 'dark' },
      { label: 'dracula', value: 'dracula' },
      { label: 'nord', value: 'nord' },
    ])
    themeSel.vm.$emit('update:value', 'nord')
    expect(themeApply).toHaveBeenCalledWith('nord')
  })

  it('「默认字段显示」渲染全部字段勾选项,label 用中文表头', () => {
    const w = mountAppearance()
    const boxes = w.findAll('input[type=checkbox]')
    expect(boxes).toHaveLength(3)
    const text = w.text()
    expect(text).toContain('时间')
    expect(text).toContain('级别')
    expect(text).toContain('内容')
  })

  it('点击未勾字段触发 setVisibility(true) 并落盘;message 勾死不可关', async () => {
    const w = mountAppearance()
    const boxes = w.findAll('input[type=checkbox]')
    expect((boxes[1].element as HTMLInputElement).checked).toBe(false) // level 未勾
    await boxes[1].setValue(true)
    const s = useFieldsStore()
    expect(s.all[1].visible).toBe(true)
    expect(saveFields).toHaveBeenCalled()
    // message 恒显
    expect((boxes[2].element as HTMLInputElement).disabled).toBe(true)
  })
})
