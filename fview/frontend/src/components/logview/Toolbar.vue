<script setup lang="ts">
// 日志工具栏：搜索（time:/level: 语法）/高亮/隐藏指示/字段显隐/级别档位/Follow/字号。
import { computed, onMounted, ref } from 'vue'
import { NBadge, NButton, NDropdown, NInput, NPopover, NSelect } from 'naive-ui'
import { useTabsStore } from '../../stores/tabs'
import { useAnnotationsStore } from '../../stores/annotations'
import { useUiStore } from '../../stores/ui'
import type { LevelFilterMode } from '../../lib/filter'
import HighlightMenu from './HighlightMenu.vue'
import {
  getFields,
  matchLines,
  parseQuery,
  saveFields,
  setFollow,
} from '../../lib/backend'
import type { FieldConfig } from '../../lib/backend'

const emit = defineEmits<{
  (e: 'search', payload: { hitSeqs: number[]; words: string[] }): void
  (e: 'update:levelMode', mode: LevelFilterMode): void
  (e: 'update:fields', fields: string[]): void
}>()

const props = defineProps<{ levelMode: LevelFilterMode; fields: string[] }>()

const tabs = useTabsStore()
const ann = useAnnotationsStore()
const ui = useUiStore()

const query = ref('')
const searchErr = ref('')
const searchInput = ref<InstanceType<typeof NInput> | null>(null)

/** focusSearch 供 vim 键 `/` 聚焦。 */
function focusSearch() {
  searchInput.value?.focus()
}
defineExpose({ focusSearch })

async function runSearch() {
  const t = tabs.active
  if (!t) return
  searchErr.value = ''
  try {
    const meta = await parseQuery(query.value)
    if (meta.parseError) {
      searchErr.value = meta.parseError
      return
    }
    // MatchLines 返回 buffer 索引域（0..n-1）——镜像与后端同裁稳态对齐，
    // 在此换算为 seq 域下发（渲染/跳转/详情统一 seq 域）
    const idxs = query.value.trim() ? await matchLines(t.info.id, query.value) : []
    const lines = t.lines
    const seqs = idxs.map((i) => lines[i]?.seq).filter((s): s is number => s !== undefined)
    emit('search', { hitSeqs: seqs, words: meta.highlightKeywords ?? [] })
  } catch (e) {
    searchErr.value = String(e)
  }
}

// ---- 字段显隐（rules.yaml hides 同源） ----
const allFields = ref<FieldConfig[]>([])
onMounted(async () => {
  try {
    allFields.value = (await getFields()) ?? []
  } catch {
    allFields.value = []
  }
})

async function toggleField(name: string) {
  const f = allFields.value.find((x) => x.name === name)
  if (!f) return
  f.visible = !f.visible
  try {
    // 字段显隐只写 fields 配置——rules.yaml 的 hides 是 TUI 行级隐藏词，严禁混写（会污染 TUI）
    await saveFields(allFields.value)
    emit('update:fields', allFields.value.filter((x) => x.visible).map((x) => x.name))
  } catch {
    f.visible = !f.visible // 回滚
  }
}

/** fieldsOptions 字段下拉项（勾选态实时反映）。 */
const fieldsOptions = computed(() =>
  allFields.value.map((f) => ({
    label: `${f.visible ? '☑' : '☐'} ${f.name}`,
    key: f.name,
  }))
)

const levelOptions = [
  { label: '全部', value: 'A' },
  { label: '≥INFO', value: 'I' },
  { label: '≥WARN', value: 'W' },
  { label: 'ERROR', value: 'E' },
  { label: 'DEBUG', value: 'D' },
]

async function onFollow(follow: boolean) {
  const t = tabs.active
  if (!t) return
  tabs.toggleFollow(t.info.id, follow)
  try {
    await setFollow(t.info.id, follow)
  } catch {
    /* 后端失败保底本地态 */
  }
}

function zoom(delta: number) {
  ui.fontSize = Math.min(28, Math.max(9, ui.fontSize + delta))
  ui.persist()
}
</script>

<template>
  <div class="toolbar">
    <NInput
      ref="searchInput"
      v-model:value="query"
      size="tiny"
      clearable
      class="search"
      :placeholder="searchErr ? searchErr : '搜索（time:/level: 语法），Enter 执行'"
      :status="searchErr ? 'error' : undefined"
      @keyup.enter="runSearch"
    />
    <NPopover trigger="click" placement="bottom-start">
      <template #trigger>
        <NBadge :value="ann.highlights.length" :max="99" type="info">
          <NButton size="tiny" quaternary>🏷 高亮</NButton>
        </NBadge>
      </template>
      <HighlightMenu />
    </NPopover>
    <NPopover trigger="click" placement="bottom-start">
      <template #trigger>
        <NBadge :value="ann.hideRules.length" :max="99" type="warning">
          <NButton size="tiny" quaternary>🙈 隐藏</NButton>
        </NBadge>
      </template>
      <div class="hide-menu">
        <p class="hint">行级隐藏（会话级，右键日志行可加）</p>
        <ul v-if="ann.hideRules.length">
          <li v-for="r in ann.hideRules" :key="r">
            <span class="rule">{{ r }}</span>
            <span class="del" @click="ann.removeHideRule(r)">&times;</span>
          </li>
        </ul>
        <NButton v-if="ann.hideRules.length" size="tiny" quaternary type="warning" @click="ann.clearHideRules()">
          清空全部
        </NButton>
      </div>
    </NPopover>
    <NDropdown
      trigger="click"
      placement="bottom-start"
      :options="fieldsOptions"
      @select="(key: string) => toggleField(key)"
    >
      <NButton size="tiny" quaternary>👁 字段</NButton>
    </NDropdown>
    <NSelect
      size="tiny"
      class="level"
      :value="props.levelMode"
      :options="levelOptions"
      @update:value="(v: string) => emit('update:levelMode', v as LevelFilterMode)"
    />
    <NButton
      size="tiny"
      :quaternary="!tabs.active?.info.follow"
      :type="tabs.active?.info.follow ? 'success' : 'default'"
      @click="onFollow(!tabs.active?.info.follow)"
    >
      {{ tabs.active?.info.follow ? '● Follow' : 'Follow' }}
    </NButton>
    <div class="spacer"></div>
    <NButton size="tiny" quaternary @click="zoom(-1)">A-</NButton>
    <span class="fs">{{ ui.fontSize }}</span>
    <NButton size="tiny" quaternary @click="zoom(1)">A+</NButton>
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  border-bottom: 1px solid var(--popup-border, #1e2837);
}
.search {
  max-width: 320px;
  min-width: 200px;
}
.level {
  width: 100px;
}
.spacer {
  flex: 1;
}
.fs {
  font-size: 11px;
  color: var(--dim, #8b98a9);
  min-width: 18px;
  text-align: center;
}
.hide-menu {
  width: 240px;
  font-size: 12px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.hint {
  color: var(--dim, #8b98a9);
  font-size: 11px;
  margin: 0;
}
.hide-menu ul {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 180px;
  overflow-y: auto;
}
.hide-menu li {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 2px 0;
}
.rule {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.del {
  cursor: pointer;
  color: var(--dim, #8b98a9);
}
.del:hover {
  color: var(--fg, #fff);
}
</style>
