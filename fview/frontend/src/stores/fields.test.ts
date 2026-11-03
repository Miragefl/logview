import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useFieldsStore } from './fields'
import { getFields, saveFields } from '../lib/backend'
import type { FieldConfig } from '../lib/backend'

vi.mock('../lib/backend', () => ({
  getFields: vi.fn(),
  saveFields: vi.fn(),
}))

function cfg(name: string, visible: boolean, width = 0): FieldConfig {
  return { name, visible, width }
}

// fieldsStore:rules.yaml fields 的前端单一正本(默认字段层)。
// visible/width 两类写回都经 saveFields(this.all 整体落盘),互不踩的关键是单源。
describe('useFieldsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(getFields).mockReset()
    vi.mocked(saveFields).mockReset()
  })

  it('load 后 defaults 按 rules.yaml 配置序返回可见字段', async () => {
    vi.mocked(getFields).mockResolvedValue([cfg('time', true, 12), cfg('level', false, 5), cfg('message', true, 0)])
    const s = useFieldsStore()
    await s.load()
    expect(s.defaults).toEqual(['time', 'message'])
  })

  it('getFields 失败 → all 保空,defaults fallback 全显', async () => {
    vi.mocked(getFields).mockRejectedValue(new Error('boom'))
    const s = useFieldsStore()
    await s.load()
    expect(s.all).toEqual([])
    expect(s.defaults).toEqual(['time', 'level', 'thread', 'traceId', 'logger', 'message'])
  })

  it('全部不可见 → defaults fallback 全显(现状语义)', async () => {
    vi.mocked(getFields).mockResolvedValue([cfg('time', false), cfg('level', false)])
    const s = useFieldsStore()
    await s.load()
    expect(s.defaults).toEqual(['time', 'level', 'thread', 'traceId', 'logger', 'message'])
  })

  it('setVisibility 落盘保留列宽(与表头拖宽互不踩)', async () => {
    vi.mocked(getFields).mockResolvedValue([cfg('time', true, 12), cfg('level', true, 5)])
    vi.mocked(saveFields).mockResolvedValue(undefined)
    const s = useFieldsStore()
    await s.load()
    await s.setVisibility('time', false)
    expect(saveFields).toHaveBeenCalledWith([
      { name: 'time', visible: false, width: 12 },
      { name: 'level', visible: true, width: 5 },
    ])
    expect(s.defaults).toEqual(['level'])
  })

  it('setWidths 落盘保留其他字段 visible(与设置页默认字段互不踩)', async () => {
    vi.mocked(getFields).mockResolvedValue([cfg('time', true, 12), cfg('level', false, 5)])
    vi.mocked(saveFields).mockResolvedValue(undefined)
    const s = useFieldsStore()
    await s.load()
    await s.setWidths({ level: 8 })
    expect(saveFields).toHaveBeenCalledWith([
      { name: 'time', visible: true, width: 12 },
      { name: 'level', visible: false, width: 8 },
    ])
  })

  it('saveFields 失败 → 本地回滚不抛', async () => {
    vi.mocked(getFields).mockResolvedValue([cfg('time', true, 12)])
    vi.mocked(saveFields).mockRejectedValue(new Error('disk'))
    const s = useFieldsStore()
    await s.load()
    await expect(s.setVisibility('time', false)).resolves.toBeUndefined()
    expect(s.defaults).toEqual(['time']) // 回滚:仍可见
  })
})
