<script setup lang="ts">
// 远程目录浏览对话框（SSH/FRP 共用）：进入目录、选文件开 tab、密码认证态重试。
import { ref, watch } from 'vue'
import { NInput, NModal } from 'naive-ui'
import {
  browseFRP,
  openFRPTab,
  openSSHTab,
  releaseFRPBrowse,
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
}>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void; (e: 'opened'): void }>()

const path = ref('/')
const entries = ref<DirEntry[]>([])
const password = ref('')
const err = ref('')
const needPwd = ref(false)
const loading = ref(false)

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
  } catch (e) {
    const msg = String(e)
    needPwd.value = /password|密码|auth/i.test(msg)
    err.value = msg.replace(/^Error:\s*/, '')
  } finally {
    loading.value = false
  }
}

// 对话框每次打开都从根目录拉起（常驻挂载，不能依赖 onMounted）
watch(
  () => props.show,
  (v) => {
    if (v) {
      path.value = '/'
      entries.value = []
      browse('/')
    }
  },
  { immediate: true }
)

/** enter 进入子目录（后端返回的 name 即相对当前路径的目录名）。 */
async function enter(name: string) {
  const base = path.value === '/' ? '' : path.value
  await browse(`${base}/${name}`)
}

/** pickFile 选日志文件开 tab（尾读 200 行 + follow）。 */
async function pickFile(name: string) {
  const base = path.value === '/' ? '' : path.value
  const full = `${base}/${name}`
  try {
    if (props.mode === 'ssh') {
      await openSSHTab(props.host ?? '', full, 200, password.value)
      usageTouch(`ssh:${props.host ?? ''}`)
    } else {
      await openFRPTab(props.connName ?? '', full, 200, password.value)
      usageTouch(`frp:${props.connName ?? ''}`)
    }
    close()
    emit('opened')
  } catch (e) {
    err.value = String(e).replace(/^Error:\s*/, '')
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
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="`${mode === 'ssh' ? 'SSH' : 'FRP'} · ${host ?? connName ?? ''}`"
    style="width: 560px"
    @update:show="(v: boolean) => (v ? null : close())"
  >
    <div class="head">
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
    <p v-if="err" class="err">{{ err }}</p>
    <p v-if="loading" class="hint">加载中…</p>
    <ul v-else class="entries">
      <li
        v-for="e in entries"
        :key="e.name"
        :class="e.isDir ? 'entry-dir' : 'entry-file'"
        @click="e.isDir ? enter(e.name) : pickFile(e.name)"
      >
        <span class="icon">{{ e.isDir ? '📁' : '📄' }}</span>{{ e.name }}
      </li>
    </ul>
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
.pwd {
  width: 140px;
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
.entries li:hover {
  background: var(--popup-bg, #1f2c3f);
}
</style>
