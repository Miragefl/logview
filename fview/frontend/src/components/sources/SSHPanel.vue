<script setup lang="ts">
// SSH 面板：~/.ssh/config 自动导入的主机列表 + 手动增删 → 点击进目录浏览。
import { onMounted, ref } from 'vue'
import { NInput } from 'naive-ui'
import { addSSHHost, getSSHHosts, removeSSHHost } from '../../lib/backend'
import BrowseDialog from './BrowseDialog.vue'

const hosts = ref<string[]>([])
const newHost = ref('')
const browse = ref({ show: false, host: '' })

async function reload() {
  try {
    hosts.value = (await getSSHHosts()) ?? []
  } catch {
    hosts.value = []
  }
}
onMounted(reload)

async function add() {
  const h = newHost.value.trim()
  if (!h) return
  try {
    await addSSHHost(h)
    newHost.value = ''
    await reload()
  } catch {
    /* 无效 host 静默 */
  }
}

async function remove(h: string) {
  try {
    await removeSSHHost(h)
    await reload()
  } catch {
    /* */
  }
}

function open(h: string) {
  browse.value = { show: true, host: h }
}
</script>

<template>
  <div class="panel">
    <div class="head">
      <span>SSH 主机</span>
    </div>
    <ul class="list">
      <li v-for="h in hosts" :key="h" class="host-item" @click="open(h)">
        <span class="name">🖥 {{ h }}</span>
        <span class="del" title="移除" @click.stop="remove(h)">&times;</span>
      </li>
    </ul>
    <p v-if="!hosts.length" class="empty">无主机——下方添加（或检查 ~/.ssh/config）</p>
    <div class="add-row">
      <NInput v-model:value="newHost" size="tiny" placeholder="user@host[:port]" @keyup.enter="add" />
      <button class="add" @click="add">＋</button>
    </div>
    <BrowseDialog v-model:show="browse.show" mode="ssh" :host="browse.host" />
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
.del {
  color: var(--dim, #8b98a9);
  padding: 0 4px;
}
.del:hover {
  color: var(--level-error, #e57373);
}
.empty {
  color: var(--dim, #5d7292);
}
.add-row {
  display: flex;
  gap: 4px;
}
.add {
  background: var(--accent, #2a5db0);
  color: #fff;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  padding: 2px 10px;
}
</style>
