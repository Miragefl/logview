// useTypeFilter 打字过滤 composable(r2):TUI sourcepicker 式无框过滤——
// 面板 onDocKey 对未消费的可打印无修饰字符直接进词,退格删尾,Esc 词非空清空。
// 词存 ui.panelFilter(会话级):SidebarRail 挂 document capture 先于面板注册,
// 须凭词非空让位(词空才关面板)——Esc 逐级退出的层级依据。
// 修饰键(ctrl/meta/alt)一律不消费:与面板 ⌃j/k/a/d/e/c/n 组合键天然互斥,
// 由面板在调用 typeKey 之前处理。
import { computed } from 'vue'
import { useUiStore } from '../../stores/ui'

export function useTypeFilter() {
  const ui = useUiStore()
  const q = computed(() => ui.panelFilter)

  /** typeKey 面板键盘路由的打字分支:消费返回 true(调用方 preventDefault+stopPropagation)。 */
  function typeKey(e: KeyboardEvent): boolean {
    if (e.ctrlKey || e.metaKey || e.altKey) return false
    if (e.key.length === 1) {
      // 可打印字符(含空格——路径/主机名合法字符)
      ui.panelFilter += e.key
      return true
    }
    if (e.key === 'Backspace' && ui.panelFilter) {
      ui.panelFilter = ui.panelFilter.slice(0, -1)
      return true
    }
    if (e.key === 'Escape' && ui.panelFilter) {
      ui.panelFilter = ''
      return true
    }
    return false
  }

  return { q, typeKey }
}
