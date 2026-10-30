<script lang="ts">
  // 虚拟滚动日志视图：固定行高 windowing + ANSI 着色 + 级别色 + 跟随滚底。
  // follow 为期望态（父级按钮）；用户向上滚动即暂停跟随，贴底或父级重置时恢复。
  import { ansiToHtml } from '../lib/ansi'
  import { levelClass } from '../lib/levelColor'
  import type { LineDTO } from '../lib/stores/tabs'

  export let lines: LineDTO[] = []
  export let rowHeight = 20
  export let follow = false
  export let onFollowPause: (() => void) | undefined = undefined
  export let hits: number[] = []
  export let currentLine = -1

  let viewport: HTMLElement
  let scrollTop = 0
  let viewportH = 600
  let autoFollow = false
  const OVERSCAN = 20
  const NEAR_BOTTOM = 30 // 距底多少像素内视为"贴底"

  $: hitSet = new Set(hits)

  $: first = Math.max(0, Math.floor(scrollTop / rowHeight) - OVERSCAN)
  $: visibleCount = Math.ceil(viewportH / rowHeight) + OVERSCAN * 2
  $: slice = lines.slice(first, first + visibleCount)
  $: totalH = lines.length * rowHeight

  // 期望态变化：开跟随时立即贴底
  $: if (follow) autoFollow = true

  // 新行到达且自动跟随时贴底（在 DOM 更新后执行）
  $: if (autoFollow && lines.length && viewport) {
    viewport.scrollTop = totalH
  }

  // 当前命中变化：滚动定位到该行（视口中线）
  $: if (currentLine >= 0 && viewport) {
    viewport.scrollTop = Math.max(0, currentLine * rowHeight - viewportH / 2)
  }

  function onScroll(e: WheelEvent) {
    const el = e.target as HTMLElement
    scrollTop = el.scrollTop
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < NEAR_BOTTOM
    if (autoFollow && !atBottom) {
      autoFollow = false
      onFollowPause?.()
    } else if (!autoFollow && atBottom && follow === false) {
      // 用户手动滚回底部且未开跟随：仅更新滚动位置
    }
  }
</script>

<div class="viewport" bind:this={viewport} bind:clientHeight={viewportH} on:scroll={onScroll}>
  <div class="spacer" style:height="{totalH}px">
    <div class="rows" style:transform="translateY({first * rowHeight}px)">
      {#each slice as ln, i (ln.seq)}
        <div
          class="row {levelClass(ln.level)}"
          class:hit={hitSet.has(first + i)}
          class:hit-current={first + i === currentLine}
          style:height="{rowHeight}px"
        >
          <span class="level">{ln.level}</span>
          <span class="text">{@html ansiToHtml(ln.raw)}</span>
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
  .level {
    flex: 0 0 64px;
    text-align: center;
    border-radius: 3px;
    font-size: 11px;
    padding: 0 2px;
  }
  .row.lv-error .level {
    background: #5c1e24;
    color: #ff6b6b;
  }
  .row.lv-warn .level {
    background: #5c4a1e;
    color: #ffb84d;
  }
  .row.lv-info .level {
    background: #1e3a5c;
    color: #6bb9ff;
  }
  .row.lv-debug .level {
    background: #2a2f36;
    color: #8b98a9;
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
