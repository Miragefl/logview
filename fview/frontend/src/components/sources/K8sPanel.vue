<script setup lang="ts">
// K8s 面板：context → namespace 两级下拉 + 资源列表，点资源直接开 tab（kubectl logs 复用）。
// 键盘化(2026-11-04):ctrl+j/k 选资源、Enter 开 tab;两级 NSelect 走 naive 原生 Tab 可达。
// 搜索+新增统一迭代:切 ctx/ns 键迁移 c/n→ctrl+c/ctrl+n(循环行为不变,裸键退役)+搜索过滤资源。
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { NSelect } from 'naive-ui'
import { getK8sContexts, getK8sNamespaces, getK8sResources, openK8sMergedTab, openK8sTab, usageTouch } from '../../lib/backend'
import { useUiStore } from '../../stores/ui'
import { isSubseq, splitHighlight } from '../../lib/highlight'
import { useTypeFilter } from './useTypeFilter'

const ui = useUiStore()

const ctx = ref<string | null>(null)
const ns = ref<string | null>(null)
const ctxOptions = ref<{ label: string; value: string }[]>([])
const nsOptions = ref<{ label: string; value: string }[]>([])
const resources = ref<{ kind: string; name: string }[]>([])

/** q 打字过滤词(r2 无框:直接敲字母进词,退格删尾,Esc 清空)。 */
const { q, typeKey } = useTypeFilter()
/** items 可选列表（资源 ∩ 子序列过滤词命中——kind/name 拼接域(与高亮一致，跨界
 * 如 d/p 亦命中)，连续/非连续(hde→hardware)，键盘导航域）。 */
const items = computed(() => {
  const kw = q.value.trim()
  return kw ? resources.value.filter((r) => isSubseq(r.kind + '/' + r.name, kw)) : resources.value
})
const navIdx = ref(-1)
/** marked 多选标记集合(key=kind/name)——Space toggle(TUI sourcepicker 同款),
 * Enter 合流开单 tab/⌘Enter 各开多 tab;切 ctx/ns 清,q 过滤不清(隐藏项保留)。 */
const marked = ref(new Set<string>())
// 过滤域变化 → sel 指向的行可能已不在视图,重置(切 ns 重置同款)
watch(q, () => {
  navIdx.value = -1
})

onMounted(async () => {
  // 监听必须先于任何 await 挂载——getK8sContexts 是 exec kubectl(四面板唯一慢源),
  // 挂在 await 后则慢源期间/hang 时键盘全失效(真机"ctrl+j/k 没用"根因)
  document.addEventListener('keydown', onDocKey, true)
  try {
    ctxOptions.value = ((await getK8sContexts()) ?? []).map((c) => ({ label: c, value: c }))
    if (ctxOptions.value.length) {
      ctx.value = ctxOptions.value[0].value
    }
  } catch {
    /* 无 kubeconfig：下拉为空 */
  }
})
onBeforeUnmount(() => document.removeEventListener('keydown', onDocKey, true))

watch(ctx, async (c) => {
  marked.value.clear() // 资源域切换,标记失效
  ns.value = null
  resources.value = []
  if (!c) return
  try {
    nsOptions.value = ((await getK8sNamespaces(c)) ?? []).map((n) => ({ label: n, value: n }))
    if (nsOptions.value.length) {
      ns.value = nsOptions.value[0].value
    }
  } catch {
    nsOptions.value = []
  }
})

watch(ns, async (n) => {
  resources.value = []
  navIdx.value = -1 // 资源域切换,选中态重置
  marked.value.clear() // 同上,标记失效
  if (!n || !ctx.value) return
  try {
    const rs = (await getK8sResources(ctx.value, n)) ?? []
    resources.value = rs.map((r) => ({ kind: r.kind, name: r.name }))
  } catch {
    resources.value = []
  }
})

async function open(r: { kind: string; name: string }) {
  if (!ctx.value || !ns.value) return
  try {
    await openK8sTab(`${r.kind}/${r.name}`, ns.value, ctx.value, 0)
    usageTouch(`k8sres:${ctx.value}/${ns.value}/${r.kind}/${r.name}`)
  } catch {
    /* 打开失败静默 */
  }
}

/** openByKey kind/name 键开单资源 tab(⌘Enter 各开复用,记频同 open)。 */
async function openByKey(key: string) {
  const r = resources.value.find((x) => x.kind + '/' + x.name === key)
  if (r) await open(r)
}

/** mergeErr 合流失败提示(面板内 dim 红显示;重试成功清除)。 */
const mergeErr = ref('')

/** openMerged 多资源合流开单 tab(stern 式:dp/sts 展开副本 pod 并行合流,行来源=pod 名)。
 * keys 字典序(TUI sortStrings 对齐);单标记走单开(TUI 同构);成功清标记(面板常驻,
 * 残留标记会劫持后续 Enter 单开),失败保留可调整+面板内提示(N 选 1 错全错不能静默)。 */
async function openMerged(keys: string[]) {
  if (!ctx.value || !ns.value) return
  mergeErr.value = ''
  if (keys.length === 1) {
    await openByKey(keys[0])
    marked.value.clear()
    return
  }
  try {
    await openK8sMergedTab([...keys].sort(), ns.value, ctx.value, 0)
    for (const k of keys) usageTouch(`k8sres:${ctx.value}/${ns.value}/${k}`)
    marked.value.clear()
  } catch (e) {
    mergeErr.value = `合流失败: ${[...keys].sort().join(', ')}${e instanceof Error ? ` (${e.message})` : ''}`
  }
}

/** navMove ctrl+j/k 上下(首次 j 落 0 / k 落尾,搜索历史下拉同款)。 */
function navMove(delta: number) {
  const n = items.value.length
  if (n === 0) return
  navIdx.value = Math.min(n - 1, Math.max(0, navIdx.value < 0 ? (delta > 0 ? 0 : n - 1) : navIdx.value + delta))
  document.querySelectorAll('.panel .list li')[navIdx.value]?.scrollIntoView({ block: 'nearest' })
}

/** confirmSel 开当前选中资源的 tab(面板 onDocKey Enter)。 */
function confirmSel() {
  const r = items.value[navIdx.value]
  if (r) void open(r)
}

/** onDocKey 面板键盘路由:capture 消费即阻断(防 App 全局 vim 双动作;
 * n 在 App 是 nextHit,K8s 面板语境优先——要 nextHit 先 q 收面板);
 * NSelect 聚焦/展开态经 closest 守卫放行(下拉内键归 naive,见函数内注释)。 */
function onDocKey(e: KeyboardEvent) {
  // 全局弹窗(BrowseDialog/设置等)打开期让位(Final Review I1,对齐 App modalOpen)
  if (ui.browse.show || ui.modalOpen) return // browse 态是面板自设的可靠信号(BrowseDialog 单例未挂时 dialogs 栈无它,测试环境同款)
  const t = e.target as HTMLElement
  if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable)) return
  // NSelect 聚焦/展开态放行:非 filterable NSelect 焦点元素是 div.n-base-selection 非 input
  // (INPUT 判定不命中),方向键+Enter 是 naive 原生 Tab 可达路径——面板不得抢跑(Final Review I2)
  if (t && t.closest?.('.n-base-selection, .n-base-select-menu')) return
  let hit = false
  if (e.ctrlKey && e.key === 'j') {
    navMove(1)
    hit = true
  } else if (e.ctrlKey && e.key === 'k') {
    navMove(-1)
    hit = true
  } else if (e.metaKey && e.key === 'Enter' && marked.value.size > 0) {
    // ⌘Enter:每个标记项各开一 tab(多 tab 对照)——串行开保 tab 创建序=标记字典序,
    // 开完清标记(Final Review M2/M3)
    const keys = [...marked.value].sort()
    marked.value.clear()
    void (async () => {
      for (const key of keys) await openByKey(key)
    })()
    hit = true
  } else if (e.key === 'Enter' && (marked.value.size > 0 || navIdx.value >= 0)) {
    // Enter:有标记=合流开单 tab(多 pod 同看),无标记=开选中单个(现状)
    if (marked.value.size > 0) {
      void openMerged([...marked.value])
    } else {
      confirmSel()
    }
    hit = true
  } else if (e.key === ' ' && !e.ctrlKey && !e.metaKey && !e.altKey) {
    // Space 多选标记(TUI sourcepicker 同款;K8s 资源名无空格,与打字过滤词不冲突);
    // 无条件消费——空格不进词(K8s 面板语义)
    const r = items.value[navIdx.value]
    if (r) {
      const key = r.kind + '/' + r.name
      if (marked.value.has(key)) marked.value.delete(key)
      else marked.value.add(key)
    }
    hit = true
  } else if (e.ctrlKey && e.key === 'c') {
    // 循环下一个 context(尾回首)——watch 链自动重载 ns/资源(裸 c 退役,纯键位迁移);
    // opts 空(无 kubeconfig)也消费——⌃d/⌃e 无条件消费同款,防穿透 App ⌃c 复制(Final Review)
    const opts = ctxOptions.value
    if (opts.length) {
      const i = opts.findIndex((o) => o.value === ctx.value)
      ctx.value = opts[(i + 1) % opts.length].value
    }
    hit = true
  } else if (e.ctrlKey && e.key === 'n') {
    // 循环下一个 namespace——watch 重载资源+选中态重置;opts 空也消费(同上防穿透)
    const opts = nsOptions.value
    if (opts.length) {
      const i = opts.findIndex((o) => o.value === ns.value)
      ns.value = opts[(i + 1) % opts.length].value
    }
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
      <span>Kubernetes <span class="kbd-hint">⌃j/k 资源 · ⌃c/⌃n 切换 · Space 多选</span></span>
    </div>
    <div class="sel-row">
      <NSelect v-model:value="ctx" size="tiny" placeholder="context (⌃c)" :options="ctxOptions" />
      <NSelect v-model:value="ns" size="tiny" placeholder="namespace (⌃n)" :options="nsOptions" />
    </div>
    <p v-if="q" class="filter-hint">⌕ {{ q }}</p>
    <ul class="list">
      <li
        v-for="(r, i) in items"
        :key="r.kind + '/' + r.name"
        class="res-item"
        :class="{ sel: i === navIdx }"
        @click="open(r)"
      >
        <!-- 命中字母高亮(segments 插值,不用 v-html;kind/name 拼接串统一切分);✓ 多选标记 -->
        🟢 <span v-if="marked.has(r.kind + '/' + r.name)" class="mark">✓</span> <span v-for="(s, si) in splitHighlight(r.kind + '/' + r.name, q)" :key="si" :class="{ 'kw-hit': s.hit }">{{ s.t }}</span>
      </li>
    </ul>
    <p v-if="!items.length" class="empty">{{ q ? '无匹配资源' : ctx ? '该 namespace 暂无工作负载' : '未发现 kubeconfig' }}</p>
    <p v-if="mergeErr" class="err">{{ mergeErr }}</p>
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
  color: var(--dim, #8b98a9);
}
/* 上下堆叠占满面板宽：并排 115px 会截断长 namespace（验收反馈①） */
.sel-row {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.list {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 44vh;
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
/* 合流失败提示(Final Review I2:N 选 1 错全错不能静默) */
.err {
  color: var(--level-error, #e57373);
  margin: 0;
}
/* Space 多选标记 ✓ */
.mark {
  color: var(--accent, #7dc4e4);
  font-weight: 600;
}
</style>
