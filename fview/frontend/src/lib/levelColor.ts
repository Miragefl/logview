// 日志级别 → 样式类（级别集合对齐 Go 侧 matchLevelFilter：ERROR/ERR/FATAL 等）
export function levelClass(level: string): string {
  const lv = level.toUpperCase()
  if (lv === 'ERROR' || lv === 'ERR' || lv === 'FATAL') return 'lv-error'
  if (lv === 'WARN' || lv === 'WARNING') return 'lv-warn'
  if (lv === 'INFO') return 'lv-info'
  if (lv === 'DEBUG' || lv === 'DBG') return 'lv-debug'
  return ''
}
