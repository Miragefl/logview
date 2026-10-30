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

  const dispatch = createEventDispatcher<{ close: void; changed: void }>()

  export let tab: 'conn' | 'rules' | 'ui' = 'conn'

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
    <div class="head">
      <div class="tabs">
        <button class="tab" class:on={tab === 'conn'} on:click={() => (tab = 'conn')}>连接</button>
        <button class="tab" class:on={tab === 'rules'} on:click={() => (tab = 'rules')}>解析规则</button>
        <button class="tab" class:on={tab === 'ui'} on:click={() => (tab = 'ui')}>界面</button>
      </div>
      <button class="mini" on:click={() => dispatch('close')}>&times;</button>
    </div>
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
        <div class="ui-note">
          <p>全局缩放：Cmd/Ctrl + = / - / 0（整个界面等比缩放并记忆），也可用底部状态栏的「界面 −/＋」。</p>
          <p>日志行字号：底部状态栏 A- / A+（仅日志行）。</p>
          <p>主题：顶栏右侧下拉切换，写回共享 rules.yaml（TUI 下次启动同主题）。</p>
        </div>
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
    background: var(--popup-bg, #ffffff);
    border: 1px solid var(--popup-border, #c7c7bc);
    border-radius: 10px;
    display: flex;
    flex-direction: column;
    color: var(--fg, #1a1d21);
  }
  .head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 14px 0;
  }
  .tabs {
    display: flex;
    gap: 4px;
  }
  .tab {
    background: none;
    border: none;
    border-bottom: 2px solid transparent;
    color: #6b7280;
    cursor: pointer;
    font-size: 14px;
    padding: 8px 14px;
  }
  .tab.on {
    color: #1a1d21;
    border-bottom-color: #3b5b8c;
  }
  .body {
    padding: 12px 18px 18px;
    overflow-y: auto;
  }
  h4 {
    margin: 16px 0 8px;
    color: #6b7280;
    font-size: 13px;
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
    background: #ffffff;
    border: 1px solid #d5d5ca;
    border-radius: 4px;
    color: #1a1d21;
    padding: 5px 8px;
    font-size: 13px;
  }
  .addrow input {
    flex: 1;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }
  th {
    text-align: left;
    color: #9aa0a8;
    font-weight: normal;
    padding: 5px 6px;
    border-bottom: 1px solid #e2e2d8;
  }
  td {
    padding: 5px 6px;
    border-bottom: 1px solid #ecece4;
  }
  td.ops {
    text-align: right;
    white-space: nowrap;
  }
  td.dim {
    color: #9aa0a8;
  }
  .form {
    display: flex;
    gap: 6px;
    align-items: center;
    margin: 10px 0;
    padding: 10px;
    background: #ffffff;
    border-radius: 6px;
    flex-wrap: wrap;
  }
  .btn {
    background: #e2e2d8;
    color: #3a3f47;
    border: 1px solid #c7c7bc;
    border-radius: 4px;
    padding: 5px 14px;
    cursor: pointer;
    font-size: 13px;
  }
  .btn:hover {
    background: #d8d8cd;
  }
  .btn.primary {
    background: #3b5b8c;
    color: #fff;
    border: none;
  }
  .btn.primary:disabled {
    opacity: 0.5;
    cursor: default;
  }
  .mini {
    background: #e2e2d8;
    color: #6b7280;
    border: 1px solid #c7c7bc;
    border-radius: 4px;
    cursor: pointer;
    padding: 2px 9px;
    font-size: 12px;
  }
  .mini.danger:hover {
    color: #dc2626;
    border-color: #eac4c4;
  }
  .hint {
    color: #9aa0a8;
    font-size: 12px;
    margin: 8px 0 0;
  }
  .errbar {
    padding: 7px 18px;
    color: #dc2626;
    font-size: 13px;
    border-bottom: 1px solid #eac4c4;
  }
  .rulegrid {
    display: grid;
    grid-template-columns: 150px 1fr 44px 44px;
    gap: 8px;
    align-items: center;
    padding: 4px 0;
    font-size: 13px;
  }
  .rulegrid.head-row {
    color: #9aa0a8;
    font-size: 12px;
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
  .ui-note {
    color: #5f6672;
    font-size: 13px;
    line-height: 2;
  }
  input.mono {
    font-family: 'SF Mono', Menlo, monospace;
  }
</style>
