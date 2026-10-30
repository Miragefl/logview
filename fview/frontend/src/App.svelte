<script lang="ts">
  // fview 主框架：顶栏（打开/tab）+ 搜索栏 + 过滤栏 + 主区（日志视图 + 行详情侧板）。
  import { onMount } from 'svelte'
  import TabBar from './components/TabBar.svelte'
  import SearchBar from './components/SearchBar.svelte'
  import FilterBar from './components/FilterBar.svelte'
  import LogView from './components/LogView.svelte'
  import Welcome from './components/Welcome.svelte'
  import DetailPanel from './components/DetailPanel.svelte'
  import RulesPanel from './components/RulesPanel.svelte'
  import BrowseDialog from './components/BrowseDialog.svelte'
  import ConnectionPanel from './components/ConnectionPanel.svelte'
  import {
    openFileViaDialog,
    setFollow,
    openSSHTab,
    subscribeTabEvents,
    parseQuery,
    matchLines,
    getHides,
    saveHides,
    getFields,
    saveFields,
  themeNames,
  themeCSS,
  currentTheme,
  setTheme,
    type QueryMeta,
    type FieldConfig,
  } from './lib/backend'
  import {
    tabsStore,
    activeTab,
    applyLinesEvent,
    applyMetaEvent,
    applyClosedEvent,
    applyOpenedEvent,
    toggleFollow,
    type LineDTO,
  } from './lib/stores/tabs'
  import type { LevelFilterMode } from './lib/filter'

  $: current = $tabsStore.find((t) => t.info.id === $activeTab)

  let searchBar: SearchBar
  let queryMeta: QueryMeta | null = null
  let hits: number[] = []
  let hitPos = -1
  let levelMode: LevelFilterMode = 'A'
  let hides: string[] = []
  let fields: FieldConfig[] = []
  let detailLine: LineDTO | null = null
  let showRules = false
  let browseHost = '' // 非空即弹远程目录浏览
  let fontSize = Number(localStorage.getItem('fview.font-size')) || 12
  $: rowHeight = fontSize + 8
  $: localStorage.setItem('fview.font-size', String(fontSize))
  let themes: string[] = []
  let theme = ''

  async function applyTheme(name: string) {
    theme = name
    const css = await themeCSS(name)
    const root = document.documentElement
    for (const [k, v] of Object.entries(css)) {
      if (v) root.style.setProperty(k, v)
    }
    await setTheme(name)
  }

  const DEFAULT_FIELDS = ['time', 'level', 'message']
  $: visibleFields = fields.length
      ? fields.filter((f) => f.visible).map((f) => f.name)
      : DEFAULT_FIELDS

  onMount(() => {
    subscribeTabEvents({
      onOpened: applyOpenedEvent,
      onLines: applyLinesEvent,
      onMeta: applyMetaEvent,
      onClosed: applyClosedEvent,
    })
    getHides().then((v) => (hides = v))
    getFields().then((v) => (fields = v))
    themeNames().then((v) => (themes = v))
    currentTheme().then((v) => {
      if (v) applyTheme(v)
    })
  })

  async function handleOpen(follow: boolean) {
    // tab 注册由 tab:opened 事件驱动（applyOpenedEvent → registerTab 语义），
    // 此处只发起打开；follow 期望态在事件到达后由 toggleFollow 补
    await openFileViaDialog(follow)
  }

  async function handleFollowToggle() {
    if (!current) return
    const next = !current.info.follow
    toggleFollow(current.info.id, next)
    await setFollow(current.info.id, next)
  }

  async function runSearch(q: string) {
    if (!current || !q.trim()) {
      queryMeta = null
      hits = []
      hitPos = -1
      return
    }
    queryMeta = await parseQuery(q)
    if (queryMeta.parseError) {
      hits = []
      hitPos = -1
      return
    }
    hits = await matchLines(current.info.id, q)
    hitPos = hits.length > 0 ? 0 : -1
  }

  function jumpHit(delta: number) {
    if (hits.length === 0) return
    hitPos = (hitPos + delta + hits.length) % hits.length
  }

  async function handleSaveHides(list: string[]) {
    hides = list
    await saveHides(list)
  }

  async function handleToggleField(name: string) {
    fields = fields.map((f) => (f.name === name ? { ...f, visible: !f.visible } : f))
    await saveFields(fields)
  }

  function onKeydown(e: KeyboardEvent) {
    const tag = (e.target as HTMLElement).tagName
    if (e.key === '/' && tag !== 'INPUT' && tag !== 'TEXTAREA') {
      e.preventDefault()
      searchBar.focus()
    }
    // 字号缩放：Cmd/Ctrl + =/-/0（输入框外）
    if ((e.metaKey || e.ctrlKey) && tag !== 'INPUT' && tag !== 'TEXTAREA') {
      if (e.key === '=' || e.key === '+') {
        e.preventDefault()
        fontSize = Math.min(24, fontSize + 1)
      } else if (e.key === '-') {
        e.preventDefault()
        fontSize = Math.max(9, fontSize - 1)
      } else if (e.key === '0') {
        e.preventDefault()
        fontSize = 12
      }
    }
  }
</script>

<svelte:window on:keydown={onKeydown} />
<div class="app">
  <ConnectionPanel
    on:openLocal={(e) => handleOpen(e.detail)}
    on:browseSSH={(e) => (browseHost = e.detail)}
    on:openK8s={() => {}}
  />
  <div class="main-col">
  <header class="topbar">
    <button class="top-btn" on:click={() => (showRules = true)}>规则</button>
    <TabBar />
    <div class="spacer" />
    {#if themes.length}
      <select class="theme-select" value={theme} on:change={(e) => applyTheme(e.currentTarget.value)}>
        {#each themes as tn (tn)}
          <option value={tn}>{tn}</option>
        {/each}
      </select>
    {/if}
  </header>
  <main>
    {#if !current}
      <Welcome />
    {:else}
      <div class="pane">
        <SearchBar
          bind:this={searchBar}
          meta={queryMeta}
          hitsCount={hits.length}
          hitPos={hitPos}
          onSearch={runSearch}
          onJump={jumpHit}
        />
        <FilterBar
          {levelMode}
          {hides}
          {fields}
          onLevel={(m) => (levelMode = m)}
          onSaveHides={handleSaveHides}
          onToggleField={handleToggleField}
        />
        <div class="pane-head">
          <span>{current.info.label} · {current.info.lines} 行</span>
          <span class="zoom">
            <button class="follow-toggle" on:click={() => (fontSize = Math.max(9, fontSize - 1))} title="缩小 (Cmd+-)">A-</button>
            <span class="zoom-val">{fontSize}px</span>
            <button class="follow-toggle" on:click={() => (fontSize = Math.min(24, fontSize + 1))} title="放大 (Cmd+=)">A+</button>
          </span>
          <button class="follow-toggle" class:on={current.info.follow} on:click={handleFollowToggle}>
            {current.info.follow ? '跟随中 ↓' : '已暂停'}
          </button>
        </div>
        <div class="pane-body">
          <LogView
            lines={current.lines}
            follow={current.info.follow}
            {fontSize}
            {rowHeight}
            onFollowPause={() => current && current.info.follow && handleFollowToggle()}
            {hits}
            currentLine={hitPos >= 0 ? hits[hitPos] : -1}
            {levelMode}
            hidesList={hides}
            highlightWords={queryMeta?.highlightKeywords ?? []}
            {visibleFields}
            onLineClick={(ln) => (detailLine = ln)}
          />
          {#if detailLine}
            <DetailPanel line={detailLine} onClose={() => (detailLine = null)} />
          {/if}
        </div>
      </div>
    {/if}
  </main>
  {#if showRules}
    <RulesPanel onClose={() => (showRules = false)} />
  {/if}
  {#if browseHost}
    <BrowseDialog
      host={browseHost}
      onPick={async (path, password) => {
        const id = await openSSHTab(browseHost, path, 0, password)
        if (id) browseHost = ''
      }}
      onClose={() => (browseHost = '')}
    />
  {/if}
  </div>
</div>

<style>
  .app {
    width: 100vw;
    height: 100vh;
    display: flex;
    background: var(--bg, #10161f);
    color: var(--fg, #e6e6e6);
    font-family: -apple-system, 'PingFang SC', sans-serif;
  }
  .main-col {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    position: relative;
  }
  .topbar {
    display: flex;
    align-items: flex-end;
    gap: 8px;
    padding: 6px 8px 0;
    background: #0c1119;
  }
  .spacer {
    flex: 1;
  }
  .theme-select {
    align-self: center;
    background: #1c2836;
    color: #8b98a9;
    border: 1px solid #2a3a4f;
    border-radius: 4px;
    padding: 2px 6px;
    font-size: 11px;
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
  .open-btn.follow {
    background: #2f7d4f;
  }
  .open-btn.follow:hover {
    background: #389760;
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
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 4px 10px;
    font-size: 12px;
    color: #8b98a9;
    border-bottom: 1px solid #1c2836;
  }
  .follow-toggle {
    background: #1c2836;
    color: #8b98a9;
    border: 1px solid #2a3a4f;
    border-radius: 4px;
    padding: 2px 10px;
    cursor: pointer;
    font-size: 11px;
  }
  .follow-toggle.on {
    color: #7ee29a;
    border-color: #2f7d4f;
  }
  .top-btn {
    background: #1c2836;
    color: #8b98a9;
    border: 1px solid #2a3a4f;
    border-radius: 6px 6px 0 0;
    padding: 6px 12px;
    cursor: pointer;
    font-size: 12px;
  }
  .top-btn:hover {
    color: #e6e6e6;
  }
  .zoom {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .zoom-val {
    font-size: 11px;
    min-width: 32px;
    text-align: center;
  }
  .pane-body {
    flex: 1;
    display: flex;
    overflow: hidden;
  }
</style>
