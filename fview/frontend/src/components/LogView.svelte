<script lang="ts">
  // 虚拟滚动日志视图（M2 Task 7）：固定行高 windowing，仅渲染可视区 ±OVERSCAN 行；
  // 行内 ANSI 着色 + 级别色；超长行单行截断（1MB 单行不冻结——Review Focus 1）。
  import { ansiToHtml } from '../lib/ansi'
  import { levelClass } from '../lib/levelColor'
  import type { LineDTO } from '../lib/stores/tabs'

  export let lines: LineDTO[] = []
  export let rowHeight = 20

  let scrollTop = 0
  let viewportH = 600
  const OVERSCAN = 20

  $: first = Math.max(0, Math.floor(scrollTop / rowHeight) - OVERSCAN)
  $: visibleCount = Math.ceil(viewportH / rowHeight) + OVERSCAN * 2
  $: slice = lines.slice(first, first + visibleCount)
  $: totalH = lines.length * rowHeight

  function onScroll(e: WheelEvent) {
    scrollTop = (e.target as HTMLElement).scrollTop
  }
</script>

<div class="viewport" bind:clientHeight={viewportH} on:scroll={onScroll}>
  <div class="spacer" style:height="{totalH}px">
    <div class="rows" style:transform="translateY({first * rowHeight}px)">
      {#each slice as ln (ln.seq)}
        <div class="row {levelClass(ln.level)}" style:height="{rowHeight}px">
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
