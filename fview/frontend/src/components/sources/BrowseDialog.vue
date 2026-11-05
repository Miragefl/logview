<script setup lang="ts">
// 远程目录浏览对话框（SSH/FRP 共用）：进入目录、选文件开 tab、密码认证态重试。
// 键盘流对齐 TUI 弹窗:ctrl+j/k 上下、Enter 进目录/选文件、ctrl+h/Backspace 回上级、Esc 关闭(第三轮验收④)。
// 无框打字过滤(browse-typeahead):面板 r2/r3 同款——打开直接敲字母即过滤(不依赖焦点,
// 旧"打开即聚焦过滤框"排在网络往返后,FRP 建隧道慢时物理键全漏),子序列匹配+命中高亮。
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { NInput, NModal } from 'naive-ui'
import { useUiStore } from '../../stores/ui'
import { isSubseq, splitHighlight } from '../../lib/highlight'
import { useLocalTypeFilter } from './useTypeFilter'
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
/** filter 打字过滤词(无框,本地 ref 随弹窗生命周期,不进面板共享词 ui.panelFilter);
 * 子序列匹配(isSubseq)+splitHighlight 高亮——面板 r2/r3 同款。shown 为导航/激活的作用集。 */
const { q: filter, typeKey } = useLocalTypeFilter()
const shown = computed(() => {
  const f = filter.value.trim()
  if (!f) return entries.value
  return entries.value.filter((e) => isSubseq(e.name, f))
})
// 过滤变化 cursor 重置首项(面板同款;防过滤缩小后 cursor 悬空激活错项)
watch(filter, () => {
  cursor.value = 0
})

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
    // 词的清空归目录切换动作(enter/goUp/直达)——首次打开与 retry 不清:
    // 慢源(FRP 建隧道)加载窗口盲打的词须在新目录渲染后照常生效(I1)
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
      })()
    }
  },
  { immediate: true }
)

/** enter 进入子目录（后端返回的 name 即相对当前路径的目录名）。换目录清词。 */
async function enter(name: string) {
  const base = path.value === '/' ? '' : path.value
  filter.value = ''
  await browse(`${base}/${name}`)
}

/** goUp 回退上级目录(根目录不动)。换目录清词。 */
function goUp() {
  if (path.value === '/' || loading.value) return
  const up = path.value.replace(/\/[^/]*$/, '') || '/'
  filter.value = ''
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

/** onDocKey 键盘流(document 级,焦点无关——无框打字不依赖焦点):
 *  Esc 分层(词非空清词→关窗) · ctrl+j/k 上下 · ctrl+h 回上级(裸 h 进词,用户裁决)
 *  · Enter(词以 / 开头路径直达,否则激活选中项;needPwd 让位 keyup retry)
 *  · Backspace 分层(词非空删尾→回上级) · 其余可打印无修饰字符进词。
 *  needPwd 密码框输入态放行(仅 Esc 穿透),密码字符不进过滤词。 */
function onDocKey(e: KeyboardEvent) {
  if (!props.show) return
  const t = e.target as HTMLElement | null
  const inInput = !!t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA')
  if (inInput && e.key !== 'Escape') return
  if (e.key === 'Escape') {
    e.preventDefault()
    if (typeKey(e)) return // 词非空清词即完
    close()
    return
  }
  if (e.ctrlKey && e.key === 'j') {
    e.preventDefault()
    move(1)
  } else if (e.ctrlKey && e.key === 'k') {
    e.preventDefault()
    move(-1)
  } else if (e.ctrlKey && e.key === 'h') {
    e.preventDefault()
    goUp()
  } else if (e.key === 'Enter') {
    e.preventDefault()
    // needPwd 态让位密码框自身的 keyup retry——keydown 若走 activate 会误开列表项（Final Review I2）
    if (needPwd.value) return
    const f = filter.value.trim()
    if (f.startsWith('/')) {
      filter.value = '' // 路径直达=换目录,词即路径,进 browse 前清
      void browse(f)
    } else {
      activate(cursor.value)
    }
  } else if (e.key === 'Backspace') {
    e.preventDefault()
    if (!typeKey(e)) goUp() // 词非空删词尾,词空回上级
  } else if (typeKey(e)) {
    e.preventDefault()
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
    style="width: min(94vw, calc(var(--ui-font-size) * 47))"
    @update:show="(v: boolean) => (v ? null : close())"
  >
    <div class="head">
      <a class="up" title="回上级(ctrl+h)" @click="goUp">⬆</a>
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
      <a v-if="needPwd" class="retry" @click="retry">重试</a>
    </div>
    <p v-if="filter" class="filter-hint">⌕ {{ filter }}</p>
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
        </span><span v-for="(s, si) in splitHighlight(e.name, filter)" :key="si" :class="{ 'kw-hit': s.hit }">{{ s.t }}</span>
      </li>
    </ul>
    <p class="keyhint">ctrl+j/k 移动 · Enter 打开/直达 · 打字过滤 · Backspace 删词/回上级 · ctrl+h 回上级 · Esc 关闭</p>
  </NModal>
</template>

<style scoped>
.head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  font-size: var(--font-ui-base, 12px);
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
  font-size: var(--font-ui-lg, 13px);
  padding: 0 2px;
}
.up:hover {
  color: var(--fg, #e6e6e6);
}
.pwd {
  width: calc(var(--ui-font-size) * 12);
}
/* 无框打字过滤状态行+命中高亮:与面板 .panel .filter-hint/.kw-hit(style.css 正本)
 * 同 CSS 变量同值——弹窗不在 .panel 作用域,不共用选择器以免动四面板正本 */
.filter-hint {
  color: var(--dim, #5d7292);
  font-family: var(--font-mono, monospace);
  font-size: var(--font-ui-sm, 11px);
  margin: 0 0 6px;
}
.entries .kw-hit {
  color: var(--accent, #7dc4e4);
  font-weight: 600;
}
.retry {
  color: var(--accent, #4a8fe0);
  cursor: pointer;
}
.err {
  color: var(--level-error, #e57373);
  font-size: var(--font-ui-base, 12px);
  margin: 0 0 6px;
}
.hint {
  color: var(--dim, #8b98a9);
  font-size: var(--font-ui-base, 12px);
}
.entries {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: calc(var(--ui-font-size) * 27);
  overflow-y: auto;
  font-size: var(--font-ui-base, 12px);
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
  font-size: var(--font-ui-sm, 11px);
  margin: 6px 0 0;
}
</style>
