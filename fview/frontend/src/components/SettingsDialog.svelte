<script lang="ts">
  // 设置中心：连接（SSH 主机 + FRP 服务器/连接）/ 解析规则 / 界面 三区统一入口
  //（替代 ConfigDialog + RulesPanel；数据全部共享正本，TUI 同源）。
  import { createEventDispatcher, onMount } from 'svelte'
  import {
    getSSHHosts,
    addSSHHost,
    removeSSHHost,
    getFrpServers,
    getFrpConns,
    upsertFrpServer,
    upsertFrpConn,
    deleteFrpServer,
    deleteFrpConn,
    getRules,
    saveRules,
    type FrpServer,
    type FrpConn,
  } from '../lib/backend'

  const dispatch = createEventDispatcher<{
    close: void
    changed: void
    menuZoom: number
    logFontSize: number
    uiFontFamily: string
    uiFontSize: number
    logFontFamily: string
  }>()

  export let tab: 'conn' | 'rules' | 'ui' = 'conn'
  export let menuZoom = 1.05
  export let logFontSize = 13
  export let uiFontFamily = ''
  export let uiFontSize = 14
  export let logFontFamily = '' 

  let sshHosts: string[] = []
  let servers: FrpServer[] = []
  let conns: FrpConn[] = []
  let newHost = ''
  let srvForm: FrpServer | null = null
  let connForm: FrpConn | null = null
  let error = ''
  let confirmDel = ''

  let ruleRows: Array<{ name: string; pattern: string; fallback: boolean }> = []
  let rulesDirty = false
  let rulesSaving = false
  let rulesSavedAt = ''

  async function refresh() {
    sshHosts = await getSSHHosts()
    servers = await getFrpServers()
    conns = await getFrpConns()
  }

  onMount(async () => {
    await refresh()
    const rs = await getRules()
    ruleRows = rs.map((r) => ({ name: r.name, pattern: r.pattern, fallback: r.fallback }))
  })

  async function changed() {
    await refresh()
    dispatch('changed')
  }

  async function guard(fn: () => Promise<void>) {
    error = ''
    try {
      await fn()
    } catch (e) {
      error = String(e)
    }
  }

  async function addHost() {
    const h = newHost.trim()
    if (!h) return
    await guard(async () => {
      await addSSHHost(h)
      newHost = ''
      await changed()
    })
  }

  async function saveSrv() {
    if (srvForm && srvForm.name.trim() && srvForm.addr.trim()) {
      await guard(async () => {
        await upsertFrpServer({ ...srvForm! })
        srvForm = null
        await changed()
      })
    }
  }

  async function saveConn() {
    if (connForm && connForm.name.trim() && connForm.server && connForm.proxy.trim()) {
      await guard(async () => {
        await upsertFrpConn({ ...connForm! })
        connForm = null
        await changed()
      })
    }
  }

  function addRule() {
    ruleRows = [...ruleRows, { name: 'new-rule', pattern: '(?P<message>.*)', fallback: false }]
    rulesDirty = true
  }

  async function saveRulesNow() {
    rulesSaving = true
    try {
      await saveRules(ruleRows.map((r) => ({ ...r })))
      rulesDirty = false
      rulesSavedAt = new Date().toLocaleTimeString()
    } finally {
      rulesSaving = false
    }
  }
</script>

<div class="overlay" role="presentation" on:click|self={() => dispatch('close')}>
  <div class="win" role="dialog" aria-label="设置">
    <nav class="nav">
      <div class="nav-title">设置</div>
      <button class="nav-item" class:on={tab === 'conn'} on:click={() => (tab = 'conn')}>连接</button>
      <button class="nav-item" class:on={tab === 'rules'} on:click={() => (tab = 'rules')}>解析规则</button>
      <button class="nav-item" class:on={tab === 'ui'} on:click={() => (tab = 'ui')}>界面</button>
      <div class="nav-spacer" />
      <button class="nav-close" title="关闭" on:click={() => dispatch('close')}>&times;</button>
    </nav>
    {#if error}
      <div class="errbar">{error}</div>
    {/if}
    <div class="body">
      {#if tab === 'conn'}
        <h4>SSH 主机</h4>
        <div class="addrow">
          <input placeholder="user@host" bind:value={newHost} on:keydown={(e) => e.key === 'Enter' && addHost()} />
          <button class="btn" on:click={addHost}>添加</button>
        </div>
        <table>
          <thead><tr><th>主机（user@host）</th><th /></tr></thead>
          <tbody>
            {#each sshHosts as h (h)}
              <tr>
                <td>{h}</td>
                <td class="ops">
                  {#if confirmDel === `ssh:${h}`}
                    <button
                      class="mini danger"
                      on:click={async () => {
                        confirmDel = ''
                        await guard(async () => {
                          await removeSSHHost(h)
                          await changed()
                        })
                      }}>确认删除</button>
                  {:else}
                    <button class="mini danger" on:click={() => (confirmDel = `ssh:${h}`)}>删除</button>
                  {/if}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
        <p class="hint">主机凭据（端口/密钥）沿用 ~/.ssh/config；密码在连接时输入（会话内使用，不落盘）。</p>

        <h4>frps 服务器</h4>
        <table>
          <thead><tr><th>名称</th><th>地址</th><th>Token</th><th /></tr></thead>
          <tbody>
            {#each servers as s (s.name)}
              <tr>
                <td>{s.name}</td><td>{s.addr}</td><td class="dim">{s.token ? '•••' : '—'}</td>
                <td class="ops">
                  <button class="mini" on:click={() => (srvForm = { ...s })}>编辑</button>
                  {#if confirmDel === `srv:${s.name}`}
                    <button
                      class="mini danger"
                      on:click={async () => {
                        confirmDel = ''
                        await guard(async () => {
                          await deleteFrpServer(s.name)
                          await changed()
                        })
                      }}>确认删除</button>
                  {:else}
                    <button class="mini danger" on:click={() => (confirmDel = `srv:${s.name}`)}>删除</button>
                  {/if}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
        {#if srvForm}
          <div class="form">
            <input placeholder="名称" bind:value={srvForm.name} />
            <input placeholder="地址 host:port" bind:value={srvForm.addr} />
            <input placeholder="Token" bind:value={srvForm.token} />
            <button class="btn primary" on:click={saveSrv}>保存服务器</button>
            <button class="mini" on:click={() => (srvForm = null)}>取消</button>
          </div>
        {:else}
          <button class="btn" on:click={() => (srvForm = { name: '', addr: '', token: '' })}>+ 新增服务器</button>
        {/if}

        <h4>FRP 连接</h4>
        <table>
          <thead><tr><th>名称</th><th>服务器</th><th>Proxy</th><th>用户</th><th>直达路径</th><th /></tr></thead>
          <tbody>
            {#each conns as c (c.name)}
              <tr>
                <td>{c.name}</td><td>{c.server}</td><td>{c.proxy}</td><td>{c.user}</td><td class="dim">{c.path || '—'}</td>
                <td class="ops">
                  <button class="mini" on:click={() => (connForm = { ...c })}>编辑</button>
                  {#if confirmDel === `conn:${c.name}`}
                    <button
                      class="mini danger"
                      on:click={async () => {
                        confirmDel = ''
                        await guard(async () => {
                          await deleteFrpConn(c.name)
                          await changed()
                        })
                      }}>确认删除</button>
                  {:else}
                    <button class="mini danger" on:click={() => (confirmDel = `conn:${c.name}`)}>删除</button>
                  {/if}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
        {#if connForm}
          <div class="form">
            <input placeholder="名称" bind:value={connForm.name} />
            <select bind:value={connForm.server}>
              <option value="">选服务器…</option>
              {#each servers as s (s.name)}<option value={s.name}>{s.name}</option>{/each}
            </select>
            <input placeholder="SK（可空）" bind:value={connForm.sk} />
            <input placeholder="Proxy 名" bind:value={connForm.proxy} />
            <input placeholder="用户" bind:value={connForm.user} />
            <input class="mono" placeholder="直达日志路径（可空）" bind:value={connForm.path} />
            <button class="btn primary" on:click={saveConn}>保存连接</button>
            <button class="mini" on:click={() => (connForm = null)}>取消</button>
          </div>
        {:else}
          <button class="btn" on:click={() => (connForm = { name: '', server: servers[0]?.name ?? '', sk: '', proxy: '', user: 'root', path: '' })}>+ 新增连接</button>
        {/if}
        <p class="hint">连接配置存 ~/.local/state/logview/frp.json（TUI 同源）；stcp proxy 均映射目标机 sshd。</p>

      {:else if tab === 'rules'}
        <p class="hint">命名捕获组：time / level / thread / traceId / logger / message；保存写回共享 rules.yaml（TUI 同源），新开 tab 生效。</p>
        <div class="rulegrid head-row">
          <span>名称</span><span>正则</span><span>兜底</span><span />
        </div>
        {#each ruleRows as r, i (i)}
          <div class="rulegrid">
            <input bind:value={r.name} on:input={() => (rulesDirty = true)} />
            <input class="mono" bind:value={r.pattern} on:input={() => (rulesDirty = true)} />
            <input type="checkbox" bind:checked={r.fallback} on:change={() => (rulesDirty = true)} />
            <button
              class="mini danger"
              on:click={() => {
                ruleRows = ruleRows.filter((_, idx) => idx !== i)
                rulesDirty = true
              }}>删</button>
          </div>
        {/each}
        <div class="rules-foot">
          <button class="btn" on:click={addRule}>+ 新增规则</button>
          <span class="hint">{rulesSavedAt ? `已保存 ${rulesSavedAt}` : ''}</span>
          <button class="btn primary" disabled={!rulesDirty || rulesSaving} on:click={saveRulesNow}>
            {rulesSaving ? '保存中…' : '保存规则'}
          </button>
        </div>

      {:else if tab === 'ui'}
        <div class="fontctl">
          <div class="fc-row">
            <div class="fc-label">
              <span class="fc-name">日志字体</span>
              <span class="fc-desc">日志行等宽字体族</span>
            </div>
            <div class="fc-ctrl">
              <select class="fc-select" value={logFontFamily} on:change={(e) => dispatch('logFontFamily', e.currentTarget.value)}>
                <option value="">SF Mono（默认）</option>
                <option value="Menlo, monospace">Menlo</option>
                <option value="Monaco, monospace">Monaco</option>
                <option value="'Courier New', monospace">Courier New</option>
                <option value="'Andale Mono', monospace">Andale Mono</option>
              </select>
            </div>
          </div>
          <div class="fc-row">
            <div class="fc-label">
              <span class="fc-name">界面字体</span>
              <span class="fc-desc">菜单/侧栏/弹窗的字体族（日志区恒为等宽）</span>
            </div>
            <div class="fc-ctrl">
              <select class="fc-select" value={uiFontFamily} on:change={(e) => dispatch('uiFontFamily', e.currentTarget.value)}>
                <option value="">系统默认（PingFang SC）</option>
                <option value="'SF Mono', Menlo, monospace">SF Mono（等宽）</option>
                <option value="Menlo, monospace">Menlo</option>
                <option value="'Helvetica Neue', sans-serif">Helvetica</option>
                <option value="'Songti SC', serif">宋体 Songti</option>
                <option value="'Kaiti SC', serif">楷体 Kaiti</option>
                <option value="Georgia, serif">Georgia</option>
              </select>
            </div>
          </div>
          <div class="fc-row">
            <div class="fc-label">
              <span class="fc-name">界面字号</span>
              <span class="fc-desc">菜单/侧栏/弹窗文字大小（12-20px）</span>
            </div>
            <div class="fc-ctrl">
              <button class="mini" on:click={() => dispatch('uiFontSize', Math.max(12, uiFontSize - 1))}>−</button>
              <span class="fc-val">{uiFontSize}px</span>
              <button class="mini" on:click={() => dispatch('uiFontSize', Math.min(20, uiFontSize + 1))}>＋</button>
            </div>
          </div>
          <div class="fc-row">
            <div class="fc-label">
              <span class="fc-name">界面缩放</span>
              <span class="fc-desc">整体等比缩放（独立于字号；快捷键 Cmd ± / 0）</span>
            </div>
            <div class="fc-ctrl">
              <button class="mini" on:click={() => dispatch('menuZoom', Math.max(0.8, Math.round((menuZoom - 0.05) * 100) / 100))}>−</button>
              <span class="fc-val">{Math.round(menuZoom * 100)}%</span>
              <button class="mini" on:click={() => dispatch('menuZoom', Math.min(1.6, Math.round((menuZoom + 0.05) * 100) / 100))}>＋</button>
            </div>
          </div>
          <div class="fc-row">
            <div class="fc-label">
              <span class="fc-name">日志文本字号</span>
              <span class="fc-desc">日志行文字（状态栏 A− / A＋，仅影响日志区）</span>
            </div>
            <div class="fc-ctrl">
              <button class="mini" on:click={() => dispatch('logFontSize', Math.max(10, logFontSize - 1))}>−</button>
              <span class="fc-val">{logFontSize}px</span>
              <button class="mini" on:click={() => dispatch('logFontSize', Math.min(26, logFontSize + 1))}>＋</button>
            </div>
          </div>
        </div>
        <p class="hint">主题：顶栏右侧下拉切换，写回共享 rules.yaml（TUI 下次启动同主题）。</p>
      {/if}
    </div>
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.55);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 120;
  }
  .win {
    width: 780px;
    max-height: 84vh;
    background: var(--popup-bg, var(--card));
    border: 1px solid var(--popup-border, var(--border-strong));
    border-radius: 10px;
    display: flex;
    flex-direction: column;
    color: var(--fg, var(--fg));
  }
  .body {
    flex: 1;
    padding: 18px 22px 22px;
    overflow-y: auto;
  }
  h4 {
    margin: 16px 0 8px;
    color: var(--muted-fg);
    font-size: 0.8125rem;
  }
  .addrow {
    display: flex;
    gap: 6px;
    margin-bottom: 10px;
  }
  .addrow input,
  .form input,
  .form select,
  .rulegrid input {
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: 7px;
    color: var(--fg);
    padding: 5px 8px;
    font-size: 0.8125rem;
  }
  .addrow input {
    flex: 1;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.8125rem;
  }
  th {
    text-align: left;
    color: var(--faint);
    font-weight: normal;
    padding: 5px 6px;
    border-bottom: 1px solid var(--muted);
  }
  td {
    padding: 5px 6px;
    border-bottom: 1px solid var(--border);
  }
  td.ops {
    text-align: right;
    white-space: nowrap;
  }
  td.dim {
    color: var(--faint);
  }
  .form {
    display: flex;
    gap: 6px;
    align-items: center;
    margin: 10px 0;
    padding: 10px;
    background: var(--card);
    border-radius: 6px;
    flex-wrap: wrap;
  }
  .btn {
    background: var(--muted);
    color: var(--fg);
    border: 1px solid var(--border-strong);
    border-radius: 7px;
    padding: 5px 14px;
    cursor: pointer;
    font-size: 0.8125rem;
  }
  .btn:hover {
    background: var(--hover);
  }
  .btn.primary {
    background: var(--primary);
    color: var(--card);
    border: none;
  }
  .btn.primary:disabled {
    opacity: 0.5;
    cursor: default;
  }
  .mini {
    background: var(--muted);
    color: var(--muted-fg);
    border: 1px solid var(--border-strong);
    border-radius: 7px;
    cursor: pointer;
    padding: 2px 9px;
    font-size: 0.75rem;
  }
  .mini.danger:hover {
    color: var(--error);
    border-color: color-mix(in srgb, var(--error) 40%, transparent);
  }
  .hint {
    color: var(--faint);
    font-size: 0.75rem;
    margin: 8px 0 0;
  }
  .errbar {
    padding: 7px 18px;
    color: var(--error);
    font-size: 0.8125rem;
    border-bottom: 1px solid color-mix(in srgb, var(--error) 40%, transparent);
  }
  .rulegrid {
    display: grid;
    grid-template-columns: 150px 1fr 44px 44px;
    gap: 8px;
    align-items: center;
    padding: 4px 0;
    font-size: 0.8125rem;
  }
  .rulegrid.head-row {
    color: var(--faint);
    font-size: 0.75rem;
  }
  .rules-foot {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-top: 10px;
  }
  .rules-foot .hint {
    flex: 1;
    text-align: right;
  }
  input.mono {
    font-family: 'SF Mono', Menlo, monospace;
  }
</style>
