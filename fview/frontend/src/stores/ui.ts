import { defineStore } from 'pinia'

// 桌面专属 UI 设置（字体/字号/行高）：localStorage 持久化，不进 TUI 共享 rules.yaml

const UI_STORAGE_KEY = 'fview-ui'

interface UiPersist {
  fontFamily: string
  fontSize: number
  lineHeight: number
  /** uiFontSize 界面字号(非日志区,第九轮验收③;旧数据缺省回落 12) */
  uiFontSize: number
}

function loadPersisted(): UiPersist {
  const defaults: UiPersist = { fontFamily: 'SF Mono, Menlo, monospace', fontSize: 13, lineHeight: 1.5, uiFontSize: 12 }
  try {
    const raw = localStorage.getItem(UI_STORAGE_KEY)
    if (!raw) return defaults
    const v = JSON.parse(raw) as Partial<UiPersist>
    return {
      fontFamily: v.fontFamily || defaults.fontFamily,
      fontSize: v.fontSize ?? defaults.fontSize,
      lineHeight: v.lineHeight ?? defaults.lineHeight,
      uiFontSize: v.uiFontSize ?? defaults.uiFontSize,
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
    /** +x/-x 输入态提示(状态栏展示;非输入态空——「混入 ×K」持久徽标走 tab 锚数) */
    ctxHint: '',
    /** 键盘导航态(2026-11-04):vim 键置位/mousemove 复位——日志区藏鼠标指针+抑制
     *  悬停行底色(鼠标停着不动时 hover 底色卡一行碍眼;恢复=动一下鼠标) */
    kbNav: false,
    /** 源面板打字过滤词(r2 无框):会话级共享——SidebarRail 据此 Esc 逐级(词非空让位
     *  面板清词,词空才关面板);切面板(toggle/cyclePane)清空防残留 */
    panelFilter: '',
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
        uiFontSize: this.uiFontSize,
      }
      localStorage.setItem(UI_STORAGE_KEY, JSON.stringify(v))
    },
    /** rowPx 计算单行像素高（虚拟滚动行高 = 字号 × 行高）。 */
    rowPx(): number {
      return Math.round(this.fontSize * this.lineHeight)
    },
  },
})
