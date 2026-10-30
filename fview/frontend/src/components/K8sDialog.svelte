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
    loading = 'ns'
    error = ''
    try {
      namespaces = await getK8sNamespaces(kubeCtx)
      namespace = namespaces.includes('default') ? 'default' : (namespaces[0] ?? '')
      await loadResources()
    } catch (e) {
      error = String(e)
    } finally {
      loading = ''
    }
  }

  async function openRes(kind: string, name: string) {
    await openK8sTab(`${kind}/${name}`, namespace, kubeCtx, 0)
    dispatch('close')
  }

  async function loadResources() {
    if (!namespace) return
    loading = 'res'
    error = ''
    try {
      resources = await getK8sResources(kubeCtx, namespace)
    } catch (e) {
      error = String(e)
    } finally {
      loading = ''
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
  }
  .bar {
    display: flex;
    gap: 6px;
    padding: 6px 14px;
    border-bottom: 1px solid #1c2836;
  }
  .bar select {
    flex: 1;
    background: #10161f;
    border: 1px solid #24354a;
    border-radius: 4px;
    color: #e6e6e6;
    padding: 3px 6px;
    font-size: 12px;
  }
  .err {
    padding: 4px 14px;
    color: #ff6b6b;
    font-size: 11px;
  }
  .list {
    flex: 1;
    overflow-y: auto;
    padding: 6px;
  }
  .hint {
    color: #46586e;
    padding: 8px 10px;
    font-size: 12px;
  }
  .res {
    padding: 3px 10px;
    font-size: 12px;
    font-family: 'SF Mono', Menlo, monospace;
    border-radius: 4px;
    cursor: pointer;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .res:hover {
    background: #1a2534;
  }
  .kind {
    color: #7cc0ff;
    margin-right: 6px;
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
</style>
