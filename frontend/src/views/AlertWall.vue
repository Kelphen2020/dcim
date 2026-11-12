<template>
  <div class="aw">
    <!-- 顶栏 -->
    <div class="aw-top">
      <div class="aw-title">
        <span class="aw-dot"></span>告警盯屏
        <span class="aw-site">{{ siteName }}</span>
        <span class="aw-clock">{{ clock }}</span>
      </div>
      <div class="aw-actions">
        <button class="aw-btn" :class="{ on: batchMode }" @click="batchMode=!batchMode; selIds=[]"><SvgIcon v-if="!batchMode" name="check-square" />{{ batchMode ? '批量模式' : '批量操作' }}</button>
        <template v-if="batchMode && selIds.length">
          <button class="aw-btn" @click="batchAct('acknowledge')">批量确认({{ selIds.length }})</button>
          <button class="aw-btn" @click="batchAct('resolve')">批量结束({{ selIds.length }})</button>
        </template>
        <button class="aw-btn" :class="{ on: sound }" @click="sound = !sound">{{ sound ? '声音开' : '声音关' }}</button>
        <button class="aw-btn" :class="{ on: locked }" @click="locked = !locked">{{ locked ? '已锁定' : '未锁定' }}</button>
        <button class="aw-btn" @click="exportCSV"><SvgIcon name="download" />导出</button>
        <button class="aw-btn" @click="toggleFs"><SvgIcon name="expand" />全屏</button>
        <router-link to="/monitor" class="aw-btn">← 返回动环监控</router-link>
      </div>
    </div>

    <!-- 统计条 -->
    <div v-if="sevOnly" class="aw-deeplink">已按大屏穿透过滤：仅显示「{{ SEV[sevOnly] }}」级告警 <a class="aw-link" @click="sevOnly=0">清除过滤</a></div>
    <div class="aw-stats">
      <div class="aw-stat sev3"><b>{{ count(3) }}</b><span>严重</span></div>
      <div class="aw-stat sev2"><b>{{ count(2) }}</b><span>警告</span></div>
      <div class="aw-stat sev1"><b>{{ count(1) }}</b><span>提示</span></div>
      <div class="aw-stat"><b>{{ alerts.filter(a => a.status === 0).length }}</b><span>待处理</span></div>
      <div class="aw-stat"><b>{{ alerts.filter(a => a.status === 1).length }}</b><span>已挂起</span></div>
      <div class="aw-stat"><b>{{ alerts.filter(a => a.status === 2).length }}</b><span>已确认</span></div>
      <div class="aw-stat ok"><b>{{ alerts.filter(a => a.status === 3).length }}</b><span>已结束</span></div>
    </div>

    <!-- 筛选 -->
    <div class="aw-filter">
      <button v-for="f in FILTERS" :key="f.key" class="aw-tab" :class="{ on: filter === f.key }" @click="filter = f.key">
        {{ f.name }}<em v-if="f.count">{{ f.count }}</em>
      </button>
    </div>

    <!-- 告警墙 -->
    <div class="aw-wall">
      <div v-if="!filtered.length" class="aw-empty">
        <div class="aw-empty-ico"><SvgIcon name="check" /></div>
        <div>当前视图无告警</div>
      </div>
      <div v-for="a in filtered" :key="a.id" :class="['aw-card', 'sev' + a.severity, { flashed: a._new }]">
        <div class="aw-card-l">
          <input v-if="batchMode && !locked" type="checkbox" class="aw-chk" :value="a.id" v-model="selIds" :disabled="a.status===3" />
          <div class="aw-sev">{{ SEV[a.severity] }}</div>
          <div class="aw-st">{{ STATUS[a.status] }}<span v-if="a.watched"> · 关注</span><span v-if="a.blocked"> · 屏蔽</span></div>
        </div>
        <div class="aw-card-m">
          <div class="aw-msg">{{ a.message }}</div>
          <div class="aw-meta">
            <span>#{{ a.id }}</span>
            <span>{{ fmt(a.triggered_at) }}</span>
            <a v-if="a.point_id" class="aw-link" @click="$router.push({ path: '/monitoring-points', query: { id: a.point_id, detail: '1' } })" :title="'打开测点详情'"><SvgIcon name="pin" />{{ a.point_name || ('测点#'+a.point_id) }}</a>
            <a v-if="a.work_order_id" class="aw-link" @click="$router.push({ path: '/work-orders', query: { id: a.work_order_id } })" :title="'打开工单详情'"><SvgIcon name="wrench" />{{ a.wo_code || ('工单#'+a.work_order_id) }}</a>
            <span v-if="a.acknowledged_at">确认于 {{ fmt(a.acknowledged_at) }}</span>
            <span v-if="a.storm_id" class="aw-storm" :title="'已归入告警风暴 #'+a.storm_id+(a.suppressed?'（抑制告警，根因见风暴首条）':'（根因告警）')" @click="$router.push({path:'/duty-roster',query:{tab:'storm'}})"><SvgIcon name="storm" />风暴#{{ a.storm_id }}{{ a.suppressed?'·抑制':'·根因' }}</span>
          </div>
        </div>
        <div class="aw-card-r" v-if="!locked">
          <button v-if="a.status === 0" class="aw-op" @click="act(a, 'suspend', '挂起原因')">挂起</button>
          <button v-if="a.status === 1" class="aw-op" @click="act(a, 'resume')">恢复</button>
          <button v-if="a.status === 0 || a.status === 1" class="aw-op ack" @click="act(a, 'acknowledge', '确认说明')">确认</button>
          <button v-if="a.status !== 3" class="aw-op done" @click="act(a, 'resolve', '结束说明')">结束</button>
          <button class="aw-op" @click="act(a, a.watched ? 'unwatch' : 'watch')">{{ a.watched ? '取消关注' : '关注' }}</button>
          <button class="aw-op" @click="act(a, a.blocked ? 'unblock' : 'block')">{{ a.blocked ? '解除屏蔽' : '屏蔽' }}</button>
          <button class="aw-op log" @click="openLog(a)">留痕</button>
        </div>
        <div class="aw-card-r" v-else><span class="aw-lock-hint"><SvgIcon name="lock" />锁定中</span></div>
      </div>
    </div>

    <!-- 留痕抽屉 -->
    <div v-if="logDrawer.show" class="drawer-mask" @click.self="logDrawer.show = false">
      <div class="drawer" style="width:460px">
        <div class="drawer-head"><h3>操作留痕 · 告警 #{{ logDrawer.id }}</h3><button class="btn btn-mini" @click="logDrawer.show = false"><SvgIcon name="x" /></button></div>
        <div class="drawer-body">
          <div v-if="!logDrawer.items.length" class="aw-empty small">暂无操作记录</div>
          <div v-for="l in logDrawer.items" :key="l.id" class="log-item">
            <b>{{ ACTION_NAME[l.action] || l.action }}</b>
            <span class="muted">{{ l.reason || '—' }}</span>
            <span class="muted small">{{ fmt(l.created_at) }}</span>
          </div>
          <div v-if="logDrawer.plans.length" class="aw-cases-sec">
            <div class="aw-cases-t"><SvgIcon name="shield" />匹配应急预案（V59）</div>
            <div v-for="ep in logDrawer.plans" :key="ep.id" class="aw-case" @click="$router.push('/emergency-plans')">
              <b>{{ ep.title }}</b>
              <span class="muted small">匹配理由：{{ ep.reason }} · 优先级{{ ['', '高', '中', '低'][ep.priority] }} · 演练{{ ep.drill_count || 0 }}次</span>
            </div>
          </div>
          <div class="aw-cases-sec">
            <div class="aw-cases-t"><SvgIcon name="book" />相似历史案例（故障案例库）</div>
            <div v-if="logDrawer.cases.length">
              <div v-for="fc in logDrawer.cases" :key="fc.id" class="aw-case" @click="$router.push('/fault-cases')">
                <b>{{ fc.code }}</b> {{ fc.title }} <span class="muted small">匹配 {{ fc.score }}</span>
                <div v-if="fc.solution" class="aw-case-sol muted small">方案: {{ String(fc.solution).slice(0, 70) }}</div>
              </div>
            </div>
            <div v-else class="muted small">暂无匹配案例</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import api from '../api'
import { showPrompt, toastAuto } from '../ui-dialog'

const route = useRoute()
const sevOnly = ref(0) // 深链 ?severity=N 过滤

const SEV = { 1: '提示', 2: '警告', 3: '严重' }
const STATUS = { 0: '待处理', 1: '已挂起', 2: '已确认', 3: '已结束' }
const ACTION_NAME = { suspend: '挂起', resume: '恢复待处理', acknowledge: '确认', resolve: '结束', watch: '标记关注', unwatch: '取消关注', block: '屏蔽', unblock: '解除屏蔽' }

const alerts = ref([])
const filter = ref('active')
const sound = ref(true)
const locked = ref(false)
const clock = ref('')
const siteName = ref('')
const logDrawer = ref({ show: false, id: 0, items: [], cases: [], plans: [] })
let timer = null, clockTimer = null, lastCount = -1
const seenIds = new Set()

const FILTERS = computed(() => [
  { key: 'active', name: '活跃', count: alerts.value.filter(a => a.status !== 3 && !a.blocked).length },
  { key: 'pending', name: '待处理', count: alerts.value.filter(a => a.status === 0).length },
  { key: 'suspended', name: '已挂起', count: alerts.value.filter(a => a.status === 1).length },
  { key: 'acked', name: '已确认', count: alerts.value.filter(a => a.status === 2).length },
  { key: 'resolved', name: '已结束', count: alerts.value.filter(a => a.status === 3).length },
  { key: 'watched', name: '已关注', count: alerts.value.filter(a => a.watched).length },
  { key: 'blocked', name: '已屏蔽', count: alerts.value.filter(a => a.blocked).length },
])

const filtered = computed(() => {
  let a = alerts.value
  if (sevOnly.value) a = a.filter(x => x.severity === sevOnly.value)
  switch (filter.value) {
    case 'active': return a.filter(x => x.status !== 3 && !x.blocked)
    case 'pending': return a.filter(x => x.status === 0)
    case 'suspended': return a.filter(x => x.status === 1)
    case 'acked': return a.filter(x => x.status === 2)
    case 'resolved': return a.filter(x => x.status === 3)
    case 'watched': return a.filter(x => x.watched)
    case 'blocked': return a.filter(x => x.blocked)
    default: return a
  }
})

function count(sev) { return alerts.value.filter(a => a.severity === sev && a.status !== 3).length }
function fmt(t) { return t ? String(t).replace('T', ' ').slice(0, 19) : '—' }

function beep(severe) {
  if (!sound.value) return
  try {
    const ctx = new (window.AudioContext || window.webkitAudioContext)()
    const o = ctx.createOscillator(), g = ctx.createGain()
    o.connect(g); g.connect(ctx.destination)
    // 严重告警: 高频三连音; 普通: 单音
    o.frequency.value = severe ? 1200 : 880
    g.gain.value = severe ? 0.12 : 0.08
    o.start()
    if (severe) {
      o.frequency.setValueAtTime(1200, ctx.currentTime)
      o.frequency.setValueAtTime(900, ctx.currentTime + 0.15)
      o.frequency.setValueAtTime(1200, ctx.currentTime + 0.3)
      o.stop(ctx.currentTime + 0.45)
    } else {
      o.stop(ctx.currentTime + 0.25)
    }
  } catch { /* 忽略 */ }
}

// V15 批量选择模式
const batchMode = ref(false)
const selIds = ref([])
const batchAll = computed(() => filtered.value.length > 0 && filtered.value.every(a => selIds.value.includes(a.id)))
function toggleBatchAll() {
  selIds.value = batchAll.value ? [] : filtered.value.map(a => a.id)
}
async function batchAct(action) {
  if (!selIds.value.length) return
  const label = action === 'acknowledge' ? '批量确认' : '批量结束'
  const reason = await showPrompt({ title: label, label: `${label} ${selIds.value.length} 条告警，说明（可留空）：` })
  if (reason === null) return
  const r = await api.post('/alerts/batch', { ids: selIds.value, action, reason: reason || label })
  if (r.code === 0) { selIds.value = []; await load(true) }
  else toastAuto(r.msg)
}

async function load(silent = true) {
  const r = await api.get('/alerts', { size: 200 })
  const items = r.data?.items || []
  // V15 按 ID 集合检测新告警(数量相同但有替换时也能发现)
  const newItems = items.filter(x => x.status < 3 && !seenIds.has(x.id))
  for (const a of items) a._new = newItems.includes(a)
  alerts.value = items
  for (const x of items) seenIds.add(x.id)
  if (newItems.length) {
    beep(newItems.some(x => x.severity === 3))
  }
  lastCount = items.length
}

async function act(a, action, reasonPrompt) {
  if (locked.value) return
  let reason = ''
  if (reasonPrompt) {
    reason = (await showPrompt({ title: '填写说明', label: reasonPrompt + '（可留空）' })) || ''
  }
  await api.post(`/alerts/${a.id}/action`, { action, reason })
  await load(true)
}

async function openLog(a) {
  const r = await api.get(`/alerts/${a.id}/logs`)
  logDrawer.value = { show: true, id: a.id, items: r.data?.items || [], cases: [], plans: [] }
  // V22-① 相似案例推荐
  const rc = await api.get('/fault-cases/recommend', { alert_id: a.id, limit: 4 })
  logDrawer.value.cases = (rc.data?.items || []).slice(0, 4)
  // V59 匹配应急预案推荐
  try {
    const rp = await api.get('/emergency-plans/match', { alert_id: a.id })
    logDrawer.value.plans = (rp.data?.list || []).slice(0, 3)
  } catch { /* 预案推荐失败不阻断 */ }
}

function exportCSV() {
  const head = ['ID', '级别', '状态', '消息', '触发时间', '确认时间', '结束时间', '关注', '屏蔽']
  const lines = [head.join(',')]
  filtered.value.forEach(a => lines.push([
    a.id, SEV[a.severity], STATUS[a.status], `"${(a.message || '').replace(/"/g, '""')}"`,
    fmt(a.triggered_at), a.acknowledged_at ? fmt(a.acknowledged_at) : '',
    a.resolved_at ? fmt(a.resolved_at) : '', a.watched ? '是' : '否', a.blocked ? '是' : '否'
  ].join(',')))
  const blob = new Blob(['\ufeff' + lines.join('\n')], { type: 'text/csv' })
  const el = document.createElement('a')
  el.href = URL.createObjectURL(blob)
  el.download = `告警导出_${new Date().toISOString().slice(0, 19).replace(/[:T]/g, '-')}.csv`
  el.click()
}

function toggleFs() {
  if (!document.fullscreenElement) document.documentElement.requestFullscreen()
  else document.exitFullscreen()
}

async function loadSite() {
  const r = await api.get('/sites')
  siteName.value = (r.data?.items || [])[0]?.name || ''
}

onMounted(async () => {
  const qs = Number(route.query.severity) || 0
  if (qs >= 1 && qs <= 3) sevOnly.value = qs
  await loadSite()
  await load(false)
  timer = setInterval(() => load(false), 15000)
  clockTimer = setInterval(() => { clock.value = new Date().toLocaleString('zh-CN', { hour12: false }) }, 1000)
})
onBeforeUnmount(() => { clearInterval(timer); clearInterval(clockTimer) })
</script>

<style scoped>
.aw { min-height: calc(100vh - 40px); }
.aw-top { display: flex; align-items: center; justify-content: space-between; margin-bottom: 14px; flex-wrap: wrap; gap: 10px; }
.aw-title { font-size: 20px; font-weight: 700; display: flex; align-items: center; gap: 10px; }
.aw-dot { width: 10px; height: 10px; border-radius: 50%; background: #f5222d; animation: pulse 1.5s infinite; }
@keyframes pulse { 0%,100% { opacity: 1 } 50% { opacity: .3 } }
.aw-site { font-size: 13px; font-weight: 400; color: var(--muted); }
.aw-clock { font-size: 13px; font-weight: 400; color: var(--muted); font-variant-numeric: tabular-nums; }
.aw-actions { display: flex; gap: 8px; }
.aw-btn { border: 1px solid var(--border); background: var(--bg); color: var(--text); border-radius: 8px; padding: 6px 12px; font-size: 13px; cursor: pointer; text-decoration: none; display: inline-flex; align-items: center; }
.aw-btn:hover { border-color: var(--primary); }
.aw-btn.on { background: var(--primary); color: #fff; border-color: var(--primary); }
.aw-stats { display: grid; grid-template-columns: repeat(7, 1fr); gap: 10px; margin-bottom: 14px; }
.aw-stat { background: var(--card); border: 1px solid var(--border); border-radius: 10px; padding: 10px 14px; display: flex; flex-direction: column; gap: 2px; }
.aw-stat b { font-size: 22px; font-variant-numeric: tabular-nums; }
.aw-stat span { font-size: 12px; color: var(--muted); }
.aw-stat.sev3 b { color: #f5222d; } .aw-stat.sev2 b { color: #fa8c16; } .aw-stat.sev1 b { color: #1677ff; }
.aw-stat.ok b { color: #52c41a; }
.aw-filter { display: flex; gap: 8px; margin-bottom: 14px; flex-wrap: wrap; }
.aw-tab { border: 0; background: rgba(22,119,255,.06); border-radius: 8px; padding: 6px 14px; font-size: 13px; cursor: pointer; color: var(--muted); }
.aw-tab em { font-style: normal; margin-left: 5px; background: rgba(0,0,0,.08); border-radius: 8px; padding: 0 6px; font-size: 11px; }
.aw-tab.on { background: var(--primary); color: #fff; }
.aw-tab.on em { background: rgba(255,255,255,.22); }
.aw-wall { display: flex; flex-direction: column; gap: 10px; }
.aw-card { display: flex; gap: 14px; align-items: center; background: var(--card); border: 1px solid var(--border); border-left-width: 4px; border-radius: 10px; padding: 12px 16px; }
.aw-card.sev3 { border-left-color: #f5222d; }
.aw-card.sev2 { border-left-color: #fa8c16; }
.aw-card.sev1 { border-left-color: #1677ff; }
.aw-card.flashed { animation: flash 1.2s ease 2; }
@keyframes flash { 0%,100% { background: var(--card) } 50% { background: rgba(245,34,45,.08) } }
.aw-card-l { display: flex; flex-direction: column; gap: 4px; min-width: 64px; }
.aw-chk { width: 15px; height: 15px; cursor: pointer; accent-color: var(--primary); }
.aw-sev { font-size: 12px; font-weight: 700; }
.sev3 .aw-sev { color: #f5222d; } .sev2 .aw-sev { color: #fa8c16; } .sev1 .aw-sev { color: #1677ff; }
.aw-st { font-size: 12px; color: var(--muted); }
.aw-card-m { flex: 1; min-width: 0; }
.aw-msg { font-size: 14px; font-weight: 600; margin-bottom: 4px; }
.aw-storm { color: #9d7bff; border: 1px solid rgba(157,123,255,.5); border-radius: 10px; padding: 0 8px; font-size: 11px; cursor: pointer; }
.aw-cases-sec { margin-top: 14px; border-top: 1px dashed var(--border); padding-top: 10px; }
.aw-cases-t { font-size: 13px; font-weight: 600; color: var(--accent); margin-bottom: 8px; }
.aw-case { padding: 7px 9px; background: var(--bg2); border-radius: 7px; font-size: 12.5px; margin-bottom: 5px; cursor: pointer; }
.aw-case:hover { outline: 1px solid var(--accent); }
.aw-case-sol { margin-top: 2px; }
.aw-meta { display: flex; gap: 14px; font-size: 12px; color: var(--muted); flex-wrap: wrap; }
.aw-link { color: var(--accent); cursor: pointer; font-weight: 600; }
.aw-link:hover { text-decoration: underline; }
.aw-deeplink { padding: 8px 14px; margin-bottom: 10px; background: var(--card); border: 1px solid var(--accent); border-radius: 8px; font-size: 13px; color: var(--text2, var(--text)); display: flex; align-items: center; gap: 10px; }
.aw-card-r { display: flex; gap: 6px; flex-wrap: wrap; justify-content: flex-end; }
.aw-op { border: 1px solid var(--border); background: none; border-radius: 6px; padding: 4px 10px; font-size: 12px; cursor: pointer; color: var(--text); }
.aw-op:hover { border-color: var(--primary); color: var(--primary); }
.aw-op.ack { border-color: #1677ff; color: #1677ff; }
.aw-op.done { border-color: #52c41a; color: #52c41a; }
.aw-lock-hint { font-size: 12px; color: var(--muted); }
.aw-empty { text-align: center; padding: 60px 0; color: var(--muted); }
.aw-empty.small { padding: 20px 0; }
.aw-empty-ico { font-size: 40px; color: #52c41a; margin-bottom: 8px; }
.log-item { padding: 10px 0; border-bottom: 1px dashed var(--border); display: flex; flex-direction: column; gap: 2px; font-size: 13px; }
@media (max-width: 768px) { .aw-stats { grid-template-columns: repeat(3, 1fr); } .aw-card { flex-wrap: wrap; } }
</style>
