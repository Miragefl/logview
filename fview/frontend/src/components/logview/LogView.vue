<script setup lang="ts">
// 日志视图：虚拟滚动（固定行高+overscan）+ 双域高亮渲染 + 行级隐藏 +
// 右键隐藏菜单 + 双击行详情 + follow 跟随。vim 键（j/k/n/N）由 App 挂键盘层调用 expose。
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { NDropdown, NModal, NInput, NButton } from 'naive-ui'
import { useTabsStore } from '../../stores/tabs'
import { useAnnotationsStore } from '../../stores/annotations'
import { useUiStore } from '../../stores/ui'
import type { LevelFilterMode } from '../../lib/filter'
import { levelFilter } from '../../lib/filter'
import { levelClass } from '../../lib/levelColor'
import { visibleRange } from '../../lib/virtual'
import { splitSpans } from '../../lib/hlSpan'
import { getLine, setFollow } from '../../lib/backend'
import type { LineDTO } from '../../lib/types'

const props = defineProps<{
  /** 搜索命中行 seq 集合（n/N 跳转与行标记） */
  hitSeqs: Set<number>
  /** 搜索关键词（命中词暗色底纹域） */
  hitWords: string[]
  /** 级别过滤档位 */
  levelMode: LevelFilterMode
  /** 显示中的字段名列表（rules.yaml hides 控制） */
  fields: string[]
}>()

const tabs = useTabsStore()
const ann = useAnnotationsStore()
const ui = useUiStore()

const scrollEl = ref<HTMLElement | null>(null)
const viewportH = ref(400)
const scrollTop = ref(0)

/** filteredLines 级别档位 + 行级隐藏后的可见行（虚拟滚动的数据底座）。 */
const filteredLines = computed<LineDTO[]>(() => {
  const t = tabs.active
  if (!t) return []
  if (props.levelMode === 'A' && ann.hideRules.length === 0) return t.lines
  return t.lines.filter(
    (l) => levelFilter(props.levelMode, l.level) && !ann.hidden(l)
  )
})

const rowH = computed(() => ui.rowPx())
/** range 可视行区间 [start, end)（响应式，随滚动/裁剪/字号联动）。 */
const range = computed<[number, number]>(() =>
  visibleRange(scrollTop.value, viewportH.value, rowH.value, filteredLines.value.length)
)
const visLines = computed(() => filteredLines.value.slice(range.value[0], range.value[1]))

/** renderSpans 行 message 分段（高亮词亮色粗体下划线 / 命中词暗色底纹）。 */
function renderSpans(text: string) {
  return splitSpans(text, ann.highlights, new Set(props.hitWords))
}

// 视口尺寸/滚动跟踪 + follow 智能暂停（滚离底部自动关，对齐 TUI/m4m6 行为）
let atBottom = true
function onScroll() {
  const el = scrollEl.value
  if (!el) return
  scrollTop.value = el.scrollTop
  atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 30
  const t = tabs.active
  if (!atBottom && t?.info.follow) {
    tabs.toggleFollow(t.info.id, false)
    setFollow(t.info.id, false).catch(() => {})
  }
}
let ro: ResizeObserver | null = null
onMounted(() => {
  const el = scrollEl.value
  if (!el) return
  viewportH.value = el.clientHeight
  ro = new ResizeObserver(() => {
    viewportH.value = el.clientHeight
  })
  ro.observe(el)
})
onBeforeUnmount(() => ro?.disconnect())

// 新行批到达（lines 引用变化）：
// ① follow 开 → 贴底（不能依赖行数——10 万稳态裁头时行数恒定）
// ② 非贴底且首行 seq 前移（裁头）→ scrollTop 反向补偿，保住阅读位置
let lastTabId = ''
let lastFirstSeq = -1
watch(
  () => tabs.active?.lines,
  async (lines) => {
    const t = tabs.active
    if (!t || !lines || lines.length === 0) {
      lastTabId = t?.info.id ?? ''
      lastFirstSeq = -1
      return
    }
    if (t.info.id === lastTabId && lastFirstSeq >= 0 && !atBottom) {
      const delta = lines[0].seq - lastFirstSeq
      if (delta > 0 && scrollEl.value) {
        scrollEl.value.scrollTop -= delta * rowH.value
      }
    }
    lastTabId = t.info.id
    lastFirstSeq = lines[0].seq
    if (t.info.follow) {
      await nextTick()
      const el = scrollEl.value
      if (el) el.scrollTop = el.scrollHeight
    }
  }
)
// 镜像裁头后 scrollTop 出界修正
watch(
  () => filteredLines.value.length,
  (total) => {
    const el = scrollEl.value
    if (!el) return
    const maxScroll = Math.max(0, total * rowH.value - viewportH.value)
    if (el.scrollTop > maxScroll) el.scrollTop = maxScroll
  }
)

// ---- 右键隐藏菜单 ----
const ctxMenu = ref({ show: false, x: 0, y: 0, line: null as LineDTO | null })
const kwModal = ref({ show: false, word: '' })
const ctxOptions = computed(() => [
  { label: '隐藏此行（按内容）', key: 'hide-line' },
  { label: '隐藏含关键词的行…', key: 'hide-kw' },
])

function onContext(e: MouseEvent, ln: LineDTO) {
  e.preventDefault()
  ctxMenu.value = { show: true, x: e.clientX, y: e.clientY, line: ln }
}

function onCtxSelect(key: string) {
  const ln = ctxMenu.value.line
  ctxMenu.value.show = false
  if (!ln) return
  if (key === 'hide-line') {
    ann.addHideRule(ln.message || ln.raw) // 按内容隐藏（同内容行全隐藏）
  } else {
    // 预填该行 message 首个空格分词
    kwModal.value = { show: true, word: (ln.message || ln.raw).split(/\s+/)[0] ?? '' }
  }
}

function confirmHideKw() {
  ann.addHideRule(kwModal.value.word.trim())
  kwModal.value.show = false
}

// ---- 双击行详情 ----
const detail = ref({ show: false, kv: {} as Record<string, string> })
async function openDetail(ln: LineDTO) {
  const t = tabs.active
  if (!t) return
  try {
    // GetLine 的 idx 是 buffer 索引域——镜像内 seq→index 换算（裁头后 seq≠索引）
    const idx = t.lines.findIndex((l) => l.seq === ln.seq)
    if (idx >= 0) {
      detail.value.kv = (await getLine(t.info.id, idx)) ?? {}
    } else {
      detail.value.kv = { ...ln } as unknown as Record<string, string>
    }
  } catch {
    detail.value.kv = { ...ln } as unknown as Record<string, string>
  }
  detail.value.show = true
}

// ---- 对外暴露（vim 键驱动） ----
/** scrollLines 滚动 ±n 行。 */
function scrollLines(delta: number) {
  const el = scrollEl.value
  if (el) el.scrollTop += delta * rowH.value
}
/** jumpHit 跳到上/下一个搜索命中行（seq 有序命中集合内顺序查找）。 */
function jumpHit(dir: 1 | -1) {
  if (props.hitSeqs.size === 0) return
  const arr = filteredLines.value
  const first = range.value[0]
  const firstSeq = arr[first]?.seq ?? 0
  let target: LineDTO | undefined
  if (dir === 1) {
    target = arr.find((l) => l.seq > firstSeq && props.hitSeqs.has(l.seq))
  } else {
    for (let i = Math.min(first, arr.length - 1); i >= 0; i--) {
      if (props.hitSeqs.has(arr[i].seq) && arr[i].seq < firstSeq) {
        target = arr[i]
        break
      }
    }
  }
  if (target && scrollEl.value) {
    const idx = arr.indexOf(target)
    scrollEl.value.scrollTop = Math.max(0, idx * rowH.value - viewportH.value / 2)
  }
}

/** scrollToEnd 滚到顶/底（vim g/G）。 */
function scrollToEnd(top: boolean) {
  const el = scrollEl.value
  if (!el) return
  el.scrollTop = top ? 0 : el.scrollHeight
}

defineExpose({ scrollLines, jumpHit, scrollToEnd })
</script>

<template>
  <div ref="scrollEl" class="logview" @scroll="onScroll">
    <div class="spacer" :style="{ height: filteredLines.length * rowH + 'px' }"></div>
    <div
      class="rows"
      :style="{ transform: `translateY(${range[0] * rowH}px)`, height: filteredLines.length * rowH + 'px' }"
    >
      <div
        v-for="(ln, i) in visLines"
        :key="ln.seq"
        class="row"
        :class="{ hit: hitSeqs.has(ln.seq) }"
        :style="{ top: i * rowH + 'px' }"
        @contextmenu="onContext($event, ln)"
        @dblclick="openDetail(ln)"
      >
        <span v-if="fields.includes('time')" class="f-time">{{ ln.time }}</span>
        <span v-if="fields.includes('level')" class="f-level" :class="levelClass(ln.level)">{{
          ln.level
        }}</span>
        <span v-if="fields.includes('thread') && ln.thread" class="f-thread">[{{ ln.thread }}]</span>
        <span v-if="fields.includes('traceId') && ln.traceId" class="f-trace">{{ ln.traceId }}</span>
        <span v-if="fields.includes('logger') && ln.logger" class="f-logger">{{ ln.logger }}</span>
        <span v-if="fields.includes('message')" class="f-msg">
          <template v-for="(s, si) in renderSpans(ln.message)" :key="si">
            <span v-if="s.hl" class="hl-w" :style="{ color: s.hl }">{{ s.text }}</span>
            <span v-else-if="s.hit" class="hit-w">{{ s.text }}</span>
            <template v-else>{{ s.text }}</template>
          </template>
        </span>
      </div>
    </div>

    <!-- 右键隐藏菜单 -->
    <NDropdown
      placement="bottom-start"
      trigger="manual"
      :x="ctxMenu.x"
      :y="ctxMenu.y"
      :options="ctxOptions"
      :show="ctxMenu.show"
      @select="onCtxSelect"
      @clickoutside="ctxMenu.show = false"
    />
    <!-- 关键词隐藏输入 -->
    <NModal v-model:show="kwModal.show" preset="dialog" title="隐藏含关键词的行" style="width: 380px">
      <NInput v-model:value="kwModal.word" placeholder="匹配 message/raw 子串" @keyup.enter="confirmHideKw" />
      <template #action>
        <NButton size="small" @click="kwModal.show = false">取消</NButton>
        <NButton size="small" type="primary" @click="confirmHideKw">隐藏</NButton>
      </template>
    </NModal>
    <!-- 行详情浮层 -->
    <NModal v-model:show="detail.show" preset="card" title="行详情" style="width: 640px">
      <table class="detail">
        <tr v-for="(v, k) in detail.kv" :key="k">
          <td class="k">{{ k }}</td>
          <td class="v">{{ v }}</td>
        </tr>
      </table>
    </NModal>
  </div>
</template>

<style scoped>
.logview {
  position: relative;
  flex: 1;
  overflow: auto;
  font-family: var(--font-mono, 'SF Mono', Menlo, monospace);
  font-size: var(--log-font-size, 13px);
  background: var(--bg, #10161f);
}
.spacer {
  position: absolute;
  left: 0;
  right: 0;
  top: 0;
  pointer-events: none;
}
.rows {
  position: absolute;
  left: 0;
  right: 0;
  top: 0;
}
.row {
  position: absolute;
  left: 0;
  right: 0;
  height: v-bind('rowH + "px"');
  line-height: v-bind('rowH + "px"');
  white-space: pre;
  padding: 0 10px;
  color: var(--fg, #c8d3e0);
  cursor: default;
}
.row:hover {
  background: var(--popup-bg, #16202e);
}
.row.hit {
  background: var(--hit-row-bg, rgba(122, 162, 214, 0.12));
}
.f-time {
  color: var(--time, #5d7292);
}
.f-level {
  font-weight: 600;
}
/* 字段色走主题变量（验收反馈⑤）——logger 无专用色保持 dim */
.f-thread {
  color: var(--thread, #9e9e9e);
}
.f-trace {
  color: var(--trace, #87ffff);
}
.f-logger {
  color: var(--dim, #8b98a9);
}
/* 双域视觉语言：高亮词=亮色粗体下划线（color 内联），命中词=暗色底纹 */
.hl-w {
  font-weight: 700;
  text-decoration: underline;
}
.hit-w {
  font-weight: 700;
  background: var(--hit-word-bg, rgba(125, 196, 228, 0.25));
  border-radius: 2px;
}
.lv-error {
  color: var(--level-error, #e57373);
}
.lv-warn {
  color: var(--level-warn, #e2c77e);
}
.lv-info {
  color: var(--level-info, #7ee29a);
}
.lv-debug {
  color: var(--level-debug, #8b98a9);
}
.detail {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}
.detail .k {
  width: 90px;
  color: var(--dim, #8b98a9);
  text-align: right;
  padding: 3px 10px 3px 0;
  vertical-align: top;
}
.detail .v {
  word-break: break-all;
  white-space: pre-wrap;
  padding: 3px 0;
}
</style>
