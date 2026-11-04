<script setup lang="ts">
// 常驻搜索条(第六轮 SLS 式改造):查询输入(回车执行/清空回车恢复全量,执行成功即失焦)+
// 级别档位 + 命中数 + 高亮/隐藏按钮(弹 WordsPopup)。/ 键聚焦,Esc 失焦不清词。
import { nextTick, ref, watch } from 'vue'
import { NBadge, NButton, NInput, NSelect } from 'naive-ui'
import { useTabsStore } from '../../stores/tabs'
import { useAnnotationsStore } from '../../stores/annotations'
import { useSearchStore } from '../../stores/searchStore'
import { matchLines, parseQuery } from '../../lib/backend'
import type { LevelFilterMode } from '../../lib/filter'

const props = defineProps<{
  /** 命中行数(搜索过滤后) */
  hitCount: number
  /** 级别档位(受控,E/W/I/D/A 键与下拉双入口) */
  levelMode: LevelFilterMode
}>()

const emit = defineEmits<{
  (e: 'search', payload: { hitSeqs: number[]; words: string[] }): void
  (e: 'update:levelMode', mode: LevelFilterMode): void
}>()

const tabs = useTabsStore()
const ann = useAnnotationsStore()
const search = useSearchStore()

const levelOptions = [
  { label: '全部', value: 'A' },
  { label: '≥INFO', value: 'I' },
  { label: '≥WARN', value: 'W' },
  { label: 'ERROR', value: 'E' },
  { label: 'DEBUG', value: 'D' },
]

const q = ref('')
const qErr = ref('')
const inputEl = ref<InstanceType<typeof NInput> | null>(null)

// / 键聚焦(store 驱动;重复 / 幂等——已是聚焦态不再重复设 true)
watch(
  () => search.barFocused,
  (v) => {
    if (v) nextTick(() => inputEl.value?.focus())
  }
)

/** confirmSearch 回车执行:parseQuery 校验 → matchLines 索引域换算 seq 域 → emit。
 * 清空 query 直接 emit 空集恢复全量(不调后端)。
 * 执行成功即失焦,焦点交还日志区(j/k 导航回归;再搜按 / 或点击输入框);
 * parseError/异常路径不失焦——留在框内改词,blur 会把错误提示带离视线。 */
async function confirmSearch() {
  const t = tabs.active
  if (!t) return
  const query = q.value.trim()
  qErr.value = ''
  try {
    if (!query) {
      emit('search', { hitSeqs: [], words: [] })
      inputEl.value?.blur()
      return
    }
    const meta = await parseQuery(query)
    if (meta.parseError) {
      qErr.value = meta.parseError
      return
    }
    // MatchLines 返回 buffer 索引域——镜像与后端同裁稳态对齐,换算 seq 域下发
    const idxs = await matchLines(t.info.id, query)
    const seqs = idxs.map((i) => t.lines[i]?.seq).filter((s): s is number => s !== undefined)
    search.addSearchHistory(query)
    emit('search', { hitSeqs: seqs, words: meta.highlightKeywords ?? [] })
    inputEl.value?.blur()
  } catch (e) {
    qErr.value = String(e).replace(/^Error:\s*/, '')
  }
}

// ---- 历史下拉(ctrl+R,键盘流迁自 SearchPopup) ----
const HIST_MAX = 8
const histShow = ref(false)
const histWords = ref<string[]>([])
const navIdx = ref(-1)
const histEls = ref<HTMLElement[]>([])

async function toggleHist() {
  if (histShow.value) {
    histShow.value = false
    return
  }
  histWords.value = search.history.slice(0, HIST_MAX)
  histShow.value = true
  navIdx.value = -1
}

function applyHist(w: string) {
  q.value = w
  histShow.value = false
  navIdx.value = -1
  inputEl.value?.focus()
}

function navMove(delta: number) {
  const n = histWords.value.length
  if (n === 0) return
  navIdx.value = Math.min(n - 1, Math.max(0, navIdx.value < 0 ? (delta > 0 ? 0 : n - 1) : navIdx.value + delta))
  histEls.value[navIdx.value]?.scrollIntoView({ block: 'nearest' })
}

/** onBlur 失焦回写聚焦态 + 收历史浮层(final review C1/I3):
 *  barFocused 单向同步会让"点击别处后再按 /"同值赋值不触发 watch → / 键死锁;
 *  历史浮层非模态,失焦不收会残留遮挡日志区。 */
function onBlur() {
  search.blurBar()
  histShow.value = false
  navIdx.value = -1
}

/** onKey 输入框键流:Enter 执行或应用历史 / Esc 失焦不清词 / ctrl+R 历史下拉。 */
function onKey(e: KeyboardEvent) {
  if (e.key === 'Enter') {
    e.preventDefault()
    if (histShow.value && navIdx.value >= 0) {
      const w = histWords.value[navIdx.value]
      if (w !== undefined) applyHist(w)
    } else {
      void confirmSearch()
    }
  } else if (e.key === 'Escape') {
    e.preventDefault()
    if (histShow.value) {
      histShow.value = false
      navIdx.value = -1
    } else {
      search.blurBar()
      inputEl.value?.blur()
    }
  } else if (e.key === 'r' && (e.ctrlKey || e.metaKey)) {
    e.preventDefault()
    navIdx.value = -1
    void toggleHist()
  } else if (e.ctrlKey && e.key === 'j') {
    e.preventDefault()
    navMove(1)
  } else if (e.ctrlKey && e.key === 'k') {
    e.preventDefault()
    navMove(-1)
  }
}

// 高亮/隐藏词入共享词频(管理框内加词时由 WordsPopup 记,此处按钮只开关)
</script>

<template>
  <div class="searchbar">
    <NInput
      ref="inputEl"
      v-model:value="q"
      size="small"
      clearable
      class="q"
      :placeholder="qErr ? qErr : '搜索(time:/level: 语法),回车执行 · ctrl+R 历史'"
      :status="qErr ? 'error' : undefined"
      @keydown="onKey"
      @blur="onBlur"
    />
    <!-- 历史下拉(ctrl+R) -->
    <ul v-if="histShow" class="hist">
      <li
        v-for="(w, i) in histWords"
        :key="w"
        :ref="(el) => (histEls[i] = el as HTMLElement)"
        :class="{ sel: i === navIdx }"
        @click="applyHist(w)"
        @mousemove="navIdx = i"
      >
        {{ w }}
      </li>
      <li v-if="!histWords.length" class="empty">暂无历史</li>
    </ul>
    <NSelect
      size="small"
      class="level"
      :value="props.levelMode"
      :options="levelOptions"
      @update:value="(v: string) => emit('update:levelMode', v as LevelFilterMode)"
    />
    <span v-if="hitCount > 0" class="hits">命中 {{ hitCount }}</span>
    <NBadge :value="ann.highlights.length" :max="99" type="info" :show="ann.highlights.length > 0">
      <NButton size="tiny" quaternary @click="search.openWords('hl')">
        <!-- 马克笔+底线:高亮意象(描边 SVG,与 SidebarRail 同风格——emoji 弃用,验收反馈⑦) -->
        <svg viewBox="0 0 24 24" class="ico">
          <path d="M12 3l5 5-9 9H4v-4l8-10z" />
          <path d="M14 20h6" />
        </svg>
        高亮
      </NButton>
    </NBadge>
    <NBadge :value="ann.hideRules.length" :max="99" type="info" :show="ann.hideRules.length > 0">
      <NButton size="tiny" quaternary @click="search.openWords('hide')">
        <!-- 眼睛+斜杠:隐藏意象 -->
        <svg viewBox="0 0 24 24" class="ico">
          <path d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12z" />
          <circle cx="12" cy="12" r="3" />
          <path d="M4 4l16 16" />
        </svg>
        隐藏
      </NButton>
    </NBadge>
  </div>
</template>

<style scoped>
.searchbar {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 10px;
  border-bottom: 1px solid var(--popup-border, #1e2837);
}
.q {
  flex: 1;
  min-width: 0;
}
.level {
  width: 96px;
  flex: none;
}
.hits {
  flex: none;
  font-size: var(--font-ui-sm, 11px);
  color: var(--dim, #8b98a9);
  white-space: nowrap;
}
/* 描边图标:与 SidebarRail 同款(22px stroke currentColor) */
/* 图标基类收编 style.css 全局正本(2026-11-03 统一),此处仅留对齐布局附加 */
.ico {
  vertical-align: -3px;
  margin-right: 2px;
}
/* 历史下拉:输入框下方浮层 */
.hist {
  position: absolute;
  top: 100%;
  left: 10px;
  width: 50%;
  min-width: 320px;
  z-index: 40;
  list-style: none;
  margin: 2px 0 0;
  padding: 4px;
  background: var(--popup-bg, #16202e);
  border: 1px solid var(--popup-border, #2a3a4f);
  border-radius: 6px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
  font-size: var(--font-ui-base, 12px);
  max-height: 220px;
  overflow-y: auto;
}
.hist li {
  padding: 3px 8px;
  border-radius: 3px;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.hist li:hover,
.hist li.sel {
  background: var(--popup-bg, #1f2c3f);
}
.hist li.sel {
  box-shadow: inset 2px 0 0 var(--accent, #4a8fe0);
}
.hist .empty {
  color: var(--dim, #5d7292);
  cursor: default;
}
</style>
