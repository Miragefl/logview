<script setup lang="ts">
// 外观页：主题（全局多配色，壳+日志区一起换）/ 系统字体下拉 / 字号 / 行高。
import { onMounted, ref } from 'vue'
import { NInputNumber, NSelect } from 'naive-ui'
import { useThemeStore } from '../../themes'
import { useUiStore } from '../../stores/ui'
import { systemFonts } from '../../lib/backend'

const theme = useThemeStore()
const ui = useUiStore()

const fontOptions = ref<{ label: string; value: string }[]>([])

// 面板自取主题列表（App 的 init 幂等，此处兜底单测/异常路径）
onMounted(async () => {
  if (!theme.nameList.length) {
    theme.init()
  }
  // 系统字体下拉（枚举失败保底 monospace）
  try {
    const fonts = (await systemFonts()) ?? []
    fontOptions.value = fonts.map((f) => ({ label: f, value: f }))
  } catch {
    fontOptions.value = []
  }
})

function onFontChange(v: string | null) {
  ui.fontFamily = v || 'SF Mono, Menlo, monospace'
  ui.persist()
}
</script>

<template>
  <div class="appearance">
    <section>
      <h4>主题（与 TUI rules.yaml 共享）</h4>
      <div class="themes">
        <button
          v-for="n in theme.nameList"
          :key="n"
          class="theme-chip"
          :class="{ on: theme.name === n }"
          @click="theme.apply(n)"
        >
          {{ n }}
        </button>
        <span v-if="!theme.nameList.length" class="hint">主题列表为空</span>
      </div>
    </section>
    <section class="row">
      <div>
        <h4>字体</h4>
        <NSelect
          :value="ui.fontFamily"
          size="small"
          filterable
          placeholder="系统字体"
          :options="fontOptions"
          @update:value="onFontChange"
        />
      </div>
      <div>
        <h4>字号</h4>
        <NInputNumber
          v-model:value="ui.fontSize"
          size="small"
          :min="9"
          :max="28"
          @update:value="ui.persist()"
        />
      </div>
      <div>
        <h4>行高</h4>
        <NInputNumber
          v-model:value="ui.lineHeight"
          size="small"
          :min="1"
          :max="2.5"
          :step="0.1"
          @update:value="ui.persist()"
        />
      </div>
    </section>
    <p class="hint">主题切换即时生效（含原生标题栏亮暗）；字体/字号/行高仅桌面端保存（localStorage）。</p>
  </div>
</template>

<style scoped>
.appearance {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
h4 {
  margin: 0 0 6px;
  font-size: 12px;
  color: var(--dim, #8b98a9);
  font-weight: normal;
}
.themes {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.theme-chip {
  border: 1px solid var(--popup-border, #2a3547);
  background: var(--popup-bg, #0f1520);
  color: var(--fg, #c8d3e0);
  border-radius: 4px;
  padding: 4px 12px;
  cursor: pointer;
  font-size: 12px;
}
.theme-chip.on {
  border-color: var(--accent, #4a8fe0);
  color: var(--accent, #4a8fe0);
}
.row {
  display: flex;
  gap: 14px;
}
.row > div {
  flex: 1;
}
.hint {
  color: var(--dim, #5d7292);
  font-size: 11px;
  margin: 0;
}
</style>
