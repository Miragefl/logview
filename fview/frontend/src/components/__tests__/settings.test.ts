import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import naive from 'naive-ui'

vi.mock('../../lib/backend', () => ({
  getRules: vi.fn().mockResolvedValue([
    { name: 'java-logback', pattern: '{time} {level} {message}', parse: '', fallback: false },
    { name: 'json-log', pattern: '(?P<raw>.*)', parse: 'json', fallback: true },
  ]),
  saveRules: vi.fn().mockResolvedValue(undefined),
  getPatterns: vi.fn().mockResolvedValue({
    time: '(?P<time>\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2})',
    level: '(?P<level>\\w+)',
  }),
  savePatterns: vi.fn().mockResolvedValue(undefined),
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

  it('解析规则页：列表+弹窗新增保存（parse 透传）+ patterns 片段渲染', async () => {
    const w = mountDlg()
    await new Promise((r) => setTimeout(r, 0))
    const navBtns = document.body.querySelectorAll('.nav-item')
    const rulesBtn = Array.from(navBtns).find((b) => b.textContent?.includes('解析规则'))
    rulesBtn!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))
    // 列表渲染规则名
    expect(document.body.textContent).toContain('java-logback')
    // patterns 片段渲染（time 字段正则可见）
    expect(document.body.textContent).toContain('字段片段')
    // 新增规则：弹窗 → 填表 → 保存
    const addBtn = document.body.querySelector('.rules-add')
    addBtn!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))
    expect(document.body.querySelector('.rule-form')).toBeTruthy()
    const saveBtn = document.body.querySelector('.rule-save')
    saveBtn!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))
    expect(backend.saveRules).toHaveBeenCalled()
    // parse 透传（I-9）：现有 json 规则保存后 parse 不被抹掉
    const savedArg = (backend.saveRules as ReturnType<typeof vi.fn>).mock.calls[0][0] as Array<{
      name: string
      parse: string
    }>
    const jsonRule = savedArg.find((r) => r.name === 'json-log')
    expect(jsonRule?.parse).toBe('json')
  })
})
