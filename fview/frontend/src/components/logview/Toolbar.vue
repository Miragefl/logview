<script setup lang="ts">
// 日志工具栏:字段显隐(单日志覆盖层,F 键)/Follow/字号(搜索/级别/高亮隐藏入口已迁 SearchBar)。
// 字段勾选只 emit 覆盖给 App 落 localStorage——rules.yaml 默认层在设置中心管,两层互不沾。
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { NButton, NDropdown } from 'naive-ui'
import { useTabsStore } from '../../stores/tabs'
import { useAnnotationsStore } from '../../stores/annotations'
import { useUiStore } from '../../stores/ui'
import { useFieldsStore } from '../../stores/fields'
import { setFollow } from '../../lib/backend'

const emit = defineEmits<{
  (e: 'override-fields', v: string[] | null): void
}>()

const props = defineProps<{ configured: string[] }>()

const tabs = useTabsStore()
const ann = useAnnotationsStore()
const ui = useUiStore()
const fieldsStore = useFieldsStore()

/** 字段下拉受控显隐(F 键入口;manual trigger 勾选后不关,支持连续勾,第四轮验收)。 */
const fieldsShow = ref(false)
function toggleFields() {
  fieldsShow.value = !fieldsShow.value
}
defineExpose({ toggleFields })

/** onFieldMenuKey 下拉打开时的键盘流(2026-11-08 r4~r6 三轮):
 *  - ⌃j/⌃k 中继方向键:naive dropdown 键盘导航(vooks useKeyboard)经 evtd 委托、
 *    只认方向键——evtd 监听挂 **window**、按 target→window 传播链匹配 document,
 *    合成事件必须 bubbles:true 且派发到焦点元素(activeElement)复刻原生按键路径
 *    (r4 派发 document 本体无 bubbles,evtd 匹配不到,jsdom spy 错层假绿)。
 *  - Space 勾选:naive 键盘表无 Space(r5 想当然踩坑),中继为 Enter;勾选后 naive
 *    的 doUpdateShow(false) 因 manual 受控无人接——歪打正着保住"连续勾选不关"。
 *  - Esc 关闭:naive handleKeydownEsc 只 emit update:show,manual 受控没接回调
 *    (r5 踩坑),自己关;故意不接 @update:show,否则 Enter 勾选后也会关。
 *  capture 拦截后 stopPropagation,同时掐断 App.vue 全局 vim 键
 *  (ctrl+j/k 原映射 navDown/navUp)。treemate 移动 pending 自动跳过 divider/禁用项。 */
function onFieldMenuKey(e: KeyboardEvent) {
  if (!fieldsShow.value) return
  const inInput = (() => {
    const t = e.target as HTMLElement | null
    return !!t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA')
  })()
  if ((e.ctrlKey || e.metaKey) && (e.key === 'j' || e.key === 'k')) {
    e.preventDefault()
    e.stopPropagation()
    synthKey(e.key === 'j' ? 'ArrowDown' : 'ArrowUp')
  } else if (e.key === ' ' && !inInput) {
    e.preventDefault() // 兼防按钮 focus 时 Space 原生触发 click(会把刚关的下拉重开)
    e.stopPropagation()
    synthKey('Enter')
  } else if (e.key === 'Escape') {
    e.stopPropagation()
    fieldsShow.value = false
  }
}

/** synthKey 合成按键派发到焦点元素(与原生按键传播路径一致,evtd 才能匹配)。 */
function synthKey(key: string) {
  const target = document.activeElement ?? document.body
  target.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }))
}
// 开关注册:弹窗栈登记(全局 vim 键让位,同 WordsPopup;合成方向键亦不穿透)+监听挂卸
watch(fieldsShow, (v) => {
  if (v) {
    ui.dialogOpen('fields')
    document.addEventListener('keydown', onFieldMenuKey, true)
  } else {
    ui.dialogClose('fields')
    document.removeEventListener('keydown', onFieldMenuKey, true)
  }
})
onBeforeUnmount(() => {
  if (fieldsShow.value) ui.dialogClose('fields')
  document.removeEventListener('keydown', onFieldMenuKey, true)
})

/**
 * fieldsOptions 字段下拉项:
 * - 勾选态 = 当前 tab 配置(props.configured,未裁剪)
 * - 解析器不产出的字段置灰标「未解析」(列按能力显隐,勾了也无处显示)
 * - message 恒显勾死(没内容列的日志查看器没有意义)
 */
const fieldsOptions = computed(() => {
  const cap = tabs.active?.parserFields ?? null
  const items = fieldsStore.all.map((f) => {
    const capable = cap === null || cap.includes(f.name)
    return {
      key: f.name,
      label: `${props.configured.includes(f.name) ? '☑' : '☐'} ${f.name}${capable ? '' : '(未解析)'}`,
      disabled: f.name === 'message' || !capable,
    }
  })
  return [...items, { type: 'divider' }, { label: '恢复默认', key: '__reset__' }]
})

/** onSelect 勾选/恢复默认:全部 emit 给 App 写覆盖层,不碰 rules.yaml(严禁与 hides 混写)。 */
function onSelect(key: string) {
  if (key === '__reset__') {
    emit('override-fields', null)
    return
  }
  if (key === 'message') return
  const on = props.configured.includes(key)
  const next = on ? props.configured.filter((f) => f !== key) : [...props.configured, key]
  // 保序:按 fieldsStore.all 配置序重排(覆盖值与默认层同序)
  emit('override-fields', fieldsStore.all.filter((f) => next.includes(f.name)).map((f) => f.name))
}

async function onFollow(follow: boolean) {
  const t = tabs.active
  if (!t) return
  tabs.toggleFollow(t.info.id, follow)
  try {
    await setFollow(t.info.id, follow)
  } catch {
    /* 后端失败保底本地态 */
  }
}

function zoom(delta: number) {
  ui.fontSize = Math.min(28, Math.max(9, ui.fontSize + delta))
  ui.persist()
}
</script>

<template>
  <div class="toolbar">
    <!-- 源信息条(验收反馈⑥):本地=路径/SSH=user@host:path/FRP=连接→服务器/K8s=ctx·ns/资源 -->
    <span v-if="tabs.active?.info.info" class="src-info" :title="tabs.active.info.info">{{
      tabs.active.info.info
    }}</span>
    <NDropdown
      trigger="manual"
      placement="bottom-start"
      :show="fieldsShow"
      :options="fieldsOptions"
      @clickoutside="fieldsShow = false"
      @select="onSelect"
    >
      <NButton size="tiny" quaternary @click="toggleFields">
        <!-- 三列分隔:字段显隐意象(描边 SVG,与 SidebarRail 同风格——emoji 弃用,验收反馈⑦) -->
        <svg viewBox="0 0 24 24" class="ico">
          <rect x="4" y="5" width="16" height="14" rx="1.5" />
          <path d="M9.3 5v14" />
          <path d="M14.6 5v14" />
        </svg>
        字段
      </NButton>
    </NDropdown>
    <NButton
      size="tiny"
      :quaternary="!tabs.active?.info.follow"
      :type="tabs.active?.info.follow ? 'success' : 'default'"
      @click="onFollow(!tabs.active?.info.follow)"
    >
      {{ tabs.active?.info.follow ? '● Follow' : 'Follow' }}
    </NButton>
    <div class="spacer"></div>
    <NButton size="tiny" quaternary @click="zoom(-1)">A-</NButton>
    <span class="fs">{{ ui.fontSize }}</span>
    <NButton size="tiny" quaternary @click="zoom(1)">A+</NButton>
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  border-bottom: 1px solid var(--popup-border, #1e2837);
}
/* 源信息条:dim 色,最大 40% 宽不挤搜索按钮 */
.src-info {
  max-width: 40%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--font-ui-sm, 11px);
  color: var(--dim, #5d7292);
  font-family: var(--font-mono, monospace);
}
.spacer {
  flex: 1;
}
.fs {
  font-size: var(--font-ui-sm, 11px);
  color: var(--dim, #8b98a9);
  min-width: 18px;
  text-align: center;
}
/* 描边图标:与 SidebarRail 同款(22px stroke currentColor) */
/* 图标基类收编 style.css 全局正本(2026-11-03 统一),此处仅留对齐布局附加 */
.ico {
  vertical-align: -3px;
  margin-right: 2px;
}
</style>
