// 回归（2026-11-02 空记录事故）：backend.ts → wailsjs 的 createFrom 传参必须用小写驼峰 key。
// wails 生成的 Conn 构造器只读 source["name"] 等 camelCase——传 Pascal key 全部变 undefined，
// Go 收到全零 Conn 落盘成空记录。前端组件测试全 mock 本层，故必须单独直测这一层。
import { describe, expect, it, vi, beforeEach } from 'vitest'

// mock 工厂被 hoist，不能引用外部变量——工厂内建 vi.fn()，测试经 vi.mocked 取用
vi.mock('../../../wailsjs/go/main/App', () => ({
  UpsertFrpConn: vi.fn().mockResolvedValue(undefined),
}))
// runtime 依赖（EventsOn 在模块顶层被引用）
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))

import { UpsertFrpConn } from '../../../wailsjs/go/main/App'
import { upsertFrpConn } from '../backend'

const upsertSpy = vi.mocked(UpsertFrpConn)

describe('backend.upsertFrpConn 序列化形状', () => {
  beforeEach(() => {
    upsertSpy.mockClear()
  })

  it('camelCase key 完整到达 wails 绑定层（Pascal key 会全灭成 undefined）', async () => {
    await upsertFrpConn({
      name: '办公-1',
      alias: '办公隧道',
      type: 'stcp',
      serverAddr: 'frps.example.com',
      serverPort: 7000,
      token: 'tk',
      serverName: 'ssh-a',
      secretKey: 'sk',
      remotePort: 0,
      user: 'root',
      path: '/var/log/a.log',
    })
    expect(upsertSpy).toHaveBeenCalledTimes(1)
    const sent = upsertSpy.mock.calls[0][0]
    // 每个字段必须原值到达（undefined 即 createFrom key 错位回归）
    expect(sent.name).toBe('办公-1')
    expect(sent.alias).toBe('办公隧道') // 中文原样（UTF-8 无损）
    expect(sent.type).toBe('stcp')
    expect(sent.serverAddr).toBe('frps.example.com')
    expect(sent.serverPort).toBe(7000)
    expect(sent.token).toBe('tk')
    expect(sent.serverName).toBe('ssh-a')
    expect(sent.secretKey).toBe('sk')
    expect(sent.remotePort).toBe(0)
    expect(sent.user).toBe('root')
    expect(sent.path).toBe('/var/log/a.log')
  })
})
