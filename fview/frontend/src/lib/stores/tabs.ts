import { get, writable } from 'svelte/store'

// 行/tab 前端状态（数据源为后端 Manager 事件流；类型与 Go 侧 LineDTO/TabInfo 对齐）

export interface LineDTO {
  seq: number
  time: string
  level: string
  thread: string
  traceId: string
  logger: string
  message: string
  raw: string
}

export interface TabInfo {
  id: string
  label: string
  lines: number
  follow: boolean
}

export interface TabState {
  info: TabInfo
  lines: LineDTO[]
}

// DEFAULT_MAX_LINES 与后端 RingBuffer 默认容量对齐（前端同步裁剪，防双份内存膨胀）
export const DEFAULT_MAX_LINES = 100000

export const tabsStore = writable<TabState[]>([])
export const activeTab = writable<string>('')

/**
 * addLines 向升序行数组合并一批行：
 * - 递增尾部直接追加（follow 增量快路径）
 * - 乱序按 seq 二分插入（repaint 部分重发）
 * - 同 seq 替换（repaint 全量重发的更新语义）
 * - 超过 maxLines 裁掉头部（对齐后端 RingBuffer 淘汰）
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

/** registerTab 新 tab 注册（打开时前端先登记，行流随后到达）。 */
export function registerTab(info: TabInfo): void {
  tabsStore.update((ts) => [...ts, { info, lines: [] }])
  activeTab.set(info.id)
}

/** applyLinesEvent 合并 tab:lines 事件（增量追加/repaint 重发统一走 addLines）。 */
export function applyLinesEvent(ev: { tabId: string; lines: LineDTO[] }): void {
  tabsStore.update((ts) => {
    const i = ts.findIndex((t) => t.info.id === ev.tabId)
    if (i === -1) return ts
    const copy = [...ts]
    copy[i] = { ...copy[i], lines: addLines(copy[i].lines, ev.lines) }
    return copy
  })
}

/** applyMetaEvent 更新 tab 元信息（label/parser 名；EOF 终态也走这里）。 */
export function applyMetaEvent(ev: {
  tabId: string
  label: string
  parserName: string
}): void {
  tabsStore.update((ts) => {
    const i = ts.findIndex((t) => t.info.id === ev.tabId)
    if (i === -1) return ts
    const copy = [...ts]
    copy[i] = { ...copy[i], info: { ...copy[i].info, label: ev.label } }
    return copy
  })
}

/** applyClosedEvent 移除已关 tab；关的是激活 tab 时切到末位。 */
export function applyClosedEvent(ev: { tabId: string }): void {
  tabsStore.update((ts) => ts.filter((t) => t.info.id !== ev.tabId))
  if (get(activeTab) === ev.tabId) {
    const rest = get(tabsStore)
    activeTab.set(rest.length > 0 ? rest[rest.length - 1].info.id : '')
  }
}

/** toggleFollow 切换 tab 跟随态（本地 store + Go 侧同步由调用方负责）。 */
export function toggleFollow(tabId: string, follow: boolean): void {
  tabsStore.update((ts) => {
    const i = ts.findIndex((t) => t.info.id === tabId)
    if (i === -1) return ts
    const copy = [...ts]
    copy[i] = { ...copy[i], info: { ...copy[i].info, follow } }
    return copy
  })
}
