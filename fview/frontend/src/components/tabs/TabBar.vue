<script setup lang="ts">
// 顶栏 tab 条：点击切换、×关闭（浏览器式；样式走主题 CSS 变量）
import { useTabsStore } from '../../stores/tabs'
import { closeTab } from '../../lib/backend'

const tabs = useTabsStore()
</script>

<template>
  <div class="tabbar">
    <button
      v-for="t in tabs.tabs"
      :key="t.info.id"
      class="tab"
      :class="{ active: t.info.id === tabs.activeId }"
      :title="t.info.info || t.info.label"
      @click="tabs.activeId = t.info.id"
    >
      <span class="label">{{ t.info.display || t.info.label }}</span>
      <span class="lines">{{ t.info.lines }}</span>
      <span
        class="close"
        title="关闭"
        @click.stop="closeTab(t.info.id).then(() => tabs.remove(t.info.id)).catch(() => {})"
      >
        <svg class="ico" viewBox="0 0 24 24"><path d="M18 6L6 18M6 6l12 12" /></svg>
      </span>
    </button>
    <!-- 标题栏融合后 tab 右侧空隙兼任窗口拖拽区(wails 原生 drag 协议) -->
    <div class="drag-spacer" style="--wails-draggable: drag"></div>
  </div>
</template>

<style scoped>
.tabbar {
  display: flex;
  align-items: center;
  gap: 2px;
  overflow-x: auto;
  flex: 1;
  min-width: 0;
}
.tab {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  background: transparent;
  color: var(--dim, #8b98a9);
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-size: var(--font-ui-base, 12px);
  white-space: nowrap;
  max-width: 220px;
}
.tab:hover {
  background: var(--popup-bg, #1c2836);
}
.tab.active {
  background: var(--popup-bg, #1f2c3f);
  color: var(--fg, #e6e6e6);
}
.label {
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 140px;
}
.lines {
  opacity: 0.5;
  font-size: var(--font-ui-sm, 11px);
}
.drag-spacer {
  flex: 1;
  height: 100%;
  min-width: 24px;
}
/* 关闭钮:svg 走全局 .ico(16px 描边 1.7,style.css 正本);弃 &times; 字形——视觉直径仅字号六成 */
.close {
  display: inline-flex;
  border-radius: 3px;
  padding: 0 5px;
}
.close:hover {
  background: var(--accent, #3a4a61);
  color: var(--bg, #fff);
}
</style>
