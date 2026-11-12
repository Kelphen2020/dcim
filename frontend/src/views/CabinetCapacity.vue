<template>
  <div>
    <div class="page-head"><h1>机柜容量</h1></div>

    <!-- 总览热力矩阵（未选择具体机柜时展示） -->
    <div v-if="!cabId">
      <div class="toolbar">
        <select class="input" style="width:160px" v-model.number="siteId" @change="loadOverview">
          <option :value="0">全部站点</option>
          <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
        </select>
        <select class="input" style="width:130px" v-model="metric" @change="loadOverview">
          <option value="u">按 U 位占用</option><option value="power">按功率负载</option>
        </select>
        <span style="flex:1"></span>
        <span class="muted small">机柜 {{ overview.length }} 个 · 点击机柜查看详情</span>
      </div>
      <div class="card" style="padding:16px">
        <div class="legend">
          <span class="lg"><i class="lv0"></i>空闲(&lt;30%)</span>
          <span class="lg"><i class="lv1"></i>正常(30-70%)</span>
          <span class="lg"><i class="lv2"></i>较满(70-90%)</span>
          <span class="lg"><i class="lv3"></i>紧张(&gt;90%)</span>
          <span class="lg"><i class="al"></i><SvgIcon name="bell" />=有活跃告警</span>
        </div>
        <div v-for="grp in roomGroups" :key="grp.name" class="room-grp">
          <div class="room-title">{{ grp.name }}<span class="muted small">（{{ grp.items.length }} 柜）</span></div>
          <div class="cab-grid">
            <div v-for="cb in grp.items" :key="cb.id" class="cab-cell" :class="'lv'+levelOf(cb)" @click="selectCab(cb)"
                 :title="`${cb.code}\nU位 ${cb.used_u}/${cb.total_u} (${cb.u_pct.toFixed(0)}%)\n功率 ${cb.used_kw.toFixed(2)}/${cb.rated_kw}kW (${cb.power_pct.toFixed(0)}%)\n设备 ${cb.asset_cnt} 台${cb.temp_value!=null?`\n温度 ${cb.temp_value.toFixed(1)}°C`:''}`">
              <div class="cc-code">{{ cb.code }}<span v-if="cb.alerts>0" class="cc-al"><SvgIcon name="bell" /></span></div>
              <div class="cc-bar"><div class="cc-bar-in" :style="{width: Math.min(100, metric==='u'?cb.u_pct:cb.power_pct)+'%'}"></div></div>
              <div class="cc-val">{{ metric==='u' ? cb.u_pct.toFixed(0)+'% U' : cb.power_pct.toFixed(0)+'% P' }}</div>
              <div v-if="cb.temp_value!=null" class="cc-temp" :class="cb.temp_value>28?'hot':''">{{ cb.temp_value.toFixed(1) }}°C</div>
            </div>
          </div>
        </div>
        <div v-if="!overview.length" class="empty" style="padding:40px;text-align:center;color:var(--muted)">暂无机柜数据</div>
      </div>
      <!-- 容量预警汇总 -->
      <div v-if="warnList.length" class="card" style="margin-top:16px;padding:14px 16px">
        <div class="card-title"><SvgIcon name="alert" />容量预警（{{ warnList.length }}）</div>
        <div class="warn-list">
          <span v-for="cb in warnList" :key="cb.id" class="warn-chip" @click="selectCab(cb)">
            {{ cb.code }} <b>{{ cb.u_pct>=90?('U位'+cb.u_pct.toFixed(0)+'%'):('功率'+cb.power_pct.toFixed(0)+'%') }}</b>
          </span>
        </div>
      </div>

      <!-- V56 容量饱和预测 -->
      <div class="card" style="margin-top:16px;padding:14px 16px">
        <div class="card-title" style="display:flex;align-items:center;gap:8px">
          <SvgIcon name="trend" /><span>容量饱和预测（按近90天装机增速外推）</span>
          <span class="fc-sum">
            <span v-for="g in ['risk','warn','ok','stable']" :key="g" class="fc-chip" :class="'fc-'+g" :title="FCGN[g]+' '+(fc.summary[g]||0)+' 个站点'">{{ FCGN[g] }} {{ fc.summary[g]||0 }}</span>
          </span>
          <span style="flex:1"></span>
          <button v-if="auth.isAdmin" class="btn btn-mini" @click="evalForecast" :disabled="fc.busy">{{ fc.busy?'评估中…':'立即评估' }}</button>
        </div>
        <div v-if="!fc.items.length" class="muted small" style="padding:10px 0">暂无预测数据 —— 历史快照积累 ≥7 天后自动生成，或点「立即评估」。</div>
        <table v-else class="data-table fc-table">
          <thead><tr><th>站点</th><th>风险等级</th><th>U 位（增速/预计饱和）</th><th>功率（增速/预计饱和）</th><th>置信度</th><th>操作</th></tr></thead>
          <tbody><tr v-for="it in fc.items" :key="it.site_id" :class="{ 'fc-row-bad': it.grade==='risk' }">
            <td><b>{{ it.site_name }}</b></td>
            <td><span class="badge hg" :class="'hg-'+it.grade">{{ FCGN[it.grade]||it.grade }}</span></td>
            <td>{{ it.used_u }}/{{ it.total_u }}U · {{ it.u_slope>=0.001? ('+'+it.u_slope.toFixed(2)+'U/天 '):'无增长 ' }}
              <b v-if="it.u_full_at" :class="'fc-full-'+it.grade">{{ it.u_full_at }}</b><span v-else class="muted">不饱和</span></td>
            <td>{{ (it.used_power_w/1000).toFixed(1) }}/{{ (it.rated_power_w/1000).toFixed(1) }}kW · {{ it.power_slope>=0.001? ('+'+(it.power_slope/1000).toFixed(2)+'kW/天 '):'无增长 ' }}
              <b v-if="it.power_full_at" :class="'fc-full-'+it.grade">{{ it.power_full_at }}</b><span v-else class="muted">不饱和</span></td>
            <td><span v-if="it.low_confidence" class="badge fc-lowconf" title="增长规律性弱(r²<0.3)，仅供参考不报警">低置信</span>
                <span v-else class="muted small">U r²={{ (it.u_r2||0).toFixed(2) }} · P r²={{ (it.power_r2||0).toFixed(2) }}</span></td>
            <td class="ops">
              <button class="btn btn-mini" @click="openChart(it)">增长曲线</button>
              <button class="btn btn-mini" @click="openSim(it)">模拟扩容</button>
              <button class="btn btn-mini" v-if="!it.open_work_order_id" @click="capWO(it)">开扩容工单</button>
              <span v-else class="badge fc-wo">工单处理中</span>
            </td>
          </tr></tbody>
        </table>
      </div>
    </div>

    <!-- V56 增长曲线抽屉 -->
    <div v-if="chart.show" class="drawer-mask" @click.self="chart.show=false"><div class="drawer" style="width:760px">
      <div class="drawer-head"><h3><SvgIcon name="trend" />{{ chart.siteName }} · 容量增长与饱和预测</h3><button class="btn btn-mini" @click="chart.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <div class="fc-note muted small">实线为近 {{ capHistDays }} 天实际快照（按资产上架时间逐日累计），虚线为线性外推预测；横线为当前额定上限。预测基于近 90 天增速，装机节奏变化后会随每小时评估自动修正。</div>
        <div class="card" style="padding:10px"><div class="card-title">U 位占用趋势</div><div ref="chartU" style="height:240px"></div></div>
        <div class="card" style="padding:10px;margin-top:12px"><div class="card-title">功率占用趋势（kW）</div><div ref="chartW" style="height:240px"></div></div>
      </div>
    </div></div>

    <!-- 单柜详情 -->
    <!-- V74/B4-2 容量风险 → 模拟扩容抽屉 -->
    <div v-if="sim.show" class="drawer-mask" @click.self="sim.show=false"><div class="drawer" style="width:560px">
      <div class="drawer-head"><h3><SvgIcon name="gauge" />{{ sim.siteName }} · 模拟扩容</h3><button class="btn btn-mini" @click="sim.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <div class="fc-note muted small">选择本站机柜，输入拟新增设备的 U 高、功率与重量，按 U 位/功率/承重/制冷四维校验；确认可行后可转扩容规划工单。</div>
        <label style="display:flex;flex-direction:column;gap:4px;font-size:13px;color:var(--muted);margin-top:10px">目标机柜
          <select class="input" v-model.number="sim.cabId" :disabled="!sim.cabinets.length">
            <option v-if="!sim.cabinets.length" :value="0">该站点暂无机柜</option>
            <option v-for="c in sim.cabinets" :key="c.id" :value="c.id">{{ c.code }}（{{ roomName(c.room_id) }}）</option>
          </select>
        </label>
        <div class="wif-form" style="margin-top:10px">
          <label>U高<input class="input" type="number" min="1" v-model.number="sim.height_u" /></label>
          <label>功率W<input class="input" type="number" min="0" v-model.number="sim.power_w" /></label>
          <label>重量kg<input class="input" type="number" min="0" step="0.1" v-model.number="sim.weight_kg" /></label>
          <button class="btn btn-primary" :disabled="!sim.cabId||sim.busy" @click="runSim">{{ sim.busy?'评估中…':'评估' }}</button>
        </div>
        <div v-if="sim.res" class="wi-box" style="margin-top:12px">
          <div v-for="d in sim.res.checks" :key="d.dim" class="wi-row" :class="'wi-'+d.status">
            <span class="wi-ic"><SvgIcon :name="d.status==='ok'?'check-circle':d.status==='over'?'x-circle':d.status==='warn'?'alert':'info'" /></span>
            <span class="wi-label">{{ d.label }}</span>
            <span class="wi-msg">{{ d.message }}</span>
          </div>
          <div class="wi-verdict" :class="'wv-'+sim.res.verdict">{{ sim.res.conclusion }}</div>
        </div>
        <div v-else class="muted small" style="padding:10px 0">尚未评估。默认按 2U / 300W / 10kg 预填，可直接改数。</div>
      </div>
      <div class="drawer-foot">
        <button class="btn" @click="sim.show=false">关闭</button>
        <button class="btn btn-primary" v-if="sim.res" @click="simWO">转扩容规划工单</button>
      </div>
    </div></div>

    <!-- 单柜详情 -->
    <div v-if="cabId">
      <div class="toolbar">
        <button class="btn" @click="backOverview">← 返回总览</button>
        <select class="input" style="width:160px" v-model.number="siteId" @change="loadCabinets">
          <option :value="0">全部站点</option>
          <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
        </select>
        <select class="input" style="width:220px" v-model.number="cabId" @change="loadCapacity">
          <option v-for="c in cabinets" :key="c.id" :value="c.id">{{ c.code }}（{{ roomName(c.room_id) }}）</option>
        </select>
        <button class="btn" @click="loadCapacity">刷新</button>
        <span style="flex:1"></span>
        <button class="btn" @click="goAssets"><SvgIcon name="box" />资产台账</button>
        <button class="btn" @click="go3D"><SvgIcon name="cube" />3D查看</button>
      </div>

      <div v-if="cap" class="cap-wrap">
        <div class="cap-stats">
          <div class="cap-card">
            <div class="cap-label">U 位占用</div>
            <div class="cap-value">{{ cap.used_u }}<span class="cap-unit"> / {{ cap.total_u }}U</span></div>
            <div class="cap-bar"><div class="cap-bar-fill" :style="{ width: uPct + '%' }"></div></div>
            <div class="cap-sub">空闲 {{ cap.free_u }}U · 可上 {{ Math.floor(cap.free_u/2) }} 台2U设备</div>
          </div>
          <div class="cap-card">
            <div class="cap-label">功率使用</div>
            <div class="cap-value">{{ cap.used_power_w }}<span class="cap-unit">W</span></div>
            <div class="cap-sub">{{ ratedPower ? '额定 ' + ratedPower + 'kW · 使用率 ' + powerPct + '%' : '未设额定功率' }}</div>
            <div v-if="ratedPower" class="cap-bar"><div class="cap-bar-fill" :class="powerPct>90?'danger':powerPct>70?'warn':''" :style="{ width: Math.min(100,powerPct) + '%' }"></div></div>
          </div>
          <div class="cap-card">
            <div class="cap-label">承重使用</div>
            <div class="cap-value">{{ cap.used_weight_kg.toFixed(1) }}<span class="cap-unit">kg</span></div>
            <div class="cap-sub">{{ ratedWeight ? '上限 ' + ratedWeight + 'kg · 使用率 ' + weightPct + '%' : '未设承重上限' }}</div>
            <div v-if="ratedWeight" class="cap-bar"><div class="cap-bar-fill warn" :style="{ width: Math.min(100,weightPct) + '%' }"></div></div>
          </div>
          <div class="cap-card">
            <div class="cap-label">机柜环境</div>
            <div v-if="ctxPoints.length" class="env-rows">
              <div v-for="p in ctxPoints" :key="p.id" class="env-row" :class="{'env-alarm':p.status===2}">
                <span>{{ p.name }}</span>
                <b>{{ p.current_value!=null ? Number(p.current_value).toFixed(1)+(p.unit||'') : '—' }}</b>
              </div>
            </div>
            <div v-else class="cap-sub" style="margin-top:8px">无关联测点</div>
          </div>
        </div>

        <div class="two-col">
          <div>
            <!-- U位布局：2D 图 / 3D 视图（V55） -->
            <div class="card" style="margin-top:16px">
              <div class="card-title" style="display:flex;align-items:center;gap:8px">
                <span>{{ capView==='3d' ? '3D 机柜视图（点击设备拉出检查）' : 'U 位布局图（点击设备查看详情）' }}</span>
                <span style="flex:1"></span>
                <button class="btn btn-mini" :class="{ 'btn-primary': capView==='2d' }" @click="capView='2d'">2D 布局</button>
                <button class="btn btn-mini" :class="{ 'btn-primary': capView==='3d' }" @click="capView='3d'"><SvgIcon name="cube" />3D 视图</button>
              </div>
              <Cabinet3D v-if="capView==='3d'" :cabinet="cap.cabinet" :assets="cap.assets" :health-map="healthMap" @go-asset="goAsset3D" />
              <div v-else class="umap">
                <div v-for="u in uRows" :key="u" class="umap-row">
                  <span class="umap-u">{{ u }}U</span>
                  <div class="umap-cell" :class="{ filled: cap.u_map[u-1] }" @click="cap.u_map[u-1] && selectAsset(cap.u_map[u-1])">
                    <span v-if="cap.u_map[u-1]" class="umap-name">{{ assetName(cap.u_map[u-1]) }}</span>
                    <span v-else class="umap-free">空闲</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div>
            <!-- 业务上下文 -->
            <div class="card" style="margin-top:16px">
              <div class="card-title"><SvgIcon name="bell" />活跃告警（{{ ctx.alerts?.length || 0 }}）</div>
              <div v-if="ctx.alerts?.length" class="ctx-list">
                <div v-for="a in ctx.alerts" :key="a.id" class="ctx-row alarm" @click="goAlertWall">
                  <span class="badge" :class="'sev-'+a.severity">{{ SEV[a.severity] }}</span>
                  <span class="ctx-main">{{ a.point_name ? a.point_name + '：' : '' }}{{ a.message }}</span>
                  <span class="muted small">{{ fmtT(a.triggered_at) }}</span>
                </div>
              </div>
              <div v-else class="muted small" style="padding:8px 0">无活跃告警 <SvgIcon name="check" /></div>
            </div>
            <div class="card" style="margin-top:16px">
              <div class="card-title"><SvgIcon name="wrench" />维保与工单</div>
              <div v-if="ctx.maintenances?.length" class="ctx-list">
                <div v-for="(m,i) in ctx.maintenances" :key="i" class="ctx-row" @click="goMaint">
                  <span class="badge warn-badge">{{ m.days_left }}天后到期</span>
                  <span class="ctx-main">{{ m.asset_name }}（{{ m.end_at }}）</span>
                </div>
              </div>
              <div v-else class="muted small" style="padding:4px 0">30天内无维保到期</div>
              <div class="muted small" style="padding:4px 0">关联未完成工单：<b>{{ ctx.open_work_orders || 0 }}</b> 张<span v-if="ctx.open_work_orders" class="link" @click="goWO"> 查看 →</span></div>
            </div>
            <div class="card" style="margin-top:16px">
              <div class="card-title"><SvgIcon name="gauge" />What-if 上架试算</div>
              <div class="wif-form">
                <label>U高<input class="input" type="number" min="1" v-model.number="wif.height_u" /></label>
                <label>功率W<input class="input" type="number" min="0" v-model.number="wif.power_w" /></label>
                <label>重量kg<input class="input" type="number" min="0" step="0.1" v-model.number="wif.weight_kg" /></label>
                <button class="btn btn-primary" @click="runWhatIf">评估</button>
              </div>
              <div v-if="wif.res" class="wi-box">
                <div v-for="d in wif.res.checks" :key="d.dim" class="wi-row" :class="'wi-'+d.status">
                  <span class="wi-ic"><SvgIcon :name="d.status==='ok'?'check-circle':d.status==='over'?'x-circle':d.status==='warn'?'alert':'info'" /></span>
                  <span class="wi-label">{{ d.label }}</span>
                  <span class="wi-msg">{{ d.message }}</span>
                </div>
                <div class="wi-verdict" :class="'wv-'+wif.res.verdict">{{ wif.res.conclusion }}</div>
              </div>
              <div v-else class="muted small" style="padding:6px 0">输入拟上架设备的 U高/功率/重量，评估四维容量余量（U位/功率/承重/制冷）</div>
            </div>
            <div v-if="ctx.network_devices?.length" class="card" style="margin-top:16px">
              <div class="card-title"><SvgIcon name="globe" />柜内网络设备（{{ ctx.network_devices.length }}）</div>
              <div class="ctx-list">
                <div v-for="d in ctx.network_devices" :key="d.id" class="ctx-row">
                  <span class="badge" :class="'st-'+d.status">{{ NETST[d.status] }}</span>
                  <span class="ctx-main">{{ d.name }}<span class="muted small">（{{ d.mgmt_ip || '无IP' }}）</span></span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 在架设备 -->
        <div class="card" style="margin-top:16px">
          <div class="card-title">在架设备（{{ cap.assets.length }}）</div>
          <div class="table-wrap">
            <table class="data-table">
              <thead><tr><th>编码</th><th>名称</th><th>U 位</th><th>高度</th><th>功耗</th><th>重量</th><th>IP</th><th>操作</th></tr></thead>
              <tbody>
                <tr v-for="a in cap.assets" :key="a.id">
                  <td><span class="mono">{{ a.code }}</span></td>
                  <td>{{ a.name }}</td>
                  <td>{{ a.start_u }}~{{ a.start_u + a.height_u - 1 }}U</td>
                  <td>{{ a.height_u }}U</td>
                  <td>{{ a.power_w }}W</td>
                  <td>{{ a.weight_kg }}kg</td>
                  <td class="small mono">{{ a.ip || '-' }}</td>
                  <td class="ops"><button class="btn btn-mini" @click="selectAsset(a.id)">详情</button></td>
                </tr>
                <tr v-if="!cap.assets.length"><td colspan="8" class="empty">机柜为空</td></tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
      <div v-else class="card empty-hint">加载中…</div>
    </div>
  </div>
</template>
<script setup>
import { ref, reactive, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import api from '../api'
import { toastAuto, showConfirm } from '../ui-dialog'
import * as echarts from 'echarts'
import Cabinet3D from '../components/Cabinet3D.vue'
import { useAuthStore } from '../stores/auth'
import { useSettingsStore } from '../stores/settings'
const auth = useAuthStore()
const settings = useSettingsStore()
const FCGN = { ok: '充足', warn: '180天内', risk: '90天内', stable: '无增长' }
const capHistDays = 120

const route = useRoute(), router = useRouter()
const SEV = { 1: '提示', 2: '警告', 3: '严重' }
const NETST = { 0: '离线', 1: '在线', 2: '维护', 3: '故障' }
const sites = ref([]), rooms = ref([]), cabinets = ref([])
const siteId = ref(0), cabId = ref(0), metric = ref('u')
const cap = ref(null), overview = ref([]), ctx = ref({})
const wif = reactive({ height_u: 2, power_w: 300, weight_kg: 10, res: null })
// V55 3D 机柜视图：默认 3D（用户抱怨 2D 单一），记住用户切换偏好
const capView = ref(localStorage.getItem('dcim_cap_view') || '3d')
watch(capView, v => localStorage.setItem('dcim_cap_view', v))
const healthMap = ref({})
async function loadHealthMap() {
  try {
    const r = await api.get('/asset-health', { size: 500 })
    if (r.code === 0) {
      const m = {}
      ;(r.data.items || []).forEach(x => { m[x.asset_id] = { score: x.score, grade: x.grade } })
      healthMap.value = m
    }
  } catch (e) { /* 静默：无健康数据时 3D 健康着色显示灰色 */ }
}
function goAsset3D(a) { router.push({ path: '/assets', query: { keyword: a.code } }) }

// ===== V56 容量饱和预测 =====
const fc = reactive({ items: [], summary: {}, busy: false })
const chart = reactive({ show: false, siteName: '', siteId: 0 })
const sim = reactive({ show: false, siteId: 0, siteName: '', cabinets: [], cabId: 0, height_u: 2, power_w: 300, weight_kg: 10, res: null, busy: false })
const chartU = ref(null), chartW = ref(null)
let ecU = null, ecW = null
async function loadForecast() {
  const r = await api.get('/capacity/forecast')
  if (r.code === 0) { fc.items = r.data.items || []; fc.summary = r.data.summary || {} }
}
async function evalForecast() {
  fc.busy = true
  try {
    const r = await api.post('/capacity/forecast/evaluate', {})
    if (r.code === 0) await loadForecast(); else toastAuto(r.msg)
  } finally { fc.busy = false }
}
async function capWO(it) {
  if (!(await showConfirm({ message: `为「${it.site_name}」创建容量扩容规划工单？` }))) return
  const r = await api.post(`/capacity/forecast/${it.site_id}/work-order`, {})
  if (r.code === 0) { toastAuto('扩容规划工单已创建，可在运维工单页跟踪'); await loadForecast() } else toastAuto(r.msg)
}
async function openChart(it) {
  chart.siteName = it.site_name; chart.siteId = it.site_id; chart.show = true
  const r = await api.get(`/capacity/forecast/${it.site_id}`)
  if (r.code !== 0) return
  await nextTick()
  const d = r.data
  const pal = settings.chartPalette || ['#409eff', '#67c23a', '#e6a23c', '#f56c6c', '#9d7bff', '#1e88e5']
  const hist = d.history || [], fcs = d.forecast || [], rec = d.record || {}
  const days = [...hist.map(x => x.day), ...fcs.map(x => x.day)]
  const mk = (el, inst, histKey, limitKey, fcKey, unit, fcMul) => {
    if (!el) return null
    if (inst) inst.dispose()
    inst = echarts.init(el)
    const axisColor = getComputedStyle(document.documentElement).getPropertyValue('--muted').trim() || '#8a93a6'
    inst.setOption({
      grid: { left: 46, right: 16, top: 24, bottom: 26 },
      tooltip: { trigger: 'axis' },
      xAxis: { type: 'category', data: days, axisLabel: { color: axisColor, fontSize: 10, formatter: v => v.slice(5) }, axisLine: { lineStyle: { color: axisColor } } },
      yAxis: { type: 'value', name: unit, nameTextStyle: { color: axisColor }, axisLabel: { color: axisColor }, splitLine: { lineStyle: { color: 'rgba(128,148,178,.15)' } } },
      series: [
        { name: '实际', type: 'line', smooth: true, showSymbol: false, data: [...hist.map(x => x[histKey]), ...fcs.map(() => null)], lineStyle: { width: 2.4, color: pal[0] }, areaStyle: { color: pal[0], opacity: 0.08 },
          markLine: { silent: true, symbol: 'none', data: [{ yAxis: rec[limitKey], label: { formatter: '上限 ' + rec[limitKey], color: '#f56c6c' }, lineStyle: { color: '#f56c6c', type: 'dashed' } }] } },
        { name: '预测', type: 'line', smooth: true, showSymbol: false, data: [...hist.map(() => null), ...fcs.map(x => x[fcKey] * (fcMul || 1))], lineStyle: { width: 2, type: 'dashed', color: pal[3] } },
      ],
    })
    return inst
  }
  ecU = mk(chartU.value, ecU, 'used_u', 'total_u', 'u', 'U')
  ecW = mk(chartW.value, ecW, 'used_power_w', 'rated_power_w', 'w', 'kW', 0.001)
}
const redrawCharts = () => { if (ecU) ecU.resize(); if (ecW) ecW.resize() }
const onThemeFC = () => { if (chart.show && chart.siteId) openChart({ site_id: chart.siteId, site_name: chart.siteName }) }
async function openSim(it) {
  Object.assign(sim, { show: true, siteId: it.site_id, siteName: it.site_name, cabinets: [], cabId: 0, height_u: 2, power_w: 300, weight_kg: 10, res: null, busy: true })
  try {
    const r = await api.get('/cabinets', { site_id: it.site_id, size: 500 })
    sim.cabinets = r.data?.items || []
    if (sim.cabinets.length) sim.cabId = sim.cabinets[0].id
  } finally { sim.busy = false }
}
async function runSim() {
  if (!sim.cabId) return
  sim.busy = true
  try {
    const r = await api.get(`/cabinets/${sim.cabId}/what-if`, { height_u: sim.height_u || 1, power_w: sim.power_w || 0, weight_kg: sim.weight_kg || 0 })
    sim.res = r.code === 0 ? r.data : null
    if (r.code !== 0) toastAuto(r.msg)
  } finally { sim.busy = false }
}
async function simWO() {
  if (!(await showConfirm({ message: `为「${sim.siteName}」创建容量扩容规划工单？` }))) return
  const r = await api.post(`/capacity/forecast/${sim.siteId}/work-order`, {})
  if (r.code === 0) { toastAuto('扩容规划工单已创建，可在运维工单页跟踪'); sim.show = false; await loadForecast() } else toastAuto(r.msg)
}

async function runWhatIf() {
  if (!cabId.value) return
  const r = await api.get(`/cabinets/${cabId.value}/what-if`, { height_u: wif.height_u || 1, power_w: wif.power_w || 0, weight_kg: wif.weight_kg || 0 })
  wif.res = r.code === 0 ? r.data : null
}

const roomName = id => (rooms.value.find(r => r.id === id) || {}).name || '-'
const assetById = computed(() => Object.fromEntries((cap.value?.assets || []).map(a => [a.id, a])))
const assetName = id => assetById.value[id]?.name || ''
const ctxPoints = computed(() => (ctx.value.points || []).filter(p => p.current_value != null).slice(0, 4))

const uRows = computed(() => {
  const n = cap.value?.total_u || 42
  return Array.from({ length: n }, (_, i) => n - i)
})
const uPct = computed(() => cap.value ? Math.round(cap.value.used_u / cap.value.total_u * 100) : 0)
const ratedPower = computed(() => cap.value?.cabinet?.rated_power_kw || 0)
const ratedWeight = computed(() => cap.value?.cabinet?.max_weight_kg || 0)
const powerPct = computed(() => ratedPower.value ? Math.min(100, Math.round(cap.value.used_power_w / (ratedPower.value * 1000) * 100)) : 0)
const weightPct = computed(() => ratedWeight.value ? Math.min(100, Math.round(cap.value.used_weight_kg / ratedWeight.value * 100)) : 0)

const roomGroups = computed(() => {
  const m = {}
  overview.value.forEach(cb => { (m[cb.room_name || '未分组'] = m[cb.room_name || '未分组'] || []).push(cb) })
  return Object.entries(m).map(([name, items]) => ({ name, items }))
})
const warnList = computed(() => overview.value.filter(cb => cb.u_pct >= 90 || cb.power_pct >= 90))
function levelOf(cb) {
  const p = metric.value === 'u' ? cb.u_pct : cb.power_pct
  return p >= 90 ? 3 : p >= 70 ? 2 : p >= 30 ? 1 : 0
}
const fmtT = t => t ? String(t).replace('T', ' ').slice(5, 16) : '-'

async function loadBase() {
  const [rs, rr] = await Promise.all([api.get('/sites', { size: 100 }), api.get('/rooms', { size: 200 })])
  if (rs.code === 0) sites.value = rs.data.items || []
  if (rr.code === 0) rooms.value = rr.data.items || []
  const qSite = Number(route.query.site_id) || 0
  const qCab = Number(route.query.cabinet_id) || 0
  if (qSite) siteId.value = qSite
  if (qCab) {
    const res = await api.get('/cabinets', { size: 500 })
    if (res.code === 0) cabinets.value = res.data.items || []
    const cab = cabinets.value.find(c => c.id === qCab)
    if (cab && cab.site_id && !qSite) { siteId.value = cab.site_id; await loadCabinets() }
    cabId.value = qCab
    loadCapacity()
    return
  }
  await loadCabinets()
  loadOverview()
}

async function loadCabinets() {
  const params = { size: 500 }
  if (siteId.value) params.site_id = siteId.value
  const res = await api.get('/cabinets', params)
  if (res.code === 0) cabinets.value = res.data.items || []
}

async function loadOverview() {
  if (cabId.value) return
  const params = {}
  if (siteId.value) params.site_id = siteId.value
  const r = await api.get('/capacity-overview', params)
  if (r.code === 0) overview.value = r.data.items || []
}

async function loadCapacity() {
  cap.value = null; ctx.value = {}; wif.res = null
  if (!cabId.value) return
  const [res, cr] = await Promise.all([
    api.get(`/cabinets/${cabId.value}/capacity`),
    api.get(`/cabinets/${cabId.value}/context`),
  ])
  if (res.code === 0) cap.value = res.data
  if (cr.code === 0) ctx.value = cr.data
}

function selectCab(cb) {
  cabId.value = cb.id
  siteId.value = cb.site_id || siteId.value
  loadCabinets().then(loadCapacity)
}
function backOverview() {
  cabId.value = 0
  router.replace({ path: '/capacity', query: siteId.value ? { site_id: siteId.value } : {} })
  loadOverview()
}
function selectAsset(id) {
  const a = assetById.value[id]
  if (a) router.push({ path: '/assets', query: { keyword: a.code } })
}
function goAssets() { router.push({ path: '/assets', query: { cabinet_id: cabId.value } }) }
function go3D() { router.push({ path: '/room-3d', query: { cabinet_id: cabId.value } }) }
function goAlertWall() { router.push({ path: '/alert-wall' }) }
function goMaint() { router.push({ path: '/maintenances' }) }
function goWO() { router.push({ path: '/work-orders', query: { status: 'open' } }) }

watch(() => route.query.cabinet_id, v => {
  const n = Number(v) || 0
  if (n && n !== cabId.value) { cabId.value = n; loadCabinets().then(loadCapacity) }
  else if (!n && cabId.value) backOverview()
})

onMounted(() => { loadBase(); loadHealthMap(); loadForecast(); window.addEventListener('resize', redrawCharts); document.addEventListener('dcim-theme-change', onThemeFC) })
onBeforeUnmount(() => { window.removeEventListener('resize', redrawCharts); document.removeEventListener('dcim-theme-change', onThemeFC); ecU?.dispose(); ecW?.dispose() })
</script>

<style scoped>
.cap-wrap { }
.fc-sum { display: flex; gap: 6px; margin-left: 8px; }
.fc-chip { font-size: 11.5px; padding: 2px 9px; border-radius: 10px; border: 1px solid var(--border); color: var(--muted); }
.fc-chip.fc-risk { color: #f56c6c; border-color: rgba(245,108,108,.45); font-weight: 700; }
.fc-chip.fc-warn { color: #e6a23c; border-color: rgba(230,162,60,.45); }
.fc-chip.fc-ok { color: #67c23a; border-color: rgba(103,194,58,.4); }
.badge.hg { font-weight: 700; }
.badge.hg-ok { color: #67c23a; border-color: rgba(103,194,58,.45); }
.badge.hg-warn { color: #e6a23c; border-color: rgba(230,162,60,.45); }
.badge.hg-risk { color: #f56c6c; border-color: rgba(245,108,108,.45); }
.badge.hg-stable { color: #909399; border-color: rgba(144,147,153,.4); }
.badge.fc-lowconf { color: #e6a23c; border-color: rgba(230,162,60,.45); }
.badge.fc-wo { color: #409eff; border-color: rgba(64,158,255,.4); }
.fc-row-bad td { background: rgba(245,108,108,.045); }
.fc-full-risk { color: #f56c6c; } .fc-full-warn { color: #e6a23c; } .fc-full-ok { color: var(--text); }
.fc-note { margin-bottom: 12px; line-height: 1.6; }
.cap-stats { display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; }
.cap-card { background: var(--card); border-radius: 10px; padding: 16px 18px; box-shadow: var(--shadow-sm); }
.cap-label { font-size: 12.5px; color: var(--text2); }
.cap-value { font-size: 26px; font-weight: 700; color: var(--primary); margin: 6px 0; }
.cap-unit { font-size: 14px; color: var(--text2); font-weight: 500; }
.cap-sub { font-size: 12px; color: var(--text2); margin-bottom: 6px; }
.cap-bar { height: 6px; background: var(--bg2); border-radius: 3px; overflow: hidden; }
.cap-bar-fill { height: 100%; background: var(--primary); border-radius: 3px; transition: width .3s; }
.cap-bar-fill.warn { background: #f59e0b; }
.cap-bar-fill.danger { background: #f56c6c; }
.env-rows { margin-top: 6px; }
.env-row { display: flex; justify-content: space-between; font-size: 12px; padding: 3px 0; border-bottom: 1px dashed var(--border); }
.env-row b { color: var(--primary); }
.env-row.env-alarm b { color: #f56c6c; }
.two-col { display: grid; grid-template-columns: 1fr 1fr; gap: 0 16px; }
@media (max-width: 1100px) { .two-col { grid-template-columns: 1fr; } .cap-stats { grid-template-columns: repeat(2, 1fr); } }
.umap { display: flex; flex-direction: column; gap: 2px; max-height: 560px; overflow-y: auto; padding-right: 8px; }
.umap-row { display: flex; align-items: center; gap: 8px; }
.umap-u { width: 40px; text-align: right; font-size: 11px; color: var(--text3); font-family: Consolas, monospace; }
.umap-cell { flex: 1; height: 15px; background: var(--bg2); border-radius: 3px; display: flex; align-items: center; padding: 0 8px; }
.umap-cell.filled { background: var(--primary); cursor: pointer; }
.umap-cell.filled:hover { filter: brightness(1.2); }
.umap-name { font-size: 10px; color: #fff; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.umap-free { font-size: 9px; color: var(--text3); opacity: .6; }
.empty-hint { text-align: center; color: var(--text3); padding: 60px 0; }
/* 总览热力 */
.legend { display: flex; gap: 16px; flex-wrap: wrap; margin-bottom: 14px; font-size: 12px; color: var(--muted); }
.lg i { display: inline-block; width: 12px; height: 12px; border-radius: 3px; margin-right: 4px; vertical-align: -2px; }
.lv0 { background: var(--bg2); border: 1px solid var(--border); } .lv1 { background: rgba(103,194,58,.55); } .lv2 { background: rgba(230,162,60,.65); } .lv3 { background: rgba(245,108,108,.7); }
.room-grp { margin-bottom: 18px; }
.room-title { font-weight: 700; font-size: 13.5px; margin-bottom: 8px; }
.cab-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(108px, 1fr)); gap: 8px; }
.cab-cell { border: 1px solid var(--border); border-radius: 8px; padding: 8px 10px; cursor: pointer; background: var(--bg2); transition: transform .12s, box-shadow .12s; }
.cab-cell:hover { transform: translateY(-2px); box-shadow: var(--shadow-sm); border-color: var(--accent); }
.cab-cell.lv1 { background: rgba(103,194,58,.12); } .cab-cell.lv2 { background: rgba(230,162,60,.14); } .cab-cell.lv3 { background: rgba(245,108,108,.14); }
.cc-code { font-weight: 700; font-size: 13px; display: flex; justify-content: space-between; }
.cc-al { font-size: 11px; }
.cc-bar { height: 5px; background: rgba(128,128,128,.2); border-radius: 3px; margin: 5px 0 3px; overflow: hidden; }
.cc-bar-in { height: 100%; background: var(--accent); border-radius: 3px; }
.cab-cell.lv1 .cc-bar-in { background: #67c23a; } .cab-cell.lv2 .cc-bar-in { background: #e6a23c; } .cab-cell.lv3 .cc-bar-in { background: #f56c6c; }
.cc-val { font-size: 11px; color: var(--muted); }
.cc-temp { font-size: 11px; color: #409eff; margin-top: 2px; }
.cc-temp.hot { color: #f56c6c; font-weight: 700; }
.warn-list { display: flex; flex-wrap: wrap; gap: 8px; }
.warn-chip { padding: 4px 12px; background: rgba(245,108,108,.1); border: 1px solid rgba(245,108,108,.4); border-radius: 14px; font-size: 12.5px; cursor: pointer; }
.warn-chip:hover { background: rgba(245,108,108,.2); }
.warn-chip b { color: #f56c6c; }
/* 上下文 */
.ctx-list { display: flex; flex-direction: column; gap: 6px; padding: 4px 0; }
.ctx-row { display: flex; align-items: center; gap: 8px; font-size: 12.5px; padding: 6px 8px; background: var(--bg2); border-radius: 6px; cursor: pointer; }
.ctx-row:hover { background: var(--border); }
.ctx-row.alarm { background: rgba(245,108,108,.08); }
.ctx-main { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.badge.sev-1 { color: #909399; border-color: rgba(144,147,153,.4); } .badge.sev-2 { color: #e6a23c; border-color: rgba(230,162,60,.4); } .badge.sev-3 { color: #f56c6c; border-color: rgba(245,108,108,.4); }
.badge.warn-badge { color: #e6a23c; border-color: rgba(230,162,60,.4); }
.badge.st-1 { color: #67c23a; border-color: rgba(103,194,58,.4); } .badge.st-0 { color: #909399; border-color: rgba(144,147,153,.4); } .badge.st-2 { color: #e6a23c; border-color: rgba(230,162,60,.4); } .badge.st-3 { color: #f56c6c; border-color: rgba(245,108,108,.4); }
.link { color: var(--accent); cursor: pointer; font-weight: 600; }
.card-title { font-weight: 700; font-size: 14px; margin-bottom: 8px; }
.wif-form { display: flex; gap: 10px; align-items: flex-end; flex-wrap: wrap; margin-bottom: 8px; }
.wif-form label { display: flex; flex-direction: column; gap: 4px; font-size: 12px; color: var(--muted); }
.wif-form .input { width: 86px; }
.wi-box{margin-top:8px;padding:10px 12px;border:1px solid var(--border);border-radius:8px;background:var(--bg2)}
.wi-row{display:flex;align-items:flex-start;gap:8px;padding:5px 0;font-size:12.5px;border-bottom:1px dashed var(--border)}
.wi-row:last-of-type{border-bottom:none}
.wi-ic{flex-shrink:0;margin-top:2px}
.wi-label{flex-shrink:0;width:34px;font-weight:600}
.wi-msg{color:var(--text2);line-height:1.5}
.wi-ok .wi-ic,.wi-ok .wi-label{color:#16a34a}
.wi-warn .wi-ic,.wi-warn .wi-label{color:#d97706}
.wi-over .wi-ic,.wi-over .wi-label{color:#dc2626}
.wi-na .wi-ic,.wi-na .wi-label{color:var(--muted)}
.wi-verdict{margin-top:8px;padding:7px 10px;border-radius:6px;font-size:12.5px;font-weight:600}
.wv-ok{background:rgba(22,163,74,.08);color:#16a34a}
.wv-warn{background:rgba(217,119,6,.08);color:#d97706}
.wv-over{background:rgba(220,38,38,.08);color:#dc2626}
</style>
