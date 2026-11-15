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
    /** 带词聚焦种子(field-interact):focusBar(seed) 写入,SearchBar 消费后清''
     *  ——一次性通道,无 seed 调用不触碰(手动聚焦不清词) */
    seedQ: '',
    /** 高亮/隐藏词管理弹窗:''关 'hl'高亮 'hide'隐藏 */
    wordsShow: '' as '' | 'hl' | 'hide',
    /** 词管理框种子(field-interact):openWords(mode,seed) 写入,WordsPopup 消费后清'' */
    wordsSeed: '',
    /** 会话级搜索历史(最新在上,不落盘) */
    history: [] as string[],
  }),
  actions: {
    /** focusBar 聚焦常驻搜索条(/、f 键);seed 非空时预填搜索词(一次性)。 */
    focusBar(seed?: string) {
      if (seed) this.seedQ = seed
      this.barFocused = true
    },
    /** blurBar 失焦(词保留,查询态不动)。 */
    blurBar() {
      this.barFocused = false
    },
    /** openWords 打开高亮/隐藏词管理框;seed 非空时预填新词输入框(一次性)。 */
    openWords(mode: 'hl' | 'hide', seed?: string) {
      if (seed) this.wordsSeed = seed
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
