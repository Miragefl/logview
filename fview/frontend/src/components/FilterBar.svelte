<script lang="ts">
  // 过滤栏（简化版）：隐藏词 + 字段列开关；级别过滤由侧栏统计面板承担。
  export let hides: string[] = []
  export let fields: Array<{ name: string; visible: boolean }> = []

  export let onSaveHides: (list: string[]) => void = () => {}
  export let onToggleField: (name: string) => void = () => {}

  let hidesInput = hides.join(', ')
  let showFields = false

  function saveHides() {
    onSaveHides(
      hidesInput
        .split(/[,，]/)
        .map((x) => x.trim())
        .filter(Boolean)
    )
  }
</script>

<div class="filterrow">
  <span class="lbl">隐藏</span>
  <input bind:value={hidesInput} placeholder="逗号分隔关键词（写回共享 rules.yaml）" />
  <button class="mini" on:click={saveHides}>保存</button>
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
  .filterrow {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .lbl {
    color: #9aa0a8;
    font-size: 0.8125rem;
  }
  .filterrow input {
    background: #ffffff;
    border: 1px solid #d5d5ca;
    border-radius: 4px;
    color: #1a1d21;
    padding: 3px 8px;
    font-size: 0.8125rem;
    width: 300px;
  }
  .mini {
    background: #e2e2d8;
    color: #5f6672;
    border: 1px solid #c7c7bc;
    border-radius: 4px;
    cursor: pointer;
    padding: 3px 10px;
    font-size: 0.8125rem;
    white-space: nowrap;
  }
  .mini:hover {
    background: #d8d8cd;
  }
  .fields-wrap {
    position: relative;
  }
  .fields-pop {
    position: absolute;
    top: 30px;
    right: 0;
    background: #ffffff;
    border: 1px solid #c7c7bc;
    border-radius: 6px;
    padding: 8px 12px;
    display: flex;
    flex-direction: column;
    gap: 4px;
    z-index: 10;
    min-width: 110px;
    font-size: 0.8125rem;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  }
</style>
