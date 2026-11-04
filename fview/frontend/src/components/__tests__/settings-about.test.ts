import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import SettingsAbout from '../dialogs/SettingsAbout.vue'
import { appVersion, checkUpdate } from '../../lib/backend'

vi.mock('../../lib/backend', () => ({
  appVersion: vi.fn(),
  checkUpdate: vi.fn(),
}))
vi.mock('../../assets/appicon.svg', () => ({ default: 'appicon.svg' }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({
  BrowserOpenURL: vi.fn(),
  EventsOn: vi.fn(),
  ClipboardSetText: vi.fn().mockResolvedValue(undefined),
}))

// 关于页:图标+当前版本+检测更新(第九轮验收②)
describe('SettingsAbout', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(appVersion).mockReset()
    vi.mocked(checkUpdate).mockReset()
  })

  it('渲染图标与当前版本(注入 v0.20.1)', async () => {
    vi.mocked(appVersion).mockResolvedValue('v0.20.1')
    const w = mount(SettingsAbout)
    await new Promise((r) => setTimeout(r, 0))
    expect(w.find('img.icon').attributes('src')).toBe('appicon.svg')
    expect(w.text()).toContain('v0.20.1')
  })

  it('dev 构建版本空串显示 dev 兜底', async () => {
    vi.mocked(appVersion).mockResolvedValue('')
    const w = mount(SettingsAbout)
    await new Promise((r) => setTimeout(r, 0))
    expect(w.text()).toContain('dev')
  })

  it('检测更新:发现新版本显示结果与下载链接', async () => {
    vi.mocked(appVersion).mockResolvedValue('v0.20.1')
    vi.mocked(checkUpdate).mockResolvedValue({
      current: 'v0.20.1',
      latest: 'v9.9.9',
      url: 'https://github.com/Miragefl/logview/releases/tag/v9.9.9',
    })
    const w = mount(SettingsAbout)
    await new Promise((r) => setTimeout(r, 0))
    await w.find('button.check').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(checkUpdate).toHaveBeenCalled()
    expect(w.text()).toContain('v9.9.9')
  })

  it('检测更新已是最新', async () => {
    vi.mocked(appVersion).mockResolvedValue('v0.20.1')
    vi.mocked(checkUpdate).mockResolvedValue({
      current: 'v0.20.1',
      latest: 'v0.20.1',
      url: 'https://x',
    })
    const w = mount(SettingsAbout)
    await new Promise((r) => setTimeout(r, 0))
    await w.find('button.check').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.text()).toContain('已是最新')
  })

  it('检测更新失败显示错误可重试', async () => {
    vi.mocked(appVersion).mockResolvedValue('v0.20.1')
    vi.mocked(checkUpdate).mockRejectedValue(new Error('net'))
    const w = mount(SettingsAbout)
    await new Promise((r) => setTimeout(r, 0))
    await w.find('button.check').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.text()).toContain('检测失败')
  })
})
