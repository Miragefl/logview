<script setup lang="ts">
// 外观页：主题下拉（全局多配色，壳+日志区一起换）/ 默认字段显示 / 系统字体下拉 / 字号 / 行高。
// 字段默认层写 rules.yaml(与 TUI 共享)；单日志覆盖在工具栏 F,两层互不沾。
import { computed, onMounted, ref } from 'vue'
import { NInputNumber, NSelect } from 'naive-ui'
import { useThemeStore } from '../../themes'
import { useUiStore } from '../../stores/ui'
import { useFieldsStore } from '../../stores/fields'
import { HEADER_LABELS } from '../../lib/columns'
import { systemFonts } from '../../lib/backend'

const theme = useThemeStore()
const ui = useUiStore()
const fieldsStore = useFieldsStore()

/** themeOptions 主题下拉项(nameList 单源,选择即 theme.apply 即时生效)。 */
const themeOptions = computed(() => theme.nameList.map((n) => ({ label: n, value: n })))

const fontOptions = ref<{ label: string; value: string }[]>([])

// 面板自取主题列表（App 的 init 幂等，此处兜底单测/异常路径）
onMounted(async () => {
  if (!theme.nameList.length) {
    theme.init()
  }
  if (!fieldsStore.loaded) {
    fieldsStore.load()
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
      <NSelect
        :value="theme.name"
        size="small"
        :options="themeOptions"
        @update:value="(v: string) => theme.apply(v)"
      />
    </section>
    <section>
      <h4>默认字段显示</h4>
      <div class="field-opts">
        <label v-for="f in fieldsStore.all" :key="f.name" class="field-opt" :title="f.name">
          <input
            type="checkbox"
            :checked="f.visible"
            :disabled="f.name === 'message'"
            @change="fieldsStore.setVisibility(f.name, !f.visible)"
          />{{ HEADER_LABELS[f.name] ?? f.name }}
        </label>
      </div>
      <p class="hint">所有日志初始显示（与 TUI 共享 rules.yaml）；单个日志可在工具栏「字段」单独调整。</p>
    </section>
    <!-- 字体独占整行:长字体名完整可见(验收反馈⑧);字号/行高第二行二等分 -->
    <section>
      <h4>字体</h4>
      <NSelect
        :value="ui.fontFamily"
        size="small"
        filterable
        placeholder="系统字体"
        :options="fontOptions"
        @update:value="onFontChange"
      />
    </section>
    <section class="row">
      <div>
        <h4>界面字号</h4>
        <NInputNumber
          v-model:value="ui.uiFontSize"
          size="small"
          :min="10"
          :max="18"
          @update:value="ui.persist()"
        />
      </div>
      <div>
        <h4>日志字号</h4>
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
    <p class="hint">主题切换即时生效（含原生标题栏亮暗）；界面字号作用于工具栏/弹窗等非日志文字；其余仅桌面端保存。</p>
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
  font-size: var(--font-ui-base, 12px);
  color: var(--dim, #8b98a9);
  font-weight: normal;
}
.field-opts {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 14px;
}
.field-opt {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: var(--font-ui-base, 12px);
  cursor: pointer;
  color: var(--fg, #c8d3e0);
}
.field-opt input {
  accent-color: var(--accent, #4a8fe0);
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
  font-size: var(--font-ui-sm, 11px);
  margin: 0;
}
</style>
