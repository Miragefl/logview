<script setup lang="ts">
// 图标条：四源；点图标从旁滑出面板，再点同图标或点日志区收起(设置入口在标题栏右端)。
// 图标为 22px 描边 SVG（emoji 跨平台渲染不稳且辨识度差——验收反馈②）。
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { isPortalNode } from '../../lib/portal'
import LocalPanel from './LocalPanel.vue'
import SSHPanel from './SSHPanel.vue'
import K8sPanel from './K8sPanel.vue'
import FRPPanel from './FRPPanel.vue'

type Pane = '' | 'local' | 'ssh' | 'k8s' | 'frp'
const active = ref<Pane>('')
const railEl = ref<HTMLElement | null>(null)

const icons: { key: Pane; label: string }[] = [
  { key: 'local', label: '本地文件' },
  { key: 'ssh', label: 'SSH' },
  { key: 'k8s', label: 'K8s' },
  { key: 'frp', label: 'FRP' },
]

function toggle(k: Pane) {
  active.value = active.value === k ? '' : k
}

/** cyclePane q 键循环源面板(第四轮验收③全键盘):local→ssh→k8s→frp→收起。 */
const ORDER: Exclude<Pane, ''>[] = ['local', 'ssh', 'k8s', 'frp']
function cyclePane() {
  const cur = ORDER.indexOf(active.value as Exclude<Pane, ''>)
  active.value = cur < 0 ? ORDER[0] : cur === ORDER.length - 1 ? '' : ORDER[cur + 1]
}
defineExpose({ cyclePane })

/** 点击 rail/面板外收起（面板在 rail 容器内，点日志区即收）；
 * naive-ui 的下拉菜单/弹窗 portal 到 body，不在 rail DOM 内——portal 容器内的
 * 点击（选 namespace、点密码框）不算外部，否则面板被误卸载（验收反馈①）。 */
function onDocClick(e: MouseEvent) {
  if (!railEl.value?.contains(e.target as Node) && !isPortalNode(e.target as Node)) {
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
        <!-- 本地：文件夹 -->
        <svg v-if="i.key === 'local'" viewBox="0 0 24 24" class="ico ico-lg">
          <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z" />
        </svg>
        <!-- SSH：终端 -->
        <svg v-else-if="i.key === 'ssh'" viewBox="0 0 24 24" class="ico ico-lg">
          <rect x="3" y="4" width="18" height="16" rx="2" />
          <path d="M7 9l3 3-3 3M13 15h4" />
        </svg>
        <!-- K8s：舵轮（圆心+七向辐条，helm 意象） -->
        <svg v-else-if="i.key === 'k8s'" viewBox="0 0 24 24" class="ico ico-lg">
          <circle cx="12" cy="12" r="3.2" />
          <path
            d="M12 8.8V3.5M12 15.2v5.3M8.8 12H3.5M15.2 12h5.3M9.7 9.7L6 6M14.3 14.3L18 18M14.3 9.7L18 6M9.7 14.3L6 18"
          />
        </svg>
        <!-- FRP：链路 -->
        <svg v-else-if="i.key === 'frp'" viewBox="0 0 24 24" class="ico ico-lg">
          <path d="M10 14a4 4 0 0 0 5.66 0l3.17-3.18a4 4 0 1 0-5.66-5.65L11.6 6.74" />
          <path d="M14 10a4 4 0 0 0-5.66 0L5.17 13.18a4 4 0 1 0 5.66 5.65l1.57-1.57" />
        </svg>
      </button>
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
  width: 46px;
  display: flex;
  flex-direction: column;
  background: var(--popup-bg, #141b26);
  border-right: 1px solid var(--popup-border, #2a3547);
}
.icons {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding-top: 10px;
}
.icon-btn {
  width: 36px;
  height: 36px;
  border: none;
  border-radius: 8px;
  background: transparent;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--dim, #8b98a9);
}
.icon-btn:hover {
  color: var(--fg, #e6e6e6);
  background: var(--popup-border, #1f2c3f);
}
.icon-btn.on {
  color: var(--accent, #7dc4e4);
  background: color-mix(in srgb, var(--accent, #2a4d8f) 22%, transparent);
}
/* 图标基类收编 style.css 全局正本(2026-11-03 统一),此处仅留布局附加 */
.flyout {
  position: absolute;
  left: 50px;
  top: 8px;
  z-index: 20;
  background: var(--popup-bg, #16202e);
  border: 1px solid var(--popup-border, #2a3a4f);
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
}
</style>
