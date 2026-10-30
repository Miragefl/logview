<script setup lang="ts">
// 解析规则页：rules.yaml 共享正本的表格编辑（m4m6 RulesPanel 逻辑迁入）。
// 保存后已开 tab 不受影响，新 tab 用新规则解析。
import { onMounted, ref } from 'vue'
import { NButton } from 'naive-ui'
import { getRules, saveRules, type RuleConfig } from '../../lib/backend'

const rows = ref<RuleConfig[]>([])
const dirty = ref(false)
const saving = ref(false)
const savedAt = ref('')

onMounted(async () => {
  try {
    rows.value = ((await getRules()) ?? []).map((r) => ({ ...r }))
  } catch {
    rows.value = []
  }
})

function add() {
  rows.value.push({ name: 'new-rule', pattern: '(?P<message>.*)', fallback: false })
  dirty.value = true
}

function remove(i: number) {
  rows.value.splice(i, 1)
  dirty.value = true
}

async function save() {
  saving.value = true
  try {
    await saveRules(rows.value.map((r) => ({ ...r })))
    dirty.value = false
    savedAt.value = new Date().toLocaleTimeString()
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="rules">
    <div class="head-row">
      <span class="c-name">名称</span>
      <span class="c-pattern">正则（命名捕获组：time/level/thread/traceId/logger/message）</span>
      <span class="c-fb">兜底</span>
      <span></span>
    </div>
    <div class="rows">
      <div v-for="(r, i) in rows" :key="i" class="row">
        <input v-model="r.name" class="c-name" @input="dirty = true" />
        <input v-model="r.pattern" class="c-pattern mono" @input="dirty = true" />
        <input
          v-model="r.fallback"
          type="checkbox"
          class="c-fb"
          @change="dirty = true"
        />
        <button class="del" @click="remove(i)">删</button>
      </div>
    </div>
    <div class="foot">
      <NButton size="tiny" class="rules-add" @click="add">+ 新增规则</NButton>
      <span class="hint">
        {{ savedAt ? `已保存 ${savedAt}` : '保存写回 ~/.config/logview/rules.yaml，TUI 下次启动同样生效' }}
      </span>
      <NButton
        size="tiny"
        type="primary"
        class="rules-save"
        :disabled="!dirty || saving"
        @click="save"
      >
        {{ saving ? '保存中…' : '保存' }}
      </NButton>
    </div>
  </div>
</template>

<style scoped>
.rules {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.head-row,
.row {
  display: grid;
  grid-template-columns: 110px 1fr 36px 36px;
  gap: 8px;
  align-items: center;
  font-size: 12px;
}
.head-row {
  color: var(--dim, #5d7292);
  font-size: 11px;
}
.rows {
  max-height: 300px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
input {
  background: var(--bg, #10161f);
  border: 1px solid var(--popup-border, #24354a);
  border-radius: 4px;
  color: var(--fg, #e6e6e6);
  padding: 3px 8px;
  font-size: 12px;
  width: 100%;
  box-sizing: border-box;
}
.mono {
  font-family: var(--font-mono, monospace);
}
.c-fb {
  width: auto;
}
.del {
  background: transparent;
  border: 1px solid var(--popup-border, #2a3a4f);
  border-radius: 4px;
  color: var(--dim, #8b98a9);
  cursor: pointer;
  font-size: 11px;
  padding: 1px 6px;
}
.del:hover {
  color: var(--level-error, #e57373);
}
.foot {
  display: flex;
  align-items: center;
  gap: 10px;
}
.hint {
  flex: 1;
  color: var(--dim, #5d7292);
  font-size: 11px;
}
</style>
