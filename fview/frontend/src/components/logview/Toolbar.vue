<script setup lang="ts">
// 日志工具栏:统一搜索入口(搜索/高亮/隐藏三合一弹窗,验收反馈③)+
// 字段显隐/级别档位/Follow/字号。搜索输入与历史全部收进 SearchPopup。
import { computed, onMounted, ref } from 'vue'
import { NBadge, NButton, NDropdown, NSelect } from 'naive-ui'
import { useTabsStore } from '../../stores/tabs'
import { useAnnotationsStore } from '../../stores/annotations'
import { useSearchStore } from '../../stores/searchStore'
import { useUiStore } from '../../stores/ui'
import type { LevelFilterMode } from '../../lib/filter'
import {
  getFields,
  saveFields,
  setFollow,
} from '../../lib/backend'
import type { FieldConfig } from '../../lib/backend'

const emit = defineEmits<{
  (e: 'update:levelMode', mode: LevelFilterMode): void
  (e: 'update:fields', fields: string[]): void
}>()

const props = defineProps<{ levelMode: LevelFilterMode; fields: string[] }>()

const tabs = useTabsStore()
const ann = useAnnotationsStore()
const search = useSearchStore()
const ui = useUiStore()

/** openSearch 供 vim 键 /、f、h、x 打开统一弹窗并定位分区(0/1/2)。 */
function openSearch(tab = 0) {
  search.open(tab)
}

/** 字段下拉受控显隐(F 键入口)。 */
const fieldsShow = ref(false)
function toggleFields() {
  fieldsShow.value = !fieldsShow.value
}
defineExpose({ openSearch, toggleFields })

// ---- 字段显隐（rules.yaml fields 同源） ----
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
    <NBadge :value="ann.highlights.length + ann.hideRules.length" :max="99" type="info">
      <NButton size="tiny" quaternary @click="openSearch(0)">🔍 搜索</NButton>
    </NBadge>
    <NDropdown
      trigger="manual"
      placement="bottom-start"
      :show="fieldsShow"
      :options="fieldsOptions"
      @clickoutside="fieldsShow = false"
      @select="
        (key: string) => {
          toggleField(key)
          fieldsShow = false
        }
      "
    >
      <NButton size="tiny" quaternary @click="toggleFields">👁 字段</NButton>
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
</style>
