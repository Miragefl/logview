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
  // wrap 为会话态开关(w 键),不持久化——loadPersisted 不含它;
  // dialogs 为打开中的弹窗 id 栈(键盘层让位/ESC 逐层关闭,第三轮验收②)
  // browse 为全局目录浏览弹窗态(SSH/FRP 面板与欢迎页最近源共用,spec 2026-11-03)
  state: () => ({
    ...loadPersisted(),
    wrap: false,
    dialogs: [] as string[],
    visualActive: false,
    browse: { show: false, mode: 'ssh' as 'ssh' | 'frp', host: '', connName: '', initPath: '' },
  }),
  getters: {
    /** modalOpen 是否有弹窗打开(全局 vim 键此时让位,由弹窗自理)。 */
    modalOpen(state): boolean {
      return state.dialogs.length > 0
    },
    /** topDialog 栈顶弹窗 id(ESC 关闭目标)。 */
    topDialog(state): string {
      return state.dialogs[state.dialogs.length - 1] ?? ''
    },
  },
  actions: {
    /** dialogOpen 弹窗打开登记(id 重复时不重复入栈)。 */
    dialogOpen(id: string) {
      if (!this.dialogs.includes(id)) this.dialogs.push(id)
    },
    /** dialogClose 弹窗关闭出栈。 */
    dialogClose(id: string) {
      this.dialogs = this.dialogs.filter((d) => d !== id)
    },
    /** openBrowse 打开全局目录浏览弹窗（SSH/FRP 面板点击与欢迎页最近源共用入口）。 */
    openBrowse(p: { mode: 'ssh' | 'frp'; host?: string; connName?: string; initPath?: string }) {
      this.browse = {
        show: true,
        mode: p.mode,
        host: p.host ?? '',
        connName: p.connName ?? '',
        initPath: p.initPath ?? '',
      }
    },
    /** closeBrowse 关闭全局目录浏览弹窗。 */
    closeBrowse() {
      this.browse.show = false
    },
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
