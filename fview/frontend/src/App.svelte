<script lang="ts">
  // fview 主框架：顶栏（打开按钮 + tab 栏）+ 主区（欢迎页/虚拟滚动日志视图）。
  import { onMount } from 'svelte'
  import TabBar from './components/TabBar.svelte'
  import Welcome from './components/Welcome.svelte'
  import LogView from './components/LogView.svelte'
  import { openFileViaDialog, subscribeTabEvents } from './lib/backend'
  import {
    tabsStore,
    activeTab,
    applyLinesEvent,
    applyMetaEvent,
    applyClosedEvent,
  } from './lib/stores/tabs'

  $: current = $tabsStore.find((t) => t.info.id === $activeTab)

  onMount(() => {
    subscribeTabEvents({
      onLines: applyLinesEvent,
      onMeta: applyMetaEvent,
      onClosed: applyClosedEvent,
    })
  })

  async function handleOpen() {
    const id = await openFileViaDialog()
    if (id) activeTab.set(id)
  }
</script>

<div class="app">
  <header class="topbar">
    <button class="open-btn" on:click={handleOpen}>打开文件</button>
    <TabBar />
  </header>
  <main>
    {#if !current}
      <Welcome />
    {:else}
      <div class="pane">
        <div class="pane-head">
          {current.info.label} · {current.info.lines} 行
        </div>
        <LogView lines={current.lines} />
      </div>
    {/if}
  </main>
</div>

<style>
  .app {
    width: 100vw;
    height: 100vh;
    display: flex;
    flex-direction: column;
    background: #10161f;
    color: #e6e6e6;
    font-family: -apple-system, 'PingFang SC', sans-serif;
  }
  .topbar {
    display: flex;
    align-items: flex-end;
    gap: 8px;
    padding: 6px 8px 0;
    background: #0c1119;
  }
  .open-btn {
    background: #2a5db0;
    color: #fff;
    border: none;
    border-radius: 6px 6px 0 0;
    padding: 6px 14px;
    cursor: pointer;
    font-size: 12px;
  }
  .open-btn:hover {
    background: #3671cc;
  }
  main {
    flex: 1;
    overflow: hidden;
  }
  .pane {
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
  }
  .pane-head {
    padding: 4px 10px;
    font-size: 12px;
    color: #8b98a9;
    border-bottom: 1px solid #1c2836;
  }
</style>
