<script lang="ts">
  // 搜索栏：防抖 300ms 触发 onSearch；语法错误红显；命中计数与上/下跳转。
  import type { QueryMeta } from '../lib/backend'

  export let meta: QueryMeta | null = null
  export let hitsCount = 0
  export let hitPos = -1
  export let onSearch: (q: string) => void | Promise<void> = () => {}
  export let onJump: (delta: number) => void = () => {}

  let input: HTMLInputElement
  let value = ''
  let timer: ReturnType<typeof setTimeout>

  function onInput() {
    clearTimeout(timer)
    timer = setTimeout(() => onSearch(value), 300)
  }

  export function focus() {
    input.focus()
  }

  async function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      clearTimeout(timer)
      await onSearch(value) // 先完成搜索再跳转——Enter 跳的是新命中集（Final Review #10）
      onJump(e.shiftKey ? -1 : 1)
    }
    if (e.key === 'Escape') {
      input.blur()
    }
  }
</script>

<div class="searchbar">
  <span class="prompt">/</span>
  <input
    bind:this={input}
    bind:value
    on:input={onInput}
    on:keydown={onKeydown}
    placeholder="搜索：关键词 / level:ERROR AND time:1h / traceId:xxx"
    class:error={meta?.parseError}
  />
  {#if meta?.parseError}
    <span class="err" title={meta.parseError}>✗ 语法错误</span>
  {/if}
  {#if !meta?.parseError && value.trim()}
    <span class="count">{hitPos >= 0 ? hitPos + 1 : 0}/{hitsCount}</span>
  {/if}
  <button class="nav" on:click={() => onJump(-1)} disabled={hitsCount === 0}>↑</button>
  <button class="nav" on:click={() => onJump(1)} disabled={hitsCount === 0}>↓</button>
</div>

<style>
  .searchbar {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 10px;
  }
  .prompt {
    color: #46586e;
    font-family: 'SF Mono', Menlo, monospace;
  }
  input {
    flex: 1;
    background: #10161f;
    border: 1px solid #24354a;
    border-radius: 4px;
    color: #e6e6e6;
    padding: 3px 8px;
    font-size: 14px;
    font-family: 'SF Mono', Menlo, monospace;
  }
  input:focus {
    outline: none;
    border-color: #2a5db0;
  }
  input.error {
    border-color: #8a3038;
  }
  .err {
    color: #ff6b6b;
    font-size: 13px;
    white-space: nowrap;
  }
  .count {
    color: #8b98a9;
    font-size: 13px;
    white-space: nowrap;
  }
  .nav {
    background: #1c2836;
    color: #8b98a9;
    border: 1px solid #2a3a4f;
    border-radius: 4px;
    cursor: pointer;
    padding: 2px 8px;
  }
  .nav:disabled {
    opacity: 0.4;
    cursor: default;
  }
</style>
