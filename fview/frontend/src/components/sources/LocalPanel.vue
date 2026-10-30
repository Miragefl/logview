<script setup lang="ts">
// 本地文件面板：最近文件（usage.json 热点排序，与 TUI 共享）+ 系统选择框打开。
import { onMounted, ref } from 'vue'
import { openFileTab, openFileViaDialog, usageRecent, usageTouch } from '../../lib/backend'

const recent = ref<string[]>([])

onMounted(async () => {
  try {
    recent.value = (await usageRecent()) ?? []
  } catch {
    recent.value = []
  }
})

/** fileKey 只显示本地文件条目，剥前缀得路径。 */
function filePath(k: string): string {
  return k.startsWith('file:') ? k.slice(5) : ''
}

async function open(k: string) {
  const p = filePath(k)
  if (!p) return
  try {
    await openFileTab(p, 0, true)
    await usageTouch(k)
    recent.value = ((await usageRecent()) ?? []) // 刷新排序
  } catch {
    /* 打开失败静默（文件可能已移动） */
  }
}

async function pick() {
  try {
    await openFileViaDialog(true)
  } catch {
    /* 取消 */
  }
}
</script>

<template>
  <div class="panel">
    <div class="head">
      <span>最近文件</span>
      <button class="open" @click="pick">打开…</button>
    </div>
    <ul class="list">
      <li v-for="k in recent.filter((x) => filePath(x))" :key="k" class="recent-item" @click="open(k)">
        📄 {{ filePath(k) }}
      </li>
    </ul>
    <p v-if="!recent.filter((x) => filePath(x)).length" class="empty">暂无记录，点「打开…」选择日志文件</p>
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
.open {
  background: var(--accent, #2f7d4f);
  color: #fff;
  border: none;
  border-radius: 4px;
  padding: 3px 10px;
  cursor: pointer;
  font-size: 12px;
}
.list {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 50vh;
  overflow-y: auto;
  font-family: var(--font-mono, monospace);
}
.list li {
  padding: 4px 6px;
  border-radius: 4px;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.list li:hover {
  background: var(--popup-bg, #1f2c3f);
}
.empty {
  color: var(--dim, #5d7292);
}
</style>
