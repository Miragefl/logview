<script setup lang="ts">
// 键位速查弹窗(? 键):内容与 lib/keymap.ts 对齐;任意键/点击遮罩关闭。
import { onBeforeUnmount, onMounted } from 'vue'

defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void }>()

function close() {
  emit('update:show', false)
}
function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.stopPropagation()
  }
  close()
}
onMounted(() => document.addEventListener('keydown', onKey, true))
onBeforeUnmount(() => document.removeEventListener('keydown', onKey, true))

const KEYS: [string, string][] = [
  ['j / k', '光标下移 / 上移一行'],
  ['g / G', '回顶(关跟随) / 到底(开跟随)'],
  ['ctrl+d / ctrl+u', '下移 / 上移半页'],
  ['ctrl+f / ctrl+b, PgDn/PgUp', '下翻 / 上翻整页'],
  ['H / M / L', '光标跳可视区顶 / 中 / 底'],
  ['zt / zz / zb', '光标行置顶 / 居中 / 置底'],
  ['n / N', '下一个 / 上一个搜索命中'],
  ['/ 或 f, h, x', '搜索弹窗:搜索 / 高亮 / 隐藏分区(Tab 轮换,ctrl+R 历史)'],
  ['E / W / I / D / A', '级别档位:ERROR / ≥WARN / ≥INFO / DEBUG / 全部'],
  ['e', '折叠 / 展开光标处堆栈组'],
  ['d', '光标行详情(同双击)'],
  ['v / V, y, Esc', '可视选择 / 复制选中行 / 退出'],
  ['m / \'', '光标行加书签 / 书签列表跳转'],
  ['F', '字段显隐面板'],
  ['#', '行号开关'],
  ['w', '换行开关(仅折 message 列)'],
  ['C', '清屏(缓冲全清)'],
  ['?', '本帮助'],
]
</script>

<template>
  <Teleport to="body">
    <div v-if="show" class="overlay" @click.self="close()">
      <div class="dialog">
        <div class="title">键位速查</div>
        <table class="keys">
          <tr v-for="[k, desc] in KEYS" :key="k">
            <td class="k">{{ k }}</td>
            <td class="d">{{ desc }}</td>
          </tr>
        </table>
        <p class="hint">鼠标:双击行详情 · 右键隐藏 · 滚轮/拖拽滚动 · 点击折叠行展开</p>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  z-index: 110;
  background: rgba(0, 0, 0, 0.45);
  display: flex;
  align-items: center;
  justify-content: center;
}
.dialog {
  width: 520px;
  max-width: 90vw;
  background: var(--popup-bg, #16202e);
  border: 1px solid var(--popup-border, #2a3a4f);
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
  padding: 12px 16px 14px;
}
.title {
  font-size: 13px;
  color: var(--dim, #8b98a9);
  margin-bottom: 8px;
}
.keys {
  border-collapse: collapse;
  font-size: 12px;
  width: 100%;
}
.keys td {
  padding: 3px 8px 3px 0;
  vertical-align: top;
}
.k {
  color: var(--accent, #7dc4e4);
  white-space: pre;
  font-family: var(--font-mono, monospace);
  width: 200px;
}
.d {
  color: var(--fg, #c8d3e0);
}
.hint {
  margin: 10px 0 0;
  font-size: 11px;
  color: var(--dim, #5d7292);
}
</style>
