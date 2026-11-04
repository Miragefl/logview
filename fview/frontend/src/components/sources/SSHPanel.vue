<script setup lang="ts">
// SSH 面板：~/.ssh/config 自动发现（cfg 角标，删不掉属预期）+ 手动增删 → 点击进目录浏览。
// 键盘化(2026-11-04):ctrl+j/k 选主机、Enter 进浏览、d 删除选中(cfg 跳过)。
// 搜索+新增统一迭代:内联添加框改 NModal 弹窗(右上角 +/ctrl+a,FRP 同款 topDialog 层级)，
// 裸键 a 退役;新增搜索过滤列表。
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { NButton, NInput, NModal } from 'naive-ui'
import { addSSHHost, getSSHHosts, removeSSHHost, type SSHHost } from '../../lib/backend'
import { useUiStore } from '../../stores/ui'
import { isSubseq, splitHighlight } from '../../lib/highlight'
import { useTypeFilter } from './useTypeFilter'

const ui = useUiStore()
const hosts = ref<SSHHost[]>([])
/** q 打字过滤词(r2 无框:直接敲字母进词,退格删尾,Esc 清空)。 */
const { q, typeKey } = useTypeFilter()
/** items 可选列表（主机 ∩ 子序列过滤词命中——连续/非连续(hde→hardware)，键盘导航域）。 */
const items = computed(() => {
  const kw = q.value.trim()
  return kw ? hosts.value.filter((h) => isSubseq(h.name, kw)) : hosts.value
})
const navIdx = ref(-1)
// 过滤域变化 → sel 指向的行可能已不在视图,重置(K8sPanel 切 ns 同款)
watch(q, () => {
  navIdx.value = -1
})

/** formShow 添加弹窗开关;formHost 弹窗内主机串(user@host[:port])。 */
const formShow = ref(false)
const formHost = ref('')
// 弹窗登记进 ui.dialogs 栈(键盘层让位/ESC 逐层关闭;FRP 表单同款机制)
watch(
  () => formShow.value,
  (v) => (v ? ui.dialogOpen('ssh-form') : ui.dialogClose('ssh-form'))
)

async function reload() {
  try {
    hosts.value = (await getSSHHosts()) ?? []
  } catch {
    hosts.value = []
  }
}
onMounted(() => {
  void reload()
  document.addEventListener('keydown', onDocKey, true)
})
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onDocKey, true)
  ui.dialogClose('ssh-form')
})

/** openForm 打开添加弹窗并清空上次残留。 */
function openForm() {
  formHost.value = ''
  formShow.value = true
}

/** saveForm 提交添加:成功才关弹窗(无效 host 静默留在弹窗内可改)。Enter/按钮双入口。 */
async function saveForm() {
  const h = formHost.value.trim()
  if (!h) return
  try {
    await addSSHHost(h)
    formShow.value = false
    await reload()
  } catch {
    /* 无效 host 静默 */
  }
}

async function remove(h: string) {
  try {
    await removeSSHHost(h)
    await reload()
  } catch {
    /* */
  }
}

function open(h: string) {
  // 全局浏览弹窗（App.vue 单例，欢迎页最近源同入口）
  ui.openBrowse({ mode: 'ssh', host: h })
}

/** navMove ctrl+j/k 上下（首次 j 落 0 / k 落尾,搜索历史下拉同款）。 */
function navMove(delta: number) {
  const n = items.value.length
  if (n === 0) return
  navIdx.value = Math.min(n - 1, Math.max(0, navIdx.value < 0 ? (delta > 0 ? 0 : n - 1) : navIdx.value + delta))
  document.querySelectorAll('.panel .list li')[navIdx.value]?.scrollIntoView({ block: 'nearest' })
}

/** confirmSel 进入选中主机的目录浏览(面板 onDocKey Enter)。 */
function confirmSel() {
  const h = items.value[navIdx.value]
  if (h) open(h.name)
}

/** onDocKey 面板键盘路由:capture 消费即阻断(App 全局 vim 先注册先触发,d/a 在
 *  App 是 detail/level——不阻断双动作);弹窗期列表键全让位只留 Esc 兜底
 *  (焦点在 body 时 naive esc 失灵,topDialog 条件防跨层误关——FRP 同款);
 *  INPUT 聚焦放行(防御保留——无框后仅弹窗内输入命中)。 */
function onDocKey(e: KeyboardEvent) {
  if (formShow.value) {
    if (e.key === 'Escape' && ui.topDialog === 'ssh-form') {
      formShow.value = false
      e.preventDefault()
      e.stopPropagation()
    }
    return
  }
  // 其他全局弹窗(BrowseDialog/设置等)打开期让位——面板 capture 先于弹窗 bubble 监听,
  // 不让位则弹窗内导航失效且面板键误动作(Final Review I1;自家表单分支在前——Esc 兜底依赖)
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
    // 新增统一:ctrl+a 开添加弹窗(裸 a 退役)
    openForm()
    hit = true
  } else if (e.ctrlKey && e.key === 'd') {
    // ⌃d 删除选中(原裸 d 迁移——r2 打字过滤占用字母域;cfg 主机 ~/.ssh/config 来源,
    // 删了也会被重新发现——跳过);删后 navIdx 落到顶上来的下一项(clamp),sel 焦点不消失。
    // 无条件消费(不可动作也不放行,防穿透 App 全局键——r1 Final M2 顺手修)
    const h = items.value[navIdx.value]
    if (h && !h.fromSshConfig) {
      void remove(h.name)
      navIdx.value = Math.min(navIdx.value, items.value.length - 2)
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
      <span>SSH 主机 <span class="kbd-hint">⌃j/k</span></span>
      <button class="panel-add" title="添加主机 (⌃a)" @click="openForm">
        <svg class="ico" viewBox="0 0 24 24"><path d="M12 5v14M5 12h14" /></svg>
        <span class="kbd-hint">⌃a</span>
      </button>
    </div>
    <p v-if="q" class="filter-hint">⌕ {{ q }}</p>
    <ul class="list">
      <li
        v-for="(h, i) in items"
        :key="h.name"
        class="host-item"
        :class="{ sel: i === navIdx }"
        :title="h.name"
        @click="open(h.name)"
      >
        <span class="name">
          <svg class="ico" viewBox="0 0 24 24"><path d="M2 3h20v14H2z" /><path d="M8 21h8M12 17v4" /></svg>
          <!-- 命中字母高亮(segments 插值,不用 v-html) -->
          <span v-for="(s, si) in splitHighlight(h.name, q)" :key="si" :class="{ 'kw-hit': s.hit }">{{ s.t }}</span><span v-if="h.fromSshConfig" class="cfg" title="~/.ssh/config 自动发现">cfg</span>
        </span>
        <!-- cfg 主机来自 ~/.ssh/config，删了也会被重新发现——不提供 ×，只留手动添加的 -->
        <span v-if="!h.fromSshConfig" class="del" title="移除 (⌃d)" @click.stop="remove(h.name)">
          <svg class="ico" viewBox="0 0 24 24"><path d="M18 6L6 18M6 6l12 12" /></svg>
          <span class="kbd-hint">(⌃d)</span>
        </span>
      </li>
    </ul>
    <p v-if="!items.length" class="empty">{{ q ? '无匹配主机' : '无主机——点右上角 + 添加（或检查 ~/.ssh/config）' }}</p>

    <NModal v-model:show="formShow" preset="dialog" title="新增 SSH 主机" style="width: 360px">
      <div class="ssh-form">
        <NInput
          v-model:value="formHost"
          size="small"
          placeholder="user@host[:port]"
          @keydown.enter="saveForm"
        />
      </div>
      <template #action>
        <NButton size="small" @click="formShow = false">取消</NButton>
        <NButton size="small" type="primary" class="ssh-save" @click="saveForm">添加</NButton>
      </template>
    </NModal>
  </div>
</template>

<style scoped>
.panel {
  /* 宽随内容自适应(验收反馈:边框按文字长度调整);上限防超长主机名撑爆,
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
  max-height: 44vh;
  overflow-y: auto;
  font-family: var(--font-mono, monospace);
}
.list li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 6px;
  border-radius: 4px;
  cursor: pointer;
}
.list li:hover {
  background: var(--popup-bg, #1f2c3f);
}
.name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.del {
  color: var(--dim, #8b98a9);
  padding: 0 4px;
}
/* cfg 角标：~/.ssh/config 自动发现的主机 */
.cfg {
  margin-left: 4px;
  font-size: var(--font-ui-xs, 10px);
  color: var(--dim, #5d7292);
  border: 1px solid var(--popup-border, #2a3a4f);
  border-radius: 3px;
  padding: 0 3px;
}
.del:hover {
  color: var(--level-error, #e57373);
}
.empty {
  color: var(--dim, #5d7292);
}
.ssh-form {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
</style>
