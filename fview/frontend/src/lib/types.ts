// 行/tab 前端类型（数据源为后端 Manager 事件流；字段与 Go 侧 LineDTO/TabInfo 对齐）
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
  /** 顶栏展示名(文件名;空=回落 label) */
  display?: string
  /** 源信息条全称(路径/服务器/ns 等) */
  info?: string
}
