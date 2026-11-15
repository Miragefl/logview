// StatusBar 复制反馈文案测试(field-interact):数量文案守护 + 空值提示分支。
import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('../../lib/backend', () => ({
  getMemStats: vi.fn().mockResolvedValue(0),
}))

import StatusBar from '../logview/StatusBar.vue'

describe('StatusBar 复制反馈', () => {
  it('数量文案:n>0 显示"已复制 N 单位"(守护)', () => {
    setActivePinia(createPinia())
    const w = mount(StatusBar, { props: { copied: 3, copiedUnit: '字符' } })
    expect(w.text()).toContain('已复制 3 字符')
    expect(w.text()).not.toContain('空值未复制')
  })

  it('空值提示(field-interact):empty 标记闪"空值未复制"', () => {
    setActivePinia(createPinia())
    const w = mount(StatusBar, { props: { copied: 0, copiedUnit: '字符', copiedEmpty: true } })
    expect(w.text()).toContain('空值未复制')
  })

  it('无标记无文案:no-op 静默(守护)', () => {
    setActivePinia(createPinia())
    const w = mount(StatusBar, { props: { copied: 0 } })
    expect(w.text()).not.toContain('已复制')
    expect(w.text()).not.toContain('空值未复制')
  })
})
