import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import naive from 'naive-ui'

// mock backend：源面板只消费封装函数（jsdom 无 Wails 运行时）
vi.mock('../../lib/backend', () => ({
  usageRecent: vi.fn().mockResolvedValue(['file:/tmp/app.log', 'file:/tmp/error.log']),
  usageTouch: vi.fn().mockResolvedValue(undefined),
  openFileTab: vi.fn().mockResolvedValue('tab-1'),
  openFileViaDialog: vi.fn().mockResolvedValue('tab-2'),
  getSSHHosts: vi.fn().mockResolvedValue(['web1', 'dev-box']),
  addSSHHost: vi.fn().mockResolvedValue(undefined),
  removeSSHHost: vi.fn().mockResolvedValue(undefined),
  sshBrowse: vi.fn().mockResolvedValue([
    { name: 'logs', isDir: true },
    { name: 'app.log', isDir: false },
  ]),
  openSSHTab: vi.fn().mockResolvedValue('ssh-tab'),
  getK8sContexts: vi.fn().mockResolvedValue(['prod', 'dev']),
  getK8sNamespaces: vi.fn().mockResolvedValue(['default', 'kube-system']),
  getK8sResources: vi.fn().mockResolvedValue([
    { kind: 'pod', name: 'order-svc-xk2lp' },
    { kind: 'deployment', name: 'pay-svc' },
  ]),
  openK8sTab: vi.fn().mockResolvedValue('k8s-tab'),
  getFrpConns: vi.fn().mockResolvedValue([
    { name: 'office-frp', server: 'frps.example', sk: 'xxx', proxy: '', user: '', path: '' },
  ]),
  browseFRP: vi.fn().mockResolvedValue([{ name: 'sys.log', isDir: false }]),
  releaseFRPBrowse: vi.fn().mockResolvedValue(undefined),
  openFRPTab: vi.fn().mockResolvedValue('frp-tab'),
  subscribeTabEvents: vi.fn(),
  getHides: vi.fn().mockResolvedValue([]),
  currentTheme: vi.fn().mockResolvedValue(''),
  themeNames: vi.fn().mockResolvedValue([]),
  themeCSS: vi.fn().mockResolvedValue({}),
  setTheme: vi.fn().mockResolvedValue(undefined),
  getMemStats: vi.fn().mockResolvedValue(0),
  closeTab: vi.fn().mockResolvedValue(undefined),
}))

import * as backend from '../../lib/backend'
import LocalPanel from '../sources/LocalPanel.vue'
import SSHPanel from '../sources/SSHPanel.vue'
import K8sPanel from '../sources/K8sPanel.vue'
import FRPPanel from '../sources/FRPPanel.vue'
import BrowseDialog from '../sources/BrowseDialog.vue'

function mountC(c: Parameters<typeof mount>[0]) {
  return mount(c as never, { global: { plugins: [naive] } })
}

describe('源面板', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('LocalPanel：渲染最近文件，点击开 tab 并计热点', async () => {
    const w = mountC(LocalPanel)
    await new Promise((r) => setTimeout(r, 0))
    expect(w.text()).toContain('app.log')
    expect(w.text()).toContain('error.log')
    // 点击第一项 → openFileTab + usageTouch('file:...')
    await w.findAll('.recent-item')[0].trigger('click')
    expect(backend.openFileTab).toHaveBeenCalledWith('/tmp/app.log', 200, true)
    expect(backend.usageTouch).toHaveBeenCalledWith('file:/tmp/app.log')
  })

  it('SSHPanel：渲染主机列表，点击主机打开目录浏览', async () => {
    const w = mountC(SSHPanel)
    await new Promise((r) => setTimeout(r, 0))
    expect(w.text()).toContain('web1')
    expect(w.text()).toContain('dev-box')
    await w.findAll('.host-item')[0].trigger('click')
    expect(backend.sshBrowse).toHaveBeenCalled()
    // NModal teleport 到 body，断言走 document.body.textContent
    expect(document.body.textContent).toContain('logs')
  })

  it('K8sPanel：渲染 context 与资源列表，点击资源开 tab', async () => {
    const w = mountC(K8sPanel)
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))
    expect(w.text()).toContain('order-svc-xk2lp')
    await w.findAll('.res-item')[0].trigger('click')
    expect(backend.openK8sTab).toHaveBeenCalledWith(
      'pod/order-svc-xk2lp',
      'default',
      'prod',
      200
    )
  })

  it('FRPPanel：渲染连接列表，点击进入浏览', async () => {
    const w = mountC(FRPPanel)
    await new Promise((r) => setTimeout(r, 0))
    expect(w.text()).toContain('office-frp')
    await w.findAll('.conn-item')[0].trigger('click')
    expect(backend.browseFRP).toHaveBeenCalled()
  })

  it('BrowseDialog：目录项渲染，点文件开 tab 并计热点', async () => {
    const ssh = mountC(SSHPanel)
    await new Promise((r) => setTimeout(r, 0))
    await ssh.findAll('.host-item')[0].trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    // NModal teleport 到 body：文本与元素都从 document 查
    expect(document.body.textContent).toContain('app.log')
    const fileEl = document.body.querySelector('.entry-file')
    expect(fileEl).toBeTruthy()
    fileEl!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))
    expect(backend.openSSHTab).toHaveBeenCalled()
    expect(backend.usageTouch).toHaveBeenCalledWith('ssh:web1')
  })
})
