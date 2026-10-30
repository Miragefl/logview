<script lang="ts">
  // 虚拟滚动日志视图：固定行高 windowing + 级别/隐藏词过滤（windowing 基于过滤后行集）
  // + 字段列显隐 + ANSI 着色 + 搜索命中/关键词高亮 + 跟随滚底。
  import { ansiToHtml } from '../lib/ansi'
  import { levelClass } from '../lib/levelColor'
  import { levelFilter, hidesFilter, highlightRanges, type LevelFilterMode } from '../lib/filter'
  import type { LineDTO } from '../lib/stores/tabs'

  export let lines: LineDTO[] = []
  export let rowHeight = 20
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

  function onScroll(e: WheelEvent) {
    const el = e.target as HTMLElement
    scrollTop = el.scrollTop
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < NEAR_BOTTOM
    if (autoFollow && !atBottom) {
      autoFollow = false
      onFollowPause?.()
    }
  }

  // 关键词高亮：仅纯文本行（无 ANSI）按区间切 span；多词多色（hl-0/hl-1/...）
  function renderText(ln: LineDTO): string {
    if (!highlightWords.length) return escapeHtml(ln.raw)
    if (ln.raw.includes('\x1b[')) return ansiToHtml(ln.raw)
    const ranges = highlightRanges(ln.raw, highlightWords)
    if (!ranges.length) return escapeHtml(ln.raw)
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

<div class="viewport" bind:this={viewport} bind:clientHeight={viewportH} on:scroll={onScroll}>
  <div class="spacer" style:height="{totalH}px">
    <div class="rows" style:transform="translateY({first * rowHeight}px)">
      {#each slice as r (r.ln.seq)}
        <div
          class="row {levelClass(r.ln.level)}"
          class:hit={hitSet.has(r.idx)}
          class:hit-current={r.idx === currentLine}
          style:height="{rowHeight}px"
          on:click={() => onLineClick?.(r.ln)}
        >
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
    font-size: 12px;
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
  }
  .row {
    cursor: pointer;
  }
  .row:hover {
    background: #1a2534;
  }
  .row.hit {
    background: #3a3214;
  }
  .row.hit-current {
    background: #5c4a1e;
    outline: 1px solid #ffb84d;
  }
  .col {
    color: #8b98a9;
    flex-shrink: 0;
    max-width: 160px;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .col.time {
    color: #46586e;
  }
  .col.logger {
    color: #6b7f95;
  }
  .col.level {
    flex: 0 0 64px;
    text-align: center;
    border-radius: 3px;
    font-size: 11px;
    padding: 0 2px;
  }
  .row.lv-error .col.level {
    color: var(--level-error, #ff6b6b);
    border: 1px solid var(--level-error, #5c1e24);
  }
  .row.lv-warn .col.level {
    color: var(--level-warn, #ffb84d);
    border: 1px solid var(--level-warn, #5c4a1e);
  }
  .row.lv-info .col.level {
    color: var(--level-info, #6bb9ff);
    border: 1px solid var(--level-info, #1e3a5c);
  }
  .row.lv-debug .col.level {
    color: var(--level-debug, #8b98a9);
    border: 1px solid var(--level-debug, #2a2f36);
  }
  .row.lv-error .text {
    color: #ff8585;
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
    background: #5c4a1e;
    color: #ffe066;
    border-radius: 2px;
  }
  .text :global(.hl-1) {
    background: #1e3a5c;
    color: #7cc0ff;
    border-radius: 2px;
  }
  .text :global(.hl-2) {
    background: #3a1e5c;
    color: #e89bf5;
    border-radius: 2px;
  }
  .text :global(.fg-red) {
    color: #e05555;
  }
  .text :global(.fg-green) {
    color: #5cb878;
  }
  .text :global(.fg-yellow) {
    color: #d8c04a;
  }
  .text :global(.fg-blue) {
    color: #5a9bd8;
  }
  .text :global(.fg-magenta) {
    color: #c76bd8;
  }
  .text :global(.fg-cyan) {
    color: #4ec9c9;
  }
  .text :global(.fg-white) {
    color: #e6e6e6;
  }
  .text :global(.fg-bright-red) {
    color: #ff6b6b;
  }
  .text :global(.fg-bright-green) {
    color: #7ee29a;
  }
  .text :global(.fg-bright-yellow) {
    color: #ffe066;
  }
  .text :global(.fg-bright-blue) {
    color: #7cc0ff;
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

