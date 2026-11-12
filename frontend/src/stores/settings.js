import { defineStore } from 'pinia'

// 主题预设：light-tech（默认）为 :root 内置，无需额外 CSS
// V4：新增公安蓝（政务）/ 科技金（高端演示）；V35：新增克莱因蓝（#022A99），共 9 套；每套含 ECharts 图表色板
export const THEME_PRESETS = [
  { key: 'light-tech',     name: '极客蓝（默认）', desc: '浅色科技风 · 蓝色渐变',       dark: false, scene: '通用办公',
    palette: ['#1677ff', '#00b96b', '#fa8c16', '#722ed1', '#eb2f96', '#13c2c2'] },
  { key: 'emerald-green',  name: '翡翠绿',       desc: '清新绿调 · 自然护眼',         dark: false, scene: '通用办公',
    palette: ['#00A670', '#2ED3A0', '#1677ff', '#fa8c16', '#722ed1', '#eb2f96'] },
  { key: 'royal-purple',   name: '典雅紫',       desc: '紫色渐变 · 稳重典雅',         dark: false, scene: '通用办公',
    palette: ['#7C3AED', '#B794F6', '#1677ff', '#00b96b', '#fa8c16', '#eb2f96'] },
  { key: 'police-blue',    name: '公安蓝',       desc: '庄重深蓝 · 政务行业定制',      dark: true,  scene: '政务客户',
    palette: ['#4FC3F7', '#1D6FE0', '#00E676', '#FFC53D', '#FF7875', '#B37FEB'] },
  { key: 'tech-gold',      name: '科技金',       desc: '深金黑 · 高端演示定制',        dark: true,  scene: '高端演示',
    palette: ['#F0D060', '#D4AF37', '#FF8C42', '#5EEAD4', '#93C5FD', '#F9A8D4'] },
  { key: 'midnight-dark',  name: '暗夜蓝',       desc: '深色模式 · 夜间护眼',         dark: true,  scene: '监控大屏',
    palette: ['#409eff', '#67c23a', '#e6a23c', '#f56c6c', '#9d7bff', '#00d4ff'] },
  { key: 'obsidian-black', name: '曜石黑',       desc: '深色模式 · 深空碳黑 · 琥珀金', dark: true,  scene: '监控大屏',
    palette: ['#F5A623', '#FFC861', '#4D8DFF', '#00E676', '#FF7875', '#B37FEB'] },
  { key: 'nebula-purple',  name: '暮紫星河',     desc: '深色模式 · 星空紫调 · 优雅',   dark: true,  scene: '监控大屏',
    palette: ['#9D7BFF', '#E879F9', '#4FC3F7', '#00E676', '#FFC53D', '#FF7875'] },
  { key: 'klein-blue',     name: '克莱因蓝',     desc: '国际克莱因蓝 · #022A99 · 深邃纯粹', dark: false, scene: '品牌定制',
    palette: ['#022A99', '#4A6FE3', '#00B4A0', '#F5A623', '#E0457B', '#7C5CFF'] },
  { key: 'klein-blue-dark', name: '克莱因蓝·夜', desc: '深色模式 · 克莱因蓝底 · 电光蓝点缀', dark: true, scene: '监控大屏',
    palette: ['#4C7DFF', '#8AA8FF', '#00D9A3', '#FFC53D', '#FF7875', '#B37FEB'] }
]

const DEFAULT_SETTINGS = {
  site_title: 'DCIM 数据中心基础设施管理',
  site_logo: '',
  theme: 'light-tech'
}

const LS_KEY = 'dcim_settings'

export const useSettingsStore = defineStore('settings', {
  state: () => ({
    site_title: DEFAULT_SETTINGS.site_title,
    site_logo: DEFAULT_SETTINGS.site_logo,
    theme: DEFAULT_SETTINGS.theme,
    loaded: false
  }),
  getters: {
    // 当前主题的 ECharts 色板（随主题切换响应式更新）
    chartPalette(state) {
      const preset = THEME_PRESETS.find(t => t.key === state.theme) || THEME_PRESETS[0]
      return preset.palette
    }
  },
  actions: {
    applyTheme(theme) {
      const preset = THEME_PRESETS.find(t => t.key === theme) || THEME_PRESETS[0]
      if (preset.key === 'light-tech') {
        document.documentElement.removeAttribute('data-theme')
      } else {
        document.documentElement.setAttribute('data-theme', preset.key)
      }
      // data-dark 标记：供全局深色覆盖样式使用（深色主题共用一套修正规则）
      if (preset.dark) {
        document.documentElement.setAttribute('data-dark', '1')
      } else {
        document.documentElement.removeAttribute('data-dark')
      }
      // 图表重渲信号：趋势/大屏页 watch 此值重绘 ECharts 色板
      document.dispatchEvent(new CustomEvent('dcim-theme-change', { detail: { theme: preset.key } }))
    },
    applyAll() {
      this.applyTheme(this.theme)
      document.title = this.site_title
    },
    // P0：设置保存在浏览器本地；V4：登录前若无本地偏好，拉取站点默认主题
    async load() {
      try {
        const raw = localStorage.getItem(LS_KEY)
        if (raw) {
          const s = JSON.parse(raw)
          this.site_title = s.site_title || DEFAULT_SETTINGS.site_title
          this.site_logo = s.site_logo || ''
          this.theme = s.theme || DEFAULT_SETTINGS.theme
        } else {
          // 无本地偏好 → 应用管理员设置的站点默认主题
          try {
            const res = await fetch('/api/v1/public-settings')
            const j = await res.json()
            const dt = j?.data?.default_theme
            if (dt && THEME_PRESETS.some(t => t.key === dt)) this.theme = dt
          } catch { /* 后端未就绪时静默回退默认 */ }
        }
      } catch (e) { /* 忽略损坏的本地数据 */ }
      this.loaded = true
      this.applyAll()
    },
    updateLocal(patch) {
      Object.assign(this, patch)
      this.applyAll()
      localStorage.setItem(LS_KEY, JSON.stringify({
        site_title: this.site_title, site_logo: this.site_logo, theme: this.theme
      }))
    }
  }
})
