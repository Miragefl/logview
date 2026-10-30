import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import naive from 'naive-ui'

// mock backend：App/Welcome 只依赖这几个封装（jsdom 下无真实 Wails 运行时）
vi.mock('../../lib/backend', () => ({
  subscribeTabEvents: vi.fn(),
  closeTab: vi.fn().mockResolvedValue(undefined),
  usageRecent: vi.fn().mockResolvedValue(['file:/tmp/app.log', 'file:/tmp/error.log']),
  currentTheme: vi.fn().mockResolvedValue(''),
  themeNames: vi.fn().mockResolvedValue([]),
  themeCSS: vi.fn().mockResolvedValue({}),
  setTheme: vi.fn().mockResolvedValue(undefined),
}))

import App from '../../App.vue'
import { useTabsStore } from '../../stores/tabs'

// mount 用当前 active pinia（beforeEach 已 setActivePinia），保证与测试内 store 同源
function mountApp() {
  return mount(App, { global: { plugins: [naive] } })
}

describe('应用壳', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('无 tab 时渲染欢迎页（含最近源文件名）', async () => {
    const w = mountApp()
    await new Promise((r) => setTimeout(r, 0)) // 等 usageRecent promise
    expect(w.text()).toContain('fview')
    expect(w.text()).toContain('app.log')
    expect(w.text()).toContain('error.log')
  })

  it('有 tab 时渲染 TabBar 且 label 正确、active 高亮', () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'app.log' })
    tabs.applyOpened({ tabId: 't2', label: 'web1:/var/log/x.log' })
    tabs.activeId = 't2'
    const w = mountApp()
    expect(w.text()).toContain('app.log')
    expect(w.text()).toContain('web1:/var/log/x.log')
    const active = w.find('.tab.active')
    expect(active.exists()).toBe(true)
    expect(active.text()).toContain('web1')
  })

  it('TabBar 点击切换 activeId，点 × 关闭 tab', async () => {
    const tabs = useTabsStore()
    tabs.applyOpened({ tabId: 't1', label: 'a.log' })
    tabs.applyOpened({ tabId: 't2', label: 'b.log' })
    tabs.activeId = 't1'
    const w = mountApp()
    // 切换
    await w.findAll('.tab')[1].trigger('click')
    expect(useTabsStore().activeId).toBe('t2')
    // 关闭 t2
    await w.findAll('.tab')[1].find('.close').trigger('click')
    expect(useTabsStore().tabs.map((t) => t.info.id)).toEqual(['t1'])
  })
})
