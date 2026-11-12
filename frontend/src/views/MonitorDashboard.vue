<template>
  <div>
    <div class="page-head"><h1>动环监控</h1>
      <div class="head-ops">
        <span class="live-dot" :class="refreshCount>0?'live-on':''"></span>
        <span class="muted small">{{ refreshCount>0?'● 实时数据':'数据就绪' }}</span>
        <span v-if="refreshCount>0" class="muted small">· {{ refreshCount }}s 前刷新</span>
        <router-link to="/alert-wall" class="btn"><SvgIcon name="monitor" />告警盯屏</router-link>
        <router-link to="/trend-analysis" class="btn"><SvgIcon name="trend" />趋势分析</router-link>
        <router-link to="/alert-rules" class="btn"><SvgIcon name="gear" />告警规则</router-link>
        <router-link to="/monitoring-points" class="btn btn-primary"><SvgIcon name="pin" />测点管理</router-link>
      </div>
    </div>

    <!-- 异常测点聚焦 -->
    <div v-if="alarmingPts.length" class="alarm-focus">
      <div class="af-head"><SvgIcon name="bell" />{{ alarmingPts.length }} 个测点正在告警</div>
      <div class="af-list">
        <div v-for="p in alarmingPts" :key="p.id" class="af-chip" @click="$router.push({path:'/monitoring-points',query:{id:p.id,detail:'1'}})">
          <span class="af-type">{{ TYPE_ICON[p.type]||'?' }}{{ TYPE_NAME[p.type]||p.type }}</span>
          <span class="af-name">{{ p.name }}</span>
          <span class="af-val" :class="p.status===2?'af-warn':''">{{ p.current_value!=null ? Number(p.current_value).toFixed(1)+(p.unit||'') : '-' }}</span>
          <span v-if="p.min_threshold!=null" class="af-th">min {{ p.min_threshold }}</span>
          <span v-if="p.max_threshold!=null" class="af-th">max {{ p.max_threshold }}</span>
          <span v-if="p.alert_count>0" class="af-ac"><SvgIcon name="bell" />{{ p.alert_count }}</span>
        </div>
      </div>
      <a v-if="alarmingPts.length>=20" class="link" @click="$router.push({path:'/monitoring-points',query:{status:'2'}})">查看全部告警测点 →</a>
    </div>

    <!-- 告警趋势微图 -->
    <div class="card" style="margin-bottom:16px;padding:12px 16px">
      <div class="card-title"><SvgIcon name="chart" />24h 告警趋势
        <span style="flex:1"></span>
        <span v-if="trendTypes.length" class="muted small">每列=1小时</span>
      </div>
      <div class="trend-rows" v-if="trendTypes.length">
        <div v-for="t in trendTypes" :key="t" class="trend-row">
          <span class="tr-label" :title="TYPE_NAME[t]">{{ TYPE_ICON[t]||'?' }}</span>
          <div class="tr-bar-wrap">
            <div v-for="(v,i) in (trends[t]||[])" :key="i" class="tr-bar" :style="{height: barH(v,t)+'px'}" :title="'第'+(24-i)+'h前: '+v+' 条'">
              <div class="tr-bar-in" :class="v>2?'t-hot':v>0?'t-warm':''"></div>
            </div>
          </div>
          <span class="tr-total" :class="trendTotal(t)>2?'t-hot':''">{{ trendTotal(t) }}</span>
        </div>
      </div>
      <div v-else class="muted small" style="padding:8px 0">近 24 小时无告警 <SvgIcon name="check" /></div>
    </div>

    <!-- 六大类状态总览 -->
    <div class="monitor-grid">
      <div class="monitor-card" v-for="cat in visCats" :key="cat.key" :class="{'mc-alert': s(cat.key).alert>0}">
        <div class="mc-head">
          <span class="mc-icon">{{ cat.icon }}</span>
          <span class="mc-name link" @click="drill(cat.key, '')" :title="'查看全部'+cat.name+'测点'">{{ cat.name }}</span>
          <span class="mc-badge" :class="s(cat.key).alert>0?'warn':''" @click="drill(cat.key, '2')">{{ s(cat.key).alert>0 ? s(cat.key).alert+' 告警' : '正常' }}</span>
        </div>
        <div class="mc-stats" v-if="s(cat.key).total>0">
          <div class="mc-stat clickable" @click="drill(cat.key, '')" title="点击查看全部测点"><span class="mc-num total">{{ s(cat.key).total }}</span><span class="mc-label">总计</span></div>
          <div class="mc-stat clickable" @click="drill(cat.key, '1')" title="点击查看正常测点"><span class="mc-num ok">{{ s(cat.key).normal }}</span><span class="mc-label">正常</span></div>
          <div class="mc-stat clickable" @click="drill(cat.key, '2')" title="点击查看告警测点"><span class="mc-num warn">{{ s(cat.key).alert }}</span><span class="mc-label">告警</span></div>
          <div class="mc-stat clickable" @click="drill(cat.key, '0')" title="点击查看离线测点"><span class="mc-num off">{{ s(cat.key).offline }}</span><span class="mc-label">离线</span></div>
          <div class="mc-bar-wrap">
            <div class="mc-bar ok-bar" :style="{width: Math.round(s(cat.key).online_pct||0)+'%'}"></div>
          </div>
          <div class="mc-sub">在线率 {{ (s(cat.key).online_pct||0).toFixed(0) }}% · 告警率 {{ (s(cat.key).alert_pct||0).toFixed(1) }}%</div>
        </div>
        <div v-else class="empty">暂无数据</div>
      </div>
    </div>

    <!-- 活跃告警列表 -->
    <div class="card" style="margin-top:20px">
      <div class="card-title">
        活跃告警
        <span v-if="activeCount > 0" class="alert-badge">{{ activeCount }}</span>
        <span style="flex:1"></span>
        <input type="checkbox" v-if="alerts.length" @change="toggleAll" :checked="allChecked&&alerts.length"> 全选
        <button v-if="selAlerts.length" class="btn btn-mini" @click="batchAct('acknowledge')" style="margin-left:8px">批量确认</button>
        <button v-if="selAlerts.length" class="btn btn-mini" @click="batchAct('resolve')">批量结束</button>
        <router-link to="/alert-wall" class="link" style="margin-left:8px;font-size:13px">进入告警盯屏 →</router-link>
      </div>
      <div class="table-wrap">
        <table class="data-table">
          <thead><tr><th style="width:30px"></th><th>时间</th><th>严重度</th><th>消息</th><th>测点</th><th>关联工单</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="a in alerts" :key="a.id" :class="{'row-new': a._new}">
              <td><input type="checkbox" :value="a.id" v-model="selAlerts" /></td>
              <td class="muted">{{ fmt(a.triggered_at) }}</td>
              <td><span class="badge" :class="'sev-' + a.severity">{{ SEV[a.severity] }}</span></td>
              <td>{{ a.message }}</td>
              <td>
                <a v-if="a.point_id" class="link" @click="$router.push({ path: '/monitoring-points', query: { id: a.point_id, detail: '1' } })">{{ a.point_name || ('测点#'+a.point_id) }}</a>
                <span v-else class="muted">-</span>
              </td>
              <td>
                <a v-if="a.work_order_id" class="link" @click="$router.push({ path: '/work-orders', query: { id: a.work_order_id } })">{{ a.wo_code || ('工单#'+a.work_order_id) }}</a>
                <span v-else class="muted">-</span>
              </td>
              <td class="ops">
                <button class="btn btn-mini" @click="ack(a)">确认</button>
                <button class="btn btn-mini" @click="resolve(a)">结束</button>
              </td>
            </tr>
            <tr v-if="!alerts.length"><td colspan="7" class="empty">暂无活跃告警</td></tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import { toastAuto, showConfirm } from '../ui-dialog'

const router = useRouter()
const CATS = [
  { key: 'power_dist', icon: '', name: '配电' },
  { key: 'ups', icon: '', name: 'UPS' },
  { key: 'hvac', icon: '', name: '空调' },
  { key: 'temp_humidity', icon: '', name: '温湿度' },
  { key: 'water_leak', icon: '', name: '漏水' },
  { key: 'smoke', icon: '', name: '烟感' },
  { key: 'cdu', icon: '', name: '液冷CDU' },
  { key: 'leak_rope', icon: '', name: '漏液绳' },
]
// 液冷两类卡片仅在存在对应测点时显示（不做摆设）
const visCats = computed(() => CATS.filter(c => !['cdu','leak_rope'].includes(c.key) || s(c.key).total > 0))
const TYPE_ICON = { power_dist:'', ups:'', hvac:'', temp_humidity:'', water_leak:'', smoke:'', cdu:'', leak_rope:'' }
const TYPE_NAME = { power_dist:'配电', ups:'UPS', hvac:'空调', temp_humidity:'温湿度', water_leak:'漏水', smoke:'烟感', cdu:'液冷CDU', leak_rope:'漏液绳' }
const SEV = { 1: '提示', 2: '警告', 3: '严重' }

const d = ref({})
const alerts = ref([])
const selAlerts = ref([])
const trends = ref({})
const alarmingPts = ref([])
const refreshCount = ref(0)
let timer = null
let prevIds = new Set()

const activeCount = computed(() => alerts.value.filter(a => a.status < 3).length)
const allChecked = computed(() => alerts.value.length > 0 && selAlerts.value.length === alerts.value.length)
const trendTypes = computed(() => Object.keys(trends.value).filter(k => trends.value[k]&&trends.value[k].length))
const s = (key) => d.value[key] || { total: 0, normal: 0, alert: 0, offline: 0, online_pct: 0, alert_pct: 0 }

const barH = (v, t) => {
  const maxV = Math.max(1, ...(trends.value[t]||[1]))
  return Math.max(3, Math.round(v / maxV * 28))
}
const trendTotal = (t) => (trends.value[t]||[]).reduce((a,b)=>a+b,0)
const fmt = t => t ? String(t).replace('T', ' ').slice(0, 16) : '-'

function drill(type, status) {
  const query = { type }
  if (status !== '') query.status = String(status)
  router.push({ path: '/monitoring-points', query })
}

async function load() {
  const [s, a] = await Promise.all([
    api.get('/monitoring-summary'),
    api.get('/alerts', { size: 50 })
  ])
  // 摘要含 trends + alarming_points
  d.value = s.data || {}
  trends.value = s.data?.trends || {}
  alarmingPts.value = s.data?.alarming_points || []

  // 告警去重 + 新告警高亮标记
  const activeList = (a.data?.items || []).filter(x => x.status < 3)
  const currentIds = new Set(activeList.map(x => x.id))
  for (const al of activeList) {
    al._new = !prevIds.has(al.id)
  }
  prevIds = currentIds
  alerts.value = activeList
  refreshCount.value = refreshCount.value + 1
  if (refreshCount.value > 99) refreshCount.value = 1
}

function toggleAll(e) {
  selAlerts.value = e.target.checked ? alerts.value.map(a => a.id) : []
}

async function batchAct(action) {
  if (!selAlerts.value.length) return
  const label = action === 'acknowledge' ? '批量确认' : '批量结束'
  if (!(await showConfirm({ message: `${label} ${selAlerts.value.length} 条告警？` }))) return
  const r = await api.post('/alerts/batch', { ids: selAlerts.value, action, reason: label })
  if (r.code === 0) { selAlerts.value = []; load() }
}

async function ack(a) {
  const r = await api.post(`/alerts/${a.id}/acknowledge`, {})
  if (r.code !== 0) return toastAuto(r.msg)
  load()
}

async function resolve(a) {
  if (!(await showConfirm({ message: `结束告警「${a.message}」？` }))) return
  const r = await api.post(`/alerts/${a.id}/resolve`, {})
  if (r.code !== 0) return toastAuto(r.msg)
  load()
}

onMounted(async () => {
  await load()
  timer = setInterval(() => load(), 30000) // 30s 轮询
})
onBeforeUnmount(() => {
  if (timer) { clearInterval(timer); timer = null }
})
</script>

<style scoped>
.page-head { display: flex; align-items: center; justify-content: space-between; }
.head-ops { display: flex; gap: 8px; align-items: center; }
.head-ops .btn { text-decoration: none; }
.live-dot { width: 8px; height: 8px; border-radius: 50%; background: #909399; display: inline-block; }
.live-dot.live-on { background: #67c23a; animation: pulse-dot 2s infinite; }
@keyframes pulse-dot { 0%,100%{opacity:1} 50%{opacity:.4} }

/* 异常聚焦 */
.alarm-focus { background: rgba(245,108,108,.06); border: 1px solid rgba(245,108,108,.3); border-radius: 10px; padding: 12px 16px; margin-bottom: 16px; }
.af-head { font-weight: 700; color: #f56c6c; margin-bottom: 8px; }
.af-list { display: flex; flex-wrap: wrap; gap: 6px; }
.af-chip { display: inline-flex; align-items: center; gap: 5px; padding: 3px 10px; background: var(--card); border: 1px solid var(--border); border-radius: 14px; font-size: 12px; cursor: pointer; }
.af-chip:hover { border-color: #f56c6c; background: rgba(245,108,108,.06); }
.af-type { font-size: 11px; color: var(--muted); }
.af-name { font-weight: 600; }
.af-val { font-weight: 700; color: var(--primary); }
.af-val.af-warn { color: #f56c6c; }
.af-th { font-size: 10px; color: var(--muted); }
.af-ac { font-size: 10px; color: #f56c6c; }
.link { color: var(--accent); cursor: pointer; font-weight: 600; font-size: 12px; }
.link:hover { text-decoration: underline; }

/* 趋势微图 */
.trend-rows { display: flex; flex-direction: column; gap: 4px; margin-top: 8px; }
.trend-row { display: flex; align-items: flex-end; gap: 6px; }
.tr-label { font-size: 14px; width: 18px; text-align: center; }
.tr-bar-wrap { display: flex; gap: 1px; align-items: flex-end; flex: 1; height: 32px; }
.tr-bar { width: 7px; background: var(--bg2); border-radius: 2px 2px 0 0; position: relative; min-height: 2px; }
.tr-bar-in { position: absolute; bottom: 0; left: 0; right: 0; border-radius: 2px 2px 0 0; height: 100%; background: rgba(103,194,58,.5); }
.tr-bar-in.t-warm { background: rgba(230,162,60,.6); }
.tr-bar-in.t-hot { background: rgba(245,108,108,.7); }
.tr-total { font-size: 11px; font-weight: 700; width: 28px; text-align: right; color: var(--muted); }
.tr-total.t-hot { color: #f56c6c; }

/* 卡片 */
.monitor-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 14px; }
.monitor-card { background: var(--card); border: 1px solid var(--border); border-radius: 10px; padding: 16px; }
.monitor-card.mc-alert { border-color: rgba(245,108,108,.4); background: rgba(245,108,108,.03); }
.mc-head { display: flex; align-items: center; gap: 8px; margin-bottom: 10px; }
.mc-icon { font-size: 20px; }
.mc-name { font-size: 14px; font-weight: 600; flex: 1; }
.mc-badge { font-size: 10px; padding: 2px 8px; border-radius: 10px; background: rgba(103,194,58,.15); color: #67c23a; }
.mc-badge.warn { background: rgba(245,108,108,.15); color: #f56c6c; cursor: pointer; }
.mc-stats { display: grid; grid-template-columns: 1fr 1fr; gap: 6px; }
.mc-stat { text-align: center; border-radius: 8px; padding: 2px; }
.mc-stat.clickable { cursor: pointer; transition: background .15s, transform .15s; }
.mc-stat.clickable:hover { background: var(--bg2); transform: translateY(-1px); }
.mc-num { display: block; font-size: 20px; font-weight: 700; }
.mc-num.total { color: #409eff; }
.mc-num.ok { color: #67c23a; }
.mc-num.warn { color: #e6a23c; }
.mc-num.off { color: #909399; }
.mc-label { font-size: 10px; color: var(--muted); }
.mc-bar-wrap { grid-column: 1 / -1; height: 4px; background: var(--bg2); border-radius: 2px; overflow: hidden; margin: 2px 0; }
.mc-bar { height: 100%; }
.mc-bar.ok-bar { background: #67c23a; }
.mc-sub { grid-column: 1 / -1; font-size: 10px; color: var(--muted); text-align: center; }
.alert-badge { background: #f56c6c; color: #fff; border-radius: 10px; padding: 1px 8px; font-size: 11px; margin-left: 8px; }
.badge.sev-3 { color: #f56c6c; border-color: rgba(245,108,108,.4); }
.badge.sev-2 { color: #e6a23c; border-color: rgba(230,162,60,.4); }
.badge.sev-1 { color: #409eff; border-color: rgba(64,158,255,.4); }
.card-title { display: flex; align-items: center; gap: 8px; }
.row-new { animation: row-new-in .6s ease-out; background: rgba(64,158,255,.06); }
@keyframes row-new-in { 0% { background: rgba(64,158,255,.2); } 100% { background: transparent; } }
</style>