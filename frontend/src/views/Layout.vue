<template>
  <svg style="display:none" aria-hidden="true">
    <symbol id="i-chart" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <path d="M3 3v18h18"/><path d="M7 15l4-5 3 3 5-7"/>
    </symbol>
    <symbol id="i-layers" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <path d="M12 3l9 5-9 5-9-5 9-5z"/><path d="M3 13l9 5 9-5"/><path d="M3 17l9 5 9-5"/>
    </symbol>
    <symbol id="i-gateway" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <rect x="3" y="9" width="18" height="6" rx="1.5"/><circle cx="7" cy="12" r="1"/><circle cx="11" cy="12" r="1"/>
      <path d="M16 12h2M12 3v3M12 18v3M6 15v3M18 15v3"/>
    </symbol>
    <symbol id="i-box" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <path d="M21 8l-9-5-9 5v8l9 5 9-5V8z"/><path d="M3 8l9 5 9-5M12 13v8"/>
    </symbol>
    <symbol id="i-trend" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <path d="M3 17l5-5 4 3 7-8"/><path d="M19 7h-4m4 0v4"/><path d="M3 21h18"/>
    </symbol>
    <symbol id="i-users" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <circle cx="9" cy="8" r="3.5"/><path d="M3 20c0-3.3 2.7-6 6-6s6 2.7 6 6"/><path d="M16 4.5a3.5 3.5 0 0 1 0 7M17.5 14.2c2 .8 3.5 2.9 3.5 5.3"/>
    </symbol>
    <symbol id="i-doc" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8l-5-5z"/>
      <path d="M14 3v5h5M9 13h6M9 17h4"/>
    </symbol>
    <symbol id="i-asset" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <rect x="4" y="4" width="16" height="7" rx="1.5"/><rect x="4" y="13" width="16" height="7" rx="1.5"/>
      <path d="M8 7.5h.01M8 16.5h.01M12 7.5h4M12 16.5h4"/>
    </symbol>
    <symbol id="i-capacity" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <path d="M12 3a9 9 0 0 1 9 9"/><path d="M12 21a9 9 0 0 1-9-9"/><circle cx="12" cy="12" r="1.5"/>
      <path d="M12 12l4.5-4.5"/>
    </symbol>
    <symbol id="i-gear" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <circle cx="12" cy="12" r="3.2"/>
      <path d="M12 2v3M12 19v3M2 12h3M19 12h3M4.9 4.9l2.1 2.1M17 17l2.1 2.1M19.1 4.9L17 7M7 17l-2.1 2.1"/>
    </symbol>
    <symbol id="i-order" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <rect x="5" y="4" width="14" height="17" rx="2"/><path d="M9 4V2.5M15 4V2.5M9 9h6M9 13h6M9 17h4"/>
    </symbol>
    <symbol id="i-inspect" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <circle cx="11" cy="11" r="7"/><path d="M20 20l-3.8-3.8"/><path d="M8.5 11l1.8 1.8 3.2-3.6"/>
    </symbol>
    <symbol id="i-change" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <path d="M4 7h11l-2.5-2.5M20 17H9l2.5 2.5"/><circle cx="19" cy="7" r="1.2"/><circle cx="5" cy="17" r="1.2"/>
    </symbol>
    <symbol id="i-power" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <path d="M13 2L5 13h5l-1 9 8-11h-5l1-9z"/>
    </symbol>
    <symbol id="i-bell" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <path d="M18 9a6 6 0 1 0-12 0c0 6-2.5 7-2.5 7h17S18 15 18 9z"/><path d="M10 19.5a2.2 2.2 0 0 0 4 0"/>
    </symbol>
    <symbol id="i-wrench" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <path d="M14.7 6.3a4.5 4.5 0 0 0-6.1 5.4L3 17.3a2 2 0 1 0 2.8 2.8l5.6-5.6a4.5 4.5 0 0 0 5.4-6.1l-3 3-2.1-.7-.7-2.1 3-3z"/>
    </symbol>
    <symbol id="i-file" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8l-5-5z"/><path d="M14 3v5h5"/>
    </symbol>
    <symbol id="i-code" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <path d="M8 9l-4 3 4 3M16 9l4 3-4 3M13 6l-2 12"/>
    </symbol>
    <symbol id="i-screen" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <rect x="3" y="4" width="18" height="13" rx="1.5"/><path d="M8 21h8M12 17v4"/>
    </symbol>
    <symbol id="i-cube" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <path d="M12 2l8 4.5v11L12 22l-8-4.5v-11L12 2z"/><path d="M12 11L4 6.5M12 11l8-4.5M12 11v11"/>
    </symbol>
  </svg>

  <div class="app-shell">
    <header class="topbar">
      <button class="menu-btn" @click="mobileOpen = !mobileOpen" aria-label="菜单"><SvgIcon name="menu" /></button>
      <div class="logo">
        <span class="logo-mark">◈</span>
        <span class="logo-text">{{ settings.site_title }}</span>
      </div>
      <div class="topbar-right">
        <span class="topbar-build" :title="'前端构建版本与时间（判断浏览器是否拿到最新版）'">{{ buildVer }}</span>
        <span class="topbar-phase" :title="'当前平台版本阶段：' + phase + '（P0骨架→P1资产→P2运维→P3动环→P4网络→P5大屏与高级能力），表示系统已建设到的功能阶段，仅作版本标识'">{{ phase }}</span>
        <button class="gs-open-btn" title="全局搜索（Ctrl/⌘ + K）：资产/测点/工单/告警/网络设备/备件" @click="searchRef && searchRef.openPal()">
          <SvgIcon name="search" /><span class="gs-open-label">搜索</span><span class="gs-open-kbd">Ctrl K</span>
        </button>
        <AIAssistant />
        <div class="bell-wrap" ref="bellWrap">
          <button class="bell-btn" @click="togglePanel" title="消息通知">
            <svg width="18" height="18"><use href="#i-bell"/></svg>
            <span v-if="unread > 0" class="bell-dot">{{ unread > 99 ? '99+' : unread }}</span>
          </button>
          <div v-if="panelOpen" class="bell-panel">
            <div class="bell-panel-head">
              <span>通知中心</span>
              <button v-if="unread > 0" class="btn btn-mini" @click="markAllRead">全部已读</button>
            </div>
            <div class="bell-list">
              <div v-for="n in notifs" :key="n.id" class="bell-item" :class="{ unread: !n.read }" @click="readOne(n)">
                <div class="bell-title">
                  <span class="badge" :class="'kind-' + n.kind">{{ KINDS[n.kind] || n.kind }}</span>
                  {{ n.title }}
                </div>
                <div class="bell-content">{{ n.content }}</div>
                <div class="muted bell-time">{{ fmt(n.created_at) }}</div>
              </div>
              <div v-if="!notifs.length" class="empty">暂无通知</div>
            </div>
          </div>
        </div>
        <div class="user-chip" @click="router.push('/user-center')" title="进入用户中心：个人资料、密码、主题设置" role="button" tabindex="0">
          <span class="user-avatar">{{ (auth.user?.display_name || auth.user?.username || '?').slice(0, 1) }}</span>
          <span class="user-name">{{ auth.user?.display_name || auth.user?.username }}</span>
          <span class="role-tag" :class="'role-' + auth.role">{{ roleLabel }}</span>
        </div>
        <button class="btn btn-ghost btn-sm" @click="logout">退出</button>
      </div>
    </header>
    <div class="app-body">
      <div v-if="mobileOpen" class="nav-backdrop" @click="mobileOpen=false"></div>
      <aside class="sidebar" :class="{ open: mobileOpen }">
        <nav>
          <template v-for="g in visibleMenu" :key="g.gid || g.key">
            <div class="nav-group">{{ g.label }}</div>
            <router-link v-for="m in g.items" :key="m.to" :to="m.to" @click="mobileOpen = false">
              <span class="nav-ico"><svg><use :href="'#' + m.ico"/></svg></span>{{ m.label }}
            </router-link>
          </template>
        </nav>
      </aside>
      <main class="main-content">
        <router-view />
      </main>
    </div>
    <GlobalSearch ref="searchRef" :pages="visibleMenu" />
  </div>
</template>

<script setup>
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useSettingsStore } from '../stores/settings'
import AIAssistant from '../components/AIAssistant.vue'
import GlobalSearch from '../components/GlobalSearch.vue'
import { BUILD_VERSION } from '../build-info'
const buildVer = BUILD_VERSION
import api from '../api'
import { toastAuto } from '../ui-dialog'

const router = useRouter()
const auth = useAuthStore()
const settings = useSettingsStore()
const searchRef = ref(null) // V76 全局搜索面板

// ==================== 数据驱动菜单（按功能分组 + 权限过滤） ====================
// group.key 与后端 allowed_groups 白名单一一对应
const MENU = [
  { key: 'visual', label: '可视化', items: [
    { to: '/big-screen', label: '可视化大屏', ico: 'i-screen' },
    { to: '/room-3d', label: '3D机房', ico: 'i-cube' }
  ] },
  { key: 'dashboard', label: '运行总览', items: [
    { to: '/dashboard', label: '运行总览', ico: 'i-chart' }
  ] },
  { key: 'infra', label: '基础设施', items: [
    { to: '/topology', label: '基础设施层级', ico: 'i-layers' },
    { to: '/capacity', label: '机柜容量', ico: 'i-capacity' },
    { to: '/power-chain', label: '供电链路', ico: 'i-power' },
    { to: '/power-sld', label: '供电单线图', ico: 'i-power' },
    { to: '/gateways', label: '站点网关', ico: 'i-gateway' }
  ] },
  { key: 'asset', label: '资产管理', items: [
    { to: '/assets', label: '资产台账', ico: 'i-asset' },
    { to: '/stocktake', label: '资产盘点', ico: 'i-inspect' },
    { to: '/asset-retirement', label: '资产退役处置', ico: 'i-box' },
    { to: '/maintenances', label: '维保管理', ico: 'i-wrench' }
  ] },
  // V76 B5-3: 运维组拆「作业执行 / 知识与预案」两组（权限 key 均保持 ops，路由与深链不变）
  { key: 'ops', gid: 'ops-exec', label: '作业执行', items: [
    { to: '/work-orders', label: '运维工单', ico: 'i-order' },
    { to: '/duty-roster', label: '值班与交接', ico: 'i-users' },
    { to: '/spare-parts', label: '备件台账', ico: 'i-box' },
    { to: '/inspection', label: '巡检管理', ico: 'i-inspect' },
    { to: '/inspection-reports', label: '巡检报告', ico: 'i-doc' },
    { to: '/changes', label: '变更管理', ico: 'i-change' }
  ] },
  { key: 'ops', gid: 'ops-knowledge', label: '知识与预案', items: [
    { to: '/trend-forecast', label: '趋势预测', ico: 'i-trend' },
    { to: '/fault-cases', label: '故障案例库', ico: 'i-doc' },
    { to: '/emergency-plans', label: '应急预案与演练', ico: 'i-doc' }
  ] },
  { key: 'monitor', label: '动环监控', items: [
    { to: '/monitor', label: '监控总览', ico: 'i-chart' },
    { to: '/monitoring-points', label: '测点管理', ico: 'i-gear' },
    { to: '/alert-rules', label: '告警规则', ico: 'i-bell' },
    { to: '/alert-wall', label: '告警盯屏', ico: 'i-bell' },
    { to: '/trend-analysis', label: '趋势分析', ico: 'i-chart' }
  ] },
  { key: 'energy', label: '能耗管理', items: [
    { to: '/energy', label: '能耗总览', ico: 'i-power' }
  ] },
  { key: 'network', label: '网络管理', items: [
    { to: '/network-topology', label: '网络拓扑', ico: 'i-layers' },
    { to: '/network-devices', label: '网络设备', ico: 'i-gateway' },
    { to: '/network-links', label: '链路台账', ico: 'i-layers' },
    { to: '/ip-management', label: 'IP管理', ico: 'i-code' },
    { to: '/vlan-management', label: 'VLAN管理', ico: 'i-layers' },
    { to: '/config-backups', label: '配置备份', ico: 'i-doc' }
  ] },
  { key: 'report', label: '报表与开放', items: [
    { to: '/reports', label: '报表中心', ico: 'i-file' },
    { to: '/api-docs', label: '开放API', ico: 'i-code' }
  ] },
  { key: 'system', label: '系统管理', items: [
    { to: '/theme-center', label: '主题中心', ico: 'i-gear' },
    { to: '/users', label: '用户与权限', ico: 'i-users', roles: ['global_admin'] },
    { to: '/audits', label: '审计日志', ico: 'i-doc' },
    { to: '/notify-channels', label: '通知通道', ico: 'i-bell', roles: ['global_admin', 'site_admin'] },
    { to: '/settings', label: '系统设置', ico: 'i-gear' }
  ] }
]

function groupAllowed(key) {
  if (auth.isGlobalAdmin) return true
  const ag = auth.user?.allowed_groups
  if (ag === undefined || ag === null || ag === '' || ag === '*') return true
  return String(ag).split(',').map(s => s.trim()).includes(key)
}

const visibleMenu = computed(() =>
  MENU
    .filter(g => groupAllowed(g.key))
    .map(g => ({ ...g, items: g.items.filter(m => !m.roles || m.roles.includes(auth.role)) }))
    .filter(g => g.items.length)
)

// 移动端导航
const mobileOpen = ref(false)
router.afterEach(() => { mobileOpen.value = false })

// ==================== 通知中心 ====================
const KINDS = { maintenance_due: '维保', work_order: '工单', change: '变更', inspection: '巡检', system: '系统', asset_retirement: '退役', drill_overdue: '演练', asset_health: '健康度', capacity_forecast: '容量', ops_report: '月报', inspection_missed: '漏检', alert_quality: '噪声', pue_over: '合规' }
const REF_ROUTES = { asset_retirement: '/asset-retirement', ops_report: '/reports?tab=monthly', inspection_task: '/inspection?tab=tasks', alert_rule: '/alert-rules?tab=quality', site: '/energy' }
const unread = ref(0)
const notifs = ref([])
const panelOpen = ref(false)
const bellWrap = ref(null)
const fmt = t => t ? String(t).replace('T', ' ').slice(0, 16) : '-'

// 点击面板外部任意位置关闭通知面板
function onDocClick(e) {
  if (panelOpen.value && bellWrap.value && !bellWrap.value.contains(e.target)) panelOpen.value = false
}
function onEsc(e) { if (e.key === 'Escape') panelOpen.value = false }

async function refreshUnread() {
  try {
    const r = await api.get('/notifications-unread-count')
    unread.value = r.data?.unread || 0
  } catch { /* ignore */ }
}
async function loadNotifs() {
  const r = await api.get('/notifications', { size: 20 })
  notifs.value = r.data?.items || []
}
function togglePanel() {
  panelOpen.value = !panelOpen.value
  if (panelOpen.value) loadNotifs()
}
async function readOne(n) {
  if (!n.read) {
    await api.post(`/notifications-read/${n.id}`, {})
    n.read = true
    refreshUnread()
  }
  // V60 可穿透通知：点击直达业务详情（深链双路径由目标页承接）
  const base = REF_ROUTES[n.ref_type]
  if (base && n.ref_id) {
    panelOpen.value = false
    const sep = base.includes('?') ? '&' : '?'
    router.push(`${base}${sep}id=${n.ref_id}&detail=1`)
  }
}
async function markAllRead() {
  await api.post('/notifications-read/0', {})
  notifs.value.forEach(n => { n.read = true })
  unread.value = 0
}
let unreadTimer = null

// 系统阶段徽章：从后端 /health 动态读取
const phase = ref('…')
async function loadPhase() {
  try {
    const res = await api.get('/health')
    phase.value = (res.data?.phase || '') + ' · ' + phaseName(res.data?.phase)
  } catch { phase.value = 'DCIM' }
}
onMounted(() => {
    // 30分钟无操作提示
    let idleTimer = null
    const SESSION_TIMEOUT = 30 * 60 * 1000
    function resetIdle() { auth.lastActivity = Date.now(); if (idleTimer) clearTimeout(idleTimer); idleTimer = setTimeout(checkSession, 60000) }
    function checkSession() { if (Date.now() - auth.lastActivity > SESSION_TIMEOUT) { auth.logout(); router.push('/login'); toastAuto('会话已超时，请重新登录') } else { resetIdle() } }
    window.addEventListener('mousemove', resetIdle); window.addEventListener('keydown', resetIdle); window.addEventListener('click', resetIdle)
    resetIdle()
    loadPhase()
    refreshUnread()
    unreadTimer = setInterval(refreshUnread, 60000)
    document.addEventListener('click', onDocClick, true)
    document.addEventListener('keydown', onEsc)
  })
onUnmounted(() => { if (unreadTimer) clearInterval(unreadTimer); document.removeEventListener('click', onDocClick, true); document.removeEventListener('keydown', onEsc) })
function phaseName(p) {
  return { P0: '系统骨架', P1: '资产管理', P2: '基础运维', P3: '动环监控', P4: '网络管理', P5: '大屏与高级能力' }[p] || '运行中'
}

const ROLE_LABELS = {
  global_admin: '全局管理员',
  site_admin: '站点管理员',
  operator: '值班员',
  viewer: '只读'
}
const roleLabel = computed(() => ROLE_LABELS[auth.role] || auth.role)

function logout() {
  auth.logout()
  router.push('/login')
}
</script>

<style scoped>
.app-body { display: flex; flex: 1; min-height: 0; }
.main-content { flex: 1; padding: 20px 24px; overflow-y: auto; }
.topbar-phase { font-size: 11px; color: var(--muted); border: 1px solid var(--border); border-radius: 20px; padding: 3px 10px; }
.topbar-build { font-size: 10.5px; color: var(--muted); opacity: .75; font-family: var(--font-mono, monospace); letter-spacing: .2px; }
.user-chip { display: flex; align-items: center; gap: 8px; cursor: pointer; padding: 3px 8px; border-radius: 8px; transition: background .15s; }
.user-chip:hover { background: var(--bg2, rgba(0,0,0,.04)); }
.user-avatar { width: 28px; height: 28px; border-radius: 50%; background: var(--accent); color: #fff; display: flex; align-items: center; justify-content: center; font-size: 13px; font-weight: 600; }
.user-name { font-size: 13px; }
.role-tag { font-size: 11px; padding: 2px 8px; border-radius: 10px; background: var(--accent-glow); color: var(--accent); }
/* 通知铃铛 */
.bell-wrap { position: relative; }
.bell-btn { position: relative; background: none; border: 1px solid var(--border); border-radius: 8px; color: var(--muted); width: 32px; height: 32px; display: flex; align-items: center; justify-content: center; cursor: pointer; }
.bell-btn:hover { color: var(--accent); border-color: var(--accent); }
.bell-dot { position: absolute; top: -5px; right: -5px; min-width: 16px; height: 16px; padding: 0 4px; border-radius: 8px; background: #f56c6c; color: #fff; font-size: 10px; line-height: 16px; text-align: center; }
.bell-panel { position: absolute; top: 40px; right: 0; width: 380px; max-height: 460px; background: var(--bg-card, #16181d); border: 1px solid var(--border); border-radius: 10px; box-shadow: 0 8px 32px rgba(0,0,0,.5); z-index: 100; display: flex; flex-direction: column; }
.bell-panel-head { display: flex; justify-content: space-between; align-items: center; padding: 10px 14px; border-bottom: 1px solid var(--border); font-size: 13px; font-weight: 600; }
.bell-list { overflow-y: auto; max-height: 400px; }
.bell-item { padding: 10px 14px; border-bottom: 1px solid rgba(255,255,255,.05); cursor: pointer; }
.bell-item:hover { background: rgba(255,255,255,.03); }
.bell-item.unread { background: rgba(64,158,255,.06); }
.bell-title { font-size: 13px; display: flex; align-items: center; gap: 6px; }
.bell-content { font-size: 12px; color: var(--muted); margin-top: 4px; line-height: 1.5; }
.bell-time { font-size: 11px; margin-top: 4px; }
.badge.kind-maintenance_due { color: #e6a23c; border-color: rgba(230,162,60,.4); }
.badge.kind-work_order { color: #409eff; border-color: rgba(64,158,255,.4); }
.badge.kind-system { color: #909399; border-color: rgba(144,147,153,.4); }
/* 导航分组标题 */
.nav-group { font-size: 11px; color: var(--muted); padding: 12px 16px 4px; text-transform: uppercase; letter-spacing: .08em; opacity: .7; }
/* V76 全局搜索入口 */
.gs-open-btn { display: flex; align-items: center; gap: 6px; background: none; border: 1px solid var(--border); border-radius: 8px; color: var(--muted); height: 32px; padding: 0 10px; cursor: pointer; font-size: 12.5px; transition: color .15s, border-color .15s; }
.gs-open-btn:hover { color: var(--accent); border-color: var(--accent); }
.gs-open-kbd { font-size: 10px; border: 1px solid var(--border); border-radius: 4px; padding: 1px 5px; opacity: .8; }

/* ==================== 移动端适配 ==================== */
.menu-btn { display: none; }
.nav-backdrop { display: none; }
@media (max-width: 900px) {
  .menu-btn {
    display: flex; align-items: center; justify-content: center;
    width: 34px; height: 34px; border: 1px solid var(--border); border-radius: 8px;
    background: none; color: var(--text2); font-size: 17px; cursor: pointer; flex-shrink: 0;
  }
  .topbar { padding: 0 12px; gap: 8px; }
  .topbar-right { gap: 8px; flex-shrink: 0; }
  .topbar-phase { display: none; }
  .topbar-build { display: none; }
  .user-name { display: none; }
  .role-tag { display: none; }
  .gs-open-label, .gs-open-kbd { display: none; }
  .gs-open-btn { width: 32px; padding: 0; justify-content: center; }
  .topbar-right .btn { flex-shrink: 0; white-space: nowrap; padding: 7px 10px; }
  .bell-panel { position: fixed; top: 56px; left: 8px; right: 8px; width: auto; }
  .app-body { position: relative; }
  .nav-backdrop {
    display: block; position: fixed; inset: 0; z-index: 190;
    background: rgba(15, 30, 60, .45); backdrop-filter: blur(2px);
  }
  .sidebar {
    position: fixed; top: 0; left: 0; bottom: 0; z-index: 200;
    width: 260px; max-width: 82vw; overflow-y: auto;
    background: var(--card); border-right: 1px solid var(--border);
    transform: translateX(-102%); transition: transform .25s ease;
    box-shadow: 4px 0 24px rgba(20, 40, 90, .18);
  }
  .sidebar.open { transform: translateX(0); }
  .main-content { padding: 14px 12px; }
}
</style>
