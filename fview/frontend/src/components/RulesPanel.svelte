<script lang="ts">
  // 解析规则编辑面板（rules.yaml 共享正本）：name/pattern 行内编辑 + 增删 + 保存。
  // 保存后已开 tab 不受影响，新 tab 用新规则解析。
  import { getRules, saveRules } from '../lib/backend'

  export let onClose: () => void = () => {}

  interface RuleRow {
    name: string
    pattern: string
    fallback: boolean
  }

  let rows: RuleRow[] = []
  let dirty = false
  let saving = false
  let savedAt = ''

  getRules().then((v) => {
    rows = (v ?? []).map((r) => ({ name: r.name, pattern: r.pattern, fallback: r.fallback }))
  })

  function markDirty() {
    dirty = true
  }

  function add() {
    rows = [...rows, { name: 'new-rule', pattern: '(?P<message>.*)', fallback: false }]
    markDirty()
  }

  function remove(i: number) {
    rows = rows.filter((_, idx) => idx !== i)
    markDirty()
  }

  async function save() {
    saving = true
    try {
      await saveRules(rows.map((r) => ({ ...r })))
      dirty = false
      savedAt = new Date().toLocaleTimeString()
    } finally {
      saving = false
    }
  }
</script>

<div class="overlay" role="presentation" on:click|self={onClose}>
  <div class="panel" role="dialog" aria-label="解析规则配置">
    <div class="head">
      <span>解析规则（共享 rules.yaml，保存后新 tab 生效）</span>
      <button class="mini" on:click={onClose}>&times;</button>
    </div>
    <div class="list">
      <div class="row head-row">
        <span>名称</span>
        <span>正则（命名捕获组：time/level/thread/traceId/logger/message）</span>
        <span>兜底</span>
        <span />
      </div>
      {#each rows as r, i (i)}
        <div class="row">
          <input bind:value={r.name} on:input={markDirty} />
          <input class="mono" bind:value={r.pattern} on:input={markDirty} />
          <input type="checkbox" bind:checked={r.fallback} on:change={markDirty} />
          <button class="mini" on:click={() => remove(i)}>删</button>
        </div>
      {/each}
    </div>
    <div class="foot">
      <button class="mini" on:click={add}>+ 新增规则</button>
      <span class="hint">{savedAt ? `已保存 ${savedAt}` : '提示：保存写回 ~/.config/logview/rules.yaml，TUI 下次启动同样生效'}</span>
      <button class="primary" disabled={!dirty || saving} on:click={save}>
        {saving ? '保存中…' : '保存'}
      </button>
    </div>
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
  }
  .panel {
    width: 780px;
    max-height: 80vh;
    background: var(--popup-bg, #16202e);
    border: 1px solid var(--popup-border, #2a3a4f);
    border-radius: 10px;
    display: flex;
    flex-direction: column;
    color: var(--fg, #e6e6e6);
  }
  .head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 10px 14px;
    border-bottom: 1px solid #1c2836;
    font-size: 13px;
  }
  .list {
    overflow-y: auto;
    padding: 8px 14px;
    flex: 1;
  }
  .row {
    display: grid;
    grid-template-columns: 140px 1fr 44px 44px;
    gap: 8px;
    align-items: center;
    padding: 4px 0;
    font-size: 12px;
  }
  .head-row {
    color: #46586e;
    font-size: 11px;
  }
  input {
    background: #10161f;
    border: 1px solid #24354a;
    border-radius: 4px;
    color: #e6e6e6;
    padding: 3px 8px;
    font-size: 12px;
    width: 100%;
    box-sizing: border-box;
  }
  input.mono {
    font-family: 'SF Mono', Menlo, monospace;
  }
  .mini {
    background: #1c2836;
    color: #8b98a9;
    border: 1px solid #2a3a4f;
    border-radius: 4px;
    cursor: pointer;
    padding: 2px 8px;
    font-size: 11px;
  }
  .foot {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 14px;
    border-top: 1px solid #1c2836;
  }
  .hint {
    flex: 1;
    color: #46586e;
    font-size: 11px;
  }
  .primary {
    background: #2a5db0;
    color: #fff;
    border: none;
    border-radius: 4px;
    padding: 4px 16px;
    cursor: pointer;
    font-size: 12px;
  }
  .primary:disabled {
    opacity: 0.5;
    cursor: default;
  }
</style>
