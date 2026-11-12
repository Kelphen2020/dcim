<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h2><SvgIcon name="box" /> 资产退役与处置</h2>
        <p class="muted">退役申请 → 管理员审批（批准自动下架并报废）→ 处置登记（方式/残值/凭证/数据擦除确认），全程留痕——资产处置记录是机房检查的硬性材料。盘点盘亏可一键转退役。</p>
      </div>
    </div>

    <!-- KPI -->
    <div class="ar-kpis">
      <div class="ar-kpi" :class="{ warn: summary.pending > 0 }" @click="quickFilter('status', '1')"><span class="k-v">{{ summary.pending || 0 }}</span><span class="k-l">待审批</span></div>
      <div class="ar-kpi" :class="{ warn: summary.pending_disposal > 0 }" @click="quickFilter('status', '2')"><span class="k-v">{{ summary.pending_disposal || 0 }}</span><span class="k-l">已批准待处置</span></div>
      <div class="ar-kpi" @click="quickFilter('status', '4')"><span class="k-v">{{ summary.month_done || 0 }}</span><span class="k-l">本月处置完成</span></div>
      <div class="ar-kpi"><span class="k-v">{{ fmtAmount(summary.month_amount) }}</span><span class="k-l">本月残值回收（元）</span></div>
    </div>

    <!-- 工具栏 -->
    <div class="toolbar">
      <select class="input" v-model="filters.status" style="width:150px" @change="load(1)">
        <option value="">全部状态</option>
        <option v-for="(lb, k) in STATUS" :key="k" :value="k">{{ lb }}</option>
      </select>
      <select class="input" v-model="filters.reason" style="width:150px" @change="load(1)">
        <option value="">全部原因</option>
        <option v-for="(lb, k) in REASONS" :key="k" :value="k">{{ lb }}</option>
      </select>
      <input class="input" v-model="filters.keyword" placeholder="搜索单号/资产/凭证号…" style="width:220px" @keyup.enter="load(1)" />
      <span style="flex:1"></span>
      <button class="btn btn-primary" v-if="canOps" @click="openCreate()"><SvgIcon name="check-square" /> 发起退役申请</button>
    </div>

    <!-- 列表 -->
    <div class="tbl-wrap card">
      <table class="tbl data-table">
        <thead><tr>
          <th>申请单号</th><th>资产</th><th>退役原因</th><th>状态</th><th>申请人</th>
          <th>申请时间</th><th>处置方式</th><th>残值(元)</th><th style="width:230px">操作</th>
        </tr></thead>
        <tbody>
          <tr v-for="r in list" :key="r.id">
            <td class="mono small"><a class="link" @click="openDetail(r.id)">{{ r.code }}</a></td>
            <td><span class="mono small">{{ r.asset_code }}</span> {{ r.asset_name }}</td>
            <td><span class="badge" :class="'rsn-' + r.reason">{{ REASONS[r.reason] || r.reason }}</span></td>
            <td><span class="badge" :class="'st-' + r.status">{{ STATUS[r.status] }}</span></td>
            <td>{{ r.applicant_name || '—' }}</td>
            <td class="small">{{ fmtTime(r.applied_at) }}</td>
            <td>{{ r.status === 4 ? (METHODS[r.disposal_method] || r.disposal_method) : '—' }}</td>
            <td class="mono">{{ r.status === 4 ? fmtAmount(r.disposal_amount) : '—' }}</td>
            <td>
              <button class="btn btn-mini" @click="openDetail(r.id)">详情</button>
              <button class="btn btn-mini btn-primary" v-if="r.status === 1 && auth.isAdmin" @click="openApprove(r)">审批</button>
              <button class="btn btn-mini" v-if="r.status === 2 && canOps" @click="openDisposal(r)">处置登记</button>
              <button class="btn btn-mini" v-if="r.status === 2 && canOps" @click="openRepl(r)">替换采购</button>
              <button class="btn btn-mini btn-mini-danger" v-if="r.status === 1 && canCancel(r)" @click="cancel(r)">撤销</button>
            </td>
          </tr>
          <tr v-if="!list.length"><td colspan="9" class="empty-cell">暂无退役申请</td></tr>
        </tbody>
      </table>
      <div class="pager" v-if="total > size">
        <button class="btn btn-mini" :disabled="page <= 1" @click="load(page - 1)">上一页</button>
        <span class="muted small">第 {{ page }} / {{ Math.ceil(total / size) }} 页 · 共 {{ total }} 条</span>
        <button class="btn btn-mini" :disabled="page * size >= total" @click="load(page + 1)">下一页</button>
      </div>
    </div>

    <!-- 发起申请抽屉 -->
    <div v-if="create.show" class="drawer-mask" @click.self="create.show = false">
      <div class="drawer" style="width:620px">
        <div class="drawer-head">
          <h3><SvgIcon name="box" /> 发起退役申请</h3>
          <button class="btn btn-mini" @click="create.show = false"><SvgIcon name="x" /></button>
        </div>
        <div class="drawer-body">
          <fieldset class="fs"><legend>退役资产</legend>
            <div v-if="create.asset.id" class="ar-picked">
              <SvgIcon name="server" />
              <div class="ar-picked-info">
                <b>{{ create.asset.name }}</b> <span class="mono small">{{ create.asset.code }}</span>
                <div class="muted small">{{ catLabel(create.asset.category) }} · {{ ['','在架','在库','维修中','已报废'][create.asset.status] || '—' }}<template v-if="create.asset.brand"> · {{ create.asset.brand }} {{ create.asset.model }}</template></div>
              </div>
              <button class="btn btn-mini" @click="create.asset = {}">重选</button>
            </div>
            <template v-else>
              <input class="input" v-model="create.assetKw" placeholder="输入资产编码/名称/SN 搜索…" @input="searchAssets" />
              <div class="ar-search" v-if="create.results.length">
                <div v-for="a in create.results" :key="a.id" class="ar-search-item" @click="pickAsset(a)">
                  <span class="mono small">{{ a.code }}</span> <b>{{ a.name }}</b>
                  <span class="muted small">{{ ['','在架','在库','维修中','已报废'][a.status] }}</span>
                </div>
              </div>
              <div v-else-if="create.searched" class="muted small" style="margin-top:8px">无匹配资产</div>
            </template>
          </fieldset>
          <fieldset class="fs"><legend>退役原因</legend>
            <div class="ar-reason-row">
              <label v-for="(lb, k) in REASONS" :key="k" class="ar-radio" :class="{ on: create.form.reason === k }">
                <input type="radio" :value="k" v-model="create.form.reason" /> {{ lb }}
              </label>
            </div>
            <p v-if="create.form.source_type === 'stocktake'" class="ar-src muted small">来源：资产盘点盘亏转办——账实不符资产退役核销</p>
          </fieldset>
          <fieldset class="fs"><legend>退役说明 *</legend>
            <textarea class="input" rows="4" v-model="create.form.description" placeholder="说明退役背景与依据（如：服役已满 8 年、多次故障维修记录、盘点确认遗失等），处置留痕是检查材料"></textarea>
          </fieldset>
        </div>
        <div class="drawer-foot">
          <button class="btn btn-primary" :disabled="create.saving" @click="submitCreate">{{ create.saving ? '提交中…' : '提交申请' }}</button>
          <button class="btn" @click="create.show = false">取消</button>
        </div>
      </div>
    </div>

    <!-- 审批抽屉 -->
    <div v-if="approve.show" class="drawer-mask" @click.self="approve.show = false">
      <div class="drawer" style="width:560px">
        <div class="drawer-head">
          <h3><SvgIcon name="check-square" /> 退役审批 · {{ approve.row.code }}</h3>
          <button class="btn btn-mini" @click="approve.show = false"><SvgIcon name="x" /></button>
        </div>
        <div class="drawer-body">
          <div class="ar-asset-card">
            <b>{{ approve.row.asset_name }}</b> <span class="mono small">{{ approve.row.asset_code }}</span>
            <div class="muted small">原因：{{ REASONS[approve.row.reason] }} · 申请人：{{ approve.row.applicant_name }} · {{ fmtTime(approve.row.applied_at) }}</div>
          </div>
          <div class="ar-desc">{{ approve.row.description }}</div>
          <p class="muted small ar-hint">批准后资产将自动下架（如在架）并置为报废终态，U 位与功率容量即刻释放；报废资产不可再上架。</p>
          <p v-if="approve.row.netdev_names" class="ar-warn">关联网络设备：{{ approve.row.netdev_names }} —— 批准退役后将自动置为离线（留痕+通知），如需继续使用请先在「网络设备」页解绑。</p>
          <div class="form-grid">
            <label class="full">审批决定 *
              <div class="ar-reason-row">
                <label class="ar-radio" :class="{ on: approve.form.approve }"><input type="radio" :value="true" v-model="approve.form.approve" /> 批准退役</label>
                <label class="ar-radio" :class="{ on: !approve.form.approve }"><input type="radio" :value="false" v-model="approve.form.approve" /> 驳回</label>
              </div>
            </label>
            <label class="full">审批意见<textarea class="input" rows="3" v-model="approve.form.note" placeholder="批准/驳回理由（记入留痕）"></textarea></label>
          </div>
        </div>
        <div class="drawer-foot">
          <button class="btn btn-primary" :disabled="approve.saving" @click="submitApprove">{{ approve.saving ? '提交中…' : '提交审批' }}</button>
          <button class="btn" @click="approve.show = false">取消</button>
        </div>
      </div>
    </div>

    <!-- 处置登记抽屉 -->
    <div v-if="disposal.show" class="drawer-mask" @click.self="disposal.show = false">
      <div class="drawer" style="width:600px">
        <div class="drawer-head">
          <h3><SvgIcon name="log" /> 处置登记 · {{ disposal.row.code }}</h3>
          <button class="btn btn-mini" @click="disposal.show = false"><SvgIcon name="x" /></button>
        </div>
        <div class="drawer-body">
          <div class="ar-asset-card">
            <b>{{ disposal.row.asset_name }}</b> <span class="mono small">{{ disposal.row.asset_code }}</span>
            <div class="muted small">原因：{{ REASONS[disposal.row.reason] }} · 批准人：{{ disposal.row.approver_name || '—' }}</div>
          </div>
          <div class="form-grid">
            <label>处置方式 *
              <select class="input" v-model="disposal.form.method">
                <option v-for="(lb, k) in METHODS" :key="k" :value="k">{{ lb }}</option>
              </select>
            </label>
            <label>处置日期<input class="input" type="date" v-model="disposal.form.date" /></label>
            <label>残值回收金额（元）<input class="input" type="number" min="0" step="0.01" v-model.number="disposal.form.amount" /></label>
            <label>处置凭证编号<input class="input" v-model="disposal.form.certificate_no" placeholder="如：报废单/回收合同编号" /></label>
            <label>见证人<input class="input" v-model="disposal.form.witness" placeholder="处置现场见证人" /></label>
            <label class="full ar-wipe" :class="{ on: disposal.form.data_wiped }">
              <input type="checkbox" v-model="disposal.form.data_wiped" />
              <span><SvgIcon name="shield" /> 已确认存储介质数据擦除/物理销毁（保密合规必选项，未确认无法完成处置）</span>
            </label>
            <label class="full">处置备注<textarea class="input" rows="3" v-model="disposal.form.note" placeholder="处置过程说明（运输/交接/销毁方式等）"></textarea></label>
          </div>
        </div>
        <div class="drawer-foot">
          <button class="btn btn-primary" :disabled="disposal.saving" @click="submitDisposal">{{ disposal.saving ? '提交中…' : '完成处置登记' }}</button>
          <button class="btn" @click="disposal.show = false">取消</button>
        </div>
      </div>
    </div>

    <!-- 详情抽屉 -->
    <div v-if="detail.show" class="drawer-mask" @click.self="detail.show = false">
      <div class="drawer" style="width:700px">
        <div class="drawer-head">
          <h3><SvgIcon name="doc" /> {{ detail.data.retirement?.code }} · {{ detail.data.status_label }}</h3>
          <button class="btn btn-mini" @click="detail.show = false"><SvgIcon name="x" /></button>
        </div>
        <div class="drawer-body" v-if="detail.data.retirement">
          <div class="ar-timeline">
            <div v-for="(t, i) in detail.data.timeline" :key="i" class="ar-tl-item" :class="{ done: t.done, todo: !t.done }">
              <span class="ar-tl-dot"></span>
              <div>
                <b>{{ t.label }}</b>
                <span v-if="t.time" class="muted small"> · {{ t.time }} · {{ t.who || '—' }}</span>
                <span v-if="t.note" class="muted small"> · {{ t.note }}</span>
              </div>
            </div>
          </div>
          <div class="ar-sec"><b>资产快照</b>
            <div class="ar-kv">
              <span>编码</span><i class="mono">{{ detail.data.asset?.code }}</i>
              <span>名称</span><i>{{ detail.data.asset?.name }}</i>
              <span>分类</span><i>{{ catLabel(detail.data.asset?.category) }}</i>
              <span>品牌/型号</span><i>{{ detail.data.asset?.brand || '—' }} {{ detail.data.asset?.model || '' }}</i>
              <span>SN</span><i class="mono">{{ detail.data.asset?.sn || '—' }}</i>
              <span>站点</span><i>{{ detail.data.site_name || '—' }}</i>
              <span>台账状态</span><i>{{ ['','在架','在库','维修中','已报废'][detail.data.asset?.status] || '—' }}</i>
            </div>
          </div>
          <div class="ar-sec"><b>申请信息</b>
            <div class="ar-kv">
              <span>退役原因</span><i>{{ detail.data.reason_label }}</i>
              <span>申请人</span><i>{{ detail.data.applicant_name || '—' }} · {{ fmtTime(detail.data.retirement.applied_at) }}</i>
              <span v-if="detail.data.retirement.source_type === 'stocktake'">来源</span><i v-if="detail.data.retirement.source_type === 'stocktake'">盘点盘亏转办</i>
            </div>
            <p class="ar-desc">{{ detail.data.retirement.description }}</p>
          </div>
          <div class="ar-sec" v-if="detail.data.retirement.approved_at"><b>审批</b>
            <div class="ar-kv">
              <span>审批人</span><i>{{ detail.data.approver_name || '—' }} · {{ fmtTime(detail.data.retirement.approved_at) }}</i>
              <span>意见</span><i>{{ detail.data.retirement.approve_note || '—' }}</i>
            </div>
          </div>
          <div class="ar-sec" v-if="detail.data.retirement.status === 4"><b>处置信息</b>
            <div class="ar-kv">
              <span>处置方式</span><i>{{ METHODS[detail.data.retirement.disposal_method] || detail.data.retirement.disposal_method }}</i>
              <span>处置日期</span><i>{{ fmtDate(detail.data.retirement.disposal_date) }}</i>
              <span>残值回收</span><i class="mono">{{ fmtAmount(detail.data.retirement.disposal_amount) }} 元</i>
              <span>凭证编号</span><i class="mono">{{ detail.data.retirement.certificate_no || '—' }}</i>
              <span>数据擦除</span><i>{{ detail.data.retirement.data_wiped ? '已确认' : '未确认' }}</i>
              <span>见证人</span><i>{{ detail.data.retirement.witness || '—' }}</i>
              <span>登记人</span><i>{{ detail.data.disposal_by_name || '—' }} · {{ fmtTime(detail.data.retirement.disposal_at) }}</i>
            </div>
            <p class="ar-desc" v-if="detail.data.retirement.disposal_note">{{ detail.data.retirement.disposal_note }}</p>
          </div>
        </div>
        <div class="drawer-foot">
          <button class="btn btn-primary" v-if="detail.data.retirement?.status === 1 && auth.isAdmin" @click="openApprove({ ...detail.data.retirement, asset_name: detail.data.asset?.name, asset_code: detail.data.asset?.code, applicant_name: detail.data.applicant_name })">审批</button>
          <button class="btn btn-primary" v-if="detail.data.retirement?.status === 2 && canOps" @click="openDisposal({ ...detail.data.retirement, asset_name: detail.data.asset?.name, asset_code: detail.data.asset?.code, approver_name: detail.data.approver_name })">处置登记</button>
          <a class="btn" v-if="detail.data.asset?.id" @click="gotoAsset">资产台账 ›</a>
          <span style="flex:1"></span>
          <button class="btn" @click="detail.show = false">关闭</button>
        </div>
      </div>
    </div>
    <ReplacementDrawer v-if="repl.show" :asset-id="repl.asset.id" :asset-name="repl.asset.name" @close="repl.show=false" @created="onReplCreated" />
  </div>
</template>

<script setup>
import { reactive, ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../api'
import { toast, showConfirm } from '../ui-dialog'
import SvgIcon from '../components/SvgIcon.vue'
import ReplacementDrawer from '../components/ReplacementDrawer.vue'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const REASONS = { age: '服役到期', fault: '故障不可修复', loss: '盘点盘亏', upgrade: '技术更新淘汰', other: '其他' }
const STATUS = { 1: '待审批', 2: '已批准待处置', 3: '已驳回', 4: '处置完成', 5: '已撤销' }
const METHODS = { resale: '转售', scrap: '报废销毁', takeback: '厂商回收', donate: '捐赠', other: '其他' }
const CATS = { server: '服务器', storage: '存储', network: '网络设备', power: '供电设备', cooling: '制冷设备', security: '安防', other: '其他' }
const catLabel = c => CATS[c] || c || '—'
const canOps = computed(() => ['global_admin', 'site_admin', 'operator'].includes(auth.role))
const repl = reactive({ show: false, asset: null })
function openRepl(r){ repl.asset = { id: r.asset_id, name: r.asset_name }; repl.show = true }
function onReplCreated(){ repl.show = false; load(page.value) }

const list = ref([]), summary = ref({}), total = ref(0), page = ref(1), size = 20
const filters = reactive({ status: '', reason: '', keyword: '' })

const create = reactive({ show: false, saving: false, assetKw: '', results: [], searched: false, asset: {}, form: { reason: 'age', description: '', source_type: '', source_id: 0 } })
const approve = reactive({ show: false, saving: false, row: {}, form: { approve: true, note: '' } })
const disposal = reactive({ show: false, saving: false, row: {}, form: {} })
const detail = reactive({ show: false, data: {} })

function fmtTime(t) { return t ? String(t).replace('T', ' ').slice(0, 16) : '—' }
function fmtDate(t) { return t ? String(t).slice(0, 10) : '—' }
function fmtAmount(v) { return Number(v || 0).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }
function canCancel(r) { return auth.isAdmin || r.applicant_id === auth.user?.id }
function quickFilter(k, v) { filters.status = filters.status === v ? '' : v; load(1) }

async function load(p) {
  page.value = p || page.value
  const q = { page: page.value, size }
  if (filters.status) q.status = filters.status
  if (filters.reason) q.reason = filters.reason
  if (filters.keyword) q.keyword = filters.keyword
  const r = await api.get('/asset-retirements', q)
  if (r.code === 0) {
    list.value = r.data.items || []
    total.value = r.data.total || 0
    summary.value = r.data.summary || {}
  }
}

let searchTimer = null
function searchAssets() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(async () => {
    const kw = create.assetKw.trim()
    if (!kw) { create.results = []; create.searched = false; return }
    const r = await api.get('/assets', { keyword: kw, size: 8 })
    create.results = (r.data?.items || []).filter(a => a.status !== 4)
    create.searched = true
  }, 300)
}
function pickAsset(a) { create.asset = a }

function openCreate(prefill) {
  create.assetKw = ''; create.results = []; create.searched = false
  create.asset = prefill?.asset || {}
  create.form = { reason: prefill?.reason || 'age', description: prefill?.description || '', source_type: prefill?.source_type || '', source_id: prefill?.source_id || 0 }
  create.show = true
}

async function submitCreate() {
  if (!create.asset.id) return toast('请先搜索并选择退役资产', 'error')
  if (!create.form.description.trim()) return toast('请填写退役说明（处置留痕是检查材料）', 'error')
  create.saving = true
  const r = await api.post('/asset-retirements', {
    asset_id: create.asset.id, reason: create.form.reason, description: create.form.description,
    source_type: create.form.source_type, source_id: create.form.source_id
  })
  create.saving = false
  if (r.code !== 0) return toast(r.msg || '提交失败', 'error')
  create.show = false
  toast(`退役申请已提交（${r.data.code}），等待管理员审批`)
  await load(1)
}

function openApprove(row) {
  approve.row = row
  approve.form = { approve: true, note: '' }
  approve.show = true
  detail.show = false
}
async function submitApprove() {
  approve.saving = true
  const r = await api.post(`/asset-retirements/${approve.row.id}/approve`, approve.form)
  approve.saving = false
  if (r.code !== 0) return toast(r.msg || '审批失败', 'error')
  approve.show = false
  toast(approve.form.approve ? '已批准，资产已置为报废（如在架已自动下架）' : '已驳回申请')
  await load()
}

function openDisposal(row) {
  disposal.row = row
  disposal.form = { method: 'scrap', date: new Date().toISOString().slice(0, 10), amount: 0, certificate_no: '', data_wiped: false, witness: '', note: '' }
  disposal.show = true
  detail.show = false
}
async function submitDisposal() {
  if (!disposal.form.data_wiped) return toast('必须确认数据已擦除/介质已销毁（保密合规要求）', 'error')
  disposal.saving = true
  const r = await api.post(`/asset-retirements/${disposal.row.id}/disposal`, disposal.form)
  disposal.saving = false
  if (r.code !== 0) return toast(r.msg || '登记失败', 'error')
  disposal.show = false
  toast('处置登记完成，退役流程闭环')
  await load()
}

async function cancel(r) {
  if (!(await showConfirm({ title: '撤销退役申请', message: `确认撤销 ${r.code}（${r.asset_name}）的退役申请？`, tone: 'danger', confirmText: '撤销申请' }))) return
  const rr = await api.post(`/asset-retirements/${r.id}/cancel`, {})
  if (rr.code !== 0) return toast(rr.msg || '撤销失败', 'error')
  toast('申请已撤销')
  await load()
}

async function openDetail(id) {
  const r = await api.get(`/asset-retirements/${id}`)
  if (r.code !== 0) return toast(r.msg || '加载失败', 'error')
  detail.data = r.data
  detail.show = true
}
function gotoAsset() {
  detail.show = false
  router.push('/assets?keyword=' + encodeURIComponent(detail.data.asset?.code || ''))
}

// 深链双路径：?new=1&asset_id&reason&source_type&source_id（发起）；?id=N&detail=1（详情）
async function applyQuery(q) {
  if (q.new === '1') {
    let asset = null
    if (q.asset_id) {
      const r = await api.get('/assets', { id: q.asset_id })
      asset = (r.data?.items || [])[0] || null
    }
    openCreate({ asset, reason: q.reason || 'age', source_type: q.source_type || '', source_id: Number(q.source_id || 0) })
  } else if (q.id && q.detail === '1') {
    openDetail(q.id)
  }
}
watch(() => route.query, q => { if (route.path === '/asset-retirement') applyQuery(q) })

onMounted(() => { load(1); applyQuery(route.query) })
</script>

<style scoped>
.page-head { display: flex; align-items: flex-start; justify-content: space-between; margin-bottom: 16px; }
.page-head h2 { display: flex; align-items: center; gap: 8px; margin: 0 0 6px; font-size: 20px; }
.page-head h2 svg { width: 22px; height: 22px; color: var(--accent); }
.page-head p { margin: 0; font-size: 13px; max-width: 760px; }
.muted { color: var(--muted); }
.small { font-size: 12px; }
.mono { font-family: var(--font-mono, monospace); }
.ar-kpis { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-bottom: 16px; }
.ar-kpi { background: var(--card); border: 1px solid var(--border); border-radius: 12px; padding: 14px 16px; display: flex; flex-direction: column; gap: 4px; cursor: pointer; transition: transform .18s ease; }
.ar-kpi:hover { transform: translateY(-2px); }
.ar-kpi .k-v { font-size: 26px; font-weight: 700; font-family: var(--font-mono, monospace); }
.ar-kpi .k-l { font-size: 12px; color: var(--muted); }
.ar-kpi.warn .k-v { color: #e6a23c; }
.toolbar { display: flex; gap: 10px; align-items: center; margin-bottom: 14px; flex-wrap: wrap; }
.card { background: var(--card); border: 1px solid var(--border); border-radius: 12px; }
.tbl-wrap { overflow-x: auto; }
.badge { display: inline-block; font-size: 11.5px; padding: 2px 8px; border-radius: 10px; border: 1px solid var(--border); color: var(--muted); white-space: nowrap; }
.badge.st-1 { color: #e6a23c; border-color: rgba(230, 162, 60, .45); }
.badge.st-2 { color: #409eff; border-color: rgba(64, 158, 255, .45); }
.badge.st-3 { color: #f56c6c; border-color: rgba(245, 108, 108, .45); }
.badge.st-4 { color: #67c23a; border-color: rgba(103, 194, 58, .45); }
.badge.st-5 { color: var(--muted); }
.badge.rsn-age { color: #b37feb; border-color: rgba(179, 127, 235, .4); }
.badge.rsn-fault { color: #f56c6c; border-color: rgba(245, 108, 108, .45); }
.badge.rsn-loss { color: #e6a23c; border-color: rgba(230, 162, 60, .45); }
.badge.rsn-upgrade { color: #409eff; border-color: rgba(64, 158, 255, .4); }
.link { color: var(--accent); cursor: pointer; }
.empty-cell { text-align: center; color: var(--muted); padding: 30px 0 !important; }
.pager { display: flex; align-items: center; justify-content: center; gap: 12px; padding: 12px; }
.fs { border: 1px solid var(--border); border-radius: 8px; padding: 16px; margin-bottom: 16px; }
.fs legend { font-weight: 600; font-size: 14px; padding: 0 8px; color: var(--accent); }
.ar-picked { display: flex; align-items: center; gap: 12px; border: 1.5px solid var(--accent); border-radius: 10px; padding: 12px 14px; background: rgba(64, 158, 255, .05); }
.ar-picked svg { width: 26px; height: 26px; color: var(--accent); flex-shrink: 0; }
.ar-picked-info { flex: 1; min-width: 0; }
.ar-search { border: 1px solid var(--border); border-radius: 8px; margin-top: 8px; max-height: 220px; overflow-y: auto; }
.ar-search-item { display: flex; align-items: center; gap: 10px; padding: 9px 12px; cursor: pointer; border-bottom: 1px solid var(--border); font-size: 13px; }
.ar-search-item:last-child { border-bottom: none; }
.ar-search-item:hover { background: rgba(64, 158, 255, .07); }
.ar-reason-row { display: flex; gap: 10px; flex-wrap: wrap; }
.ar-radio { display: flex !important; flex-direction: row !important; align-items: center; gap: 6px; border: 1.5px solid var(--border); border-radius: 8px; padding: 7px 14px; cursor: pointer; font-size: 13px; transition: all .15s ease; }
.ar-radio.on { border-color: var(--accent); color: var(--accent); background: rgba(64, 158, 255, .06); }
.ar-radio input { accent-color: var(--accent); }
.ar-src { margin: 10px 0 0; padding: 8px 10px; border-left: 3px solid var(--accent); background: rgba(64, 158, 255, .05); border-radius: 0 6px 6px 0; }
.ar-asset-card { border: 1px solid var(--border); border-radius: 10px; padding: 12px 14px; margin-bottom: 12px; }
.ar-desc { font-size: 13px; color: var(--text); line-height: 1.7; margin: 0 0 12px; white-space: pre-wrap; }
.ar-warn { font-size: 12.5px; color: #e6a23c; border: 1px solid rgba(230,162,60,.4); background: rgba(230,162,60,.08); border-radius: 6px; padding: 8px 10px; margin: 0 0 12px; line-height: 1.6; }
.ar-hint { margin: 0 0 12px; padding: 8px 10px; border-left: 3px solid #e6a23c; background: rgba(230, 162, 60, .06); border-radius: 0 6px 6px 0; }
.ar-wipe { display: flex !important; flex-direction: row !important; align-items: center; gap: 10px; border: 1.5px solid var(--border); border-radius: 10px; padding: 12px 14px; cursor: pointer; font-size: 13px; }
.ar-wipe.on { border-color: #67c23a; background: rgba(103, 194, 58, .06); }
.ar-wipe svg { width: 20px; height: 20px; color: #67c23a; flex-shrink: 0; }
.ar-wipe input { accent-color: #67c23a; width: 17px; height: 17px; }
.ar-timeline { margin-bottom: 18px; }
.ar-tl-item { display: flex; gap: 12px; padding: 8px 0 8px 4px; position: relative; font-size: 13px; }
.ar-tl-item:not(:last-child)::before { content: ''; position: absolute; left: 9px; top: 26px; bottom: -8px; width: 2px; background: var(--border); }
.ar-tl-dot { width: 12px; height: 12px; border-radius: 50%; margin-top: 4px; flex-shrink: 0; background: var(--border); }
.ar-tl-item.done .ar-tl-dot { background: #67c23a; box-shadow: 0 0 0 3px rgba(103, 194, 58, .15); }
.ar-tl-item.todo { color: var(--muted); }
.ar-tl-item.todo .ar-tl-dot { background: #e6a23c; box-shadow: 0 0 0 3px rgba(230, 162, 60, .15); }
.ar-sec { margin-bottom: 18px; }
.ar-sec > b { display: block; font-size: 13.5px; margin-bottom: 8px; color: var(--text); }
.ar-kv { display: grid; grid-template-columns: 90px 1fr; gap: 6px 12px; font-size: 13px; }
.ar-kv span { color: var(--muted); }
.ar-kv i { font-style: normal; color: var(--text); }
@media (max-width: 900px) { .ar-kpis { grid-template-columns: repeat(2, 1fr); } }
</style>
