import { defineStore } from 'pinia'
import type { LineDTO } from '../lib/types'

// 高亮词 + 行级隐藏规则（会话级不落盘；视觉语言沿用 TUI v0.19 双域区分：
// 高亮词=亮色粗体下划线，搜索命中=暗色底纹）

/** HL_COLORS 10 色亮色系轮换板（与 TUI 高亮词视觉语言对齐）。 */
export const HL_COLORS = [
  '#e57373', // 红
  '#e2c77e', // 黄
  '#7ee29a', // 绿
  '#7dc4e4', // 蓝
  '#c792ea', // 紫
  '#f78c6c', // 橙
  '#89ddff', // 天蓝
  '#ff9cac', // 粉
  '#a6e22e', // 黄绿
  '#66d9ef', // 青
]

export interface Highlight {
  word: string
  color: string
}

export const useAnnotationsStore = defineStore('annotations', {
  state: () => ({
    highlights: [] as Highlight[],
    hideRules: [] as string[],
  }),
  getters: {
    /** hidden 判断行是否命中任一隐藏规则（message 或 raw 的子串,忽略大小写——对齐 TUI matchHides）。 */
    hidden(state) {
      return (line: LineDTO): boolean =>
        state.hideRules.some(
          (r) =>
            r !== '' &&
            (line.message.toLowerCase().includes(r.toLowerCase()) ||
              line.raw.toLowerCase().includes(r.toLowerCase()))
        )
    },
  },
  actions: {
    /** addHighlight 加高亮词（已存在不重复），颜色按加入序 10 色轮换。 */
    addHighlight(word: string) {
      if (!word || this.highlights.some((h) => h.word === word)) return
      this.highlights.push({ word, color: HL_COLORS[this.highlights.length % HL_COLORS.length] })
    },
    removeHighlight(word: string) {
      this.highlights = this.highlights.filter((h) => h.word !== word)
    },
    /** addHideRule 加行级隐藏规则（子串匹配；去重）。 */
    addHideRule(rule: string) {
      if (!rule || this.hideRules.includes(rule)) return
      this.hideRules.push(rule)
    },
    removeHideRule(rule: string) {
      this.hideRules = this.hideRules.filter((r) => r !== rule)
    },
    clearHideRules() {
      this.hideRules = []
    },
  },
})
