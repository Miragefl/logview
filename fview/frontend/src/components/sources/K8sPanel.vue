<script setup lang="ts">
// K8s 面板：context → namespace 两级下拉 + 资源列表，点资源直接开 tab（kubectl logs 复用）。
import { onMounted, ref, watch } from 'vue'
import { NSelect } from 'naive-ui'
import { getK8sContexts, getK8sNamespaces, getK8sResources, openK8sTab, usageTouch } from '../../lib/backend'

const ctx = ref<string | null>(null)
const ns = ref<string | null>(null)
const ctxOptions = ref<{ label: string; value: string }[]>([])
const nsOptions = ref<{ label: string; value: string }[]>([])
const resources = ref<{ kind: string; name: string }[]>([])

onMounted(async () => {
  try {
    ctxOptions.value = ((await getK8sContexts()) ?? []).map((c) => ({ label: c, value: c }))
    if (ctxOptions.value.length) {
      ctx.value = ctxOptions.value[0].value
    }
  } catch {
    /* 无 kubeconfig：下拉为空 */
  }
})

watch(ctx, async (c) => {
  ns.value = null
  resources.value = []
  if (!c) return
  try {
    nsOptions.value = ((await getK8sNamespaces(c)) ?? []).map((n) => ({ label: n, value: n }))
    if (nsOptions.value.length) {
      ns.value = nsOptions.value[0].value
    }
  } catch {
    nsOptions.value = []
  }
})

watch(ns, async (n) => {
  resources.value = []
  if (!n || !ctx.value) return
  try {
    const rs = (await getK8sResources(ctx.value, n)) ?? []
    resources.value = rs.map((r) => ({ kind: r.kind, name: r.name }))
  } catch {
    resources.value = []
  }
})

async function open(r: { kind: string; name: string }) {
  if (!ctx.value || !ns.value) return
  try {
    await openK8sTab(`${r.kind}/${r.name}`, ns.value, ctx.value, 0)
    usageTouch(`k8sres:${ctx.value}/${ns.value}/${r.kind}/${r.name}`)
  } catch {
    /* 打开失败静默 */
  }
}
</script>

<template>
  <div class="panel">
    <div class="head">
      <span>Kubernetes</span>
    </div>
    <div class="sel-row">
      <NSelect v-model:value="ctx" size="tiny" placeholder="context" :options="ctxOptions" />
      <NSelect v-model:value="ns" size="tiny" placeholder="namespace" :options="nsOptions" />
    </div>
    <ul class="list">
      <li v-for="r in resources" :key="r.kind + '/' + r.name" class="res-item" @click="open(r)">
        🟢 {{ r.kind }}/{{ r.name }}
      </li>
    </ul>
    <p v-if="!resources.length" class="empty">{{ ctx ? '该 namespace 暂无工作负载' : '未发现 kubeconfig' }}</p>
  </div>
</template>

<style scoped>
.panel {
  width: 250px;
  padding: 8px;
  font-size: 12px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.head {
  color: var(--dim, #8b98a9);
}
/* 上下堆叠占满面板宽：并排 115px 会截断长 namespace（验收反馈①） */
.sel-row {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.list {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 44vh;
  overflow-y: auto;
  font-family: var(--font-mono, monospace);
}
.list li {
  padding: 4px 6px;
  border-radius: 4px;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.list li:hover {
  background: var(--popup-bg, #1f2c3f);
}
.empty {
  color: var(--dim, #5d7292);
}
</style>
