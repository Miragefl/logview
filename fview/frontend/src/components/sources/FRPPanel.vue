<script setup lang="ts">
// FRP 面板：连接列表（frp.json v2 官方字段平铺）→ 点击进目录浏览；
// "浏览中"为前端本地标记（存在已开 tab 即活跃——后端无状态机，spec 决策）。
// 新增/删除/编辑（✎）走同一表单：字段=frp 官方名所见即 frpc 参数；
// stcp=拉 visitor 隧道（serverName+secretKey），tcp=frps remotePort 直连（零配置不拉 frpc）。
// 搜索+新增统一迭代(2026-11-04):新增表单键 n→ctrl+a(三面板统一,裸 n 退役)+搜索过滤(name/alias)。
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { NButton, NInput, NInputNumber, NModal, NSelect } from 'naive-ui'
import { useTabsStore } from '../../stores/tabs'
import { useUiStore } from '../../stores/ui'
import { deleteFrpConn, getFrpConns, upsertFrpConn, type FrpConn } from '../../lib/backend'
import { isSubseq, splitHighlight } from '../../lib/highlight'
import { useTypeFilter } from './useTypeFilter'

const tabs = useTabsStore()
const ui = useUiStore()
const conns = ref<FrpConn[]>([])

/** q 打字过滤词(r2 无框:直接敲字母进词,退格删尾,Esc 清空)。 */
const { q, typeKey } = useTypeFilter()
/** items 可选列表（连接 ∩ 子序列过滤词命中——name/alias 双域，连续/非连续(hde→hardware)，键盘导航域）。 */
const items = computed(() => {
  const kw = q.value.trim()
  return kw
    ? conns.value.filter((c) => isSubseq(c.name, kw) || isSubseq(c.alias || '', kw))
    : conns.value
})
const navIdx = ref(-1)
// 过滤域变化 → sel 指向的行可能已不在视图,重置(K8sPanel 切 ns 同款)
watch(q, () => {
  navIdx.value = -1
})

async function reload() {
  try {
    conns.value = (await getFrpConns()) ?? []
  } catch {
    conns.value = []
  }
}
onMounted(() => {
  void reload()
  document.addEventListener('keydown', onDocKey, true)
})

/** activeConn 前端"浏览中"标记：该连接存在已开 tab（tab 标题可能是别名，双匹配）。 */
function isActive(c: FrpConn): boolean {
  return tabs.tabs.some(
    (t) => t.info.label.includes(c.name) || (c.alias && t.info.label.includes(c.alias))
  )
}
/** connSummary 列表摘要：类型对应的关键参数（与 frpc 语义一致）。 */
function connSummary(c: FrpConn): string {
  return c.type === 'tcp' ? `${c.serverAddr}:${c.remotePort}` : `${c.serverName}@${c.serverAddr}`
}

function open(name: string) {
  // 全局浏览弹窗（App.vue 单例，欢迎页最近源同入口）
  ui.openBrowse({ mode: 'frp', connName: name })
}

async function remove(name: string) {
  try {
    await deleteFrpConn(name)
    await reload()
  } catch {
    /* */
  }
}

// ---- 连接表单（新增 / 编辑共用，官方字段）----
const formShow = ref(false)
const form = ref({
  name: '',
  alias: '',
  type: 'stcp',
  serverAddr: '',
  serverPort: 7000,
  token: '',
  serverName: '',
  secretKey: '',
  remotePort: 6000,
  user: '',
})
const typeOpts = [
  { label: 'stcp（加密点对点，需 serverName+secretKey）', value: 'stcp' },
  { label: 'tcp（frps 端口直连，需 remotePort）', value: 'tcp' },
]
const saving = ref(false)
const formErr = ref('')
// 编辑中的原始连接（null = 新增模式）；改名以它为旧名删除依据
const editing = ref<FrpConn | null>(null)
const isTCP = computed(() => form.value.type === 'tcp')

// 表单弹窗进栈(全局键让位;第三轮验收②)
watch(
  () => formShow.value,
  (v) => (v ? ui.dialogOpen('frp-form') : ui.dialogClose('frp-form')),
  { immediate: true }
)
// ESC 兜底关闭:naive 的 esc 依赖焦点管理,焦点在 body 时关不掉(点列表项
// 开表单即此态)——document 级监听才可靠,SettingsDialog 同款(第三轮验收②);
// topDialog 条件防跨层误关(表单上还压着别的弹窗时让位)。
// 键盘化(2026-11-04):capture 阶段消费即阻断(App 全局 vim 先注册先触发,
// e/d/n 在 App 是 fold/detail/nextHit——不阻断双动作);表单开着列表键全让位
// 只留 Esc;未消费键放行(仅余少数非打印态;字母归打字过滤)。挂载在 onMounted(reload) 处。

/** navMove ctrl+j/k 上下(首次 j 落 0 / k 落尾,搜索历史下拉同款)。 */
function navMove(delta: number) {
  const n = items.value.length
  if (n === 0) return
  navIdx.value = Math.min(n - 1, Math.max(0, navIdx.value < 0 ? (delta > 0 ? 0 : n - 1) : navIdx.value + delta))
  document.querySelectorAll('.panel .list li')[navIdx.value]?.scrollIntoView({ block: 'nearest' })
}

/** confirmSel 进入选中连接的目录浏览(面板 onDocKey Enter)。 */
function confirmSel() {
  const c = items.value[navIdx.value]
  if (c) open(c.name)
}

function onDocKey(e: KeyboardEvent) {
  if (formShow.value) {
    if (e.key === 'Escape' && ui.topDialog === 'frp-form') {
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
    // 新增统一:ctrl+a 开新建表单(裸 n 退役)
    openForm()
    hit = true
  } else if (e.ctrlKey && e.key === 'e') {
    // ⌃e 编辑选中(原裸 e 迁移——r2 打字过滤占用字母域);无条件消费防穿透 App 全局
    const c = items.value[navIdx.value]
    if (c) openForm(c)
    hit = true
  } else if (e.ctrlKey && e.key === 'd') {
    // ⌃d 删除选中(原裸 d 迁移);删除后 navIdx 落到顶上来的下一项(clamp),sel 焦点不消失;
    // 无条件消费(不可动作也不放行,防穿透——r1 Final M2 顺手修)
    const c = items.value[navIdx.value]
    if (c) {
      void remove(c.name)
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
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onDocKey, true)
  ui.dialogClose('frp-form')
})

/** openForm 打开表单：无参=新增；传连接=编辑（全字段预填，path 透传）。 */
function openForm(edit?: FrpConn) {
  editing.value = edit ?? null
  form.value = edit
    ? {
        name: edit.name,
        alias: edit.alias,
        type: edit.type || 'stcp',
        serverAddr: edit.serverAddr,
        serverPort: edit.serverPort || 7000,
        token: edit.token,
        serverName: edit.serverName,
        secretKey: edit.secretKey,
        remotePort: edit.remotePort || 6000,
        user: edit.user,
      }
    : { name: '', alias: '', type: 'stcp', serverAddr: '', serverPort: 7000, token: '', serverName: '', secretKey: '', remotePort: 6000, user: '' }
  formErr.value = ''
  formShow.value = true
}

async function saveForm() {
  const f = form.value
  if (!f.name.trim() || !f.serverAddr.trim()) {
    formErr.value = '连接名与 serverAddr 为必填'
    return
  }
  if (isTCP.value && !f.remotePort) {
    formErr.value = 'tcp 连接需填 remotePort（frps 暴露端口）'
    return
  }
  if (!isTCP.value && (!f.serverName.trim() || !f.secretKey)) {
    formErr.value = 'stcp 连接需填 serverName 与 secretKey'
    return
  }
  const newName = f.name.trim()
  // 编辑改名撞已有连接名必须拦下——upsert 是静默覆盖，不拦会吃掉别人的配置
  if (editing.value && editing.value.name !== newName && conns.value.some((c) => c.name === newName)) {
    formErr.value = `连接名「${newName}」已存在`
    return
  }
  saving.value = true
  try {
    await upsertFrpConn({
      name: newName,
      alias: f.alias.trim(),
      type: f.type,
      serverAddr: f.serverAddr.trim(),
      serverPort: f.serverPort || 7000,
      token: f.token,
      serverName: isTCP.value ? '' : f.serverName.trim(),
      secretKey: isTCP.value ? '' : f.secretKey,
      remotePort: isTCP.value ? f.remotePort : 0,
      user: f.user.trim(),
      // 编辑透传原 path（直达 tail 路径不能被表单清掉）
      path: editing.value?.path ?? '',
    })
    // 改名 = 新名落库后再删旧名（upsert 不会清旧记录；先删后插失败会丢数据，顺序不能反）
    if (editing.value && editing.value.name !== newName) {
      await deleteFrpConn(editing.value.name)
    }
    formShow.value = false
    await reload()
  } catch (e) {
    formErr.value = String(e).replace(/^Error:\s*/, '')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="panel">
    <div class="head">
      <span>FRP 连接 <span class="kbd-hint">⌃j/k</span></span>
      <button class="panel-add conn-add" title="新增连接 (⌃a)" @click="openForm()">
        <svg class="ico" viewBox="0 0 24 24"><path d="M12 5v14M5 12h14" /></svg>
        <span class="kbd-hint">⌃a</span>
      </button>
    </div>
    <p v-if="q" class="filter-hint">⌕ {{ q }}</p>
    <ul class="list">
      <li
        v-for="(c, i) in items"
        :key="c.name"
        class="conn-item"
        :class="{ sel: i === navIdx }"
        @click="open(c.name)"
      >
        <span class="name"
          ><svg class="ico" viewBox="0 0 24 24"
            ><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" /><path
              d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" /></svg>
          <!-- 命中字母高亮(segments 插值,不用 v-html;显示什么高亮什么——name 命中而显示别名时不高亮) -->
          <span v-for="(s, si) in splitHighlight(c.alias || c.name, q)" :key="si" :class="{ 'kw-hit': s.hit }">{{ s.t }}</span><span class="tag" :class="c.type">{{ c.type }}</span></span>
        <span class="meta">
          <span v-if="isActive(c)" class="live">浏览中</span>
          <span class="conn-edit" title="编辑连接 (⌃e)" @click.stop="openForm(c)">
            <svg class="ico" viewBox="0 0 24 24"><path d="M17 3l4 4L7.5 20.5 2 22l1.5-5.5L17 3z" /></svg>
            <span class="kbd-hint">(⌃e)</span>
          </span>
          <span class="conn-del" title="删除连接 (⌃d)" @click.stop="remove(c.name)">
            <svg class="ico" viewBox="0 0 24 24"><path d="M18 6L6 18M6 6l12 12" /></svg>
            <span class="kbd-hint">(⌃d)</span>
          </span>
        </span>
        <span class="summary">{{ connSummary(c) }}</span>
      </li>
    </ul>
    <p v-if="!items.length" class="empty">{{ q ? '无匹配连接' : '无连接——点右上角新增（frp.json 共享正本）' }}</p>

    <NModal
      v-model:show="formShow"
      preset="dialog"
      :title="editing ? '编辑 FRP 连接' : '新增 FRP 连接'"
      style="width: 440px"
    >
      <div class="conn-form">
        <label>name *（记录名，仅显示用不进 frpc）</label>
        <NInput v-model:value="form.name" size="small" placeholder="如 home-nas" />
        <label>别名（可选，列表展示优先用它）</label>
        <NInput v-model:value="form.alias" size="small" placeholder="alias" />
        <label>type *</label>
        <NSelect v-model:value="form.type" size="small" :options="typeOpts" />
        <label>serverAddr * / serverPort</label>
        <div class="row2">
          <NInput v-model:value="form.serverAddr" size="small" placeholder="frps.example.com" />
          <NInputNumber v-model:value="form.serverPort" size="small" :show-button="false" placeholder="7000" />
        </div>
        <label>token（frps 带鉴权时填）</label>
        <NInput v-model:value="form.token" size="small" show-password-on="click" type="password" placeholder="auth token" />
        <template v-if="isTCP">
          <label>remotePort *（frps 上暴露的端口）</label>
          <NInputNumber v-model:value="form.remotePort" size="small" :show-button="false" placeholder="6000" />
        </template>
        <template v-else>
          <label>serverName *（远端 proxies name）</label>
          <NInput v-model:value="form.serverName" size="small" placeholder="如 secret_ssh" />
          <label>secretKey *（与远端 stcp 一致）</label>
          <NInput v-model:value="form.secretKey" size="small" show-password-on="click" type="password" placeholder="secretKey" />
        </template>
        <label>user（SSH 用户，空默认 root）</label>
        <NInput v-model:value="form.user" size="small" placeholder="root" />
        <p v-if="formErr" class="err">{{ formErr }}</p>
      </div>
      <template #action>
        <NButton size="small" @click="formShow = false">取消</NButton>
        <NButton size="small" type="primary" class="conn-save" :loading="saving" @click="saveForm">
          保存
        </NButton>
      </template>
    </NModal>

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
  max-height: 44vh;
  overflow-y: auto;
  font-family: var(--font-mono, monospace);
}
.list li {
  display: grid;
  grid-template-columns: 1fr auto;
  align-items: center;
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
.summary {
  grid-column: 1 / 3;
  color: var(--dim, #5d7292);
  font-size: var(--font-ui-sm, 11px);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
/* 类型徽标：tcp 直连绿 / stcp 隧道蓝（与列表摘要呼应） */
.tag {
  margin-left: 4px;
  font-size: var(--font-ui-xs, 10px);
  border-radius: 3px;
  padding: 0 3px;
  border: 1px solid var(--popup-border, #2a3a4f);
  color: var(--dim, #5d7292);
}
.tag.tcp {
  color: var(--level-info, #7ee29a);
}
.tag.stcp {
  color: var(--accent, #4a8fe0);
}
.meta {
  display: flex;
  align-items: center;
  gap: 6px;
}
.live {
  color: var(--level-info, #7ee29a);
  font-size: var(--font-ui-sm, 11px);
}
/* 行内图标按钮:svg 走全局 .ico(16px 描边 1.7,style.css 正本),
   弃字符字形(✎/× 字形只占字号六成,各字体渲染不一,两轮"还是小"的根因) */
.conn-edit,
.conn-del {
  display: inline-flex;
  color: var(--dim, #8b98a9);
  padding: 0 4px;
  cursor: pointer;
}
.conn-edit:hover {
  color: var(--accent, #4a8fe0);
}
.conn-del:hover {
  color: var(--level-error, #e57373);
}
.conn-del:hover {
  color: var(--level-error, #e57373);
}
.empty {
  color: var(--dim, #5d7292);
}
.conn-form {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: var(--font-ui-base, 12px);
}
.conn-form label {
  color: var(--dim, #8b98a9);
  font-size: var(--font-ui-sm, 11px);
  margin-top: 4px;
}
.row2 {
  display: flex;
  gap: 6px;
}
.row2 .n-input-number {
  width: 110px;
}
.err {
  color: var(--level-error, #e57373);
  font-size: var(--font-ui-sm, 11px);
  margin: 4px 0 0;
}
</style>
