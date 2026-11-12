<template>
  <div class="dash-v43">
    <!-- ===== 视觉中心：运行状态英雄区 ===== -->
    <section class="hero" :class="'hero-' + healthLevel">
      <div class="hero-status" @click="go('/alert-wall')" title="点击进入告警盯屏">
        <div class="hero-orb" :class="'orb-' + healthLevel">
          <SvgIcon :name="healthLevel === 'red' ? 'alert' : healthLevel === 'amber' ? 'bell' : 'check-circle'" />
        </div>
        <div class="hero-text">
          <div class="hero-label">当前运行状态</div>
          <div class="hero-title">{{ healthTitle }}</div>
          <div class="hero-desc">{{ healthDesc }}</div>
        </div>
        <div class="hero-sevs">
          <div class="sev-chip sev-3" @click.stop="go('/alert-wall', { severity: 3 })">
            <b>{{ d.sev3 ?? 0 }}</b><span>严重</span>
          </div>
          <div class="sev-chip sev-2" @click.stop="go('/alert-wall', { severity: 2 })">
            <b>{{ d.sev2 ?? 0 }}</b><span>警告</span>
          </div>
          <div class="sev-chip sev-1" @click.stop="go('/alert-wall', { severity: 1 })">
            <b>{{ d.sev1 ?? 0 }}</b><span>提示</span>
          </div>
        </div>
      </div>
      <div class="hero-alerts">
        <div class="hero-alerts-head">
          <span class="hero-alerts-title">活跃告警焦点</span>
          <span class="hero-alerts-more clickable" @click="go('/alert-wall')">全部 {{ d.active_alerts ?? 0 }} 条 ›</span>
        </div>
        <div class="hero-alerts-body">
          <div v-for="a in recentAlerts" :key="a.id" class="ha-item clickable" @click="alertClick(a)">
            <span class="ha-sev" :class="'sev-' + a.severity">{{ SEVN[a.severity] }}</span>
            <span class="ha-msg">{{ a.point_name ? a.point_name + '：' : '' }}{{ a.message }}</span>
            <span class="ha-time">{{ fmtTime(a.triggered_at) }}</span>
          </div>
          <div v-if="!recentAlerts.length" class="ha-empty">暂无活跃告警，系统运行平稳</div>
        </div>
      </div>
    </section>

    <!-- ===== 核心指标（大 KPI）===== -->
    <div class="sec-head">核心运行指标</div>
    <section class="kpi-row">
      <div class="kpi-card kpi-lg clickable" @click="go('/energy')" title="进入能耗总览">
        <div class="kpi-label">实时 PUE</div>
        <div class="kpi-value" :class="pueClass">{{ fmtNum(e.latest_pue) }}</div>
        <div class="kpi-sub">目标 ≤ 1.40 · 近24h走势</div>
        <div ref="sparkRef" class="kpi-spark"></div>
      </div>
      <div class="kpi-card kpi-lg clickable" @click="go('/power-chain')" title="进入供电链路">
        <div class="kpi-label">总功率</div>
        <div class="kpi-value">{{ fmtNum(e.total_power_kw) }}<i>kW</i></div>
        <div class="kpi-sub">IT 负载 {{ fmtNum(e.it_power_kw) }} kW</div>
        <div class="kpi-extra">近 7 天能耗 <b>{{ fmtInt(e.energy_7d_kwh) }}</b> kWh</div>
      </div>
      <div class="kpi-card kpi-lg clickable" @click="go('/monitoring-points', { status: d.monitor_alert > 0 ? 2 : 1 })" title="进入测点管理">
        <div class="kpi-label">测点在线率</div>
        <div class="kpi-value" :class="d.monitor_alert > 0 ? 'v-warn' : 'v-ok'">{{ onlineRate }}<i>%</i></div>
        <div class="kpi-sub">告警 {{ d.monitor_alert ?? 0 }} · 离线 {{ d.monitor_offline ?? 0 }} / 共 {{ d.monitor_points ?? 0 }}</div>
        <div class="kpi-bar"><div class="kpi-bar-fill" :class="d.monitor_alert > 0 ? 'b-warn' : 'b-ok'" :style="{ width: onlineRate + '%' }"></div></div>
      </div>
      <div class="kpi-card kpi-lg clickable" @click="go('/work-orders')" title="进入运维工单">
        <div class="kpi-label">运维负荷</div>
        <div class="kpi-value" :class="{ 'v-warn': (d.orders_open ?? 0) > 0 }">{{ d.orders_open ?? 0 }}<i>单待处理</i></div>
        <div class="kpi-sub">今日新增 {{ d.today_orders ?? 0 }} · 今日告警 {{ d.today_alerts ?? 0 }}</div>
        <div class="kpi-extra">待审批变更 <b>{{ d.changes_pending ?? 0 }}</b> · 供电异常 <b :class="{ 'v-err': (d.power_alarm ?? 0) > 0 }">{{ d.power_alarm ?? 0 }}</b></div>
      </div>
    </section>

    <!-- ===== V65 风险与洞察（B1-2/G7：五指标回流总览，卡卡穿透） ===== -->
    <div class="sec-head" v-if="ins.health">风险与洞察</div>
    <section class="ins-row" v-if="ins.health">
      <div class="ins-card clickable" @click="go('/assets', { health_grade: 'risk' })" title="进入资产台账（风险级过滤）">
        <div class="ins-head"><SvgIcon name="shield" /><span>设备健康度</span></div>
        <div class="ins-main" :class="insBadHealth > 0 ? 'v-err' : 'v-ok'">{{ insBadHealth }}<i>台风险</i></div>
        <div class="ins-sub">评估 {{ ins.health.total ?? 0 }} 台 · 均分 {{ ins.health.avg_score ?? '-' }} · 关联工单 {{ ins.health.open_work_orders ?? 0 }}</div>
      </div>
      <div class="ins-card clickable" @click="go('/energy')" title="进入能耗总览（能效合规）">
        <div class="ins-head"><SvgIcon name="bolt" /><span>PUE 合规</span></div>
        <div class="ins-main" :class="insPueBad > 0 ? 'v-warn' : 'v-ok'">{{ insPueBad }}<i>站需关注</i></div>
        <div class="ins-sub">超标 {{ ins.pue?.over ?? 0 }} · 临限 {{ ins.pue?.edge ?? 0 }} · 达标 {{ ins.pue?.ok ?? 0 }} / {{ ins.pue?.sites ?? 0 }} 站</div>
      </div>
      <div class="ins-card clickable" @click="go('/capacity')" title="进入机柜容量（饱和预测）">
        <div class="ins-head"><SvgIcon name="layers" /><span>容量预测</span></div>
        <div class="ins-main" :class="insCapBad > 0 ? 'v-warn' : 'v-ok'">{{ insCapBad }}<i>站趋饱和</i></div>
        <div class="ins-sub">告急 {{ ins.capacity?.risk ?? 0 }} · 预警 {{ ins.capacity?.warn ?? 0 }} · 覆盖 {{ ins.capacity?.total ?? 0 }} 站</div>
      </div>
      <div class="ins-card clickable" @click="go('/alert-rules', { tab: 'quality' })" title="进入告警规则（质量治理页签）">
        <div class="ins-head"><SvgIcon name="storm" /><span>告警质量</span>
          <span class="ins-badge" :class="insNoiseCls">{{ { ok: '良好', warn: '偏高', bad: '噪声高' }[ins.alert_quality?.grade] || '-' }}</span>
        </div>
        <div class="ins-main" :class="insNoiseCls">{{ ins.alert_quality?.noise_rate ?? 0 }}<i>% 噪声率</i></div>
        <div class="ins-sub">近7天 {{ ins.alert_quality?.total ?? 0 }} 条 · 疑似误报 {{ ins.alert_quality?.suspected_false ?? 0 }} · 抑制 {{ ins.alert_quality?.suppressed ?? 0 }}</div>
      </div>
      <div class="ins-card clickable" @click="go('/work-orders', { tab: 'sla' })" title="进入运维工单（SLA 考核页签）">
        <div class="ins-head"><SvgIcon name="clock" /><span>SLA 达标</span></div>
        <div class="ins-main" :class="insSlaCls">{{ ins.sla?.breach_rate ?? 0 }}<i>% 违约率</i></div>
        <div class="ins-sub">近30天 {{ ins.sla?.total ?? 0 }} 单 · 违约 {{ ins.sla?.breached ?? 0 }}（响应 {{ ins.sla?.respond_breach ?? 0 }} / 解决 {{ ins.sla?.resolve_breach ?? 0 }}）</div>
      </div>
    </section>

    <!-- ===== 运维动态（中卡） ===== -->
    <div class="sec-head">运维动态</div>
    <section class="ops-row">
      <div class="ops-card ops-power clickable" @click="go('/power-chain', { tab: 'redundancy' })" title="供电冗余分析：单点检测/负载率/机柜供电路径">
        <div class="ops-icon i-pw"><SvgIcon name="bolt" /></div>
        <div class="ops-body">
          <div class="ops-num" :class="{ 'v-err': (pwr.summary?.risk ?? 0) > 0, 'v-warn': !(pwr.summary?.risk > 0) && (pwr.summary?.warn ?? 0) > 0 }">{{ (pwr.summary?.risk ?? 0) + (pwr.summary?.warn ?? 0) }}</div>
          <div class="ops-label">供电冗余风险站点（单点+不足）</div>
          <div class="hg-chips">
            <span v-for="g in ['ok','warn','risk']" :key="g" class="hg-chip" :class="'pw-c-'+g" @click.stop="go('/power-chain', { tab: 'redundancy' })" :title="PWN[g]+' '+(pwr.summary?.[g]??0)+' 个站点，点击查看冗余分析'">{{ PWN[g] }} {{ pwr.summary?.[g] ?? 0 }}</span>
          </div>
        </div>
        <div class="ops-trend" title="供电冗余分析">冗余分析 ›</div>
      </div>
      <div class="ops-card clickable" @click="go('/work-orders')">
        <div class="ops-icon i-wo"><SvgIcon name="clipboard" /></div>
        <div class="ops-body">
          <div class="ops-num" :class="{ 'v-warn': (d.orders_open ?? 0) > 0 }">{{ d.orders_open ?? 0 }}</div>
          <div class="ops-label">未结工单</div>
        </div>
        <div class="ops-trend">今日 +{{ d.today_orders ?? 0 }}</div>
      </div>
      <div class="ops-card clickable" @click="go('/changes')">
        <div class="ops-icon i-chg"><SvgIcon name="swap" /></div>
        <div class="ops-body">
          <div class="ops-num">{{ d.changes_pending ?? 0 }}</div>
          <div class="ops-label">待审批变更</div>
        </div>
        <div class="ops-trend">执行中 {{ d.changes_running ?? 0 }}</div>
      </div>
      <div class="ops-card clickable" @click="go('/power-chain')">
        <div class="ops-icon i-pw"><SvgIcon name="bolt" /></div>
        <div class="ops-body">
          <div class="ops-num" :class="{ 'v-err': (d.power_alarm ?? 0) > 0 }">{{ d.power_alarm ?? 0 }}</div>
          <div class="ops-label">供电链路异常</div>
        </div>
        <div class="ops-trend">告警 / 故障节点</div>
      </div>
      <div class="ops-card clickable" @click="go('/inspection')">
        <div class="ops-icon i-insp"><SvgIcon name="search" /></div>
        <div class="ops-body">
          <div class="ops-num">{{ d.today_alerts ?? 0 }}</div>
          <div class="ops-label">今日告警</div>
        </div>
        <div class="ops-trend">活跃 {{ d.active_alerts ?? 0 }}</div>
      </div>
      <div class="ops-card ops-health">
        <div class="ops-icon i-hg"><SvgIcon name="gauge" /></div>
        <div class="ops-body">
          <div class="ops-num" :class="{ 'v-err': (hs.grades?.risk ?? 0) > 0, 'v-warn': !(hs.grades?.risk > 0) && (hs.grades?.sub_healthy ?? 0) > 0 }">{{ (hs.grades?.risk ?? 0) + (hs.grades?.sub_healthy ?? 0) }}</div>
          <div class="ops-label">健康风险设备（亚健康+风险）</div>
          <div class="hg-chips">
            <span v-for="g in ['healthy','attention','sub_healthy','risk']" :key="g" class="hg-chip" :class="'hg-c-'+g" @click.stop="go('/assets', { health_grade: g })" :title="HGN[g]+' '+(hs.grades?.[g]??0)+' 台，点击查看清单'">{{ HGN[g] }} {{ hs.grades?.[g] ?? 0 }}</span>
          </div>
        </div>
        <div class="ops-trend hg-avg" @click.stop="go('/assets', { health_grade: 'risk' })" title="查看风险设备清单">均分 {{ hs.avg_score ?? '-' }} ›</div>
      </div>
      <div class="ops-card clickable" @click="go('/work-orders')">
        <div class="ops-body ops-orders">
          <div class="ops-label" style="margin-bottom:4px">最近工单</div>
          <div class="mini-orders">
            <div v-for="w in orders.slice(0, 3)" :key="w.id" class="mini-order">
              <span class="mono muted">{{ w.code }}</span>
              <span class="mo-title">{{ w.title }}</span>
              <span class="badge" :class="'wost-' + w.status">{{ WOST[w.status] }}</span>
            </div>
            <div v-if="!orders.length" class="empty">暂无工单</div>
          </div>
        </div>
      </div>
    </section>

    <!-- ===== 资产底盘（小卡） ===== -->
    <div class="sec-head">基础设施规模</div>
    <section class="asset-row">
      <div class="asset-card clickable" @click="go('/topology')">
        <div class="asset-num">{{ d.sites ?? '-' }}</div><div class="asset-label">数据中心站点</div>
      </div>
      <div class="asset-card clickable" @click="go('/topology')">
        <div class="asset-num">{{ d.rooms ?? '-' }}</div><div class="asset-label">机房 / 模块</div>
      </div>
      <div class="asset-card clickable" @click="go('/capacity')">
        <div class="asset-num">{{ d.cabinets ?? '-' }}</div><div class="asset-label">机柜（在用 {{ d.cabinets_active ?? 0 }}）</div>
      </div>
      <div class="asset-card clickable" @click="go('/assets')">
        <div class="asset-num">{{ d.devices ?? '-' }}</div><div class="asset-label">设备（在架 {{ d.devices_mounted ?? 0 }}）</div>
      </div>
      <div class="asset-card clickable" @click="go('/gateways')">
        <div class="asset-num">{{ d.gateways ?? '-' }}</div><div class="asset-label">网关（在线 {{ d.gateways_online ?? 0 }}）</div>
      </div>
      <div class="asset-card clickable" @click="go('/monitor')">
        <div class="asset-num">{{ d.monitor_points ?? 0 }}</div><div class="asset-label">监控测点</div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import * as echarts from 'echarts'
import api from '../api'
import { useSettingsStore } from '../stores/settings'

const router = useRouter()
function go(path, query) {
  router.push(query ? { path, query } : path)
}

const d = ref({})
const e = ref({})
const orders = ref([])
const sparkRef = ref(null)
let sparkChart = null
const WOST = { 1: '待受理', 2: '处理中', 3: '已完成', 4: '已关闭', 5: '已取消' }
const SEVN = { 1: '提示', 2: '警告', 3: '严重' }

const fmtNum = (v) => {
  if (v == null || v === '') return '-'
  const n = Number(v)
  return Math.abs(n) < 10 && n % 1 !== 0 ? n.toFixed(2) : n.toFixed(1).replace(/\.0$/, '')
}
const fmtInt = (v) => v == null ? '-' : Math.round(Number(v)).toLocaleString('zh-CN')
const fmtTime = t => t ? String(t).replace('T', ' ').slice(5, 16) : '-'

// 运行健康分级：有严重告警=red，有警告/待办负荷=amber，否则=ok
const healthLevel = computed(() => {
  if ((d.value.sev3 ?? 0) > 0) return 'red'
  if ((d.value.sev2 ?? 0) > 0 || (d.value.active_alerts ?? 0) > 0) return 'amber'
  return 'ok'
})
const healthTitle = computed(() => ({ red: '存在严重告警', amber: '需要关注', ok: '运行正常' }[healthLevel.value]))
const healthDesc = computed(() => {
  if (healthLevel.value === 'red') return `${d.value.sev3} 条严重告警待处置，点击分级徽章直达盯屏`
  if (healthLevel.value === 'amber') return `${d.value.active_alerts ?? 0} 条活跃告警，未结工单 ${d.value.orders_open ?? 0} 单`
  return '全部测点平稳，无活跃告警'
})
const onlineRate = computed(() => {
  const t = d.value.monitor_points || 0
  if (!t) return 100
  const online = (d.value.monitor_normal ?? 0) + (d.value.monitor_alert ?? 0)
  return Math.round((online / t) * 100)
})
const pueClass = computed(() => {
  const v = e.value.latest_pue
  if (!v) return ''
  return v <= 1.5 ? 'v-ok' : v <= 1.7 ? 'v-warn' : 'v-err'
})
const recentAlerts = computed(() => (d.value.recent_alerts || []).slice(0, 5))

function alertClick(a) {
  if (a.point_id) go('/monitoring-points', { id: a.point_id, detail: '1' })
  else go('/alert-wall')
}

function drawSpark() {
  if (!sparkRef.value || !e.value.trend_values?.length) return
  if (!sparkChart) sparkChart = echarts.init(sparkRef.value)
  const st = useSettingsStore()
  const palette = st.chartPalette || []
  const color = palette[0] || '#00d4ff'
  sparkChart.setOption({
    grid: { left: 2, right: 2, top: 6, bottom: 2 },
    tooltip: { trigger: 'axis', confine: true },
    xAxis: { type: 'category', show: false, data: e.value.trend_labels },
    yAxis: { type: 'value', show: false, min: (v) => Math.floor((v.min - 0.05) * 100) / 100 },
    series: [{
      type: 'line', data: e.value.trend_values, smooth: true, symbol: 'none', lineStyle: { width: 2, color },
      areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: color + '55' }, { offset: 1, color: color + '05' }]) },
      markLine: { silent: true, symbol: 'none', label: { show: false }, lineStyle: { type: 'dashed', color: '#00e676', width: 1 }, data: [{ yAxis: 1.4 }] },
    }],
  })
}

// V50 设备健康汇总
const HGN = { healthy: '健康', attention: '关注', sub_healthy: '亚健康', risk: '风险' }
const hs = ref({})
async function loadHealth() {
  try {
    const r = await api.get('/asset-health/summary')
    if (r.code === 0) hs.value = r.data || {}
  } catch (err) { /* 静默 */ }
}

// V54 供电冗余汇总
const PWN = { ok: '充足', warn: '不足', risk: '单点' }
const pwr = ref({})
async function loadPowerRed() {
  try {
    const r = await api.get('/power/redundancy')
    if (r.code === 0) pwr.value = r.data || {}
  } catch (err) { /* 静默 */ }
}

// V65 风险与洞察聚合（B1-2/G7，五指标复用既有口径）
const ins = ref({})
async function loadInsights() {
  try {
    const r = await api.get('/dashboard/insights')
    if (r.code === 0) ins.value = r.data || {}
  } catch (err) { /* 静默 */ }
}
const insBadHealth = computed(() => ((ins.value.health?.grades?.sub_healthy) ?? 0) + ((ins.value.health?.grades?.risk) ?? 0))
const insPueBad = computed(() => (ins.value.pue?.over ?? 0) + (ins.value.pue?.edge ?? 0))
const insCapBad = computed(() => (ins.value.capacity?.risk ?? 0) + (ins.value.capacity?.warn ?? 0))
const insNoiseCls = computed(() => ({ ok: 'v-ok', warn: 'v-warn', bad: 'v-err' }[ins.value.alert_quality?.grade] || ''))
const insSlaCls = computed(() => (ins.value.sla?.breached ?? 0) > 0 ? 'v-warn' : 'v-ok')

async function load() {
  try {
    const res = await api.get('/dashboard')
    if (res.code === 0) {
      d.value = res.data.stats || res.data
      e.value = res.data.energy || {}
      orders.value = res.data.recent_orders || []
      await nextTick()
      drawSpark()
    }
  } catch (err) { /* 静默 */ }
  loadHealth()
  loadPowerRed()
  loadInsights()
}

const onTheme = () => drawSpark()
const onResize = () => sparkChart && sparkChart.resize()

onMounted(() => {
  load()
  document.addEventListener('dcim-theme-change', onTheme)
  window.addEventListener('resize', onResize)
})
onUnmounted(() => {
  document.removeEventListener('dcim-theme-change', onTheme)
  window.removeEventListener('resize', onResize)
  sparkChart && sparkChart.dispose()
})
</script>

<style scoped>
.dash-v43 { display: flex; flex-direction: column; gap: 14px; }

/* ===== V65 风险与洞察 ===== */
.ins-row { display: grid; grid-template-columns: repeat(5, 1fr); gap: 12px; }
.ins-card { border: 1px solid var(--border); background: var(--card); border-radius: 12px; padding: 13px 15px; cursor: pointer; transition: transform .15s, border-color .15s; min-width: 0; }
.ins-card:hover { transform: translateY(-2px); border-color: var(--accent); }
.ins-head { display: flex; align-items: center; gap: 6px; font-size: 13px; color: var(--muted); margin-bottom: 7px; }
.ins-head svg { width: 15px; height: 15px; flex-shrink: 0; }
.ins-main { font-size: 26px; font-weight: 700; line-height: 1.15; }
.ins-main i { font-style: normal; font-size: 12px; color: var(--muted); margin-left: 4px; font-weight: 400; }
.ins-sub { font-size: 12px; color: var(--muted); margin-top: 5px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.ins-badge { display: inline-block; padding: 1px 8px; border-radius: 10px; font-size: 11px; border: 1px solid var(--border); }
.ins-badge.v-ok { color: var(--ok, #00e676); border-color: currentColor; }
.ins-badge.v-warn { color: var(--warn, #ffb300); border-color: currentColor; }
.ins-badge.v-err { color: var(--err, #ff5252); border-color: currentColor; }
@media (max-width: 1200px) { .ins-row { grid-template-columns: repeat(3, 1fr); } }
@media (max-width: 900px) { .ins-row { grid-template-columns: repeat(2, 1fr); } }

/* ===== 英雄区 ===== */
.hero { display: grid; grid-template-columns: 1.1fr 1fr; gap: 0; border-radius: 14px; overflow: hidden;
  border: 1px solid var(--border); background: var(--card); position: relative; }
.hero::before { content: ''; position: absolute; inset: 0; pointer-events: none; opacity: .5;
  background: radial-gradient(600px 200px at 18% 0%, color-mix(in srgb, var(--accent) 10%, transparent), transparent 70%); }
.hero-red { border-color: color-mix(in srgb, #f56c6c 55%, var(--border)); box-shadow: 0 0 24px color-mix(in srgb, #f56c6c 18%, transparent); }
.hero-amber { border-color: color-mix(in srgb, #e6a23c 45%, var(--border)); }
.hero-ok { border-color: color-mix(in srgb, #67c23a 40%, var(--border)); }
.hero-status { display: flex; flex-direction: column; gap: 12px; padding: 20px 22px; cursor: pointer; position: relative; }
.hero-orb { width: 54px; height: 54px; border-radius: 50%; display: flex; align-items: center; justify-content: center; }
.hero-orb svg, .hero-orb .svg-icon { width: 28px; height: 28px; }
.orb-ok { background: color-mix(in srgb, #67c23a 16%, transparent); color: #67c23a; border: 2px solid color-mix(in srgb, #67c23a 50%, transparent); box-shadow: 0 0 18px color-mix(in srgb, #67c23a 25%, transparent); }
.orb-amber { background: color-mix(in srgb, #e6a23c 16%, transparent); color: #e6a23c; border: 2px solid color-mix(in srgb, #e6a23c 50%, transparent); box-shadow: 0 0 18px color-mix(in srgb, #e6a23c 25%, transparent); animation: hero-pulse 2s infinite; }
.orb-red { background: color-mix(in srgb, #f56c6c 18%, transparent); color: #f56c6c; border: 2px solid color-mix(in srgb, #f56c6c 60%, transparent); box-shadow: 0 0 22px color-mix(in srgb, #f56c6c 35%, transparent); animation: hero-pulse 1.2s infinite; }
@keyframes hero-pulse { 50% { transform: scale(1.06); } }
.hero-text .hero-label { font-size: 12px; color: var(--muted); letter-spacing: 2px; }
.hero-text .hero-title { font-size: 30px; font-weight: 800; line-height: 1.2; margin: 2px 0; }
.hero-red .hero-title { color: #f56c6c; }
.hero-amber .hero-title { color: #e6a23c; }
.hero-ok .hero-title { color: #67c23a; }
.hero-text .hero-desc { font-size: 12.5px; color: var(--text2, var(--muted)); }
.hero-sevs { display: flex; gap: 10px; margin-top: auto; }
.sev-chip { flex: 1; text-align: center; padding: 8px 4px; border-radius: 10px; border: 1px solid var(--border); background: var(--card2, transparent); transition: transform .15s; }
.sev-chip:hover { transform: translateY(-2px); }
.sev-chip b { display: block; font-size: 22px; font-variant-numeric: tabular-nums; }
.sev-chip span { font-size: 11px; color: var(--muted); }
.sev-chip.sev-3 b { color: #f56c6c; }
.sev-chip.sev-3 { border-color: color-mix(in srgb, #f56c6c 35%, var(--border)); }
.sev-chip.sev-2 b { color: #e6a23c; }
.sev-chip.sev-1 b { color: #409eff; }
.hero-alerts { border-left: 1px solid var(--border); padding: 16px 18px; display: flex; flex-direction: column; min-height: 190px; position: relative; }
.hero-alerts-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
.hero-alerts-title { font-size: 13px; font-weight: 700; color: var(--text2, var(--text)); }
.hero-alerts-more { font-size: 11.5px; color: var(--muted); }
.hero-alerts-more:hover { color: var(--accent); }
.hero-alerts-body { flex: 1; overflow-y: auto; display: flex; flex-direction: column; gap: 2px; }
.ha-item { display: flex; align-items: center; gap: 8px; padding: 6px 4px; border-bottom: 1px solid var(--border); font-size: 12px; border-radius: 4px; }
.ha-item:hover { background: var(--card2, rgba(64,158,255,.06)); }
.ha-sev { flex-shrink: 0; font-size: 10px; padding: 1px 7px; border-radius: 8px; font-weight: 700; }
.ha-sev.sev-3 { color: #f56c6c; border: 1px solid color-mix(in srgb, #f56c6c 50%, transparent); background: color-mix(in srgb, #f56c6c 10%, transparent); }
.ha-sev.sev-2 { color: #e6a23c; border: 1px solid color-mix(in srgb, #e6a23c 50%, transparent); background: color-mix(in srgb, #e6a23c 8%, transparent); }
.ha-sev.sev-1 { color: #409eff; border: 1px solid color-mix(in srgb, #409eff 50%, transparent); background: color-mix(in srgb, #409eff 8%, transparent); }
.ha-msg { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text2, var(--text)); }
.ha-time { color: var(--muted); font-size: 10px; flex-shrink: 0; }
.ha-empty { color: var(--muted); font-size: 12px; text-align: center; padding: 26px 0; }

/* ===== 小节标题 ===== */
.sec-head { font-size: 13px; font-weight: 700; color: var(--text2, var(--text)); letter-spacing: 1px;
  padding-left: 10px; border-left: 3px solid var(--accent); margin-top: 4px; }

/* ===== 核心 KPI ===== */
.kpi-row { display: grid; grid-template-columns: repeat(4, 1fr); gap: 14px; }
.kpi-card { background: var(--card); border: 1px solid var(--border); border-radius: 12px; padding: 16px 18px;
  transition: transform .15s, box-shadow .15s, border-color .2s; position: relative; overflow: hidden; }
.kpi-card.clickable { cursor: pointer; }
.kpi-card.clickable:hover { transform: translateY(-2px); border-color: var(--accent); box-shadow: var(--shadow, 0 6px 18px rgba(0,0,0,.15)); }
.kpi-label { font-size: 12px; color: var(--muted); letter-spacing: 1px; }
.kpi-value { font-size: 40px; font-weight: 800; line-height: 1.15; margin: 4px 0; font-variant-numeric: tabular-nums; color: var(--text); }
.kpi-value i { font-style: normal; font-size: 14px; font-weight: 400; color: var(--muted); margin-left: 4px; }
.kpi-sub { font-size: 11.5px; color: var(--muted); }
.kpi-extra { font-size: 11.5px; color: var(--muted); margin-top: 6px; padding-top: 6px; border-top: 1px dashed var(--border); }
.kpi-extra b { color: var(--text); font-size: 13px; }
.kpi-spark { height: 52px; margin: 4px -6px -4px; }
.kpi-bar { height: 6px; border-radius: 3px; background: var(--border); margin-top: 10px; overflow: hidden; }
.kpi-bar-fill { height: 100%; border-radius: 3px; transition: width .5s; }
.b-ok { background: linear-gradient(90deg, #67c23a, #95d475); }
.b-warn { background: linear-gradient(90deg, #e6a23c, #f3c78a); }
.v-ok { color: #67c23a; }
.v-warn { color: #e6a23c; }
.v-err { color: #f56c6c; }

/* ===== 运维动态 ===== */
.ops-row { display: grid; grid-template-columns: repeat(5, 1fr) 1.6fr; gap: 14px; }
.ops-card { background: var(--card); border: 1px solid var(--border); border-radius: 12px; padding: 14px 16px;
  display: flex; align-items: center; gap: 12px; transition: transform .15s, border-color .2s; }
.ops-card.clickable { cursor: pointer; }
.ops-card.clickable:hover { transform: translateY(-2px); border-color: var(--accent); }
.ops-icon { width: 38px; height: 38px; border-radius: 10px; display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
.ops-icon svg, .ops-icon .svg-icon { width: 20px; height: 20px; }
.i-wo { background: color-mix(in srgb, #409eff 14%, transparent); color: #409eff; }
.i-chg { background: color-mix(in srgb, #b37feb 14%, transparent); color: #b37feb; }
.i-pw { background: color-mix(in srgb, #e6a23c 14%, transparent); color: #e6a23c; }
.i-insp { background: color-mix(in srgb, #f56c6c 14%, transparent); color: #f56c6c; }
.i-hg { background: color-mix(in srgb, #67c23a 14%, transparent); color: #67c23a; }
/* V50 健康卡 */
.hg-chips { display: flex; gap: 5px; margin-top: 6px; flex-wrap: wrap; }
.hg-chip { font-size: 10.5px; padding: 2px 7px; border-radius: 999px; cursor: pointer; border: 1px solid transparent; transition: filter .15s; white-space: nowrap; }
.hg-chip:hover { filter: brightness(1.15); }
.hg-c-healthy { background: rgba(103,194,58,.13); color: #67c23a; }
.hg-c-attention { background: rgba(64,158,255,.13); color: #409eff; }
.hg-c-sub_healthy { background: rgba(230,162,60,.15); color: #e6a23c; }
.hg-c-risk { background: rgba(245,108,108,.13); color: #f56c6c; }
.hg-avg { cursor: pointer; }
.hg-avg:hover { color: var(--accent); }
.ops-body { flex: 1; min-width: 0; }
.ops-num { font-size: 24px; font-weight: 800; line-height: 1.1; font-variant-numeric: tabular-nums; }
.ops-label { font-size: 12px; color: var(--muted); }
.ops-trend { font-size: 11px; color: var(--muted); align-self: flex-start; flex-shrink: 0; }
.ops-orders { display: flex; flex-direction: column; }
.mini-orders { display: flex; flex-direction: column; gap: 4px; }
.mini-order { display: flex; align-items: center; gap: 8px; font-size: 12px; }
.mo-title { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.badge.wost-1 { color: #e6a23c; border-color: rgba(230, 162, 60, .4); }
.badge.wost-2 { color: #409eff; border-color: rgba(64, 158, 255, .4); }
.badge.wost-3 { color: #67c23a; border-color: rgba(103, 194, 58, .4); }
.badge.wost-4, .badge.wost-5 { color: #909399; border-color: rgba(144, 147, 153, .4); }

/* ===== 资产底盘 ===== */
.asset-row { display: grid; grid-template-columns: repeat(6, 1fr); gap: 14px; }
.asset-card { background: var(--card); border: 1px solid var(--border); border-radius: 10px; padding: 12px 14px; text-align: center;
  transition: transform .15s, border-color .2s; cursor: pointer; }
.asset-card:hover { transform: translateY(-1px); border-color: var(--accent); }
.asset-num { font-size: 22px; font-weight: 700; font-variant-numeric: tabular-nums; color: var(--text); }
.asset-label { font-size: 11px; color: var(--muted); margin-top: 2px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

/* ===== 响应式 ===== */
@media (max-width: 1200px) {
  .kpi-row { grid-template-columns: repeat(2, 1fr); }
  .ops-row { grid-template-columns: repeat(2, 1fr); }
  .ops-row .ops-card:last-child { grid-column: span 2; }
  .asset-row { grid-template-columns: repeat(3, 1fr); }
}
@media (max-width: 900px) {
  .hero { grid-template-columns: 1fr; }
  .hero-alerts { border-left: none; border-top: 1px solid var(--border); min-height: 140px; }
  .kpi-row, .ops-row, .asset-row { grid-template-columns: 1fr 1fr; gap: 10px; }
  .ops-row .ops-card:last-child { grid-column: span 2; }
  .kpi-value { font-size: 32px; }
}
@media (max-width: 560px) {
  .kpi-row, .ops-row, .asset-row { grid-template-columns: 1fr; }
  .ops-row .ops-card:last-child { grid-column: span 1; }
}
</style>
