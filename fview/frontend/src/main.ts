import { createApp } from 'vue'
import { createPinia } from 'pinia'
import naive from 'naive-ui'
import App from './App.vue'
import './style.css'

// fview 前端入口：pinia 状态 + naive-ui 全量注册（桌面应用不做按需）
createApp(App).use(createPinia()).use(naive).mount('#app')
