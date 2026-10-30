<script lang="ts">
  // fview 主框架（重设计版）：左侧源列表 → 顶栏（tab+设置）→ 工具栏一行（搜索+过滤）
  // → 日志区 → 底部状态栏。设置中心统一三区（连接/规则/界面）。
  import { onMount } from 'svelte'
  import TabBar from './components/TabBar.svelte'
  import SearchBar from './components/SearchBar.svelte'
  import FilterBar from './components/FilterBar.svelte'
  import LogView from './components/LogView.svelte'
  import Welcome from './components/Welcome.svelte'
  import DetailPanel from './components/DetailPanel.svelte'
  import BrowseDialog from './components/BrowseDialog.svelte'
  import ConnectionPanel from './components/ConnectionPanel.svelte'
  import K8sDialog from './components/K8sDialog.svelte'
  import SettingsDialog from './components/SettingsDialog.svelte'
  import SeverityPanel from './components/SeverityPanel.svelte'
  import {
    openFileViaDialog,
    setFollow,
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
    openSSHTab,
    openFRPTab,
    releaseFRPBrowse,
    type QueryMeta,
    type FieldConfig,
    type FrpConn,
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
  let logView: LogView
  let queryMeta: QueryMeta | null = null
  let hits: number[] = []
  let hitPos = -1
  let levelMode: LevelFilterMode = 'A'
  let hides: string[] = []
  let fields: FieldConfig[] = []
  let detailLine: LineDTO | null = null
  let toolbarOpen = true // 工具栏展开态（f 或按钮收起）
  let sidebarOpen = true // 侧栏展开态（顶栏 ☰ 切换）
  let showK8s = false
  let showSettings = false
  let settingsTab: 'conn' | 'rules' | 'ui' = 'conn'
  let connPanel: ConnectionPanel
  let browseHost = ''
  let browseFRPConn: FrpConn | null = null

  // 字号与缩放
  let fontSize = Number(localStorage.getItem('fview.font-size')) || 13
  $: rowHeight = fontSize + 8
  $: localStorage.setItem('fview.font-size', String(fontSize))
  // 菜单字体与字号（真 font 设置；缩放 menuZoom 为独立功能）
  let uiFontFamily = localStorage.getItem('fview.ui-font-family') || ''
  let uiFontSize = Number(localStorage.getItem('fview.ui-font-size')) || 14
  let logFontFamily = localStorage.getItem('fview.log-font-family') || ''
  $: localStorage.setItem('fview.log-font-family', logFontFamily)
  $: {
    document.body.style.fontFamily = uiFontFamily || "-apple-system, 'PingFang SC', 'Helvetica Neue', sans-serif"
    document.documentElement.style.fontSize = uiFontSize + 'px'
    localStorage.setItem('fview.ui-font-family', uiFontFamily)
    localStorage.setItem('fview.ui-font-size', String(uiFontSize))
  }
  let menuZoom = Number(localStorage.getItem('fview.menu-zoom')) || 1.05
  $: {
    ;(document.body.style as CSSStyleDeclaration & { zoom: string }).zoom = String(menuZoom)
    localStorage.setItem('fview.menu-zoom', String(menuZoom))
  }

  let themes: string[] = []
  let theme = ''

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

  async function applyTheme(name: string) {
    theme = name
    const css = await themeCSS(name)
    const root = document.documentElement
    for (const [k, v] of Object.entries(css)) {
      if (v) root.style.setProperty(k, v)
    }
    await setTheme(name)
  }

  async function handleOpen(follow: boolean) {
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
    const inInput = tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT'
    if (e.key === '/' && !inInput) {
      e.preventDefault()
      searchBar.focus()
    }
    if ((e.metaKey || e.ctrlKey) && !inInput) {
      if (e.key === '=' || e.key === '+') {
        e.preventDefault()
        menuZoom = Math.min(1.6, Math.round((menuZoom + 0.05) * 100) / 100)
      } else if (e.key === '-') {
        e.preventDefault()
        menuZoom = Math.max(0.8, Math.round((menuZoom - 0.05) * 100) / 100)
      } else if (e.key === '0') {
        e.preventDefault()
        menuZoom = 1.05
      }
      return
    }
    // vim 风格导航（对齐 TUI 习惯；输入框内不触发）
    if (!inInput) {
      switch (e.key) {
        case 'n':
          jumpHit(1)
          break
        case 'N':
          jumpHit(-1)
          break
        case 'j':
          logView?.scrollLines(3)
          break
        case 'k':
          logView?.scrollLines(-3)
          break
        case 'g':
          logView?.scrollToRatio(0)
          break
        case 'G':
          logView?.scrollToRatio(1)
          break
        case 'e':
        case 'E':
          levelMode = levelMode === 'E' ? 'A' : 'E'
          break
        case 'w':
        case 'W':
          levelMode = levelMode === 'W' ? 'A' : 'W'
          break
        case 'i':
        case 'I':
          levelMode = levelMode === 'I' ? 'A' : 'I'
          break
        case 'a':
        case 'A':
          levelMode = 'A'
          break
        case 'd':
        case 'D':
          if (hitPos >= 0 && current) {
            const idx = hits[hitPos]
            detailLine = current.lines[idx] ?? null
          }
          break
        case 'f':
        case 'F':
          toolbarOpen = !toolbarOpen
          break
      }
    }
  }
</script>

<svelte:window on:keydown={onKeydown} />
<div class="app">
  {#if sidebarOpen}
  <aside class="sidebar">
  <ConnectionPanel
    bind:this={connPanel}
    on:openLocal={(e) => handleOpen(e.detail)}
    on:browseSSH={(e) => (browseHost = e.detail)}
    on:openFRP={(e) => (browseFRPConn = e.detail)}
    on:openK8s={() => (showK8s = true)}
    on:openConfig={(e) => {
      settingsTab = e.detail ?? 'conn'
      showSettings = true
    }}
  />
  {#if current}
    <div class="sev-wrap">
      <SeverityPanel lines={current.lines} {levelMode} onLevel={(m) => (levelMode = m)} />
    </div>
  {/if}
  </aside>
  {/if}

  <div class="main-col">
    <header class="topbar">
      <button class="hamburger" title="收起/展开侧栏" on:click={() => (sidebarOpen = !sidebarOpen)}>☰</button>
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
          <div class="toolbar">
            <SearchBar
              bind:this={searchBar}
              meta={queryMeta}
              hitsCount={hits.length}
              hitPos={hitPos}
              onSearch={runSearch}
              onJump={jumpHit}
            />
            <button class="fold-btn" title="收起/展开过滤行（f）" on:click={() => (toolbarOpen = !toolbarOpen)}>
              {toolbarOpen ? '▾' : '▸'}
            </button>
          </div>
          {#if toolbarOpen}
            <div class="toolbar sub">
              <FilterBar
                {hides}
                {fields}
                onSaveHides={handleSaveHides}
                onToggleField={handleToggleField}
              />
            </div>
          {/if}
          <div class="pane-body">
            <LogView
              bind:this={logView}
              lines={current.lines}
              uiZoom={menuZoom}
              fontFamily={logFontFamily}
              follow={current.info.follow}
              onFollowPause={() => current && current.info.follow && handleFollowToggle()}
              {hits}
              currentLine={hitPos >= 0 ? hits[hitPos] : -1}
              {levelMode}
              hidesList={hides}
              highlightWords={queryMeta?.highlightKeywords ?? []}
              {visibleFields}
              {fontSize}
              {rowHeight}
              onLineClick={(ln) => (detailLine = ln)}
            />
            {#if detailLine}
              <DetailPanel line={detailLine} onClose={() => (detailLine = null)} />
            {/if}
          </div>
          <footer class="statusbar">
            <span class="st-file">{current.info.label}</span>
            <span>{current.info.lines.toLocaleString()} 行</span>
            <button class="st-follow" class:on={current.info.follow} on:click={handleFollowToggle}>
              {current.info.follow ? '跟随中 ↓' : '已暂停'}
            </button>
            <div class="spacer" />
            {#if hits.length}
              <span>命中 {hitPos >= 0 ? hitPos + 1 : 0}/{hits.length}</span>
            {/if}
            <span class="zoom">
              <button class="st-btn" title="缩小 (Cmd+-)" on:click={() => (fontSize = Math.max(10, fontSize - 1))}>A-</button>
              <span class="zoom-val">{fontSize}px</span>
              <button class="st-btn" title="放大 (Cmd+=)" on:click={() => (fontSize = Math.min(26, fontSize + 1))}>A+</button>
            </span>
            <span class="zoom">
              <button class="st-btn" title="缩小菜单界面 (Cmd+-)" on:click={() => (menuZoom = Math.max(0.8, Math.round((menuZoom - 0.05) * 100) / 100))}>界面 -</button>
              <span class="zoom-val">{Math.round(menuZoom * 100)}%</span>
              <button class="st-btn" title="放大菜单界面 (Cmd+=)" on:click={() => (menuZoom = Math.min(1.6, Math.round((menuZoom + 0.05) * 100) / 100))}>界面 +</button>
            </span>
          </footer>
        </div>
      {/if}
    </main>
  </div>

  {#if showK8s}
    <K8sDialog on:close={() => (showK8s = false)} />
  {/if}
  {#if showSettings}
    <SettingsDialog
      tab={settingsTab}
      menuZoom={menuZoom}
      logFontSize={fontSize}
      uiFontFamily={uiFontFamily}
      uiFontSize={uiFontSize}
      logFontFamily={logFontFamily}
      on:menuZoom={(e) => (menuZoom = e.detail)}
      on:logFontSize={(e) => (fontSize = e.detail)}
      on:uiFontFamily={(e) => (uiFontFamily = e.detail)}
      on:uiFontSize={(e) => (uiFontSize = e.detail)}
      on:logFontFamily={(e) => (logFontFamily = e.detail)}
      on:close={() => (showSettings = false)}
      on:changed={() => connPanel?.refresh()}
    />
  {/if}
  {#if browseHost}
    <BrowseDialog
      host={browseHost}
      onPick={async (path, password) => {
        try {
          const id = await openSSHTab(browseHost, path, 0, password)
          if (id) browseHost = ''
        } catch (e) {
          window.alert(`SSH 开 tab 失败：${e}`)
        }
      }}
      onClose={() => (browseHost = '')}
    />
  {/if}
  {#if browseFRPConn}
    <BrowseDialog
      host=""
      frpConn={browseFRPConn}
      onPick={async (path, password) => {
        const name = browseFRPConn?.name ?? ''
        try {
          const id = await openFRPTab(name, path, 0, password)
          if (id) browseFRPConn = null
        } catch (e) {
          window.alert(`FRP 开 tab 失败：${e}`)
        } finally {
          await releaseFRPBrowse(name).catch(() => {})
        }
      }}
      onClose={async () => {
        if (browseFRPConn) await releaseFRPBrowse(browseFRPConn.name)
        browseFRPConn = null
      }}
    />
  {/if}
</div>

<style>
  .app {
    width: 100vw;
    height: 100vh;
    display: flex;
    background: var(--bg, var(--bg));
    color: var(--fg, var(--fg));
    font-family: -apple-system, 'PingFang SC', 'Helvetica Neue', sans-serif;
  }
  .sidebar {
    width: 232px;
    flex-shrink: 0;
    background: var(--bg);
    border-right: 1px solid var(--muted);
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 10px 10px;
  }
  .sev-wrap {
    border-top: 1px solid var(--muted);
    padding-top: 10px;
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
    align-items: center;
    gap: 8px;
    padding: 4px 10px 0;
    background: var(--bg);
    border-bottom: 1px solid var(--muted);
    min-height: 34px;
  }
  .spacer { flex: 1; }
  .hamburger {
    background: none;
    border: 1px solid var(--border-strong);
    border-radius: 6px;
    color: var(--muted-fg);
    cursor: pointer;
    font-size: 1rem;
    padding: 2px 8px;
    line-height: 1.3;
  }
  .hamburger:hover {
    background: var(--muted);
  }
  .theme-select {
    background: var(--muted);
    color: var(--muted-fg);
    border: 1px solid var(--border-strong);
    border-radius: 4px;
    padding: 3px 6px;
    font-size: 0.875rem;
  }
  main { flex: 1; overflow: hidden; }
  .pane {
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
  }
  .toolbar {
    display: flex;
    align-items: stretch;
    gap: 8px;
    padding: 6px 10px 2px;
    background: var(--bg);
  }
  .toolbar.sub {
    padding: 2px 10px 6px;
    border-bottom: 1px solid var(--muted);
  }
  .fold-btn {
    background: var(--muted);
    color: var(--muted-fg);
    border: 1px solid var(--border-strong);
    border-radius: 4px;
    cursor: pointer;
    padding: 0 10px;
    font-size: 0.8125rem;
  }
  .pane-body { flex: 1; display: flex; overflow: hidden; }
  .statusbar {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 3px 12px;
    background: var(--bg);
    border-top: 1px solid var(--muted);
    font-size: 0.875rem;
    color: var(--muted-fg);
    min-height: 26px;
  }
  .st-file {
    font-family: 'SF Mono', Menlo, monospace;
    color: var(--fg);
    max-width: 320px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .st-follow {
    background: var(--muted);
    color: var(--muted-fg);
    border: 1px solid var(--border-strong);
    border-radius: 4px;
    padding: 1px 10px;
    cursor: pointer;
    font-size: 0.875rem;
  }
  .st-follow.on { color: var(--ok); border-color: var(--ok); }
  .zoom { display: flex; align-items: center; gap: 4px; }
  .zoom-val { min-width: 40px; text-align: center; }
  .st-btn {
    background: var(--muted);
    color: var(--muted-fg);
    border: 1px solid var(--border-strong);
    border-radius: 4px;
    cursor: pointer;
    padding: 1px 8px;
    font-size: 0.875rem;
  }
  .st-btn:hover { color: var(--fg); }
</style>
