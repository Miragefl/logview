<script lang="ts">
  // 过滤栏：级别五档（E/W/I/D/A，语义对齐 TUI）+ 隐藏词编辑 + 字段列开关。
  import type { LevelFilterMode } from '../lib/filter'

  export let levelMode: LevelFilterMode = 'A'
  export let hides: string[] = []
  export let fields: Array<{ name: string; visible: boolean }> = []

  export let onLevel: (m: LevelFilterMode) => void = () => {}
  export let onSaveHides: (list: string[]) => void = () => {}
  export let onToggleField: (name: string) => void = () => {}

  let hidesInput = hides.join(', ')
  let showFields = false

  const MODES: Array<{ m: LevelFilterMode; tip: string }> = [
    { m: 'E', tip: '仅 ERROR' },
    { m: 'W', tip: 'ERROR+WARN' },
    { m: 'I', tip: '去 DEBUG' },
    { m: 'D', tip: '全部级别' },
    { m: 'A', tip: '取消过滤' },
  ]

  function saveHides() {
    onSaveHides(
      hidesInput
        .split(/[,，]/)
        .map((s) => s.trim())
        .filter(Boolean)
    )
  }
</script>

<div class="filterbar">
  <div class="levels">
    {#each MODES as { m, tip }}
      <button class="lv" class:on={levelMode === m} title={tip} on:click={() => onLevel(m)}>{m}</button>
    {/each}
  </div>
  <div class="hides">
    <span class="lbl">隐藏</span>
    <input bind:value={hidesInput} placeholder="逗号分隔关键词" />
    <button class="mini" on:click={saveHides}>保存</button>
  </div>
  <div class="fields-wrap">
    <button class="mini" on:click={() => (showFields = !showFields)}>字段 ▾</button>
    {#if showFields}
      <div class="fields-pop">
        {#each fields as f (f.name)}
          <label>
            <input type="checkbox" checked={f.visible} on:change={() => onToggleField(f.name)} />
            {f.name}
          </label>
        {/each}
      </div>
    {/if}
  </div>
</div>

<style>
  .filterbar {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 4px 10px;
    font-size: 14px;
  }
  .levels {
    display: flex;
    gap: 2px;
  }
  .lv {
    width: 24px;
    height: 20px;
    background: #1c2836;
    color: #8b98a9;
    border: 1px solid #2a3a4f;
    border-radius: 3px;
    cursor: pointer;
  }
  .lv.on {
    background: #2a5db0;
    color: #fff;
    border-color: #3671cc;
  }
  .hides {
    display: flex;
    align-items: center;
    gap: 4px;
    flex: 1;
  }
  .lbl {
    color: #46586e;
  }
  .hides input {
    background: #10161f;
    border: 1px solid #24354a;
    border-radius: 4px;
    color: #e6e6e6;
    padding: 2px 8px;
    font-size: 13px;
    flex: 1;
    max-width: 320px;
  }
  .mini {
    background: #1c2836;
    color: #8b98a9;
    border: 1px solid #2a3a4f;
    border-radius: 4px;
    cursor: pointer;
    padding: 2px 8px;
    font-size: 13px;
  }
  .fields-wrap {
    position: relative;
  }
  .fields-pop {
    position: absolute;
    top: 26px;
    right: 0;
    background: #16202e;
    border: 1px solid #2a3a4f;
    border-radius: 6px;
    padding: 8px 12px;
    display: flex;
    flex-direction: column;
    gap: 4px;
    z-index: 10;
    min-width: 120px;
  }
</style>
