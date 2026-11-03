<script setup lang="ts">
// SSH 面板：~/.ssh/config 自动发现（cfg 角标，删不掉属预期）+ 手动增删 → 点击进目录浏览。
import { onMounted, ref } from 'vue'
import { NInput } from 'naive-ui'
import { addSSHHost, getSSHHosts, removeSSHHost, type SSHHost } from '../../lib/backend'
import { useUiStore } from '../../stores/ui'

const ui = useUiStore()
const hosts = ref<SSHHost[]>([])
const newHost = ref('')

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
  // 全局浏览弹窗（App.vue 单例，欢迎页最近源同入口）
  ui.openBrowse({ mode: 'ssh', host: h })
}
</script>

<template>
  <div class="panel">
    <div class="head">
      <span>SSH 主机</span>
    </div>
    <ul class="list">
      <li v-for="h in hosts" :key="h.name" class="host-item" @click="open(h.name)">
        <span class="name">
          <svg class="ico" viewBox="0 0 24 24"><path d="M2 3h20v14H2z" /><path d="M8 21h8M12 17v4" /></svg>
          {{ h.name }}<span v-if="h.fromSshConfig" class="cfg" title="~/.ssh/config 自动发现">cfg</span>
        </span>
        <!-- cfg 主机来自 ~/.ssh/config，删了也会被重新发现——不提供 ×，只留手动添加的 -->
        <span v-if="!h.fromSshConfig" class="del" title="移除" @click.stop="remove(h.name)">
          <svg class="ico" viewBox="0 0 24 24"><path d="M18 6L6 18M6 6l12 12" /></svg>
        </span>
      </li>
    </ul>
    <p v-if="!hosts.length" class="empty">无主机——下方添加（或检查 ~/.ssh/config）</p>
    <div class="add-row">
      <NInput v-model:value="newHost" size="tiny" placeholder="user@host[:port]" @keyup.enter="add" />
      <button class="add" @click="add">
        <svg class="ico" viewBox="0 0 24 24"><path d="M12 5v14M5 12h14" /></svg>
      </button>
    </div>
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
/* cfg 角标：~/.ssh/config 自动发现的主机 */
.cfg {
  margin-left: 4px;
  font-size: 10px;
  color: var(--dim, #5d7292);
  border: 1px solid var(--popup-border, #2a3a4f);
  border-radius: 3px;
  padding: 0 3px;
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
