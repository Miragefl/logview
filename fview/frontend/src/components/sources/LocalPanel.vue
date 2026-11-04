<script setup lang="ts">
// 本地文件面板：最近文件（usage.json 热点排序，与 TUI 共享）+ 系统选择框新增。
// 键盘化(2026-11-04):ctrl+j/k 选文件、Enter 打开;搜索+新增统一迭代:ctrl+a 呼系统框(裸 o 退役)+搜索过滤。
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { openFileViaDialog, usageRecent } from '../../lib/backend'
import { openRecent } from '../../lib/recent'
import { useUiStore } from '../../stores/ui'
import { isSubseq, splitHighlight } from '../../lib/highlight'
import { useTypeFilter } from './useTypeFilter'

const ui = useUiStore()
/** q 打字过滤词(r2 无框:直接敲字母进词,退格删尾,Esc 清空)。 */
const { q, typeKey } = useTypeFilter()

const recent = ref<string[]>([])

onMounted(async () => {
  document.addEventListener('keydown', onDocKey, true) // 先挂监听再做数据(慢源期间键盘可用,对齐 K8sPanel 教训)
  try {
    recent.value = (await usageRecent()) ?? []
  } catch {
    recent.value = []
  }
})
onBeforeUnmount(() => document.removeEventListener('keydown', onDocKey, true))

/** fileKey 只显示本地文件条目，剥前缀得路径。 */
function filePath(k: string): string {
  return k.startsWith('file:') ? k.slice(5) : ''
}

/** items 可选列表（本地文件条目 ∩ 子序列过滤词命中——连续/非连续(hde→hardware)，键盘导航域）。 */
const items = computed(() => {
  const kw = q.value.trim()
  return recent.value.filter((x) => {
    const p = filePath(x)
    return p && isSubseq(p, kw)
  })
})
const navIdx = ref(-1)
// 过滤域变化 → sel 指向的行可能已不在视图,重置(K8sPanel 切 ns 同款)
watch(q, () => {
  navIdx.value = -1
})

/** navMove ctrl+j/k 上下（首次 j 落 0 / k 落尾，搜索历史下拉同款）。 */
function navMove(delta: number) {
  const n = items.value.length
  if (n === 0) return
  navIdx.value = Math.min(n - 1, Math.max(0, navIdx.value < 0 ? (delta > 0 ? 0 : n - 1) : navIdx.value + delta))
  document.querySelectorAll('.panel .list li')[navIdx.value]?.scrollIntoView({ block: 'nearest' })
}

async function open(k: string) {
  const p = filePath(k)
  if (!p) return
  try {
    await openRecent(k) // 分派正本（lib/recent.ts，Welcome 同源）
    recent.value = ((await usageRecent()) ?? []) // 刷新排序
  } catch {
    /* 打开失败静默（文件可能已移动） */
  }
}

async function pick() {
  try {
    await openFileViaDialog(true)
  } catch {
    /* 取消 */
  }
}

/** confirmSel 打开当前选中项(面板 onDocKey Enter)。 */
function confirmSel() {
  const k = items.value[navIdx.value]
  if (k !== undefined) void open(k)
}

/** onDocKey 面板键盘路由:capture 阶段消费即阻断——面板非 modal,App 全局 vim 键
 *  先注册先触发,不阻断会双动作;未消费键放行(仅余 Enter 无选中等少数态;可打印字母归打字过滤——r2 面板=打字态优先)。 */
function onDocKey(e: KeyboardEvent) {
  // 全局弹窗(BrowseDialog/设置等,登记 ui.dialogs 栈)打开期让位——面板 capture 先于
  // 弹窗 bubble 监听,不让位则弹窗内导航失效且面板键误动作(Final Review I1,对齐 App modalOpen)
  if (ui.browse.show || ui.modalOpen) return // browse 态是面板自设的可靠信号(BrowseDialog 单例未挂时 dialogs 栈无它,测试环境同款)
  const t = e.target as HTMLElement
  if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable)) return
  let hit = false
  if (e.ctrlKey && e.key === 'j') {
    navMove(1)
    hit = true
  } else if (e.ctrlKey && e.key === 'k') {
    navMove(-1)
    hit = true
  } else if (e.key === 'Enter' && navIdx.value >= 0) {
    confirmSel()
    hit = true
  } else if (e.ctrlKey && e.key === 'a') {
    // 新增统一:ctrl+a 呼系统文件框(裸 o 退役)
    void pick()
    hit = true
  } else if ((e.key === 'q' || e.key === '?') && !e.ctrlKey && !e.metaKey && !e.altKey) {
    // 元键放行:q 面板循环/? 帮助归 App 全局(r2 Final Review——打字态不吞元键,不消费不阻断)
    return
  } else if (typeKey(e)) {
    // r2 无框打字过滤:可打印字符进词/退格删尾/Esc 词非空清空
    hit = true
  }
  if (hit) {
    e.preventDefault()
    e.stopPropagation()
  }
}
</script>

<template>
  <div class="panel">
    <div class="head">
      <span>最近文件 <span class="kbd-hint">⌃j/k</span></span>
      <button class="panel-add" title="打开日志文件 (⌃a)" @click="pick">
        <svg class="ico" viewBox="0 0 24 24"><path d="M12 5v14M5 12h14" /></svg>
        <span class="kbd-hint">⌃a</span>
      </button>
    </div>
    <p v-if="q" class="filter-hint">⌕ {{ q }}</p>
    <ul class="list">
      <li
        v-for="(k, i) in items"
        :key="k"
        class="recent-item"
        :class="{ sel: i === navIdx }"
        @click="open(k)"
      >
        <svg class="ico" viewBox="0 0 24 24"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><path d="M14 2v6h6" /></svg>
        <!-- 命中字母高亮(segments 插值,不用 v-html) -->
        <span v-for="(s, si) in splitHighlight(filePath(k), q)" :key="si" :class="{ 'kw-hit': s.hit }">{{ s.t }}</span>
      </li>
    </ul>
    <p v-if="!items.length" class="empty">{{ q ? '无匹配文件' : '暂无记录，点右上角 + 选择日志文件' }}</p>
  </div>
</template>

<style scoped>
.panel {
  /* 宽随内容自适应(验收反馈:边框按文字长度调整);上限防超长路径撑爆,
     超长项 ellipsis+title 截断 */
  width: max-content;
  min-width: calc(var(--ui-font-size) * 21);
  max-width: min(480px, 42vw);
  padding: 8px;
  font-size: var(--font-ui-base, 12px);
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  color: var(--dim, #8b98a9);
}
.list {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 50vh;
  overflow-y: auto;
  font-family: var(--font-mono, monospace);
}
.list li {
  padding: 4px 6px;
  border-radius: 4px;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.list li:hover {
  background: var(--popup-bg, #1f2c3f);
}
.empty {
  color: var(--dim, #5d7292);
}
</style>
