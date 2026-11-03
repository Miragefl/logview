// BrowseDialog 密码流直测：预填（Keychain 命中免输）、成功回存（幂等覆盖）、
// initPath 落父目录选中文件（spec 2026-11-03-pass-search-recent Task 2）。
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import naive from 'naive-ui'

// mock backend：BrowseDialog 只消费这几个封装（jsdom 无 Wails 运行时）
vi.mock('../../lib/backend', () => ({
  sshBrowse: vi.fn().mockResolvedValue([{ name: 'app.log', isDir: false }, { name: 'logs', isDir: true }]),
  browseFRP: vi.fn().mockResolvedValue([]),
  openSSHTab: vi.fn().mockResolvedValue('tab-1'),
  openFRPTab: vi.fn().mockResolvedValue('tab-2'),
  releaseFRPBrowse: vi.fn().mockResolvedValue(undefined),
  usageTouch: vi.fn().mockResolvedValue(undefined),
  getSavedPass: vi.fn().mockResolvedValue(''),
  savePass: vi.fn().mockResolvedValue(undefined),
}))

import BrowseDialog from '../sources/BrowseDialog.vue'
import { getSavedPass, openSSHTab, savePass, sshBrowse, usageTouch } from '../../lib/backend'

const b = {
  sshBrowse: vi.mocked(sshBrowse),
  openSSHTab: vi.mocked(openSSHTab),
  getSavedPass: vi.mocked(getSavedPass),
  savePass: vi.mocked(savePass),
  usageTouch: vi.mocked(usageTouch),
}

const flush = () => new Promise((r) => setTimeout(r, 0))

// 收集每用例 mount 的 wrapper：afterEach 统一卸载——BrowseDialog 的 document
// keydown 监听随组件卸载移除，不卸载则残 listener 跨用例抢按键（假触发）
const wrappers: ReturnType<typeof mount>[] = []
function mountBrowse(props: Record<string, unknown> = {}) {
  const w = mount(BrowseDialog, {
    props: { show: true, mode: 'ssh', host: 'web1', ...props },
    global: { plugins: [naive] },
  })
  wrappers.push(w)
  return w
}
afterEach(() => {
  wrappers.splice(0).forEach((w) => w.unmount())
  document.body.innerHTML = ''
})

describe('BrowseDialog 密码流（Keychain）', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })
  // NModal teleport 到 body 不随 wrapper 卸载——不清场，后续测试会抓到上个测试残留的弹窗 DOM
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('打开即预填存过的密码：sshBrowse 第三参为存值（免输直查）', async () => {
    b.getSavedPass.mockResolvedValue('p1')
    mountBrowse()
    await flush()
    await flush()
    expect(b.sshBrowse).toHaveBeenCalledWith('web1', '/', 'p1')
  })

  it('认证成功后回存：savePass 以 (ssh:web1, 密码) 被调（幂等覆盖语义）', async () => {
    b.getSavedPass.mockResolvedValue('p1')
    mountBrowse()
    await flush()
    await flush()
    expect(b.savePass).toHaveBeenCalledWith('ssh:web1', 'p1')
  })

  it('无存密码：sshBrowse 第三参空串（needPwd 现流程不破坏、不回存空值）', async () => {
    b.getSavedPass.mockResolvedValue('')
    mountBrowse()
    await flush()
    await flush()
    expect(b.sshBrowse).toHaveBeenCalledWith('web1', '/', '')
    expect(b.savePass).not.toHaveBeenCalled()
  })

  it('initPath 落父目录并选中目标文件（不直接 load 文件全路径）', async () => {
    b.getSavedPass.mockResolvedValue('')
    document.body.innerHTML = ''
    mountBrowse({ initPath: '/var/log/logs' })
    await flush()
    await flush()
    expect(b.sshBrowse).toHaveBeenCalledWith('web1', '/var/log', '')
    // cursor 选中 initPath 目标文件 logs（.sel 项）
    const sel = document.querySelector('li.sel')
    expect(sel?.textContent).toContain('logs')
  })

  it('pickFile 写 v2 键：usageTouch 以 ssh:{host}:{全路径} 被调', async () => {
    b.getSavedPass.mockResolvedValue('')
    mountBrowse()
    await flush()
    await flush()
    document.querySelector<HTMLLIElement>('.entry-file')!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flush()
    expect(b.openSSHTab).toHaveBeenCalledWith('web1', '/app.log', 0, '')
    expect(b.usageTouch).toHaveBeenCalledWith('ssh:web1:/app.log')
  })
})

describe('BrowseDialog 过滤与路径直达', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    b.sshBrowse.mockResolvedValue([
      { name: 'app.log', isDir: false },
      { name: 'error.log', isDir: false },
      { name: 'logs', isDir: true },
    ])
  })
  afterEach(() => {
    document.body.innerHTML = ''
  })

  /** mountBrowse 并等到列表渲染完成 */
  async function mounted() {
    const w = mountBrowse()
    await flush()
    await flush()
    return w
  }

  it('输入即过滤当前目录（子串、不区分大小写）', async () => {
    await mounted()
    const input = document.querySelector<HTMLInputElement>('.filter input')!
    input.value = 'APP'
    input.dispatchEvent(new Event('input', { bubbles: true }))
    await flush()
    const lis = document.querySelectorAll('.entries li')
    expect(lis.length).toBe(1)
    expect(lis[0].textContent).toContain('app.log')
  })

  it('过滤空结果：列表空、Enter 不抛错不开 tab', async () => {
    const w = await mounted()
    const input = document.querySelector<HTMLInputElement>('.filter input')!
    input.value = 'zzz'
    input.dispatchEvent(new Event('input', { bubbles: true }))
    await flush()
    expect(document.querySelectorAll('.entries li').length).toBe(0)
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
    await flush()
    expect(b.openSSHTab).not.toHaveBeenCalled()
    expect(w.emitted('update:show')).toBeFalsy() // 弹窗不关
  })

  it('输入 / 开头回车直达：sshBrowse 以目标路径调用、过滤清空', async () => {
    await mounted()
    const input = document.querySelector<HTMLInputElement>('.filter input')!
    input.value = '/var/log'
    input.dispatchEvent(new Event('input', { bubbles: true }))
    await flush()
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
    await flush()
    await flush()
    expect(b.sshBrowse).toHaveBeenLastCalledWith('web1', '/var/log', '')
    expect(document.querySelectorAll('.entries li').length).toBe(3) // 过滤已清
  })

  it('非 / 开头回车：激活当前选中项（过滤集内）', async () => {
    await mounted()
    const input = document.querySelector<HTMLInputElement>('.filter input')!
    input.value = 'error'
    input.dispatchEvent(new Event('input', { bubbles: true }))
    await flush()
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
    await flush()
    expect(b.openSSHTab).toHaveBeenCalledWith('web1', '/error.log', 0, '')
  })

  it('过滤态 ctrl+j/k 放行导航：焦点在输入框光标仍移动', async () => {
    await mounted()
    const input = document.querySelector<HTMLInputElement>('.filter input')!
    input.focus()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'j', ctrlKey: true }))
    await flush()
    const sel = document.querySelector('li.sel')
    expect(sel?.textContent).toContain('error.log') // cursor 0→1
  })

  it('Esc 分层：有过滤先清过滤不关窗，无过滤才关窗', async () => {
    const w = await mounted()
    const input = document.querySelector<HTMLInputElement>('.filter input')!
    input.value = 'app'
    input.dispatchEvent(new Event('input', { bubbles: true }))
    await flush()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flush()
    expect(document.querySelectorAll('.entries li').length).toBe(3) // 过滤清了
    expect(w.emitted('update:show')).toBeFalsy() // 窗没关
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flush()
    expect(w.emitted('update:show')?.at(-1)).toEqual([false]) // 第二次才关
  })

  it('needPwd 态 Enter 不误激活列表项（keyup retry 才是确认动作,Final Review I2）', async () => {
    b.sshBrowse.mockResolvedValueOnce([
      { name: 'app.log', isDir: false },
      { name: 'logs', isDir: true },
    ])
    b.sshBrowse.mockRejectedValueOnce(new Error('Permission denied (password)'))
    document.body.innerHTML = ''
    mountBrowse({ initPath: '/var/logs' }) // 初始 /var 非根，回上级才有效
    await flush()
    await flush()
    // 回上级触发第二次 browse 失败 → needPwd=true 且 entries 残留
    document.querySelector('.up')!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flush()
    await flush()
    const pwd = document.querySelector<HTMLInputElement>('.pwd input')
    expect(pwd).toBeTruthy()
    pwd!.focus()
    pwd!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
    await flush()
    expect(b.openSSHTab).not.toHaveBeenCalled() // keydown 不得 activate 误开文件
    pwd!.dispatchEvent(new KeyboardEvent('keyup', { key: 'Enter', bubbles: true }))
    await flush()
    expect(b.sshBrowse.mock.calls.length).toBeGreaterThanOrEqual(3) // keyup retry 重查
  })

  it('打开后过滤框自动聚焦——直接打字即过滤，无需点击（验收反馈①）', async () => {
    await mounted()
    const input = document.querySelector<HTMLInputElement>('.filter input')
    expect(input).toBeTruthy()
    expect(document.activeElement).toBe(input)
  })

  it('快速关开换源：迟到的旧预填不得覆盖新源（代际守卫,Final Review I3）', async () => {
    let resolveOld!: (v: string) => void
    b.getSavedPass.mockImplementationOnce(() => new Promise((r) => (resolveOld = r)))
    b.getSavedPass.mockResolvedValueOnce('p2')
    const w = await Promise.resolve(mountBrowse()) // IIFE1 挂在 getSavedPass
    await flush()
    await w.setProps({ show: false })
    await w.setProps({ show: true, host: 'web2' }) // IIFE2 完成
    await flush()
    await flush()
    resolveOld('p1') // IIFE1 迟到恢复
    await flush()
    await flush()
    expect(b.sshBrowse).toHaveBeenLastCalledWith('web2', '/', 'p2') // 旧 IIFE 不得再 browse
  })
})
