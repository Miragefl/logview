import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import naive from 'naive-ui'

vi.mock('../../lib/backend', () => ({
  getRules: vi.fn().mockResolvedValue([
    { name: 'java-logback', pattern: '(?P<message>.*)', fallback: false },
    { name: 'json', pattern: '\\{.*\\}', fallback: true },
  ]),
  saveRules: vi.fn().mockResolvedValue(undefined),
  getHides: vi.fn().mockResolvedValue([]),
  subscribeTabEvents: vi.fn(),
  currentTheme: vi.fn().mockResolvedValue('midnight'),
  themeNames: vi.fn().mockResolvedValue(['midnight', 'slate', 'solarized']),
  themeCSS: vi.fn().mockResolvedValue({ '--bg': '#10161f' }),
  setTheme: vi.fn().mockResolvedValue(undefined),
  getMemStats: vi.fn().mockResolvedValue(0),
  usageRecent: vi.fn().mockResolvedValue([]),
  closeTab: vi.fn().mockResolvedValue(undefined),
}))

import * as backend from '../../lib/backend'
import SettingsDialog from '../dialogs/SettingsDialog.vue'
import { useUiStore } from '../../stores/ui'

function mountDlg() {
  return mount(SettingsDialog, {
    props: { show: true },
    global: { plugins: [naive] },
  })
}

describe('设置中心', () => {
  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })
  afterEach(() => {
    // 清 NModal teleport 到 body 的残留（避免跨用例 DOM 串扰）
    document.body.innerHTML = ''
  })

  it('默认打开外观页：主题列表渲染', async () => {
    const w = mountDlg()
    await new Promise((r) => setTimeout(r, 0))
    // NModal teleport 到 body
    expect(document.body.textContent).toContain('外观')
    expect(document.body.textContent).toContain('解析规则')
    expect(document.body.textContent).toContain('关于')
    expect(document.body.textContent).toContain('midnight')
  })

  it('改字号 → ui store 更新且 localStorage 落盘', async () => {
    mountDlg()
    await new Promise((r) => setTimeout(r, 0))
    const ui = useUiStore()
    ui.fontSize = 16
    ui.persist()
    expect(JSON.parse(localStorage.getItem('fview-ui')!).fontSize).toBe(16)
  })

  it('解析规则页：渲染规则表格，可新增并保存', async () => {
    const w = mountDlg()
    await new Promise((r) => setTimeout(r, 0))
    // 切到解析规则页（teleport 内容里找 nav 按钮）
    const navBtns = document.body.querySelectorAll('.nav-item')
    const rulesBtn = Array.from(navBtns).find((b) => b.textContent?.includes('解析规则'))
    rulesBtn!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))
    // 规则名在 input value 里（textContent 查不到）
    const nameInputs = document.body.querySelectorAll<HTMLInputElement>('.row .c-name')
    expect(nameInputs.length).toBeGreaterThanOrEqual(2)
    expect(nameInputs[0].value).toBe('java-logback')
    // 新增一行
    const addBtn = document.body.querySelector('.rules-add')
    addBtn!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))
    const saveBtn = document.body.querySelector('.rules-save')
    expect(saveBtn).toBeTruthy()
    saveBtn!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))
    expect(backend.saveRules).toHaveBeenCalled()
  })
})
