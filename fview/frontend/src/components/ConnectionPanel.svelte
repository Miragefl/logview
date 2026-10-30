<script lang="ts">
  // 连接管家侧边栏：本地文件 / SSH 主机 / FRP 连接 三区 + k8s 入口（T19 接线）。
  // 数据全部共享正本（frp.json + rules.yaml ssh_hosts），TUI 同源可见。
  import { createEventDispatcher, onMount } from 'svelte'
  import {
    getFrpServers,
    getFrpConns,
    upsertFrpServer,
    upsertFrpConn,
    deleteFrpServer,
    deleteFrpConn,
    getSSHHosts,
    addSSHHost,
    removeSSHHost,
    type FrpServer,
    type FrpConn,
  } from '../lib/backend'

  const dispatch = createEventDispatcher<{
    openLocal: boolean // true=跟踪模式
    browseSSH: string // 主机名 → 弹远程浏览
    openFRP: FrpConn // 一键连接（T17 接线）
    openK8s: void
  }>()

  let sshHosts: string[] = []
  let servers: FrpServer[] = []
  let conns: FrpConn[] = []
  let newHost = ''

  // FRP 表单态
  let showSrvForm = false
  let showConnForm = false
  let srvDraft: FrpServer = { name: '', addr: '', token: '' }
  let connDraft: FrpConn = { name: '', server: '', sk: '', proxy: '', user: 'root', path: '' }

  async function refresh() {
    sshHosts = await getSSHHosts()
    servers = await getFrpServers()
    conns = await getFrpConns()
  }

  onMount(refresh)

  async function addHost() {
    const h = newHost.trim()
    if (!h) return
    await addSSHHost(h)
    newHost = ''
    await refresh()
  }

  async function saveServer() {
    if (!srvDraft.name.trim() || !srvDraft.addr.trim()) return
    await upsertFrpServer({ ...srvDraft })
    showSrvForm = false
    srvDraft = { name: '', addr: '', token: '' }
    await refresh()
  }

  async function saveConn() {
    if (!connDraft.name.trim() || !connDraft.server.trim() || !connDraft.proxy.trim()) return
    await upsertFrpConn({ ...connDraft })
    showConnForm = false
    connDraft = { name: '', server: '', sk: '', proxy: '', user: 'root', path: '' }
    await refresh()
  }
</script>

<aside class="panel">
  <section>
    <h3>本地文件</h3>
    <button class="act" on:click={() => dispatch('openLocal', false)}>打开文件</button>
    <button class="act follow" on:click={() => dispatch('openLocal', true)}>打开并跟踪</button>
  </section>

  <section>
    <h3>SSH 主机</h3>
    <div class="addrow">
      <input placeholder="user@host" bind:value={newHost} on:keydown={(e) => e.key === 'Enter' && addHost()} />
      <button class="mini" on:click={addHost}>+</button>
    </div>
    {#each sshHosts as h (h)}
      <div class="item">
        <button class="link" title="浏览远程目录" on:click={() => dispatch('browseSSH', h)}>{h}</button>
        <button class="mini danger" title="移除" on:click={async () => { await removeSSHHost(h); await refresh() }}>×</button>
      </div>
    {/each}
  </section>

  <section>
    <h3>
      FRP 连接
      <button class="mini" on:click={() => (showConnForm = !showConnForm)}>+</button>
    </h3>
    {#each conns as c (c.name)}
      <div class="item">
        <button class="link" title={`${c.server} · ${c.proxy}`} on:click={() => dispatch('openFRP', c)}>
          {c.name}
        </button>
        <button class="mini danger" title="删除" on:click={async () => { await deleteFrpConn(c.name); await refresh() }}>×</button>
      </div>
    {/each}
    {#if showConnForm}
      <div class="form">
        <input placeholder="名称" bind:value={connDraft.name} />
        <select bind:value={connDraft.server}>
          <option value="">选服务器…</option>
          {#each servers as s (s.name)}
            <option value={s.name}>{s.name}</option>
          {/each}
        </select>
        <input placeholder="SK（留空=无加密）" bind:value={connDraft.sk} />
        <input placeholder="Proxy 名（stcp）" bind:value={connDraft.proxy} />
        <input placeholder="用户（默认 root）" bind:value={connDraft.user} />
        <button class="act" on:click={saveConn}>保存连接</button>
        {#if servers.length === 0}
          <span class="hint">先配置下方服务器</span>
        {/if}
      </div>
    {/if}
    <div class="srvrow">
      <span class="lbl">frps 服务器（{servers.length}）</span>
      <button class="mini" on:click={() => (showSrvForm = !showSrvForm)}>+</button>
    </div>
    {#each servers as s (s.name)}
      <div class="item sub">
        <span class="lbl" title={s.addr}>{s.name} · {s.addr}</span>
        <button class="mini danger" on:click={async () => { await deleteFrpServer(s.name); await refresh() }}>×</button>
      </div>
    {/each}
    {#if showSrvForm}
      <div class="form">
        <input placeholder="名称" bind:value={srvDraft.name} />
        <input placeholder="地址 host:port" bind:value={srvDraft.addr} />
        <input placeholder="Token" bind:value={srvDraft.token} />
        <button class="act" on:click={saveServer}>保存服务器</button>
      </div>
    {/if}
  </section>

  <section>
    <h3>Kubernetes</h3>
    <button class="act" on:click={() => dispatch('openK8s')}>选择集群资源…</button>
  </section>
</aside>

<style>
  .panel {
    width: 230px;
    flex-shrink: 0;
    background: #0c1119;
    border-right: 1px solid #1c2836;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 10px 8px;
    font-size: 12px;
  }
  h3 {
    margin: 0 0 6px;
    font-size: 11px;
    color: #46586e;
    display: flex;
    justify-content: space-between;
    align-items: center;
    text-transform: uppercase;
    letter-spacing: 1px;
  }
  .act {
    display: block;
    width: 100%;
    background: #1c2836;
    color: #c5cdd8;
    border: 1px solid #2a3a4f;
    border-radius: 5px;
    padding: 5px 8px;
    cursor: pointer;
    font-size: 12px;
    margin-bottom: 4px;
    text-align: left;
  }
  .act:hover {
    background: #243348;
  }
  .act.follow {
    color: #7ee29a;
  }
  .addrow,
  .srvrow {
    display: flex;
    gap: 4px;
    align-items: center;
  }
  .addrow input,
  .form input,
  .form select {
    flex: 1;
    min-width: 0;
    background: #10161f;
    border: 1px solid #24354a;
    border-radius: 4px;
    color: #e6e6e6;
    padding: 3px 6px;
    font-size: 11px;
  }
  .item {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .item.sub {
    padding-left: 6px;
  }
  .link {
    flex: 1;
    min-width: 0;
    background: none;
    border: none;
    color: #7cc0ff;
    cursor: pointer;
    text-align: left;
    font-size: 12px;
    padding: 2px 4px;
    border-radius: 4px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .link:hover {
    background: #1a2534;
  }
  .mini {
    background: #1c2836;
    color: #8b98a9;
    border: 1px solid #2a3a4f;
    border-radius: 4px;
    cursor: pointer;
    padding: 1px 7px;
    font-size: 11px;
  }
  .mini.danger:hover {
    color: #ff6b6b;
    border-color: #8a3038;
  }
  .form {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 6px;
    background: #10161f;
    border-radius: 6px;
    margin: 4px 0;
  }
  .lbl {
    color: #6b7f95;
    font-size: 11px;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .hint {
    color: #8a6d3b;
    font-size: 11px;
  }
</style>
