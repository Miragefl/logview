<script setup lang="ts">
// FRP 面板：连接列表（frp.json）→ 点击建 stcp 隧道进目录浏览；
// "浏览中"为前端本地标记（存在已开 tab 即活跃——后端无状态机，spec 决策）。
// 新增/删除连接走管理表单（server 不存在则一并 upsert，一站式）。
import { computed, onMounted, ref } from 'vue'
import { NButton, NInput, NModal } from 'naive-ui'
import { useTabsStore } from '../../stores/tabs'
import {
  deleteFrpConn,
  getFrpConns,
  getFrpServers,
  upsertFrpConn,
  upsertFrpServer,
} from '../../lib/backend'
import BrowseDialog from './BrowseDialog.vue'

const tabs = useTabsStore()
const conns = ref<{ name: string; server: string }[]>([])
const browse = ref({ show: false, conn: '' })

async function reload() {
  try {
    conns.value = ((await getFrpConns()) ?? []).map((c) => ({ name: c.name, server: c.server }))
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

async function remove(name: string) {
  try {
    await deleteFrpConn(name)
    await reload()
  } catch {
    /* */
  }
}

// ---- 新增连接表单 ----
const formShow = ref(false)
const form = ref({ name: '', server: '', addr: '', token: '', sk: '', proxy: '', user: '' })
const servers = ref<string[]>([])
const saving = ref(false)
const formErr = ref('')

async function openForm() {
  try {
    servers.value = ((await getFrpServers()) ?? []).map((s) => s.name)
  } catch {
    servers.value = []
  }
  form.value = { name: '', server: servers.value[0] ?? '', addr: '', token: '', sk: '', proxy: '', user: '' }
  formErr.value = ''
  formShow.value = true
}

async function saveForm() {
  const f = form.value
  if (!f.name.trim() || !f.server.trim()) {
    formErr.value = '连接名与服务器名为必填'
    return
  }
  saving.value = true
  try {
    // 地址非空 → 一并 upsert 服务器（新 server 或改地址）；留空则引用已有 server
    if (f.addr.trim()) {
      await upsertFrpServer({ name: f.server.trim(), addr: f.addr.trim(), token: f.token })
    }
    await upsertFrpConn({
      name: f.name.trim(),
      server: f.server.trim(),
      sk: f.sk,
      proxy: f.proxy,
      user: f.user,
      path: '',
    })
    formShow.value = false
    await reload()
  } catch (e) {
    formErr.value = String(e).replace(/^Error:\s*/, '')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="panel">
    <div class="head">
      <span>FRP 连接</span>
      <button class="add conn-add" title="新增连接" @click="openForm">＋</button>
    </div>
    <ul class="list">
      <li v-for="c in conns" :key="c.name" class="conn-item" @click="open(c.name)">
        <span class="name">🔗 {{ c.name }}</span>
        <span class="meta">
          <span v-if="isActive(c.name)" class="live">浏览中</span>
          <span
            class="conn-del"
            title="删除连接"
            @click.stop="remove(c.name)"
            >&times;</span
          >
        </span>
      </li>
    </ul>
    <p v-if="!anyConn" class="empty">无连接——点「＋」新增（frp.json 共享正本）</p>

    <NModal v-model:show="formShow" preset="dialog" title="新增 FRP 连接" style="width: 420px">
      <div class="conn-form">
        <label>连接名 *</label>
        <NInput v-model:value="form.name" size="small" placeholder="如 home-nas" />
        <label>服务器名 *（已选：{{ servers.join(' / ') || '无' }}）</label>
        <NInput v-model:value="form.server" size="small" placeholder="如 office-frps" />
        <label>服务器地址（填了则新建/更新该服务器）</label>
        <NInput v-model:value="form.addr" size="small" placeholder="frps.example.com:7000" />
        <label>Token（服务器带鉴权时填）</label>
        <NInput v-model:value="form.token" size="small" show-password-on="click" type="password" />
        <label>SK（stcp 密钥）</label>
        <NInput v-model:value="form.sk" size="small" show-password-on="click" type="password" />
        <label>代理名 / 用户（可选）</label>
        <div class="row2">
          <NInput v-model:value="form.proxy" size="small" placeholder="proxy" />
          <NInput v-model:value="form.user" size="small" placeholder="user" />
        </div>
        <p v-if="formErr" class="err">{{ formErr }}</p>
      </div>
      <template #action>
        <NButton size="small" @click="formShow = false">取消</NButton>
        <NButton size="small" type="primary" class="conn-save" :loading="saving" @click="saveForm">
          保存
        </NButton>
      </template>
    </NModal>

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
  display: flex;
  justify-content: space-between;
  align-items: center;
  color: var(--dim, #8b98a9);
}
.add {
  background: transparent;
  border: 1px solid var(--popup-border, #2a3a4f);
  border-radius: 4px;
  color: var(--dim, #8b98a9);
  cursor: pointer;
  padding: 1px 8px;
  font-size: 13px;
}
.add:hover {
  color: var(--accent, #4a8fe0);
  border-color: var(--accent, #4a8fe0);
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
.meta {
  display: flex;
  align-items: center;
  gap: 6px;
}
.live {
  color: var(--level-info, #7ee29a);
  font-size: 11px;
}
.conn-del {
  color: var(--dim, #8b98a9);
  padding: 0 4px;
  cursor: pointer;
}
.conn-del:hover {
  color: var(--level-error, #e57373);
}
.empty {
  color: var(--dim, #5d7292);
}
.conn-form {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
}
.conn-form label {
  color: var(--dim, #8b98a9);
  font-size: 11px;
  margin-top: 4px;
}
.row2 {
  display: flex;
  gap: 6px;
}
.err {
  color: var(--level-error, #e57373);
  font-size: 11px;
  margin: 4px 0 0;
}
</style>
