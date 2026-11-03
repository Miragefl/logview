import { defineStore } from 'pinia'
import type { LineDTO, TabInfo } from '../lib/types'
import type { FoldRange } from '../lib/folds'

// 行/tab 前端状态（数据源为后端 Manager 事件流 tab:opened/tab:lines/tab:meta/tab:closed/tab:folds）

export interface TabState {
  info: TabInfo
  lines: LineDTO[]
  /** 解析器可产出字段集(tab:meta 推送;null=后端未发不裁剪,['message']=纯文本) */
  parserFields: string[] | null
  /** 折叠组(tab:folds 推送/懒拉,seq 域) */
  folds: FoldRange[]
  /** 已折叠组的 startSeq(会话级) */
  collapsed: number[]
  /** 书签行 seq(会话级,验收反馈④ m 键) */
  bookmarks: number[]
}

// DEFAULT_MAX_LINES 与后端 RingBuffer 默认容量对齐（前端同步裁剪，防双份内存膨胀）
export const DEFAULT_MAX_LINES = 100000

/**
 * addLines 向升序行数组合并一批行：
 * - 递增尾部直接追加（follow 增量快路径）
 * - 乱序按 seq 二分插入（repaint 部分重发）
 * - 同 seq 替换（repaint 全量重发的更新语义）
 * - 超过 maxLines 裁掉头部（对齐后端 RingBuffer 淘汰）
 * （自 m4m6 Svelte 版原样移植，语义不变）
 */
export function addLines(
  arr: LineDTO[],
  batch: LineDTO[],
  maxLines = DEFAULT_MAX_LINES
): LineDTO[] {
  const out = [...arr]
  for (const ln of batch) {
    if (out.length === 0 || ln.seq > out[out.length - 1].seq) {
      out.push(ln)
      continue
    }
    let lo = 0
    let hi = out.length - 1
    let pos = -1
    while (lo <= hi) {
      const mid = (lo + hi) >> 1
      if (out[mid].seq === ln.seq) {
        pos = mid
        break
      }
      if (out[mid].seq < ln.seq) lo = mid + 1
      else hi = mid - 1
    }
    if (pos >= 0) {
      out[pos] = ln
    } else {
      out.splice(lo, 0, ln)
    }
  }
  if (out.length > maxLines) return out.slice(out.length - maxLines)
  return out
}

export const useTabsStore = defineStore('tabs', {
  state: () => ({
    tabs: [] as TabState[],
    activeId: '',
  }),
  getters: {
    active(state): TabState | null {
      return state.tabs.find((t) => t.info.id === state.activeId) ?? null
    },
  },
  actions: {
    /** register 新 tab 注册（前端打开动作先登记，tab:opened 随后到达时去重）。 */
    register(info: TabInfo) {
      if (!this.tabs.some((t) => t.info.id === info.id)) {
        this.tabs.push({ info, lines: [], parserFields: null, folds: [], collapsed: [], bookmarks: [] })
      }
      this.activeId = info.id
    },
    /** applyLines 合并 tab:lines 事件（增量追加/repaint 重发统一走 addLines），
     * 顺带同步 info.lines（后端 Tabs() 不轮询，行数以本地数组为准）。 */
    applyLines(ev: { tabId: string; lines: LineDTO[] }) {
      const t = this.tabs.find((x) => x.info.id === ev.tabId)
      if (!t) return
      t.lines = addLines(t.lines, ev.lines)
      t.info.lines = t.lines.length
    },
    /** applyOpened 响应 tab:opened（后端 OpenTab 广播,带 display/info）。 */
    applyOpened(ev: { tabId: string; label: string; display?: string; info?: string }) {
      this.register({ id: ev.tabId, label: ev.label, lines: 0, follow: false, display: ev.display, info: ev.info })
    },
    /** applyFolds 应用折叠组变更（tab:folds 推送/懒拉）;清已消失组的孤儿折叠态。 */
    applyFolds(ev: { tabId: string; ranges: FoldRange[] }) {
      const t = this.tabs.find((x) => x.info.id === ev.tabId)
      if (!t) return
      t.folds = ev.ranges ?? []
      const live = new Set(t.folds.map((f) => f.startSeq))
      t.collapsed = t.collapsed.filter((s) => live.has(s))
    },
    /** toggleCollapse 活动 tab 折叠/展开指定组（按 startSeq 键控）。 */
    toggleCollapse(startSeq: number) {
      const t = this.tabs.find((x) => x.info.id === this.activeId)
      if (!t) return
      t.collapsed = t.collapsed.includes(startSeq)
        ? t.collapsed.filter((s) => s !== startSeq)
        : [...t.collapsed, startSeq]
    },
    /** toggleBookmark 活动 tab 光标行书签增删（seq 域,m 键）。 */
    toggleBookmark(seq: number) {
      const t = this.tabs.find((x) => x.info.id === this.activeId)
      if (!t) return
      t.bookmarks = t.bookmarks.includes(seq)
        ? t.bookmarks.filter((s) => s !== seq)
        : [...t.bookmarks, seq]
    },
    /** applyCleared 响应 tab:cleared（C 键清屏:镜像/行数/折叠态全清,tab 不关）。 */
    applyCleared(tabId: string) {
      const t = this.tabs.find((x) => x.info.id === tabId)
      if (!t) return
      t.lines = []
      t.info.lines = 0
      t.folds = []
      t.collapsed = []
      t.bookmarks = []
    },
    /** applyMeta 更新 tab 元信息（label/跟随态/解析器字段集；EOF 终态也走这里）。
     * follow-only meta 不带 parserFields(后端 omitempty),undefined 跳过不清已有值。 */
    applyMeta(ev: { tabId: string; label: string; parserName: string; parserFields?: string[]; follow?: boolean | null }) {
      const t = this.tabs.find((x) => x.info.id === ev.tabId)
      if (!t) return
      t.info.label = ev.label
      if (ev.parserFields !== undefined) t.parserFields = ev.parserFields
      if (ev.follow != null) t.info.follow = ev.follow
    },
    /** toggleFollow 本地切换（Go 侧同步由调用方负责）。 */
    toggleFollow(tabId: string, follow: boolean) {
      const t = this.tabs.find((x) => x.info.id === tabId)
      if (t) t.info.follow = follow
    },
    /** remove 关闭 tab；关的是当前 tab 时回落到邻近 tab。 */
    remove(tabId: string) {
      const i = this.tabs.findIndex((t) => t.info.id === tabId)
      if (i === -1) return
      this.tabs.splice(i, 1)
      if (this.activeId === tabId) {
        this.activeId = this.tabs[Math.min(i, this.tabs.length - 1)]?.info.id ?? ''
      }
    },
  },
})
