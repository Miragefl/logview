import { defineStore } from 'pinia'

// 桌面专属 UI 设置（字体/字号/行高）：localStorage 持久化，不进 TUI 共享 rules.yaml

const UI_STORAGE_KEY = 'fview-ui'

interface UiPersist {
  fontFamily: string
  fontSize: number
  lineHeight: number
}

function loadPersisted(): UiPersist {
  const defaults: UiPersist = { fontFamily: 'SF Mono, Menlo, monospace', fontSize: 13, lineHeight: 1.5 }
  try {
    const raw = localStorage.getItem(UI_STORAGE_KEY)
    if (!raw) return defaults
    const v = JSON.parse(raw) as Partial<UiPersist>
    return {
      fontFamily: v.fontFamily || defaults.fontFamily,
      fontSize: v.fontSize ?? defaults.fontSize,
      lineHeight: v.lineHeight ?? defaults.lineHeight,
    }
  } catch {
    return defaults
  }
}

export const useUiStore = defineStore('ui', {
  // wrap 为会话态开关(w 键),不持久化——loadPersisted 不含它
  state: () => ({ ...loadPersisted(), wrap: false }),
  actions: {
    /** persist 落盘（调用方在修改后触发，或 App 挂载时 $subscribe 自动调用）。 */
    persist() {
      const v: UiPersist = {
        fontFamily: this.fontFamily,
        fontSize: this.fontSize,
        lineHeight: this.lineHeight,
      }
      localStorage.setItem(UI_STORAGE_KEY, JSON.stringify(v))
    },
    /** rowPx 计算单行像素高（虚拟滚动行高 = 字号 × 行高）。 */
    rowPx(): number {
      return Math.round(this.fontSize * this.lineHeight)
    },
  },
})
