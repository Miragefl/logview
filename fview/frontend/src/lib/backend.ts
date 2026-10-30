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

/** openFileViaDialog 弹系统文件选择框（多选）并开 tab，取消返回 ''。 */
export function openFileViaDialog(): Promise<string> {
  return PickAndOpenFile()
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
