<script lang="ts">
  // 远程目录浏览弹层：路径栏 + 目录/文件列表 + 双击进入 + 选文件回调。
  // 认证失败（错误信息含 auth/password 字样）弹密码输入行重试。
  import { sshBrowse } from '../lib/backend'

  export let host: string
  export let initialPath = '/'
  export let onPick: (path: string, password: string) => void = () => {}
  export let onClose: () => void = () => {}

  let path = initialPath
  let entries: Array<{ name: string; isDir: boolean }> = []
  let loading = false
  let error = ''
  let password = ''
  let needPassword = false

  async function load() {
    loading = true
    error = ''
    try {
      entries = await sshBrowse(host, path, password)
      needPassword = false
    } catch (e) {
      error = String(e)
      if (/auth|password|denied|permission/i.test(error)) {
        needPassword = true
      }
    } finally {
      loading = false
    }
  }

  function enter(name: string, isDir: boolean) {
    if (isDir) {
      path = path === '/' ? `/${name}` : `${path}/${name}`
      load()
    } else {
      onPick(`${path}/${name}`, password)
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
      <span>{host}:{path}</span>
      <button class="mini" on:click={onClose}>&times;</button>
    </div>
    <div class="bar">
      <button class="mini" on:click={goUp}>↑ 上级</button>
      <input bind:value={path} on:keydown={(e) => e.key === 'Enter' && load()} />
      <button class="mini" on:click={load}>刷新</button>
    </div>
    {#if needPassword}
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
    padding: 8px 14px;
    border-bottom: 1px solid #1c2836;
    font-size: 13px;
    font-family: 'SF Mono', Menlo, monospace;
  }
  .bar {
    display: flex;
    gap: 6px;
    padding: 6px 14px;
    border-bottom: 1px solid #1c2836;
  }
  .bar input {
    flex: 1;
    background: #10161f;
    border: 1px solid #24354a;
    border-radius: 4px;
    color: #e6e6e6;
    padding: 3px 8px;
    font-size: 12px;
    font-family: 'SF Mono', Menlo, monospace;
  }
  .pwbar {
    display: flex;
    gap: 6px;
    padding: 6px 14px;
    border-bottom: 1px solid #1c2836;
  }
  .pwbar input {
    flex: 1;
    background: #10161f;
    border: 1px solid #8a3038;
    border-radius: 4px;
    color: #e6e6e6;
    padding: 3px 8px;
    font-size: 12px;
  }
  .err {
    padding: 6px 14px;
    color: #ff6b6b;
    font-size: 11px;
  }
  .list {
    flex: 1;
    overflow-y: auto;
    padding: 6px;
  }
  .entry {
    padding: 3px 10px;
    font-size: 12px;
    font-family: 'SF Mono', Menlo, monospace;
    border-radius: 4px;
    cursor: pointer;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .entry:hover {
    background: #1a2534;
  }
  .entry.dir {
    color: #7cc0ff;
  }
  .mini {
    background: #1c2836;
    color: #8b98a9;
    border: 1px solid #2a3a4f;
    border-radius: 4px;
    cursor: pointer;
    padding: 2px 8px;
    font-size: 11px;
  }
  .hint {
    color: #46586e;
    padding: 8px 10px;
    font-size: 12px;
  }
</style>
