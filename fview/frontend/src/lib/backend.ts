// Wails 绑定封装：Go 方法调用 + tab 事件订阅（事件协议见 internal/fview/session.go）
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { frp } from '../../wailsjs/go/models'
import {
  PickAndOpenFile,
  CloseTab,
  SetFollow,
  ParseQuery,
  MatchLines,
  Hides,
  SaveHides,
  Fields,
  SaveFields,
  GetLine,
  ThemeNames,
  ThemeCSS,
  CurrentTheme,
  SetTheme,
  Rules,
  SaveRules,
  SSHBrowse,
  OpenSSHTab,
  FrpServers,
  FrpConns,
  UpsertFrpServer,
  UpsertFrpConn,
  DeleteFrpServer,
  DeleteFrpConn,
  SSHHosts,
  AddSSHHost,
  RemoveSSHHost,
  BrowseFRP,
  ReleaseFRPBrowse,
  OpenFRPTab,
} from '../../wailsjs/go/main/App'

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
  follow?: boolean | null
}

export interface ClosedEvent {
  tabId: string
}

export interface OpenedEvent {
  tabId: string
  label: string
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
  onOpened: (ev: OpenedEvent) => void
  onLines: (ev: LinesEvent) => void
  onMeta: (ev: MetaEvent) => void
  onClosed: (ev: ClosedEvent) => void
}

/** subscribeTabEvents 订阅 Manager 的 tab 事件流（onMount 时调用一次）。 */
export function subscribeTabEvents(cb: TabEventCallbacks): void {
  EventsOn('tab:opened', cb.onOpened)
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

// FieldConfig 前端统一小写字段；与 Wails 生成的大写字段模型在此映射
export interface FieldConfig {
  name: string
  visible: boolean
}

export function getHides(): Promise<string[]> {
  return Hides()
}
export function saveHides(list: string[]): Promise<void> {
  return SaveHides(list)
}
export async function getFields(): Promise<FieldConfig[]> {
  const raw = await Fields()
  return (raw ?? []).map((f) => ({ name: f.Name, visible: f.Visible }))
}
export function saveFields(fields: FieldConfig[]): Promise<void> {
  return SaveFields(fields.map((f) => ({ Name: f.name, Visible: f.visible, Width: 0 })))
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

export interface RuleConfig {
  name: string
  pattern: string
  fallback: boolean
}

export async function getRules(): Promise<RuleConfig[]> {
  const raw = await Rules()
  return (raw ?? []).map((r) => ({ name: r.Name, pattern: r.Pattern, fallback: r.Fallback }))
}

export function saveRules(rules: RuleConfig[]): Promise<void> {
  return SaveRules(rules.map((r) => ({ Name: r.name, Pattern: r.pattern, Parse: '', Fallback: r.fallback })))
}

export interface DirEntry {
  name: string
  isDir: boolean
}

export function sshBrowse(
  host: string,
  path: string,
  password: string
): Promise<DirEntry[]> {
  return SSHBrowse(host, path, password)
}

export function openSSHTab(host: string, path: string, tailLines: number, password: string): Promise<string> {
  return OpenSSHTab(host, path, tailLines, password)
}

// ---- 连接库（共享正本 frp.json + rules.yaml ssh_hosts）----

export interface FrpServer {
  name: string
  addr: string
  token: string
}

export interface FrpConn {
  name: string
  server: string
  sk: string
  proxy: string
  user: string
  path: string
}

export async function getFrpServers(): Promise<FrpServer[]> {
  return (await FrpServers()) ?? []
}
export async function getFrpConns(): Promise<FrpConn[]> {
  return (await FrpConns()) ?? []
}
export function upsertFrpServer(s: FrpServer): Promise<void> {
  return UpsertFrpServer(frp.Server.createFrom({ Name: s.name, Addr: s.addr, Token: s.token }))
}
export function upsertFrpConn(c: FrpConn): Promise<void> {
  return UpsertFrpConn(frp.Conn.createFrom({ Name: c.name, Server: c.server, SK: c.sk, Proxy: c.proxy, User: c.user, Path: c.path }))
}
export function deleteFrpServer(name: string): Promise<void> {
  return DeleteFrpServer(name)
}
export function deleteFrpConn(name: string): Promise<void> {
  return DeleteFrpConn(name)
}
export async function getSSHHosts(): Promise<string[]> {
  return (await SSHHosts()) ?? []
}
export function addSSHHost(host: string): Promise<void> {
  return AddSSHHost(host)
}
export function removeSSHHost(host: string): Promise<void> {
  return RemoveSSHHost(host)
}

export function browseFRP(connName: string, path: string, password: string): Promise<DirEntry[]> {
  return BrowseFRP(connName, path, password)
}
export function releaseFRPBrowse(connName: string): Promise<void> {
  return ReleaseFRPBrowse(connName)
}
export function openFRPTab(connName: string, path: string, tailLines: number, password: string): Promise<string> {
  return OpenFRPTab(connName, path, tailLines, password)
}
