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
  openK8sMergedTab: vi.fn().mockResolvedValue('k8s-merged-tab'),
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
import SidebarRail from '../sources/SidebarRail.vue'
import BrowseDialog from '../sources/BrowseDialog.vue'

// 全实例收集:面板组件把 capture 监听挂 document,残留实例会在后续用例继续
// 消费按键(defaultPrevented 假绿)——两个 describe 的 afterEach 统一 unmount;
// attachTo 真实 DOM——detached 树里 input.focus() 不进 activeElement(jsdom 限制)
const all: ReturnType<typeof mount>[] = []
function mountC(c: Parameters<typeof mount>[0]) {
  const div = document.createElement('div')
  document.body.appendChild(div)
  const w = mount(c as never, { attachTo: div, global: { plugins: [naive] } })
  all.push(w)
  return w
}
function unmountAll() {
  all.forEach((w) => w.unmount())
  all.length = 0
  document.body.innerHTML = ''
}

describe('源面板', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })
  // NModal teleport 到 body 不随 wrapper 卸载——unmountAll 清实例+DOM(document 级监听随实例摘除)
  afterEach(unmountAll)

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
    // r2 Final Review I1:模板跨行残迹曾渲染孤立 ">" 且打乱 grid 布局——钉死
    for (const li of w.findAll('.conn-item')) {
      expect(li.text()).not.toContain('>')
    }
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

// ---- 源面板键盘化(2026-11-04):ctrl+j/k 导航+Enter 激活+动作键,capture 消费 ----
function key(k: string, ctrl = false) {
  document.dispatchEvent(new KeyboardEvent('keydown', { key: k, ctrlKey: ctrl, bubbles: true, cancelable: true }))
}

describe('源面板键盘化', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })
  afterEach(unmountAll)

  it('LocalPanel:ctrl+j 选中+Enter 打开+ctrl+a 呼系统框(裸 o 退役)', async () => {
    const w = mountC(LocalPanel)
    await new Promise((r) => setTimeout(r, 0))
    key('j', true)
    await new Promise((r) => setTimeout(r, 0))
    expect(w.findAll('.recent-item')[0].classes()).toContain('sel')
    key('Enter')
    await new Promise((r) => setTimeout(r, 0))
    expect(backend.openFileTab).toHaveBeenCalledWith('/tmp/app.log', 0, true)
    key('a', true)
    await new Promise((r) => setTimeout(r, 0))
    expect(backend.openFileViaDialog).toHaveBeenCalled()
    // 旧裸键 o 退役:不再呼框(未消费键放行=无面板动作)
    vi.mocked(backend.openFileViaDialog).mockClear()
    key('o')
    await new Promise((r) => setTimeout(r, 0))
    expect(backend.openFileViaDialog).not.toHaveBeenCalled()
  })

  it('LocalPanel:打字过滤+命中高亮+退格删词+Esc 清词,无匹配空态区分', async () => {
    const w = mountC(LocalPanel)
    await new Promise((r) => setTimeout(r, 0))
    // 无框直打:e r r o r → 只剩 error.log,命中字母高亮
    for (const ch of 'error') key(ch)
    await new Promise((r) => setTimeout(r, 0))
    const items = w.findAll('.recent-item')
    expect(items.length).toBe(1)
    expect(items[0].find('.kw-hit').text()).toBe('error')
    expect(w.find('.filter-hint').text()).toContain('error')
    // 退格删词尾 → rro 仍命中
    key('Backspace')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.findAll('.recent-item').length).toBe(1)
    // ctrl+j 选中过滤域 → Enter 打开
    key('j', true)
    key('Enter')
    await new Promise((r) => setTimeout(r, 0))
    expect(backend.openFileTab).toHaveBeenCalledWith('/tmp/error.log', 0, true)
    // Esc 清词(词空才归 SidebarRail)→ 全量恢复
    key('Escape')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.findAll('.recent-item').length).toBe(2)
    expect(w.find('.filter-hint').exists()).toBe(false)
    // 无匹配空态
    key('z')
    key('z')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.text()).toContain('无匹配')
    key('Escape')
    await new Promise((r) => setTimeout(r, 0))
  })

  it('SSHPanel:ctrl+j/k 移动+Enter 进浏览+⌃d 删除选中(cfg 主机跳过)', async () => {
    const w = mountC(SSHPanel)
    await new Promise((r) => setTimeout(r, 0))
    key('j', true)
    key('j', true)
    await new Promise((r) => setTimeout(r, 0))
    expect(w.findAll('.host-item')[1].classes()).toContain('sel')
    key('Enter')
    await new Promise((r) => setTimeout(r, 0))
    expect(useUiStore().browse).toMatchObject({ show: true, mode: 'ssh', host: 'dev-box' })
    // 光标回第一项(cfg 主机)按 ⌃d:~/.ssh/config 来源删不掉,跳过不调 remove
    useUiStore().browse.show = false
    key('k', true)
    await new Promise((r) => setTimeout(r, 0))
    key('d', true)
    await new Promise((r) => setTimeout(r, 0))
    expect(backend.removeSSHHost).not.toHaveBeenCalled()
    // 第二项(手动主机)⌃d 直接删
    key('j', true)
    key('d', true)
    await new Promise((r) => setTimeout(r, 0))
    expect(backend.removeSSHHost).toHaveBeenCalledWith('dev-box')
  })

  it('FRPPanel:ctrl+a 开表单+表单期列表键让位+e 编辑预填+d 删除(裸 n 退役)', async () => {
    const w = mountC(FRPPanel)
    await new Promise((r) => setTimeout(r, 0))
    key('a', true)
    await new Promise((r) => setTimeout(r, 0))
    const ui = useUiStore()
    expect(ui.dialogs).toContain('frp-form')
    // 表单开着:ctrl+j 不得动列表选中(列表键全让位)
    key('j', true)
    await new Promise((r) => setTimeout(r, 0))
    expect(w.findAll('.conn-item')[0].classes()).not.toContain('sel')
    // Esc 关表单后:ctrl+j 选中第一项,⌃e 编辑预填 office-frp
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))
    key('j', true)
    await new Promise((r) => setTimeout(r, 0))
    expect(w.findAll('.conn-item')[0].classes()).toContain('sel')
    key('e', true)
    await new Promise((r) => setTimeout(r, 0))
    expect(ui.dialogs).toContain('frp-form')
    expect((w.vm as unknown as { form: { name: string } }).form.name).toBe('office-frp')
    // 关表单,⌃d 删除选中
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))
    key('d', true)
    await new Promise((r) => setTimeout(r, 0))
    expect(backend.deleteFrpConn).toHaveBeenCalledWith('office-frp')
    // 旧裸键 n/e/d 退役:不触发动作(裸字母进词)
    vi.mocked(backend.deleteFrpConn).mockClear()
    key('n')
    key('e')
    key('d')
    await new Promise((r) => setTimeout(r, 0))
    expect(ui.dialogs).not.toContain('frp-form')
    expect(backend.deleteFrpConn).not.toHaveBeenCalled()
    // 裸字母进词(ned 无命中)但不过滤出删除动作
    expect(w.find('.filter-hint').text()).toContain('ned')
    key('Escape')
    await new Promise((r) => setTimeout(r, 0))
  })

  it('FRPPanel:打字过滤(name/alias 命中)+高亮+Enter 进浏览', async () => {
    const w = mountC(FRPPanel)
    await new Promise((r) => setTimeout(r, 0))
    // name 命中(ASCII 直通)
    for (const ch of 'home') key(ch)
    await new Promise((r) => setTimeout(r, 0))
    let items = w.findAll('.conn-item')
    expect(items.length).toBe(1)
    expect(items[0].find('.kw-hit').text()).toBe('home')
    // ctrl+j 选中过滤域 → Enter 进浏览
    key('j', true)
    key('Enter')
    await new Promise((r) => setTimeout(r, 0))
    expect(useUiStore().browse).toMatchObject({ show: true, mode: 'frp', connName: 'home-frp' })
    // Esc 逐级:浏览弹窗期面板让位(词不清)——模拟浏览关闭再 Esc 清词
    useUiStore().browse.show = false
    key('Escape')
    await new Promise((r) => setTimeout(r, 0))
    key('办')
    await new Promise((r) => setTimeout(r, 0))
    items = w.findAll('.conn-item')
    expect(items.length).toBe(1)
    expect(items[0].text()).toContain('办公隧道')
    key('Escape')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.findAll('.conn-item').length).toBe(2)
  })

  it('K8sPanel:资源列表 ctrl+j 选中+Enter 开 tab', async () => {
    const w = mountC(K8sPanel)
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))
    key('j', true)
    await new Promise((r) => setTimeout(r, 0))
    expect(w.findAll('.res-item')[0].classes()).toContain('sel')
    key('Enter')
    await new Promise((r) => setTimeout(r, 0))
    expect(backend.openK8sTab).toHaveBeenCalledWith('pod/order-svc-xk2lp', 'default', 'prod', 0)
  })

  it('K8sPanel:contexts 加载 pending(kubectl 慢/hang)时键盘监听已可用——真机"ctrl+j/k 没用"回归', async () => {
    // 数据链唯一慢源(exec kubectl):监听若挂在 await 后,慢源期间按键全失效
    vi.mocked(backend.getK8sContexts).mockReturnValue(new Promise(() => {}))
    const w = mountC(K8sPanel)
    await new Promise((r) => setTimeout(r, 0))
    // contexts 未回,列表空——但监听必须已挂(按键不得无声丢弃)
    key('j', true)
    key('k', true)
    // 无列表可断言 sel,退而断言事件被消费(preventDefault 生效 = 监听在跑)
    const ev = new KeyboardEvent('keydown', { key: 'j', ctrlKey: true, bubbles: true, cancelable: true })
    document.dispatchEvent(ev)
    expect(ev.defaultPrevented).toBe(true)
    void w
  })

  it('K8sPanel:ctrl+c 循环 context(联动 ns 重载)、ctrl+n 循环 namespace(裸 c/n 退役)', async () => {
    // 上一用例把 getK8sContexts 换成了 never-pending(mockReturnValue 属 implementation,
    // clearAllMocks 不清)——显式恢复,否则 mounted 链断在第一步
    vi.mocked(backend.getK8sContexts).mockResolvedValue(['prod', 'dev'])
    const w = mountC(K8sPanel)
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))
    expect(backend.getK8sNamespaces).toHaveBeenCalledWith('prod')

    key('c', true)
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))
    // prod→dev(尾回首再回 prod 也覆盖)+1 步:dev 的 ns 链被触发
    expect(backend.getK8sNamespaces).toHaveBeenCalledWith('dev')

    key('n', true)
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))
    // default→kube-system:资源以新 ns 重载
    expect(backend.getK8sResources).toHaveBeenCalledWith('dev', 'kube-system')

    // 旧裸键 c/n 退役:纯键位迁移,裸键不再切(未消费放行)
    const ctxCalls = vi.mocked(backend.getK8sNamespaces).mock.calls.length
    key('c')
    key('n')
    await new Promise((r) => setTimeout(r, 0))
    expect(vi.mocked(backend.getK8sNamespaces).mock.calls.length).toBe(ctxCalls)
    // ⌘C 放行钉死(mac 复制键 meta+c 面板不拦,⌃c=切 ctx 复制让位是用户拍板语义)
    const evMeta = new KeyboardEvent('keydown', { key: 'c', metaKey: true, bubbles: true, cancelable: true })
    document.dispatchEvent(evMeta)
    await new Promise((r) => setTimeout(r, 0))
    expect(vi.mocked(backend.getK8sNamespaces).mock.calls.length).toBe(ctxCalls)
    void w
  })

  it('K8sPanel:打字过滤资源(kind/name 命中)+高亮+Enter 开 tab', async () => {
    vi.mocked(backend.getK8sContexts).mockResolvedValue(['prod', 'dev'])
    const w = mountC(K8sPanel)
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))
    // name 命中(ASCII 直通)
    for (const ch of 'pay') key(ch)
    await new Promise((r) => setTimeout(r, 0))
    let items = w.findAll('.res-item')
    expect(items.length).toBe(1)
    expect(items[0].find('.kw-hit').text()).toBe('pay')
    // 退格清词 → kind 命中 deployment
    for (let i = 0; i < 3; i++) key('Backspace')
    for (const ch of 'deployment') key(ch)
    await new Promise((r) => setTimeout(r, 0))
    items = w.findAll('.res-item')
    expect(items.length).toBe(1)
    expect(items[0].find('.kw-hit').text()).toBe('deployment')
    // ctrl+j 选中过滤域 → Enter 开 tab
    key('j', true)
    vi.mocked(backend.openK8sTab).mockClear()
    key('Enter')
    await new Promise((r) => setTimeout(r, 0))
    expect(backend.openK8sTab).toHaveBeenCalledWith('deployment/pay-svc', 'default', 'prod', 0)
    key('Escape')
    await new Promise((r) => setTimeout(r, 0))
  })

  it('SSHPanel:ctrl+a 开添加弹窗,Enter 提交添加,Esc 兜底关闭(裸 a 退役)', async () => {
    const w = mountC(SSHPanel)
    await new Promise((r) => setTimeout(r, 0))
    key('a', true)
    await new Promise((r) => setTimeout(r, 0))
    const ui = useUiStore()
    expect(ui.dialogs).toContain('ssh-form')
    // 弹窗内填主机 Enter 提交 → addSSHHost + 弹窗自动关
    const input = document.body.querySelector<HTMLInputElement>('.ssh-form input')!
    input.value = 'root@10.0.0.1:2222'
    input.dispatchEvent(new Event('input', { bubbles: true }))
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))
    await new Promise((r) => setTimeout(r, 0))
    expect(backend.addSSHHost).toHaveBeenCalledWith('root@10.0.0.1:2222')
    expect(ui.dialogs).not.toContain('ssh-form')
    // 弹窗期列表键让位(FRP 同款):ctrl+j 不得动列表选中
    key('a', true)
    await new Promise((r) => setTimeout(r, 0))
    expect(ui.dialogs).toContain('ssh-form')
    key('j', true)
    await new Promise((r) => setTimeout(r, 0))
    expect(w.findAll('.host-item')[0].classes()).not.toContain('sel')
    // Esc 兜底关闭(焦点在 body 时 naive esc 失灵,面板 onDocKey 兜底——FRP 第三轮验收②同款)
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await new Promise((r) => setTimeout(r, 0))
    expect(ui.dialogs).not.toContain('ssh-form')
    // 旧裸键 a 退役:无 ctrl 不开弹窗
    vi.mocked(backend.addSSHHost).mockClear()
    key('a')
    await new Promise((r) => setTimeout(r, 0))
    expect(ui.dialogs).not.toContain('ssh-form')
    expect(backend.addSSHHost).not.toHaveBeenCalled()
  })

  it('SSHPanel:打字过滤(d 开头词不误触删除)+高亮+Enter 进浏览', async () => {
    const w = mountC(SSHPanel)
    await new Promise((r) => setTimeout(r, 0))
    // d 开头过滤词:裸 d 已退役进词,不触发删除(⌃d 才是删除)
    for (const ch of 'dev') key(ch)
    await new Promise((r) => setTimeout(r, 0))
    const items = w.findAll('.host-item')
    expect(items.length).toBe(1)
    expect(items[0].find('.kw-hit').text()).toBe('dev')
    // ctrl+j 选中过滤域 → Enter 进浏览
    key('j', true)
    key('Enter')
    await new Promise((r) => setTimeout(r, 0))
    expect(useUiStore().browse).toMatchObject({ show: true, mode: 'ssh', host: 'dev-box' })
    // Esc 逐级:浏览弹窗开着时面板让位(词不清)——先模拟浏览关闭,再 Esc 清词 → 全量恢复
    useUiStore().browse.show = false
    key('Escape')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.findAll('.host-item').length).toBe(2)
    // 裸 d 进词不删除(旧动作键退役)
    vi.mocked(backend.removeSSHHost).mockClear()
    key('d')
    await new Promise((r) => setTimeout(r, 0))
    expect(vi.mocked(backend.removeSSHHost)).not.toHaveBeenCalled()
    expect(w.find('.filter-hint').text()).toContain('d')
    key('Escape')
    await new Promise((r) => setTimeout(r, 0))
  })

  it('FRPPanel:d 删除后 sel 落到顶上来的下一项,焦点不消失', async () => {
    // 可变清单:删除真缩短列表,sel clamp 才可断言(固定 mock 删后永远 2 项)
    let conns = [
      { name: 'office-frp', alias: '', type: 'stcp', serverAddr: 'a.com', serverPort: 7000, token: '', serverName: 's', secretKey: 'k', remotePort: 0, user: '', path: '' },
      { name: 'home-frp', alias: '', type: 'tcp', serverAddr: 'a.com', serverPort: 7000, token: '', serverName: '', secretKey: '', remotePort: 6000, user: '', path: '' },
    ]
    vi.mocked(backend.getFrpConns).mockImplementation(async () => conns)
    vi.mocked(backend.deleteFrpConn).mockImplementation(async (name: string) => {
      conns = conns.filter((c) => c.name !== name)
    })
    const w = mountC(FRPPanel)
    await new Promise((r) => setTimeout(r, 0))
    key('j', true)
    key('j', true)
    await new Promise((r) => setTimeout(r, 0))
    expect(w.findAll('.conn-item')[1].classes()).toContain('sel') // 选中 home-frp

    key('d', true)
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))
    // 删除后剩余 1 项(office-frp),sel 应回落到第 1 项而非消失
    const items = w.findAll('.conn-item')
    expect(items.length).toBe(1)
    expect(items[0].classes()).toContain('sel')
  })

  // ---- Final Review 修复轮(I1/I2):面板 capture 与其他 document 键盘消费者共存 ----

  it('BrowseDialog 打开期面板键全让位:ctrl+j 不动背面 sel、Enter 不切浏览目标、d 不误删', async () => {
    const w = mountC(SSHPanel)
    await new Promise((r) => setTimeout(r, 0))
    key('j', true)
    key('Enter') // 选中 web1 进目录浏览 → ui.browse 弹窗(登记 dialogs 栈)
    await new Promise((r) => setTimeout(r, 0))
    const ui = useUiStore()
    expect(ui.browse).toMatchObject({ show: true, host: 'web1' })
    // 弹窗期按面板键:sel 保持第一项不动、不误删主机、不把浏览弹窗切到另一台主机
    vi.mocked(backend.removeSSHHost).mockClear()
    key('j', true)
    key('d')
    key('Enter')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.findAll('.host-item')[0].classes()).toContain('sel')
    expect(w.findAll('.host-item')[1].classes()).not.toContain('sel')
    expect(backend.removeSSHHost).not.toHaveBeenCalled()
    expect(useUiStore().browse).toMatchObject({ show: true, host: 'web1' })
  })

  it('BrowseDialog 打开期 FRP d 不静默删除背后面板选中的连接(配置丢失路径)', async () => {
    const w = mountC(FRPPanel)
    await new Promise((r) => setTimeout(r, 0))
    key('j', true)
    key('Enter') // 选中 office-frp 进浏览
    await new Promise((r) => setTimeout(r, 0))
    expect(useUiStore().browse).toMatchObject({ show: true, connName: 'office-frp' })
    vi.mocked(backend.deleteFrpConn).mockClear()
    key('d')
    await new Promise((r) => setTimeout(r, 0))
    expect(backend.deleteFrpConn).not.toHaveBeenCalled()
    expect(useUiStore().browse).toMatchObject({ show: true, connName: 'office-frp' })
  })

  it('SidebarRail:Esc 逐级退出——词非空清词面板在,词空关面板,弹窗期让位', async () => {
    const w = mountC(SidebarRail)
    await new Promise((r) => setTimeout(r, 0))
    await w.findAll('.icon-btn')[0].trigger('click') // 开 local 面板
    await new Promise((r) => setTimeout(r, 0))
    expect(w.find('.panel').exists()).toBe(true)
    // 打字进词 → Esc 清词(面板还在)
    key('e')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.find('.filter-hint').exists()).toBe(true)
    key('Escape')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.find('.filter-hint').exists()).toBe(false)
    expect(w.find('.panel').exists()).toBe(true)
    // 词空再 Esc → 关面板(q 的反向)
    key('Escape')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.find('.panel').exists()).toBe(false)
    // 再开面板,弹窗开着(ui.browse.show)时 Esc 让位不关面板
    await w.findAll('.icon-btn')[0].trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    useUiStore().browse.show = true
    key('Escape')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.find('.panel').exists()).toBe(true)
  })

  it('四面板元键放行:q(面板循环)/?(帮助)不进词归 App 全局', async () => {
    const w = mountC(LocalPanel)
    await new Promise((r) => setTimeout(r, 0))
    key('q')
    key('?')
    await new Promise((r) => setTimeout(r, 0))
    // 元键不进词(panelFilter 空),事件不被面板消费(defaultPrevented false→App keymap 走 q 循环/? 帮助)
    expect(useUiStore().panelFilter).toBe('')
    const ev = new KeyboardEvent('keydown', { key: 'q', bubbles: true, cancelable: true })
    document.dispatchEvent(ev)
    expect(ev.defaultPrevented).toBe(false)
    void w
  })

  it('K8sPanel:无 kubeconfig 时 ⌃c/⌃n 仍消费不放行(防穿透 App ⌃c 复制)', async () => {
    vi.mocked(backend.getK8sContexts).mockResolvedValue([])
    const w = mountC(K8sPanel)
    await new Promise((r) => setTimeout(r, 0))
    const evC = new KeyboardEvent('keydown', { key: 'c', ctrlKey: true, bubbles: true, cancelable: true })
    document.dispatchEvent(evC)
    expect(evC.defaultPrevented).toBe(true)
    const evN = new KeyboardEvent('keydown', { key: 'n', ctrlKey: true, bubbles: true, cancelable: true })
    document.dispatchEvent(evN)
    expect(evN.defaultPrevented).toBe(true)
    void w
  })

  it('四面板非连续子序列过滤:hde 式——okl 命中 order-svc-xk2lp(各命中字符高亮)', async () => {
    vi.mocked(backend.getK8sContexts).mockResolvedValue(['prod', 'dev'])
    const w = mountC(K8sPanel)
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))
    // o…k…l 按序命中(r3 真机验收:hardware 输 hde 同款语义)
    for (const ch of 'okl') key(ch)
    await new Promise((r) => setTimeout(r, 0))
    const items = w.findAll('.res-item')
    expect(items.length).toBe(1)
    expect(items[0].text()).toContain('order-svc-xk2lp')
    // 命中字符各自高亮(o/k/l 三段 hit)
    const hits = items[0].findAll('.kw-hit')
    expect(hits.map((h) => h.text())).toEqual(['o', 'k', 'l'])
    key('Escape')
    await new Promise((r) => setTimeout(r, 0))
  })

  it('K8sPanel:Space 多选+Enter 合流(字典序/成功清标记)+⌘Enter 各开+空格不进词+切 ns 清', async () => {
    vi.mocked(backend.getK8sContexts).mockResolvedValue(['prod', 'dev'])
    const w = mountC(K8sPanel)
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))
    key('j', true) // 选中第一项
    key(' ') // Space 标记 pod/order-svc-xk2lp
    await new Promise((r) => setTimeout(r, 0))
    expect(w.findAll('.res-item')[0].find('.mark').text()).toBe('✓')
    key('j', true)
    key(' ') // 标记 deployment/pay-svc
    await new Promise((r) => setTimeout(r, 0))
    // 空格被面板消费不进过滤词(K8s 面板语义,与 Local/SSH/FRP 不同)
    expect(useUiStore().panelFilter).toBe('')
    // Enter=合流开单 tab(keys 字典序=TUI sortStrings 对齐;动作成功清标记)
    key('Enter')
    await new Promise((r) => setTimeout(r, 0))
    expect(backend.openK8sMergedTab).toHaveBeenCalledWith(
      ['deployment/pay-svc', 'pod/order-svc-xk2lp'],
      'default',
      'prod',
      0
    )
    expect(w.findAll('.mark').length).toBe(0) // 成功即清(残留会劫持后续 Enter 单开)
    // 重新标记 → ⌘Enter=每个标记项各开一 tab(开完清标记)
    key('k', true) // 回第一项(合流后 navIdx 不变=1,回 0)
    key(' ')
    key('j', true)
    key(' ')
    await new Promise((r) => setTimeout(r, 0))
    vi.mocked(backend.openK8sTab).mockClear()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', metaKey: true, bubbles: true, cancelable: true }))
    await new Promise((r) => setTimeout(r, 0))
    expect(backend.openK8sTab).toHaveBeenCalledTimes(2)
    expect(w.findAll('.mark').length).toBe(0)
    // 切 ns(循环)→ 资源域变,标记清空
    key('n', true)
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))
    expect(w.findAll('.mark').length).toBe(0)
  })

  it('K8sPanel:合流失败面板内提示+标记保留可调整;过滤隐藏的标记项仍进合流 keys', async () => {
    vi.mocked(backend.getK8sContexts).mockResolvedValue(['prod', 'dev'])
    const w = mountC(K8sPanel)
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))
    // 标记两项
    key('j', true)
    key(' ')
    key('j', true)
    key(' ')
    await new Promise((r) => setTimeout(r, 0))
    // 打字过滤到只剩一项——隐藏的标记项仍在合流域
    for (const ch of 'pay') key(ch)
    await new Promise((r) => setTimeout(r, 0))
    expect(w.findAll('.mark').length).toBe(1) // 只剩可见的标记 ✓
    // 合流失败(N 选 1 错全错)→ 面板内错误提示+标记保留
    vi.mocked(backend.openK8sMergedTab).mockRejectedValueOnce(new Error('discover pods: deployment/pay-svc not found'))
    key('Enter')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.find('.err').text()).toContain('pay-svc')
    // 重试成功 → 错误清除+标记清空
    vi.mocked(backend.openK8sMergedTab).mockResolvedValue('k8s-merged-tab')
    key('Escape') // 清过滤词,恢复全列表(标记仍保留——失败不清,此刻 2 个 ✓ 可见)
    await new Promise((r) => setTimeout(r, 0))
    expect(w.findAll('.mark').length).toBe(2) // 失败保留(成功才清)
    key('Enter')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.find('.err').exists()).toBe(false)
    expect(w.findAll('.mark').length).toBe(0)
  })

  it('K8sPanel:NSelect 聚焦态 Enter/ctrl+n 放行(下拉键盘可达路径不被面板抢跑)', async () => {
    vi.mocked(backend.getK8sContexts).mockResolvedValue(['prod', 'dev'])
    const w = mountC(K8sPanel)
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))
    key('j', true) // 选中第一项资源(navIdx=0)——聚焦 NSelect 后 Enter 若被抢跑即开 tab
    await new Promise((r) => setTimeout(r, 0))
    // 非 filterable NSelect 焦点元素是 div.n-base-selection(tabindex=0),非 input——
    // INPUT 放行判定不命中,须 closest 守卫。不真 focus():jsdom 无 matchMedia,
    // naive focus 会炸 vueuc 虚拟列表;守卫只看 e.target,dispatch 直达语义等价
    const sel = document.querySelector('.n-base-selection') as HTMLElement
    expect(sel).toBeTruthy()
    const nsCalls = vi.mocked(backend.getK8sNamespaces).mock.calls.length
    sel.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))
    sel.dispatchEvent(new KeyboardEvent('keydown', { key: 'n', ctrlKey: true, bubbles: true, cancelable: true }))
    await new Promise((r) => setTimeout(r, 0))
    expect(backend.openK8sTab).not.toHaveBeenCalled()
    expect(vi.mocked(backend.getK8sNamespaces).mock.calls.length).toBe(nsCalls)
  })
})
