<script lang="ts">
  // 虚拟滚动日志视图：固定行高 windowing + 级别/隐藏词过滤（windowing 基于过滤后行集）
  // + 字段列显隐 + ANSI 着色 + 搜索命中/关键词高亮 + 跟随滚底。
  import { levelClass } from '../lib/levelColor'
  import { levelFilter, hidesFilter, highlightRanges, type LevelFilterMode } from '../lib/filter'
  import type { LineDTO } from '../lib/stores/tabs'

  export let lines: LineDTO[] = []
  export let rowHeight = 20
  export let fontSize = 13 // 日志字号（与菜单字号独立调节）
  export let uiZoom = 1 // 菜单/UI 缩放（日志区反向抵消，日志显示只由 fontSize 决定）
  export let follow = false
  export let onFollowPause: (() => void) | undefined = undefined
  export let hits: number[] = []
  export let currentLine = -1
  export let levelMode: LevelFilterMode = 'A'
  export let hidesList: string[] = []
  export let highlightWords: string[] = []
  export let visibleFields: string[] = ['time', 'level', 'message']
  export let onLineClick: ((ln: LineDTO) => void) | undefined = undefined

  interface Row {
    ln: LineDTO
    idx: number // 原始行下标（Go 侧 MatchLines 的索引域）
  }

  let viewport: HTMLElement
  let scrollTop = 0
  let viewportH = 600
  let autoFollow = false
  const OVERSCAN = 20
  const NEAR_BOTTOM = 30

  $: hitSet = new Set(hits)
  $: filtered =
    levelMode === 'A' && hidesList.length === 0
      ? lines.map((ln, idx) => ({ ln, idx }))
      : lines
          .map((ln, idx) => ({ ln, idx }))
          .filter((r) => levelFilter(levelMode, r.ln.level) && hidesFilter(hidesList, r.ln.raw))
  $: first = Math.max(0, Math.floor(scrollTop / rowHeight) - OVERSCAN)
  $: visibleCount = Math.ceil(viewportH / rowHeight) + OVERSCAN * 2
  $: slice = filtered.slice(first, first + visibleCount)
  $: totalH = filtered.length * rowHeight
  $: currentRowPos = currentLine >= 0 ? filtered.findIndex((r) => r.idx === currentLine) : -1

  $: if (follow) autoFollow = true
  $: if (autoFollow && filtered.length && viewport) {
    viewport.scrollTop = totalH
  }
  $: if (currentRowPos >= 0 && viewport) {
    viewport.scrollTop = Math.max(0, currentRowPos * rowHeight - viewportH / 2)
  }

  // vim 滚动控制（App 快捷键调用）
  export function scrollLines(n: number) {
    if (viewport) viewport.scrollTop += n * rowHeight
  }

  export function scrollToRatio(t: number) {
    if (viewport) viewport.scrollTop = Math.max(0, (totalH - viewport.clientHeight) * t)
  }

  export function lineHeight() {
    return rowHeight
  }

  function onScroll(e: Event) {
    const el = e.currentTarget as HTMLElement
    scrollTop = el.scrollTop
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < NEAR_BOTTOM
    if (autoFollow && !atBottom) {
      autoFollow = false
      onFollowPause?.()
    }
  }

  // 关键词高亮：后端已 StripANSI（与 TUI 一致），行文本恒为纯文本——
  // 统一 escapeHtml 后按区间切 span（多词多色 hl-0/1/2），无注入面。
  function renderText(ln: LineDTO): string {
    const escaped = escapeHtml(ln.raw)
    if (!highlightWords.length) return escaped
    const ranges = highlightRanges(ln.raw, highlightWords)
    if (!ranges.length) return escaped
    let out = ''
    let pos = 0
    ranges.forEach(([s, e], i) => {
      if (s < pos) return // 重叠区间跳过
      const wordIdx = i % 3
      out += escapeHtml(ln.raw.slice(pos, s))
      out += `<span class="hl-${wordIdx}">${escapeHtml(ln.raw.slice(s, e))}</span>`
      pos = e
    })
    out += escapeHtml(ln.raw.slice(pos))
    return out
  }

  function escapeHtml(s: string): string {
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  }

  const FIELD_LABEL: Record<string, string> = {}
  function fieldVal(ln: LineDTO, f: string): string {
    switch (f) {
      case 'time':
        return ln.time ? ln.time.slice(11) : ''
      case 'level':
        return ln.level
      case 'thread':
        return ln.thread
      case 'traceId':
        return ln.traceId
      case 'logger':
        return ln.logger
      case 'source':
        return ''
      default:
        return ''
    }
  }
</script>

<div
  class="viewport"
  bind:this={viewport}
  bind:clientHeight={viewportH}
  on:scroll={onScroll}
  style:font-size="{fontSize}px"
  style:zoom={String(1 / uiZoom)}
>
  <div class="spacer" style:height="{totalH}px">
    <div class="rows" style:transform="translateY({first * rowHeight}px)">
      {#each slice as r (r.ln.seq)}
        <div
          class="row {levelClass(r.ln.level)}"
          class:hit={hitSet.has(r.idx)}
          class:hit-current={r.idx === currentLine}
          style:height="{rowHeight}px"
          role="button"
          tabindex="-1"
          on:click={() => onLineClick?.(r.ln)}
          on:keydown={(e) => e.key === 'Enter' && onLineClick?.(r.ln)}
        >
          <span class="lineno">{r.ln.seq}</span>
          {#if visibleFields.includes('time')}<span class="col time">{fieldVal(r.ln, 'time')}</span>{/if}
          {#if visibleFields.includes('level')}<span class="col level">{r.ln.level}</span>{/if}
          {#if visibleFields.includes('thread')}<span class="col">{r.ln.thread}</span>{/if}
          {#if visibleFields.includes('traceId')}<span class="col">{r.ln.traceId}</span>{/if}
          {#if visibleFields.includes('logger')}<span class="col logger">{r.ln.logger}</span>{/if}
          {#if visibleFields.includes('message')}
            <span class="text">{@html renderText(r.ln)}</span>
          {/if}
        </div>
      {/each}
    </div>
  </div>
</div>

<style>
  .viewport {
    width: 100%;
    height: 100%;
    overflow-y: auto;
    font-family: 'SF Mono', Menlo, monospace;
    position: relative;
  }
  .spacer {
    position: relative;
  }
  .rows {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0 10px;
    white-space: nowrap;
    font-size: inherit;
  }
  .lineno {
    flex: 0 0 64px;
    text-align: right;
    color: #b8bdc4;
    user-select: none;
  }
  .row {
    cursor: pointer;
  }
  .row:hover {
    background: #f0f0e7;
  }
  .row.hit {
    background: #f3efd3;
  }
  .row.hit-current {
    background: #f5d76e;
    outline: 2px solid #b45309;
  }
  .col {
    color: #6b7280;
    flex-shrink: 0;
    max-width: 160px;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .col.time {
    color: #9aa0a8;
  }
  .col.logger {
    color: #6b7280;
  }
  .col.level {
    flex: 0 0 64px;
    text-align: center;
    border-radius: 3px;
    font-size: 0.8125rem;
    padding: 0 2px;
  }
  .row.lv-error .col.level {
    color: var(--level-error, #dc2626);
    border: 1px solid var(--level-error, #fdeaea);
  }
  .row.lv-warn .col.level {
    color: var(--level-warn, #b45309);
    border: 1px solid var(--level-warn, #f6e8c8);
  }
  .row.lv-info .col.level {
    color: var(--level-info, #2563eb);
    border: 1px solid var(--level-info, #e2ecf8);
  }
  .row.lv-debug .col.level {
    color: var(--level-debug, #6b7280);
    border: 1px solid var(--level-debug, #eef0ea);
  }
  .row.lv-error .text {
    color: #dc2626;
  }
  .text {
    overflow: hidden;
    text-overflow: ellipsis;
    flex: 1;
  }
  .text :global(.bold) {
    font-weight: 700;
  }
  .text :global(.hl-0) {
    background: #f6e8c8;
    color: #92670f;
    border-radius: 2px;
  }
  .text :global(.hl-1) {
    background: #e2ecf8;
    color: #2563eb;
    border-radius: 2px;
  }
  .text :global(.hl-2) {
    background: #3a1e5c;
    color: #e89bf5;
    border-radius: 2px;
  }
  .text :global(.fg-red) {
    color: #dc2626;
  }
  .text :global(.fg-green) {
    color: #15803d;
  }
  .text :global(.fg-yellow) {
    color: #a16207;
  }
  .text :global(.fg-blue) {
    color: #2563eb;
  }
  .text :global(.fg-magenta) {
    color: #9333ea;
  }
  .text :global(.fg-cyan) {
    color: #0e7490;
  }
  .text :global(.fg-white) {
    color: #1a1d21;
  }
  .text :global(.fg-bright-red) {
    color: #dc2626;
  }
  .text :global(.fg-bright-green) {
    color: #15803d;
  }
  .text :global(.fg-bright-yellow) {
    color: #92670f;
  }
  .text :global(.fg-bright-blue) {
    color: #2563eb;
  }
  .text :global(.fg-bright-magenta) {
    color: #e89bf5;
  }
  .text :global(.fg-bright-cyan) {
    color: #7fe8e8;
  }
  .text :global(.fg-bright-white) {
    color: #ffffff;
  }
</style>

