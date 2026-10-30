import { defineStore } from 'pinia'
import type { GlobalThemeOverrides } from 'naive-ui'
import { darkTheme } from 'naive-ui'
import { currentTheme, setTheme, themeCSS, themeNames } from '../lib/backend'
import { WindowSetDarkTheme, WindowSetLightTheme } from '../../wailsjs/runtime/runtime'

// 全局主题系统：ThemeCSS 的 CSS 变量（单源）同时驱动
// ① :root 变量（日志区/自定义样式直接用） ② Naive UI themeOverrides（壳组件）
// 亮暗基底由 --bg 亮度启发式判定，不给 rules.yaml 加 schema 字段。

/** parseHex 解析 #rgb/#rrggbb 为 [r,g,b]；非法返回 null。 */
function parseHex(v: string): [number, number, number] | null {
  const s = v.trim()
  if (!s.startsWith('#')) return null
  const h = s.slice(1)
  if (h.length === 3) {
    const r = parseInt(h[0] + h[0], 16)
    const g = parseInt(h[1] + h[1], 16)
    const b = parseInt(h[2] + h[2], 16)
    if (Number.isNaN(r) || Number.isNaN(g) || Number.isNaN(b)) return null
    return [r, g, b]
  }
  if (h.length === 6) {
    const r = parseInt(h.slice(0, 2), 16)
    const g = parseInt(h.slice(2, 4), 16)
    const b = parseInt(h.slice(4, 6), 16)
    if (Number.isNaN(r) || Number.isNaN(g) || Number.isNaN(b)) return null
    return [r, g, b]
  }
  return null
}

/**
 * detectBase 按 --bg 亮度判定亮暗基底（YIQ < 150 判 dark：
 * 中等亮度也走暗色，宁可暗底配浅字；缺失/非法回落 dark——
 * 后端 ThemeToCSS 已保证 --bg 非空回落 #10161f，此处为前端兜底）。
 */
export function detectBase(vars: Record<string, string>): 'dark' | 'light' {
  const rgb = parseHex(vars['--bg'] ?? '')
  if (!rgb) return 'dark'
  const yiq = (rgb[0] * 299 + rgb[1] * 587 + rgb[2] * 114) / 1000
  return yiq < 150 ? 'dark' : 'light'
}

/** overrideMap CSS 变量 → Naive UI common overrides 键的映射（缺失键跳过）。 */
const overrideMap: Record<string, string> = {
  '--bg': 'bodyColor',
  '--fg': 'textColorBase',
  '--accent': 'primaryColor',
  '--popup-bg': 'popoverColor',
  '--popup-border': 'borderColor',
  '--dim': 'textColorDisabled',
  '--highlight': 'primaryColorHover',
}

/** toNaiveOverrides 把主题 CSS 变量映射为 Naive UI overrides（cardColor 同 popup-bg）。 */
export function toNaiveOverrides(vars: Record<string, string>): GlobalThemeOverrides {
  const common: Record<string, string> = {}
  for (const [v, k] of Object.entries(overrideMap)) {
    if (vars[v]) common[k] = vars[v]
  }
  if (vars['--popup-bg']) common['cardColor'] = vars['--popup-bg']
  return { common: common as GlobalThemeOverrides['common'] }
}

export const useThemeStore = defineStore('theme', {
  state: () => ({
    name: '',
    nameList: [] as string[],
    vars: {} as Record<string, string>,
    overrides: {} as GlobalThemeOverrides,
    base: 'dark' as 'dark' | 'light',
  }),
  getters: {
    /** naiveTheme Naive UI 基底主题对象（light 返回 null）。 */
    naiveTheme(state) {
      return state.base === 'dark' ? darkTheme : null
    },
  },
  actions: {
    /** apply 切换主题：拉 CSS 变量 → 注入 :root → 更新 overrides/基底 → 原生标题栏跟随 → 持久化。
     * 空值变量不注入（CSS 里 --x:'' 会废掉 var() 的 fallback）；切主题先清旧键防残留。 */
    async apply(name: string) {
      const vars = (await themeCSS(name)) ?? {}
      this.name = name
      this.vars = vars
      this.overrides = toNaiveOverrides(vars)
      this.base = detectBase(vars)
      const root = document.documentElement
      // 清上一主题的全部键（旧键残留会让缺字段的变体错拿旧值）
      for (const k of Object.keys(this.vars)) {
        root.style.removeProperty(k)
      }
      for (const [k, v] of Object.entries(vars)) {
        if (v) root.style.setProperty(k, v)
      }
      // 原生窗口标题栏跟随亮暗（macOS/Windows 均生效；非桌面环境调用无害）
      if (this.base === 'dark') {
        WindowSetDarkTheme()
      } else {
        WindowSetLightTheme()
      }
      await setTheme(name)
    },
    /** init 启动恢复当前主题 + 拉主题名列表（App 挂载时调用一次）。 */
    async init() {
      try {
        this.nameList = (await themeNames()) ?? []
        const cur = (await currentTheme()) || ''
        if (cur) {
          await this.apply(cur)
        }
      } catch {
        // 后端不可用（如测试环境）静默跳过
      }
    },
  },
})
