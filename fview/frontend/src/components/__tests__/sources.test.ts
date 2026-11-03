import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import naive from 'naive-ui'
import { useUiStore } from '../../stores/ui'

// mock backend：源面板只消费封装函数（jsdom 无 Wails 运行时）
vi.mock('../../lib/backend', () => ({
  usageRecent: vi.fn().mockResolvedValue(['file:/tmp/app.log', 'file:/tmp/error.log']),
  usageTouch: vi.fn().mockResolvedValue(undefined),
  getSavedPass: vi.fn().mockResolvedValue(''),
  savePass: vi.fn().mockResolvedValue(undefined),
  openFileTab: vi.fn().mockResolvedValue('tab-1'),
  openFileViaDialog: vi.fn().mockResolvedValue('tab-2'),
  getSSHHosts: vi.fn().mockResolvedValue([
    { name: 'web1', fromSshConfig: true },
    { name: 'dev-box', fromSshConfig: false },
  ]),
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
    { name: 'office-frp', alias: '办公隧道', type: 'stcp', serverAddr: 'frps.example.com', serverPort: 7000, token: 't', serverName: 'ssh-a', secretKey: 'xxx', remotePort: 0, user: '', path: '' },
    { name: 'home-frp', alias: '', type: 'tcp', serverAddr: 'frps.example.com', serverPort: 7000, token: '', serverName: '', secretKey: '', remotePort: 6000, user: 'u2', path: '/var/log/nas.log' },
  ]),
  upsertFrpConn: vi.fn().mockResolvedValue(undefined),
  deleteFrpConn: vi.fn().mockResolvedValue(undefined),
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
  // NModal teleport 到 body 不随 wrapper 卸载——不清场，后续测试会抓到上个测试残留的弹窗 DOM
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('LocalPanel：渲染最近文件，点击开 tab 并计热点', async () => {
    const w = mountC(LocalPanel)
    await new Promise((r) => setTimeout(r, 0))
    expect(w.text()).toContain('app.log')
    expect(w.text()).toContain('error.log')
    // 点击第一项 → openFileTab + usageTouch('file:...')
    await w.findAll('.recent-item')[0].trigger('click')
    expect(backend.openFileTab).toHaveBeenCalledWith('/tmp/app.log', 0, true)
    expect(backend.usageTouch).toHaveBeenCalledWith('file:/tmp/app.log')
  })

  it('SSHPanel：渲染主机列表，点击主机打开目录浏览', async () => {
    const w = mountC(SSHPanel)
    await new Promise((r) => setTimeout(r, 0))
    expect(w.text()).toContain('web1')
    expect(w.text()).toContain('dev-box')
    // ~/.ssh/config 发现的主机：cfg 角标 + 无删除键；手动主机：有 × 可删
    expect(w.text()).toContain('cfg')
    const dels = w.findAll('.del')
    expect(dels.length).toBe(1)
    await dels[0].trigger('click')
    expect(backend.removeSSHHost).toHaveBeenCalledWith('dev-box')
    await w.findAll('.host-item')[0].trigger('click')
    // 全局浏览弹窗（App.vue 单例）：面板点击改为触发 ui.browse 态
    expect(useUiStore().browse).toMatchObject({ show: true, mode: 'ssh', host: 'web1' })
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
      0
    )
    // 写读对称回归网：v2 键前缀 k8sres 与 lib/recent.ts openRecent 拆解格式必须一致
    //（Final Review C1：plan 笔误 k8res 两侧自洽地一起错，此断言钉死契约）
    expect(backend.usageTouch).toHaveBeenCalledWith('k8sres:prod/default/pod/order-svc-xk2lp')
  })

  it('FRPPanel：渲染连接列表（别名优先/回落 name，类型徽标+摘要），点击进入浏览', async () => {
    const w = mountC(FRPPanel)
    await new Promise((r) => setTimeout(r, 0))
    // 别名优先展示；无别名回落 name
    expect(w.text()).toContain('办公隧道')
    expect(w.text()).toContain('home-frp')
    // 官方字段摘要：stcp=serverName@addr / tcp=addr:remotePort
    expect(w.text()).toContain('ssh-a@frps.example.com')
    expect(w.text()).toContain('frps.example.com:6000')
    await w.findAll('.conn-item')[0].trigger('click')
    // 全局浏览弹窗：触发 ui.browse 态（BrowseDialog 挂 App.vue 单例）
    expect(useUiStore().browse).toMatchObject({ show: true, mode: 'frp', connName: 'office-frp' })
  })

  it('FRPPanel：新增 stcp 连接（官方字段）与删除', async () => {
    const w = mountC(FRPPanel)
    await new Promise((r) => setTimeout(r, 0))
    // 打开新增表单（默认 stcp）
    await w.find('.conn-add').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    const q = (ph: string) =>
      document.body.querySelector<HTMLInputElement>(`.conn-form input[placeholder="${ph}"]`)!
    const fill = (ph: string, v: string) => {
      const el = q(ph)
      el.value = v
      el.dispatchEvent(new Event('input', { bubbles: true }))
    }
    fill('如 home-nas', 'nas')
    fill('frps.example.com', 'frps2.example.com')
    fill('如 secret_ssh', 'ssh-nas')
    fill('secretKey', 'sk1')
    fill('root', 'root')
    await new Promise((r) => setTimeout(r, 0))
    document.body.querySelector('.conn-save')!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))
    expect(backend.upsertFrpConn).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'nas', type: 'stcp', serverAddr: 'frps2.example.com', serverPort: 7000,
        serverName: 'ssh-nas', secretKey: 'sk1', user: 'root',
      })
    )
    // 删除连接
    await w.findAll('.conn-del')[0].trigger('click')
    expect(backend.deleteFrpConn).toHaveBeenCalledWith('office-frp')
  })

  it('FRPPanel：stcp 缺 serverName/secretKey 时保存拦截', async () => {
    const w = mountC(FRPPanel)
    await new Promise((r) => setTimeout(r, 0))
    await w.find('.conn-add').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    const q = (ph: string) =>
      document.body.querySelector<HTMLInputElement>(`.conn-form input[placeholder="${ph}"]`)!
    const fill = (ph: string, v: string) => {
      const el = q(ph)
      el.value = v
      el.dispatchEvent(new Event('input', { bubbles: true }))
    }
    fill('如 home-nas', 'nas2')
    fill('frps.example.com', 'frps.example.com')
    await new Promise((r) => setTimeout(r, 0))
    document.body.querySelector('.conn-save')!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))
    expect(backend.upsertFrpConn).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain('serverName')
  })

  it('FRPPanel：编辑 tcp 连接——全字段预填、remotePort 显隐、改名删旧名', async () => {
    const w = mountC(FRPPanel)
    await new Promise((r) => setTimeout(r, 0))
    // 第二条 home-frp 是 tcp 连接（带 remotePort/path）
    await w.findAll('.conn-edit')[1].trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    const q = (ph: string) =>
      document.body.querySelector<HTMLInputElement>(`.conn-form input[placeholder="${ph}"]`)!
    const fill = (ph: string, v: string) => {
      const el = q(ph)
      el.value = v
      el.dispatchEvent(new Event('input', { bubbles: true }))
    }
    expect(q('如 home-nas').value).toBe('home-frp') // name 预填
    expect(q('frps.example.com').value).toBe('frps.example.com') // serverAddr 预填
    expect(q('6000').value).toBe('6000') // tcp：remotePort 预填
    expect(q('如 secret_ssh')).toBeNull() // tcp：stcp 字段不渲染
    // 改名保存 → upsert 新名（tcp 字段+path 透传）+ 删旧名
    fill('如 home-nas', 'nas-frp')
    await new Promise((r) => setTimeout(r, 0))
    document.body.querySelector('.conn-save')!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))
    expect(backend.upsertFrpConn).toHaveBeenCalledWith(
      expect.objectContaining({ name: 'nas-frp', type: 'tcp', serverAddr: 'frps.example.com', remotePort: 6000, path: '/var/log/nas.log' })
    )
    expect(backend.deleteFrpConn).toHaveBeenCalledWith('home-frp')
  })

  it('FRPPanel：编辑改名撞已有连接名——报错拦截不落库', async () => {
    const w = mountC(FRPPanel)
    await new Promise((r) => setTimeout(r, 0))
    await w.findAll('.conn-edit')[0].trigger('click') // office-frp（stcp）
    await new Promise((r) => setTimeout(r, 0))
    const q = (ph: string) =>
      document.body.querySelector<HTMLInputElement>(`.conn-form input[placeholder="${ph}"]`)!
    const el = q('如 home-nas')
    expect(q('如 secret_ssh')).toBeTruthy() // stcp：serverName 字段渲染
    el.value = 'home-frp' // 撞第二条连接名
    el.dispatchEvent(new Event('input', { bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))
    document.body.querySelector('.conn-save')!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))
    expect(backend.upsertFrpConn).not.toHaveBeenCalled()
    expect(backend.deleteFrpConn).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain('已存在')
  })

  // BrowseDialog 目录渲染/点文件开 tab：已迁 browsedialog.test.ts 直测
  // （全局单例化后面板不再内嵌 BrowseDialog，经 SSHPanel 的旧路径已不存在）

  it('FRP 表单弹窗 ESC 兜底关闭(焦点在 body 时 naive esc 依赖焦点失灵,第三轮验收②同款)', async () => {
    const w = mountC(FRPPanel)
    await new Promise((r) => setTimeout(r, 0))
    // 点 ✎ 开编辑表单:焦点仍在 body(列表项),NModal 自带 esc 不触发
    await w.findAll('.conn-edit')[0].trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    const ui = useUiStore()
    expect(ui.dialogs).toContain('frp-form')
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await new Promise((r) => setTimeout(r, 0))
    // 状态层断言:formShow false → 弹窗登记出栈(NModal DOM 卸载在 jsdom 受 transition 影响不可靠)
    expect(ui.dialogs).not.toContain('frp-form')
  })
})
