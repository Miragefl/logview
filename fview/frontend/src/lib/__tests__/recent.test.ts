// lib/recent.ts 直测：openRecent 四类键分派 + 无密码弹浏览（spec 2026-11-03-pass-search-recent Task 4）。
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('../backend', () => ({
  openFileTab: vi.fn().mockResolvedValue('tab-f'),
  openSSHTab: vi.fn().mockResolvedValue('tab-s'),
  openFRPTab: vi.fn().mockResolvedValue('tab-r'),
  openK8sTab: vi.fn().mockResolvedValue('tab-k'),
  getSavedPass: vi.fn().mockResolvedValue(''),
  usageTouch: vi.fn().mockResolvedValue(undefined),
}))

import { getSavedPass, openFileTab, openFRPTab, openK8sTab, openSSHTab } from '../backend'
import { openRecent, recentLabel } from '../recent'
import { useUiStore } from '../../stores/ui'

const b = {
  openFileTab: vi.mocked(openFileTab),
  openSSHTab: vi.mocked(openSSHTab),
  openFRPTab: vi.mocked(openFRPTab),
  openK8sTab: vi.mocked(openK8sTab),
  getSavedPass: vi.mocked(getSavedPass),
}
const flush = () => new Promise((r) => setTimeout(r, 0))

describe('openRecent 分派', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    b.getSavedPass.mockResolvedValue('')
  })

  it('file: 直接开本地 tab', async () => {
    await openRecent('file:/tmp/app.log')
    expect(b.openFileTab).toHaveBeenCalledWith('/tmp/app.log', 0, true)
  })

  it('ssh v2 键 + 有存密码 → 直接开 tab（带密码）', async () => {
    b.getSavedPass.mockResolvedValue('p1')
    await openRecent('ssh:web1:/var/log/a.log')
    expect(b.getSavedPass).toHaveBeenCalledWith('ssh:web1')
    expect(b.openSSHTab).toHaveBeenCalledWith('web1', '/var/log/a.log', 0, 'p1')
  })

  it('ssh v2 键 + 无存密码 → 弹全局 BrowseDialog（initPath 落点，不静默）', async () => {
    await openRecent('ssh:web1:/var/log/a.log')
    await flush()
    expect(b.openSSHTab).not.toHaveBeenCalled()
    const ui = useUiStore()
    expect(ui.browse.show).toBe(true)
    expect(ui.browse).toMatchObject({ mode: 'ssh', host: 'web1', initPath: '/var/log/a.log' })
  })

  it('frp v2 键：有密码直开、无密码弹浏览（connName+initPath）', async () => {
    await openRecent('frp:office:/x/y.log')
    const ui = useUiStore()
    expect(ui.browse).toMatchObject({ mode: 'frp', connName: 'office', initPath: '/x/y.log' })
    b.getSavedPass.mockResolvedValue('p2')
    await openRecent('frp:office:/x/y.log')
    expect(b.openFRPTab).toHaveBeenCalledWith('office', '/x/y.log', 0, 'p2')
  })

  it('k8sres v2 键 → openK8sTab(ctx/ns/kind/name 拆解)', async () => {
    await openRecent('k8sres:ctx1/ns1/pod/web')
    expect(b.openK8sTab).toHaveBeenCalledWith('pod/web', 'ns1', 'ctx1', 0)
  })

  it('旧格式 ssh:/frp: 键（无路径）→ 降级弹对应目录浏览（根目录,验收反馈②）', async () => {
    await openRecent('frp:XC0902121-1')
    expect(useUiStore().browse).toMatchObject({ show: true, mode: 'frp', connName: 'XC0902121-1' })
    await openRecent('ssh:web1')
    expect(useUiStore().browse).toMatchObject({ show: true, mode: 'ssh', host: 'web1' })
  })

  it('旧格式 k8sres 键（缺 ctx/ns 无法定位）no-op 不抛错', async () => {
    await openRecent('k8sres:pod/web')
    expect(b.openK8sTab).not.toHaveBeenCalled()
  })
})

describe('recentLabel 展示', () => {
  it('四类键的展示名', () => {
    expect(recentLabel('file:/tmp/app.log')).toBe('app.log')
    expect(recentLabel('ssh:web1:/var/log/a.log')).toBe('web1 · a.log')
    expect(recentLabel('frp:office:/x/y.log')).toBe('office · y.log')
    expect(recentLabel('k8sres:ctx1/ns1/pod/web')).toBe('pod/web · ns1')
    expect(recentLabel('ssh:web1')).toBe('ssh:web1') // 旧格式原样
  })
})
