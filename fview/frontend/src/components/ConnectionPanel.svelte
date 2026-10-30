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

<aside class="panel">
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
</aside>

<style>
  .panel {
    width: 220px;
    flex-shrink: 0;
    background: #0c1119;
    border-right: 1px solid #1c2836;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 10px 8px;
    font-size: 15px;
  }
  h3 {
    margin: 0 0 6px;
    font-size: 13px;
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
    padding: 6px 8px;
    cursor: pointer;
    font-size: 15px;
    margin-bottom: 4px;
    text-align: left;
  }
  .act:hover { background: #243348; }
  .act.follow { color: #7ee29a; }
  .item { display: flex; }
  .link {
    flex: 1;
    min-width: 0;
    background: none;
    border: none;
    color: #7cc0ff;
    cursor: pointer;
    text-align: left;
    font-size: 15px;
    padding: 3px 4px;
    border-radius: 4px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .link:hover { background: #1a2534; }
  .mini {
    background: #1c2836;
    color: #8b98a9;
    border: 1px solid #2a3a4f;
    border-radius: 4px;
    cursor: pointer;
    padding: 1px 7px;
    font-size: 13px;
  }
  .mini:hover { color: #e6e6e6; }
  .foot { margin-top: auto; }
  .cfg {
    width: 100%;
    background: none;
    border: 1px dashed #2a3a4f;
    border-radius: 5px;
    color: #8b98a9;
    cursor: pointer;
    font-size: 15px;
    padding: 6px;
  }
  .cfg:hover { color: #e6e6e6; border-color: #3671cc; }
</style>
