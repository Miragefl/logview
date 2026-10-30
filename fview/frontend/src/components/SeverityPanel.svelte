<script lang="ts">
  // 级别分布统计面板（Gonzo 式）：分段过滤 + 各级别横条（宽度=占比），点击条目过滤。
  import type { LineDTO } from '../lib/stores/tabs'
  import type { LevelFilterMode } from '../lib/filter'

  export let lines: LineDTO[] = []
  export let levelMode: LevelFilterMode = 'A'
  export let onLevel: (m: LevelFilterMode) => void = () => {}

  $: counts = countLevels(lines)
  $: max = Math.max(1, ...Object.values(counts))

  const SEGMENTS: Array<{ m: LevelFilterMode; label: string }> = [
    { m: 'A', label: '全部' },
    { m: 'E', label: '错误' },
    { m: 'W', label: '警告' },
    { m: 'I', label: '信息' },
  ]

  const LEVEL_ROWS = [
    { key: 'error', mode: 'E' as LevelFilterMode, label: '错误' },
    { key: 'warn', mode: 'W' as LevelFilterMode, label: '警告' },
    { key: 'info', mode: 'I' as LevelFilterMode, label: '信息' },
    { key: 'debug', mode: 'A' as LevelFilterMode, label: '调试' },
    { key: 'other', mode: 'A' as LevelFilterMode, label: '其他' },
  ]

  function countLevels(ls: LineDTO[]) {
    const c = { error: 0, warn: 0, info: 0, debug: 0, other: 0 }
    for (const ln of ls) {
      const lv = ln.level.toUpperCase()
      if (lv === 'ERROR' || lv === 'ERR' || lv === 'FATAL') c.error++
      else if (lv === 'WARN' || lv === 'WARNING') c.warn++
      else if (lv === 'INFO') c.info++
      else if (lv === 'DEBUG' || lv === 'DBG') c.debug++
      else c.other++
    }
    return c
  }

  // 点击条目：该级别档位；再点同档回全部
  function pick(m: LevelFilterMode) {
    onLevel(levelMode === m ? 'A' : m)
  }
</script>

<div class="sev">
  <h3>级别分布</h3>
  <div class="segs">
    {#each SEGMENTS as s (s.m)}
      <button class="seg" class:on={levelMode === s.m} on:click={() => (s.m === 'A' ? onLevel('A') : pick(s.m))}>
        {s.label}
      </button>
    {/each}
  </div>
  <div class="bars">
    {#each LEVEL_ROWS as r (r.key)}
      <button class="row" class:dim={counts[r.key] === 0} on:click={() => pick(r.mode)}>
        <span class="dot {r.key}" />
        <span class="name">{r.label}</span>
        <span class="count">{counts[r.key].toLocaleString()}</span>
        <span class="track"><span class="bar {r.key}" style:width="{Math.round((counts[r.key] / max) * 100)}%" /></span>
      </button>
    {/each}
  </div>
</div>

<style>
  .sev {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  h3 {
    margin: 0;
    font-size: 0.6875rem;
    color: var(--faint);
    text-transform: uppercase;
    letter-spacing: 1px;
  }
  .segs {
    display: flex;
    background: var(--muted);
    border-radius: 6px;
    padding: 2px;
    gap: 2px;
  }
  .seg {
    flex: 1;
    background: none;
    border: none;
    border-radius: 5px;
    color: var(--muted-fg);
    cursor: pointer;
    font-size: 0.75rem;
    padding: 4px 0;
  }
  .seg.on {
    background: var(--card);
    color: var(--fg);
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.08);
  }
  .bars {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .row {
    display: grid;
    grid-template-columns: 10px 44px 52px 1fr;
    gap: 6px;
    align-items: center;
    background: none;
    border: none;
    cursor: pointer;
    padding: 3px 4px;
    border-radius: 4px;
    font-size: 0.75rem;
    text-align: left;
  }
  .row:hover {
    background: var(--hover);
  }
  .row.dim {
    opacity: 0.45;
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
  }
  .dot.error { background: var(--error); }
  .dot.warn { background: #d97706; }
  .dot.info { background: var(--primary); }
  .dot.debug { background: var(--muted-fg); }
  .dot.other { background: var(--faint); }
  .name {
    color: var(--fg);
  }
  .count {
    color: var(--muted-fg);
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
  .track {
    background: #eceade;
    border-radius: 3px;
    height: 8px;
    overflow: hidden;
  }
  .bar {
    display: block;
    height: 100%;
    border-radius: 3px;
  }
  .bar.error { background: #f0a4a4; }
  .bar.warn { background: #f3cf8f; }
  .bar.info { background: #a8c4ec; }
  .bar.debug { background: #c5c9ce; }
  .bar.other { background: #d8dbdf; }
</style>
