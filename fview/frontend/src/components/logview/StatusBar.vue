<script setup lang="ts">
// 状态栏：可见行数/搜索命中数/Follow 态/内存徽章（5s 轮询 GetMemStats）/字号。
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useTabsStore } from '../../stores/tabs'
import { useUiStore } from '../../stores/ui'
import { getMemStats } from '../../lib/backend'

defineProps<{ hitCount: number }>()

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
    <span v-if="hitCount > 0">命中 {{ hitCount }}</span>
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
  font-size: 11px;
  color: var(--dim, #5d7292);
  border-top: 1px solid var(--popup-border, #1e2837);
  user-select: none;
}
.follow {
  color: var(--level-info, #7ee29a);
}
.spacer {
  flex: 1;
}
</style>
