<script setup lang="ts">
// fview 主壳：顶栏 tab 条 + 图标条（Task 9 SidebarRail 接入）+ 主区（Task 8 LogView 接入）。
// 主题全局化：NConfigProvider 消费 theme store；tab 事件流在此统一接线。
import { onMounted } from 'vue'
import { useThemeStore } from './themes'
import { useTabsStore } from './stores/tabs'
import { subscribeTabEvents } from './lib/backend'
import TabBar from './components/tabs/TabBar.vue'
import Welcome from './components/Welcome.vue'

const theme = useThemeStore()
const tabs = useTabsStore()

onMounted(() => {
  theme.init()
  subscribeTabEvents({
    onOpened: tabs.applyOpened,
    onLines: tabs.applyLines,
    onMeta: tabs.applyMeta,
    onClosed: (ev) => tabs.remove(ev.tabId),
  })
})
</script>

<template>
  <n-config-provider :theme="theme.naiveTheme" :theme-overrides="theme.overrides">
    <div class="app">
      <header class="topbar">
        <TabBar />
      </header>
      <div class="body">
        <aside class="rail">
          <!-- 图标条 + 源面板：Task 9 接入 -->
        </aside>
        <main class="main">
          <Welcome v-if="tabs.tabs.length === 0" />
          <!-- 日志区（工具栏+LogView+状态栏）：Task 8 接入 -->
        </main>
      </div>
    </div>
  </n-config-provider>
</template>

<style scoped>
.app {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--bg, #10161f);
  color: var(--fg, #e6e6e6);
}
.topbar {
  display: flex;
  align-items: center;
  padding: 4px 8px;
  background: var(--popup-bg, #0c1119);
  border-bottom: 1px solid var(--popup-border, #1c2836);
}
.body {
  flex: 1;
  display: flex;
  min-height: 0;
}
.rail {
  width: 40px;
  background: var(--popup-bg, #141b26);
  border-right: 1px solid var(--popup-border, #2a3547);
}
.main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
</style>
