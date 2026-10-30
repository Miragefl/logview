<script lang="ts">
  // tab 栏：浏览器式标签页（M2 Task 6；点击切换、×关闭）
  import { tabsStore, activeTab } from '../lib/stores/tabs'
  import { closeTab } from '../lib/backend'
</script>

<div class="tabbar">
  {#each $tabsStore as t (t.info.id)}
    <button
      class="tab"
      class:active={t.info.id === $activeTab}
      on:click={() => activeTab.set(t.info.id)}
      title={t.info.label}
    >
      <span class="label">{t.info.label}</span>
      <span class="lines">{t.info.lines}</span>
      <span
        class="close"
        on:click|stopPropagation={() => closeTab(t.info.id)}
        on:keydown|preventDefault={() => {}}>&times;</span
      >
    </button>
  {/each}
</div>

<style>
  .tabbar {
    display: flex;
    align-items: flex-end;
    gap: 2px;
    overflow-x: auto;
    flex: 1;
  }
  .tab {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 10px;
    background: #ffffff;
    color: #6b7280;
    border: none;
    border-radius: 6px 6px 0 0;
    cursor: pointer;
    font-size: 14px;
    white-space: nowrap;
    max-width: 220px;
  }
  .tab.active {
    background: #e8e8de;
    color: #1a1d21;
  }
  .label {
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 140px;
  }
  .lines {
    opacity: 0.5;
    font-size: 13px;
  }
  .close {
    border-radius: 3px;
    padding: 0 3px;
  }
  .close:hover {
    background: #d8d8cd;
    color: #fff;
  }
</style>
