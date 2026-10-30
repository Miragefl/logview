<script lang="ts">
  // 连接管家侧边栏（瘦身版）：配置管理在 ConfigDialog 弹窗，此处只做连接入口。
  import { createEventDispatcher, onMount } from 'svelte'
  import { getFrpConns, getSSHHosts, type FrpConn } from '../lib/backend'

  const dispatch = createEventDispatcher<{
    openLocal: boolean
    browseSSH: string
    openFRP: FrpConn
    openK8s: void
    openConfig: 'conn' | 'rules' | 'ui'
  }>()

  let sshHosts: string[] = []
  let conns: FrpConn[] = []

  export async function refresh() {
    sshHosts = await getSSHHosts()
    conns = await getFrpConns()
  }

  onMount(refresh)
</script>

<div class="sources">
  <section>
    <h3>本地文件</h3>
    <button class="act" on:click={() => dispatch('openLocal', false)}>打开文件</button>
    <button class="act follow" on:click={() => dispatch('openLocal', true)}>打开并跟踪</button>
  </section>

  <section>
    <h3>
      SSH 主机（{sshHosts.length}）
      <button class="mini" title="配置" on:click={() => dispatch('openConfig', 'conn')}>⚙</button>
    </h3>
    {#each sshHosts as h (h)}
      <div class="item">
        <button class="link" title="浏览远程目录" on:click={() => dispatch('browseSSH', h)}>{h}</button>
      </div>
    {/each}
  </section>

  <section>
    <h3>
      FRP 连接（{conns.length}）
      <button class="mini" title="配置" on:click={() => dispatch('openConfig', 'conn')}>⚙</button>
    </h3>
    {#each conns as c (c.name)}
      <div class="item">
        <button class="link" title={`${c.server} · ${c.proxy}`} on:click={() => dispatch('openFRP', c)}>
          {c.name}
        </button>
      </div>
    {/each}
  </section>

  <section>
    <h3>Kubernetes</h3>
    <button class="act" on:click={() => dispatch('openK8s')}>选择集群资源…</button>
  </section>

  <div class="foot">
    <button class="cfg" on:click={() => dispatch('openConfig', 'conn')}>⚙ 设置</button>
  </div>
</div>

<style>
  h3 {
    margin: 0 0 6px;
    font-size: 13px;
    color: #9aa0a8;
    display: flex;
    justify-content: space-between;
    align-items: center;
    text-transform: uppercase;
    letter-spacing: 1px;
  }
  .act {
    display: block;
    width: 100%;
    background: #e2e2d8;
    color: #3a3f47;
    border: 1px solid #c7c7bc;
    border-radius: 5px;
    padding: 6px 8px;
    cursor: pointer;
    font-size: 15px;
    margin-bottom: 4px;
    text-align: left;
  }
  .act:hover { background: #d8d8cd; }
  .act.follow { color: #15803d; }
  .item { display: flex; }
  .link {
    flex: 1;
    min-width: 0;
    background: none;
    border: none;
    color: #2563eb;
    cursor: pointer;
    text-align: left;
    font-size: 15px;
    padding: 3px 4px;
    border-radius: 4px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .link:hover { background: #f0f0e7; }
  .mini {
    background: #e2e2d8;
    color: #6b7280;
    border: 1px solid #c7c7bc;
    border-radius: 4px;
    cursor: pointer;
    padding: 1px 7px;
    font-size: 13px;
  }
  .mini:hover { color: #1a1d21; }
  .foot { margin-top: auto; }
  .cfg {
    width: 100%;
    background: none;
    border: 1px dashed #c7c7bc;
    border-radius: 5px;
    color: #6b7280;
    cursor: pointer;
    font-size: 15px;
    padding: 6px;
  }
  .cfg:hover { color: #1a1d21; border-color: #4a6fa5; }
</style>
