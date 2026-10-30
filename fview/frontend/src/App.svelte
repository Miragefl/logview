<script lang="ts">
  // M1 骨架占位：tab 容器与日志视图在后续任务接入，此处不 import 任何 wailsjs 绑定
  //（App 后端当前零方法，绑定文件不生成）。
</script>

<main>
  <div class="hero">
    <h1>fview</h1>
    <p>LogView 桌面版 · 骨架已就绪（tab / 日志视图将在后续任务接入）</p>
  </div>
</main>

<style>
  main {
    width: 100%;
    height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    background: #1b2636;
    color: #e6e6e6;
    font-family: -apple-system, 'PingFang SC', sans-serif;
  }
  .hero { text-align: center; }
  h1 { font-size: 42px; margin: 0 0 12px; letter-spacing: 2px; }
  p { opacity: 0.65; margin: 0; }
</style>
