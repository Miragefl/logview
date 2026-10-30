<script setup lang="ts">
// 统一搜索弹窗(验收反馈③):搜索/高亮/隐藏三合一,TUI searchpopup 的桌面版。
// - Tab 键轮换分区;Enter 确认当前分区;ctrl+R 呼出关键词历史
// - 搜索=整句查询(ParseQuery 语法)会话级历史;高亮/隐藏=逗号词表,词频进
//   共享 usage.json(7 天半衰期,scope 隔离,与 TUI 同读写)
import { computed, nextTick, ref, watch } from 'vue'
import { NButton, NInput } from 'naive-ui'
import { useTabsStore } from '../../stores/tabs'
import { useAnnotationsStore } from '../../stores/annotations'
import { useSearchStore } from '../../stores/searchStore'
import { bumpKeyword, matchLines, parseQuery, usageKeywords } from '../../lib/backend'
import { scopeOfLabel } from '../../lib/scope'
import { HL_COLORS } from '../../stores/annotations'

const emit = defineEmits<{
  (e: 'search', payload: { hitSeqs: number[]; words: string[] }): void
}>()

const tabs = useTabsStore()
const ann = useAnnotationsStore()
const search = useSearchStore()

const TAB_NAMES = ['搜索', '高亮', '隐藏']
const HIST_MAX = 8

// ---- 输入态(三分区各一个词框) ----
const q = ref('')
const qErr = ref('')
const hlInput = ref('')
const hideInput = ref('')
const inputEl = ref<InstanceType<typeof NInput> | null>(null)

/** scope 词频隔离域(对齐 TUI 各源 Scope,lib/scope 单源)。 */
const scope = computed(() => scopeOfLabel(tabs.active?.info.label ?? ''))

watch(
  () => search.show,
  (show) => {
    if (show) {
      qErr.value = ''
      setTimeout(() => inputEl.value?.focus(), 50)
    } else {
      histShow.value = false
    }
  }
)
watch(
  () => search.tab,
  () => {
    histShow.value = false
    qErr.value = ''
    // 分区切换后新输入框重新聚焦(Final #5:否则键盘流断裂,Esc/j 落回全局键位)
    nextTick(() => inputEl.value?.focus())
  }
)

// ---- 历史面板(ctrl+R) ----
const histShow = ref(false)
const histWords = ref<string[]>([])
async function toggleHist() {
  if (histShow.value) {
    histShow.value = false
    return
  }
  if (search.tab === 0) {
    histWords.value = search.history.slice(0, HIST_MAX)
  } else {
    const kind = search.tab === 1 ? 'hl' : 'hide'
    histWords.value = ((await usageKeywords(scope.value, kind)) ?? []).slice(0, HIST_MAX)
  }
  histShow.value = true
}
/** applyHist 点历史条目:搜索整句替换;高亮/隐藏逗号追加。 */
function applyHist(w: string) {
  if (search.tab === 0) {
    q.value = w
  } else if (search.tab === 1) {
    hlInput.value = hlInput.value ? `${hlInput.value},${w}` : w
  } else {
    hideInput.value = hideInput.value ? `${hideInput.value},${w}` : w
  }
  histShow.value = false
  inputEl.value?.focus()
}

/** splitKeywords 逗号拆词(对齐 TUI)。 */
function splitKeywords(s: string): string[] {
  return s
    .split(/[,，]/)
    .map((w) => w.trim())
    .filter((w) => w !== '')
}

// ---- 三分区确认 ----
async function confirmSearch() {
  const t = tabs.active
  if (!t) return
  const query = q.value.trim()
  qErr.value = ''
  try {
    const meta = await parseQuery(query)
    if (meta.parseError) {
      qErr.value = meta.parseError
      return
    }
    // MatchLines 返回 buffer 索引域——镜像与后端同裁稳态对齐,换算 seq 域下发
    const idxs = query ? await matchLines(t.info.id, query) : []
    const seqs = idxs.map((i) => t.lines[i]?.seq).filter((s): s is number => s !== undefined)
    if (query) search.addSearchHistory(query)
    emit('search', { hitSeqs: seqs, words: meta.highlightKeywords ?? [] })
    search.close()
  } catch (e) {
    qErr.value = String(e)
  }
}

function confirmHl() {
  const words = splitKeywords(hlInput.value)
  for (const w of words) {
    ann.addHighlight(w)
    bumpKeyword(scope.value, 'hl', w).catch(() => {})
  }
  hlInput.value = ''
}

function confirmHide() {
  const words = splitKeywords(hideInput.value)
  for (const w of words) {
    ann.addHideRule(w)
    bumpKeyword(scope.value, 'hide', w).catch(() => {})
  }
  hideInput.value = ''
}

/** onKey 捕获弹窗内按键:Esc 关闭 / Tab 轮换 / ctrl+R 历史。 */
function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    search.close()
  } else if (e.key === 'Tab') {
    e.preventDefault()
    search.cycleTab()
  } else if (e.key === 'r' && (e.ctrlKey || e.metaKey)) {
    e.preventDefault()
    void toggleHist()
  }
}
</script>

<template>
  <Teleport to="body">
    <div v-if="search.show" class="overlay" @click.self="search.close()">
      <div class="dialog">
        <div class="dlg-title">搜索</div>
        <div class="popup" tabindex="-1" @keydown="onKey">
      <div class="tabs">
        <button
          v-for="(name, i) in TAB_NAMES"
          :key="name"
          class="tab-btn"
          :class="{ on: search.tab === i }"
          @click="search.tab = i"
        >
          {{ name }}
        </button>
        <span class="hint">Tab 轮换 · ctrl+R 历史</span>
      </div>

      <!-- 搜索分区 -->
      <div v-if="search.tab === 0" class="sec">
        <NInput
          ref="inputEl"
          v-model:value="q"
          size="small"
          clearable
          :placeholder="qErr ? qErr : '整句查询（time:/level: 语法），Enter 执行'"
          :status="qErr ? 'error' : undefined"
          @keyup.enter="confirmSearch"
        />
        <ul v-if="histShow" class="hist">
          <li v-for="w in histWords" :key="w" @click="applyHist(w)">{{ w }}</li>
          <li v-if="!histWords.length" class="empty">暂无历史</li>
        </ul>
      </div>

      <!-- 高亮分区 -->
      <div v-else-if="search.tab === 1" class="sec">
        <NInput
          ref="inputEl"
          v-model:value="hlInput"
          size="small"
          clearable
          placeholder="高亮词,逗号分隔,Enter 加入（10 色轮换）"
          @keyup.enter="confirmHl"
        />
        <ul v-if="histShow" class="hist">
          <li v-for="w in histWords" :key="w" @click="applyHist(w)">
            <span class="dot" :style="{ background: HL_COLORS[0] }"></span>{{ w }}
          </li>
          <li v-if="!histWords.length" class="empty">暂无高频词</li>
        </ul>
        <ul v-if="ann.highlights.length" class="words">
          <li v-for="h in ann.highlights" :key="h.word">
            <span class="dot" :style="{ background: h.color }"></span>
            <span class="w" :style="{ color: h.color }">{{ h.word }}</span>
            <span class="del" @click="ann.removeHighlight(h.word)">&times;</span>
          </li>
        </ul>
        <p v-else class="empty">暂无高亮词——命中词以亮色粗体下划线展示</p>
      </div>

      <!-- 隐藏分区 -->
      <div v-else class="sec">
        <NInput
          ref="inputEl"
          v-model:value="hideInput"
          size="small"
          clearable
          placeholder="隐藏词,逗号分隔,Enter 加入（含词的行不显示）"
          @keyup.enter="confirmHide"
        />
        <ul v-if="histShow" class="hist">
          <li v-for="w in histWords" :key="w" @click="applyHist(w)">{{ w }}</li>
          <li v-if="!histWords.length" class="empty">暂无高频词</li>
        </ul>
        <ul v-if="ann.hideRules.length" class="words">
          <li v-for="r in ann.hideRules" :key="r">
            <span class="w">{{ r }}</span>
            <span class="del" @click="ann.removeHideRule(r)">&times;</span>
          </li>
        </ul>
        <p v-else class="empty">暂无隐藏词——右键日志行也可快捷加词</p>
        <NButton v-if="ann.hideRules.length" size="tiny" quaternary type="warning" @click="ann.clearHideRules()">
          清空全部
        </NButton>
      </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
/* 自绘 overlay(居中弹窗 + 遮罩,对齐 TUI searchpopup 形态;jsdom 测试友好) */
.overlay {
  position: fixed;
  inset: 0;
  z-index: 100;
  background: rgba(0, 0, 0, 0.45);
  display: flex;
  align-items: center;
  justify-content: center;
}
.dialog {
  width: 560px;
  max-width: 90vw;
  background: var(--popup-bg, #16202e);
  border: 1px solid var(--popup-border, #2a3a4f);
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
  padding: 10px 14px 14px;
}
.dlg-title {
  font-size: 13px;
  color: var(--dim, #8b98a9);
  margin-bottom: 8px;
}
.popup {
  display: flex;
  flex-direction: column;
  gap: 10px;
  font-size: 13px;
}
.tabs {
  display: flex;
  align-items: center;
  gap: 6px;
}
.tab-btn {
  border: 1px solid var(--popup-border, #2a3a4f);
  background: transparent;
  color: var(--dim, #8b98a9);
  border-radius: 4px;
  padding: 3px 14px;
  cursor: pointer;
  font-size: 12px;
}
.tab-btn.on {
  border-color: var(--accent, #4a8fe0);
  color: var(--accent, #4a8fe0);
}
.hint {
  margin-left: auto;
  color: var(--dim, #5d7292);
  font-size: 11px;
}
.sec {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.hist {
  list-style: none;
  margin: 0;
  padding: 4px;
  border: 1px solid var(--popup-border, #2a3a4f);
  border-radius: 4px;
  max-height: 180px;
  overflow-y: auto;
}
.hist li {
  padding: 3px 8px;
  border-radius: 3px;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 6px;
}
.hist li:hover {
  background: var(--popup-bg, #1f2c3f);
}
.words {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 200px;
  overflow-y: auto;
}
.words li {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 3px 2px;
}
.dot {
  width: 10px;
  height: 10px;
  border-radius: 3px;
  flex: none;
}
.w {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 700;
  text-decoration: underline;
}
.del {
  cursor: pointer;
  color: var(--dim, #8b98a9);
  padding: 0 4px;
}
.del:hover {
  color: var(--fg, #fff);
}
.empty {
  color: var(--dim, #8b98a9);
  font-size: 11px;
  margin: 0;
}
</style>
