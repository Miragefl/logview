<script setup lang="ts">
// fview 主壳：顶栏 tab 条 + 图标条（Task 9 接入源面板）+ 主区（工具栏/日志区/状态栏）。
// 搜索态/级别档位/字段显隐在此接线并下传；vim 键（j/k///n/N）挂 document 键盘层。
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useThemeStore } from './themes'
import { useTabsStore } from './stores/tabs'
import { useUiStore } from './stores/ui'
import { clearTab, foldRanges, getFields, subscribeTabEvents } from './lib/backend'
import type { FieldConfig } from './lib/backend'
import { fieldsWidths } from './lib/columns'
import { resolveKey } from './lib/keymap'
import type { KeyAction } from './lib/keymap'
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
const toolbarRef = ref<InstanceType<typeof Toolbar> | null>(null)

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
// ---- vim 键位机(验收反馈④):resolveKey 纯映射 + dispatch 接线 ----
const keyState = { pendingZ: false }
function onKeydown(e: KeyboardEvent) {
  if (isTyping(e)) return
  const { action, arg } = resolveKey(e, keyState)
  if (action === 'none' || action === 'pending') return
  if (action === 'search' || action === 'highlight' || action === 'hide') {
    e.preventDefault() // 阻止浏览器快速搜索(/)等默认行为
  }
  dispatchKey(action, arg)
}

function dispatchKey(action: KeyAction, arg?: string) {
  const lv = logviewRef.value
  switch (action) {
    case 'cursorDown':
      lv?.moveCursor(1)
      break
    case 'cursorUp':
      lv?.moveCursor(-1)
      break
    case 'gotoTop':
      lv?.gotoTop()
      break
    case 'gotoBottom':
      lv?.gotoBottom()
      break
    case 'viewportTop':
      lv?.cursorToViewport('top')
      break
    case 'viewportMid':
      lv?.cursorToViewport('mid')
      break
    case 'viewportBottom':
      lv?.cursorToViewport('bottom')
      break
    case 'anchorTop':
      lv?.anchorCursor('t')
      break
    case 'anchorMid':
      lv?.anchorCursor('z')
      break
    case 'anchorBottom':
      lv?.anchorCursor('b')
      break
    case 'halfDown':
      lv?.halfPage(1)
      break
    case 'halfUp':
      lv?.halfPage(-1)
      break
    case 'pageDown':
      lv?.pageMove(1)
      break
    case 'pageUp':
      lv?.pageMove(-1)
      break
    case 'nextHit':
      lv?.cursorToHit(1)
      break
    case 'prevHit':
      lv?.cursorToHit(-1)
      break
    case 'search':
      searchStore.open(0)
      break
    case 'highlight':
      searchStore.open(1)
      break
    case 'hide':
      searchStore.open(2)
      break
    case 'level':
      // 级别档位 = 大写 E/W/I/D/A(小写已释放给折叠/换行/详情,对齐 TUI)
      if (arg) levelMode.value = arg as LevelFilterMode
      break
    case 'detail':
      lv?.openDetailAtCursor()
      break
    case 'fields':
      toolbarRef.value?.toggleFields()
      break
    case 'fold':
      lv?.toggleFold()
      break
    case 'visual':
      lv?.startVisual(false)
      break
    case 'visualLine':
      lv?.startVisual(true)
      break
    case 'yank':
      lv?.yankSelection()
      break
    case 'bookmark':
      lv?.toggleBookmark()
      break
    case 'gotoBookmarks':
      lv?.showBookmarks()
      break
    case 'clear': {
      const id = tabs.activeId
      if (id) {
        // 本地立即清(后端清完广播 tab:cleared 再清一次,幂等)
        tabs.applyCleared(id)
        onSearch({ hitSeqs: [], words: [] })
        clearTab(id).catch(() => {})
      }
      break
    }
    // wrap(T10)/lineNumbers(T9)/help(T9):后续任务实装,先占位
    default:
      break
  }
}

onMounted(async () => {
  theme.init()
  subscribeTabEvents({
    onOpened: (ev) => {
      tabs.applyOpened(ev)
      // 折叠组懒拉补一次全量(后续靠 tab:folds 增量推送)
      foldRanges(ev.tabId)
        .then((rs) => tabs.applyFolds({ tabId: ev.tabId, ranges: rs }))
        .catch(() => {})
    },
    onLines: tabs.applyLines,
    onMeta: tabs.applyMeta,
    onClosed: (ev) => tabs.remove(ev.tabId),
    onFolds: tabs.applyFolds,
    onCleared: (ev) => {
      const wasActive = tabs.activeId === ev.tabId
      tabs.applyCleared(ev.tabId)
      if (wasActive) onSearch({ hitSeqs: [], words: [] })
    },
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
              ref="toolbarRef"
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
