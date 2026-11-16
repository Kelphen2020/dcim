<template>
  <div>
    <div class="page-head"><h1>备件台账</h1>
      <div class="toolbar" style="margin:0;gap:8px">
        <button class="btn" :class="{primary: tab==='parts'}" @click="tab='parts'">台账</button>
        <button class="btn" :class="{primary: tab==='pr'}" @click="switchPR">采购申请<span v-if="prPending" class="pr-badge">{{ prPending }}</span></button>
        <button v-if="tab==='parts'" class="btn primary" @click="openEdit(null)">+ 新增备件</button>
      </div>
    </div>

    <template v-if="tab==='parts'">
    <div class="stat-row">
      <div class="stat-card"><b>{{ stats.total_kinds || 0 }}</b><span>备件种类</span></div>
      <div class="stat-card" :class="{warn: stats.low_stock>0}"><b>{{ stats.low_stock || 0 }}</b><span>低库存</span></div>
      <div class="stat-card"><b>¥{{ fmtMoney(stats.stock_value) }}</b><span>库存总值</span></div>
      <div class="stat-card top-out">
        <span class="to-h">近30天消耗 TOP</span>
        <div v-for="t in stats.top_out_30d||[]" :key="t.spare_id" class="to-r"><i>{{ t.name }}</i><b>×{{ t.qty }}</b></div>
        <div v-if="!(stats.top_out_30d||[]).length" class="to-r dim">暂无出库记录</div>
      </div>
    </div>

    <div class="toolbar">
      <input class="input" v-model="f.keyword" placeholder="搜索 编码/名称/型号" style="width:220px" @keyup.enter="load(1)" />
      <select class="input" style="width:130px" v-model="f.category" @change="load(1)">
        <option value="">全部分类</option>
        <option v-for="c in CATS" :key="c" :value="c">{{ c }}</option>
      </select>
      <label class="ck"><input type="checkbox" v-model="f.lowOnly" @change="load(1)" /> 只看低库存</label>
      <button class="btn" @click="load(1)">查询</button>
    </div>

    <table class="data-table">
      <thead><tr><th>编码</th><th>名称</th><th>分类</th><th>规格型号</th><th>库存</th><th>安全水位</th><th>存放位置</th><th>单价</th><th style="width:230px">操作</th></tr></thead>
      <tbody>
        <tr v-for="sp in list" :key="sp.id" :class="{lowrow: sp.safety_stock>0 && sp.stock<=sp.safety_stock}">
          <td><code>{{ sp.code }}</code></td>
          <td><b>{{ sp.name }}</b></td>
          <td>{{ sp.category || '—' }}</td>
          <td class="dim">{{ sp.spec || '—' }}</td>
          <td>
            <b :class="sp.safety_stock>0 && sp.stock<=sp.safety_stock ? 'danger' : ''">{{ sp.stock }}</b> {{ sp.unit }}
            <span v-if="sp.safety_stock>0 && sp.stock<=sp.safety_stock" class="tag danger">低</span>
          </td>
          <td class="dim">{{ sp.safety_stock || '—' }}</td>
          <td class="dim">{{ sp.location || '—' }}</td>
          <td>{{ sp.price ? '¥'+sp.price.toFixed(2) : '—' }}</td>
          <td>
            <button class="btn sm" @click="openMove(sp,'in')">入库</button>
            <button v-if="sp.safety_stock>0 && sp.stock<=sp.safety_stock" class="btn sm warn" @click="quickPR(sp)">采购</button>
            <button class="btn sm" @click="openMove(sp,'out')">出库</button>
            <button class="btn sm" @click="openMoves(sp)">流水</button>
            <button class="btn sm" @click="openBom(sp)">BOM</button>
            <button class="btn sm" @click="openEdit(sp)">编辑</button>
          </td>
        </tr>
        <tr v-if="!list.length"><td colspan="9" class="empty">暂无备件，点右上「新增备件」建台账</td></tr>
      </tbody>
    </table>
    <div class="pager" v-if="total > f.size">
      <button class="btn sm" :disabled="f.page<=1" @click="load(f.page-1)">‹</button>
      <span>{{ f.page }} / {{ Math.ceil(total/f.size) }}</span>
      <button class="btn sm" :disabled="f.page>=Math.ceil(total/f.size)" @click="load(f.page+1)">›</button>
    </div>
    </template>

    <!-- V30 采购申请流 -->
    <template v-if="tab==='pr'">
      <div class="toolbar">
        <select class="input" style="width:130px" v-model="prf.status" @change="loadPR">
          <option value="">全部状态</option>
          <option value="1">待审批</option><option value="2">已批准</option>
          <option value="3">已驳回</option><option value="4">已入库</option>
        </select>
        <span class="dim" style="font-size:12.5px">流程: 低库存发起申请 → 管理员审批 → 执行入库(自动写流水)</span>
      </div>
      <table class="data-table">
        <thead><tr><th>编号</th><th>备件</th><th>数量</th><th>供应商/预算</th><th>当前库存/水位</th><th>事由</th><th>申请人</th><th>状态</th><th>审批</th><th style="width:150px">操作</th></tr></thead>
        <tbody>
          <tr v-for="pr in prList" :key="pr.id">
            <td><code>{{ pr.code }}</code></td>
            <td><b>{{ pr.spare_name }}</b><div class="dim code">{{ pr.spare_code }}</div></td>
            <td><b>{{ pr.qty }}</b> {{ pr.spare_unit }}</td>
            <td class="dim" style="font-size:12px">{{ pr.supplier || '—' }}<div v-if="pr.budget_yuan">¥{{ pr.budget_yuan.toFixed(2) }}</div></td>
            <td :class="{danger: pr.stock_now<=pr.safety_stock}">{{ pr.stock_now }} / {{ pr.safety_stock }}</td>
            <td class="dim" style="max-width:200px">{{ pr.reason }}</td>
            <td>{{ pr.applicant }}</td>
            <td><span class="tag" :class="'ps'+pr.status">{{ PRS[pr.status] }}</span></td>
            <td class="dim">{{ pr.approver ? pr.approver + (pr.approve_note? '·'+pr.approve_note:'') : '—' }}</td>
            <td>
              <template v-if="pr.status===1 && isAdmin">
                <button class="btn sm" @click="reviewPR(pr,true)">批准</button>
                <button class="btn sm danger" @click="reviewPR(pr,false)">驳回</button>
              </template>
              <button v-else-if="pr.status===2 && isAdmin" class="btn sm primary" @click="stockPR(pr)">执行入库</button>
              <span v-else class="dim">—</span>
            </td>
          </tr>
          <tr v-if="!prList.length"><td colspan="10" class="empty">暂无采购申请</td></tr>
        </tbody>
      </table>
    </template>

    <!-- 新增/编辑抽屉 -->
    <div v-if="edit.show" class="drawer-mask" @click.self="edit.show=false">
      <div class="drawer" style="width:480px">
        <div class="drawer-head"><h3>{{ edit.id ? '编辑备件' : '新增备件' }}</h3><button class="dp-x" @click="edit.show=false"><SvgIcon name="x" /></button></div>
        <div class="drawer-body">
          <div class="form-row"><label>名称 *</label><input class="input" v-model="edit.name" placeholder="如：C13电源线" /></div>
          <div class="form-row"><label>编码</label><input class="input" v-model="edit.code" :disabled="!!edit.id" placeholder="留空自动生成" /></div>
          <div class="form-row"><label>分类</label>
            <select class="input" v-model="edit.category">
              <option v-for="c in CATS" :key="c" :value="c">{{ c }}</option>
            </select>
          </div>
          <div class="form-row"><label>规格型号</label><input class="input" v-model="edit.spec" /></div>
          <div class="form-row"><label>单位</label><input class="input" v-model="edit.unit" style="width:90px" /> </div>
          <div class="form-row" v-if="!edit.id"><label>初始库存</label><input class="input" type="number" v-model.number="edit.stock" min="0" style="width:110px" /></div>
          <div class="form-row"><label>安全水位</label><input class="input" type="number" v-model.number="edit.safety_stock" min="0" style="width:110px" />
            <span class="dim" style="font-size:12px;margin-left:8px">库存≤水位自动预警</span></div>
          <div class="form-row"><label>存放位置</label><input class="input" v-model="edit.location" placeholder="如：备件库A-3架" /></div>
          <div class="form-row"><label>单价(元)</label><input class="input" type="number" v-model.number="edit.price" min="0" step="0.01" style="width:120px" /></div>
          <div class="form-row"><label>供应商</label><input class="input" v-model="edit.supplier" /></div>
          <div class="form-row"><label>备注</label><textarea class="input" v-model="edit.remark" rows="2"></textarea></div>
          <button class="btn primary block" @click="saveEdit" :disabled="edit.saving">{{ edit.saving ? '保存中…' : '保存' }}</button>
          <button v-if="edit.id && !edit.hasMv" class="btn danger block" style="margin-top:8px" @click="delSpare">删除备件</button>
        </div>
      </div>
    </div>

    <!-- 出入库抽屉 -->
    <div v-if="mv.show" class="drawer-mask" @click.self="mv.show=false">
      <div class="drawer" style="width:440px">
        <div class="drawer-head"><h3>{{ mv.type==='in' ? '入库' : '出库' }} · {{ mv.sp?.name }}</h3><button class="dp-x" @click="mv.show=false"><SvgIcon name="x" /></button></div>
        <div class="drawer-body">
          <div class="mv-info">当前库存 <b>{{ mv.sp?.stock }}</b> {{ mv.sp?.unit }}<span v-if="mv.sp?.safety_stock"> · 安全水位 {{ mv.sp.safety_stock }}</span></div>
          <div class="form-row"><label>数量 *</label><input class="input" type="number" v-model.number="mv.qty" min="1" style="width:120px" /> {{ mv.sp?.unit }}</div>
          <div class="form-row"><label>事由</label>
            <select class="input" v-model="mv.reason">
              <option v-for="r in (mv.type==='in'?IN_REASONS:OUT_REASONS)" :key="r" :value="r">{{ r }}</option>
            </select>
          </div>
          <div class="form-row" v-if="mv.type==='out'"><label>关联工单</label>
            <select class="input" v-model.number="mv.work_order_id">
              <option :value="0">不关联</option>
              <option v-for="wo in openWos" :key="wo.id" :value="wo.id">{{ wo.code }} · {{ wo.title }}</option>
            </select>
            <div class="dim" style="font-size:12px;margin-top:4px">关联后自动写入工单流转日志</div>
          </div>
          <button class="btn primary block" @click="doMove" :disabled="mv.saving || !mv.qty">{{ mv.saving ? '提交中…' : '确认' + (mv.type==='in'?'入库':'出库') }}</button>
        </div>
      </div>
    </div>

    <!-- 采购申请抽屉 -->
    <div v-if="prBox.show" class="drawer-mask" @click.self="prBox.show=false">
      <div class="drawer" style="width:430px">
        <div class="drawer-head"><h3><SvgIcon name="pen" />发起采购申请 · {{ prBox.sp?.name }}</h3><button class="dp-x" @click="prBox.show=false"><SvgIcon name="x" /></button></div>
        <div class="drawer-body">
          <div class="mv-info">当前库存 <b :class="{danger: prBox.sp?.stock<=prBox.sp?.safety_stock}">{{ prBox.sp?.stock }}</b> {{ prBox.sp?.unit }} · 安全水位 {{ prBox.sp?.safety_stock }}</div>
          <div class="form-row"><label>采购数量 *</label><input class="input" type="number" v-model.number="prBox.qty" min="1" style="width:120px" /> {{ prBox.sp?.unit }}</div>
          <div class="form-row"><label>供应商</label><input class="input" v-model="prBox.supplier" placeholder="留空默认台账供应商" /></div>
          <div class="form-row"><label>预算金额(元)</label><input class="input" type="number" min="0" step="0.01" v-model.number="prBox.budget_yuan" style="width:130px" />
            <span class="dim" style="font-size:12px;margin-left:8px">留空按 数量×单价 自动估算</span></div>
          <div class="form-row"><label>事由</label><textarea class="input" rows="2" v-model="prBox.reason"></textarea></div>
          <button class="btn primary block" :disabled="prBox.saving||!prBox.qty" @click="submitPR">{{ prBox.saving?'提交中…':'提交申请' }}</button>
        </div>
      </div>
    </div>

    <!-- V31 BOM 配置抽屉 -->
    <div v-if="bom.show" class="drawer-mask" @click.self="bom.show=false">
      <div class="drawer" style="width:520px">
        <div class="drawer-head"><h3><SvgIcon name="puzzle" />BOM 关联 · {{ bom.sp?.name }}</h3><button class="dp-x" @click="bom.show=false"><SvgIcon name="x" /></button></div>
        <div class="drawer-body">
          <div class="dim" style="font-size:12.5px;margin-bottom:10px">该备件适用于哪些设备？绑定后，对应资产详情页会自动推荐此备件。</div>
          <div v-for="b in bom.list" :key="b.id" class="sp-line">
            <span style="flex:1"><b>{{ ACAT[b.asset_cat] || b.asset_cat }}</b><span v-if="b.asset_model" class="dim"> / {{ b.asset_model }}</span><span v-else class="dim">(通用)</span></span>
            <span class="dim">建议×{{ b.qty }}</span>
            <button class="btn sm danger" @click="delBom(b)"><SvgIcon name="x" /></button>
          </div>
          <div v-if="!bom.list.length" class="dim" style="padding:8px 0">暂无绑定</div>
          <div class="bom-add">
            <select class="input" v-model="bom.cat" style="width:110px">
              <option v-for="(n,k) in ACAT" :key="k" :value="k">{{ n }}</option>
            </select>
            <input class="input" v-model="bom.model" placeholder="型号(留空=通用)" style="width:140px" />
            <input class="input" type="number" min="1" v-model.number="bom.qty" style="width:60px" title="单台建议数" />
            <button class="btn sm primary" @click="addBom">添加</button>
          </div>
        </div>
      </div>
    </div>

    <!-- 流水抽屉 -->
    <div v-if="moves.show" class="drawer-mask" @click.self="moves.show=false">
      <div class="drawer" style="width:560px">
        <div class="drawer-head"><h3><SvgIcon name="log" />出入库流水 · {{ moves.sp?.name }}</h3><button class="dp-x" @click="moves.show=false"><SvgIcon name="x" /></button></div>
        <div class="drawer-body">
          <table class="data-table">
            <thead><tr><th>时间</th><th>类型</th><th>数量</th><th>结余</th><th>事由</th><th>操作人</th><th>工单</th></tr></thead>
            <tbody>
              <tr v-for="m in moves.list" :key="m.id">
                <td>{{ fmtT(m.created_at) }}</td>
                <td><span class="tag" :class="m.type==='in'?'ok':'warn'">{{ m.type==='in'?'入库':'出库' }}</span></td>
                <td><b>{{ m.type==='in'?'+':'-' }}{{ m.qty }}</b></td>
                <td>{{ m.stock_after }}</td>
                <td class="dim">{{ m.reason || '—' }}</td>
                <td>{{ m.operator || '—' }}</td>
                <td><a v-if="m.work_order_id" @click="gotoWo(m.work_order_id)">查看</a><span v-else class="dim">—</span></td>
              </tr>
              <tr v-if="!moves.list.length"><td colspan="7" class="empty">暂无流水</td></tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import api from '../api'
import { showConfirm, showPrompt, toastAuto } from '../ui-dialog'

const router = useRouter()
const route = useRoute()
// G批: 大屏低库存穿透深链 ?low_only=1&keyword=CODE（双路径: onMounted+watch，防同组件复用时不重挂载）
function applyQuery(q) {
  // V76 全局搜索深链：/spare-parts?id=N&detail=1 → 打开备件详情抽屉
  if (q && q.id && q.detail === '1') { openPartById(Number(q.id)); return }
  if (q && (q.low_only || q.keyword)) {
    f.lowOnly = q.low_only === '1' || q.low_only === 1
    if (q.keyword) f.keyword = String(q.keyword)
    load(1)
  }
}
async function openPartById(id) {
  const r = await api.get('/spare-parts', { id, size: 1 })
  const sp = (r.data?.items || [])[0]
  if (sp) openEdit(sp)
}
watch(() => route.query, (q) => { if (route.path === '/spare-parts') applyQuery(q) })
const CATS = ['电气', '暖通', 'IT设备', '安防', '线缆', '耗材', '其他']
const IN_REASONS = ['采购入库', '退库归还', '盘盈', '调拨入库']
const OUT_REASONS = ['维修领用', '安装领用', '盘亏', '调拨出库', '报废出库']

const list = ref([]); const total = ref(0); const stats = ref({})
// V30 采购申请
const tab = ref('parts')
const PRS = { 1: '待审批', 2: '已批准', 3: '已驳回', 4: '已入库' }
const prList = ref([]); const prf = reactive({ status: '' })
const prBox = reactive({ show: false, sp: null, qty: 10, reason: '', supplier: '', budget_yuan: null, saving: false })
// V31 BOM
const ACAT = { server: '服务器', network: '网络设备', storage: '存储', power: '动力设备', hvac: '空调制冷', security: '安防', other: '其他' }
const bom = reactive({ show: false, sp: null, list: [], cat: 'server', model: '', qty: 1 })
const prPending = ref(0)
const isAdmin = ['global_admin','site_admin'].includes(JSON.parse(localStorage.getItem('dcim_user')||'{}').role||'')
const f = reactive({ page: 1, size: 20, keyword: '', category: '', lowOnly: false })
const edit = reactive({ show: false, id: 0, saving: false, hasMv: false, name: '', code: '', category: '电气', spec: '', unit: '个', stock: 0, safety_stock: 0, location: '', price: 0, supplier: '', remark: '' })
const mv = reactive({ show: false, sp: null, type: 'in', qty: 1, reason: '采购入库', work_order_id: 0, saving: false })
const moves = reactive({ show: false, sp: null, list: [] })
const openWos = ref([])

async function load(page) {
  f.page = page || f.page
  const r = await api.get('/spare-parts', { page: f.page, size: f.size, keyword: f.keyword, category: f.category, low_only: f.lowOnly ? 1 : '' })
  if (r.code === 0) { list.value = r.data.items || r.data.list || []; total.value = r.data.total || 0 }
  const s = await api.get('/spare-parts/stats')
  if (s.code === 0) stats.value = s.data
}
function openEdit(sp) {
  Object.assign(edit, sp ? { ...sp, show: true, id: sp.id, saving: false, hasMv: false } :
    { show: true, id: 0, saving: false, hasMv: false, name: '', code: '', category: '电气', spec: '', unit: '个', stock: 0, safety_stock: 0, location: '', price: 0, supplier: '', remark: '' })
}
async function saveEdit() {
  edit.saving = true
  const body = { name: edit.name, code: edit.code, category: edit.category, spec: edit.spec, unit: edit.unit,
    safety_stock: edit.safety_stock || 0, location: edit.location, price: edit.price || 0, supplier: edit.supplier, remark: edit.remark }
  let r
  if (edit.id) r = await api.put('/spare-parts/' + edit.id, body)
  else r = await api.post('/spare-parts', { ...body, stock: edit.stock || 0, site_id: 1 })
  edit.saving = false
  if (r.code === 0 || r.code === 201) { edit.show = false; load() } else toastAuto(r.msg || '保存失败')
}
async function delSpare() {
  if (!(await showConfirm({ message: '确认删除该备件？' }))) return
  const r = await api.del('/spare-parts/' + edit.id)
  if (r.code === 0) { edit.show = false; load() } else toastAuto(r.msg || '删除失败')
}
async function openMove(sp, type) {
  mv.sp = sp; mv.type = type; mv.qty = 1; mv.saving = false; mv.work_order_id = 0
  mv.reason = type === 'in' ? '采购入库' : '维修领用'
  mv.show = true
  if (type === 'out') {
    const r = await api.get('/work-orders', { status: 2, size: 50 })
    if (r.code === 0) openWos.value = r.data.list || []
  }
}
async function doMove() {
  mv.saving = true
  const r = await api.post(`/spare-parts/${mv.sp.id}/move`, { type: mv.type, qty: mv.qty, reason: mv.reason, work_order_id: mv.work_order_id || 0 })
  mv.saving = false
  if (r.code === 0) { mv.show = false; load() } else toastAuto(r.msg || '操作失败')
}
async function openMoves(sp) {
  moves.sp = sp; moves.show = true
  const r = await api.get(`/spare-parts/${sp.id}/movements`)
  moves.list = r.code === 0 ? (r.data.list || []) : []
}
function gotoWo(id) { router.push({ path: '/work-orders', query: { id } }) }
// ── V30 采购申请流 ──
function switchPR() { tab.value = 'pr'; loadPR() }
async function loadPR() {
  const r = await api.get('/purchase-requests', { status: prf.status })
  if (r.code === 0) {
    prList.value = r.data.list || []
    prPending.value = prList.value.filter(p => p.status === 1).length
  }
}
function quickPR(sp) {
  prBox.sp = sp; prBox.qty = Math.max(sp.safety_stock * 2 - sp.stock, sp.safety_stock || 5)
  prBox.reason = `库存 ${sp.stock}${sp.unit} 低于安全水位 ${sp.safety_stock}，申请补货`
  prBox.supplier = sp.supplier || ''; prBox.budget_yuan = sp.price ? +(prBox.qty * sp.price).toFixed(2) : null
  prBox.show = true; prBox.saving = false
}
// ── V31 BOM ──
async function openBom(sp) {
  bom.sp = sp; bom.show = true; bom.model = ''; bom.qty = 1
  const r = await api.get('/spare-boms', { spare_id: sp.id })
  bom.list = r.code === 0 ? (r.data.list || []) : []
}
async function addBom() {
  const r = await api.post('/spare-boms', { spare_id: bom.sp.id, asset_cat: bom.cat, asset_model: bom.model.trim(), qty: bom.qty || 1 })
  if (r.code === 0 || r.code === 201) openBom(bom.sp); else toastAuto(r.msg || '添加失败(组合重复?)')
}
async function delBom(b) {
  const r = await api.del('/spare-boms/' + b.id)
  if (r.code === 0) bom.list = bom.list.filter(x => x.id !== b.id); else toastAuto(r.msg || '删除失败')
}
async function submitPR() {
  prBox.saving = true
  const r = await api.post('/purchase-requests', { spare_id: prBox.sp.id, qty: prBox.qty, reason: prBox.reason, supplier: prBox.supplier, budget_yuan: prBox.budget_yuan || 0 })
  prBox.saving = false
  if (r.code === 0 || r.code === 201) { prBox.show = false; loadPR(); load() } else toastAuto(r.msg || '提交失败')
}
async function reviewPR(pr, approve) {
  const note = approve ? '' : ((await showPrompt({ title: '驳回采购申请', label: '驳回理由（可选）' })) ?? '')
  const r = await api.post(`/purchase-requests/${pr.id}/review`, { approve, note })
  if (r.code === 0) loadPR(); else toastAuto(r.msg || '操作失败')
}
async function stockPR(pr) {
  if (!(await showConfirm({ message: `确认「${pr.spare_name} ×${pr.qty}」已到货并执行入库？` }))) return
  const r = await api.post(`/purchase-requests/${pr.id}/stock`, {})
  if (r.code === 0) { loadPR(); load() } else toastAuto(r.msg || '入库失败')
}
const fmtT = t => (t || '').slice(0, 16).replace('T', ' ')
const fmtMoney = v => (v || 0) >= 10000 ? ((v / 10000).toFixed(2) + '万') : (v || 0).toFixed(2)
onMounted(() => { applyQuery(route.query); load(1); loadPR() })
</script>

<style scoped>
.stat-row { display: grid; grid-template-columns: repeat(3, 150px) 1fr; gap: 12px; margin-bottom: 14px }
.stat-card { background: var(--card, rgba(127,150,180,.07)); border-radius: 10px; padding: 12px 16px; text-align: center }
.stat-card b { display: block; font-size: 22px; color: var(--accent, #409eff) }
.stat-card.warn b { color: #ff5252 }
.stat-card span { font-size: 12px; opacity: .7 }
.stat-card.top-out { text-align: left; padding: 8px 14px }
.to-h { font-size: 11px; opacity: .6 }
.to-r { display: flex; justify-content: space-between; font-size: 12.5px; padding: 1px 0 }
.to-r.dim { opacity: .5 }
.lowrow { background: rgba(255, 82, 82, .06) }
.danger { color: #ff5252 }
.mv-info { background: rgba(64,158,255,.1); border-radius: 8px; padding: 10px 14px; margin-bottom: 14px; font-size: 13px }
.form-row { margin-bottom: 12px }
.form-row label { display: block; font-size: 12.5px; opacity: .75; margin-bottom: 4px }
.block { width: 100%; padding: 10px }
.toolbar { display: flex; gap: 10px; align-items: center; margin-bottom: 12px }
.ck { font-size: 13px; display: flex; align-items: center; gap: 4px }
.pager { display: flex; gap: 10px; align-items: center; justify-content: center; margin-top: 12px }
.pr-badge { background: #ff5252; color: #fff; border-radius: 8px; font-size: 10.5px; padding: 1px 6px; margin-left: 5px }
.tag.ps1 { background: rgba(255,167,38,.14); color: #ffa726 }
.tag.ps2 { background: rgba(64,196,255,.14); color: #40c4ff }
.tag.ps3 { background: rgba(255,82,82,.14); color: #ff8a80 }
.tag.ps4 { background: rgba(0,230,118,.12); color: #00e676 }
.btn.warn { color: #ffa726; border-color: rgba(255,167,38,.45) }
.btn.primary { background: var(--accent, #409eff); color: #fff }
.code { font-size: 11px; font-family: monospace }
.bom-add { display: flex; gap: 8px; align-items: center; margin-top: 12px; padding-top: 12px; border-top: 1px dashed var(--border, rgba(127,150,180,.25)) }
.sp-line { display: flex; gap: 8px; align-items: center; padding: 6px 0; border-bottom: 1px solid rgba(127,150,180,.08); font-size: 13px }
.tag { font-size: 11px; padding: 1px 7px; border-radius: 8px }
.tag.danger { background: rgba(255,82,82,.15); color: #ff5252 }
.tag.ok { background: rgba(0,230,118,.12); color: #00e676 }
.tag.warn { background: rgba(255,167,38,.14); color: #ffa726 }
.empty { text-align: center; opacity: .5; padding: 30px !important }
a { color: var(--accent, #409eff); cursor: pointer }
</style>
