<script setup lang="ts">
// 远程目录浏览对话框（SSH/FRP 共用）：进入目录、选文件开 tab、密码认证态重试。
// 键盘流对齐 TUI 弹窗:ctrl+j/k 上下、Enter 进目录/选文件、Backspace 回上级、Esc 关闭(第三轮验收④)。
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { NInput, NModal } from 'naive-ui'
import { useUiStore } from '../../stores/ui'
import {
  browseFRP,
  getSavedPass,
  openFRPTab,
  openSSHTab,
  releaseFRPBrowse,
  savePass,
  sshBrowse,
  usageTouch,
  type DirEntry,
} from '../../lib/backend'

/** filterEl 过滤框引用：弹窗打开后自动聚焦——直接打字即过滤，无需先点击（验收反馈①）。 */
const filterEl = ref<InstanceType<typeof NInput> | null>(null)

const props = defineProps<{
  show: boolean
  /** mode ssh：host 直连；frp：connName 走 stcp 隧道 */
  mode: 'ssh' | 'frp'
  host?: string
  connName?: string
  /** initPath 目标文件全路径：打开时落父目录并选中该文件（最近源直达用） */
  initPath?: string
}>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void; (e: 'opened'): void }>()

const ui = useUiStore()

const path = ref('/')
const entries = ref<DirEntry[]>([])
const password = ref('')
const err = ref('')
const needPwd = ref(false)
const loading = ref(false)
/** cursor 键盘选中索引 + itemEls 列表项 DOM(scrollIntoView 跟随)。 */
const cursor = ref(0)
const itemEls = ref<HTMLElement[]>([])
/** filter 过滤片段（子串、不区分大小写）；空=全量。shown 为导航/激活的作用集。 */
const filter = ref('')
const shown = computed(() => {
  const f = filter.value.trim().toLowerCase()
  if (!f) return entries.value
  return entries.value.filter((e) => e.name.toLowerCase().includes(f))
})

/** onFilterEnter 过滤框回车：/ 开头视为路径直达，否则激活当前选中项（空集天然 no-op）。 */
function onFilterEnter() {
  const f = filter.value.trim()
  if (f.startsWith('/')) {
    void browse(f)
    return
  }
  activate(cursor.value)
}

/** passKey 密码仓 key：ssh:{host} / frp:{connName}（lib/recent.ts openRecent 同式）。 */
const passKey = computed(() => (props.mode === 'ssh' ? `ssh:${props.host ?? ''}` : `frp:${props.connName ?? ''}`))

/** browse 拉取当前路径目录（密码态透传；失败提示密码/错误）。 */
async function browse(p: string) {
  loading.value = true
  err.value = ''
  try {
    if (props.mode === 'ssh') {
      entries.value = (await sshBrowse(props.host ?? '', p, password.value)) ?? []
    } else {
      entries.value = (await browseFRP(props.connName ?? '', p, password.value)) ?? []
    }
    path.value = p
    needPwd.value = false
    cursor.value = 0
    filter.value = ''
    nextTick(() => itemEls.value[0]?.scrollIntoView({ block: 'nearest' }))
    // 认证成功回存钥匙串（幂等覆盖；空密码后端 guard 不入库）
    if (password.value) void savePass(passKey.value, password.value)
  } catch (e) {
    const msg = String(e)
    needPwd.value = /password|密码|auth/i.test(msg)
    err.value = msg.replace(/^Error:\s*/, '')
  } finally {
    loading.value = false
  }
}

// 对话框每次打开：预填 Keychain 密码（存过免输直查）→ initPath 落父目录选中目标文件，
// 否则根目录拉起（常驻挂载，不能依赖 onMounted）。openGen 代际守卫：快速关开换源时
// 迟到的旧预填链不得覆盖新源状态（Final Review I3）
let openGen = 0
watch(
  () => props.show,
  (v) => {
    if (v) {
      entries.value = []
      filter.value = ''
      const gen = ++openGen
      void (async () => {
        const pass = (await getSavedPass(passKey.value)) ?? ''
        if (gen !== openGen) return
        password.value = pass
        const init = props.initPath ? props.initPath.replace(/\/[^/]*$/, '') || '/' : '/'
        path.value = init
        await browse(init)
        if (gen !== openGen) return
        if (props.initPath) {
          const base = props.initPath.replace(/.*\//, '')
          cursor.value = Math.max(0, entries.value.findIndex((e) => e.name === base))
        }
        // 打开即聚焦过滤框（needPwd 态过滤框不渲染，focus 空调用无害）
        nextTick(() => filterEl.value?.focus())
      })()
    }
  },
  { immediate: true }
)

/** enter 进入子目录（后端返回的 name 即相对当前路径的目录名）。 */
async function enter(name: string) {
  const base = path.value === '/' ? '' : path.value
  await browse(`${base}/${name}`)
}

/** goUp 回退上级目录(根目录不动)。 */
function goUp() {
  if (path.value === '/' || loading.value) return
  const up = path.value.replace(/\/[^/]*$/, '') || '/'
  browse(up)
}

/** pickFile 选日志文件开 tab（尾读 200 行 + follow）。 */
async function pickFile(name: string) {
  const base = path.value === '/' ? '' : path.value
  const full = `${base}/${name}`
  try {
    if (props.mode === 'ssh') {
      await openSSHTab(props.host ?? '', full, 0, password.value)
      usageTouch(`ssh:${props.host ?? ''}:${full}`)
    } else {
      await openFRPTab(props.connName ?? '', full, 0, password.value)
      usageTouch(`frp:${props.connName ?? ''}:${full}`)
    }
    close()
    emit('opened')
  } catch (e) {
    err.value = String(e).replace(/^Error:\s*/, '')
  }
}

/** activate 当前选中项:目录进入、文件开 tab（作用于过滤集）。 */
function activate(idx: number) {
  const e = shown.value[idx]
  if (!e) return
  if (e.isDir) {
    void enter(e.name)
  } else {
    void pickFile(e.name)
  }
}

function close() {
  if (props.mode === 'frp') {
    releaseFRPBrowse(props.connName ?? '').catch(() => {})
  }
  emit('update:show', false)
}

function retry() {
  browse(path.value)
}

/** move 光标 ±delta 并滚动跟随（作用于过滤集）。 */
function move(delta: number) {
  const n = shown.value.length
  if (n === 0) return
  cursor.value = Math.min(n - 1, Math.max(0, cursor.value + delta))
  itemEls.value[cursor.value]?.scrollIntoView({ block: 'nearest' })
}

/** onDocKey 键盘流(document 级,焦点丢失也响应):
 *  ctrl+j/k 上下 · Enter 激活(过滤框聚焦时走过滤语义) · Backspace/h 回上级
 *  · Esc 分层(先清过滤再关窗)。
 *  输入态只放行 Esc、ctrl+j/k 与 Enter(过滤时也要能导航确认;物理 j/k 等仍挡
 *  防打字误触)。Enter 收编 document 层:naive NInput 的 key 事件透传在 jsdom
 *  不可靠,统一分发自可控。 */
function onDocKey(e: KeyboardEvent) {
  if (!props.show) return
  const t = e.target as HTMLElement | null
  const inInput = !!t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA')
  if (e.key === 'Escape') {
    e.preventDefault()
    if (filter.value) {
      filter.value = ''
      return
    }
    close()
    return
  }
  if (inInput && !(e.ctrlKey && (e.key === 'j' || e.key === 'k')) && e.key !== 'Enter') return
  if (e.ctrlKey && e.key === 'j') {
    e.preventDefault()
    move(1)
  } else if (e.ctrlKey && e.key === 'k') {
    e.preventDefault()
    move(-1)
  } else if (e.key === 'Enter') {
    e.preventDefault()
    // needPwd 态让位密码框自身的 keyup retry——keydown 若走 activate 会误开列表项（Final Review I2）
    if (needPwd.value) return
    // 过滤框聚焦走过滤语义（/ 直达或激活选中项）
    if (inInput) onFilterEnter()
    else activate(cursor.value)
  } else if (e.key === 'Backspace' || e.key === 'h') {
    e.preventDefault()
    goUp()
  }
}

// 弹窗栈登记(全局键让位)+ document 键监听随 show 挂卸(第三轮验收②④)
watch(
  () => props.show,
  (v) => {
    if (v) {
      ui.dialogOpen('browse')
      document.addEventListener('keydown', onDocKey)
    } else {
      ui.dialogClose('browse')
      document.removeEventListener('keydown', onDocKey)
    }
  },
  { immediate: true }
)
onBeforeUnmount(() => {
  ui.dialogClose('browse')
  document.removeEventListener('keydown', onDocKey)
})
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :auto-focus="false"
    :trap-focus="false"
    :title="`${mode === 'ssh' ? 'SSH' : 'FRP'} · ${host ?? connName ?? ''}`"
    style="width: 560px"
    @update:show="(v: boolean) => (v ? null : close())"
  >
    <div class="head">
      <a class="up" title="回上级(Backspace)" @click="goUp">⬆</a>
      <span class="path">{{ path }}</span>
      <NInput
        v-if="needPwd"
        v-model:value="password"
        size="tiny"
        type="password"
        show-password-on="click"
        class="pwd"
        placeholder="密码"
        @keyup.enter="retry"
      />
      <NInput
        v-else
        ref="filterEl"
        v-model:value="filter"
        size="tiny"
        class="filter"
        placeholder="过滤，或输 / 路径回车直达"
      />
      <a v-if="needPwd" class="retry" @click="retry">重试</a>
    </div>
    <p v-if="err" class="err">{{ err }}</p>
    <p v-if="loading" class="hint">加载中…</p>
    <ul v-else class="entries">
      <li
        v-for="(e, i) in shown"
        :key="e.name"
        :ref="(el) => (itemEls[i] = el as HTMLElement)"
        :class="[e.isDir ? 'entry-dir' : 'entry-file', { sel: i === cursor }]"
        @click="e.isDir ? enter(e.name) : pickFile(e.name)"
        @mousemove="cursor = i"
      >
        <span class="icon">
          <svg v-if="e.isDir" class="ico" viewBox="0 0 24 24"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" /></svg>
          <svg v-else class="ico" viewBox="0 0 24 24"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><path d="M14 2v6h6" /></svg>
        </span>{{ e.name }}
      </li>
    </ul>
    <p class="keyhint">ctrl+j/k 移动 · Enter 打开 · Backspace 回上级 · 输入过滤/直达 · Esc 关闭</p>
  </NModal>
</template>

<style scoped>
.head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  font-size: 12px;
}
.path {
  flex: 1;
  color: var(--dim, #8b98a9);
  font-family: var(--font-mono, monospace);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.up {
  flex: none;
  cursor: pointer;
  color: var(--accent, #4a8fe0);
  font-size: 13px;
  padding: 0 2px;
}
.up:hover {
  color: var(--fg, #e6e6e6);
}
.pwd {
  width: 140px;
}
.filter {
  width: 180px;
}
.retry {
  color: var(--accent, #4a8fe0);
  cursor: pointer;
}
.err {
  color: var(--level-error, #e57373);
  font-size: 12px;
  margin: 0 0 6px;
}
.hint {
  color: var(--dim, #8b98a9);
  font-size: 12px;
}
.entries {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 320px;
  overflow-y: auto;
  font-size: 12px;
  font-family: var(--font-mono, monospace);
}
.entries li {
  padding: 4px 6px;
  border-radius: 4px;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 6px;
}
.entries li:hover,
.entries li.sel {
  background: var(--popup-bg, #1f2c3f);
}
.entries li.sel {
  box-shadow: inset 2px 0 0 var(--accent, #4a8fe0);
}
.keyhint {
  color: var(--dim, #5d7292);
  font-size: 11px;
  margin: 6px 0 0;
}
</style>
