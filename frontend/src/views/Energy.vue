<template>
  <div>
    <div class="page-head">
      <h1>能耗总览</h1>
      <div class="toolbar">
        <select class="input" style="width:180px" v-model.number="siteId" @change="load">
          <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
        </select>
        <div class="seg">
          <button v-for="d in [7, 30, 90]" :key="d" class="seg-btn" :class="{ on: days === d }" @click="days = d; load()">近 {{ d }} 天</button>
        </div>
        <span style="flex:1"></span>
        <button class="btn" @click="openPolicyLib"><SvgIcon name="book" />政策库</button>
        <button class="btn" @click="exportCSV">导出 CSV</button>
        <span class="muted" style="font-size:12px">电网排放因子 0.5703 kgCO₂/kWh · PUE 采样每小时</span>
      </div>
    </div>

    <!-- KPI 卡 -->
    <div class="kpi-row" v-if="kpi">
      <div class="kpi-card">
        <div class="kpi-label">IT 装机功率</div>
        <div class="kpi-val">{{ kpi.it_power_kw }} <small>kW</small></div>
        <div class="kpi-sub">额定容量 {{ kpi.rated_power_kw }} kW · 利用率 {{ kpi.cap_usage_pct }}%</div>
        <div class="kpi-bar"><i :style="{ width: Math.min(kpi.cap_usage_pct, 100) + '%' }"></i></div>
      </div>
      <div class="kpi-card accent">
        <div class="kpi-label">最新 PUE</div>
        <div class="kpi-val">{{ kpi.latest_pue }}</div>
        <div class="kpi-sub">期间均值 {{ kpi.avg_pue }}（近 {{ days }} 天）
          <span v-if="kpi.pue_delta_pct" class="delta" :class="kpi.pue_delta_pct>0?'up':'down'">{{ kpi.pue_delta_pct>0?'▲':'▼' }} {{ Math.abs(kpi.pue_delta_pct) }}%</span>
        </div>
      </div>
      <div class="kpi-card">
        <div class="kpi-label">期间能耗</div>
        <div class="kpi-val">{{ fmt(kpi.energy_kwh) }} <small>kWh</small></div>
        <div class="kpi-sub">总功率 {{ kpi.latest_total_kw }} kW · IT {{ kpi.latest_it_kw }} kW
          <span v-if="kpi.energy_delta_pct" class="delta" :class="kpi.energy_delta_pct>0?'up':'down'">{{ kpi.energy_delta_pct>0?'▲':'▼' }} {{ Math.abs(kpi.energy_delta_pct) }}%</span>
        </div>
      </div>
      <div class="kpi-card">
        <div class="kpi-label">碳排放估算</div>
        <div class="kpi-val">{{ fmt(kpi.co2_kg) }} <small>kg</small></div>
        <div class="kpi-sub">≈ {{ (kpi.co2_kg / 1000).toFixed(1) }} 吨 CO₂ · 因子 {{ kpi.co2_factor }}</div>
      </div>
      <div class="kpi-card">
        <div class="kpi-label">电费估算</div>
        <div class="kpi-val">¥{{ fmt(kpi.cost_yuan) }}</div>
        <div class="kpi-sub">电价 {{ kpi.price_kwh }} 元/kWh · 上期 {{ fmt(kpi.prev_energy_kwh) }} kWh</div>
      </div>
      <div class="kpi-card green" v-if="prediction">
        <div class="kpi-label">节能潜力（PUE −0.1）</div>
        <div class="kpi-val">−{{ fmt(prediction.saving_kwh) }} <small>kWh</small></div>
        <div class="kpi-sub">目标 PUE {{ prediction.target_pue }} · 减碳 {{ fmt(prediction.saving_co2_kg) }} kg</div>
      </div>
    </div>

    <!-- V49 能效合规卡片 -->
    <div class="card comp-card" v-if="comp" :class="'comp-' + comp.status">
      <div class="comp-hd">
        <span class="comp-title"><SvgIcon name="shield" /> 能效合规（{{ comp.site_name }}）</span>
        <span class="comp-badge" :class="comp.status">{{ compStatusText }}</span>
        <span style="flex:1"></span>
      </div>
      <div v-if="comp.status === 'unknown'" class="comp-unknown">
        {{ comp.message }}
      </div>
      <div v-else-if="comp.policy" class="comp-body">
        <div class="comp-metrics">
          <div class="comp-m">
            <div class="comp-m-label">当前 PUE</div>
            <div class="comp-m-val" :style="{ color: compColor }">{{ comp.latest_pue || comp.avg_pue_30d || '—' }}</div>
            <div class="comp-m-sub">近30天均值 {{ comp.avg_pue_30d || '—' }}</div>
          </div>
          <div class="comp-m">
            <div class="comp-m-label">当地红线（{{ comp.policy.province }}·新建大型）</div>
            <div class="comp-m-val">{{ comp.policy.new_large_limit || '—' }}</div>
            <div class="comp-m-sub">余量 <b :style="{ color: compColor }">{{ comp.margin > 0 ? '+' : '' }}{{ comp.margin }}</b> · 全国枢纽均值 {{ comp.national_avg }}</div>
          </div>
          <div class="comp-m">
            <div class="comp-m-label">年化电量估算</div>
            <div class="comp-m-val">{{ fmt(comp.annual_kwh) }} <small>kWh</small></div>
            <div class="comp-m-sub">{{ comp.policy.region }} · 匹配方式 {{ comp.match_by === 'province' ? '站点省份' : '地址推断' }}</div>
          </div>
          <div class="comp-m" :class="{ 'comp-m-warn': comp.surcharge_annual > 0 }">
            <div class="comp-m-label">差别电价敞口（年）</div>
            <div class="comp-m-val">{{ comp.surcharge_annual > 0 ? '¥' + fmt(comp.surcharge_annual) : '¥0' }}</div>
            <div class="comp-m-sub">{{ comp.surcharge_hit || '未触发加价规则' }}</div>
          </div>
        </div>
        <div class="comp-notes" v-if="comp.policy.subsidy || comp.policy.existing_target">
          <div v-if="comp.policy.subsidy" class="comp-note"><SvgIcon name="bolt" /> <b>补贴政策：</b>{{ comp.policy.subsidy }}</div>
          <div v-if="comp.policy.existing_target" class="comp-note"><SvgIcon name="clock" /> <b>存量改造：</b>{{ comp.policy.existing_target }}</div>
          <div v-if="comp.policy.features" class="comp-note"><SvgIcon name="info" /> <b>特色要求：</b>{{ comp.policy.features }}</div>
        </div>
      </div>
      <!-- V73/B3-3 主动合规告警状态 -->
      <div class="comp-alertbox" v-if="comp.pue_notify">
        <div class="comp-alert-hd">
          <span class="comp-alert-title"><SvgIcon name="bell" /> 主动合规告警</span>
          <span class="badge" :class="comp.pue_notify.enabled ? 'ok' : 'muted'">{{ comp.pue_notify.enabled ? '已启用' : '已关闭' }}</span>
          <span class="muted small">连续 {{ comp.pue_notify.sustain_hours }} 小时越线触发 · 同状态 24 小时去重</span>
        </div>
        <div v-if="comp.pue_notify.active" class="comp-alert-active">
          <span class="comp-alert-dot"></span>
          <span class="comp-alert-msg">{{ comp.pue_notify.active.message }}</span>
          <span class="muted small">{{ comp.pue_notify.active.triggered_at }}</span>
          <span style="flex:1"></span>
          <button class="btn btn-mini" @click="$router.push('/monitor')">查看告警</button>
        </div>
        <div v-else class="comp-alert-idle muted small">
          当前无活跃合规告警 · 近30天 {{ comp.pue_notify.recent_count }} 条<span v-if="comp.pue_notify.last_at"> · 最近 {{ comp.pue_notify.last_at }}</span><span v-if="comp.pue_notify.last_notified_at"> · 最近通知 {{ comp.pue_notify.last_notified_at }}</span>
        </div>
      </div>
    </div>

    <!-- V57 政策库抽屉（单抽屉两态：列表 ↔ 编辑，不再双层叠加） -->
    <div v-if="policyLib.show" class="drawer-mask" @click.self="policyLib.show = false">
      <div class="drawer" style="width:860px">
        <template v-if="!policyForm.show">
          <div class="drawer-head">
            <h3><SvgIcon name="book" /> 各省 PUE 政策库（{{ policies.length }} 条）</h3>
            <button class="btn btn-mini ic-btn" title="关闭" @click="policyLib.show = false"><SvgIcon name="x" /></button>
          </div>
          <div class="drawer-body">
            <div class="toolbar" style="margin-bottom:10px">
              <input class="input" style="width:160px" v-model="policyLib.keyword" placeholder="搜索省份…" @input="loadPolicies" />
              <select class="input" style="width:140px" v-model="policyLib.region" @change="loadPolicies">
                <option value="">全部区域</option>
                <option v-for="rg in ['东部枢纽','西部枢纽','中部承接','东北拓展','港澳台']" :key="rg" :value="rg">{{ rg }}</option>
              </select>
              <span style="flex:1"></span>
            </div>
            <div class="table-wrap">
              <table class="tbl">
                <thead><tr><th>省份</th><th>区域</th><th>新建大型限额</th><th>差别电价(元/度)</th><th>补贴/激励</th><th style="width:90px">操作</th></tr></thead>
                <tbody>
                  <tr v-for="p in policies" :key="p.id">
                    <td><b>{{ p.province }}</b></td>
                    <td><span class="tag">{{ p.region }}</span></td>
                    <td><b :style="{ color: p.new_large_limit <= 1.25 ? 'var(--success)' : 'inherit' }">≤{{ p.new_large_limit }}</b></td>
                    <td class="muted" style="font-size:12px">{{ p.surcharge_rules ? p.surcharge_rules.replaceAll(';', ' ') : '—' }}</td>
                    <td class="muted" style="font-size:12px;max-width:260px">{{ p.subsidy || '—' }}</td>
                    <td>
                      <button class="btn btn-mini" @click="editPolicy(p)"><SvgIcon name="pen" />编辑</button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
          <div class="drawer-foot" style="display:flex;align-items:center;gap:10px">
            <span class="muted" style="font-size:12px">数据来源：发改委专项行动计划等 · 截至 2026-10</span>
            <span style="flex:1"></span>
            <button class="btn" @click="policyLib.show = false">关闭</button>
          </div>
        </template>
        <template v-else>
          <div class="drawer-head">
            <button class="btn btn-mini" @click="policyForm.show = false">‹ 返回列表</button>
            <h3>编辑政策 — {{ policyForm.province }}</h3>
            <button class="btn btn-mini ic-btn" title="返回列表" @click="policyForm.show = false"><SvgIcon name="x" /></button>
          </div>
          <div class="drawer-body">
            <div class="form-grid">
              <label>新建大型/超大型限额<input class="input" type="number" step="0.01" v-model.number="policyForm.new_large_limit" /></label>
              <label>新建中小型限额<input class="input" type="number" step="0.01" v-model.number="policyForm.new_small_limit" /></label>
              <label class="full">差别电价规则（格式 区间:加价元/度，分号分隔）<input class="input" v-model="policyForm.surcharge_rules" placeholder="如 1.35-1.7:0.2;>1.7:0.5" /></label>
              <label class="full">存量改造目标<input class="input" v-model="policyForm.existing_target" /></label>
              <label class="full">补贴/激励<input class="input" v-model="policyForm.subsidy" /></label>
              <label class="full">特色要求<input class="input" v-model="policyForm.features" /></label>
            </div>
            <div v-if="policyForm.err" class="form-err">{{ policyForm.err }}</div>
          </div>
          <div class="drawer-foot">
            <button class="btn" @click="policyForm.show = false">取消</button>
            <button class="btn btn-primary" @click="savePolicy">保存</button>
          </div>
        </template>
      </div>
    </div>

    <!-- 图表区 -->
    <div class="en-grid">
      <div class="card en-card wide">
        <div class="card-hd">PUE 实时曲线（近 72 个采样点）</div>
        <div class="chart-box" ref="pueRawEl"></div>
      </div>
      <div class="card en-card">
        <div class="card-hd">PUE 日趋势（均值/峰值）</div>
        <div class="chart-box" ref="pueTrendEl"></div>
      </div>
      <div class="card en-card">
        <div class="card-hd">日能耗 / 日电费</div>
        <div class="chart-box" ref="dailyEl"></div>
      </div>
      <div class="card en-card">
        <div class="card-hd">房间功耗分布</div>
        <div class="chart-box" ref="roomEl"></div>
      </div>
      <div class="card en-card">
        <div class="card-hd">机柜功耗 TOP10</div>
        <div class="chart-box" ref="topEl"></div>
      </div>
      <div class="card en-card">
        <div class="card-hd">机柜负载率分布</div>
        <div class="chart-box" ref="bucketEl"></div>
      </div>
    </div>

    <!-- 机柜排行表 -->
    <div class="card" style="margin-top:16px">
      <div class="card-hd">机柜功耗明细（TOP10）</div>
      <table class="table">
        <thead><tr><th>机柜</th><th>房间</th><th>功耗 (kW)</th><th>额定 (kW)</th><th>功率负载率</th><th></th></tr></thead>
        <tbody>
          <tr v-for="c in topCabs" :key="c.id">
            <td><b>{{ c.name }}</b></td>
            <td class="muted">{{ c.room_name }}</td>
            <td>{{ c.power_kw }}</td>
            <td>{{ c.rated_kw }}</td>
            <td>
              <div class="load-bar"><i :style="{ width: Math.min(c.load_pct, 100) + '%', background: loadColor(c.load_pct) }"></i></div>
            </td>
            <td :style="{ color: loadColor(c.load_pct), fontWeight: 600 }">{{ c.load_pct }}%</td>
          </tr>
          <tr v-if="!topCabs.length"><td colspan="6" class="muted" style="text-align:center;padding:24px">暂无数据</td></tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import api from '../api'
import { toast } from '../ui-dialog'
import * as echarts from 'echarts'
import { useSettingsStore } from '../stores/settings'
import { useRoute } from 'vue-router'

const settings = useSettingsStore()
const route = useRoute()
const sites = ref([])
const siteId = ref(1)
const days = ref(7)
const kpi = ref(null)
const prediction = ref(null)
const topCabs = ref([])
const rooms = ref([])
const pueRaw = ref([])
const pueTrend = ref([])
const buckets = ref([])

const pueRawEl = ref(null), pueTrendEl = ref(null), roomEl = ref(null), topEl = ref(null), bucketEl = ref(null), dailyEl = ref(null)
let charts = []

// V49 能效合规
const comp = ref(null)
const policies = ref([])
const policyLib = ref({ show: false, keyword: '', region: '' })
const policyForm = ref({ show: false, id: 0, province: '', new_large_limit: 1.3, new_small_limit: 0, surcharge_rules: '', existing_target: '', subsidy: '', features: '', err: '' })
const compStatusText = computed(() => ({
  ok: '合规', edge: '临界', over: '超标', nodata: '暂无PUE数据', unknown: '未匹配政策'
}[comp.value?.status] || comp.value?.status || ''))
const compColor = computed(() => {
  const s = comp.value?.status
  return s === 'over' ? 'var(--danger)' : s === 'edge' ? 'var(--warning)' : 'var(--success)'
})
async function loadCompliance() {
  try {
    const res = await api.get(`/energy/compliance?site_id=${siteId.value}`)
    comp.value = res.data || res
  } catch (e) { console.error('compliance load fail', e) }
}
function openPolicyLib() { policyLib.value.show = true; loadPolicies() }
async function loadPolicies() {
  try {
    const params = {}
    if (policyLib.value.keyword) params.keyword = policyLib.value.keyword
    if (policyLib.value.region) params.region = policyLib.value.region
    const res = await api.get('/pue-policies', params)
    policies.value = res.data?.items || []
  } catch (e) { console.error('policies load fail', e) }
}
function editPolicy(p) {
  policyForm.value = { show: true, id: p.id, province: p.province, new_large_limit: p.new_large_limit, new_small_limit: p.new_small_limit, surcharge_rules: p.surcharge_rules || '', existing_target: p.existing_target || '', subsidy: p.subsidy || '', features: p.features || '', err: '' }
}
async function savePolicy() {
  const f = policyForm.value
  const res = await api.put(`/pue-policies/${f.id}`, { new_large_limit: f.new_large_limit, new_small_limit: f.new_small_limit, surcharge_rules: f.surcharge_rules, existing_target: f.existing_target, subsidy: f.subsidy, features: f.features })
  if (res.code !== 0) { f.err = res.msg || '保存失败'; return }
  f.show = false // 回到列表态（同一抽屉内）
  toast('政策已保存')
  await loadPolicies()
  await loadCompliance()
}

function fmt(v) { return Number(v || 0).toLocaleString('zh-CN', { maximumFractionDigits: 1 }) }
function loadColor(p) { return p >= 70 ? 'var(--danger)' : p >= 40 ? 'var(--warning)' : 'var(--success)' }

function exportCSV() {
  const rows = []
  if (kpi.value) {
    rows.push(['指标', '数值', '单位'])
    rows.push(['IT 装机功率', kpi.value.it_power_kw, 'kW'])
    rows.push(['额定容量', kpi.value.rated_power_kw, 'kW'])
    rows.push(['容量利用率', kpi.value.cap_usage_pct, '%'])
    rows.push(['最新 PUE', kpi.value.latest_pue, ''])
    rows.push(['期间均值 PUE', kpi.value.avg_pue, ''])
    rows.push(['期间能耗', kpi.value.energy_kwh, 'kWh'])
    rows.push(['上期能耗(环比基准)', kpi.value.prev_energy_kwh, 'kWh'])
    rows.push(['能耗环比', (kpi.value.energy_delta_pct ?? 0) + '%', ''])
    rows.push(['PUE环比', (kpi.value.pue_delta_pct ?? 0) + '%', ''])
    rows.push(['电价', kpi.value.price_kwh, '元/kWh'])
    rows.push(['期间电费', kpi.value.cost_yuan, '元'])
    rows.push(['碳排放估算', kpi.value.co2_kg, 'kg'])
    rows.push([])
  }
  rows.push(['机柜功耗明细', '', '', '', ''])
  rows.push(['机柜', '房间', '功耗(kW)', '额定(kW)', '功率负载率(%)'])
  topCabs.value.forEach(c => rows.push([c.name, c.room_name, c.power_kw, c.rated_kw, c.load_pct]))
  const csv = '\uFEFF' + rows.map(r => r.map(x => `"${String(x ?? '').replace(/"/g, '""')}"`).join(',')).join('\n')
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `dcim_energy_site${siteId.value}_${days.value}d.csv`
  a.click()
  URL.revokeObjectURL(url)
}

function chartTheme() {
  const cs = getComputedStyle(document.documentElement)
  return {
    axis: cs.getPropertyValue('--muted').trim() || '#8a93a6',
    line: cs.getPropertyValue('--border2').trim() || '#3a4256',
    split: cs.getPropertyValue('--border').trim() || 'rgba(138,147,166,.12)',
  }
}

function mkChart(el) {
  const c = echarts.init(el)
  charts.push(c)
  return c
}

function renderAll() {
  const ct = chartTheme()
  const palette = settings.chartPalette
  const base = { backgroundColor: 'transparent', textStyle: { color: ct.axis } }
  const axis = { axisLine: { lineStyle: { color: ct.line } }, axisLabel: { color: ct.axis } }
  const split = { splitLine: { lineStyle: { color: ct.split } } }

  // PUE 实时曲线
  if (pueRawEl.value && pueRaw.value.length) {
    const c = charts.find(x => x.getDom() === pueRawEl.value) || mkChart(pueRawEl.value)
    c.setOption({
      ...base,
      tooltip: { trigger: 'axis', confine: true },
      grid: { left: 46, right: 20, top: 24, bottom: 30 },
      xAxis: { type: 'category', data: pueRaw.value.map(p => p.at), ...axis, axisLabel: { color: ct.axis, interval: 11 } },
      yAxis: { type: 'value', min: v => (v.min - 0.05).toFixed(2), max: v => (v.max + 0.05).toFixed(2), ...axis, ...split },
      series: [{
        name: 'PUE', type: 'line', smooth: true, showSymbol: false,
        data: pueRaw.value.map(p => p.pue),
        itemStyle: { color: palette[0] }, lineStyle: { width: 2 },
        areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: palette[0] + '55' }, { offset: 1, color: palette[0] + '08' }]) },
        markLine: { silent: true, symbol: 'none', lineStyle: { color: 'var(--warning)', type: 'dashed' },
          label: { color: ct.axis, formatter: '目标 1.40' }, data: [{ yAxis: 1.4 }] },
      }],
    })
  }

  // PUE 日趋势
  if (pueTrendEl.value && pueTrend.value.length) {
    const c = charts.find(x => x.getDom() === pueTrendEl.value) || mkChart(pueTrendEl.value)
    c.setOption({
      ...base,
      tooltip: { trigger: 'axis', confine: true },
      legend: { bottom: 0, textStyle: { color: ct.axis } },
      grid: { left: 46, right: 16, top: 20, bottom: 40 },
      xAxis: { type: 'category', data: pueTrend.value.map(p => p.day.slice(5)), ...axis },
      yAxis: { type: 'value', min: v => (v.min - 0.05).toFixed(2), max: v => (v.max + 0.05).toFixed(2), ...axis, ...split },
      series: [
        { name: '均值', type: 'line', smooth: true, data: pueTrend.value.map(p => p.avg_pue), itemStyle: { color: palette[0] }, lineStyle: { width: 2 } },
        { name: '峰值', type: 'line', smooth: true, data: pueTrend.value.map(p => p.max_pue), itemStyle: { color: palette[2] || '#ffb454' }, lineStyle: { width: 1, type: 'dashed' } },
      ],
    })
  }

  // 日能耗 / 日电费（由 PUE 日趋势 avg_total × 24h 推算）
  if (dailyEl.value && pueTrend.value.length) {
    const c = charts.find(x => x.getDom() === dailyEl.value) || mkChart(dailyEl.value)
    const price = Number(kpi.value?.price_kwh || 0.8)
    const kwhs = pueTrend.value.map(p => +(p.avg_total * 24).toFixed(1))
    c.setOption({
      ...base,
      tooltip: { trigger: 'axis', confine: true, formatter: ps => {
        const i = ps[0].dataIndex
        return `${pueTrend.value[i].day}<br>能耗 ${kwhs[i]} kWh<br>电费 ¥${(kwhs[i] * price).toFixed(1)}`
      } },
      legend: { bottom: 0, textStyle: { color: ct.axis } },
      grid: { left: 56, right: 46, top: 20, bottom: 40 },
      xAxis: { type: 'category', data: pueTrend.value.map(p => p.day.slice(5)), ...axis },
      yAxis: [
        { type: 'value', name: 'kWh', ...axis, ...split },
        { type: 'value', name: '元', ...axis, splitLine: { show: false } },
      ],
      series: [
        { name: '日能耗', type: 'bar', barWidth: 14, data: kwhs, itemStyle: { color: palette[0], borderRadius: [3, 3, 0, 0] } },
        { name: '日电费', type: 'line', yAxisIndex: 1, smooth: true, data: kwhs.map(k => +(k * price).toFixed(1)), itemStyle: { color: palette[2] || '#ffb454' }, lineStyle: { width: 2 } },
      ],
    }, true)
  }

  // 房间功耗分布
  if (roomEl.value && rooms.value.length) {
    const c = charts.find(x => x.getDom() === roomEl.value) || mkChart(roomEl.value)
    c.setOption({
      ...base,
      tooltip: { trigger: 'axis', confine: true, formatter: p => p.map(x => `${x.marker}${x.seriesName}: ${x.value} kW`).join('<br>') },
      legend: { bottom: 0, textStyle: { color: ct.axis } },
      grid: { left: 90, right: 20, top: 16, bottom: 40 },
      xAxis: { type: 'value', ...axis, ...split },
      yAxis: { type: 'category', data: rooms.value.map(r => r.room_name), ...axis },
      series: [
        { name: 'IT 功耗', type: 'bar', stack: 'a', barWidth: 18, data: rooms.value.map(r => r.it_power_kw), itemStyle: { color: palette[0], borderRadius: [0, 0, 0, 0] } },
        { name: '剩余容量', type: 'bar', stack: 'a', barWidth: 18, data: rooms.value.map(r => Math.max(r.rated_kw - r.it_power_kw, 0)), itemStyle: { color: 'rgba(138,147,166,0.18)', borderRadius: [0, 4, 4, 0] } },
      ],
    })
  }

  // 机柜功耗 TOP10
  if (topEl.value && topCabs.value.length) {
    const c = charts.find(x => x.getDom() === topEl.value) || mkChart(topEl.value)
    const data = [...topCabs.value].sort((a, b) => a.power_kw - b.power_kw)
    c.setOption({
      ...base,
      tooltip: { trigger: 'axis', confine: true, formatter: p => `${p[0].name}<br>功耗 ${p[0].value} kW / 额定 ${data[p[0].dataIndex].rated_kw} kW` },
      grid: { left: 84, right: 30, top: 16, bottom: 24 },
      xAxis: { type: 'value', ...axis, ...split },
      yAxis: { type: 'category', data: data.map(d => d.name), ...axis },
      series: [{
        type: 'bar', barWidth: 14,
        data: data.map(d => ({
          value: d.power_kw,
          itemStyle: { color: d.load_pct >= 70 ? '#ff5252' : d.load_pct >= 40 ? '#ffb454' : (palette[1] || '#00e676'), borderRadius: [0, 3, 3, 0] },
        })),
        label: { show: true, position: 'right', color: ct.axis, formatter: '{c} kW' },
      }],
    })
  }

  // 负载率分布饼图
  if (bucketEl.value && buckets.value.length) {
    const c = charts.find(x => x.getDom() === bucketEl.value) || mkChart(bucketEl.value)
    const colorMap = { '空闲': ct.axis, '轻载 <40%': palette[1] || '#00e676', '中载 40-70%': palette[2] || '#ffb454', '重载 >70%': '#ff5252' }
    c.setOption({
      ...base,
      tooltip: { trigger: 'item', formatter: '{b}: {c} 柜 ({d}%)' },
      legend: { bottom: 0, textStyle: { color: ct.axis } },
      series: [{
        type: 'pie', radius: ['42%', '66%'], center: ['50%', '44%'],
        label: { color: ct.axis, formatter: '{b}\n{c} 柜' },
        data: buckets.value.map(b => ({ name: b.bucket, value: b.count, itemStyle: { color: colorMap[b.bucket] || palette[0] } })),
      }],
    })
  }
}

async function load() {
  try {
    const res = await api.get(`/energy/overview?site_id=${siteId.value}&days=${days.value}`)
    const d = res.data || res
    kpi.value = d.kpi
    prediction.value = d.prediction
    topCabs.value = d.top_cabinets || []
    rooms.value = d.rooms || []
    pueRaw.value = d.pue_raw || []
    pueTrend.value = d.pue_trend || []
    buckets.value = d.buckets || []
    await nextTick()
    renderAll()
  } catch (e) { console.error('energy load fail', e) }
  loadCompliance()
}

function onThemeChange() { charts.forEach(c => c.dispose()); charts = []; nextTick(renderAll) }
function onResize() { charts.forEach(c => c.resize()) }

onMounted(async () => {
  try {
    const res = await api.get('/sites', { size: 100 })
    sites.value = res.data?.items || []
    if (sites.value.length) siteId.value = sites.value[0].id
    const qid = Number(route.query.id || 0)
    if (qid && sites.value.some(s => s.id === qid)) siteId.value = qid
  } catch (e) { /* ignore */ }
  await load()
  document.addEventListener('dcim-theme-change', onThemeChange)
  window.addEventListener('resize', onResize)
})
onBeforeUnmount(() => {
  document.removeEventListener('dcim-theme-change', onThemeChange)
  window.removeEventListener('resize', onResize)
  charts.forEach(c => c.dispose())
})
</script>

<style scoped>
.kpi-row { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 14px; margin-bottom: 16px; }
.kpi-card { background: var(--card); border: 1px solid var(--border); border-radius: 10px; padding: 14px 16px; position: relative; overflow: hidden; }
.kpi-card.accent { border-color: color-mix(in srgb, var(--primary) 40%, transparent); }
.kpi-card.green { border-color: color-mix(in srgb, var(--success) 40%, transparent); }
.kpi-label { font-size: 12px; color: var(--muted); margin-bottom: 6px; }
.kpi-val { font-size: 26px; font-weight: 700; color: var(--text); line-height: 1.2; }
.kpi-val small { font-size: 13px; color: var(--muted); font-weight: 400; }
.kpi-sub { font-size: 12px; color: var(--muted); margin-top: 6px; }
.kpi-bar { height: 4px; background: var(--border); border-radius: 2px; margin-top: 8px; overflow: hidden; }
.kpi-bar i { display: block; height: 100%; background: var(--primary); border-radius: 2px; }
.en-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 14px; }
.en-card .chart-box { height: 260px; }
.en-card.wide { grid-column: span 2; }
.card-hd { font-size: 13px; font-weight: 600; color: var(--text2); margin-bottom: 8px; }
.load-bar { width: 120px; height: 6px; background: var(--border); border-radius: 3px; overflow: hidden; }
.load-bar i { display: block; height: 100%; border-radius: 3px; }
.delta { font-weight: 600; margin-left: 4px; }
.delta.up { color: #ff5252; }
.delta.down { color: #00e676; }
@media (max-width: 1100px) { .en-grid { grid-template-columns: 1fr 1fr; } .en-card.wide { grid-column: span 2; } }
@media (max-width: 720px) {
  .en-grid { grid-template-columns: 1fr; }
  .en-card.wide { grid-column: span 1; }
  .toolbar { flex-wrap: wrap; gap: 8px; }
  .toolbar .muted { width: 100%; order: 9; }
  .kpi-val { font-size: 22px; }
  .en-card .chart-box { height: 220px; }
  table.table { display: block; overflow-x: auto; -webkit-overflow-scrolling: touch; }
  .load-bar { width: 80px; }
}

/* V49 能效合规 */
.comp-card { margin-bottom: 16px; border-left: 3px solid var(--success); }
.comp-card.comp-over { border-left-color: var(--danger); }
.comp-card.comp-edge { border-left-color: var(--warning); }
.comp-card.comp-unknown, .comp-card.comp-nodata { border-left-color: var(--muted); }
.comp-hd { display: flex; align-items: center; gap: 10px; padding: 12px 16px; border-bottom: 1px solid var(--border); }
.comp-title { font-weight: 600; display: inline-flex; align-items: center; gap: 6px; }
.comp-badge { font-size: 12px; padding: 2px 10px; border-radius: 10px; font-weight: 600; }
.comp-badge.ok { background: rgba(0,230,118,.15); color: var(--success); }
.comp-badge.edge { background: rgba(255,180,84,.15); color: var(--warning); }
.comp-badge.over { background: rgba(255,82,82,.15); color: var(--danger); }
.comp-badge.nodata, .comp-badge.unknown { background: rgba(140,140,140,.15); color: var(--muted); }
.comp-unknown { padding: 18px 16px; color: var(--muted); font-size: 13px; }
.comp-body { padding: 14px 16px; }
.comp-metrics { display: grid; grid-template-columns: repeat(4, 1fr); gap: 14px; }
.comp-m { background: var(--card, rgba(127,127,127,.05)); border: 1px solid var(--border); border-radius: 8px; padding: 12px 14px; }
.comp-m-warn { border-color: var(--danger); }
.comp-m-label { font-size: 12px; color: var(--muted); margin-bottom: 6px; }
.comp-m-val { font-size: 26px; font-weight: 700; font-family: Arial, sans-serif; line-height: 1.2; }
.comp-m-val small { font-size: 12px; font-weight: 400; color: var(--muted); }
.comp-m-sub { font-size: 12px; color: var(--muted); margin-top: 6px; }
.comp-notes { margin-top: 12px; display: flex; flex-direction: column; gap: 6px; }
.comp-note { font-size: 12.5px; color: var(--muted); display: flex; align-items: flex-start; gap: 6px; }
.comp-note b { color: var(--text, inherit); }
.comp-alertbox { padding: 12px 16px 14px; border-top: 1px dashed var(--border); }
.comp-alert-hd { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; font-size: 12.5px; color: var(--muted); }
.comp-alert-title { display: inline-flex; align-items: center; gap: 6px; font-weight: 600; color: var(--text); }
.comp-alert-active { margin-top: 9px; display: flex; align-items: center; gap: 9px; padding: 8px 10px; border-radius: 8px;
  background: color-mix(in srgb, var(--danger) 7%, transparent); border: 1px solid color-mix(in srgb, var(--danger) 28%, transparent); font-size: 13px; }
.comp-alert-msg { color: var(--text); }
.comp-alert-dot { width: 7px; height: 7px; border-radius: 50%; background: var(--danger); flex: none;
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--danger) 18%, transparent); animation: compPulse 1.8s ease-in-out infinite; }
@keyframes compPulse { 0%,100% { opacity: 1; } 50% { opacity: .35; } }
.comp-alert-idle { margin-top: 9px; }
@media (max-width: 1100px) { .comp-metrics { grid-template-columns: 1fr 1fr; } }
@media (max-width: 720px) { .comp-metrics { grid-template-columns: 1fr; } }
.form-err { color: var(--danger); font-size: 12px; margin-top: 8px; }
.ic-btn { display: inline-flex; align-items: center; padding: 5px 7px; }
.ic-btn svg { width: 14px; height: 14px; }
</style>
