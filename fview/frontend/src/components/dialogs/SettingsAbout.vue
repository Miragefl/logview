<script setup lang="ts">
// 关于页：图标 + 当前版本 + 检测更新（第九轮验收②）。
// 图标 src/assets/appicon.svg 为 fview/build/appicon.svg 的拷贝(改图以 build/ 正本为准)。
import { onMounted, ref } from 'vue'
import { NButton } from 'naive-ui'
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime'
import { appVersion, checkUpdate } from '../../lib/backend'
import type { UpdateInfo } from '../../lib/backend'
import icon from '../../assets/appicon.svg'

const version = ref('')
const checking = ref(false)
/** result 检测结果:null=未测;kind=latest 已是最新/newer 有新版/error 失败。 */
const result = ref<{ kind: 'latest' | 'newer' | 'error'; info?: UpdateInfo; msg?: string } | null>(null)

onMounted(async () => {
  try {
    version.value = (await appVersion()) || 'dev'
  } catch {
    version.value = 'dev'
  }
})

async function onCheck() {
  checking.value = true
  result.value = null
  try {
    const info = await checkUpdate()
    result.value =
      info.latest && info.latest !== version.value
        ? { kind: 'newer', info }
        : { kind: 'latest', info }
  } catch (e) {
    result.value = { kind: 'error', msg: e instanceof Error ? e.message : String(e) }
  } finally {
    checking.value = false
  }
}

function openRelease() {
  if (result.value?.info?.url) BrowserOpenURL(result.value.info.url)
}
</script>

<template>
  <div class="about">
    <img class="icon" :src="icon" alt="fview" />
    <h3>fview</h3>
    <p>当前版本 <span class="ver">{{ version }}</span></p>
    <p class="dim">LogView 桌面版 · 与 TUI 共享 logcore（stream/parser/model/buffer/frp）</p>
    <p class="dim">本地文件 / SSH / K8s / FRP 多日志源 · 标签页 · 虚拟滚动</p>
    <div class="update">
      <NButton size="small" class="check" :loading="checking" @click="onCheck">检测更新</NButton>
      <span v-if="result?.kind === 'newer'" class="newer">
        发现新版本 {{ result.info?.latest }}（当前 {{ version }}）
        <a @click.prevent="openRelease" href="#">打开下载页</a>
      </span>
      <span v-else-if="result?.kind === 'latest'" class="ok">已是最新</span>
      <span v-else-if="result?.kind === 'error'" class="err">检测失败：{{ result.msg }}</span>
    </div>
  </div>
</template>

<style scoped>
.about {
  padding: 20px 10px;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}
.icon {
  width: 96px;
  height: 96px;
  margin-bottom: 6px;
}
h3 {
  margin: 0 0 4px;
  letter-spacing: 2px;
}
p {
  margin: 4px 0;
  font-size: var(--font-ui-lg, 13px);
}
.ver {
  font-family: var(--font-mono, monospace);
}
.dim {
  color: var(--dim, #5d7292);
  font-size: var(--font-ui-sm, 11px);
}
.update {
  margin-top: 12px;
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 34px;
}
.newer {
  color: var(--accent, #4a8fe0);
  font-size: var(--font-ui-base, 12px);
}
.newer a {
  cursor: pointer;
  text-decoration: underline;
}
.ok {
  color: var(--ok, #28c840);
  font-size: var(--font-ui-base, 12px);
}
.err {
  color: var(--err, #e06c75);
  font-size: var(--font-ui-base, 12px);
}
</style>
