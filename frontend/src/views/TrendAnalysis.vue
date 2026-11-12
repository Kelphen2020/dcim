<template>
  <div>
    <div class="page-head">
      <h1>趋势分析中心</h1>
      <div class="toolbar">
        <select class="input" style="width:170px" v-model.number="siteId" @change="loadTree">
          <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
        </select>
        <select class="input" style="width:120px" v-model="range">
          <option value="1h">最近 1 小时</option>
          <option value="24h">最近 24 小时</option>
          <option value="1w">最近一周</option>
          <option value="1m">最近一月</option>
          <option value="3m">最近三月</option>
          <option value="custom">自定义区间</option>
        </select>
        <template v-if="range === 'custom'">
          <input type="datetime-local" class="input" v-model="customStart" />
          <input type="datetime-local" class="input" v-model="customEnd" />
        </template>
        <div class="seg">
          <button :class="['seg-btn', view === 'chart' && 'on']" @click="view='chart'">曲线图</button>
          <button :class="['seg-btn', view === 'table' && 'on']" @click="view='table'">点值表</button>
        </div>
        <button class="btn btn-primary" :disabled="!sel.length || loading" @click="query">{{ loading ? '查询中…' : '查询' }}</button>
        <label class="thresh-toggle" :title="'在曲线上标注测点阈值线与越限点'">
          <input type="checkbox" v-model="showThresh" @change="series.length && renderChart()" /> 阈值标注
          <span v-if="violations.length" class="viol-badge" :title="'近7天越限 '+violations.length+' 次'"><SvgIcon name="alert" />{{ violations.length }}</span>
        </label>
        <button class="btn" :disabled="!sel.length" @click="saveTpl"><SvgIcon name="save" />存为模板</button>
        <button class="btn" @click="tplShow=true"><SvgIcon name="clipboard" />模板 ({{ templates.length }})</button>
        <span style="flex:1"></span>
        <button class="btn" :disabled="!series.length" @click="exportCSV"><SvgIcon name="download" />CSV</button>
        <button class="btn" :disabled="!series.length || view!=='chart'" @click="exportPNG"><SvgIcon name="image" />PNG</button>
      </div>
    </div>

    <div class="ta-grid">
      <!-- 左侧点树 -->
      <div class="card ta-tree">
        <div class="ta-tree-head">
          <span class="card-title" style="margin:0">设备测点</span>
          <span class="muted small">已选 {{ sel.length }}/20</span>
        </div>
        <div v-if="!rooms.length" class="empty">无测点数据</div>
        <div v-for="room in rooms" :key="'r'+room.id" class="ta-node lv1">
          <div class="ta-row" @click="room.open = !room.open">
            <span class="tw">{{ room.open ? '▾' : '▸' }}</span>
            <span class="room-ico"><SvgIcon name="building" /></span>{{ room.name }}
            <span class="muted small">({{ roomPointCount(room) }})</span>
          </div>
          <template v-if="room.open">
            <div v-for="cab in room.cabinets" :key="'c'+cab.id" class="ta-node lv2">
              <div class="ta-row" @click="cab.open = !cab.open">
                <span class="tw">{{ cab.open ? '▾' : '▸' }}</span>
                <span class="room-ico"><SvgIcon name="rack" /></span>{{ cab.code }}
              </div>
              <template v-if="cab.open">
                <label v-for="p in cab.points" :key="p.id" class="ta-row leaf" :class="{ checked: sel.includes(p.id) }">
                  <input type="checkbox" :checked="sel.includes(p.id)" @change="toggle(p)" />
                  <span :class="'pt-dot t-'+p.type"></span>{{ p.name }}
                  <span class="muted small" v-if="p.current_value != null">{{ p.current_value }}{{ p.unit }}</span>
                </label>
              </template>
            </div>
            <label v-for="p in room.points" :key="p.id" class="ta-row leaf" :class="{ checked: sel.includes(p.id) }">
              <input type="checkbox" :checked="sel.includes(p.id)" @change="toggle(p)" />
              <span :class="'pt-dot t-'+p.type"></span>{{ p.name }}
              <span class="muted small" v-if="p.current_value != null">{{ p.current_value }}{{ p.unit }}</span>
            </label>
          </template>
        </div>
      </div>

      <!-- 右侧视图 -->
      <div class="card ta-main">
        <template v-if="view === 'chart'">
          <div ref="chartEl" class="ta-chart"></div>
          <div v-if="series.length" class="ta-stats">
            <div v-for="s in series" :key="s.point_id" class="stat-chip">
              <b>{{ s.name }}</b><span class="muted">min {{ s.stats.min }} / avg {{ s.stats.avg }} / max {{ s.stats.max }} {{ s.unit }}（{{ s.stats.count }} 点）</span>
            </div>
          </div>
        </template>
        <template v-else>
          <div class="table-wrap" v-if="matrix.rows.length">
            <table class="data-table ta-matrix">
              <thead>
                <tr>
                  <th>时间</th>
                  <th v-for="s in series" :key="s.point_id">{{ s.name }}（{{ s.unit }}）</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in matrix.rows" :key="row.t">
                  <td class="mono">{{ row.t }}</td>
                  <td class="mono" v-for="(s, i) in series" :key="i">{{ row.v[i] != null ? row.v[i].toFixed(2) : '—' }}</td>
                </tr>
              </tbody>
              <tfoot v-if="series.length">
                <tr><td>最小</td><td class="mono" v-for="s in series" :key="'min'+s.point_id">{{ s.stats.min }}</td></tr>
                <tr><td>平均</td><td class="mono" v-for="s in series" :key="'avg'+s.point_id">{{ s.stats.avg }}</td></tr>
                <tr><td>最大</td><td class="mono" v-for="s in series" :key="'max'+s.point_id">{{ s.stats.max }}</td></tr>
              </tfoot>
            </table>
          </div>
          <div v-else class="empty">选择测点后点击查询</div>
        </template>
        <div v-if="!series.length && view==='chart'" class="empty ta-empty">← 在左侧勾选测点后点击「查询」</div>
      </div>
    </div>

    <!-- 模板抽屉 -->
    <div v-if="tplShow" class="drawer-mask" @click.self="tplShow=false">
      <div class="drawer">
        <div class="drawer-head"><b>趋势模板</b><button class="btn btn-ghost" @click="tplShow=false"><SvgIcon name="x" /></button></div>
        <div class="drawer-body">
          <div v-if="!templates.length" class="empty">暂无模板，查询后可存为模板</div>
          <div v-for="t in templates" :key="t.id" class="tpl-item">
            <div class="tpl-info">
              <b>{{ t.name }}</b>
              <div class="muted small">{{ t.point_ids.split(',').length }} 个测点 · {{ RANGE[t.range_preset] || t.range_preset }} · {{ t.view_type === 'chart' ? '曲线图' : '点值表' }}</div>
            </div>
            <button class="btn btn-mini btn-primary" @click="applyTpl(t)">应用</button>
            <button class="btn btn-mini btn-mini-danger" @click="delTpl(t)">删除</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import * as echarts from 'echarts'
import api from '../api'
import { showPrompt } from '../ui-dialog'
import { useSettingsStore } from '../stores/settings'

const settings = useSettingsStore()

const RANGE = { '1h': '最近1小时', '24h': '最近24小时', '1w': '最近一周', '1m': '最近一月', '3m': '最近三月' }
const TYPE_COLOR = {
  temp_humidity: '#13c2c2', power_dist: '#722ed1', ups: '#fa8c16',
  hvac: '#52c41a', water_leak: '#1677ff', smoke: '#f5222d'
}

const sites = ref([])
const siteId = ref(1)
const rooms = ref([])
const sel = ref([])
const range = ref('24h')
const customStart = ref('')
const customEnd = ref('')
const view = ref('chart')
const loading = ref(false)
const series = ref([])
const templates = ref([])
const tplShow = ref(false)
const chartEl = ref(null)
let chart = null

function roomPointCount(room) {
  let n = (room.points || []).length
  for (const c of room.cabinets || []) n += (c.points || []).length
  return n
}

function toggle(p) {
  const i = sel.value.indexOf(p.id)
  if (i >= 0) sel.value.splice(i, 1)
  else if (sel.value.length < 20) sel.value.push(p.id)
}

async function loadSites() {
  const r = await api.get('/sites')
  sites.value = r.data?.items || []
  if (sites.value.length && !sites.value.find(s => s.id === siteId.value)) siteId.value = sites.value[0].id
}

async function loadTree() {
  const r = await api.get('/point-tree', { site_id: siteId.value })
  rooms.value = (r.data?.rooms || []).map(rm => ({ ...rm, open: true, cabinets: (rm.cabinets || []).map(c => ({ ...c, open: false })) }))
}

async function loadTpls() {
  const r = await api.get('/trend-templates')
  templates.value = r.data?.items || []
}

async function query() {
  if (!sel.value.length) return
  loading.value = true
  try {
    const params = { point_ids: sel.value.join(',') }
    if (range.value === 'custom') {
      if (!customStart.value || !customEnd.value) { loading.value = false; return }
      params.start = customStart.value.replace('T', ' ') + ':00'
      params.end = customEnd.value.replace('T', ' ') + ':00'
    } else params.range = range.value
    const r = await api.get('/trend-series', params)
    series.value = r.data?.series || []
    // V15 阈值与越限事件
    await loadThresholds()
    if (view.value === 'chart') renderChart()
  } finally { loading.value = false }
}

// V15 阈值线 + 越限标注
const thresholds = ref({})
const violations = ref([])
const showThresh = ref(true)
async function loadThresholds() {
  if (!sel.value.length) { thresholds.value = {}; violations.value = []; return }
  const r = await api.get('/trend-thresholds', { point_ids: sel.value.join(',') })
  if (r.code !== 0) return
  const m = {}
  for (const t of (r.data?.thresholds || [])) m[t.id] = t
  thresholds.value = m
  violations.value = r.data?.violations || []
}

const matrix = computed(() => {
  if (!series.value.length) return { rows: [] }
  // 时间轴合并
  const ts = new Set()
  series.value.forEach(s => s.data.forEach(d => ts.add(d.t)))
  const rows = [...ts].sort().map(t => ({ t, v: series.value.map(s => {
    const d = s.data.find(x => x.t === t)
    return d ? d.v : null
  }) }))
  return { rows }
})

function chartTheme() {
  const cs = getComputedStyle(document.documentElement)
  return {
    axis: cs.getPropertyValue('--muted').trim() || '#8a93a6',
    line: cs.getPropertyValue('--border2').trim() || '#3a4256',
    split: cs.getPropertyValue('--border').trim() || 'rgba(138,147,166,.12)',
  }
}

function renderChart() {
  if (!chartEl.value) return
  chart = chart || echarts.init(chartEl.value)
  const ct = chartTheme()
  const palette = settings.chartPalette
  chart.setOption({
    backgroundColor: 'transparent',
    color: palette,
    tooltip: { trigger: 'axis', confine: true },
    legend: { type: 'scroll', bottom: 0, textStyle: { color: ct.axis } },
    grid: { left: 48, right: 20, top: 16, bottom: 40 },
    xAxis: { type: 'category', data: series.value[0]?.data.map(d => d.t.slice(11, 16)) || [], axisLine: { lineStyle: { color: ct.line } }, axisLabel: { color: ct.axis } },
    yAxis: { type: 'value', splitLine: { lineStyle: { color: ct.split } }, axisLabel: { color: ct.axis } },
    series: series.value.map((s, i) => {
      const opt = {
        name: s.name + (s.unit ? ` (${s.unit})` : ''),
        type: 'line', smooth: true, showSymbol: false,
        data: s.data.map(d => d.v),
        itemStyle: { color: palette[i % palette.length] },
        lineStyle: { width: 1.6 },
      }
      // V15 阈值线: 该测点配置了阈值则画 markLine
      if (showThresh.value) {
        const th = thresholds.value[s.point_id]
        const lines = []
        if (th && th.high_threshold != null) lines.push({ yAxis: th.high_threshold, lineStyle: { color: '#c62828', type: 'solid', width: 1.6 }, label: { formatter: '过高 ' + th.high_threshold, color: '#c62828', fontSize: 10 } })
        if (th && th.max_threshold != null) lines.push({ yAxis: th.max_threshold, lineStyle: { color: '#f56c6c', type: 'dashed', width: 1.2 }, label: { formatter: '超高 ' + th.max_threshold, color: '#f56c6c', fontSize: 10 } })
        if (th && th.min_threshold != null) lines.push({ yAxis: th.min_threshold, lineStyle: { color: '#e6a23c', type: 'dashed', width: 1.2 }, label: { formatter: '过低 ' + th.min_threshold, color: '#e6a23c', fontSize: 10 } })
        if (lines.length) opt.markLine = { symbol: 'none', silent: true, data: lines }
        // V15 越限点标注(红色散点): 该测点在窗口内的越限读数
        const vs = violations.value.filter(v => v.point_id === s.point_id).slice(0, 50)
        if (vs.length) {
          const tset = new Set(s.data.map(d => d.t))
          const pts = []
          for (const v of vs) {
            const vt = String(v.timestamp).replace('T', ' ').slice(0, 16)
            // 时间近似匹配到曲线上的点
            const hit = s.data.find(d => d.t === vt || vt.startsWith(d.t.slice(0, 13)))
            if (hit && tset.has(hit.t)) pts.push({ coord: [hit.t.slice(11, 16), hit.v], value: v.value })
          }
          if (pts.length) opt.markPoint = { symbol: 'circle', symbolSize: 7, itemStyle: { color: '#f56c6c', borderColor: '#fff', borderWidth: 1 }, label: { show: false }, data: pts }
        }
      }
      return opt
    })
  }, true)
}

// V4：主题切换时按新色板重绘
function onThemeChange() {
  if (view.value === 'chart' && series.value.length) setTimeout(renderChart, 60)
}

async function saveTpl() {
  const name = await showPrompt({ title: '保存趋势模板', label: '模板名称', defaultValue: `趋势对比-${new Date().toLocaleDateString()}` })
  if (!name) return
  await api.post('/trend-templates', { name, point_ids: sel.value.join(','), range_preset: range.value, view_type: view.value })
  await loadTpls()
}

async function applyTpl(t) {
  sel.value = t.point_ids.split(',').map(Number).filter(Boolean)
  range.value = RANGE[t.range_preset] ? t.range_preset : '24h'
  view.value = t.view_type === 'table' ? 'table' : 'chart'
  tplShow.value = false
  await query()
}

async function delTpl(t) {
  await api.del(`/trend-templates/${t.id}`)
  await loadTpls()
}

function exportCSV() {
  const rows = matrix.value.rows
  const head = ['时间', ...series.value.map(s => s.name)]
  const lines = [head.join(',')]
  rows.forEach(r => lines.push([r.t, ...r.v.map(v => v != null ? v : '')].join(',')))
  const blob = new Blob(['\ufeff' + lines.join('\n')], { type: 'text/csv' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `趋势数据_${new Date().toISOString().slice(0, 19).replace(/[:T]/g, '-')}.csv`
  a.click()
}

function exportPNG() {
  if (!chart) return
  const a = document.createElement('a')
  a.href = chart.getDataURL({ type: 'png', pixelRatio: 2, backgroundColor: '#fff' })
  a.download = `趋势图_${new Date().toISOString().slice(0, 19).replace(/[:T]/g, '-')}.png`
  a.click()
}

watch(view, () => { if (view.value === 'chart' && series.value.length) setTimeout(renderChart, 50) })

const onResize = () => chart && chart.resize()

onMounted(async () => {
  await loadSites()
  await loadTree()
  await loadTpls()
  window.addEventListener('resize', onResize)
  document.addEventListener('dcim-theme-change', onThemeChange)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  document.removeEventListener('dcim-theme-change', onThemeChange)
  chart && chart.dispose()
})
</script>

<style scoped>
.ta-grid { display: grid; grid-template-columns: 300px 1fr; gap: 16px; align-items: start; }
.ta-tree { max-height: calc(100vh - 220px); overflow: auto; padding: 14px; }
.ta-tree-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
.ta-node { margin-left: 2px; }
.ta-row { display: flex; align-items: center; gap: 6px; padding: 5px 6px; border-radius: 6px; cursor: pointer; font-size: 13px; user-select: none; }
.ta-row:hover { background: rgba(22, 119, 255, .06); }
.lv2 .ta-row { padding-left: 22px; }
.leaf { padding-left: 40px; }
.leaf.checked { background: rgba(22, 119, 255, .1); }
.tw { width: 12px; color: var(--muted); font-size: 10px; }
.room-ico { font-size: 13px; }
.pt-dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; }
.t-temp_humidity { background: #13c2c2; } .t-power_dist { background: #722ed1; }
.t-ups { background: #fa8c16; } .t-hvac { background: #52c41a; }
.t-water_leak { background: #1677ff; } .t-smoke { background: #f5222d; }
.seg { display: inline-flex; border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
.thresh-toggle { display: inline-flex; align-items: center; gap: 5px; font-size: 12.5px; cursor: pointer; color: var(--text2, var(--muted)); padding: 0 6px; }
.thresh-toggle input { accent-color: var(--primary); cursor: pointer; }
.viol-badge { background: rgba(245,108,108,.15); color: #f56c6c; border-radius: 8px; padding: 1px 7px; font-size: 11px; font-weight: 700; }
.seg-btn { border: 0; background: none; padding: 6px 14px; cursor: pointer; font-size: 13px; color: var(--muted); }
.seg-btn.on { background: var(--primary); color: #fff; }
.ta-main { min-height: 520px; padding: 16px; position: relative; }
.ta-chart { height: 420px; }
.ta-stats { display: flex; gap: 10px; flex-wrap: wrap; margin-top: 12px; }
.stat-chip { background: rgba(22, 119, 255, .05); border: 1px solid var(--border); border-radius: 8px; padding: 6px 12px; font-size: 12px; display: flex; flex-direction: column; gap: 2px; }
.ta-matrix td.mono, .ta-matrix th { font-variant-numeric: tabular-nums; }
.ta-matrix tfoot td { font-weight: 600; background: rgba(22, 119, 255, .04); }
.ta-empty { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; }
.tpl-item { display: flex; align-items: center; gap: 10px; padding: 10px; border: 1px solid var(--border); border-radius: 8px; margin-bottom: 8px; }
.tpl-info { flex: 1; }
@media (max-width: 900px) { .ta-grid { grid-template-columns: 1fr; } .ta-tree { max-height: 300px; } }
</style>
