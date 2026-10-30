<script setup lang="ts">
// FRP 面板：连接列表（frp.json）→ 点击建 stcp 隧道进目录浏览；
// "浏览中"为前端本地标记（存在已开 tab 即活跃——后端无状态机，spec 决策）。
import { computed, onMounted, ref } from 'vue'
import { useTabsStore } from '../../stores/tabs'
import { getFrpConns } from '../../lib/backend'
import BrowseDialog from './BrowseDialog.vue'

const tabs = useTabsStore()
const conns = ref<{ name: string }[]>([])
const browse = ref({ show: false, conn: '' })

async function reload() {
  try {
    conns.value = ((await getFrpConns()) ?? []).map((c) => ({ name: c.name }))
  } catch {
    conns.value = []
  }
}
onMounted(reload)

/** activeConn 前端"浏览中"标记：该连接存在已开 tab。 */
function isActive(name: string): boolean {
  return tabs.tabs.some((t) => t.info.label.includes(name))
}
const anyConn = computed(() => conns.value.length > 0)

function open(name: string) {
  browse.value = { show: true, conn: name }
}
</script>

<template>
  <div class="panel">
    <div class="head">
      <span>FRP 连接</span>
    </div>
    <ul class="list">
      <li v-for="c in conns" :key="c.name" class="conn-item" @click="open(c.name)">
        <span class="name">🔗 {{ c.name }}</span>
        <span v-if="isActive(c.name)" class="live">浏览中</span>
      </li>
    </ul>
    <p v-if="!anyConn" class="empty">无连接——在 ~/.local/state/logview/frp.json 配置或用 TUI 管理</p>
    <BrowseDialog v-model:show="browse.show" mode="frp" :conn-name="browse.conn" />
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
  display: flex;
  align-items: center;
  justify-content: space-between;
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
.live {
  color: var(--level-info, #7ee29a);
  font-size: 11px;
}
.empty {
  color: var(--dim, #5d7292);
}
</style>
