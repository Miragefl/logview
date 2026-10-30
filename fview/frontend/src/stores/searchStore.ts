// 统一搜索弹窗 store(验收反馈③):搜索/高亮/隐藏三合一弹窗态 +
// 会话级搜索历史(内存 cap 20,去重提升,最新在上——与 TUI addSearchHistory
// 语义一致;高亮/隐藏词频走共享 usage.json,不在此)。
import { defineStore } from 'pinia'

/** SEARCH_HISTORY_CAP 搜索历史上限(对齐 TUI)。 */
const SEARCH_HISTORY_CAP = 20

export const useSearchStore = defineStore('search', {
  state: () => ({
    /** 弹窗显隐 */
    show: false,
    /** 当前分区:0=搜索 1=高亮 2=隐藏(Tab 键轮换) */
    tab: 0,
    /** 会话级搜索历史(最新在上,不落盘) */
    history: [] as string[],
  }),
  actions: {
    /** open 打开弹窗并定位分区(0/1/2)。 */
    open(tab = 0) {
      this.tab = tab
      this.show = true
    },
    /** close 收起弹窗。 */
    close() {
      this.show = false
    },
    /** cycleTab Tab 键轮换分区。 */
    cycleTab() {
      this.tab = (this.tab + 1) % 3
    },
    /** addSearchHistory 记一条搜索:去重提到最上,cap 20 丢最旧,空串不入。 */
    addSearchHistory(q: string) {
      const w = q.trim()
      if (!w) return
      this.history = [w, ...this.history.filter((h) => h !== w)].slice(0, SEARCH_HISTORY_CAP)
    },
  },
})
