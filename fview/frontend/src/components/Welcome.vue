<script setup lang="ts">
// 欢迎页：无 tab 时显示，最近源快捷入口（usage.json 热点，与 TUI 共享）。
// 四类键全可点（openRecent 分派）：file/ssh/frp/k8sres 有密码直开、无密码弹浏览。
import { onMounted, ref } from 'vue'
import { usageRecent } from '../lib/backend'
import { openRecent, recentLabel } from '../lib/recent'

const recent = ref<string[]>([])

onMounted(async () => {
  try {
    recent.value = (await usageRecent()) ?? []
  } catch {
    recent.value = []
  }
})
</script>

<template>
  <div class="welcome">
    <h1 class="title">fview</h1>
    <p class="subtitle">左侧图标条选择日志源，或从最近使用开始：</p>
    <ul v-if="recent.length" class="recent">
      <li v-for="k in recent.slice(0, 8)" :key="k" class="clickable" :title="k" @click="openRecent(k)">
        {{ recentLabel(k) }}
      </li>
    </ul>
    <p v-else class="empty">暂无最近记录——先打开一个日志文件</p>
  </div>
</template>

<style scoped>
.welcome {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  user-select: none;
}
.title {
  font-size: 42px;
  margin: 0;
  color: var(--fg, #e6e6e6);
  letter-spacing: 2px;
}
.subtitle {
  color: var(--dim, #8b98a9);
  font-size: 13px;
}
.recent {
  list-style: none;
  margin: 8px 0 0;
  padding: 0;
  color: var(--fg, #c8d3e0);
  font-size: 12px;
  font-family: var(--font-mono, monospace);
  line-height: 2;
  max-height: 40vh;
  overflow-y: auto;
}
.recent .clickable {
  cursor: pointer;
  padding: 0 8px;
  border-radius: 4px;
}
.recent .clickable:hover {
  background: var(--popup-bg, #1f2c3f);
}
.empty {
  color: var(--dim, #5d7292);
  font-size: 12px;
}
</style>
