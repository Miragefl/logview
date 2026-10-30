<script setup lang="ts">
// 设置中心：左导航三分类（外观/解析规则/关于），字段显隐在工具栏不进此（避免双入口）。
import { ref } from 'vue'
import { NModal } from 'naive-ui'
import SettingsAppearance from './SettingsAppearance.vue'
import SettingsRules from './SettingsRules.vue'
import SettingsAbout from './SettingsAbout.vue'

defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void }>()

type Page = 'appearance' | 'rules' | 'about'
const page = ref<Page>('appearance')
const navItems: { key: Page; label: string }[] = [
  { key: 'appearance', label: '🎨 外观' },
  { key: 'rules', label: '📋 解析规则' },
  { key: 'about', label: 'ℹ️ 关于' },
]
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    title="设置"
    style="width: 640px"
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
          {{ i.label }}
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
  width: 110px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: none;
}
.nav-item {
  text-align: left;
  border: none;
  background: transparent;
  color: var(--dim, #8b98a9);
  padding: 6px 10px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 13px;
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
  font-size: 13px;
}
</style>
