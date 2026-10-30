// 词频隔离域(对齐 TUI 各源 Scope):ssh=host、frp=连接名、k8s='k8s'、本地/管道=空即全局。
// 高亮/隐藏词频 usage.json 的 key 域由此决定,SearchPopup 与右键加词共用。

/** scopeOfLabel 从 tab Label 解析词频 scope。 */
export function scopeOfLabel(label: string): string {
  if (label.startsWith('ssh://')) return label.slice(6).split('/')[0]
  if (label.startsWith('frp://')) return label.slice(6).split('/')[0]
  if (label.startsWith('k8s/')) return 'k8s'
  return ''
}
