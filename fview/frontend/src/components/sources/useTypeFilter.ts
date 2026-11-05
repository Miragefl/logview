// useTypeFilter 打字过滤 composable(r2):TUI sourcepicker 式无框过滤——
// 面板 onDocKey 对未消费的可打印无修饰字符直接进词,退格删尾,Esc 词非空清空。
// 词存 ui.panelFilter(会话级):SidebarRail 挂 document capture 先于面板注册,
// 须凭词非空让位(词空才关面板)——Esc 逐级退出的层级依据。
// 修饰键(ctrl/meta/alt)一律不消费:与面板 ⌃j/k/a/d/e/c/n 组合键天然互斥,
// 由面板在调用 typeKey 之前处理。
// r3(browse-typeahead):核心抽纯函数 typeFilterKey + useLocalTypeFilter 本地词版
// (BrowseDialog 弹窗用,词随弹窗生命周期,不进面板共享词 ui.panelFilter)。
import { computed, ref } from 'vue'
import type { Ref } from 'vue'
import { useUiStore } from '../../stores/ui'

/** typeFilterKey 无框打字纯逻辑:可打印无修饰字符进词/退格删尾/Esc 词非空清空。消费返回新词,否则 null。 */
export function typeFilterKey(e: KeyboardEvent, word: string): string | null {
  if (e.ctrlKey || e.metaKey || e.altKey) return null
  if (e.key.length === 1) {
    // 可打印字符(含空格——路径/主机名合法字符)
    return word + e.key
  }
  if (e.key === 'Backspace' && word) {
    return word.slice(0, -1)
  }
  if (e.key === 'Escape' && word) {
    return ''
  }
  return null
}

/** 面板版:词存 ui.panelFilter(会话级,SidebarRail Esc 逐级依据)。 */
export function useTypeFilter() {
  const ui = useUiStore()
  const q = computed(() => ui.panelFilter)

  /** typeKey 面板键盘路由的打字分支:消费返回 true(调用方 preventDefault+stopPropagation)。 */
  function typeKey(e: KeyboardEvent): boolean {
    const next = typeFilterKey(e, ui.panelFilter)
    if (next === null) return false
    ui.panelFilter = next
    return true
  }

  return { q, typeKey }
}

/** 本地词版:词为调用方私有 ref,生命周期随组件(弹窗关=词亡,不串面板会话词)。 */
export function useLocalTypeFilter(): { q: Ref<string>; typeKey(e: KeyboardEvent): boolean } {
  const q = ref('')

  /** typeKey 同面板语义:消费返回 true(调用方 preventDefault)。 */
  function typeKey(e: KeyboardEvent): boolean {
    const next = typeFilterKey(e, q.value)
    if (next === null) return false
    q.value = next
    return true
  }

  return { q, typeKey }
}
