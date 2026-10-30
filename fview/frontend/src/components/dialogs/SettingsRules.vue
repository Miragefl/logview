<script setup lang="ts">
// 解析规则页：规则列表（模板版 pattern）+ 弹窗编辑（大输入框）+ 字段片段 patterns 管理。
// pattern 用 {name} 引用 patterns 片段——time/thread 等字段的正则在 patterns 段可视化管理（验收反馈④）。
import { onMounted, ref } from 'vue'
import { NButton, NCheckbox, NInput, NModal, NSelect } from 'naive-ui'
import { getPatterns, getRules, savePatterns, saveRules, type RuleConfig } from '../../lib/backend'

const rules = ref<RuleConfig[]>([])
const patterns = ref<Record<string, string>>({})
const patDirty = ref(false)
const saving = ref(false)
const savedAt = ref('')

onMounted(async () => {
  try {
    rules.value = ((await getRules()) ?? []).map((r) => ({ ...r }))
  } catch {
    rules.value = []
  }
  try {
    patterns.value = { ...((await getPatterns()) ?? {}) }
  } catch {
    patterns.value = {}
  }
})

// ---- 规则编辑弹窗 ----
const editShow = ref(false)
const editIdx = ref(-1) // -1 = 新增
const editForm = ref<RuleConfig>({ name: '', pattern: '', parse: '', fallback: false })

const parseOptions = [
  { label: '正则（默认）', value: '' },
  { label: 'JSON 解析', value: 'json' },
]

function openAdd() {
  editIdx.value = -1
  editForm.value = { name: 'new-rule', pattern: '{time} {level} {message}', parse: '', fallback: false }
  editShow.value = true
}

function openEdit(i: number) {
  editIdx.value = i
  editForm.value = { ...rules.value[i] }
  editShow.value = true
}

async function saveRule() {
  const f = editForm.value
  if (!f.name.trim() || !f.pattern.trim()) return
  saving.value = true
  try {
    if (editIdx.value >= 0) {
      rules.value[editIdx.value] = { ...f }
    } else {
      rules.value.push({ ...f })
    }
    await saveRules(rules.value.map((r) => ({ ...r })))
    editShow.value = false
    savedAt.value = new Date().toLocaleTimeString()
  } finally {
    saving.value = false
  }
}

async function removeRule(i: number) {
  rules.value.splice(i, 1)
  await saveRules(rules.value.map((r) => ({ ...r })))
  savedAt.value = new Date().toLocaleTimeString()
}

// ---- patterns 字段片段管理 ----
const newPat = ref({ name: '', regex: '' })

async function addPattern() {
  const n = newPat.value.name.trim()
  if (!n || !newPat.value.regex.trim()) return
  patterns.value[n] = newPat.value.regex
  newPat.value = { name: '', regex: '' }
  patDirty.value = true
}

function removePattern(name: string) {
  delete patterns.value[name]
  patterns.value = { ...patterns.value }
  patDirty.value = true
}

async function persistPatterns() {
  saving.value = true
  try {
    await savePatterns({ ...patterns.value })
    patDirty.value = false
    savedAt.value = new Date().toLocaleTimeString()
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="rules">
    <section>
      <h4>解析规则（pattern 支持 {name} 引用下方片段；保存后新 tab 生效）</h4>
      <ul class="rule-list">
        <li v-for="(r, i) in rules" :key="i">
          <span class="rule-name">{{ r.name }}</span>
          <span v-if="r.parse" class="tag">json</span>
          <span v-if="r.fallback" class="tag dim">兜底</span>
          <span class="rule-pattern" :title="r.pattern">{{ r.pattern }}</span>
          <span class="ops">
            <a class="op rules-edit" @click="openEdit(i)">编辑</a>
            <a class="op danger" @click="removeRule(i)">删</a>
          </span>
        </li>
      </ul>
      <NButton size="tiny" class="rules-add" @click="openAdd">＋ 新增规则</NButton>
    </section>

    <section>
      <h4>字段片段 patterns（time/thread 等字段的正则）</h4>
      <div class="pat-rows">
        <div v-for="(regex, name) in patterns" :key="name" class="pat-row">
          <span class="pat-name">{name}</span>
          <input
            class="pat-regex mono"
            :value="regex"
            @input="
              patterns[name] = ($event.target as HTMLInputElement).value;
              patDirty = true
            "
          />
          <a class="op danger" title="删除片段" @click="removePattern(name as string)">&times;</a>
        </div>
      </div>
      <div class="pat-add-row">
        <input v-model="newPat.name" class="pat-name-input mono" placeholder="片段名" />
        <input v-model="newPat.regex" class="pat-regex mono" placeholder="正则（如 (?P<time>\\d+)）" />
        <NButton size="tiny" @click="addPattern">＋</NButton>
      </div>
      <div class="foot">
        <span class="hint">
          {{ savedAt ? `已保存 ${savedAt}` : '写回 ~/.config/logview/rules.yaml，TUI 同源生效' }}
        </span>
        <NButton
          size="tiny"
          type="primary"
          class="pat-save"
          :disabled="!patDirty || saving"
          @click="persistPatterns"
        >
          保存片段
        </NButton>
      </div>
    </section>

    <!-- 规则编辑弹窗（大输入框，验收反馈④） -->
    <NModal v-model:show="editShow" preset="dialog" title="规则编辑" style="width: 620px">
      <div class="rule-form">
        <label>规则名</label>
        <NInput v-model:value="editForm.name" size="small" />
        <label>pattern（{name} 引用片段；如 {time} [{thread}] {level} {message}）</label>
        <NInput
          v-model:value="editForm.pattern"
          type="textarea"
          :rows="5"
          class="mono"
          placeholder="{time} [{thread}] [{traceId}] {level} {logger} - ?{message}"
        />
        <div class="form-row">
          <div class="half">
            <label>解析方式</label>
            <NSelect v-model:value="editForm.parse" size="small" :options="parseOptions" />
          </div>
          <div class="half chk">
            <NCheckbox v-model:checked="editForm.fallback">兜底规则（未命中其他规则时回落）</NCheckbox>
          </div>
        </div>
      </div>
      <template #action>
        <NButton size="small" @click="editShow = false">取消</NButton>
        <NButton size="small" type="primary" class="rule-save" :loading="saving" @click="saveRule">
          保存
        </NButton>
      </template>
    </NModal>
  </div>
</template>

<style scoped>
.rules {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
h4 {
  margin: 0 0 6px;
  font-size: 12px;
  color: var(--dim, #8b98a9);
  font-weight: normal;
}
.rule-list {
  list-style: none;
  margin: 0 0 6px;
  padding: 0;
  max-height: 160px;
  overflow-y: auto;
  font-size: 12px;
}
.rule-list li {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 2px;
}
.rule-name {
  flex: none;
  max-width: 130px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.tag {
  flex: none;
  font-size: 10px;
  border: 1px solid var(--accent, #4a8fe0);
  color: var(--accent, #4a8fe0);
  border-radius: 3px;
  padding: 0 4px;
}
.tag.dim {
  border-color: var(--dim, #5d7292);
  color: var(--dim, #5d7292);
}
.rule-pattern {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--dim, #c8d3e0);
  font-family: var(--font-mono, monospace);
}
.ops {
  flex: none;
  display: flex;
  gap: 8px;
}
.op {
  cursor: pointer;
  color: var(--accent, #4a8fe0);
}
.op.danger {
  color: var(--level-error, #e57373);
}
.pat-rows {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 180px;
  overflow-y: auto;
  margin-bottom: 6px;
}
.pat-row {
  display: flex;
  align-items: center;
  gap: 6px;
}
.pat-name {
  flex: none;
  width: 90px;
  color: var(--fg, #e6e6e6);
}
.pat-regex {
  flex: 1;
  min-width: 0;
}
.pat-add-row {
  display: flex;
  gap: 6px;
  align-items: center;
}
.pat-name-input {
  width: 90px;
  flex: none;
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
.foot {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 6px;
}
.hint {
  flex: 1;
  color: var(--dim, #5d7292);
  font-size: 11px;
}
.rule-form {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
}
.rule-form label {
  color: var(--dim, #8b98a9);
  font-size: 11px;
  margin-top: 6px;
}
.form-row {
  display: flex;
  gap: 14px;
  align-items: end;
}
.half {
  flex: 1;
}
.chk {
  padding-bottom: 4px;
}
</style>
