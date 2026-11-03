<script setup lang="ts">
// FRP 面板：连接列表（frp.json v2 官方字段平铺）→ 点击进目录浏览；
// "浏览中"为前端本地标记（存在已开 tab 即活跃——后端无状态机，spec 决策）。
// 新增/删除/编辑（✎）走同一表单：字段=frp 官方名所见即 frpc 参数；
// stcp=拉 visitor 隧道（serverName+secretKey），tcp=frps remotePort 直连（零配置不拉 frpc）。
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { NButton, NInput, NInputNumber, NModal, NSelect } from 'naive-ui'
import { useTabsStore } from '../../stores/tabs'
import { useUiStore } from '../../stores/ui'
import { deleteFrpConn, getFrpConns, upsertFrpConn, type FrpConn } from '../../lib/backend'

const tabs = useTabsStore()
const ui = useUiStore()
const conns = ref<FrpConn[]>([])

async function reload() {
  try {
    conns.value = (await getFrpConns()) ?? []
  } catch {
    conns.value = []
  }
}
onMounted(reload)

/** activeConn 前端"浏览中"标记：该连接存在已开 tab（tab 标题可能是别名，双匹配）。 */
function isActive(c: FrpConn): boolean {
  return tabs.tabs.some(
    (t) => t.info.label.includes(c.name) || (c.alias && t.info.label.includes(c.alias))
  )
}
const anyConn = computed(() => conns.value.length > 0)

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
function onDocKey(e: KeyboardEvent) {
  if (e.key === 'Escape' && ui.topDialog === 'frp-form') {
    formShow.value = false
  }
}
onMounted(() => document.addEventListener('keydown', onDocKey))
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onDocKey)
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
      <span>FRP 连接</span>
      <button class="add conn-add" title="新增连接" @click="openForm()">
        <svg class="ico" viewBox="0 0 24 24"><path d="M12 5v14M5 12h14" /></svg>
      </button>
    </div>
    <ul class="list">
      <li v-for="c in conns" :key="c.name" class="conn-item" @click="open(c.name)">
        <span class="name"
          ><svg class="ico" viewBox="0 0 24 24"
            ><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" /><path
              d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" /></svg>
          {{ c.alias || c.name }}<span class="tag" :class="c.type">{{ c.type }}</span></span
        >
        <span class="meta">
          <span v-if="isActive(c)" class="live">浏览中</span>
          <span class="conn-edit" title="编辑连接" @click.stop="openForm(c)">
            <svg class="ico" viewBox="0 0 24 24"><path d="M17 3l4 4L7.5 20.5 2 22l1.5-5.5L17 3z" /></svg>
          </span>
          <span class="conn-del" title="删除连接" @click.stop="remove(c.name)">
            <svg class="ico" viewBox="0 0 24 24"><path d="M18 6L6 18M6 6l12 12" /></svg>
          </span>
        </span>
        <span class="summary">{{ connSummary(c) }}</span>
      </li>
    </ul>
    <p v-if="!anyConn" class="empty">无连接——点右上角新增（frp.json 共享正本）</p>

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
  width: 250px;
  padding: 8px;
  font-size: 12px;
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
.add {
  background: transparent;
  border: 1px solid var(--popup-border, #2a3a4f);
  border-radius: 4px;
  color: var(--dim, #8b98a9);
  cursor: pointer;
  padding: 1px 8px;
  font-size: 13px;
}
.add:hover {
  color: var(--accent, #4a8fe0);
  border-color: var(--accent, #4a8fe0);
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
  font-size: 11px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
/* 类型徽标：tcp 直连绿 / stcp 隧道蓝（与列表摘要呼应） */
.tag {
  margin-left: 4px;
  font-size: 10px;
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
  font-size: 11px;
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
  font-size: 12px;
}
.conn-form label {
  color: var(--dim, #8b98a9);
  font-size: 11px;
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
  font-size: 11px;
  margin: 4px 0 0;
}
</style>
