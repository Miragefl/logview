<script setup lang="ts">
// 键位速查弹窗(? 键):内容与 lib/keymap.ts 对齐;任意键/点击遮罩关闭。
import { onBeforeUnmount, onMounted, watch } from 'vue'
import { useUiStore } from '../../stores/ui'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void }>()

const ui = useUiStore()

function close() {
  emit('update:show', false)
}
function onKey(e: KeyboardEvent) {
  if (!props.show) return
  if (e.key === 'Escape') {
    e.stopPropagation()
  }
  close()
}
onMounted(() => document.addEventListener('keydown', onKey, true))
onBeforeUnmount(() => document.removeEventListener('keydown', onKey, true))
// 弹窗栈登记(全局键让位,第三轮验收②)
watch(
  () => props.show,
  (v) => (v ? ui.dialogOpen('help') : ui.dialogClose('help')),
  { immediate: true }
)

const KEYS: [string, string][] = [
  ['j / k', '光标下移 / 上移一行'],
  ['g / G', '回顶(关跟随) / 到底(开跟随)'],
  ['ctrl+d / ctrl+u', '下移 / 上移半页'],
  ['ctrl+f / ctrl+b, PgDn/PgUp', '下翻 / 上翻整页'],
  ['H / M / L', '光标跳可视区顶 / 中 / 底'],
  ['zt / zz / zb', '光标行置顶 / 居中 / 置底'],
  ['n / N', '下一个 / 上一个命中(搜索优先,无搜索时高亮词)'],
  ['/ 或 f', '聚焦搜索条(回车执行,清空回车恢复,Esc 失焦不清词)'],
  ['h, x', '高亮 / 隐藏词管理框(ctrl+j/k 选中,d 删词,ctrl+R 历史)'],
  ['E / W / I / D / A', '级别档位:ERROR / ≥WARN / ≥INFO / DEBUG / 全部'],
  ['e', '折叠 / 展开光标处堆栈组'],
  ['d', '光标行详情(同双击)'],
  ['v / V, y, Esc', '可视选择 / 复制(无选区=当前行,TUI 同款) / 退出'],
  ['⌘C / Ctrl+C', '复制选区(鼠标拖选或 v 选区;无选区不拦截系统复制)'],
  ['m / \'', '光标行加书签 / 书签列表(ctrl+j/k 选中,Enter 跳转)'],
  ['q', '源面板循环(本地→SSH→K8s→FRP→收起)'],
  [', / cmd+,', '设置中心'],
  ['ctrl+1..9 / cmd+1..9', '切换 tab'],
  ['ctrl+w / ⌘w', '关闭 tab(mac 原生菜单)'],
  ['ctrl+= / ctrl+-, cmd+= / cmd+-', '字号放大 / 缩小'],
  ['F', '字段显隐面板(可连续勾选,点外部退出)'],
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
        <p class="hint">鼠标:双击行详情 · 右键隐藏 · 滚轮/拖拽滚动 · 点击折叠行展开 · 表头右缘拖动调列宽</p>
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
  font-size: var(--font-ui-lg, 13px);
  color: var(--dim, #8b98a9);
  margin-bottom: 8px;
}
.keys {
  border-collapse: collapse;
  font-size: var(--font-ui-base, 12px);
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
  font-size: var(--font-ui-sm, 11px);
  color: var(--dim, #5d7292);
}
</style>
