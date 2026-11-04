<script setup lang="ts">
// 设置中心：左导航三分类（外观/解析规则/关于），字段显隐在工具栏不进此（避免双入口）。
import { onBeforeUnmount, ref, watch } from 'vue'
import { NModal } from 'naive-ui'
import { useUiStore } from '../../stores/ui'
import SettingsAppearance from './SettingsAppearance.vue'
import SettingsRules from './SettingsRules.vue'
import SettingsAbout from './SettingsAbout.vue'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void }>()

const ui = useUiStore()

type Page = 'appearance' | 'rules' | 'about'
const page = ref<Page>('appearance')
// 图标与 SidebarRail 同风格:22px 描边 SVG(stroke currentColor)——emoji 弃用(验收反馈⑦)
const navItems: { key: Page; label: string; icon: string }[] = [
  { key: 'appearance', label: '外观', icon: 'palette' },
  { key: 'rules', label: '解析规则', icon: 'rules' },
  { key: 'about', label: '关于', icon: 'about' },
]

// 弹窗登记(键盘层让位)+ ESC 兜底关闭(第三轮验收②:naive 的 esc
// 依赖焦点管理,焦点在 body 时关不掉,自己监听才可靠)
function onDocKey(e: KeyboardEvent) {
  if (e.key === 'Escape' && ui.topDialog === 'settings') {
    emit('update:show', false)
  }
}
watch(
  () => props.show,
  (v) => {
    if (v) {
      ui.dialogOpen('settings')
      document.addEventListener('keydown', onDocKey)
    } else {
      ui.dialogClose('settings')
      document.removeEventListener('keydown', onDocKey)
    }
  },
  { immediate: true }
)
onBeforeUnmount(() => {
  ui.dialogClose('settings')
  document.removeEventListener('keydown', onDocKey)
})
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    title="设置"
    style="width: min(94vw, calc(var(--ui-font-size) * 54))"
    @update:show="(v: boolean) => emit('update:show', v)"
  >
    <div class="settings">
      <nav class="nav">
        <button
          v-for="i in navItems"
          :key="i.key"
          class="nav-item"
          :class="{ on: page === i.key }"
          @click="page = i.key"
        >
          <!-- 外观:调色板 -->
          <svg v-if="i.icon === 'palette'" viewBox="0 0 24 24" class="ico ico-lg">
            <path d="M12 3a9 9 0 1 0 0 18c1.5 0 2-.9 2-2 0-1.4-1-1.6-1-2.6 0-.9.7-1.4 1.8-1.4H17a4 4 0 0 0 4-4c0-4.4-4-8-9-8z" />
            <circle cx="7.5" cy="11" r="0.6" />
            <circle cx="11" cy="7.5" r="0.6" />
            <circle cx="15.5" cy="8.5" r="0.6" />
          </svg>
          <!-- 解析规则:列表+尖括号 -->
          <svg v-else-if="i.icon === 'rules'" viewBox="0 0 24 24" class="ico ico-lg">
            <path d="M4 6h16M4 12h10M4 18h7" />
            <path d="M17 14l-2.5 2.5L17 19M21 14l-2.5 2.5L21 19" />
          </svg>
          <!-- 关于:info -->
          <svg v-else viewBox="0 0 24 24" class="ico ico-lg">
            <circle cx="12" cy="12" r="9" />
            <path d="M12 11v5" />
            <circle cx="12" cy="8" r="0.6" />
          </svg>
          <span>{{ i.label }}</span>
        </button>
      </nav>
      <div class="content">
        <SettingsAppearance v-if="page === 'appearance'" />
        <SettingsRules v-else-if="page === 'rules'" />
        <SettingsAbout v-else />
      </div>
    </div>
  </NModal>
</template>

<style scoped>
.settings {
  display: flex;
  gap: 14px;
  min-height: 300px;
}
.nav {
  width: calc(var(--ui-font-size) * 9);
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: none;
}
.nav-item {
  display: flex;
  align-items: center;
  gap: 8px;
  text-align: left;
  border: none;
  background: transparent;
  color: var(--dim, #8b98a9);
  padding: 6px 10px;
  border-radius: 4px;
  cursor: pointer;
  font-size: var(--font-ui-lg, 13px);
}
/* 与 SidebarRail 同款描边图标 */
/* 图标基类收编 style.css 全局正本(2026-11-03 统一),此处仅留 flex 布局附加 */
.ico {
  flex: none;
}
.nav-item:hover {
  background: var(--popup-bg, #1f2c3f);
}
.nav-item.on {
  background: var(--accent, #2a4d8f);
  color: #fff;
}
.content {
  flex: 1;
  min-width: 0;
  font-size: var(--font-ui-lg, 13px);
}
</style>
