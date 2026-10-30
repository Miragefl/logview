<script lang="ts">
  // k8s 资源选择弹层：context 下拉 → namespace 下拉 → 资源列表 → 开 tab。
  import { createEventDispatcher, onMount } from 'svelte'
  import { getK8sContexts, getK8sNamespaces, getK8sResources, openK8sTab, type K8sResource } from '../lib/backend'

  const dispatch = createEventDispatcher<{ close: void }>()

  let contexts: string[] = []
  let kubeCtx = ''
  let namespaces: string[] = []
  let namespace = ''
  let resources: K8sResource[] = []
  let loading = ''
  let error = ''
  let loadSeq = 0 // 级联竞态守卫：旧响应不得覆盖新选择（Final Review I3）

  onMount(async () => {
    loading = 'contexts'
    try {
      contexts = await getK8sContexts()
      if (contexts.length > 0) {
        kubeCtx = contexts[0]
        await loadNamespaces()
      }
    } catch (e) {
      error = String(e)
    } finally {
      loading = ''
    }
  })

  async function loadNamespaces() {
    const seq = ++loadSeq
    loading = 'ns'
    error = ''
    try {
      const ns = await getK8sNamespaces(kubeCtx)
      if (seq !== loadSeq) return
      namespaces = ns
      namespace = namespaces.includes('default') ? 'default' : (namespaces[0] ?? '')
      await loadResources()
    } catch (e) {
      if (seq === loadSeq) error = String(e)
    } finally {
      if (seq === loadSeq) loading = ''
    }
  }

  async function openRes(kind: string, name: string) {
    try {
      await openK8sTab(`${kind}/${name}`, namespace, kubeCtx, 0)
      dispatch('close')
    } catch (e) {
      error = String(e) // 开 tab 失败不留僵弹层（Final Review I2）
    }
  }

  async function loadResources() {
    if (!namespace) return
    const seq = ++loadSeq
    loading = 'res'
    error = ''
    try {
      const res = await getK8sResources(kubeCtx, namespace)
      if (seq !== loadSeq) return
      resources = res
    } catch (e) {
      if (seq === loadSeq) error = String(e)
    } finally {
      if (seq === loadSeq) loading = ''
    }
  }
</script>

<div class="overlay" role="presentation" on:click|self={() => dispatch('close')}>
  <div class="dialog" role="dialog" aria-label="Kubernetes 资源选择">
    <div class="head">
      <span>Kubernetes</span>
      <button class="mini" on:click={() => dispatch('close')}>&times;</button>
    </div>
    <div class="bar">
      <select bind:value={kubeCtx} on:change={loadNamespaces}>
        {#each contexts as c (c)}
          <option value={c}>{c}</option>
        {/each}
      </select>
      <select bind:value={namespace} on:change={loadResources}>
        {#each namespaces as ns (ns)}
          <option value={ns}>{ns}</option>
        {/each}
      </select>
    </div>
    {#if error}
      <div class="err">{error}</div>
    {/if}
    <div class="list">
      {#if loading}
        <div class="hint">加载中（{loading}）…</div>
      {:else}
        {#each resources as r (`${r.kind}/${r.name}`)}
          <div
            class="res"
            role="button"
            tabindex="0"
            on:click={() => openRes(r.kind, r.name)}
            on:keydown={(e) => e.key === 'Enter' && openRes(r.kind, r.name)}
          >
            <span class="kind">{r.kind}</span> {r.name}
          </div>
        {/each}
        {#if resources.length === 0}
          <div class="hint">该 namespace 无工作负载资源</div>
        {/if}
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
    width: 560px;
    height: 460px;
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
  }
  .bar {
    display: flex;
    gap: 6px;
    padding: 6px 14px;
    border-bottom: 1px solid var(--muted);
  }
  .bar select {
    flex: 1;
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--fg);
    padding: 3px 6px;
    font-size: 0.875rem;
  }
  .err {
    padding: 4px 14px;
    color: var(--error);
    font-size: 0.8125rem;
  }
  .list {
    flex: 1;
    overflow-y: auto;
    padding: 6px;
  }
  .hint {
    color: var(--faint);
    padding: 8px 10px;
    font-size: 0.875rem;
  }
  .res {
    padding: 3px 10px;
    font-size: 0.875rem;
    font-family: 'SF Mono', Menlo, monospace;
    border-radius: 4px;
    cursor: pointer;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .res:hover {
    background: var(--hover);
  }
  .kind {
    color: var(--primary);
    margin-right: 6px;
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
</style>
