<script setup lang="ts">
// fview 主壳（Task 7 重写为完整布局；当前空壳验证脚手架链路）
import { darkTheme } from 'naive-ui'
</script>

<template>
  <n-config-provider :theme="darkTheme">
    <div class="placeholder">fview · Vue3 + Naive UI</div>
  </n-config-provider>
</template>

<style scoped>
.placeholder {
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  user-select: none;
}
</style>
