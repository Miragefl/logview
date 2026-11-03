// Wails 绑定封装：Go 方法调用 + tab 事件订阅（事件协议见 internal/fview/session.go）
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { frp } from '../../wailsjs/go/models'
import {
  GetMemStats,
  OpenFileTab,
  Patterns,
  SavePatterns,
  PickAndOpenFile,
  SystemFonts,
  UsageRecent,
  UsageTouch,
  UsageKeywords,
  BumpKeyword,
  FoldRanges,
  ClearTab,
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
  FrpConns,
  UpsertFrpConn,
  DeleteFrpConn,
  SSHHosts,
  AddSSHHost,
  RemoveSSHHost,
  BrowseFRP,
  ReleaseFRPBrowse,
  OpenFRPTab,
  K8sContexts,
  K8sNamespaces,
  K8sResources,
  OpenK8sTab,
  GetSavedPass,
  SavePass,
} from '../../wailsjs/go/main/App'

import type { LineDTO, TabInfo } from './types'
import type { FoldRange } from './folds'

export interface LinesEvent {
  tabId: string
  lines: LineDTO[]
  firstSeq: number
}

export interface MetaEvent {
  tabId: string
  label: string
  parserName: string
  /** 当前解析器可产出字段集(首次选定/升级/EOF 随 meta 推送;缺省=后端未发,不裁剪) */
  parserFields?: string[]
  repaint?: boolean
  follow?: boolean | null
}

export interface ClosedEvent {
  tabId: string
}

export interface OpenedEvent {
  tabId: string
  label: string
  display?: string
  info?: string
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

/** getSavedPass 取存的密码（系统钥匙串，空串=无；key 式 ssh:{host} / frp:{connName}）。 */
export function getSavedPass(key: string): Promise<string> {
  return GetSavedPass(key)
}

/** savePass 存密码（后端 guard 空值不入库、幂等覆盖）。 */
export function savePass(key: string, pass: string): Promise<void> {
  return SavePass(key, pass)
}

/** clearTab 清屏（C 键,后端清会话缓冲并广播 tab:cleared）。 */
export function clearTab(tabId: string): Promise<void> {
  return ClearTab(tabId)
}

export interface FoldsEvent {
  tabId: string
  ranges: FoldRange[]
}

export interface TabEventCallbacks {
  onOpened: (ev: OpenedEvent) => void
  onLines: (ev: LinesEvent) => void
  onMeta: (ev: MetaEvent) => void
  onClosed: (ev: ClosedEvent) => void
  onFolds: (ev: FoldsEvent) => void
  onCleared: (ev: ClosedEvent) => void
}

/** subscribeTabEvents 订阅 Manager 的 tab 事件流（onMount 时调用一次）。 */
export function subscribeTabEvents(cb: TabEventCallbacks): void {
  EventsOn('tab:opened', cb.onOpened)
  EventsOn('tab:lines', cb.onLines)
  EventsOn('tab:meta', cb.onMeta)
  EventsOn('tab:closed', cb.onClosed)
  EventsOn('tab:folds', cb.onFolds)
  EventsOn('tab:cleared', cb.onCleared)
}

/** onShortcutCloseTab 订阅 mac 菜单 ⌘W 快捷键事件（后端 menu.go 路由、app.go 发射）。
 *  事件名 shortcut:closeTab 与 Go 侧逐字一致，漂移即静默失效（backend-shortcut.test 钉死）。 */
export function onShortcutCloseTab(cb: () => void): void {
  EventsOn('shortcut:closeTab', cb)
}

/** foldRanges 折叠组懒拉（tab 打开/切换时补一次全量）。 */
export function foldRanges(tabId: string): Promise<FoldRange[]> {
  return FoldRanges(tabId)
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
  /** 列宽覆盖(rules.yaml fields[].width,0/缺省用默认;分栏渲染数据源) */
  width?: number
}

export function getHides(): Promise<string[]> {
  return Hides()
}
export function saveHides(list: string[]): Promise<void> {
  return SaveHides(list)
}
export async function getFields(): Promise<FieldConfig[]> {
  const raw = await Fields()
  return (raw ?? []).map((f) => ({ name: f.Name, visible: f.Visible, width: f.Width }))
}
export function saveFields(fields: FieldConfig[]): Promise<void> {
  return SaveFields(fields.map((f) => ({ Name: f.name, Visible: f.visible, Width: f.width ?? 0 })))
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
  /** parse 解析器选择：空=正则；json=JSON 解析（写回必须透传，抹掉会破坏 JSON 规则） */
  parse: string
  fallback: boolean
}

export async function getRules(): Promise<RuleConfig[]> {
  const raw = await Rules()
  return (raw ?? []).map((r) => ({ name: r.Name, pattern: r.Pattern, parse: r.Parse ?? '', fallback: r.Fallback }))
}

export function saveRules(rules: RuleConfig[]): Promise<void> {
  return SaveRules(
    rules.map((r) => ({ Name: r.name, Pattern: r.pattern, Parse: r.parse, Fallback: r.fallback }))
  )
}

/** getPatterns 字段正则片段（rules.yaml patterns 键：{name} 引用 → 实际正则）。 */
export async function getPatterns(): Promise<Record<string, string>> {
  return (await Patterns()) ?? {}
}

/** savePatterns 写回 patterns 键（新 tab 生效）。 */
export function savePatterns(p: Record<string, string>): Promise<void> {
  return SavePatterns(p)
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

/** FrpConn v2：单层平铺 frp 官方字段（type stcp/tcp）；name 是记录名不进 frpc，alias 是纯展示别名。 */
export interface FrpConn {
  name: string
  alias: string
  type: string
  serverAddr: string
  serverPort: number
  token: string
  serverName: string
  secretKey: string
  remotePort: number
  user: string
  path: string
}

export async function getFrpConns(): Promise<FrpConn[]> {
  return (await FrpConns()) ?? []
}
export function upsertFrpConn(c: FrpConn): Promise<void> {
  // createFrom 构造器只认小写驼峰 key（Pascal key 会全灭成 undefined——2026-11-02 空记录事故）
  return UpsertFrpConn(
    frp.Conn.createFrom({
      name: c.name,
      alias: c.alias,
      type: c.type,
      serverAddr: c.serverAddr,
      serverPort: c.serverPort,
      token: c.token,
      serverName: c.serverName,
      secretKey: c.secretKey,
      remotePort: c.remotePort,
      user: c.user,
      path: c.path,
    })
  )
}
export function deleteFrpConn(name: string): Promise<void> {
  return DeleteFrpConn(name)
}
/** SSHHost SSH 主机条目（fromSshConfig=true 来自 ~/.ssh/config 自动发现，不可手动移除）。 */
export interface SSHHost {
  name: string
  fromSshConfig: boolean
}
export async function getSSHHosts(): Promise<SSHHost[]> {
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

export interface K8sResource {
  kind: string
  name: string
}

export async function getK8sContexts(): Promise<string[]> {
  return (await K8sContexts()) ?? []
}
export async function getK8sNamespaces(kubeCtx: string): Promise<string[]> {
  return (await K8sNamespaces(kubeCtx)) ?? []
}
export async function getK8sResources(kubeCtx: string, namespace: string): Promise<K8sResource[]> {
  return (await K8sResources(kubeCtx, namespace)) ?? []
}
export function openK8sTab(resource: string, namespace: string, kubeCtx: string, tailLines: number): Promise<string> {
  return OpenK8sTab(resource, namespace, kubeCtx, tailLines)
}

/** openFileTab 按路径开本地文件 tab（最近文件列表点击用）。 */
export function openFileTab(path: string, tailLines: number, follow: boolean): Promise<string> {
  return OpenFileTab([path], tailLines, follow)
}

/** usageRecent 源热点 key 列表（四类键：file:/ssh:/frp:/k8sres: 前缀，衰减分降序）。 */
export function usageRecent(): Promise<string[]> {
  return UsageRecent()
}

/** usageTouch 记录一次源使用（本地文件传 'file:' + 绝对路径）。 */
export function usageTouch(key: string): Promise<void> {
  return UsageTouch(key)
}

/** usageKeywords 高亮/隐藏词频 top20（与 TUI 共享 usage.json，scope 空即全局）。 */
export function usageKeywords(scope: string, kind: 'hl' | 'hide'): Promise<string[]> {
  return UsageKeywords(scope, kind)
}

/** bumpKeyword 记一次关键词使用（与 TUI 同一份词频，验收反馈③）。 */
export function bumpKeyword(scope: string, kind: 'hl' | 'hide', word: string): Promise<void> {
  return BumpKeyword(scope, kind, word)
}

/** getMemStats 状态栏内存徽章：Sys-HeapReleased 字节数，前端换算 MB。 */
export function getMemStats(): Promise<number> {
  return GetMemStats()
}

/** systemFonts 系统字体名列表（外观页字体下拉数据源）。 */
export function systemFonts(): Promise<string[]> {
  return SystemFonts()
}
