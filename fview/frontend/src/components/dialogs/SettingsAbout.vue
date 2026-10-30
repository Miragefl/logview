<script setup lang="ts">
// 关于页：版本信息（桌面端与 TUI 同仓双产物）。
</script>

<template>
  <div class="about">
    <h3>fview</h3>
    <p>LogView 桌面版 · 与 TUI 共享 logcore（stream/parser/model/buffer/frp）</p>
    <p class="dim">本地文件 / SSH / K8s / FRP 多日志源 · 标签页 · 虚拟滚动</p>
  </div>
</template>

<style scoped>
.about {
  padding: 20px 10px;
  text-align: center;
}
h3 {
  margin: 0 0 8px;
  letter-spacing: 2px;
}
p {
  margin: 4px 0;
  font-size: 13px;
}
.dim {
  color: var(--dim, #5d7292);
  font-size: 12px;
}
</style>
