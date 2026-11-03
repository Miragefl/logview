// backend.ts 薄封装直测：mac 菜单 ⌘W 事件 shortcut:closeTab 必须逐字对齐
// Go 侧发射（fview/app.go shortcutCloseTab）——事件名漂移则快捷键静默失效；
// 组件测试全 mock 本层，必须在序列化边界单独钉死。
import { describe, expect, it, vi, beforeEach } from 'vitest'

// mock 工厂被 hoist，不能引用外部变量——工厂内建 vi.fn()，测试经 vi.mocked 取用
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))

import { EventsOn } from '../../../wailsjs/runtime/runtime'
import { onShortcutCloseTab } from '../backend'

const onSpy = vi.mocked(EventsOn)

describe('backend.onShortcutCloseTab 菜单事件订阅', () => {
  beforeEach(() => {
    onSpy.mockClear()
  })

  it('以 shortcut:closeTab 注册回调（事件名与 Go 侧发射逐字一致）', () => {
    const cb = () => {}
    onShortcutCloseTab(cb)
    expect(onSpy).toHaveBeenCalledTimes(1)
    expect(onSpy).toHaveBeenCalledWith('shortcut:closeTab', cb)
  })
})
