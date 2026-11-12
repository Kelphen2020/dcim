import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import SvgIcon from './components/SvgIcon.vue'
import './style.css'

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.use(router)
app.component('SvgIcon', SvgIcon)
app.mount('#app')

// 启动时加载本地设置（主题）
import('./stores/settings').then(({ useSettingsStore }) => {
  const settings = useSettingsStore(pinia)
  settings.load()
})
