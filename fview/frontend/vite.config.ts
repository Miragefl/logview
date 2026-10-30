/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// Vue 工程配置（vitest 走 jsdom：组件渲染测试需要 DOM）
export default defineConfig({
  plugins: [vue()],
  test: {
    environment: 'jsdom',
  },
})
