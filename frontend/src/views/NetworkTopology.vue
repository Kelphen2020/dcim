<template>
  <div>
    <div class="page-head">
      <h1>网络拓扑</h1>
      <div class="toolbar-inline">
        <select class="input" style="width:160px" v-model.number="siteId" @change="reload">
          <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
        </select>
        <button class="btn" @click="autoLayout"><SvgIcon name="compass" />自动布局</button>
        <button class="btn btn-primary" @click="saveLayout" :disabled="saving">{{ saving ? '保存中…' : '保存布局' }}</button>
        <span class="muted" style="font-size:13px">设备 {{ devices.length }} · 链路 {{ links.length }}</span>
      </div>
    </div>
    <div class="net-wrap card">
      <div class="net-legend">
        <span v-for="(v,k) in TYPE_META" :key="k"><i :style="{background:v.color}"></i>{{ v.name }}</span>
        <span class="lg-status"><i class="s1"></i>在线 <i class="s0"></i>离线 <i class="s2"></i>维护 <i class="s3"></i>故障</span>
      </div>
      <div ref="canvasRef" class="net-canvas"></div>
      <div v-if="!devices.length" class="empty net-empty">该站点暂无网络设备</div>
    </div>

    <!-- 设备详情抽屉 -->
    <div v-if="sel" class="drawer-mask" @click.self="sel=null">
      <div class="drawer" style="width:520px">
        <div class="drawer-head"><h3><SvgIcon name="globe" />{{ sel.name }}</h3><button class="btn btn-mini" @click="sel=null"><SvgIcon name="x" /></button></div>
        <div class="drawer-body">
          <!-- V14 业务速览 -->
          <div class="biz-sum">
            <div class="bstat"><b>{{ sel.port_total }}</b><span>端口</span></div>
            <div class="bstat"><b :style="{color:'#67c23a'}">{{ sel.port_up }}</b><span>UP</span></div>
            <div class="bstat"><b>{{ sel.port_linked }}</b><span>已连线</span></div>
            <div class="bstat"><b :style="{color: sel.port_total ? '#409eff' : '#f56c6c'}">{{ sel.port_total ? Math.round((sel.port_total - sel.port_linked) / sel.port_total * 100) + '%' : '-' }}</b><span>空闲率</span></div>
          </div>
          <table class="kv-table">
            <tr><td>编码</td><td>{{ sel.code }}</td></tr>
            <tr><td>类型</td><td><span class="badge">{{ TYPE_META[sel.type]?.name || sel.type }}</span></td></tr>
            <tr><td>厂商/型号</td><td>{{ sel.vendor || '-' }} / {{ sel.model || '-' }}</td></tr>
            <tr><td>管理IP</td><td>{{ sel.mgmt_ip || '-' }}<span v-if="sel.ip_cnt" class="muted small">（IPAM 分配 {{ sel.ip_cnt }} 个）</span></td></tr>
            <tr><td>位置</td><td>
              <span v-if="sel.cabinet_code"><a class="link" @click="goCapacity(sel)">{{ sel.cabinet_code }}</a></span>
              <span v-else class="muted">未关联机柜</span>
            </td></tr>
            <tr><td>台账资产</td><td>
              <a v-if="sel.asset_name" class="link" @click="goAsset(sel)">{{ sel.asset_name }} →</a>
              <span v-else class="muted small">未匹配（按管理IP关联台账）</span>
            </td></tr>
            <tr><td>配置备份</td><td>
              <span v-if="sel.backup_cnt">{{ sel.backup_cnt }} 份<span v-if="sel.last_backup" class="muted small"> · 最近 {{ sel.last_backup }}</span>
                <a class="link small" style="margin-left:8px" @click="goBackups(sel)">查看 →</a></span>
              <span v-else class="warn-small"><SvgIcon name="alert" />从未备份 <a class="link small" @click="goBackups(sel)">去备份 →</a></span>
            </td></tr>
            <tr><td>状态</td><td><span class="badge" :class="'st-'+sel.status">{{ ST[sel.status] }}</span><span v-if="sel.last_seen_at" class="muted small"> · 最后在线 {{ String(sel.last_seen_at).replace('T',' ').slice(0,16) }}</span></td></tr>
          </table>

          <!-- 端口明细（含对端） -->
          <div style="margin-top:14px">
            <div style="font-weight:600;margin-bottom:8px">端口明细（{{ devPorts.length }}）</div>
            <div class="table-wrap" style="max-height:260px;overflow-y:auto">
            <table class="data-table">
              <thead><tr><th>端口</th><th>速率</th><th>物理</th><th>对端</th><th>链路</th></tr></thead>
              <tbody><tr v-for="p in devPorts" :key="p.id">
                <td class="mono small">{{ p.name }}</td>
                <td class="small">{{ p.speed || '-' }}</td>
                <td><span class="pdot" :class="p.status===1?'up':'down'"></span></td>
                <td class="small">{{ p.peer_device ? (p.peer_device + ' / ' + p.peer_port) : '-' }}</td>
                <td>
                  <span v-if="p.link_status>=0" class="badge" :class="p.link_status===1?'st-1':'st-0'">{{ p.link_status===1?'UP':'DOWN' }}</span>
                  <span v-else class="muted small">未连线</span>
                  <span v-if="p.bandwidth" class="muted small"> {{ p.bandwidth }}</span>
                </td>
              </tr></tbody>
            </table></div>
          </div>

          <div v-if="devLinks(sel.id).length" style="margin-top:14px">
            <div style="font-weight:600;margin-bottom:8px">关联链路（{{ devLinks(sel.id).length }}）<a class="link small" style="margin-left:8px" @click="goLinks">台账管理 →</a></div>
            <table class="data-table">
              <thead><tr><th>对端设备</th><th>带宽</th><th>类型</th><th>状态</th></tr></thead>
              <tbody><tr v-for="l in devLinks(sel.id)" :key="l.id">
                <td>{{ peerName(l, sel.id) }}</td><td>{{ l.bandwidth || '-' }}</td>
                <td>{{ LT[l.link_type] || l.link_type }}</td>
                <td><span class="badge" :class="l.status===1?'st-1':'st-0'">{{ l.status===1?'UP':'DOWN' }}</span></td>
              </tr></tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import { Graph } from '@antv/x6'

const router = useRouter()
const TYPE_META = {
  firewall: { name: '防火墙', color: '#f56c6c', level: 0, icon: '' },
  router:   { name: '路由器', color: '#e6a23c', level: 0, icon: '' },
  switch:   { name: '交换机', color: '#409eff', level: 1, icon: '' },
  controller: { name: 'AC控制器', color: '#9b7bff', level: 2, icon: '' },
  ap:       { name: '无线AP', color: '#67c23a', level: 2, icon: '' },
}
const ST = { 0: '离线', 1: '在线', 2: '维护', 3: '故障' }
const ST_COLOR = { 0: '#909399', 1: '#67c23a', 2: '#e6a23c', 3: '#f56c6c' }
const LT = { ethernet: '以太网', fiber: '光纤', trunk: 'Trunk' }

const sites = ref([]), siteId = ref(0)
const devices = ref([]), ports = ref([]), links = ref([])
const sel = ref(null)
const devPorts = ref([])
const saving = ref(false)
const canvasRef = ref(null)
let graph = null

// 打开设备详情时拉端口明细
async function openDevice(d) {
  sel.value = d
  devPorts.value = []
  const r = await api.get(`/network-devices/${d.id}/ports`)
  if (r.code === 0) devPorts.value = r.data?.items || []
}
function goCapacity(d) { if (d.cabinet_id) router.push({ path: '/capacity', query: { cabinet_id: d.cabinet_id } }) }
function goAsset(d) { if (d.asset_name) router.push({ path: '/assets', query: { keyword: d.code } }) }
function goBackups(d) { router.push({ path: '/config-backups', query: { device_id: d.id } }) }
function goLinks() { router.push({ path: '/network-links' }) }

// 端口 → 设备 映射
const portDevice = {}
function buildPortMap() {
  Object.keys(portDevice).forEach(k => delete portDevice[k])
  ports.value.forEach(p => { portDevice[p.id] = p.device_id })
}
function devLinks(devId) {
  return links.value.filter(l => portDevice[l.port_a_id] === devId || portDevice[l.port_b_id] === devId)
}
function peerName(l, devId) {
  const peerPort = portDevice[l.port_a_id] === devId ? l.port_b_id : l.port_a_id
  const peerId = portDevice[peerPort]
  const d = devices.value.find(x => x.id === peerId)
  return d ? d.name : '-'
}

// 层级自动布局：按设备类型分层，层内水平居中
function computeLayout(list) {
  const levels = { 0: [], 1: [], 2: [] }
  list.forEach(d => {
    const lv = TYPE_META[d.type]?.level ?? 1
    levels[lv] = levels[lv] || []
    levels[lv].push(d)
  })
  const NODE_W = 150, NODE_H = 78, X_GAP = 60, Y_GAP = 160, Y0 = 70
  const pos = {}
  const widest = Math.max(1, ...Object.values(levels).map(arr => arr.length))
  const totalW = widest * (NODE_W + X_GAP)
  Object.entries(levels).forEach(([lv, arr]) => {
    const rowW = arr.length * (NODE_W + X_GAP)
    let x = (totalW - rowW) / 2 + 40
    arr.forEach(d => {
      pos[d.id] = { x, y: Y0 + (+lv) * Y_GAP }
      x += NODE_W + X_GAP
    })
  })
  return pos
}

function buildGraph(savedPos) {
  if (graph) { graph.dispose(); graph = null }
  const list = devices.value
  if (!list.length) return
  buildPortMap()
  const auto = computeLayout(list)
  const pos = {}
  list.forEach(d => { pos[d.id] = (savedPos && savedPos[d.id]) || auto[d.id] || { x: 80, y: 80 } })

  graph = new Graph({
    container: canvasRef.value,
    autoResize: true,
    background: { color: '#0b1524' },
    grid: { visible: true, type: 'dot', size: 12, args: { color: 'rgba(120,150,190,.12)', thickness: 1 } },
    panning: { enabled: true, eventTypes: ['leftMouseDown', 'mouseWheel'] },
    mousewheel: { enabled: true, modifiers: ['ctrl', 'meta'], minScale: 0.3, maxScale: 2 },
    connecting: { enabled: false },
    interacting: { nodeMovable: true, edgeMovable: false },
  })

  const NODE_W = 150, NODE_H = 78
  list.forEach(d => {
    const meta = TYPE_META[d.type] || { name: d.type, color: '#909399', icon: '' }
    const stColor = ST_COLOR[d.status] ?? '#909399'
    graph.addNode({
      id: String(d.id),
      x: pos[d.id].x, y: pos[d.id].y,
      width: NODE_W, height: NODE_H,
      shape: 'rect',
      markup: [
        { tagName: 'rect', selector: 'body' },
        { tagName: 'rect', selector: 'typeBar' },
        { tagName: 'circle', selector: 'stDot' },
        { tagName: 'text', selector: 'icon' },
        { tagName: 'text', selector: 'name' },
        { tagName: 'text', selector: 'sub' },
        { tagName: 'text', selector: 'ip' },
      ],
      attrs: {
        body: { fill: 'rgba(18,36,63,.92)', stroke: d.status === 1 ? 'rgba(103,194,58,.5)' : stColor, strokeWidth: 1.5, rx: 8 },
        typeBar: { width: 5, height: NODE_H, fill: meta.color, rx: 2 },
        stDot: { r: 5, cx: NODE_W - 16, cy: 16, fill: stColor },
        icon: { x: 16, y: 30, fontSize: 18, text: meta.icon },
        name: { x: 42, y: 24, fontSize: 12.5, fontWeight: 700, fill: '#e0e6ed', text: d.name },
        sub: { x: 42, y: 42, fontSize: 10.5, fill: '#8899aa', text: `${meta.name} · ${d.vendor || '未知厂商'}` },
        ip: { x: 16, y: 62, fontSize: 10, fill: '#66788c', text: d.mgmt_ip || '无管理IP' },
      },
      data: { device: d },
    })
  })

  links.value.forEach(l => {
    const a = portDevice[l.port_a_id], b = portDevice[l.port_b_id]
    if (!a || !b) return
    graph.addEdge({
      id: 'l' + l.id,
      source: String(a), target: String(b),
      router: { name: 'manhattan', args: { padding: 24 } },
      connector: { name: 'rounded' },
      labels: l.bandwidth ? [{ position: 0.5, attrs: { text: { text: l.bandwidth, fontSize: 9, fill: '#8899aa' }, rect: { fill: 'rgba(10,22,40,.8)', stroke: 'none', rx: 3 } } }] : [],
      attrs: {
        line: {
          stroke: l.status === 1 ? 'rgba(103,194,58,.7)' : 'rgba(144,147,153,.5)',
          strokeWidth: l.status === 1 ? 2 : 1.4,
          strokeDasharray: l.status === 1 ? undefined : '5 4',
        },
      },
      zIndex: -1,
    })
  })

  graph.centerContent()
  graph.on('node:click', ({ node }) => { openDevice(node.data.device) })
  graph.on('edge:click', ({ edge }) => {
    // 点边 → 链路台账（V13 新页可编辑带宽/类型/状态）
    router.push({ path: '/network-links' })
  })
  graph.on('node:moved', scheduleSave)
}

let saveTimer = null
function scheduleSave() {
  clearTimeout(saveTimer)
  saveTimer = setTimeout(saveLayout, 1500)
}
async function saveLayout() {
  if (!graph) return
  saving.value = true
  try {
    const pos = {}
    graph.getNodes().forEach(nd => {
      const p = nd.getPosition()
      pos[nd.id] = { x: Math.round(p.x), y: Math.round(p.y) }
    })
    await api.put('/topo-diagram', { site_id: siteId.value, kind: 'net_topo', data: JSON.stringify(pos) })
  } catch (e) { console.warn('布局保存失败', e) }
  saving.value = false
}
function autoLayout() { buildGraph(null) }

async function reload() {
  const r = await api.get('/network-topology-rich', { site_id: siteId.value })
  devices.value = r.data?.devices || []
  ports.value = r.data?.ports || []
  links.value = r.data?.links || []
  let saved = null
  try {
    const tr = await api.get('/topo-diagram', { site_id: siteId.value, kind: 'net_topo' })
    if (tr.data?.data) saved = JSON.parse(tr.data.data)
  } catch (e) { /* 无布局 */ }
  await nextTick()
  buildGraph(saved)
}

onMounted(async () => {
  const r = await api.get('/sites', { size: 100 })
  sites.value = r.data?.items || []
  siteId.value = sites.value[0]?.id || 0
  if (siteId.value) reload()
})
onBeforeUnmount(() => { if (graph) graph.dispose() })
</script>

<style scoped>
.toolbar-inline { display: flex; gap: 8px; align-items: center; }
.net-wrap { position: relative; height: calc(100vh - 170px); min-height: 520px; overflow: hidden; }
.net-canvas { position: absolute; inset: 0; }
.net-legend { position: absolute; top: 10px; left: 12px; z-index: 5; display: flex; gap: 14px; flex-wrap: wrap;
  background: rgba(10,22,40,.85); border: 1px solid rgba(64,158,255,.2); border-radius: 8px; padding: 6px 14px; font-size: 12px; color: #8899aa; }
.net-legend i { display: inline-block; width: 10px; height: 10px; border-radius: 2px; margin-right: 4px; vertical-align: -1px; }
.net-legend .lg-status i { border-radius: 50%; }
.net-legend .s1 { background: #67c23a; } .net-legend .s0 { background: #909399; }
.net-legend .s2 { background: #e6a23c; } .net-legend .s3 { background: #f56c6c; }
.net-empty { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; z-index: 4; }
.kv-table { width: 100%; border-collapse: collapse; }
.kv-table td { padding: 9px 8px; border-bottom: 1px solid var(--border); font-size: 13px; }
.kv-table td:first-child { color: var(--muted); width: 90px; }
.biz-sum { display: flex; gap: 10px; margin-bottom: 14px; }
.bstat { flex: 1; background: var(--bg2); border: 1px solid var(--border); border-radius: 8px; padding: 8px 4px; text-align: center; }
.bstat b { font-size: 18px; display: block; }
.bstat span { font-size: 11px; color: var(--muted); }
.pdot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; }
.pdot.up { background: #67c23a; } .pdot.down { background: #909399; }
.warn-small { color: #e6a23c; font-size: 12.5px; }
.link { color: var(--accent); cursor: pointer; font-weight: 600; }
.link:hover { text-decoration: underline; }
.badge.st-1 { color: #67c23a; border-color: rgba(103,194,58,.4); }
.badge.st-2 { color: #e6a23c; border-color: rgba(230,162,60,.4); }
.badge.st-3 { color: #f56c6c; border-color: rgba(245,108,108,.4); }
.badge.st-0 { color: #909399; border-color: rgba(144,147,153,.4); }
.drawer-mask { position: fixed; inset: 0; background: rgba(0,0,0,.35); z-index: 1000; display: flex; justify-content: flex-end; }
.drawer { height: 100%; background: var(--bg); box-shadow: -4px 0 20px rgba(0,0,0,.15); display: flex; flex-direction: column; overflow: hidden; }
.drawer-head { display: flex; align-items: center; justify-content: space-between; padding: 16px 20px; border-bottom: 1px solid var(--border); flex-shrink: 0; }
.drawer-head h3 { margin: 0; font-size: 17px; }
.drawer-body { flex: 1; overflow-y: auto; padding: 20px; }
</style>
