<template>
  <div>
    <div class="page-head">
      <h1>供电单线图</h1>
      <div class="toolbar-inline">
        <select class="input" style="width:160px" v-model.number="siteId" @change="reload">
          <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
        </select>
        <button class="btn" @click="autoLayout"><SvgIcon name="compass" />自动布局</button>
        <button class="btn btn-primary" @click="saveLayout" :disabled="saving">{{ saving ? '保存中…' : '保存布局' }}</button>
        <router-link class="btn" to="/power-chain"><SvgIcon name="clipboard" />节点台账</router-link>
      </div>
    </div>
    <div class="sld-wrap card">
      <div class="sld-legend">
        <span v-for="(v,k) in KIND_META" :key="k"><i :style="{background:v.color}"></i>{{ v.name }}</span>
        <span class="lg-status"><i class="st1"></i>正常 <i class="st2"></i>告警 <i class="st3"></i>故障 <i class="st0"></i>停用</span>
      </div>
      <div ref="canvasRef" class="sld-canvas"></div>
      <div v-if="!nodes.length" class="empty sld-empty">该站点暂无供电节点，请到「供电链路」台账添加</div>
    </div>

    <!-- 节点详情抽屉 -->
    <div v-if="sel" class="drawer-mask" @click.self="sel=null">
      <div class="drawer" style="width:420px">
        <div class="drawer-head"><h3><SvgIcon name="bolt" />{{ sel.name }}</h3><button class="btn btn-mini" @click="sel=null"><SvgIcon name="x" /></button></div>
        <div class="drawer-body">
          <table class="kv-table">
            <tr><td>节点类型</td><td><span class="badge">{{ KIND_META[sel.kind]?.name || sel.kind }}</span></td></tr>
            <tr><td>型号</td><td>{{ sel.model || '-' }}</td></tr>
            <tr><td>额定容量</td><td>{{ sel.rated_kva }} kVA</td></tr>
            <tr><td>当前负载</td><td>{{ sel.load_kw }} kW
              <span v-if="sel.rated_kva" class="muted">（{{ loadPct(sel) }}%）</span></td></tr>
            <tr><td>负载率</td><td>
              <div class="loadbar"><div :style="{width: loadPct(sel)+'%'}" :class="loadPct(sel)>80?'lb-red':loadPct(sel)>60?'lb-yellow':'lb-green'"></div></div>
            </td></tr>
            <tr><td>状态</td><td><span class="badge" :class="'st-'+sel.status">{{ ST[sel.status] }}</span></td></tr>
            <tr><td>上级节点</td><td>{{ parentName(sel) || '总进线（根节点）' }}</td></tr>
            <tr><td>关联资产</td><td>{{ sel.asset_id ? '资产 #' + sel.asset_id : '-' }}</td></tr>
            <tr><td>备注</td><td>{{ sel.remark || '-' }}</td></tr>
          </table>
        </div>
        <div class="drawer-foot">
          <router-link class="btn btn-primary" :to="'/power-chain'">编辑节点信息</router-link>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount, nextTick } from 'vue'
import api from '../api'
import { Graph } from '@antv/x6'

const KIND_META = {
  utility:    { name: '市电',   color: '#409eff' },
  transformer:{ name: '变压器', color: '#9b7bff' },
  ups:        { name: 'UPS',    color: '#36cfc9' },
  ats:        { name: 'ATS',    color: '#e6a23c' },
  rack_pdu:   { name: '列头柜', color: '#67c23a' },
  pdu:        { name: 'PDU',    color: '#f5a3c7' },
  other:      { name: '其他',   color: '#909399' },
}
const ST = { 1: '正常', 2: '告警', 3: '故障', 0: '停用' }
const ST_COLOR = { 1: '#67c23a', 2: '#e6a23c', 3: '#f56c6c', 0: '#909399' }

const sites = ref([]), siteId = ref(0)
const nodes = ref([])
const sel = ref(null)
const saving = ref(false)
const canvasRef = ref(null)
let graph = null

function loadPct(n) {
  if (!n.rated_kva) return 0
  return Math.min(100, Math.round((n.load_kw / (n.rated_kva * 0.9)) * 100)) // kVA→kW 按 0.9 功率因数
}
function parentName(n) {
  const p = nodes.value.find(x => x.id === n.parent_id)
  return p ? p.name : ''
}

// 树形自动布局：按层级分列，同层水平展开
function computeLayout(list) {
  const byParent = {}
  list.forEach(n => { (byParent[n.parent_id] = byParent[n.parent_id] || []).push(n) })
  const pos = {}
  const X0 = 80, Y_GAP = 130, NODE_W = 170, NODE_H = 84
  function subtreeWidth(id) {
    const kids = byParent[id] || []
    if (!kids.length) return NODE_W + 40
    return kids.reduce((s, k) => s + subtreeWidth(k.id), 0)
  }
  function place(id, x, depth) {
    const kids = byParent[id] || []
    let cx = x
    const widths = kids.map(k => subtreeWidth(k.id))
    const total = widths.reduce((a, b) => a + b, 0)
    let startX = x - total / 2
    kids.forEach((k, i) => {
      const w = widths[i]
      pos[k.id] = { x: startX + w / 2 - NODE_W / 2, y: 60 + depth * Y_GAP }
      place(k.id, startX + w / 2, depth + 1)
      startX += w
    })
  }
  const roots = byParent[0] || []
  let rx = X0
  roots.forEach(r => {
    pos[r.id] = { x: rx, y: 60 }
    place(r.id, rx + NODE_W / 2, 1)
    rx += subtreeWidth(r.id) + 60
  })
  return pos
}

function buildGraph(savedPos) {
  if (graph) { graph.dispose(); graph = null }
  const list = nodes.value
  if (!list.length) return
  const pos = {}
  const auto = computeLayout(list)
  list.forEach(n => { pos[n.id] = (savedPos && savedPos[n.id]) || auto[n.id] || { x: 80, y: 80 } })

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

  // 节点卡片
  list.forEach(n => {
    const meta = KIND_META[n.kind] || KIND_META.other
    const pct = loadPct(n)
    const barColor = pct > 80 ? '#f56c6c' : pct > 60 ? '#e6a23c' : '#67c23a'
    const stColor = ST_COLOR[n.status] || '#909399'
    graph.addNode({
      id: String(n.id),
      x: pos[n.id].x, y: pos[n.id].y,
      width: 170, height: 84,
      shape: 'rect',
      markup: [
        { tagName: 'rect', selector: 'body' },
        { tagName: 'rect', selector: 'kindBar' },
        { tagName: 'circle', selector: 'stDot' },
        { tagName: 'text', selector: 'name' },
        { tagName: 'text', selector: 'kind' },
        { tagName: 'text', selector: 'load' },
        { tagName: 'rect', selector: 'barBg' },
        { tagName: 'rect', selector: 'bar' },
      ],
      attrs: {
        body: {
          fill: 'rgba(18,36,63,.92)', stroke: stColor, strokeWidth: 1.5, rx: 8,
        },
        kindBar: { width: 5, height: 84, fill: meta.color, rx: 2 },
        stDot: { r: 5, cx: 152, cy: 16, fill: stColor },
        name: { x: 14, y: 22, fontSize: 13, fontWeight: 700, fill: '#e0e6ed', text: n.name, textWrap: { width: 120, height: 20, ellipsis: true } },
        kind: { x: 14, y: 42, fontSize: 11, fill: '#8899aa', text: `${meta.name} · ${n.rated_kva} kVA` },
        load: { x: 14, y: 60, fontSize: 10, fill: '#66788c', text: `负载 ${n.load_kw} kW（${pct}%）` },
        barBg: { x: 14, y: 68, width: 142, height: 5, rx: 2, fill: 'rgba(140,170,210,.15)' },
        bar: { x: 14, y: 68, width: Math.max(2, 142 * pct / 100), height: 5, rx: 2, fill: barColor },
      },
      data: { node: n },
    })
  })

  // 边：父子连线（正交）
  list.filter(n => n.parent_id).forEach(n => {
    graph.addEdge({
      source: String(n.parent_id), target: String(n.id),
      router: { name: 'manhattan', args: { padding: 20 } },
      connector: { name: 'rounded' },
      attrs: {
        line: { stroke: 'rgba(64,158,255,.55)', strokeWidth: 1.6, targetMarker: { name: 'block', size: 6 } },
      },
      zIndex: -1,
    })
  })

  graph.centerContent()
  graph.on('node:click', ({ node }) => { sel.value = node.data.node })
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
    await api.put('/topo-diagram', { site_id: siteId.value, kind: 'power_sld', data: JSON.stringify(pos) })
  } catch (e) { console.warn('布局保存失败', e) }
  saving.value = false
}

function autoLayout() { buildGraph(null) }

async function reload() {
  const r = await api.get('/power-chains', { site_id: siteId.value })
  nodes.value = r.data || []
  let saved = null
  try {
    const tr = await api.get('/topo-diagram', { site_id: siteId.value, kind: 'power_sld' })
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
.sld-wrap { position: relative; height: calc(100vh - 170px); min-height: 520px; overflow: hidden; }
.sld-canvas { position: absolute; inset: 0; }
.sld-legend { position: absolute; top: 10px; left: 12px; z-index: 5; display: flex; gap: 14px; flex-wrap: wrap;
  background: rgba(10,22,40,.85); border: 1px solid rgba(64,158,255,.2); border-radius: 8px; padding: 6px 14px; font-size: 12px; color: #8899aa; }
.sld-legend i { display: inline-block; width: 10px; height: 10px; border-radius: 2px; margin-right: 4px; vertical-align: -1px; }
.sld-legend .lg-status i { border-radius: 50%; }
.sld-legend .st1 { background: #67c23a; } .sld-legend .st2 { background: #e6a23c; }
.sld-legend .st3 { background: #f56c6c; } .sld-legend .st0 { background: #909399; }
.sld-empty { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; z-index: 4; }
.kv-table { width: 100%; border-collapse: collapse; }
.kv-table td { padding: 9px 8px; border-bottom: 1px solid var(--border); font-size: 13px; }
.kv-table td:first-child { color: var(--muted); width: 90px; }
.loadbar { height: 8px; background: rgba(140,170,210,.15); border-radius: 4px; overflow: hidden; }
.loadbar div { height: 100%; border-radius: 4px; transition: width .3s; }
.lb-green { background: #67c23a; } .lb-yellow { background: #e6a23c; } .lb-red { background: #f56c6c; }
.badge.st-1 { color: #67c23a; border-color: rgba(103,194,58,.4); }
.badge.st-2 { color: #e6a23c; border-color: rgba(230,162,60,.4); }
.badge.st-3 { color: #f56c6c; border-color: rgba(245,108,108,.4); }
.badge.st-0 { color: #909399; border-color: rgba(144,147,153,.4); }
.drawer-mask { position: fixed; inset: 0; background: rgba(0,0,0,.35); z-index: 1000; display: flex; justify-content: flex-end; }
.drawer { height: 100%; background: var(--bg); box-shadow: -4px 0 20px rgba(0,0,0,.15); display: flex; flex-direction: column; overflow: hidden; }
.drawer-head { display: flex; align-items: center; justify-content: space-between; padding: 16px 20px; border-bottom: 1px solid var(--border); flex-shrink: 0; }
.drawer-head h3 { margin: 0; font-size: 17px; }
.drawer-body { flex: 1; overflow-y: auto; padding: 20px; }
.drawer-foot { padding: 14px 20px; border-top: 1px solid var(--border); display: flex; justify-content: flex-end; gap: 10px; flex-shrink: 0; }
</style>
