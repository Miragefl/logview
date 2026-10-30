<script setup lang="ts">
// 高亮词管理浮层：输入关键词回车加入，10 色轮换；列表可删。
import { ref } from 'vue'
import { NInput } from 'naive-ui'
import { useAnnotationsStore } from '../../stores/annotations'

const ann = useAnnotationsStore()
const word = ref('')

function add() {
  const w = word.value.trim()
  if (!w) return
  ann.addHighlight(w)
  word.value = ''
}
</script>

<template>
  <div class="hl-menu">
    <div class="add-row">
      <NInput
        v-model:value="word"
        size="tiny"
        placeholder="关键词回车加入（10 色轮换）"
        @keyup.enter="add"
      />
    </div>
    <ul v-if="ann.highlights.length" class="list">
      <li v-for="h in ann.highlights" :key="h.word">
        <span class="dot" :style="{ background: h.color }"></span>
        <span class="w hl-w" :style="{ color: h.color }">{{ h.word }}</span>
        <span class="del" title="移除" @click="ann.removeHighlight(h.word)">&times;</span>
      </li>
    </ul>
    <p v-else class="empty">暂无高亮词——命中的词以亮色粗体下划线展示</p>
  </div>
</template>

<style scoped>
.hl-menu {
  width: 220px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.list {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 200px;
  overflow-y: auto;
  font-size: 12px;
}
.list li {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 3px 2px;
}
.dot {
  width: 10px;
  height: 10px;
  border-radius: 3px;
  flex: none;
}
.w {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 700;
  text-decoration: underline;
}
.del {
  cursor: pointer;
  color: var(--dim, #8b98a9);
  padding: 0 4px;
}
.del:hover {
  color: var(--fg, #fff);
}
.empty {
  color: var(--dim, #8b98a9);
  font-size: 11px;
  margin: 0;
}
</style>
