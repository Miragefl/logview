import { defineStore } from 'pinia'
import { getFields, saveFields } from '../lib/backend'
import type { FieldConfig } from '../lib/backend'

// rules.yaml fields 的前端单一正本(默认字段层,2026-11-03 spec):
// App/Toolbar/SettingsAppearance 共用本 store,消灭"两处各持副本写回互踩"
// (旧 Toolbar F 写 visible 会覆盖 App 拖宽落盘的 width)。
// 单日志覆盖层不在此(见 lib/fieldOverride.ts,localStorage 不进 rules.yaml)。

/** ALL_FIELDS 全部标准字段(fallback 全显序,与 TUI FieldMask 同源语义)。 */
export const ALL_FIELDS = ['time', 'level', 'thread', 'traceId', 'logger', 'message']

export const useFieldsStore = defineStore('fields', {
  state: () => ({
    /** all rules.yaml fields 全量配置(含 width 覆盖),分栏列宽数据源。 */
    all: [] as FieldConfig[],
    /** loaded load 是否已跑(幂等标记,组件挂载处可重复调 load)。 */
    loaded: false,
  }),
  getters: {
    /** defaults 默认可见字段:按 rules.yaml 配置序;全不勾/未加载 fallback 全显(现状语义)。 */
    defaults(state): string[] {
      const vis = state.all.filter((f) => f.visible).map((f) => f.name)
      return vis.length > 0 ? vis : [...ALL_FIELDS]
    },
  },
  actions: {
    /** load 拉取 rules.yaml fields(失败保空走 fallback;幂等)。 */
    async load(): Promise<void> {
      try {
        this.all = (await getFields()) ?? []
      } catch {
        this.all = []
      }
      this.loaded = true
    },
    /**
     * setVisibility 设置字段默认显隐并整体落盘(设置中心-外观页入口)。
     * 回滚模式:改本地 → saveFields 失败恢复原值(保本地态,下次重试)。
     */
    async setVisibility(name: string, visible: boolean): Promise<void> {
      const f = this.all.find((x) => x.name === name)
      if (!f) return
      const prev = f.visible
      if (prev === visible) return
      f.visible = visible
      try {
        await saveFields(this.all)
      } catch {
        f.visible = prev
      }
    },
    /** setWidths 列宽覆盖落盘(表头拖拽入口,保留 visible 不动)。 */
    async setWidths(w: Record<string, number>): Promise<void> {
      const prev = new Map<string, number | undefined>()
      for (const f of this.all) {
        if (w[f.name] === undefined) continue
        prev.set(f.name, f.width)
        f.width = w[f.name]
      }
      try {
        await saveFields(this.all)
      } catch {
        for (const f of this.all) {
          if (prev.has(f.name)) f.width = prev.get(f.name)
        }
      }
    },
  },
})
