import { beforeEach, describe, expect, it } from 'vitest'
import { loadOverrides, persistOverrides } from './fieldOverride'

// 单日志字段覆盖层:localStorage 按源全称(tab.info.info)键控,不进 rules.yaml
describe('fieldOverride', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('persist 后 load 往返一致', () => {
    persistOverrides({ '/tmp/app.log': ['time', 'message'], 'user@host:/var/log/x.log': ['message'] })
    expect(loadOverrides()).toEqual({
      '/tmp/app.log': ['time', 'message'],
      'user@host:/var/log/x.log': ['message'],
    })
  })

  it('空/缺失键 → load 返回空对象', () => {
    expect(loadOverrides()).toEqual({})
  })

  it('损坏 JSON → load 吞异常返回空对象', () => {
    localStorage.setItem('fview-fields-override', '{oops')
    expect(loadOverrides()).toEqual({})
  })

  it('非对象 JSON → load 返回空对象', () => {
    localStorage.setItem('fview-fields-override', '["time"]')
    expect(loadOverrides()).toEqual({})
  })

  it('值非数组的键被过滤(坏值炸渲染白屏防线,终审 Minor2)', () => {
    localStorage.setItem('fview-fields-override', JSON.stringify({ 'a.log': 'time', 'b.log': ['time'] }))
    expect(loadOverrides()).toEqual({ 'b.log': ['time'] })
  })
})
