<template>
  <div>
    <div class="page-head">
      <h1>主题中心</h1>
      <div class="toolbar">
        <span class="muted small">{{ THEME_PRESETS.length }} 套行业主题 · 切换即时生效</span>
      </div>
    </div>

    <div class="card">
      <div class="card-title">当前主题</div>
      <div class="tc-current">
        <div class="tc-cur-name">{{ cur?.name }} <span class="tag" v-if="cur?.dark">深色</span></div>
        <div class="tc-cur-desc muted">{{ cur?.desc }} · 适用场景：{{ cur?.scene }}</div>
        <div class="tc-palette">
          <span v-for="(c, i) in settings.chartPalette" :key="i" class="swatch" :style="{ background: c }" :title="c"></span>
          <span class="muted small" style="margin-left:8px">ECharts 图表色板（随主题联动）</span>
        </div>
      </div>
    </div>

    <div class="card" style="margin-top:16px">
      <div class="card-title">主题皮肤库</div>
      <p class="muted" style="margin-bottom:14px">点击卡片立即切换；偏好保存在本机浏览器，登录前自动应用管理员设置的站点默认主题。</p>
      <div class="theme-grid">
        <div v-for="t in THEME_PRESETS" :key="t.key"
             class="theme-card" :class="{ active: settings.theme === t.key }"
             @click="settings.updateLocal({ theme: t.key })">
          <div class="theme-preview" :data-theme-preview="t.key" :class="{ dark: t.dark }">
            <span class="tp-bar"></span><span class="tp-side"></span><span class="tp-main"></span>
            <span v-if="settings.theme === t.key" class="tp-check"><SvgIcon name="check" /></span>
          </div>
          <div class="theme-name">{{ t.name }}<span v-if="t.key === defaultTheme" class="tag site-tag">站点默认</span></div>
          <div class="theme-desc">{{ t.desc }}</div>
          <div class="theme-scene muted">{{ t.scene }}</div>
          <div class="tc-mini-palette">
            <span v-for="(c, i) in t.palette.slice(0, 6)" :key="i" class="swatch sm" :style="{ background: c }"></span>
          </div>
        </div>
      </div>
    </div>

    <div class="card" style="margin-top:16px" v-if="isAdmin">
      <div class="card-title">站点默认主题</div>
      <p class="muted" style="margin-bottom:12px">新用户（无本机偏好）首次访问时自动应用该主题，用于客户现场统一演示风格。</p>
      <div class="tc-default-row">
        <select class="input" style="width:260px" v-model="defaultThemeSel">
          <option v-for="t in THEME_PRESETS" :key="t.key" :value="t.key">{{ t.name }}</option>
        </select>
        <button class="btn btn-primary" @click="saveDefault">保存为站点默认</button>
        <span v-if="saveMsg" class="ok">{{ saveMsg }}</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useSettingsStore, THEME_PRESETS } from '../stores/settings'
import { useAuthStore } from '../stores/auth'
import api from '../api'

const settings = useSettingsStore()
const auth = useAuthStore()
const defaultTheme = ref('')
const defaultThemeSel = ref('light-tech')
const saveMsg = ref('')

const cur = computed(() => THEME_PRESETS.find(t => t.key === settings.theme))
const isAdmin = computed(() => auth.isAdmin)

async function loadDefault() {
  try {
    const r = await api.get('/public-settings')
    defaultTheme.value = r.data?.default_theme || ''
    defaultThemeSel.value = defaultTheme.value || 'light-tech'
  } catch { /* 忽略 */ }
}

async function saveDefault() {
  saveMsg.value = ''
  await api.put('/system-setting', { key: 'default_theme', value: defaultThemeSel.value })
  defaultTheme.value = defaultThemeSel.value
  saveMsg.value = '已保存'
  setTimeout(() => saveMsg.value = '', 2000)
}

onMounted(loadDefault)
</script>

<style scoped>
.muted { color: var(--muted); }
.small { font-size: 12px; }
.tc-current { display: flex; flex-direction: column; gap: 8px; }
.tc-cur-name { font-size: 17px; font-weight: 700; display: flex; align-items: center; gap: 8px; }
.tc-cur-desc { font-size: 13px; }
.tc-palette { display: flex; align-items: center; gap: 6px; margin-top: 4px; }
.swatch { width: 26px; height: 18px; border-radius: 4px; display: inline-block; border: 1px solid rgba(0,0,0,.1); }
.swatch.sm { width: 16px; height: 10px; border-radius: 2px; }
.tc-mini-palette { display: flex; gap: 3px; margin-top: 8px; }
.theme-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 14px; }
.theme-card { border: 1px solid var(--border); border-radius: 10px; padding: 12px; cursor: pointer; transition: all .2s; background: var(--card); }
.theme-card:hover { border-color: var(--accent); transform: translateY(-2px); }
.theme-card.active { border-color: var(--accent); box-shadow: 0 0 0 2px var(--accent-glow); }
.theme-preview { position: relative; height: 72px; border-radius: 6px; overflow: hidden; margin-bottom: 10px; background: linear-gradient(135deg, #eef4ff, #dbe7ff); }
.theme-preview.dark { background: linear-gradient(135deg, #101828, #1a2438); }
.theme-preview[data-theme-preview="emerald-green"] { background: linear-gradient(135deg, #ecfdf5, #d1fae5); }
.theme-preview[data-theme-preview="royal-purple"] { background: linear-gradient(135deg, #f3efff, #e4d9ff); }
.theme-preview[data-theme-preview="police-blue"] { background: linear-gradient(135deg, #0A1B33, #12294E); }
.theme-preview[data-theme-preview="tech-gold"] { background: linear-gradient(135deg, #14100A, #2B2315); }
.theme-preview[data-theme-preview="midnight-dark"] { background: linear-gradient(135deg, #0d1424, #16203a); }
.theme-preview[data-theme-preview="obsidian-black"] { background: linear-gradient(135deg, #0a0a0c, #1c1a14); }
.theme-preview[data-theme-preview="nebula-purple"] { background: linear-gradient(135deg, #1a1030, #2d1b4e); }
.theme-preview[data-theme-preview="klein-blue"] { background: linear-gradient(135deg, #f0f3fd, #dce4fa); }
.theme-preview[data-theme-preview="klein-blue-dark"] { background: linear-gradient(135deg, #070D24, #15224E); }
.tp-bar { position: absolute; top: 0; left: 0; right: 0; height: 14px; background: rgba(59,130,246,.75); }
.theme-preview.dark .tp-bar { background: rgba(96,165,250,.5); }
.theme-preview[data-theme-preview="police-blue"] .tp-bar { background: rgba(79,195,247,.6); }
.theme-preview[data-theme-preview="tech-gold"] .tp-bar { background: rgba(212,175,55,.65); }
.theme-preview[data-theme-preview="klein-blue"] .tp-bar { background: rgba(2,42,153,.8); }
.theme-preview[data-theme-preview="klein-blue-dark"] .tp-bar { background: rgba(76,125,255,.7); }
.tp-side { position: absolute; top: 14px; left: 0; bottom: 0; width: 26%; background: rgba(59,130,246,.28); }
.theme-preview.dark .tp-side { background: rgba(96,165,250,.18); }
.theme-preview[data-theme-preview="police-blue"] .tp-side { background: rgba(79,195,247,.2); }
.theme-preview[data-theme-preview="tech-gold"] .tp-side { background: rgba(212,175,55,.22); }
.theme-preview[data-theme-preview="klein-blue"] .tp-side { background: rgba(2,42,153,.25); }
.theme-preview[data-theme-preview="klein-blue-dark"] .tp-side { background: rgba(76,125,255,.25); }
.tp-main { position: absolute; top: 22px; left: 32%; width: 56%; height: 16px; border-radius: 3px; background: rgba(59,130,246,.5); }
.theme-preview.dark .tp-main { background: rgba(96,165,250,.3); }
.theme-preview[data-theme-preview="police-blue"] .tp-main { background: rgba(79,195,247,.35); }
.theme-preview[data-theme-preview="tech-gold"] .tp-main { background: rgba(212,175,55,.4); }
.theme-preview[data-theme-preview="klein-blue"] .tp-main { background: rgba(2,42,153,.45); }
.theme-preview[data-theme-preview="klein-blue-dark"] .tp-main { background: rgba(76,125,255,.4); }
.tp-check { position: absolute; right: 6px; top: 4px; width: 18px; height: 18px; border-radius: 50%; background: var(--accent); color: #fff; font-size: 11px; display: flex; align-items: center; justify-content: center; }
.theme-name { font-size: 13px; font-weight: 600; display: flex; align-items: center; gap: 6px; }
.theme-desc { font-size: 11px; color: var(--muted); margin-top: 2px; }
.theme-scene { font-size: 11px; margin-top: 3px; }
.tag { font-size: 10px; border: 1px solid var(--accent); color: var(--accent); border-radius: 4px; padding: 1px 5px; font-weight: 400; }
.site-tag { background: var(--accent-glow); }
.tc-default-row { display: flex; align-items: center; gap: 12px; }
.ok { color: #00915b; font-size: 13px; }
</style>
