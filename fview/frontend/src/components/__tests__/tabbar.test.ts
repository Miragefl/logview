// TabBar 数字角标(vim-count r1):≤9 显示序号(⌘N 快捷切换可达提示,dim 色),
// 第 10+ 不显示(⌘ 只到 9,显示无快捷键意义的数字是误导)。
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('../../lib/backend', () => ({
  closeTab: vi.fn().mockResolvedValue(undefined),
}))

import TabBar from '../tabs/TabBar.vue'
import { useTabsStore } from '../../stores/tabs'

describe('TabBar 数字角标', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('前 9 个 tab 显示角标 1..9,第 10 个不显示', () => {
    const tabs = useTabsStore()
    for (let i = 1; i <= 10; i++) tabs.applyOpened({ tabId: `t${i}`, label: `app${i}.log` })
    const w = mount(TabBar)
    const tabEls = w.findAll('.tab')
    expect(tabEls).toHaveLength(10)
    for (let i = 0; i < 9; i++) {
      const idx = tabEls[i].find('.idx')
      expect(idx.exists(), `第 ${i + 1} 个 tab 应有角标`).toBe(true)
      expect(idx.text()).toBe(String(i + 1))
    }
    expect(tabEls[9].find('.idx').exists()).toBe(false)
  })

  it('角标在 label 前(dim 提示,不破坏 title/label 原渲染)', () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'nginx.log' })
    const w = mount(TabBar)
    const tab = w.find('.tab')
    expect(tab.text()).toContain('nginx.log')
    // DOM 序:idx 节点在 label 节点之前
    const children = tab.element.children
    const idxAt = Array.from(children).findIndex((c) => (c as HTMLElement).classList.contains('idx'))
    const labelAt = Array.from(children).findIndex((c) => (c as HTMLElement).classList.contains('label'))
    expect(idxAt).toBeGreaterThanOrEqual(0)
    expect(idxAt).toBeLessThan(labelAt)
  })
})
