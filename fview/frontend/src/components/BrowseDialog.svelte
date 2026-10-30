<script lang="ts">
  // 远程目录浏览弹层：路径栏 + 目录/文件列表 + 双击进入 + 选文件回调。
  // 认证失败（错误信息含 auth/password 字样）弹密码输入行重试。
  import { sshBrowse, browseFRP, type FrpConn } from '../lib/backend'

  export let host: string
  export let frpConn: FrpConn | null = null // 非空即 FRP 模式（隧道+SSH 浏览）
  export let initialPath = '/'
  export let onPick: (path: string, password: string) => void = () => {}
  export let onClose: () => void = () => {}

  let path = initialPath
  let entries: Array<{ name: string; isDir: boolean }> = []
  let loading = false
  let error = ''
  let password = ''
  let needPassword = false
  let pwShow = false // 手动展开密码栏（错误正则未命中时也有入口，Final Review M2）
  let inflight = false // 连按 Enter 的并发守卫

  async function load() {
    if (inflight) return
    inflight = true
    loading = true
    error = ''
    try {
      entries = frpConn
        ? await browseFRP(frpConn.name, path, password)
        : await sshBrowse(host, path, password)
      needPassword = false
    } catch (e) {
      error = String(e)
      if (/auth|password|denied|permission/i.test(error)) {
        needPassword = true
      }
    } finally {
      loading = false
      inflight = false
    }
  }

  function enter(name: string, isDir: boolean) {
    if (isDir) {
      path = path === '/' ? `/${name}` : `${path}/${name}`
      load()
    } else {
      // 根目录特判防 //name（Final Review M3）
      const fp = path === '/' ? `/${name}` : `${path}/${name}`
      onPick(fp, password)
    }
  }

  function goUp() {
    const idx = path.lastIndexOf('/')
    path = idx <= 0 ? '/' : path.slice(0, idx)
    load()
  }

  load()
</script>

<div class="overlay" role="presentation" on:click|self={onClose}>
  <div class="dialog" role="dialog" aria-label="远程目录浏览">
    <div class="head">
      <span>{frpConn ? `frp://${frpConn.name}` : host}:{path}</span>
      <button class="mini" on:click={onClose}>&times;</button>
    </div>
    <div class="bar">
      <button class="mini" on:click={goUp}>↑ 上级</button>
      <input bind:value={path} on:keydown={(e) => e.key === 'Enter' && load()} />
      <button class="mini" on:click={load}>刷新</button>
      <button class="mini" on:click={() => (pwShow = !pwShow)}>{pwShow ? '隐藏密码' : '密码'}</button>
    </div>
    {#if needPassword || pwShow}
      <div class="pwbar">
        <input
          type="password"
          bind:value={password}
          placeholder="SSH 密码（会话内使用，不落盘）"
          on:keydown={(e) => e.key === 'Enter' && load()}
        />
        <button class="mini" on:click={load}>重试</button>
      </div>
    {/if}
    {#if error && !needPassword}
      <div class="err">{error}</div>
    {/if}
    <div class="list">
      {#if loading}
        <div class="hint">加载中…</div>
      {:else}
        {#each entries as e (e.name)}
          <div
            class="entry"
            class:dir={e.isDir}
            role="button"
            tabindex="0"
            on:click={() => enter(e.name, e.isDir)}
            on:keydown={(e2) => e2.key === 'Enter' && enter(e.name, e.isDir)}
          >
            {e.isDir ? '📁' : '📄'} {e.name}
          </div>
        {/each}
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
    z-index: 100;
  }
  .dialog {
    width: 640px;
    height: 480px;
    background: var(--popup-bg, var(--card));
    border: 1px solid var(--popup-border, var(--border-strong));
    border-radius: 10px;
    display: flex;
    flex-direction: column;
    color: var(--fg, var(--fg));
  }
  .head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 14px;
    border-bottom: 1px solid var(--muted);
    font-size: 0.9375rem;
    font-family: 'SF Mono', Menlo, monospace;
  }
  .bar {
    display: flex;
    gap: 6px;
    padding: 6px 14px;
    border-bottom: 1px solid var(--muted);
  }
  .bar input {
    flex: 1;
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--fg);
    padding: 3px 8px;
    font-size: 0.875rem;
    font-family: 'SF Mono', Menlo, monospace;
  }
  .pwbar {
    display: flex;
    gap: 6px;
    padding: 6px 14px;
    border-bottom: 1px solid var(--muted);
  }
  .pwbar input {
    flex: 1;
    background: var(--card);
    border: 1px solid color-mix(in srgb, var(--error) 40%, transparent);
    border-radius: 4px;
    color: var(--fg);
    padding: 3px 8px;
    font-size: 0.875rem;
  }
  .err {
    padding: 6px 14px;
    color: var(--error);
    font-size: 0.8125rem;
  }
  .list {
    flex: 1;
    overflow-y: auto;
    padding: 6px;
  }
  .entry {
    padding: 3px 10px;
    font-size: 0.875rem;
    font-family: 'SF Mono', Menlo, monospace;
    border-radius: 4px;
    cursor: pointer;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .entry:hover {
    background: var(--hover);
  }
  .entry.dir {
    color: var(--primary);
  }
  .mini {
    background: var(--muted);
    color: var(--muted-fg);
    border: 1px solid var(--border-strong);
    border-radius: 4px;
    cursor: pointer;
    padding: 2px 8px;
    font-size: 0.8125rem;
  }
  .hint {
    color: var(--faint);
    padding: 8px 10px;
    font-size: 0.875rem;
  }
</style>
