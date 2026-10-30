<script lang="ts">
  // 连接配置独立窗口：SSH 主机 / FRP 服务器与连接 的表格化管理（共享正本即时落盘）。
  import { createEventDispatcher, onMount } from 'svelte'
  import {
    getSSHHosts, addSSHHost, removeSSHHost,
    getFrpServers, getFrpConns, upsertFrpServer, upsertFrpConn, deleteFrpServer, deleteFrpConn,
    type FrpServer, type FrpConn,
  } from '../lib/backend'

  const dispatch = createEventDispatcher<{ close: void; changed: void }>()

  let tab: 'ssh' | 'frp' = 'ssh'
  let sshHosts: string[] = []
  let servers: FrpServer[] = []
  let conns: FrpConn[] = []
  let newHost = ''

  let srvForm: FrpServer | null = null
  let connForm: FrpConn | null = null
  let error = ''
  let confirmDel = '' // 待二次确认的删除目标（kind:name，Final Review M4）

  async function refresh() {
    sshHosts = await getSSHHosts()
    servers = await getFrpServers()
    conns = await getFrpConns()
  }

  onMount(refresh)

  async function changed() {
    await refresh()
    dispatch('changed')
  }

  async function guard(fn: () => Promise<void>) {
    error = ''
    try {
      await fn()
    } catch (e) {
      error = String(e) // 业务错误上屏（如"服务器仍被连接引用"，Final Review I2）
    }
  }

  async function addHost() {
    const h = newHost.trim()
    if (!h) return
    await addSSHHost(h)
    newHost = ''
    await changed()
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

  function editConn(c: FrpConn) {
    connForm = { ...c }
  }
</script>

<div class="overlay" role="presentation" on:click|self={() => dispatch('close')}>
  <div class="win" role="dialog" aria-label="连接配置">
    <div class="head">
      <div class="tabs">
        <button class="tab" class:on={tab === 'ssh'} on:click={() => (tab = 'ssh')}>SSH 主机</button>
        <button class="tab" class:on={tab === 'frp'} on:click={() => (tab = 'frp')}>FRP</button>
      </div>
      <button class="mini" on:click={() => dispatch('close')}>&times;</button>
    </div>

    {#if error}
      <div class="errbar">{error}</div>
    {/if}
    <div class="body">
      {#if tab === 'ssh'}
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
                    <button class="mini danger" on:click={async () => { confirmDel = ''; await guard(async () => { await removeSSHHost(h); await changed() }) }}>确认删除</button>
                  {:else}
                    <button class="mini danger" on:click={() => (confirmDel = `ssh:${h}`)}>删除</button>
                  {/if}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
        <p class="hint">主机凭据（端口/密钥）沿用 ~/.ssh/config；密码在连接时输入（会话内使用，不落盘）。</p>
      {:else}
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
                    <button class="mini danger" on:click={async () => { confirmDel = ''; await guard(async () => { await deleteFrpServer(s.name); await changed() }) }}>确认删除</button>
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
            <button class="btn" on:click={saveSrv}>保存服务器</button>
            <button class="mini" on:click={() => (srvForm = null)}>取消</button>
          </div>
        {:else}
          <button class="btn" on:click={() => (srvForm = { name: '', addr: '', token: '' })}>+ 新增服务器</button>
        {/if}

        <h4>FRP 连接</h4>
        <table>
          <thead><tr><th>名称</th><th>服务器</th><th>Proxy</th><th>用户</th><th /></tr></thead>
          <tbody>
            {#each conns as c (c.name)}
              <tr>
                <td>{c.name}</td><td>{c.server}</td><td>{c.proxy}</td><td>{c.user}</td>
                <td class="ops">
                  <button class="mini" on:click={() => editConn(c)}>编辑</button>
                  {#if confirmDel === `conn:${c.name}`}
                    <button class="mini danger" on:click={async () => { confirmDel = ''; await guard(async () => { await deleteFrpConn(c.name); await changed() }) }}>确认删除</button>
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
            <input class="mono" placeholder="直达日志路径（可空，浏览时选也行）" bind:value={connForm.path} />
            <button class="btn" on:click={saveConn}>保存连接</button>
            <button class="mini" on:click={() => (connForm = null)}>取消</button>
          </div>
        {:else}
          <button class="btn" on:click={() => (connForm = { name: '', server: servers[0]?.name ?? '', sk: '', proxy: '', user: 'root', path: '' })}>+ 新增连接</button>
        {/if}
        <p class="hint">连接配置存 ~/.local/state/logview/frp.json（TUI 同源）；stcp proxy 均映射目标机 sshd。</p>
      {/if}
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
    z-index: 120;
  }
  .win {
    width: 720px;
    max-height: 82vh;
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
    padding: 8px 14px 0;
  }
  .tabs { display: flex; gap: 4px; }
  .tab {
    background: none;
    border: none;
    border-bottom: 2px solid transparent;
    color: #6b7f95;
    cursor: pointer;
    font-size: 13px;
    padding: 6px 12px;
  }
  .tab.on {
    color: #e6e6e6;
    border-bottom-color: #2a5db0;
  }
  .body {
    padding: 10px 16px 16px;
    overflow-y: auto;
  }
  h4 {
    margin: 14px 0 6px;
    color: #6b7f95;
    font-size: 12px;
  }
  .addrow { display: flex; gap: 6px; margin-bottom: 8px; }
  .addrow input, .form input, .form select {
    flex: 1;
    background: #10161f;
    border: 1px solid #24354a;
    border-radius: 4px;
    color: #e6e6e6;
    padding: 4px 8px;
    font-size: 12px;
  }
  .addrow input { flex: 1; }
  table { width: 100%; border-collapse: collapse; font-size: 12px; }
  th {
    text-align: left;
    color: #46586e;
    font-weight: normal;
    padding: 4px 6px;
    border-bottom: 1px solid #1c2836;
  }
  td { padding: 4px 6px; border-bottom: 1px solid #141d29; }
  td.ops { text-align: right; white-space: nowrap; }
  td.dim { color: #46586e; }
  .form {
    display: flex;
    gap: 6px;
    align-items: center;
    margin: 8px 0;
    padding: 8px;
    background: #10161f;
    border-radius: 6px;
    flex-wrap: wrap;
  }
  .btn {
    background: #2a5db0;
    color: #fff;
    border: none;
    border-radius: 4px;
    padding: 4px 14px;
    cursor: pointer;
    font-size: 12px;
  }
  .btn:hover { background: #3671cc; }
  .mini {
    background: #1c2836;
    color: #8b98a9;
    border: 1px solid #2a3a4f;
    border-radius: 4px;
    cursor: pointer;
    padding: 2px 8px;
    font-size: 11px;
  }
  .mini.danger:hover { color: #ff6b6b; border-color: #8a3038; }
  .hint { color: #46586e; font-size: 11px; margin: 8px 0 0; }
  .errbar {
    padding: 6px 16px;
    color: #ff6b6b;
    font-size: 12px;
    border-bottom: 1px solid #8a3038;
  }
  input.mono { font-family: 'SF Mono', Menlo, monospace; }
</style>
