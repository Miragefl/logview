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
import { DEFAULT_WIDTHS, headerCells, layoutLine } from '../../lib/columns'
import { visibleRange } from '../../lib/virtual'
import { ensureVisible, moveClamp } from '../../lib/nav'
import { splitSpans } from '../../lib/hlSpan'
import { buildViewItems } from '../../lib/folds'
import type { FoldRange, ViewItem } from '../../lib/folds'
import { buildCtxMixed } from '../../lib/ctxmix'
import { selectedSeqRange } from '../../lib/visualSel'
import type { SeqRange } from '../../lib/visualSel'
import { wrapSegments, HeightIndex, visibleRows } from '../../lib/wrap'
import { ClipboardSetText } from '../../../wailsjs/runtime/runtime'
import { bumpKeyword, getLine, setFollow } from '../../lib/backend'
import { scopeOfLabel } from '../../lib/scope'
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
  /** 列宽覆盖（rules.yaml fields[].width;缺省用 DEFAULT_WIDTHS） */
  widths?: Record<string, number>
  /** 来源列开关(k8s 多 pod 合流 tab=true:gutter 加 pod 名 chip,占宽计入 wrap 模型) */
  sourceCol?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:widths', widths: Record<string, number>): void
}>()

const tabs = useTabsStore()
const ann = useAnnotationsStore()
const ui = useUiStore()

const scrollEl = ref<HTMLElement | null>(null)
const viewportH = ref(400)
const viewportW = ref(0)
const scrollTop = ref(0)

/** filteredLines 级别档位 + 行级隐藏 + 搜索过滤后的可见行（虚拟滚动的数据底座）。
 * 搜索过滤对齐 TUI recomputeView:命中集非空时只保留命中行(第三轮验收⑤)。 */
const filteredLines = computed<LineDTO[]>(() => {
  const t = tabs.active
  if (!t) return []
  if (props.levelMode === 'A' && ann.hideRules.length === 0 && props.hitSeqs.size === 0) {
    return t.lines
  }
  return t.lines.filter(
    (l) =>
      levelFilter(props.levelMode, l.level) &&
      !ann.hidden(l) &&
      (props.hitSeqs.size === 0 || props.hitSeqs.has(l.seq))
  )
})

const rowH = computed(() => ui.rowPx())

/** mixedView 过滤视图叠上下文混入锚(2026-11-04 移植 TUI grep-context):
 *  混入行先归并(升序+dimSeqs 集)再进折叠——ctx 行 seq 落折叠组内照常被折叠吞(折叠意图优先)。 */
const mixedView = computed(() => {
  const t = tabs.active
  return buildCtxMixed(filteredLines.value, t?.lines ?? [], t?.ctxAnchors ?? {})
})

/** viewItems 折叠视图项序列(行/折叠占位,验收反馈④ e 键)。 */
const viewItems = computed<ViewItem[]>(() => {
  const t = tabs.active
  return buildViewItems(mixedView.value.lines, t?.folds ?? [], t?.collapsed ?? [])
})

// ---- w 换行变高行引擎(验收反馈④,已拍板硬做) ----
/** wrapRows seq→视觉行数缓存(增量维护;切 tab/宽度变时清)。 */
const wrapRows = new Map<number, number>()
const rowsVer = ref(0)
const charPx = ref(7)

/** measureChar 探针测单字符宽('0'×50 均值,字体/字号变化时重测)。 */
function measureChar() {
  const el = scrollEl.value
  if (!el) return
  const s = document.createElement('span')
  s.style.cssText = 'position:absolute;visibility:hidden;white-space:pre'
  s.style.font = getComputedStyle(el).font
  s.textContent = '0'.repeat(50)
  el.appendChild(s)
  const w = s.getBoundingClientRect().width / 50
  s.remove()
  if (w > 0) charPx.value = w
}

/** msgWidthCh message 列可用显示宽(ch):容器宽 - gutter - 固定列 - padding,留 1 字符余量。 */
const msgWidthCh = computed(() => {
  if (!ui.wrap) return 0
  const fixedCh = props.fields.reduce(
    (s, f) => s + (f === 'message' ? 0 : (colsW.value[f] ?? 0) + 2),
    0
  )
  const gutterCh = (showLineNum.value ? 9 : 2) + (props.sourceCol ? 18 : 0)
  const avail = viewportW.value - (fixedCh + gutterCh) * charPx.value - 20
  return Math.max(10, Math.floor(avail / charPx.value) - 1)
})

// 宽度变化(字号/字体/窗口/行号开关)→ 全量重算;行批到达只增量算新 seq
watch([() => msgWidthCh.value, () => tabs.activeId], () => {
  wrapRows.clear()
  const lines = tabs.active?.lines
  if (ui.wrap && lines) {
    for (const l of lines) {
      wrapRows.set(l.seq, Math.max(1, wrapSegments(l.message, msgWidthCh.value).length))
    }
  }
  rowsVer.value++
})
watch(
  () => tabs.active?.lines,
  (lines) => {
    if (!ui.wrap || !lines) return
    let added = false
    for (const l of lines) {
      if (!wrapRows.has(l.seq)) {
        wrapRows.set(l.seq, Math.max(1, wrapSegments(l.message, msgWidthCh.value).length))
        added = true
      }
    }
    if (added) rowsVer.value++
  },
  { immediate: true }
)

/** heightsArr 变高模型(wrap 关闭时 null = 等高路径零回退);缓存 miss 现算兜底。 */
const heightsArr = computed<number[] | null>(() => {
  void rowsVer.value
  if (!ui.wrap) return null
  return viewItems.value.map((it) =>
    it.type === 'fold'
      ? 1
      : wrapRows.get(it.ln.seq) ??
        Math.max(1, wrapSegments(it.ln.message, msgWidthCh.value).length)
  )
})
const hi = computed(() => new HeightIndex(heightsArr.value ?? [], rowH.value))
const totalH = computed(() =>
  heightsArr.value ? hi.value.total() : viewItems.value.length * rowH.value
)
/** rowTop/rowHeightAt 统一行定位(等高与变高两态)。 */
function rowTop(i: number): number {
  return heightsArr.value ? hi.value.offsetOf(i) : i * rowH.value
}
function rowHeightAt(i: number): number {
  return heightsArr.value ? hi.value.rowsAt(i) * rowH.value : rowH.value
}

/** colsW 生效列宽（分栏渲染,验收反馈②）;拖拽中优先本地态——
 *  mousemove 只改渲染列宽,mouseup 才 emit 落盘(wrap 行高全量重算一次,拖拽 60fps 不卡)。 */
const dragW = ref<Record<string, number> | null>(null)
const colsW = computed<Record<string, number>>(() => dragW.value ?? props.widths ?? DEFAULT_WIDTHS)

// ---- 表头列宽拖拽(第七轮验收②):把手 mousedown → document move/up → 落盘 ----
let dragField = ''
let dragX0 = 0
let dragW0 = 0
function startDrag(e: MouseEvent, field: string) {
  if (e.button !== 0) return
  dragField = field
  dragX0 = e.clientX
  dragW0 = colsW.value[field] ?? 0
  document.addEventListener('mousemove', onDragMove)
  document.addEventListener('mouseup', endDrag)
}
function onDragMove(e: MouseEvent) {
  if (!dragField) return
  const ch = Math.max(4, dragW0 + Math.round((e.clientX - dragX0) / charPx.value))
  dragW.value = { ...colsW.value, [dragField]: ch }
}
function endDrag() {
  document.removeEventListener('mousemove', onDragMove)
  document.removeEventListener('mouseup', endDrag)
  if (dragW.value) emit('update:widths', dragW.value)
  dragField = ''
  dragW.value = null
}
onBeforeUnmount(() => {
  document.removeEventListener('mousemove', onDragMove)
  document.removeEventListener('mouseup', endDrag)
})
/** headCells 表头行（与数据列同宽对齐,sticky 置顶）。 */
const headCells = computed(() => headerCells(props.fields, colsW.value))
/** cellsOf 行分栏（可视区内逐行调用,纯函数）。 */
function cellsOf(ln: LineDTO) {
  return layoutLine(ln, props.fields, colsW.value)
}
/** cellW 固定列宽样式（等宽字体 ch 单位;message 弹性不限宽）。 */
function cellW(field: string): Record<string, string> | undefined {
  if (field === 'message') return undefined
  return { width: `${colsW.value[field] ?? 0}ch` }
}
/** range 可视行区间 [start, end)（响应式;等高走 visibleRange,变高走 visibleRows）。 */
const range = computed<[number, number]>(() =>
  heightsArr.value
    ? visibleRows(hi.value, scrollTop.value, viewportH.value)
    : visibleRange(scrollTop.value, viewportH.value, rowH.value, viewItems.value.length)
)
const visItems = computed(() => viewItems.value.slice(range.value[0], range.value[1]))

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
  viewportW.value = el.clientWidth
  measureChar()
  ro = new ResizeObserver(() => {
    viewportH.value = el.clientHeight
    viewportW.value = el.clientWidth
  })
  ro.observe(el)
})
onBeforeUnmount(() => ro?.disconnect())
// 字体/字号变化重测字符宽(驱动 msgWidthCh → wrap 行高全量重算)
watch(
  () => [ui.fontSize, ui.fontFamily],
  () => {
    nextTick(measureChar)
  }
)

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
        if (heightsArr.value) {
          // 变高行:补偿 = 被裁行总高(镜像 seq 连续,逐 seq 累计缓存行数)
          let off = 0
          for (let s = lastFirstSeq; s < lines[0].seq; s++) {
            off += (wrapRows.get(s) ?? 1) * rowH.value
          }
          scrollEl.value.scrollTop -= off
        } else {
          scrollEl.value.scrollTop -= delta * rowH.value
        }
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
// 镜像裁头后 scrollTop 出界修正(等高/变高统一走 totalH)
watch(totalH, (th) => {
  const el = scrollEl.value
  if (!el) return
  const maxScroll = Math.max(0, th - viewportH.value)
  if (el.scrollTop > maxScroll) el.scrollTop = maxScroll
})

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
    const w = ln.message || ln.raw
    ann.addHideRule(w) // 按内容隐藏（同内容行全隐藏）
    bumpKeyword(scopeOfLabel(tabs.active?.info.label ?? ''), 'hide', w).catch(() => {})
  } else {
    // 预填该行 message 首个空格分词
    kwModal.value = { show: true, word: (ln.message || ln.raw).split(/\s+/)[0] ?? '' }
  }
}

function confirmHideKw() {
  const w = kwModal.value.word.trim()
  ann.addHideRule(w)
  bumpKeyword(scopeOfLabel(tabs.active?.info.label ?? ''), 'hide', w).catch(() => {})
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

// ---- 光标模型（验收反馈④,TUI 手感:j/k 移光标,scrolloff 跟随） ----
const cursorSeq = ref(0)
const cursorIdx = computed(() =>
  viewItems.value.findIndex((it) => it.type === 'line' && it.ln.seq === cursorSeq.value)
)
const rowsInView = computed(() => Math.max(1, Math.floor(viewportH.value / rowH.value)))

/** stopFollow 关跟随（本地+后端;锚定/回顶时用,对齐 TUI）。 */
function stopFollow() {
  const t = tabs.active
  if (t?.info.follow) {
    tabs.toggleFollow(t.info.id, false)
    setFollow(t.info.id, false).catch(() => {})
  }
}

/** ensureVisiblePx 变高行态:光标行保持 scrolloff(≈3 行)余量可见(px 语义)。 */
function ensureVisiblePx(idx: number) {
  const el = scrollEl.value
  if (!el) return
  const top = rowTop(idx)
  const h = rowHeightAt(idx)
  const so = 3 * rowH.value
  if (top < el.scrollTop + so) {
    el.scrollTop = Math.max(0, top - so)
  } else if (top + h > el.scrollTop + viewportH.value - so) {
    el.scrollTop = top + h - viewportH.value + so
  }
}

/** setCursor 置光标到视图项 idx(line 项),可选滚动保可视（scrolloff=3;等高/变高两态）。
 *  first 一律取 el.scrollTop 实时值换算(与 ensureVisiblePx 同源)——range[0] 源自
 *  异步 scroll 事件更新的 scrollTop.value,键盘连按时滞后:j 钉住态碰巧无害(下滚
 *  分支只依赖 idx),k 回爬时 stale first 偏小使光标永落 stale 底部圈,被反复拽回
 *  倒数第 4 行爬不出去(2026-11-04 回归测试 logview-cursor-follow)。 */
function setCursor(idx: number, scroll = true) {
  const arr = viewItems.value
  const it = arr[idx]
  if (idx < 0 || !it || it.type !== 'line') return
  cursorSeq.value = it.ln.seq
  if (scroll && scrollEl.value) {
    if (heightsArr.value) {
      ensureVisiblePx(idx)
    } else {
      const first = Math.round(scrollEl.value.scrollTop / rowH.value)
      const nf = ensureVisible(idx, first, rowsInView.value, arr.length)
      scrollEl.value.scrollTop = nf * rowH.value
    }
  }
}

/** moveCursor 光标 ±delta;无光标时从可视区边缘起(跳过折叠占位行)。 */
function moveCursor(delta: number) {
  const arr = viewItems.value
  if (!arr.length) return
  const ci = cursorIdx.value
  if (ci < 0) {
    setCursor(delta > 0 ? range.value[0] : Math.max(0, range.value[1] - 1))
    return
  }
  setCursor(moveClamp(ci, delta, arr.length))
}

/** cursorToViewport 光标跳到可视顶/中/底（不滚动）。 */
function cursorToViewport(pos: 'top' | 'mid' | 'bottom') {
  const [s, e] = range.value
  const idx = pos === 'top' ? s : pos === 'mid' ? Math.floor((s + e) / 2) : Math.max(0, e - 1)
  setCursor(idx, false)
}

/** cursorToSeq 光标到指定 seq 行（viewItems 域查找）。 */
function cursorToSeq(seq: number) {
  const idx = viewItems.value.findIndex((it) => it.type === 'line' && it.ln.seq === seq)
  if (idx >= 0) setCursor(idx)
}

/** anchorCursor zt/zz/zb:滚动使光标行置顶/居中/置底（关 follow,对齐 TUI;两态行定位）。 */
function anchorCursor(pos: 't' | 'z' | 'b') {
  const ci = cursorIdx.value
  const el = scrollEl.value
  if (ci < 0 || !el) return
  stopFollow()
  const top = rowTop(ci)
  const h = rowHeightAt(ci)
  if (pos === 't') {
    el.scrollTop = top
  } else if (pos === 'z') {
    el.scrollTop = Math.max(0, top - (viewportH.value - h) / 2)
  } else {
    el.scrollTop = Math.max(0, top + h - viewportH.value)
  }
}

/** cursorToHit n/N:光标跳上/下一个搜索命中;无搜索时回落高亮词命中行
 * (第五轮验收:高亮后 n/N 也要能跳;高亮词匹配大小写敏感,对齐渲染层 splitSpans)。
 * 目标在折叠组内先展开,跳后居中。 */
function cursorToHit(dir: 1 | -1) {
  // 命中集:搜索优先;空集且有高亮词时按词现扫(按键频次低,不做常驻缓存)
  let hits = props.hitSeqs
  if (hits.size === 0 && ann.highlights.length > 0) {
    hits = new Set(
      filteredLines.value
        .filter((l) => ann.highlights.some((h) => (l.message || l.raw).includes(h.word)))
        .map((l) => l.seq)
    )
  }
  if (hits.size === 0) return
  const arr = filteredLines.value
  const cur = viewItems.value[cursorIdx.value]
  const refSeq = cur && cur.type === 'line' ? cur.ln.seq : (viewItems.value[range.value[0]] as { ln?: LineDTO } | undefined)?.ln?.seq ?? 0
  let targetSeq = -1
  if (dir === 1) {
    const hit = arr.find((l) => l.seq > refSeq && hits.has(l.seq))
    targetSeq = hit?.seq ?? -1
  } else {
    for (let i = arr.length - 1; i >= 0; i--) {
      if (hits.has(arr[i].seq) && arr[i].seq < refSeq) {
        targetSeq = arr[i].seq
        break
      }
    }
  }
  if (targetSeq < 0) return
  ensureExpanded(targetSeq)
  cursorToSeq(targetSeq)
  const idx = cursorIdx.value
  if (idx >= 0 && scrollEl.value) {
    scrollEl.value.scrollTop = Math.max(0, rowTop(idx) - (viewportH.value - rowHeightAt(idx)) / 2)
  }
}

/** ensureExpanded 目标 seq 落在已折叠组内时展开该组。 */
function ensureExpanded(seq: number) {
  const t = tabs.active
  if (!t) return
  const f = t.folds.find(
    (x) => seq >= x.startSeq && seq <= x.endSeq && t.collapsed.includes(x.startSeq)
  )
  if (f) tabs.toggleCollapse(f.startSeq)
}

/** gotoTop g:回顶（关 follow）。 */
function gotoTop() {
  stopFollow()
  const first = viewItems.value[0]
  if (first?.type === 'line') cursorToSeq(first.ln.seq)
  if (scrollEl.value) scrollEl.value.scrollTop = 0
}

/** gotoBottom G:到底（开 follow,对齐 TUI autoscroll）。 */
function gotoBottom() {
  const t = tabs.active
  if (t && !t.info.follow) {
    tabs.toggleFollow(t.info.id, true)
    setFollow(t.info.id, true).catch(() => {})
  }
  const last = viewItems.value[viewItems.value.length - 1]
  if (last?.type === 'line') cursorToSeq(last.ln.seq)
  if (scrollEl.value) scrollEl.value.scrollTop = scrollEl.value.scrollHeight
}

/** halfPage ctrl+d/u:光标 ±半页。 */
function halfPage(dir: 1 | -1) {
  moveCursor(dir * Math.max(1, Math.floor(rowsInView.value / 2)))
}

/** pageMove ctrl+f/b、pgup/pgdn:光标 ±整页（留 2 行上下文）。 */
function pageMove(dir: 1 | -1) {
  moveCursor(dir * Math.max(1, rowsInView.value - 2))
}

/** openDetailAtCursor d:光标行详情（等价双击）。 */
function openDetailAtCursor() {
  const it = viewItems.value[cursorIdx.value]
  if (it && it.type === 'line') openDetail(it.ln)
}

/** toggleFold e:光标行所在堆栈组折叠/展开(TUI 同款)。 */
function toggleFold() {
  const t = tabs.active
  if (!t) return
  const f = t.folds.find(
    (x) => cursorSeq.value >= x.startSeq && cursorSeq.value <= x.endSeq
  )
  if (f) tabs.toggleCollapse(f.startSeq)
}

/** unfoldAt 点击折叠占位行展开。 */
function unfoldAt(range: FoldRange) {
  tabs.toggleCollapse(range.startSeq)
}

// ---- 可视选择(验收反馈④ v/V/y):锚=起选行,j/k 移光标即扩选,seq 域区间 ----
const visual = ref({ active: false, anchorSeq: 0 })
/** lineSeqs 可见行 seq 序列(选择域,折叠占位天然不进)。 */
const lineSeqs = computed(() =>
  viewItems.value.filter((it) => it.type === 'line').map((it) => it.ln.seq)
)
const visualRange = computed<SeqRange | null>(() =>
  visual.value.active
    ? selectedSeqRange(lineSeqs.value, visual.value.anchorSeq, cursorSeq.value)
    : null
)
/** inVisual 行是否落在选择区间(渲染 .vis)。 */
function inVisual(seq: number): boolean {
  const r = visualRange.value
  return !!r && seq >= r.lo && seq <= r.hi
}

/** startVisual v/V:从光标行起选(本版无行内选择,V 与 v 等价预留)。 */
function startVisual(_line: boolean) {
  const it = viewItems.value[cursorIdx.value]
  if (it && it.type === 'line') {
    visual.value = { active: true, anchorSeq: it.ln.seq }
  }
}

/** exitVisual Esc:退出可视选择(顺带收书签列表)。 */
function exitVisual() {
  visual.value = { active: false, anchorSeq: 0 }
  bmShow.value = false
}

// ---- 上下文混入(2026-11-04 移植 TUI grep-context 最终语义:sticky 多锚) ----
// 键序列有状态(数字累积),不进 keymap 纯映射——App.vue onKeydown 前置调 ctxKey,
// bookmarkNav 同款"返回是否消费"模式。锚存 tab store(seq 键控),渲染走 mixedView。
const ctxInput = ref('') // +x/-x 输入态("+"/"-" 前缀+累积数字;空=未激活)

/** ctxHint 输入态提示文案(状态栏展示;非输入态清空——混入 ×K 是持久徽标另走锚数)。 */
function ctxHint(): void {
  ui.ctxHint = `上下文 ${ctxInput.value} (Enter 确认/Esc 取消)`
}

/** ctxAnchorCount 活锚数(状态栏「混入 ×K」)。 */
function ctxAnchorCount(): number {
  return Object.keys(tabs.active?.ctxAnchors ?? {}).length
}

/** hasActiveFilter 过滤激活判定(无过滤时 +/- 无操作——全量视图即原始流,TUI 同)。 */
function hasActiveFilter(): boolean {
  return props.levelMode !== 'A' || ann.hideRules.length > 0 || props.hitSeqs.size > 0
}

/** applyCtxAnchor Enter 生效:光标行设锚(空数字默认 5/上限 1000 在 ctxKey 钳制);
 *  仅过滤命中行可锚——dim 行/折叠占位上按 + 无操作(锚=命中行的上下文,语义收窄自 TUI)。 */
function applyCtxAnchor(before: boolean, n: number): void {
  const seq = cursorSeq.value
  if (!filteredLines.value.some((l) => l.seq === seq)) return
  tabs.setCtxAnchor(seq, { before, n })
}

/** nearestCursor 光标回最近过滤行(TUI nearestFvCursor 语义:清锚后光标停在
 *  已消失的 dim 行时回位,线性扫最近 seq——锚清空触发频次低,O(m) 足够)。 */
function nearestCursor(): void {
  const ls = filteredLines.value
  if (!ls.length) return
  let best = ls[0]
  for (const l of ls) {
    if (Math.abs(l.seq - cursorSeq.value) < Math.abs(best.seq - cursorSeq.value)) best = l
  }
  cursorSeq.value = best.seq
}

/** clearCtx 清全部锚(--/++/Esc 显式关闭/过滤变化语义性退出);光标落 dim 行时回最近过滤行。 */
function clearCtx(): void {
  if (ctxAnchorCount() === 0) return
  tabs.clearCtxAnchors()
  if (!filteredLines.value.some((l) => l.seq === cursorSeq.value)) nearestCursor()
}

/** ctxKey +x/-x 键序列:返回是否消费(App.vue onKeydown 前置钩子)。
 *  输入态:数字累积/Enter 生效(空=5,上限 1000)/Esc 取消/Backspace 删字符/
 *  第二个 +/- 关闭混入(--/++ 任一符号)/其余吞键(TUI:输入态其他键忽略);
 *  非输入态:+/- 且过滤激活且非 visual 进输入态、有锚 Esc 先清锚、其余 false 交回键位机。 */
function ctxKey(e: KeyboardEvent): boolean {
  if (ctxInput.value) {
    if (e.key === 'Escape') {
      ctxInput.value = ''
      ui.ctxHint = ''
    } else if (e.key === 'Enter') {
      const digits = ctxInput.value.slice(1)
      const n = digits ? Math.min(1000, parseInt(digits, 10)) : 5
      applyCtxAnchor(ctxInput.value[0] === '-', n)
      ctxInput.value = ''
      ui.ctxHint = ''
    } else if (e.key === 'Backspace') {
      if (ctxInput.value.length > 1) {
        ctxInput.value = ctxInput.value.slice(0, -1)
        ctxHint()
      }
    } else if (e.key === '+' || e.key === '-') {
      // 第二个符号:混入态关闭(--/++ 任一),纯输入态等价取消(TUI handleCtxInputKeys 同)
      ctxInput.value = ''
      ui.ctxHint = ''
      clearCtx()
    } else if (/^[0-9]$/.test(e.key)) {
      ctxInput.value += e.key
      ctxHint()
    }
    return true
  }
  if ((e.key === '+' || e.key === '-') && !e.ctrlKey && !e.metaKey && !e.altKey) {
    if (hasActiveFilter() && filteredLines.value.length > 0 && !visual.value.active) {
      ctxInput.value = e.key
      ctxHint()
      return true
    }
    return false // 无过滤/visual 态 no-op,不吞键
  }
  if (e.key === 'Escape' && ctxAnchorCount() > 0) {
    clearCtx()
    return true
  }
  return false
}

// 过滤条件变化(搜索/级别/隐藏词)→ 锚点行可能已不在命中集,语义性退出混入
// (TUI recomputeView 同款);顺带取消未确认的输入态。
watch([() => props.hitSeqs, () => props.levelMode, () => ann.hideRules.length], () => {
  clearCtx()
  ctxInput.value = ''
  ui.ctxHint = ''
})

// visual 态同步 ui store(状态栏 VISUAL 徽标,第三轮验收⑨)
watch(
  () => visual.value.active,
  (v) => (ui.visualActive = v)
)

// Esc 兜底:kwModal/detail/bmShow 非模态层(App 全局键已因弹窗栈让位,此处接管;第三轮验收②)
function onDocEsc(e: KeyboardEvent) {
  if (e.key !== 'Escape') return
  if (kwModal.value.show) kwModal.value.show = false
  else if (detail.value.show) detail.value.show = false
  else if (bmShow.value) bmShow.value = false
}
onMounted(() => document.addEventListener('keydown', onDocEsc))
onBeforeUnmount(() => document.removeEventListener('keydown', onDocEsc))
// kwModal/detail 进弹窗栈(全局 vim 键让位)
watch(
  () => kwModal.value.show,
  (v) => (v ? ui.dialogOpen('kw') : ui.dialogClose('kw'))
)
watch(
  () => detail.value.show,
  (v) => (v ? ui.dialogOpen('detail') : ui.dialogClose('detail'))
)

/** yankSelection y:复制选中区间原文(raw 换行拼接),返回行数(0=无选择)。
 *  域取 mixedView.lines——光标/区间可落在混入 dim 行上(TUI yankLines 同为混入视图坐标)。 */
function yankSelection(): number {
  const r = visualRange.value
  if (!r) return 0
  const sel = mixedView.value.lines.filter((l) => l.seq >= r.lo && l.seq <= r.hi)
  ClipboardSetText(sel.map((l) => l.raw).join('\n')).catch(() => {})
  return sel.length
}

/** yank y 统一入口(TUI 语义,第九轮验收④):VISUAL 选区在复制选区,否则复制当前光标行。 */
function yank(): number {
  if (visual.value.active) return yankSelection()
  const cur = mixedView.value.lines.find((l) => l.seq === cursorSeq.value)
  if (!cur) return 0
  ClipboardSetText(cur.raw).catch(() => {})
  return 1
}

/** hasSelection ⌘C 拦截判定:选区在才吞系统复制,否则 App 放行原生行为。 */
function hasSelection(): boolean {
  return visual.value.active && visualRange.value !== null
}

// ---- 鼠标拖选(第九轮验收④):按下拖动=按行范围选中(复用 VISUAL 高亮,行粒度),
// 松开定格(ESC/再次 v 重置);未构成拖动(单击/原地)不建选区,不影响双击详情/右键菜单。 ----
const dragAnchor = ref<number | null>(null)
let dragMoved = false

function rowMouseDown(e: MouseEvent, seq: number) {
  if (e.button !== 0) return
  dragAnchor.value = seq
  dragMoved = false
}

function rowMouseMove(e: MouseEvent, seq: number) {
  if (dragAnchor.value === null) return
  if (!(e.buttons & 1)) {
    // 拖选已在日志区外松开(mouseup 丢失):选区定格,清拖拽态防"复活"跟随(终审 Important1)
    dragAnchor.value = null
    return
  }
  if (dragAnchor.value === seq) return
  if (!dragMoved) {
    visual.value = { active: true, anchorSeq: dragAnchor.value }
    dragMoved = true
  }
  cursorSeq.value = seq // 端点跟随 → visualRange/inVisual 高亮联动
}

function rowMouseUp() {
  dragAnchor.value = null // 拖成选区则自然定格;未拖则无副作用
}

// ---- 书签 m/' + 行号 # ----
/** bmSet 书签 seq 集合(行前缀渲染)。 */
const bmSet = computed(() => new Set(tabs.active?.bookmarks ?? []))
const bmShow = ref(false)
const bmItems = computed(() => {
  const t = tabs.active
  if (!t) return []
  return [...t.bookmarks].sort((a, b) => a - b).map((seq) => {
    const l = filteredLines.value.find((x) => x.seq === seq)
    return { seq, label: (l?.message ?? `seq ${seq}`).slice(0, 60) }
  })
})

/** toggleBookmark m:光标行书签增删。 */
function toggleBookmark() {
  if (cursorSeq.value > 0) tabs.toggleBookmark(cursorSeq.value)
}

/** showBookmarks ':书签列表开关。 */
function showBookmarks() {
  bmShow.value = !bmShow.value
  bmSel.value = -1
}

/** gotoBookmark 跳书签(折叠组自动展开,居中)。 */
function gotoBookmark(seq: number) {
  bmShow.value = false
  ensureExpanded(seq)
  cursorToSeq(seq)
  const idx = cursorIdx.value
  if (idx >= 0 && scrollEl.value) {
    scrollEl.value.scrollTop = Math.max(0, rowTop(idx) - (viewportH.value - rowHeightAt(idx)) / 2)
  }
}

/** 行号开关 #。 */
const showLineNum = ref(false)
function toggleLineNum() {
  showLineNum.value = !showLineNum.value
}

// ---- 书签列表键盘导航(第四轮验收③:ctrl+j/k 选中+Enter 跳转) ----
const bmSel = ref(-1)
/** bookmarkNav ctrl+j/k:列表开着时移动选中,返回是否消费(未开列表 false 交回全局)。 */
function bookmarkNav(delta: number): boolean {
  if (!bmShow.value) return false
  const n = bmItems.value.length
  if (n === 0) return true
  bmSel.value = Math.min(n - 1, Math.max(0, bmSel.value < 0 ? (delta > 0 ? 0 : n - 1) : bmSel.value + delta))
  return true
}
/** bookmarkActivate Enter:跳转选中书签(未选中不动作)。 */
function bookmarkActivate() {
  if (bmShow.value && bmSel.value >= 0 && bmItems.value[bmSel.value]) {
    gotoBookmark(bmItems.value[bmSel.value].seq)
    bmSel.value = -1
  }
}

defineExpose({
  moveCursor,
  cursorToViewport,
  cursorToSeq,
  anchorCursor,
  cursorToHit,
  gotoTop,
  gotoBottom,
  halfPage,
  pageMove,
  openDetailAtCursor,
  toggleFold,
  startVisual,
  exitVisual,
  yankSelection,
  yank,
  hasSelection,
  toggleBookmark,
  showBookmarks,
  toggleLineNum,
  bookmarkNav,
  bookmarkActivate,
  ctxKey,
  hasCtxMix: ctxAnchorCount,
})
</script>

<template>
  <div ref="scrollEl" class="logview" :class="{ 'kb-nav': ui.kbNav }" @scroll="onScroll" @mouseup="rowMouseUp()">
    <!-- 分栏表头（sticky 置顶,与数据列同宽对齐,验收反馈②;gutter 占位与数据行同构,第三轮验收⑦） -->
    <div v-if="filteredLines.length" class="row header">
      <span class="gutter"
        ><span>&nbsp;</span><span v-if="showLineNum" class="lnum">{{ ' '.repeat(6) }} │</span><span v-if="sourceCol" class="src-chip">&nbsp;</span></span
      >
      <span
        v-for="c in headCells"
        :key="c.field"
        class="cell"
        :class="'f-' + c.field"
        :style="cellW(c.field)"
      >
        {{ c.text }}
        <!-- 列宽拖拽把手(第七轮验收②):message 弹性列无把手 -->
        <span
          v-if="c.field !== 'message'"
          class="col-handle"
          title="拖动调整列宽(落盘 rules.yaml,TUI 同源)"
          @mousedown="startDrag($event, c.field)"
        ></span>
      </span>
    </div>
    <div class="spacer" :style="{ height: totalH + 'px' }"></div>
    <!-- 行用全局 top 定位;.rows 不再做 translateY(双重补偿会让滚动一屏后花屏,Final #1) -->
    <div class="rows" :style="{ height: totalH + 'px' }">
      <template v-for="(item, i) in visItems" :key="item.type === 'line' ? item.ln.seq : 'f' + item.range.startSeq">
        <!-- 折叠占位行(点击展开) -->
        <div
          v-if="item.type === 'fold'"
          class="row fold-row"
          :style="{ top: rowTop(range[0] + i) + 'px', height: rowHeightAt(range[0] + i) + 'px' }"
          @click="unfoldAt(item.range)"
        >
          ▸ {{ item.range.endSeq - item.range.startSeq + 1 }} 行堆栈: {{ item.range.leader }}
        </div>
        <div
          v-else
          class="row"
          :class="{ hit: hitSeqs.has(item.ln.seq), cur: item.ln.seq === cursorSeq, vis: inVisual(item.ln.seq), ctx: mixedView.dimSeqs.has(item.ln.seq) }"
          :style="{ top: rowTop(range[0] + i) + 'px', height: rowHeightAt(range[0] + i) + 'px' }"
          @mousedown="rowMouseDown($event, item.ln.seq)"
          @mousemove="rowMouseMove($event, item.ln.seq)"
          @mouseup="rowMouseUp()"
          @contextmenu="onContext($event, item.ln)"
          @dblclick="openDetail(item.ln)"
        >
        <span class="gutter">
          <span v-if="mixedView.dimSeqs.has(item.ln.seq)" class="ctx-mark">┆</span><span v-else-if="bmSet.has(item.ln.seq)" class="bm">▸</span><span v-else>&nbsp;</span><span
            v-if="showLineNum"
            class="lnum"
            >{{ String(range[0] + i + 1).padStart(6) }} │</span
          ><span v-if="sourceCol" class="src-chip" :title="item.ln.source">{{ item.ln.source }}</span>
        </span>
        <template v-for="c in cellsOf(item.ln)" :key="c.field">
          <span v-if="c.field === 'message'" class="cell f-msg">
            <!-- w 换行:按 wrapSegments 模型切段逐段渲染,模型即像素——
                 CSS pre-wrap 真折行与 JS 行数估算双轨口径漂移会行重叠(第三轮验收⑧) -->
            <template v-if="ui.wrap">
              <div v-for="(seg, wi) in wrapSegments(c.text, msgWidthCh)" :key="wi" class="wline">
                <template v-for="(s, si) in renderSpans(seg)" :key="si">
                  <span v-if="s.hl" class="hl-w" :style="{ color: s.hl }">{{ s.text }}</span>
                  <span v-else-if="s.hit" class="hit-w">{{ s.text }}</span>
                  <template v-else>{{ s.text }}</template>
                </template>
              </div>
            </template>
            <template v-else>
              <template v-for="(s, si) in renderSpans(item.ln.message)" :key="si">
                <span v-if="s.hl" class="hl-w" :style="{ color: s.hl }">{{ s.text }}</span>
                <span v-else-if="s.hit" class="hit-w">{{ s.text }}</span>
                <template v-else>{{ s.text }}</template>
              </template>
            </template>
          </span>
          <span
            v-else
            class="cell"
            :class="['f-' + c.field, c.field === 'level' ? levelClass(item.ln.level) : '']"
            :style="cellW(c.field)"
            >{{ c.text }}</span
          >
        </template>
        </div>
      </template>
    </div>

    <!-- 书签列表(' 键) -->
    <div v-if="bmShow" class="bm-panel">
      <div class="bm-head">书签(Esc 关闭,m 添加 · ctrl+j/k 选中,Enter 跳转)</div>
      <ul>
        <li
          v-for="(b, i) in bmItems"
          :key="b.seq"
          :class="{ sel: i === bmSel }"
          @click="gotoBookmark(b.seq)"
          @mousemove="bmSel = i"
        >
          <span class="bseq">#{{ b.seq }}</span>
          <span class="blabel">{{ b.label }}</span>
        </li>
        <li v-if="!bmItems.length" class="bempty">暂无书签——光标行按 m 添加</li>
      </ul>
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
  display: flex;
  align-items: flex-start;
  height: v-bind('rowH + "px"');
  line-height: v-bind('rowH + "px"');
  white-space: pre;
  padding: 0 10px;
  color: var(--fg, #c8d3e0);
  cursor: default;
}
.row:hover {
  background: var(--hover-bg, #16202e);
}
/* 键盘导航态(2026-11-04):藏鼠标指针 + 悬停底色切透明(鼠标停着不动时 hover 底色
   卡一行碍眼)。hover 色走变量开关而非选择器叠加——:not(.hit) 链会把主场景(过滤态
   鼠标停在命中行)排除、高特异性覆盖会反杀 hit/cur/vis 自身底色;变量切换零特异性
   变化,各行的源码序压制关系原样保留。cursor 需覆盖到行级(.row 自带 cursor:default
   会盖掉容器级 none,指针永远落在行上);规则置样式段末尾,对 .col-handle:hover 等
   同特异性指针规则稳定压制 */
.logview {
  --hover-bg: var(--popup-bg, #16202e);
}
.logview.kb-nav {
  --hover-bg: transparent;
}
.logview.kb-nav,
.logview.kb-nav * {
  cursor: none;
}
.row.hit {
  background: var(--hit-row-bg, rgba(122, 162, 214, 0.12));
}
/* 光标行(TUI Selected 色,左侧 accent 竖条) */
.row.cur {
  background: var(--selected, #30354f);
  box-shadow: inset 2px 0 0 var(--accent, #7dc4e4);
}
/* 可视选择区(TUI Visual 色;与光标行叠加时保持竖条) */
.row.vis {
  background: var(--visual, #005f2f);
}
.row.vis.cur {
  background: var(--visual, #005f2f);
  box-shadow: inset 2px 0 0 var(--accent, #7dc4e4);
}
/* 分栏:固定列定宽 + 两空格分隔;message 弹性(flex 行布局,多行块顶对齐) */
.cell {
  flex: none;
  overflow: hidden;
  white-space: pre;
}
.cell + .cell {
  margin-left: 2ch;
}
.f-msg {
  flex: 1;
  min-width: 0;
  overflow: visible;
  white-space: pre;
}
/* w 换行:模型切段每段一行(高度=行高,与 heightsArr 同源) */
.wline {
  height: v-bind('rowH + "px"');
  line-height: v-bind('rowH + "px"');
  white-space: pre;
}
/* 行首 gutter:书签 ▸ / 行号(TUI 前缀风格) */
.gutter {
  color: var(--dim, #5d7292);
  white-space: pre;
  flex: none;
}
.gutter .bm {
  color: var(--accent, #7dc4e4);
}
/* 来源 chip(k8s 多 pod 合流行=pod 名):定宽 16ch(Final Review C1:max-width 随内容
   收缩会让 gutter 按内容计宽→表头/数据列错位)+2ch 间隔,占宽与 msgWidthCh 的 +18ch 对齐
   (wrap 模型即像素,第三轮验收⑧行重叠教训) */
.gutter .src-chip {
  display: inline-block;
  width: 16ch;
  overflow: hidden;
  text-overflow: ellipsis;
  vertical-align: bottom;
  margin-right: 2ch;
  color: var(--dim, #5d7292);
}
/* 书签列表浮层 */
.bm-panel {
  position: absolute;
  top: 42px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 30;
  min-width: 420px;
  max-width: 70%;
  background: var(--popup-bg, #16202e);
  border: 1px solid var(--popup-border, #2a3a4f);
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
  padding: 8px 10px;
  font-size: var(--font-ui-base, 12px);
}
.bm-head {
  color: var(--dim, #8b98a9);
  margin-bottom: 6px;
}
.bm-panel ul {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 260px;
  overflow-y: auto;
}
.bm-panel li {
  display: flex;
  gap: 8px;
  padding: 3px 6px;
  border-radius: 4px;
  cursor: pointer;
}
.bm-panel li:hover,
.bm-panel li.sel {
  background: var(--popup-bg, #1f2c3f);
}
.bm-panel li.sel {
  box-shadow: inset 2px 0 0 var(--accent, #7dc4e4);
}
.bseq {
  color: var(--accent, #7dc4e4);
  flex: none;
}
.blabel {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.bempty {
  color: var(--dim, #5d7292);
  cursor: default;
}
/* 混入上下文行(TUI DetailDim 同语义):整行半透明盖掉一切内层列色/高亮词,
   时间序内联可见但视觉退后;┆ 标记行首身份。 */
.row.ctx {
  opacity: 0.55;
}
.ctx-mark {
  color: var(--dim, #5d7292);
}
/* 折叠占位行(TUI dim 风格) */
.fold-row {
  color: var(--dim, #8b98a9);
  font-style: italic;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: pre;
}
.fold-row:hover {
  color: var(--accent, #7dc4e4);
}
/* 表头行:sticky 置顶 */
.row.header {
  position: sticky;
  top: 0;
  z-index: 5;
  background: var(--bg, #10161f);
  color: var(--dim, #8b98a9);
  border-bottom: 1px solid var(--popup-border, #1e2837);
  cursor: default;
}
.row.header .cell {
  white-space: pre;
  position: relative;
  overflow: visible; /* 覆盖 .cell 的 hidden,把手可悬出右缘 */
}
/* 列宽拖拽把手 */
.col-handle {
  position: absolute;
  top: -2px;
  right: -3px;
  width: 7px;
  height: calc(100% + 4px);
  cursor: col-resize;
  z-index: 2;
}
.col-handle:hover,
.col-handle:active {
  background: var(--accent, #7dc4e4);
  opacity: 0.7;
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
.f-traceId {
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
  font-size: var(--font-ui-base, 12px);
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
