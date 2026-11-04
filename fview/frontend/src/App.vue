<script setup lang="ts">
// fview 主壳：顶栏 tab 条 + 图标条（Task 9 接入源面板）+ 主区（工具栏/日志区/状态栏）。
// 搜索态/级别档位/字段显隐在此接线并下传；vim 键（j/k///n/N）挂 document 键盘层。
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useThemeStore } from './themes'
import { useTabsStore } from './stores/tabs'
import { useUiStore } from './stores/ui'
import { useFieldsStore } from './stores/fields'
import { clearTab, foldRanges, subscribeTabEvents } from './lib/backend'
import { fieldsWidths } from './lib/columns'
import { effectiveFields } from './lib/fieldvis'
import { loadOverrides, persistOverrides } from './lib/fieldOverride'
import { resolveKey } from './lib/keymap'
import type { KeyAction } from './lib/keymap'
import type { LevelFilterMode } from './lib/filter'
import TabBar from './components/tabs/TabBar.vue'
import Welcome from './components/Welcome.vue'
import SidebarRail from './components/sources/SidebarRail.vue'
import SettingsDialog from './components/dialogs/SettingsDialog.vue'
import BrowseDialog from './components/sources/BrowseDialog.vue'
import { closeTab, onShortcutCloseTab } from './lib/backend'
import Toolbar from './components/logview/Toolbar.vue'
import SearchBar from './components/logview/SearchBar.vue'
import WordsPopup from './components/logview/WordsPopup.vue'
import LogView from './components/logview/LogView.vue'
import StatusBar from './components/logview/StatusBar.vue'
import HelpModal from './components/logview/HelpModal.vue'
import { useSearchStore } from './stores/searchStore'

const theme = useThemeStore()
const tabs = useTabsStore()
const ui = useUiStore()
const searchStore = useSearchStore()

// naive overrides 合并界面字号(第九轮验收③):弹窗/按钮等 Naive 组件字号跟随
// ui.uiFontSize——theme.overrides 由主题色生成,此处浅合并不侵入 themes store。
const themeOverrides = computed(() => ({
  ...theme.overrides,
  common: { ...theme.overrides?.common, fontSize: `${ui.uiFontSize}px` },
}))

// ---- 跨组件交互状态（App 为接线员） ----
const search = reactive({ hitSeqs: [] as number[], words: [] as string[] })
const hitSeqSet = ref(new Set<number>())
const levelMode = ref<LevelFilterMode>('A')
// fieldsStore 为 rules.yaml fields 前端正本(默认层+列宽)
const fieldsStore = useFieldsStore()
/** overrides 单日志字段覆盖(localStorage 按源全称键控;同源多 tab 共享视为特性)。 */
const overrides = ref<Record<string, string[]>>(loadOverrides())
/** configuredFields 当前 tab 未裁剪配置:覆盖 ?? 默认层。 */
const configuredFields = computed(() => {
  const src = tabs.active?.info.info ?? ''
  return overrides.value[src] ?? fieldsStore.defaults
})
/** fields 实际显示列 = configuredFields ∩ 当前 tab 解析器能力(列按能力显隐,null 不裁剪)。 */
const fields = computed(() => effectiveFields(configuredFields.value, tabs.active?.parserFields ?? null))
const colWidths = computed(() => fieldsWidths(fieldsStore.all, fields.value))

const logviewRef = ref<InstanceType<typeof LogView> | null>(null)
const toolbarRef = ref<InstanceType<typeof Toolbar> | null>(null)
const railRef = ref<InstanceType<typeof SidebarRail> | null>(null)

// 设置中心（Task 10 接入对话框，先占位开关）
const settingsShow = ref(false)
/** copied y 复制反馈行数(3s 闪现)。 */
const copied = ref(0)
/** 帮助弹窗(? 键)。 */
const helpShow = ref(false)

function onSearch(payload: { hitSeqs: number[]; words: string[] }) {
  search.hitSeqs = payload.hitSeqs
  search.words = payload.words
  hitSeqSet.value = new Set(payload.hitSeqs)
}

// 切 tab 清空搜索过滤(final review I4):seq 是 per-tab 域,旧 tab 的命中集
// 会串扰新 tab 的行过滤;输入词 q 留在 SearchBar 不动(切回重按回车即可)。
watch(
  () => tabs.activeId,
  () => onSearch({ hitSeqs: [], words: [] })
)

/** onColWidths 表头拖拽列宽落盘(第七轮验收②):走 fieldsStore 单源写回
 *  rules.yaml fields[].width(TUI 同源生效);失败 store 内回滚保本地宽。 */
function onColWidths(w: Record<string, number>) {
  fieldsStore.setWidths(w).catch(() => {
    /* 落盘失败保本地宽,下次拖拽重试 */
  })
}

/** onOverrideFields 工具栏 F 覆盖写回 localStorage:null=恢复默认(删覆盖键)。 */
function onOverrideFields(v: string[] | null) {
  const src = tabs.active?.info.info
  if (!src) return
  if (v === null) delete overrides.value[src]
  else overrides.value[src] = v
  persistOverrides(overrides.value)
}

// ---- 字体变量注入（日志区字号/等宽字体走 CSS 变量;界面字号单源 --ui-font-size） ----
watch(
  () => [ui.fontSize, ui.fontFamily, ui.lineHeight, ui.uiFontSize],
  () => {
    const root = document.documentElement
    root.style.setProperty('--log-font-size', `${ui.fontSize}px`)
    root.style.setProperty('--font-mono', ui.fontFamily)
    root.style.setProperty('--log-line-height', String(ui.lineHeight))
    root.style.setProperty('--ui-font-size', `${ui.uiFontSize}px`)
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
/** onMouseMove 键盘导航态复位(藏指针/悬停抑制随之解除)。位移 <2px 的 mousemove
 *  视为噪声不解除(滚动期间指针重算/触摸板抖动会派发零位移事件,真机首验踩坑)——
 *  真移动必超阈值,防误复位于未然;bool 判断 O(1),已复位时零开销。 */
let lastMX = -1
let lastMY = -1
function onMouseMove(e: MouseEvent) {
  if (lastMX >= 0 && Math.abs(e.clientX - lastMX) < 2 && Math.abs(e.clientY - lastMY) < 2) return
  lastMX = e.clientX
  lastMY = e.clientY
  if (ui.kbNav) ui.kbNav = false
}
function onKeydown(e: KeyboardEvent) {
  if (isTyping(e)) return
  // 弹窗打开时全局键让位,由各弹窗自理(ESC 关闭等,第三轮验收②)——
  // 否则 ESC 走 exitVisual、j/k 穿透弹窗动日志光标
  if (ui.modalOpen) return
  // 键盘导航态:vim 键即藏鼠标指针+抑制悬停底色,mousemove 复位(less/vim 同款)
  ui.kbNav = true
  // +x/-x 上下文混入键序列(有状态数字累积,不进 keymap 纯映射)——
  // bookmarkNav 同款"返回是否消费":输入态吞键,非输入态仅过滤激活时认领 +/-
  if (logviewRef.value?.ctxKey(e)) {
    e.preventDefault()
    return
  }
  const { action, arg } = resolveKey(e, keyState)
  if (action === 'none' || action === 'pending') return
  // ⌘C/Ctrl+C 无选区放行系统(输入框已前置让位;有选区才拦截走 yankSelection)
  if (action === 'copySelection' && !logviewRef.value?.hasSelection()) return
  // 阻止浏览器默认行为(/ 快速搜索、ctrl+w/数字等系统键)
  if (
    action === 'search' ||
    action === 'highlight' ||
    action === 'hide' ||
    action === 'closeTab' ||
    action === 'tabSwitch' ||
    action === 'copySelection' ||
    action === 'zoomIn' ||
    action === 'zoomOut' ||
    action === 'navDown' ||
    action === 'navUp'
  ) {
    e.preventDefault()
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
      // SLS 式常驻条(第六轮):/、f 聚焦输入框
      searchStore.focusBar()
      break
    case 'highlight':
      searchStore.openWords('hl')
      break
    case 'hide':
      searchStore.openWords('hide')
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
      // 混入态不可视选择(TUI 同语义:ctx 行域与过滤域两套坐标混淆)
      if (lv?.hasCtxMix()) break
      lv?.startVisual(false)
      break
    case 'visualLine':
      if (lv?.hasCtxMix()) break
      lv?.startVisual(true)
      break
    case 'yank': {
      const n = lv?.yank() ?? 0
      if (n > 0) {
        copied.value = n
        setTimeout(() => (copied.value = 0), 3000)
      }
      break
    }
    // ⌘C/Ctrl+C:有选区才吞(拦截在 onKeydown 已判),此处只管复制
    case 'copySelection': {
      const n = lv?.yankSelection() ?? 0
      if (n > 0) {
        copied.value = n
        setTimeout(() => (copied.value = 0), 3000)
      }
      break
    }
    case 'escape':
      lv?.exitVisual()
      break
    // ---- 全键盘操作(第四轮验收③) ----
    case 'sources':
      railRef.value?.cyclePane()
      break
    case 'settings':
      settingsShow.value = !settingsShow.value
      break
    case 'activate':
      lv?.bookmarkActivate()
      break
    case 'closeTab': {
      const id = tabs.activeId
      if (id) closeTab(id).then(() => tabs.remove(id)).catch(() => {})
      break
    }
    case 'tabSwitch': {
      const i = Number(arg) - 1
      if (tabs.tabs[i]) tabs.activeId = tabs.tabs[i].info.id
      break
    }
    case 'zoomIn':
      ui.fontSize = Math.min(28, ui.fontSize + 1)
      ui.persist()
      break
    case 'zoomOut':
      ui.fontSize = Math.max(9, ui.fontSize - 1)
      ui.persist()
      break
    case 'navDown':
      lv?.bookmarkNav(1)
      break
    case 'navUp':
      lv?.bookmarkNav(-1)
      break
    case 'lineNumbers':
      lv?.toggleLineNum()
      break
    case 'help':
      helpShow.value = !helpShow.value
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
    case 'wrap':
      ui.wrap = !ui.wrap
      break
    default:
      break
  }
}

onMounted(async () => {
  theme.init()
  // mac 菜单 ⌘W → 关当前 tab（原生菜单路由,不经 keydown,故不受 modalOpen
  // 让位约束——浏览器惯例:弹窗开着 ⌘W 照样关）
  onShortcutCloseTab(() => dispatchKey('closeTab'))
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
  // rules.yaml fields 拉取进 fieldsStore 单源(默认层+列宽;hides 是行级隐藏词不得混用)
  fieldsStore.load()
  document.addEventListener('keydown', onKeydown)
  document.addEventListener('mousemove', onMouseMove)
})
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown)
  document.removeEventListener('mousemove', onMouseMove)
})
</script>

<template>
  <n-config-provider :theme="theme.naiveTheme" :theme-overrides="themeOverrides">
    <div class="app">
      <header class="topbar">
        <TabBar />
        <!-- 设置入口:标题栏右端(IDEA 式;rail 底部旧入口已撤,验收反馈) -->
        <button class="topbar-settings" title="设置" @click="settingsShow = true">
          <svg viewBox="0 0 24 24" class="ico">
            <circle cx="12" cy="12" r="3" />
            <path
              d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"
            />
          </svg>
        </button>
      </header>
      <div class="body">
        <SidebarRail ref="railRef" />
        <main class="main">
          <Welcome v-if="tabs.tabs.length === 0" />
          <template v-else>
            <Toolbar
              ref="toolbarRef"
              :configured="configuredFields"
              @override-fields="onOverrideFields"
            />
            <SearchBar
              :hit-count="search.hitSeqs.length"
              :level-mode="levelMode"
              @search="onSearch"
              @update:level-mode="(m: LevelFilterMode) => (levelMode = m)"
            />
            <LogView
              ref="logviewRef"
              :hit-seqs="hitSeqSet"
              :hit-words="search.words"
              :level-mode="levelMode"
              :fields="fields"
              :widths="colWidths"
              :source-col="tabs.active?.hasSource ?? false"
              @update:widths="onColWidths"
            />
            <StatusBar :copied="copied" />
          </template>
        </main>
      </div>
    </div>
    <SettingsDialog v-model:show="settingsShow" />
    <!-- 全局目录浏览弹窗（SSH/FRP 面板与欢迎页最近源共用单例，ui.browse 驱动） -->
    <BrowseDialog
      :show="ui.browse.show"
      :mode="ui.browse.mode"
      :host="ui.browse.host"
      :conn-name="ui.browse.connName"
      :init-path="ui.browse.initPath"
      @update:show="(v: boolean) => !v && ui.closeBrowse()"
    />
    <WordsPopup v-if="tabs.tabs.length" />
    <HelpModal v-model:show="helpShow" />
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
  /* 标题栏融合:左侧 78px 让位红绿灯;30px 折中——交通灯中心 14px(系统固定),
     28px 严格居中但偏矮,32px 偏上 2px 用户可感,30px 偏 1px 肉眼极限 */
  padding: 4px 8px 4px 78px;
  min-height: 30px;
  box-sizing: border-box;
  background: var(--popup-bg, #0c1119);
  border-bottom: 1px solid var(--popup-border, #1c2836);
  /* 整行可拖:wails dragTest 只查 e.target 自身,空白区 target=topbar 即拖;
     tab/按钮 target 是其子元素(无声明)不受影响 */
  --wails-draggable: drag;
}
.topbar-settings {
  flex: none;
  width: 26px;
  height: 26px;
  border: none;
  border-radius: 6px;
  background: transparent;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--dim, #8b98a9);
}
.topbar-settings:hover {
  color: var(--fg, #e6e6e6);
  background: var(--popup-border, #1f2c3f);
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
