<script setup lang="ts">
// 高亮/隐藏词管理框(第六轮 SLS 式改造):SearchBar 两按钮/h/x 键唤起。
// 输入加词(逗号分词)+词表键盘流(ctrl+j/k 选中,ctrl+d·x 删词)+ctrl+R 历史
// (usage.json 词频,scope 隔离)——键盘流全套迁自 SearchPopup 第四轮实现。
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { NButton, NInput, NModal } from 'naive-ui'
import { useTabsStore } from '../../stores/tabs'
import { useAnnotationsStore } from '../../stores/annotations'
import { useSearchStore } from '../../stores/searchStore'
import { useUiStore } from '../../stores/ui'
import { bumpKeyword, usageKeywords } from '../../lib/backend'
import { scopeOfLabel } from '../../lib/scope'
import { HL_COLORS } from '../../stores/annotations'

const tabs = useTabsStore()
const ann = useAnnotationsStore()
const search = useSearchStore()
const ui = useUiStore()

/** mode 当前模式(''=关)。 */
const mode = computed(() => search.wordsShow)
const isHl = computed(() => mode.value === 'hl')

/** wordList 当前模式词表(高亮带色点,隐藏纯文本)。 */
const wordList = computed(() => (isHl.value ? ann.highlights.map((h) => h.word) : ann.hideRules))

/** scope 词频隔离域(对齐 TUI 各源 Scope,lib/scope 单源)。 */
const scope = computed(() => scopeOfLabel(tabs.active?.info.label ?? ''))

// ---- 输入加词 ----
const wordInput = ref('')
const inputEl = ref<InstanceType<typeof NInput> | null>(null)

/** splitKeywords 逗号拆词(对齐 TUI)。 */
function splitKeywords(s: string): string[] {
  return s
    .split(/[,，]/)
    .map((w) => w.trim())
    .filter((w) => w !== '')
}

function confirmAdd() {
  const words = splitKeywords(wordInput.value)
  for (const w of words) {
    if (isHl.value) ann.addHighlight(w)
    else ann.addHideRule(w)
    bumpKeyword(scope.value, isHl.value ? 'hl' : 'hide', w).catch(() => {})
  }
  wordInput.value = ''
}

/** removeAt 删第 i 个词(词表 × 与 d/x 键共用)。 */
function removeAt(i: number) {
  const w = wordList.value[i]
  if (w === undefined) return
  if (isHl.value) ann.removeHighlight(w)
  else ann.removeHideRule(w)
  if (navIdx.value >= wordList.value.length) navIdx.value = wordList.value.length - 1
}

// ---- 键盘流(迁自 SearchPopup:ctrl+j/k 选中,d/x 删词,ctrl+R 历史) ----
const HIST_MAX = 8
const histShow = ref(false)
const histWords = ref<string[]>([])
const navIdx = ref(-1)
const histEls = ref<HTMLElement[]>([])
const wordEls = ref<HTMLElement[]>([])

async function toggleHist() {
  if (histShow.value) {
    histShow.value = false
    return
  }
  histWords.value = ((await usageKeywords(scope.value, isHl.value ? 'hl' : 'hide')) ?? []).slice(0, HIST_MAX)
  histShow.value = true
  navIdx.value = -1
}

/** applyHist 历史词逗号追加到输入框。 */
function applyHist(w: string) {
  wordInput.value = wordInput.value ? `${wordInput.value},${w}` : w
  histShow.value = false
  navIdx.value = -1
}

function navMove(delta: number) {
  const n = histShow.value ? histWords.value.length : wordList.value.length
  if (n === 0) return
  navIdx.value = Math.min(n - 1, Math.max(0, navIdx.value < 0 ? (delta > 0 ? 0 : n - 1) : navIdx.value + delta))
  const els = histShow.value ? histEls.value : wordEls.value
  els[navIdx.value]?.scrollIntoView({ block: 'nearest' })
}

/** onDocKey 管理框键流(document 级,焦点丢失也响应):
 *  Esc 分层(历史→框) / Enter 历史应用或加词 / ctrl+j/k 导航 / ctrl+d·x 删选中词 / ctrl+R 历史。
 *  r2:删词主键改 ctrl+d——组合键不与打字冲突,焦点在输入框也生效(旧裸 d 因弹窗
 *  自动聚焦+inInput 防御在真机恒失效);x 保留但仅焦点不在输入框时(可打印字符,
 *  打 "x" 词不吞,final review C2)。 */
function onDocKey(e: KeyboardEvent) {
  if (!mode.value) return
  const t = e.target as HTMLElement | null
  const inInput = !!t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA')
  if (e.key === 'Escape') {
    if (histShow.value) {
      histShow.value = false
      navIdx.value = -1
    } else {
      search.closeWords()
    }
  } else if (e.key === 'Enter') {
    e.preventDefault()
    if (histShow.value && navIdx.value >= 0) {
      const w = histWords.value[navIdx.value]
      if (w !== undefined) applyHist(w)
    } else {
      confirmAdd()
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
  } else if (
    (e.key === 'd' ? e.ctrlKey : e.key === 'x' && !inInput) &&
    navIdx.value >= 0 &&
    !histShow.value
  ) {
    e.preventDefault()
    removeAt(navIdx.value)
  }
}

// 开关注册:弹窗栈登记(全局 vim 键让位)+键监听挂卸+状态复位;
// 打开时聚焦加词输入框(替代 naive autoFocus,避免焦点落在关闭按钮上误触)
watch(
  () => search.wordsShow,
  (v) => {
    if (v) {
      ui.dialogOpen('words')
      document.addEventListener('keydown', onDocKey)
      wordInput.value = ''
      histShow.value = false
      navIdx.value = -1
      nextTick(() => inputEl.value?.focus())
    } else {
      ui.dialogClose('words')
      document.removeEventListener('keydown', onDocKey)
    }
  },
  { immediate: true }
)
onBeforeUnmount(() => {
  // 卸载清 wordsShow(final review I2):管理框开着关最后 tab 后重开 tab,
  // v-if 重挂载若 wordsShow 残留会立即自动弹窗
  search.closeWords()
  ui.dialogClose('words')
  document.removeEventListener('keydown', onDocKey)
})
</script>

<template>
  <NModal
    :show="mode !== ''"
    preset="card"
    :title="isHl ? '高亮词' : '隐藏词'"
    style="width: 420px"
    :close-on-esc="false"
    :auto-focus="false"
    @update:show="(v: boolean) => !v && search.closeWords()"
  >
    <div class="wordspopup">
      <NInput
        ref="inputEl"
        v-model:value="wordInput"
        size="small"
        clearable
        :placeholder="isHl ? '高亮词,逗号分隔,Enter 加入(10 色轮换)' : '隐藏词,逗号分隔,Enter 加入(含词的行不显示)'"
      />
      <ul v-if="histShow" class="hist">
        <li
          v-for="(w, i) in histWords"
          :key="w"
          :ref="(el) => (histEls[i] = el as HTMLElement)"
          :class="{ sel: i === navIdx }"
          @click="applyHist(w)"
          @mousemove="navIdx = i"
        >
          <span v-if="isHl" class="dot" :style="{ background: HL_COLORS[0] }"></span>{{ w }}
        </li>
        <li v-if="!histWords.length" class="empty">暂无高频词</li>
      </ul>
      <ul v-if="wordList.length" class="words">
        <li
          v-for="(w, i) in wordList"
          :key="w"
          :ref="(el) => (wordEls[i] = el as HTMLElement)"
          :class="{ sel: !histShow && i === navIdx }"
          @mousemove="navIdx = i"
        >
          <span v-if="isHl" class="dot" :style="{ background: ann.highlights.find((h) => h.word === w)?.color }"></span>
          <span class="w">{{ w }}</span>
          <span class="del" @click="removeAt(i)">
            <svg class="ico ico-sm" viewBox="0 0 24 24"><path d="M18 6L6 18M6 6l12 12" /></svg>
          </span>
        </li>
      </ul>
      <p v-else class="empty">{{ isHl ? '暂无高亮词——命中词以亮色粗体下划线展示' : '暂无隐藏词——右键日志行也可快捷加词' }}</p>
      <div class="foot">
        <span class="hint">ctrl+j/k 选中 · ctrl+d 删词 · ctrl+R 历史</span>
        <NButton v-if="!isHl && ann.hideRules.length" size="tiny" quaternary type="warning" @click="ann.clearHideRules()">
          清空全部
        </NButton>
      </div>
    </div>
  </NModal>
</template>

<style scoped>
.wordspopup {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: var(--font-ui-base, 12px);
}
.hist,
.words {
  list-style: none;
  margin: 0;
  padding: 4px;
  border: 1px solid var(--popup-border, #2a3a4f);
  border-radius: 4px;
  max-height: 180px;
  overflow-y: auto;
}
.words {
  padding: 0;
  border: none;
}
.hist li,
.words li {
  padding: 3px 8px;
  border-radius: 3px;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 6px;
}
.hist li:hover,
.hist li.sel,
.words li:hover,
.words li.sel {
  background: var(--popup-bg, #1f2c3f);
}
li.sel {
  box-shadow: inset 2px 0 0 var(--accent, #4a8fe0);
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
  font-size: var(--font-ui-sm, 11px);
  margin: 0;
}
.foot {
  display: flex;
  align-items: center;
}
.hint {
  flex: 1;
  color: var(--dim, #5d7292);
  font-size: var(--font-ui-sm, 11px);
}
</style>
