// Wails 绑定封装：Go 方法调用 + tab 事件订阅（事件协议见 internal/fview/session.go）
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { PickAndOpenFile, CloseTab } from '../../wailsjs/go/main/App'
import type { LineDTO, TabInfo } from './stores/tabs'

export interface LinesEvent {
  tabId: string
  lines: LineDTO[]
  firstSeq: number
}

export interface MetaEvent {
  tabId: string
  label: string
  parserName: string
  repaint?: boolean
}

export interface ClosedEvent {
  tabId: string
}

/** openFileViaDialog 弹系统文件选择框（多选）并开 tab，取消返回 ''；follow 请求实时追踪。 */
export function openFileViaDialog(follow: boolean): Promise<string> {
  return PickAndOpenFile(follow)
}

/** setFollow 切换 tab 跟随态。 */
export function setFollow(tabId: string, follow: boolean): Promise<void> {
  return SetFollow(tabId, follow)
}

/** closeTab 关闭指定 tab。 */
export function closeTab(tabId: string): Promise<void> {
  return CloseTab(tabId)
}

export interface TabEventCallbacks {
  onLines: (ev: LinesEvent) => void
  onMeta: (ev: MetaEvent) => void
  onClosed: (ev: ClosedEvent) => void
}

/** subscribeTabEvents 订阅 Manager 的 tab 事件流（onMount 时调用一次）。 */
export function subscribeTabEvents(cb: TabEventCallbacks): void {
  EventsOn('tab:lines', cb.onLines)
  EventsOn('tab:meta', cb.onMeta)
  EventsOn('tab:closed', cb.onClosed)
}

export type { LineDTO, TabInfo }

export interface QueryMeta {
  parseError: string
  highlightKeywords: string[]
  timeRangeHint: string
}

/** parseQuery 校验搜索语法（返回元信息；parseError 非空表示语法错误）。 */
export function parseQuery(input: string): Promise<QueryMeta> {
  return ParseQuery(input)
}

/** matchLines 全量匹配 tab 行，返回命中行索引数组。 */
export function matchLines(tabId: string, query: string): Promise<number[]> {
  return MatchLines(tabId, query)
}

export interface FieldConfig {
  name: string
  visible: boolean
  width?: number
}

export function getHides(): Promise<string[]> {
  return Hides()
}
export function saveHides(list: string[]): Promise<void> {
  return SaveHides(list)
}
export function getFields(): Promise<FieldConfig[]> {
  return Fields()
}
export function saveFields(fields: FieldConfig[]): Promise<void> {
  return SaveFields(fields)
}

/** getLine 取单行详情（键值对：seq/time/level/thread/traceId/logger/message/raw）。 */
export function getLine(tabId: string, idx: number): Promise<Record<string, string>> {
  return GetLine(tabId, idx)
}

export function themeNames(): Promise<string[]> {
  return ThemeNames()
}
export function themeCSS(name: string): Promise<Record<string, string>> {
  return ThemeCSS(name)
}
export function currentTheme(): Promise<string> {
  return CurrentTheme()
}
export function setTheme(name: string): Promise<void> {
  return SetTheme(name)
}
