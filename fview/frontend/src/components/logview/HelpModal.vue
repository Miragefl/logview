<script setup lang="ts">
// 键位速查弹窗(? 键):内容与 lib/keymap.ts 对齐;任意键/点击遮罩关闭。
// r2 分组双列改版:键位增长单列溢出,按域六组+组内两列网格,一屏全览不滚动。
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

// GROUPS 按域六组:移动/搜索与过滤/级别与显示/行操作与选择/源面板/全局
const GROUPS: { title: string; rows: [string, string][] }[] = [
  {
    title: '移动',
    rows: [
      ['j / k', '光标下移 / 上移一行'],
      ['h / l', '字符光标左移 / 右移(消息域)'],
      ['w / b / e', '词前进 / 词后退 / 词尾'],
      ['0 / ^ / $', '消息首 / 首个非空白 / 消息尾'],
      ['g / G', '回顶(关跟随) / 到底(开跟随)'],
      ['ctrl+d / ctrl+u', '下移 / 上移半页'],
      ['ctrl+f / ctrl+b', '下翻 / 上翻整页(PgDn/PgUp)'],
      ['H / M / L', '光标跳可视区顶 / 中 / 底'],
      ['zt / zz / zb', '光标行置顶 / 居中 / 置底'],
    ],
  },
  {
    title: '搜索与过滤',
    rows: [
      ['/ 或 f', '聚焦搜索条(回车执行后失焦,Esc 失焦不清词)'],
      ['n / N', '下一个 / 上一个命中(无搜索时高亮词)'],
      ['+ / -', '上下文混入:数字+Enter 生效(空=5 行),--/++/Esc 清除'],
      ['ctrl+h, ctrl+x', '高亮 / 隐藏词管理框(⌃j/k 选中,⌃d 删词)'],
    ],
  },
  {
    title: '级别与显示',
    rows: [
      ['E / W / I / D / A', '级别档位:ERROR / ≥WARN / ≥INFO / DEBUG / 全部'],
      ['F', '字段显隐面板(⌃j/k 移动,Enter/Space 勾选,Esc 关)'],
      ['#', '行号开关'],
      ['ctrl+w', '换行开关(仅折 message 列)'],
    ],
  },
  {
    title: '行操作与选择',
    rows: [
      ['ctrl+e', '折叠 / 展开光标处堆栈组'],
      ['d', '光标行详情(同双击)'],
      ['v / V', '字符级 / 行级可视选择(移动键扩选)'],
      ['yy', '复制当前行原文'],
      ['选择中 y', '复制选区(字符=所见文本,行=原文)并退出'],
      ['鼠标拖选', '字符级拖选,松手即复制'],
      ['⌘C / Ctrl+C', '复制选区(无选区不拦截系统复制)'],
      ["m / '", '光标行加书签 / 书签列表(⌃j/k 选中,Enter 跳转)'],
      ['C', '清屏(缓冲全清)'],
    ],
  },
  {
    title: '源面板',
    rows: [
      ['q', '源面板循环(本地→SSH→K8s→FRP→收起)'],
      ['直接打字', '过滤面板列表(子序列:hardware 输 hde 亦命中;退格删词,命中字符高亮)'],
      ['⌃j/k → Enter', '选中 + 打开/进入'],
      ['Esc', '逐级退出:清过滤词 → 关面板'],
      ['⌃a', '新增(Local 文件框/SSH 弹窗/FRP 表单)'],
      ['⌃d / ⌃e', '删除 / 编辑(SSH 主机、FRP 连接)'],
      ['K8s ⌃c / ⌃n', '切 context / namespace(⌃c 面板内复制让位)'],
      ['K8s Space 多选', '标记多资源,Enter 合流开一 tab(行首=pod 名),⌘Enter 各开'],
    ],
  },
  {
    title: '全局',
    rows: [
      [', / cmd+,', '设置中心'],
      ['ctrl+1..9 / cmd+1..9', '切换 tab'],
      ['ctrl+w / ⌘w', '关闭 tab(mac 原生菜单)'],
      ['ctrl+= / ctrl+-', '字号放大 / 缩小(cmd+= / cmd+- 同)'],
      ['?', '本帮助'],
    ],
  },
]
</script>

<template>
  <Teleport to="body">
    <div v-if="show" class="overlay" @click.self="close()">
      <div class="dialog">
        <div class="title">键位速查</div>
        <div v-for="g in GROUPS" :key="g.title" class="grp">
          <div class="grp-title">{{ g.title }}</div>
          <div class="grp-grid">
            <div v-for="[k, desc] in g.rows" :key="k" class="row">
              <span class="k">{{ k }}</span><span class="d">{{ desc }}</span>
            </div>
          </div>
        </div>
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
  /* r2 分组双列:键位 27+ 条单列 520px 溢出,加宽 760px 一屏全览 */
  width: 760px;
  max-width: 92vw;
  max-height: 88vh;
  overflow-y: auto;
  background: var(--popup-bg, #16202e);
  border: 1px solid var(--popup-border, #2a3a4f);
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
  padding: 12px 16px 14px;
}
.title {
  font-size: var(--font-ui-lg, 13px);
  color: var(--dim, #8b98a9);
  margin-bottom: 6px;
}
.grp-title {
  font-size: var(--font-ui-sm, 11px);
  color: var(--dim, #5d7292);
  margin: 8px 0 3px;
  border-bottom: 1px solid var(--popup-border, #2a3a4f);
  padding-bottom: 2px;
}
.grp-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  column-gap: 18px;
}
.row {
  display: flex;
  align-items: baseline;
  gap: 8px;
  padding: 2px 0;
  font-size: var(--font-ui-base, 12px);
}
.k {
  color: var(--accent, #7dc4e4);
  white-space: pre;
  font-family: var(--font-mono, monospace);
  flex: 0 0 auto;
  min-width: 96px;
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
