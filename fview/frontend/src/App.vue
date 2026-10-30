<script setup lang="ts">
// fview 主壳：顶栏 tab 条 + 图标条（Task 9 接入源面板）+ 主区（工具栏/日志区/状态栏）。
// 搜索态/级别档位/字段显隐在此接线并下传；vim 键（j/k///n/N）挂 document 键盘层。
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useThemeStore } from './themes'
import { useTabsStore } from './stores/tabs'
import { useUiStore } from './stores/ui'
import { getFields, subscribeTabEvents } from './lib/backend'
import type { FieldConfig } from './lib/backend'
import { fieldsWidths } from './lib/columns'
import type { LevelFilterMode } from './lib/filter'
import TabBar from './components/tabs/TabBar.vue'
import Welcome from './components/Welcome.vue'
import SidebarRail from './components/sources/SidebarRail.vue'
import SettingsDialog from './components/dialogs/SettingsDialog.vue'
import Toolbar from './components/logview/Toolbar.vue'
import LogView from './components/logview/LogView.vue'
import StatusBar from './components/logview/StatusBar.vue'
import SearchPopup from './components/logview/SearchPopup.vue'
import { useSearchStore } from './stores/searchStore'

const theme = useThemeStore()
const tabs = useTabsStore()
const ui = useUiStore()
const searchStore = useSearchStore()

// ---- 跨组件交互状态（App 为接线员） ----
const ALL_FIELDS = ['time', 'level', 'thread', 'traceId', 'logger', 'message']
const search = reactive({ hitSeqs: [] as number[], words: [] as string[] })
const hitSeqSet = ref(new Set<number>())
const levelMode = ref<LevelFilterMode>('A')
const fields = ref<string[]>([...ALL_FIELDS])
/** allFieldsCfg 字段全量配置(含 rules.yaml 列宽覆盖),分栏 widths 数据源。 */
const allFieldsCfg = ref<FieldConfig[]>([])
const colWidths = computed(() => fieldsWidths(allFieldsCfg.value, fields.value))

const logviewRef = ref<InstanceType<typeof LogView> | null>(null)

// 设置中心（Task 10 接入对话框，先占位开关）
const settingsShow = ref(false)

function onSearch(payload: { hitSeqs: number[]; words: string[] }) {
  search.hitSeqs = payload.hitSeqs
  search.words = payload.words
  hitSeqSet.value = new Set(payload.hitSeqs)
}

// ---- 字体变量注入（日志区字号/等宽字体走 CSS 变量） ----
watch(
  () => [ui.fontSize, ui.fontFamily, ui.lineHeight],
  () => {
    const root = document.documentElement
    root.style.setProperty('--log-font-size', `${ui.fontSize}px`)
    root.style.setProperty('--font-mono', ui.fontFamily)
    root.style.setProperty('--log-line-height', String(ui.lineHeight))
  },
  { immediate: true }
)

// ---- vim 键（输入控件聚焦时让位） ----
function isTyping(e: KeyboardEvent): boolean {
  const t = e.target as HTMLElement | null
  if (!t) return false
  return (
    t.tagName === 'INPUT' ||
    t.tagName === 'TEXTAREA' ||
    t.isContentEditable
  )
}
function onKeydown(e: KeyboardEvent) {
  if (isTyping(e)) return
  if (e.key === 'j') {
    logviewRef.value?.scrollLines(3)
  } else if (e.key === 'k') {
    logviewRef.value?.scrollLines(-3)
  } else if (e.key === 'g') {
    logviewRef.value?.scrollToEnd(true)
  } else if (e.key === 'G') {
    logviewRef.value?.scrollToEnd(false)
  } else if (e.key === '/' || e.key === 'f') {
    // 统一搜索弹窗·搜索分区(TUI /、f 同款;h 高亮、x 隐藏,验收反馈③④)
    e.preventDefault()
    searchStore.open(0)
  } else if (e.key === 'h') {
    e.preventDefault()
    searchStore.open(1)
  } else if (e.key === 'x') {
    e.preventDefault()
    searchStore.open(2)
  } else if (e.key === 'n' && !e.shiftKey) {
    logviewRef.value?.jumpHit(1)
  } else if (e.key === 'N' || (e.key === 'n' && e.shiftKey)) {
    logviewRef.value?.jumpHit(-1)
  } else if (e.key === 'e') {
    levelMode.value = 'E' // 级别档位（对齐 TUI/m4m6：e/w/i/a/d）
  } else if (e.key === 'w') {
    levelMode.value = 'W'
  } else if (e.key === 'i') {
    levelMode.value = 'I'
  } else if (e.key === 'a') {
    levelMode.value = 'A'
  } else if (e.key === 'd') {
    levelMode.value = 'D'
  }
}

onMounted(async () => {
  theme.init()
  subscribeTabEvents({
    onOpened: tabs.applyOpened,
    onLines: tabs.applyLines,
    onMeta: tabs.applyMeta,
    onClosed: (ev) => tabs.remove(ev.tabId),
  })
  // 字段显隐按 rules.yaml 的 fields.Visible 校正（与 TUI 同源；hides 是行级隐藏词不得混用）
  try {
    const fs = (await getFields()) ?? []
    allFieldsCfg.value = fs
    const visible = new Set(fs.filter((f) => f.visible).map((f) => f.name))
    if (visible.size > 0) {
      fields.value = ALL_FIELDS.filter((f) => visible.has(f))
    }
  } catch {
    /* 默认全显 */
  }
  document.addEventListener('keydown', onKeydown)
})
onBeforeUnmount(() => document.removeEventListener('keydown', onKeydown))
</script>

<template>
  <n-config-provider :theme="theme.naiveTheme" :theme-overrides="theme.overrides">
    <div class="app">
      <header class="topbar">
        <TabBar />
      </header>
      <div class="body">
        <SidebarRail @open-settings="settingsShow = true" />
        <main class="main">
          <Welcome v-if="tabs.tabs.length === 0" />
          <template v-else>
            <Toolbar
              v-model:level-mode="levelMode"
              :fields="fields"
              @update:fields="(f: string[]) => (fields = f)"
            />
            <LogView
              ref="logviewRef"
              :hit-seqs="hitSeqSet"
              :hit-words="search.words"
              :level-mode="levelMode"
              :fields="fields"
              :widths="colWidths"
            />
            <StatusBar :hit-count="search.hitSeqs.length" />
          </template>
        </main>
      </div>
    </div>
    <SettingsDialog v-model:show="settingsShow" />
    <SearchPopup v-if="tabs.tabs.length" @search="onSearch" />
  </n-config-provider>
</template>

<style scoped>
.app {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--bg, #10161f);
  color: var(--fg, #e6e6e6);
}
.topbar {
  display: flex;
  align-items: center;
  padding: 4px 8px;
  background: var(--popup-bg, #0c1119);
  border-bottom: 1px solid var(--popup-border, #1c2836);
}
.body {
  flex: 1;
  display: flex;
  min-height: 0;
}
.rail {
  width: 40px;
  background: var(--popup-bg, #141b26);
  border-right: 1px solid var(--popup-border, #2a3547);
}
.main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
</style>
