<script setup lang="ts">
// 状态栏：可见行数/Follow 态/内存徽章（5s 轮询 GetMemStats）/字号（命中数已迁 SearchBar）。
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useTabsStore } from '../../stores/tabs'
import { useUiStore } from '../../stores/ui'
import { getMemStats } from '../../lib/backend'

defineProps<{ copied?: number }>()

const tabs = useTabsStore()
const ui = useUiStore()

const memMB = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

async function poll() {
  try {
    memMB.value = Math.round(((await getMemStats()) ?? 0) / 1024 / 1024)
  } catch {
    /* 后端不可用时静默 */
  }
}
onMounted(() => {
  poll()
  timer = setInterval(poll, 5000)
})
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <footer class="statusbar">
    <span>{{ tabs.active?.info.lines ?? 0 }} 行</span>
    <span v-if="copied && copied > 0" class="copied">已复制 {{ copied }} 行</span>
    <span v-if="ui.visualActive" class="visual-badge">VISUAL</span>
    <span v-if="tabs.active?.info.follow" class="follow">● Following</span>
    <div class="spacer"></div>
    <span v-if="memMB > 0">内存 {{ memMB }}MB</span>
    <span>{{ ui.fontSize }}px</span>
  </footer>
</template>

<style scoped>
.statusbar {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 3px 10px;
  font-size: var(--font-ui-sm, 11px);
  color: var(--dim, #5d7292);
  border-top: 1px solid var(--popup-border, #1e2837);
  user-select: none;
}
.follow {
  color: var(--level-info, #7ee29a);
}
/* VISUAL 徽标:v/V 进可视选择的模式提示(vim 同款反馈;G 在 visual 下扩选属 TUI/vim 语义) */
.visual-badge {
  background: var(--visual, #005f2f);
  color: #fff;
  border-radius: 3px;
  padding: 0 5px;
  font-weight: 600;
}
.copied {
  color: var(--accent, #7dc4e4);
}
.spacer {
  flex: 1;
}
</style>
