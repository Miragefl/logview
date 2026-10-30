<script setup lang="ts">
// 图标条：四源 + 底部设置；点图标从旁滑出面板，再点同图标或点日志区收起。
import { onBeforeUnmount, onMounted, ref } from 'vue'
import LocalPanel from './LocalPanel.vue'
import SSHPanel from './SSHPanel.vue'
import K8sPanel from './K8sPanel.vue'
import FRPPanel from './FRPPanel.vue'

const emit = defineEmits<{ (e: 'open-settings'): void }>()

type Pane = '' | 'local' | 'ssh' | 'k8s' | 'frp'
const active = ref<Pane>('')
const railEl = ref<HTMLElement | null>(null)

const icons: { key: Pane; label: string; icon: string }[] = [
  { key: 'local', label: '本地文件', icon: '📁' },
  { key: 'ssh', label: 'SSH', icon: '🖥' },
  { key: 'k8s', label: 'K8s', icon: '☸' },
  { key: 'frp', label: 'FRP', icon: '🔗' },
]

function toggle(k: Pane) {
  active.value = active.value === k ? '' : k
}

/** 点击 rail/面板外收起（面板在 rail 容器内，点日志区即收）。 */
function onDocClick(e: MouseEvent) {
  if (!railEl.value?.contains(e.target as Node)) {
    active.value = ''
  }
}
onMounted(() => document.addEventListener('mousedown', onDocClick))
onBeforeUnmount(() => document.removeEventListener('mousedown', onDocClick))
</script>

<template>
  <div ref="railEl" class="rail">
    <div class="icons">
      <button
        v-for="i in icons"
        :key="i.key"
        class="icon-btn"
        :class="{ on: active === i.key }"
        :title="i.label"
        @click="toggle(i.key)"
      >
        {{ i.icon }}
      </button>
    </div>
    <div class="bottom">
      <button class="icon-btn" title="设置" @click="emit('open-settings')">⚙</button>
    </div>
    <div v-if="active" class="flyout">
      <LocalPanel v-if="active === 'local'" />
      <SSHPanel v-else-if="active === 'ssh'" />
      <K8sPanel v-else-if="active === 'k8s'" />
      <FRPPanel v-else-if="active === 'frp'" />
    </div>
  </div>
</template>

<style scoped>
.rail {
  position: relative;
  width: 40px;
  display: flex;
  flex-direction: column;
  background: var(--popup-bg, #141b26);
  border-right: 1px solid var(--popup-border, #2a3547);
}
.icons {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding-top: 10px;
}
.bottom {
  margin-top: auto;
  display: flex;
  justify-content: center;
  padding-bottom: 10px;
}
.icon-btn {
  width: 30px;
  height: 30px;
  border: none;
  border-radius: 6px;
  background: transparent;
  font-size: 15px;
  cursor: pointer;
  opacity: 0.55;
  display: flex;
  align-items: center;
  justify-content: center;
}
.icon-btn:hover {
  opacity: 1;
  background: var(--popup-border, #1f2c3f);
}
.icon-btn.on {
  opacity: 1;
  background: var(--accent, #2a4d8f);
}
.flyout {
  position: absolute;
  left: 44px;
  top: 8px;
  z-index: 20;
  background: var(--popup-bg, #16202e);
  border: 1px solid var(--popup-border, #2a3a4f);
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
}
</style>
