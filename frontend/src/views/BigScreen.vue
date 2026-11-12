<template>
  <div class="bscreen">
    <!-- 顶栏 -->
    <header class="bs-header">
      <h1 class="bs-title">{{ settings.site_title || 'DCIM' }} · 可视化驾驶舱</h1>
      <div class="bs-header-right">
        <span class="bs-live" :class="{ok: liveOk}">● {{ liveOk ? '实时数据' : '数据延迟' }}</span>
        <span class="bs-clock">{{ clock }}</span>
        <span class="bs-running">运行 <b>{{ runDays }}</b> 天</span>
        <button class="bs-fs-btn" @click="toggleFullscreen" :title="isFs ? '退出投屏' : '投屏模式'">{{ isFs ? '退出投屏' : '投屏' }}</button>
      </div>
    </header>

    <!-- V43 英雄指标：PUE + 总功率（视觉中心） -->
    <div class="bs-heroes">
      <div class="hero-kpi clickable" @click="go('/energy')" title="进入能耗总览">
        <div class="hk-left">
          <div class="kpi-label">实时 PUE · 能效比</div>
          <div class="hk-value" :class="pueClass">{{ kpi.pue || '--' }}</div>
        </div>
        <div class="hk-right">
          <div class="hk-sub">目标 ≤ 1.40</div>
          <div class="hk-comp" v-if="comp && comp.policy" :class="'hc-' + comp.status">
            {{ comp.policy.province }}红线 ≤{{ comp.policy.new_large_limit }} ·
            <template v-if="comp.status === 'over'">超标 {{ Math.abs(comp.margin) }}</template>
            <template v-else-if="comp.status === 'edge'">临界 余量{{ comp.margin }}</template>
            <template v-else-if="comp.status === 'nodata'">暂无PUE数据</template>
            <template v-else>合规 余量+{{ comp.margin }}</template>
          </div>
          <div class="hk-go">能耗总览 ›</div>
        </div>
      </div>
      <div class="hero-kpi clickable" @click="go('/power-chain')" title="进入供电链路">
        <div class="hk-left">
          <div class="kpi-label">总功率 · 实时负载</div>
          <div class="hk-value c1">{{ kpi.totalKW }} <i>kW</i></div>
        </div>
        <div class="hk-right">
          <div class="hk-sub">IT 负载 {{ kpi.itKW }} kW</div>
          <div class="hk-go">供电链路 ›</div>
        </div>
      </div>
    </div>

    <!-- 次级指标带（全部可点击穿透） -->
    <div class="bs-kpis">
      <div class="kpi kpi-sm clickable" @click="go('/assets')" title="进入资产台账">
        <div class="kpi-label">设备总数 / 在架</div>
        <div class="kpi-value">{{ d.devices }} <i>/ {{ d.devices_mounted }}</i></div>
        <div class="kpi-sub" @click.stop="go('/network-devices')">网络设备 {{ d.net_online }}/{{ d.net_devices }} 在线 ›</div>
      </div>
      <div class="kpi kpi-sm clickable" @click="go('/monitoring-points', {status: d.monitor_alert > 0 ? 2 : 1})" title="进入测点管理">
        <div class="kpi-label">测点在线率</div>
        <div class="kpi-value" :class="d.monitor_alert > 0 ? 'red' : 'green'">{{ onlineRate }}<i>%</i></div>
        <div class="kpi-sub">告警测点 {{ d.monitor_alert }} / {{ d.monitor_total }} ›</div>
      </div>
      <div class="kpi kpi-sm clickable kpi-alert" :class="{ 'kpi-alerting': d.active_alerts > 0 }" @click="go('/alert-wall')" title="进入告警盯屏">
        <div class="kpi-label">活跃告警</div>
        <div class="kpi-value" :class="d.active_alerts > 0 ? 'red' : 'green'">{{ d.active_alerts }}</div>
        <div class="kpi-sub">今日新增 {{ d.today_alerts }} ›</div>
      </div>
      <div class="kpi kpi-sm clickable" @click="go('/work-orders')" title="进入运维工单">
        <div class="kpi-label">今日工单</div>
        <div class="kpi-value c1">{{ d.today_orders }}</div>
        <div class="kpi-sub" @click.stop="go('/power-chain', {status: 2})">供电链路 {{ d.power_alarm || 0 }} 异常 ›</div>
      </div>
    </div>

    <!-- V21-③ 项目铭牌 -->
    <div class="bs-nameplate" v-if="np.site_name" @click="go('/topology')" title="点击进入基础设施层级">
      <span class="np-item"><b>{{ np.site_name }}</b></span>
      <span class="np-item" v-if="np.utility_kva">市电 {{ fmtK(np.utility_kva) }} kVA</span>
      <span class="np-item" v-if="np.it_rated_kw">IT 额定 {{ fmtK(np.it_rated_kw) }} kW</span>
      <span class="np-item" v-if="np.area_sqm">机房面积 {{ fmtK(np.area_sqm) }} ㎡</span>
      <span class="np-item">机柜 {{ np.cabinets || 0 }} 台</span>
      <span class="np-item">U位 {{ np.u_used || 0 }}/{{ np.u_total || 0 }}</span>
      <span class="np-item" v-if="np.cooling_units">制冷 {{ np.cooling_units }} 台</span>
      <span class="np-item" v-if="np.manager">负责人 {{ np.manager }}</span>
    </div>

    <!-- 主体三列 -->
    <div class="bs-main">
      <!-- 左列 -->
      <div class="bs-col">
        <section class="bs-panel clickable-panel" @click="go('/trend-analysis')" title="点击进入趋势分析">
          <div class="panel-title"><SvgIcon name="bolt" />功率趋势（最近 12 小时）<span class="panel-go">›</span></div>
          <div ref="chartPower" class="chart-box chart-lg"></div>
        </section>
        <section class="bs-panel clickable-panel" @click="go('/energy')" title="点击进入能耗总览">
          <div class="panel-title"><SvgIcon name="trend" />PUE 走势<span class="panel-go">›</span></div>
          <div ref="chartPue" class="chart-box"></div>
        </section>
        <section class="bs-panel">
          <div class="panel-title"><SvgIcon name="building" />房间功耗分布（IT 负载）<span class="panel-go link" @click="go('/capacity')">机柜容量 ›</span></div>
          <div ref="chartRoomPower" class="chart-box"></div>
        </section>
        <section class="bs-panel">
          <div class="panel-title"><SvgIcon name="plug" />功率构成（实时分解）<span class="panel-go link" @click="go('/energy')">能耗总览 ›</span></div>
          <div ref="chartBreakdown" class="chart-box chart-sm"></div>
        </section>
      </div>

      <!-- 中列（视觉核心） -->
      <div class="bs-col bs-col-core">
        <section class="bs-panel">
          <div class="panel-title"><SvgIcon name="temp" />温湿度实时矩阵<span class="panel-go link" @click="go('/monitoring-points', {type:'temp_humidity'})">测点管理 ›</span></div>
          <div class="th-matrix" v-if="thMatrix.length">
            <div class="th-cell clickable" v-for="(c, i) in thMatrix" :key="i" :class="['th-s' + c.status]" @click="go('/monitoring-points', {keyword: c.point})" :title="'点击查看 ' + c.point">
              <div class="th-point">{{ shortName(c.point) }}</div>
              <div class="th-value">{{ c.value.toFixed(1) }}<i>°C</i></div>
              <div class="th-room">{{ c.room_name }}</div>
            </div>
          </div>
          <div v-else class="empty">暂无温湿度测点</div>
        </section>
        <section class="bs-panel">
          <div class="panel-title"><SvgIcon name="monitor" />机柜空间利用率（TOP 12）<span class="panel-go link" @click="go('/capacity')">全部机柜 ›</span></div>
          <div ref="chartCab" class="chart-box"></div>
        </section>
        <section class="bs-panel">
          <div class="panel-title"><SvgIcon name="clipboard" />今日巡检进度<span class="panel-go link" @click="go('/inspection')">巡检管理 ›</span></div>
          <div class="insp-list" v-if="todayInsp.length">
            <div class="insp-item clickable" v-for="p in todayInsp" :key="p.id" @click="go('/inspection', { plan_id: p.id })" :title="'点击查看计划 ' + p.name">
              <span class="insp-dot" :class="p.done ? 'done' : 'todo'"></span>
              <span class="insp-name">{{ p.name }}</span>
              <span class="insp-freq">{{ freqText(p.freq) }}</span>
              <span class="insp-st" :class="p.done ? 'done' : 'todo'">{{ p.done ? '已执行' : '待巡检' }}</span>
            </div>
            <div class="insp-sum">{{ inspDone }}/{{ todayInsp.length }} 已完成</div>
          </div>
          <div v-else class="empty">今日无巡检计划 <SvgIcon name="check" /></div>
        </section>
      </div>

      <!-- 右列 -->
      <div class="bs-col">
        <section class="bs-panel">
          <div class="panel-title"><SvgIcon name="bell" />告警分级统计<span class="panel-go link" @click="go('/alert-wall')">盯屏 ›</span></div>
          <div ref="chartAlert" class="chart-box chart-sm"></div>
        </section>
        <section class="bs-panel">
          <div class="panel-title"><SvgIcon name="clock" />最新告警<span class="panel-go link" @click="go('/alert-wall')">全部 ›</span></div>
          <div class="alert-list">
            <div class="alert-item clickable" v-for="a in recentAlerts" :key="a.id" @click="alertClick(a)" :title="'点击' + (a.point_id ? '查看测点' : '进入盯屏')">
              <span class="sev-dot" :class="'sev-' + a.severity"></span>
              <span class="alert-msg">{{ a.message }}</span>
              <span class="alert-time">{{ fmt(a.triggered_at) }}</span>
            </div>
            <div v-if="!recentAlerts.length" class="empty">暂无活跃告警 <SvgIcon name="check" /></div>
          </div>
        </section>
        <section class="bs-panel" v-if="opsWarningsAll.length">
          <div class="panel-title"><SvgIcon name="trend" />趋势与备件预警（24h）<span class="panel-go link" @click="go('/trend-forecast')">预测 ›</span></div>
          <div class="alert-list">
            <div class="alert-item clickable" v-for="(w,i) in opsWarningsRot" :key="w.k" :class="{'rot-on': i===warnRotIdx}" @click="go(w.kind==='trend_forecast'?'/trend-forecast':'/spare-parts')" :title="w.content">
              <span class="sev-dot" :class="w.kind==='trend_forecast'?'sev-warn':'sev-low'"></span>
              <span class="alert-msg">{{ w.title }}</span>
              <span class="alert-time">{{ fmt(w.created_at) }}</span>
            </div>
          </div>
        </section>
        <section class="bs-panel" v-if="spareLow.low_count > 0">
          <div class="panel-title"><SvgIcon name="box" />备件低库存<span class="panel-badge">{{ spareLow.low_count }}</span><span class="panel-go link" @click="go('/spare-parts', { low_only: 1 })">全部低库存 ›</span></div>
          <div class="sp-list">
            <div class="sp-item clickable" v-for="s in spareLowTop" :key="s.id" @click="go('/spare-parts', { low_only: 1, keyword: s.code })" :title="s.name + ' ' + s.code">
              <span class="sp-name">{{ s.name }}</span>
              <span class="sp-code">{{ s.code }}</span>
              <span class="sp-stock" :class="{ zero: s.stock <= 0 }">{{ s.stock }}<i>/{{ s.safety_stock }}</i></span>
            </div>
            <div class="sp-more" v-if="spareLow.low_count > spareLowTop.length">另有 {{ spareLow.low_count - spareLowTop.length }} 项低库存 ›</div>
          </div>
        </section>
        <section class="bs-panel">
          <div class="panel-title"><SvgIcon name="shield" />设备健康度<span class="panel-go link" @click="go('/assets', { health_grade: 'risk' })">风险资产 ›</span></div>
          <div class="ins-health" v-if="ins && ins.health">
            <div class="ih-bar">
              <div v-for="g in healthGrades" :key="g.k" class="ih-seg" :class="'ih-' + g.k" :style="{ width: ihPct(g.k) + '%' }" :title="g.label + ' ' + (ins.health.grades?.[g.k] ?? 0) + ' 台'"></div>
            </div>
            <div class="ih-legend">
              <span v-for="g in healthGrades" :key="g.k" class="ih-lg"><i :class="'ih-' + g.k"></i>{{ g.label }} {{ ins.health.grades?.[g.k] ?? 0 }}</span>
              <span class="ih-avg">均分 {{ ins.health.avg_score ?? '-' }}</span>
            </div>
            <div class="ih-risks" v-if="ins.health.top_risks?.length">
              <div class="ih-ritem clickable" v-for="t in ins.health.top_risks" :key="t.asset_id" @click="go('/assets', { health_grade: 'risk' })" :title="'健康分 ' + t.score">
                <span class="ih-rname">{{ t.name }}</span>
                <span class="ih-rscore" :class="t.grade === 'risk' ? 'v-err' : 'v-warn'">{{ t.score }}</span>
              </div>
            </div>
            <div class="empty" v-else>无亚健康/风险设备 <SvgIcon name="check" /></div>
          </div>
        </section>
        <section class="bs-panel">
          <div class="panel-title"><SvgIcon name="bolt" />PUE 政策合规（分站点）<span class="panel-go link" @click="go('/energy')">能耗总览 ›</span></div>
          <div class="ins-pue" v-if="ins && ins.pue">
            <div class="ip-row clickable" v-for="it in ins.pue.items" :key="it.site_id" @click="go('/energy')" :title="(it.limit ? '当地红线 ≤' + it.limit + ' · ' : '') + '30天均值 ' + (it.avg_pue_30d || '--')">
              <span class="ip-dot" :class="'hc-' + it.status"></span>
              <span class="ip-name">{{ it.site_name }}</span>
              <span class="ip-val" :class="'hc-text-' + it.status">{{ it.latest_pue || it.avg_pue_30d || '--' }}</span>
              <span class="ip-limit" v-if="it.limit">≤{{ it.limit }}</span>
            </div>
            <div class="empty" v-if="!ins.pue.items?.length">暂无站点数据</div>
          </div>
        </section>
        <section class="bs-panel">
          <div class="panel-title"><SvgIcon name="signal" />测点类型分布<span class="panel-go link" @click="go('/monitor')">监控总览 ›</span></div>
          <div ref="chartTypes" class="chart-box chart-sm"></div>
        </section>
        <section class="bs-panel">
          <div class="panel-title"><SvgIcon name="target" />容量利用率雷达<span class="panel-go link" @click="go('/capacity')">容量 ›</span></div>
          <div ref="chartRadar" class="chart-box chart-sm"></div>
        </section>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import * as echarts from 'echarts/core'
import { LineChart, BarChart, PieChart, RadarChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent, DataZoomComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { useSettingsStore } from '../stores/settings'
echarts.use([LineChart, BarChart, PieChart, RadarChart, GridComponent, TooltipComponent, LegendComponent, DataZoomComponent, CanvasRenderer])

const router = useRouter()
const settings = useSettingsStore()

// 穿透导航：投屏(全屏)时新开路由仍留在应用内
function go(path, query) { router.push({ path, query: query || {} }) }
function alertClick(a) {
  if (a.point_id) go('/monitoring-points', { id: a.point_id, detail: '1' })
  else go('/alert-wall')
}

// 主题感知的颜色：从 CSS 变量取，随主题切换实时变化
function tv(name, fallback) {
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return v || fallback
}
function themeColors() {
  const palette = settings.chartPalette
  return {
    palette,
    text: tv('--text', '#dce8f5'),
    muted: tv('--muted', '#8899aa'),
    axis: tv('--border2', 'rgba(140,170,210,.25)'),
    split: tv('--border', 'rgba(140,170,210,.08)'),
    bg: tv('--bg', '#0a1628'),
    card: tv('--card', 'rgba(20,40,70,.5)'),
    tipBg: tv('--card2', 'rgba(10,22,40,.92)'),
  }
}
function axisStyle(c) {
  return { axisLine: { lineStyle: { color: c.axis } }, axisLabel: { color: c.muted, fontSize: 10 }, splitLine: { lineStyle: { color: c.split } } }
}
function tipStyle(c) {
  return { backgroundColor: c.tipBg, borderColor: c.palette[0] + '55', textStyle: { color: c.text, fontSize: 12 } }
}

const d = ref({})
const clock = ref('')
const isFs = ref(false)
const liveOk = ref(true)
const charts = []
const chartPower = ref(null), chartPue = ref(null), chartCab = ref(null), chartAlert = ref(null), chartTypes = ref(null), chartRoomPower = ref(null), chartBreakdown = ref(null), chartRadar = ref(null)
let timer = null, clockTimer = null, ro = null

const TYPE_NAMES = { temp_humidity: '温湿度', hvac: '空调制冷', power_dist: '配电', ups: 'UPS', water_leak: '漏水', smoke: '烟感', cdu: '液冷CDU', leak_rope: '漏液绳' }
const SEV_NAMES = { 1: '提示', 2: '警告', 3: '严重' }

const fmt = t => t ? String(t).replace('T', ' ').slice(5, 16) : '-'
const shortName = n => (n || '').replace(/^(A|B|C) 机房/, '').slice(0, 12)

const kpi = computed(() => ({
  pue: d.value.latest_pue ? d.value.latest_pue.toFixed(2) : null,
  totalKW: d.value.power_latest?.total ? d.value.power_latest.total.toFixed(1) : '--',
  itKW: d.value.power_latest?.it ? d.value.power_latest.it.toFixed(1) : '--',
}))
const pueClass = computed(() => {
  const v = d.value.latest_pue
  if (!v) return ''
  return v <= 1.5 ? 'green' : v <= 1.7 ? 'c1' : 'red'
})
const onlineRate = computed(() => {
  const t = d.value.monitor_total, n = d.value.monitor_normal
  return t ? Math.round((n / t) * 100) : 100
})
const runDays = computed(() => {
  if (!d.value.running_since) return '--'
  return Math.max(0, Math.floor((Date.now() - new Date(d.value.running_since).getTime()) / 86400000))
})
const recentAlerts = computed(() => (d.value.recent_alerts || []).slice(0, 6))
// G批: 与移动端同源的今日巡检/备件低库存
const todayInsp = computed(() => (d.value.today_inspections || []).slice(0, 6))
const inspDone = computed(() => todayInsp.value.filter((p) => p.done).length)
const spareLow = computed(() => d.value.spare_low || { low_count: 0, top: [] })
const spareLowTop = computed(() => (spareLow.value.top || []).slice(0, 5))
const freqText = (f) => ({ daily: '每日', weekly: '每周', monthly: '每月', quarterly: '每季度' }[f] || (f || '周期'))
const np = computed(() => d.value.nameplate || {})
const opsWarningsAll = computed(() => (d.value.ops_warnings || []).map((w, i) => ({ ...w, k: i })))
const warnRotIdx = ref(0)
// V32 轮播: ≤5条时高亮轮换; >5条时滚动窗口(当前项始终可见)
const opsWarningsRot = computed(() => {
  const all = opsWarningsAll.value
  if (all.length <= 5) return all
  const start = Math.max(0, Math.min(warnRotIdx.value - 2, all.length - 5))
  return all.slice(start, start + 5)
})
let warnRotTimer = null
const pb = computed(() => d.value.power_breakdown || {})
const cr = computed(() => d.value.capacity_radar || {})
const fmtK = v => Number(v || 0).toLocaleString('zh-CN', { maximumFractionDigits: 0 })
const thMatrix = computed(() => (d.value.th_matrix || []).slice(0, 12))

function toggleFullscreen() {
  if (!document.fullscreenElement) { document.documentElement.requestFullscreen(); isFs.value = true }
  else { document.exitFullscreen(); isFs.value = false }
}

function mkChart(el) {
  if (!el) return null
  const c = echarts.init(el, null, { renderer: 'canvas' })
  charts.push(c)
  return c
}

function renderCharts() {
  const dd = d.value
  const C = themeColors()
  const AX = axisStyle(C), TP = tipStyle(C)
  const [P0, P1, P2, P3, P4, P5] = C.palette

  // 1. 功率趋势
  if (chartPower.value) {
    const trend = dd.power_trend || []
    const c = chartPower.value._ec || (chartPower.value._ec = mkChart(chartPower.value))
    if (c) c.setOption({
      tooltip: { ...TP, trigger: 'axis' },
      legend: { textStyle: { color: C.muted, fontSize: 10 }, top: 0, right: 0, itemWidth: 14 },
      grid: { left: 44, right: 44, top: 26, bottom: 22 },
      xAxis: { type: 'category', data: trend.map(p => p.t), ...AX, boundaryGap: false },
      yAxis: [
        { type: 'value', name: 'kW', nameTextStyle: { color: C.muted }, ...AX },
        { type: 'value', name: 'PUE', min: 1.2, max: 2.0, nameTextStyle: { color: C.muted }, ...AX, splitLine: { show: false } },
      ],
      series: [
        { name: '总功率', type: 'line', smooth: true, data: trend.map(p => p.total_power), itemStyle: { color: P0 }, areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: P0 + '55' }, { offset: 1, color: P0 + '05' }] } }, symbol: 'none', lineStyle: { width: 2 } },
        { name: 'IT 功率', type: 'line', smooth: true, data: trend.map(p => p.it_power), itemStyle: { color: P1 }, areaStyle: { color: P1 + '1f' }, symbol: 'none', lineStyle: { width: 2 } },
        { name: 'PUE', type: 'line', yAxisIndex: 1, smooth: true, data: trend.map(p => p.pue), itemStyle: { color: P2 }, symbol: 'none', lineStyle: { width: 1.5, type: 'dashed' } },
      ],
    }, true)
  }
  // 2. PUE 走势
  if (chartPue.value) {
    const trend = dd.power_trend || []
    const c = chartPue.value._ec || (chartPue.value._ec = mkChart(chartPue.value))
    if (c) c.setOption({
      tooltip: { ...TP, trigger: 'axis', valueFormatter: v => v?.toFixed(3) },
      grid: { left: 44, right: 12, top: 18, bottom: 22 },
      xAxis: { type: 'category', data: trend.map(p => p.t), ...AX, boundaryGap: false },
      yAxis: { type: 'value', min: v => (v.min - 0.05).toFixed(2), max: v => (v.max + 0.05).toFixed(2), ...AX },
      series: [{ name: 'PUE', type: 'line', smooth: true, data: trend.map(p => p.pue), itemStyle: { color: P5 || P0 }, areaStyle: { color: (P5 || P0) + '26' }, symbol: 'none', lineStyle: { width: 2 }, markLine: { silent: true, symbol: 'none', label: { color: C.muted, fontSize: 10 }, data: [{ yAxis: 1.5, name: '优秀线' }], lineStyle: { color: P1 + '80', type: 'dashed' } } }],
    }, true)
  }
  // 2.5 房间功耗分布
  if (chartRoomPower.value) {
    const roomMap = {}
    ;(dd.cabinets || []).forEach(x => {
      const k = x.room_name || '未分配'
      if (!roomMap[k]) roomMap[k] = { it: 0, rated: 0 }
      roomMap[k].it += x.power_kw || 0
      roomMap[k].rated += x.rated_kw || 0
    })
    const names = Object.keys(roomMap)
    const c = chartRoomPower.value._ec || (chartRoomPower.value._ec = mkChart(chartRoomPower.value))
    if (c) c.setOption({
      tooltip: { ...TP, trigger: 'axis', axisPointer: { type: 'shadow' } },
      legend: { textStyle: { color: C.muted, fontSize: 10 }, top: 0, right: 0, itemWidth: 14 },
      grid: { left: 44, right: 12, top: 24, bottom: 22 },
      xAxis: { type: 'category', data: names, ...AX },
      yAxis: { type: 'value', name: 'kW', nameTextStyle: { color: C.muted }, ...AX },
      series: [
        { name: 'IT 功耗', type: 'bar', data: names.map(k => +roomMap[k].it.toFixed(2)), itemStyle: { color: P0, borderRadius: [3, 3, 0, 0] }, barWidth: 22 },
        { name: '额定容量', type: 'bar', data: names.map(k => +roomMap[k].rated.toFixed(2)), itemStyle: { color: C.split, borderRadius: [3, 3, 0, 0] }, barWidth: 22 },
      ],
    }, true)
  }
  // 3. 机柜空间利用率（点击柱穿透到该机柜容量页）
  if (chartCab.value) {
    const cabs = (dd.cabinets || []).slice(0, 12)
    const c = chartCab.value._ec || (chartCab.value._ec = mkChart(chartCab.value))
    if (c) {
      c.setOption({
        tooltip: { ...TP, trigger: 'axis', axisPointer: { type: 'shadow' } },
        legend: { textStyle: { color: C.muted, fontSize: 10 }, top: 0, right: 0, itemWidth: 14 },
        grid: { left: 60, right: 16, top: 24, bottom: 8 },
        xAxis: { type: 'value', max: 42, ...AX },
        yAxis: { type: 'category', data: cabs.map(x => x.room_name + '·' + x.name).slice().reverse(), ...AX },
        series: [
          { name: '已用 U', type: 'bar', stack: 'u', data: cabs.map(x => x.used_u).reverse(), itemStyle: { color: P0 }, barWidth: 10, cursor: 'pointer' },
          { name: '剩余 U', type: 'bar', stack: 'u', data: cabs.map(x => x.total_u - x.used_u).reverse(), itemStyle: { color: C.split, borderRadius: [0, 2, 2, 0] }, barWidth: 10 },
        ],
      }, true)
      c.off('click')
      c.on('click', p => {
        const cab = cabs.slice().reverse()[p.dataIndex]
        if (cab) go('/capacity', { cabinet_id: cab.id })
      })
    }
  }
  // 4. 告警分级玫瑰图（点击扇区穿透到盯屏按级别过滤）
  if (chartAlert.value) {
    const sev = dd.alert_severity || { 1: 0, 2: 0, 3: 0 }
    const data = [1, 2, 3].map(k => ({ name: SEV_NAMES[k], value: sev[k] || 0 }))
    const c = chartAlert.value._ec || (chartAlert.value._ec = mkChart(chartAlert.value))
    if (c) {
      c.setOption({
        tooltip: { ...TP },
        legend: { textStyle: { color: C.muted, fontSize: 10 }, bottom: 0, itemWidth: 14 },
        series: [{
          type: 'pie', roseType: 'radius', radius: ['22%', '62%'], center: ['50%', '44%'],
          data, label: { color: C.muted, fontSize: 11, formatter: '{b}\n{c} 条' },
          itemStyle: { borderColor: C.bg, borderWidth: 2 },
          color: [P0, P2, '#f56c6c'],
        }],
      }, true)
      c.off('click')
      c.on('click', p => {
        const sevOf = { '提示': 1, '警告': 2, '严重': 3 }
        go('/alert-wall', { severity: sevOf[p.name] || 0 })
      })
    }
  }
  // 5. 测点类型分布（点击柱穿透到测点管理按类型过滤）
  if (chartTypes.value) {
    const types = dd.point_types || []
    const c = chartTypes.value._ec || (chartTypes.value._ec = mkChart(chartTypes.value))
    if (c) {
      c.setOption({
        tooltip: { ...TP, trigger: 'axis', axisPointer: { type: 'shadow' } },
        grid: { left: 8, right: 24, top: 8, bottom: 8, containLabel: true },
        xAxis: { type: 'value', ...AX },
        yAxis: { type: 'category', data: types.map(t => TYPE_NAMES[t.p_type] || t.p_type).reverse(), ...AX },
        series: [{ type: 'bar', data: types.map(t => t.count).reverse(), barWidth: 10, itemStyle: { color: P4 || P0, borderRadius: [0, 4, 4, 0] }, label: { show: true, position: 'right', color: C.muted, fontSize: 10 }, cursor: 'pointer' }],
      }, true)
      c.off('click')
      c.on('click', p => {
        const t = types.slice().reverse()[p.dataIndex]
        if (t) go('/monitoring-points', { type: t.p_type })
      })
    }
  }
  // 6. V21-③ 功率构成（横向堆叠条：IT/制冷/配电损耗/辅助）
  if (chartBreakdown.value) {
    const b = pb.value
    const c = chartBreakdown.value._ec || (chartBreakdown.value._ec = mkChart(chartBreakdown.value))
    if (c) {
      const cats = ['功率 kW']
      const segs = [
        { name: 'IT 设备', v: b.it || 0, color: P0 },
        { name: '制冷', v: b.cooling || 0, color: '#26c6da' },
        { name: '配电损耗', v: b.electrical || 0, color: '#ffa726' },
        { name: '辅助', v: b.auxiliary || 0, color: P4 || '#8a93a6' },
      ]
      c.setOption({
        tooltip: { ...TP, trigger: 'axis', axisPointer: { type: 'shadow' }, valueFormatter: v => (+v).toFixed(1) + ' kW' },
        legend: { textStyle: { color: C.muted, fontSize: 10 }, top: 0, itemWidth: 12 },
        grid: { left: 8, right: 16, top: 24, bottom: 8, containLabel: true },
        xAxis: { type: 'value', ...AX },
        yAxis: { type: 'category', data: cats, ...AX, axisLabel: { show: false } },
        series: segs.map(s => ({ name: s.name, type: 'bar', stack: 'p', data: [+s.v.toFixed(1)], itemStyle: { color: s.color }, barWidth: 26,
          label: { show: s.v > 0, position: 'inside', color: '#fff', fontSize: 10, formatter: () => s.v.toFixed(0) } })),
      }, true)
    }
  }
  // 7. V21-③ 容量利用率雷达（市电/UPS/制冷/机柜）
  if (chartRadar.value) {
    const r = cr.value
    const c = chartRadar.value._ec || (chartRadar.value._ec = mkChart(chartRadar.value))
    if (c) {
      const dims = [
        { name: '市电', max: 100 }, { name: 'UPS', max: 100 },
        { name: '制冷', max: 100 }, { name: '机柜', max: 100 },
      ]
      const vals = [r.utility || 0, r.ups || 0, r.cooling || 0, r.cabinet || 0]
      c.setOption({
        tooltip: { ...TP, valueFormatter: v => (+v).toFixed(1) + '%' },
        radar: { indicator: dims, radius: '62%', center: ['50%', '52%'],
          axisName: { color: C.muted, fontSize: 11 },
          splitLine: { lineStyle: { color: C.split } }, splitArea: { areaStyle: { color: ['transparent'] } },
          axisLine: { lineStyle: { color: C.axis } } },
        series: [{ type: 'radar', data: [{ value: vals, name: '利用率%',
          areaStyle: { color: P0 + '33' }, lineStyle: { color: P0, width: 2 }, itemStyle: { color: P0 } }],
          label: { show: true, color: C.text, fontSize: 10, formatter: p => p.value.toFixed(0) + '%' } }],
      }, true)
      c.off('click')
      c.on('click', () => go('/capacity'))
    }
  }
}

const comp = ref(null)
async function loadComp() {
  try { const r = await api.get('/energy/compliance', { site_id: 1 }); comp.value = r.data || null } catch (e) { /* silent */ }
}

// V65 B1-3: 风险与洞察（复用 /dashboard/insights，随大屏 15s 轮询刷新）
const ins = ref(null)
const healthGrades = [
  { k: 'healthy', label: '健康' }, { k: 'attention', label: '关注' },
  { k: 'sub_healthy', label: '亚健康' }, { k: 'risk', label: '风险' },
]
function ihPct(k) {
  const t = ins.value?.health?.total || 0
  if (!t) return 0
  return Math.round(((ins.value?.health?.grades?.[k] ?? 0) / t) * 100)
}
async function loadInsights() {
  try { const r = await api.get('/dashboard/insights'); ins.value = r.data || null } catch (e) { ins.value = null }
}
async function refresh() {
  try {
    const r = await api.get('/big-screen')
    d.value = r.data || {}
    loadComp()
    loadInsights()
    liveOk.value = true
    await nextTick()
    renderCharts()
  } catch (e) { liveOk.value = false; console.warn('big-screen refresh fail', e) }
}

function tickClock() {
  const n = new Date()
  clock.value = n.toLocaleDateString('zh-CN') + ' ' + n.toLocaleTimeString('zh-CN', { hour12: false })
}

// 主题切换：整页重渲（颜色全部来自 CSS 变量 + 主题色板）
function onThemeChange() {
  charts.forEach(c => c.dispose())
  charts.length = 0
  ;[chartPower, chartPue, chartCab, chartAlert, chartTypes, chartRoomPower, chartBreakdown, chartRadar].forEach(r => { if (r.value) r.value._ec = null })
  nextTick(renderCharts)
}
watch(() => settings.theme, onThemeChange)

onMounted(async () => {
  tickClock()
  clockTimer = setInterval(tickClock, 1000)
  await refresh()
  timer = setInterval(refresh, 15000) // 模拟器 30s 一轮，大屏 15s 刷新保证新鲜
  warnRotTimer = setInterval(() => {   // V32 预警轮播: 5s 轮换高亮/滚动窗口
    const n = opsWarningsAll.value.length
    if (n > 1) warnRotIdx.value = (warnRotIdx.value + 1) % n
  }, 5000)
  ro = new ResizeObserver(() => charts.forEach(c => c && c.resize()))
  ;[chartPower, chartPue, chartCab, chartAlert, chartTypes, chartRoomPower, chartBreakdown, chartRadar].forEach(r => r.value && ro.observe(r.value))
  document.addEventListener('dcim-theme-change', onThemeChange)
})
onUnmounted(() => {
  clearInterval(timer); clearInterval(clockTimer); clearInterval(warnRotTimer)
  ro && ro.disconnect()
  document.removeEventListener('dcim-theme-change', onThemeChange)
  charts.forEach(c => c && c.dispose())
  if (document.fullscreenElement) document.exitFullscreen()
})
</script>

<style scoped>
/* 颜色全部走主题 CSS 变量：浅色/深色主题自动适配 */
.bscreen { min-height: 100vh; background: var(--bg); color: var(--text); padding: 14px 18px; font-family: 'Arial', 'Microsoft YaHei', sans-serif; display: flex; flex-direction: column; gap: 12px; }
.bs-header { display: flex; align-items: center; gap: 16px; padding-bottom: 10px; border-bottom: 1px solid var(--border); }
.bs-title { font-size: 22px; margin: 0; flex: 1; background: linear-gradient(90deg, var(--primary, #409eff), var(--accent, #36cfc9)); -webkit-background-clip: text; -webkit-text-fill-color: transparent; letter-spacing: 1px; }
.bs-header-right { display: flex; align-items: center; gap: 14px; font-size: 13px; color: var(--muted); }
.bs-live { color: #f56c6c; font-size: 12px; }
.bs-live.ok { color: #67c23a; }
.bs-running b { color: var(--accent); font-size: 15px; }
.bs-fs-btn { background: transparent; color: var(--accent); border: 1px solid var(--border2, var(--border)); padding: 5px 14px; border-radius: 6px; cursor: pointer; font-size: 13px; }
.bs-fs-btn:hover { border-color: var(--accent); }

/* ===== V43 英雄指标（视觉中心）===== */
.bs-heroes { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.hero-kpi { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 14px 22px;
  border-radius: 12px; background:
    linear-gradient(135deg, color-mix(in srgb, var(--accent) 14%, var(--card)), color-mix(in srgb, var(--card) 92%, transparent));
  border: 1px solid color-mix(in srgb, var(--accent) 45%, var(--border));
  box-shadow: 0 0 22px color-mix(in srgb, var(--accent) 12%, transparent), inset 0 1px 0 color-mix(in srgb, var(--accent) 18%, transparent);
  transition: transform .15s, box-shadow .2s; position: relative; overflow: hidden; }
.hero-kpi::after { content: ''; position: absolute; top: 0; left: 0; right: 0; height: 2px;
  background: linear-gradient(90deg, transparent, var(--accent), transparent); opacity: .7; }
.hero-kpi.clickable:hover { transform: translateY(-2px); box-shadow: 0 0 30px color-mix(in srgb, var(--accent) 25%, transparent); }
.hk-left { display: flex; flex-direction: column; }
.hk-value { font-size: 46px; font-weight: 800; line-height: 1.1; font-variant-numeric: tabular-nums; letter-spacing: 1px;
  text-shadow: 0 0 18px color-mix(in srgb, var(--accent) 35%, transparent); }
.hk-value i { font-style: normal; font-size: 16px; color: var(--muted); font-weight: 400; }
.hk-value.green { color: var(--success, #67c23a); }
.hk-value.c1 { color: var(--primary, #409eff); }
.hk-value.red { color: var(--danger, #f56c6c); }
.hk-right { text-align: right; display: flex; flex-direction: column; gap: 4px; }
.hk-sub { font-size: 12px; color: var(--muted); }
.hk-go { font-size: 12px; color: var(--accent); }

.bs-kpis { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; }
.kpi-sm .kpi-value { font-size: 22px; }
.kpi-alerting { border-color: color-mix(in srgb, var(--danger, #f56c6c) 55%, var(--border));
  box-shadow: 0 0 16px color-mix(in srgb, var(--danger, #f56c6c) 20%, transparent); animation: alert-breathe 2.4s infinite; }
@keyframes alert-breathe { 50% { box-shadow: 0 0 26px color-mix(in srgb, var(--danger, #f56c6c) 38%, transparent); } }
.hk-comp { font-size: 12px; margin-top: 4px; padding: 2px 8px; border-radius: 8px; display: inline-block; }
.hk-comp.hc-ok { background: rgba(0,230,118,.12); color: #00e676; }
.hk-comp.hc-edge { background: rgba(255,180,84,.14); color: #ffb454; }
.hk-comp.hc-over { background: rgba(255,82,82,.16); color: #ff5252; animation: compBlink 1.6s infinite; }
.hk-comp.hc-nodata, .hk-comp.hc-unknown { background: rgba(140,140,140,.12); color: #9aa; }

/* ===== V65 B1-3 风险与洞察面板 ===== */
.ih-bar { display: flex; height: 10px; border-radius: 5px; overflow: hidden; background: var(--border); margin: 6px 0 8px; }
.ih-seg { height: 100%; transition: width .4s; }
.ih-healthy { background: #00e676; }
.ih-attention { background: #40c4ff; }
.ih-sub_healthy { background: #ffb454; }
.ih-risk { background: #ff5252; }
.ih-legend { display: flex; flex-wrap: wrap; gap: 4px 12px; font-size: 12px; color: var(--muted); }
.ih-lg { display: inline-flex; align-items: center; gap: 4px; }
.ih-lg i { width: 8px; height: 8px; border-radius: 2px; display: inline-block; }
.ih-avg { margin-left: auto; color: var(--text); }
.ih-risks { margin-top: 8px; display: flex; flex-direction: column; gap: 4px; }
.ih-ritem { display: flex; justify-content: space-between; align-items: center; font-size: 12px; padding: 3px 8px; border-radius: 6px; background: color-mix(in srgb, var(--card) 60%, transparent); border: 1px solid var(--border); }
.ih-ritem:hover { border-color: var(--accent); }
.ih-rname { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; }
.ih-rscore { font-weight: 700; flex-shrink: 0; margin-left: 8px; }
.ih-rscore.v-err { color: #ff5252; }
.ih-rscore.v-warn { color: #ffb454; }
.ip-row { display: flex; align-items: center; gap: 8px; font-size: 12px; padding: 4px 8px; border-radius: 6px; }
.ip-row.clickable:hover { background: color-mix(in srgb, var(--accent) 8%, transparent); }
.ip-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.ip-dot.hc-ok { background: #00e676; }
.ip-dot.hc-edge { background: #ffb454; }
.ip-dot.hc-over { background: #ff5252; animation: compBlink 1.6s infinite; }
.ip-dot.hc-nodata, .ip-dot.hc-unknown { background: #9aa; }
.ip-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ip-val { font-weight: 700; font-size: 13px; }
.ip-val.hc-text-ok { color: #00e676; }
.ip-val.hc-text-edge { color: #ffb454; }
.ip-val.hc-text-over { color: #ff5252; }
.ip-val.hc-text-nodata, .ip-val.hc-text-unknown { color: var(--muted); }
.ip-limit { color: var(--muted); flex-shrink: 0; }
@keyframes compBlink { 0%,100% { opacity: 1 } 50% { opacity: .55 } }
.bs-nameplate { display: flex; flex-wrap: wrap; gap: 6px 18px; align-items: center; padding: 7px 14px; border-radius: 8px;
  background: color-mix(in srgb, var(--card, rgba(20,40,70,.5)) 60%, transparent); border: 1px solid var(--border, rgba(140,170,210,.12));
  font-size: 12px; color: var(--muted, #8899aa); cursor: pointer; }
.bs-nameplate:hover { border-color: var(--accent, #409eff); }
.np-item b { color: var(--text, #dce8f5); font-size: 13px; margin-right: 4px; }
.np-item::before { content: '·'; margin-right: 6px; opacity: .5; }
.np-item:first-child::before { content: ''; margin: 0; }
.kpi { background: var(--card); border: 1px solid var(--border); border-radius: 10px; padding: 12px 14px; text-align: center; transition: border-color .3s, transform .15s; }
.clickable { cursor: pointer; }
.clickable:hover { border-color: var(--accent); transform: translateY(-1px); }
.kpi-label { font-size: 12px; color: var(--muted); margin-bottom: 4px; }
.kpi-value { font-size: 28px; font-weight: 800; color: var(--text); line-height: 1.25; font-variant-numeric: tabular-nums; }
.kpi-value i { font-style: normal; font-size: 14px; color: var(--muted); font-weight: 400; }
.kpi-value.green { color: var(--success, #67c23a); }
.kpi-value.c1 { color: var(--primary, #409eff); }
.kpi-value.red { color: var(--danger, #f56c6c); }
.kpi-sub { font-size: 11px; color: var(--muted); margin-top: 2px; }
.kpi-sub:hover { color: var(--accent); }

/* V43: 中列是视觉核心（温湿度矩阵+机柜利用率），加宽并强调 */
.bs-main { flex: 1; display: grid; grid-template-columns: 1fr 1.4fr 1fr; gap: 12px; min-height: 0; }
.bs-col { display: flex; flex-direction: column; gap: 12px; min-width: 0; }
.bs-col.bs-col-core .bs-panel { border-color: color-mix(in srgb, var(--accent) 28%, var(--border)); }
.bs-col.bs-col-core .panel-title { font-size: 15px; }
.bs-panel { background: var(--card); border: 1px solid var(--border); border-radius: 10px; padding: 12px 14px; flex: 1; display: flex; flex-direction: column; min-height: 0; transition: border-color .2s; }
.clickable-panel { cursor: pointer; }
.clickable-panel:hover { border-color: var(--accent); }
.alert-item.rot-on { background: rgba(255, 167, 38, .1); border-left: 2px solid #ffa726; }
.sev-dot.sev-warn { background: #ffa726; box-shadow: 0 0 6px rgba(255,167,38,.8); }
.sev-dot.sev-low { background: #40c4ff; box-shadow: 0 0 6px rgba(64,196,255,.8); }
.panel-title { font-size: 14px; font-weight: 700; color: var(--text2, var(--text)); margin-bottom: 8px; display: flex; align-items: center; gap: 6px; }
.panel-go { margin-left: auto; font-size: 12px; font-weight: 400; color: var(--muted); }
.panel-go.link { cursor: pointer; }
.panel-go.link:hover { color: var(--accent); }
.chart-box { flex: 1; min-height: 150px; }
.chart-lg { min-height: 220px; }
.chart-sm { min-height: 130px; }

.th-matrix { flex: 1; display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 8px; align-content: start; overflow-y: auto; }
.th-cell { border-radius: 8px; padding: 8px 10px; border: 1px solid var(--border); background: var(--card2, var(--card)); transition: transform .15s; }
.th-cell.clickable:hover { transform: scale(1.03); border-color: var(--accent); }
.th-s1 { border-color: color-mix(in srgb, var(--success, #67c23a) 40%, transparent); }
.th-s2 { background: color-mix(in srgb, var(--danger, #f56c6c) 10%, var(--card)); border-color: var(--danger, #f56c6c); animation: pulse 1.5s infinite; }
.th-s0 { opacity: .55; }
.th-point { font-size: 11px; color: var(--muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.th-value { font-size: 22px; font-weight: 800; color: var(--text); font-variant-numeric: tabular-nums; }
.th-value i { font-style: normal; font-size: 12px; color: var(--muted); }
.th-room { font-size: 10px; color: var(--muted); }
@keyframes pulse { 50% { border-color: color-mix(in srgb, var(--danger, #f56c6c) 90%, transparent); } }

.alert-list { flex: 1; overflow-y: auto; display: flex; flex-direction: column; gap: 2px; }
.alert-item { display: flex; align-items: center; gap: 8px; padding: 7px 4px; border-bottom: 1px solid var(--border); font-size: 12px; border-radius: 4px; }
.alert-item.clickable:hover { background: var(--card2, rgba(64,158,255,.06)); }
.sev-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.sev-dot.sev-3 { background: var(--danger, #f56c6c); box-shadow: 0 0 6px var(--danger, #f56c6c); }
.sev-dot.sev-2 { background: var(--warning, #e6a23c); }
.sev-dot.sev-1 { background: var(--primary, #409eff); }
.alert-msg { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text2, var(--text)); }
.alert-time { color: var(--muted); font-size: 10px; flex-shrink: 0; }
.empty { color: var(--muted); text-align: center; padding: 20px; font-size: 12px; }

/* ---- G批: 今日巡检进度 + 备件低库存 ---- */
.insp-list, .sp-list { display: flex; flex-direction: column; }
.insp-item { display: flex; align-items: center; gap: 8px; padding: 6px 4px; border-bottom: 1px solid var(--border); font-size: 12px; }
.insp-item.clickable:hover { background: var(--card2, rgba(64,158,255,.06)); }
.insp-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.insp-dot.done { background: var(--success, #67c23a); box-shadow: 0 0 6px var(--success, #67c23a); }
.insp-dot.todo { background: var(--warning, #e6a23c); }
.insp-name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text2, var(--text)); }
.insp-freq { color: var(--muted); font-size: 10px; flex-shrink: 0; }
.insp-st { font-size: 10px; flex-shrink: 0; padding: 1px 8px; border-radius: 8px; }
.insp-st.done { color: var(--success, #67c23a); border: 1px solid var(--success, #67c23a); }
.insp-st.todo { color: var(--warning, #e6a23c); border: 1px solid var(--warning, #e6a23c); }
.insp-sum { text-align: right; font-size: 11px; color: var(--muted); padding-top: 6px; }
.panel-badge { margin-left: 6px; background: var(--danger, #f56c6c); color: #fff; font-size: 10px; padding: 1px 8px; border-radius: 10px; font-weight: 700; }
.sp-item { display: flex; align-items: center; gap: 8px; padding: 6px 4px; border-bottom: 1px solid var(--border); font-size: 12px; }
.sp-item.clickable:hover { background: var(--card2, rgba(64,158,255,.06)); }
.sp-name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text2, var(--text)); }
.sp-code { color: var(--muted); font-size: 10px; flex-shrink: 0; font-family: monospace; }
.sp-stock { font-weight: 700; color: var(--warning, #e6a23c); flex-shrink: 0; }
.sp-stock.zero { color: var(--danger, #f56c6c); }
.sp-stock i { font-style: normal; font-weight: 400; color: var(--muted); font-size: 10px; }
.sp-more { font-size: 11px; color: var(--muted); padding-top: 6px; cursor: pointer; }

@media (max-width: 1280px) {
  .bs-heroes { grid-template-columns: 1fr; }
  .hk-value { font-size: 36px; }
  .bs-kpis { grid-template-columns: repeat(2, 1fr); }
  .bs-main { grid-template-columns: 1fr 1fr; }
  .bs-col:last-child { grid-column: span 2; display: grid; grid-template-columns: 1fr 1fr 1fr; }
}
@media (max-width: 900px) {
  .bscreen { padding: 10px; }
  .bs-title { font-size: 16px; }
  .bs-kpis { grid-template-columns: repeat(2, 1fr); gap: 8px; }
  .kpi-value { font-size: 22px; }
  .hk-value { font-size: 30px; }
  .bs-main { grid-template-columns: 1fr; }
  .bs-col:last-child { grid-column: auto; display: flex; flex-direction: column; }
  .chart-box { min-height: 200px; }
  .th-matrix { grid-template-columns: repeat(2, 1fr); }
}
</style>
