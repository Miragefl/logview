// 搜索态 store(第六轮 SLS 式常驻条改造):常驻条聚焦态 + 高亮/隐藏词管理框态 +
// 会话级搜索历史(内存 cap 20,去重提升,最新在上——与 TUI addSearchHistory
// 语义一致;高亮/隐藏词频走共享 usage.json,不在此)。
import { defineStore } from 'pinia'

/** SEARCH_HISTORY_CAP 搜索历史上限(对齐 TUI)。 */
const SEARCH_HISTORY_CAP = 20

export const useSearchStore = defineStore('search', {
  state: () => ({
    /** 常驻搜索条聚焦态(/ 键聚焦,Esc 失焦) */
    barFocused: false,
    /** 高亮/隐藏词管理弹窗:''关 'hl'高亮 'hide'隐藏 */
    wordsShow: '' as '' | 'hl' | 'hide',
    /** 会话级搜索历史(最新在上,不落盘) */
    history: [] as string[],
  }),
  actions: {
    /** focusBar 聚焦常驻搜索条(/、f 键)。 */
    focusBar() {
      this.barFocused = true
    },
    /** blurBar 失焦(词保留,查询态不动)。 */
    blurBar() {
      this.barFocused = false
    },
    /** openWords 打开高亮/隐藏词管理框。 */
    openWords(mode: 'hl' | 'hide') {
      this.wordsShow = mode
    },
    /** closeWords 关闭管理框。 */
    closeWords() {
      this.wordsShow = ''
    },
    /** addSearchHistory 记一条搜索:去重提到最上,cap 20 丢最旧,空串不入。 */
    addSearchHistory(q: string) {
      const w = q.trim()
      if (!w) return
      this.history = [w, ...this.history.filter((h) => h !== w)].slice(0, SEARCH_HISTORY_CAP)
    },
  },
})
