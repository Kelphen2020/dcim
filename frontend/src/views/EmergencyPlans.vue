<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h2><SvgIcon name="shield" /> 应急预案与演练</h2>
        <p class="muted">事前定义应急处置程序，定期演练留痕（机房检查硬性材料）；告警触发时自动匹配推荐，演练问题一键转整改工单。</p>
      </div>
    </div>

    <!-- KPI -->
    <div class="ep-kpis">
      <div class="ep-kpi"><span class="k-v">{{ summary.total || 0 }}</span><span class="k-l">预案总数</span></div>
      <div class="ep-kpi"><span class="k-v">{{ summary.enabled || 0 }}</span><span class="k-l">启用中</span></div>
      <div class="ep-kpi" :class="{ bad: summary.overdue > 0 }"><span class="k-v">{{ summary.overdue || 0 }}</span><span class="k-l">逾期未演练</span></div>
      <div class="ep-kpi" :class="{ warn: summary.due_soon > 0 }"><span class="k-v">{{ summary.due_soon || 0 }}</span><span class="k-l">14 天内到期</span></div>
      <div class="ep-kpi"><span class="k-v">{{ monthDrills }}</span><span class="k-l">本月演练次数</span></div>
    </div>

    <!-- 工具栏 -->
    <div class="toolbar">
      <select class="input" v-model="filters.site_id" style="width:150px" @change="load">
        <option value="">全部站点</option>
        <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
      </select>
      <select class="input" v-model="filters.scenario" style="width:130px" @change="load">
        <option value="">全部场景</option>
        <option v-for="(lb, k) in SCENARIOS" :key="k" :value="k">{{ lb }}</option>
      </select>
      <label class="chk"><input type="checkbox" v-model="filters.overdue" @change="load" /> 只看待演练（逾期/临期）</label>
      <input class="input" v-model="filters.keyword" placeholder="搜索标题/关键词…" style="width:200px" @keyup.enter="load" />
      <span style="flex:1"></span>
      <button class="btn" @click="scanOverdue" :disabled="scanning"><SvgIcon name="bell" />{{ scanning ? '检查中…' : '逾期检查' }}</button>
      <button class="btn btn-primary" v-if="auth.isGlobalAdmin" @click="openEdit(null)"><SvgIcon name="check-square" /> 新建预案</button>
    </div>

    <!-- 列表 -->
    <div class="tbl-wrap card">
      <table class="tbl data-table">
        <thead><tr>
          <th>编号</th><th>预案标题</th><th>场景</th><th>责任人</th><th>演练周期</th>
          <th>最近演练</th><th>演练状态</th><th>演练次数</th><th>状态</th><th style="width:220px">操作</th>
        </tr></thead>
        <tbody>
          <tr v-for="p in list" :key="p.id">
            <td class="mono small">{{ p.code }}</td>
            <td><a class="link" @click="openDetail(p)">{{ p.title }}</a><span v-if="p.builtin" class="badge" style="margin-left:6px">内置</span></td>
            <td><span class="badge" :class="'sc-' + p.scenario">{{ SCENARIOS[p.scenario] || p.scenario }}</span></td>
            <td>{{ p.owner_name || '—' }}</td>
            <td>{{ p.drill_cycle_days > 0 ? p.drill_cycle_days + ' 天' : '—' }}</td>
            <td class="small">{{ p.last_drill_at ? fmtDate(p.last_drill_at) : '从未' }}</td>
            <td><span class="badge" :class="'ds-' + p.drill_state">{{ STATE_LABELS[p.drill_state] }}<template v-if="p.drill_state === 'overdue'"> {{ -p.days_left }} 天</template><template v-else-if="p.drill_state === 'due_soon'"> 剩 {{ p.days_left }} 天</template></span></td>
            <td>{{ p.drill_count || 0 }}</td>
            <td><span class="badge" :class="p.status === 1 ? 'ds-ok' : 'ds-none'">{{ p.status === 1 ? '启用' : '停用' }}</span></td>
            <td>
              <button class="btn btn-mini" @click="openDetail(p)">详情</button>
              <button class="btn btn-mini" @click="openDrill(p)" v-if="p.status === 1">登记演练</button>
              <button class="btn btn-mini" v-if="auth.isGlobalAdmin" @click="openEdit(p)">编辑</button>
            </td>
          </tr>
          <tr v-if="!list.length"><td colspan="10" class="empty-cell">暂无预案</td></tr>
        </tbody>
      </table>
    </div>

    <!-- 详情抽屉 -->
    <div v-if="detail.show" class="drawer-mask" @click.self="detail.show = false">
      <div class="drawer" style="width:720px">
        <div class="drawer-head">
          <h3><SvgIcon name="shield" /> {{ detail.plan.title }}</h3>
          <button class="btn btn-mini" @click="detail.show = false"><SvgIcon name="x" /></button>
        </div>
        <div class="drawer-body">
          <div class="ep-meta">
            <span class="mono small">{{ detail.plan.code }}</span>
            <span class="badge" :class="'sc-' + detail.plan.scenario">{{ SCENARIOS[detail.plan.scenario] || detail.plan.scenario }}</span>
            <span class="badge" :class="'ds-' + detail.drill_state">{{ STATE_LABELS[detail.drill_state] }}</span>
            <span class="muted small">责任人：{{ detail.owner_name || '—' }} · 站点：{{ detail.site_name || '全站通用' }} · 优先级：{{ ['','高','中','低'][detail.plan.priority] }}</span>
          </div>
          <div class="ep-sec"><b>触发情形</b><p>{{ detail.plan.trigger_cond || '—' }}</p></div>
          <div class="ep-sec">
            <b>处置步骤</b>
            <ol class="ep-steps">
              <li v-for="(st, i) in stepList" :key="i">{{ st }}</li>
            </ol>
          </div>
          <div class="ep-sec" v-if="detail.plan.match_keywords || detail.plan.match_point_types">
            <b>告警匹配规则</b>
            <p class="muted small">
              <template v-if="detail.plan.match_point_types">测点类型：{{ detail.plan.match_point_types }}</template>
              <template v-if="detail.plan.match_keywords"> · 关键词：{{ detail.plan.match_keywords }}</template>
            </p>
          </div>
          <div class="ep-sec">
            <b>演练记录（{{ (detail.drills || []).length }}）</b>
            <div v-if="!(detail.drills || []).length" class="muted small" style="margin-top:6px">暂无演练记录——要求周期 {{ detail.plan.drill_cycle_days > 0 ? detail.plan.drill_cycle_days + ' 天' : '（未设置）' }}</div>
            <div v-for="d in detail.drills" :key="d.id" class="ep-drill">
              <div class="ep-drill-hd">
                <b>{{ fmtDate(d.drill_date) }}</b>
                <span class="badge" :class="'rs-' + d.result">{{ RESULT_LABELS[d.result] || d.result }}</span>
                <span class="muted small">{{ d.title }} · {{ d.duration_min }} 分钟 · 参与：{{ d.participants || '—' }}</span>
                <a v-if="d.wo_id" class="link small" @click="$router.push('/work-orders')">整改工单 ›</a>
              </div>
              <p v-if="d.issues" class="ep-issues">发现问题：{{ d.issues }}</p>
            </div>
          </div>
        </div>
        <div class="drawer-foot">
          <button class="btn" @click="openDrill(detail.plan)" v-if="detail.plan.status === 1"><SvgIcon name="check-square" /> 登记演练</button>
          <button class="btn btn-primary" v-if="auth.isGlobalAdmin" @click="openEdit(detail.plan)">编辑预案</button>
          <span style="flex:1"></span>
          <button class="btn" @click="detail.show = false">关闭</button>
        </div>
      </div>
    </div>

    <!-- 新建/编辑抽屉 -->
    <div v-if="edit.show" class="drawer-mask" @click.self="edit.show = false">
      <div class="drawer" style="width:640px">
        <div class="drawer-head">
          <h3>{{ edit.id ? '编辑预案 ' + edit.form.code : '新建应急预案' }}</h3>
          <button class="btn btn-mini" @click="edit.show = false"><SvgIcon name="x" /></button>
        </div>
        <div class="drawer-body">
          <div class="form-grid">
            <label class="full">预案标题 *<input class="input" v-model="edit.form.title" placeholder="如：市电中断应急处置" /></label>
            <label>场景 *
              <select class="input" v-model="edit.form.scenario">
                <option v-for="(lb, k) in SCENARIOS" :key="k" :value="k">{{ lb }}</option>
              </select>
            </label>
            <label>适用站点
              <select class="input" v-model="edit.form.site_id">
                <option :value="0">全站通用</option>
                <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
              </select>
            </label>
            <label>优先级
              <select class="input" v-model="edit.form.priority">
                <option :value="1">高</option><option :value="2">中</option><option :value="3">低</option>
              </select>
            </label>
            <label>责任人
              <select class="input" v-model="edit.form.owner_id">
                <option :value="0">未指定</option>
                <option v-for="u in users" :key="u.id" :value="u.id">{{ u.display_name || u.username }}</option>
              </select>
            </label>
            <label>要求演练周期（天，0=不定期）<input class="input" type="number" min="0" v-model.number="edit.form.drill_cycle_days" /></label>
            <label>告警匹配测点类型（逗号分隔）
              <input class="input mono" v-model="edit.form.match_point_types" placeholder="power_dist,ups,temp_humidity" />
            </label>
            <label class="full">告警匹配关键词（逗号分隔）<input class="input" v-model="edit.form.match_keywords" placeholder="市电,停电,失压" /></label>
            <label class="full">触发情形<textarea class="input" rows="2" v-model="edit.form.trigger_cond" placeholder="什么情况下启动本预案"></textarea></label>
            <label class="full">处置步骤（每行一步，按序执行）*<textarea class="input" rows="7" v-model="edit.form.steps" placeholder="1. 确认…&#10;2. 检查…"></textarea></label>
            <label v-if="edit.id">状态
              <select class="input" v-model="edit.form.status"><option :value="1">启用</option><option :value="0">停用</option></select>
            </label>
          </div>
        </div>
        <div class="drawer-foot">
          <button class="btn btn-primary" @click="saveEdit" :disabled="edit.saving">{{ edit.saving ? '保存中…' : '保存' }}</button>
          <button class="btn" @click="edit.show = false">取消</button>
        </div>
      </div>
    </div>

    <!-- 演练登记抽屉 -->
    <div v-if="drill.show" class="drawer-mask" @click.self="drill.show = false">
      <div class="drawer" style="width:560px">
        <div class="drawer-head">
          <h3><SvgIcon name="check-square" /> 登记演练 · {{ drill.plan.title }}</h3>
          <button class="btn btn-mini" @click="drill.show = false"><SvgIcon name="x" /></button>
        </div>
        <div class="drawer-body">
          <div class="form-grid">
            <label>演练日期<input class="input" type="date" v-model="drill.form.drill_date" /></label>
            <label>用时（分钟）<input class="input" type="number" min="0" v-model.number="drill.form.duration_min" /></label>
            <label class="full">演练名称<input class="input" v-model="drill.form.title" :placeholder="drill.plan.title + ' 演练'" /></label>
            <label class="full">参与人员<input class="input" v-model="drill.form.participants" placeholder="张三,李四,王五" /></label>
            <label class="full">演练结果 *
              <div class="ep-result-row">
                <label v-for="(lb, k) in RESULT_LABELS" :key="k" class="ep-radio" :class="{ on: drill.form.result === k }">
                  <input type="radio" :value="k" v-model="drill.form.result" /> {{ lb }}
                </label>
              </div>
            </label>
            <label class="full">发现问题（可转整改工单）<textarea class="input" rows="3" v-model="drill.form.issues" placeholder="演练中暴露的问题，如：应急照明不足、切换耗时超预期…"></textarea></label>
          </div>
        </div>
        <div class="drawer-foot">
          <button class="btn btn-primary" @click="saveDrill" :disabled="drill.saving">{{ drill.saving ? '保存中…' : '保存演练记录' }}</button>
          <button class="btn" @click="drill.show = false">取消</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { reactive, ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../api'
import { toast, showConfirm } from '../ui-dialog'
import SvgIcon from '../components/SvgIcon.vue'

const auth = useAuthStore()
const router = useRouter()
const SCENARIOS = { power: '市电', ups: 'UPS', hvac: '制冷空调', fire: '消防火情', network: '网络', water: '漏水', security: '安防', other: '其他' }
const STATE_LABELS = { ok: '正常', due_soon: '即将到期', overdue: '已逾期', none: '无周期要求' }
const RESULT_LABELS = { success: '成功', partial: '部分达成', failed: '未达成' }

const list = ref([]), summary = ref({}), sites = ref([]), users = ref([])
const filters = reactive({ site_id: '', scenario: '', keyword: '', overdue: false })
const scanning = ref(false)
const monthDrills = ref(0)

const detail = reactive({ show: false, plan: {}, drills: [], drill_state: 'none', owner_name: '', site_name: '' })
const stepList = computed(() => String(detail.plan.steps || '').split('\n').map(s => s.replace(/^\d+[.、)]\s*/, '').trim()).filter(Boolean))

const edit = reactive({ show: false, id: 0, saving: false, form: {} })
const drill = reactive({ show: false, plan: {}, saving: false, form: {} })

function fmtDate(t) { return t ? String(t).slice(0, 10) : '' }

async function load() {
  const q = {}
  if (filters.site_id) q.site_id = filters.site_id
  if (filters.scenario) q.scenario = filters.scenario
  if (filters.keyword) q.keyword = filters.keyword
  if (filters.overdue) q.overdue = 1
  const r = await api.get('/emergency-plans', q)
  if (r.code === 0) {
    list.value = r.data.list || []
    summary.value = r.data.summary || {}
    // 本月演练次数：从列表内预案的 drill_count 无法得月份，用详情过重——用匹配接口？简化：统计有 last_drill_at 在本月的预案数
    const mPrefix = new Date().toISOString().slice(0, 7)
    monthDrills.value = (r.data.list || []).filter(p => (p.last_drill_at || '').slice(0, 7) === mPrefix).length
  }
}

async function openDetail(p) {
  const r = await api.get(`/emergency-plans/${p.id}`)
  if (r.code !== 0) return toast(r.msg || '加载失败', 'error')
  Object.assign(detail, { show: true, plan: r.data.plan, drills: r.data.drills || [], drill_state: r.data.drill_state, owner_name: r.data.owner_name, site_name: r.data.site_name })
}

function openEdit(p) {
  if (p) {
    edit.id = p.id
    edit.form = { code: p.code, title: p.title, scenario: p.scenario, site_id: p.site_id || 0, priority: p.priority || 2, owner_id: p.owner_id || 0, drill_cycle_days: p.drill_cycle_days || 0, match_keywords: p.match_keywords || '', match_point_types: p.match_point_types || '', trigger_cond: p.trigger_cond || '', steps: p.steps || '', status: p.status }
  } else {
    edit.id = 0
    edit.form = { title: '', scenario: 'power', site_id: 0, priority: 2, owner_id: 0, drill_cycle_days: 90, match_keywords: '', match_point_types: '', trigger_cond: '', steps: '', status: 1 }
  }
  edit.show = true
}

async function saveEdit() {
  if (!edit.form.title || !edit.form.steps) return toast('标题与处置步骤必填', 'error')
  edit.saving = true
  const body = { ...edit.form }; delete body.code
  const r = edit.id ? await api.put(`/emergency-plans/${edit.id}`, body) : await api.post('/emergency-plans', body)
  edit.saving = false
  if (r.code === 0) { toast(edit.id ? '预案已更新' : '预案已创建'); edit.show = false; await load() }
  else toast(r.msg || '保存失败', 'error')
}

function openDrill(p) {
  drill.plan = p
  drill.form = { title: '', drill_date: new Date().toISOString().slice(0, 10), participants: '', duration_min: 30, result: 'success', issues: '' }
  drill.show = true
}

async function saveDrill() {
  drill.saving = true
  const r = await api.post(`/emergency-plans/${drill.plan.id}/drill`, drill.form)
  drill.saving = false
  if (r.code !== 0) return toast(r.msg || '登记失败', 'error')
  drill.show = false
  toast('演练记录已登记')
  // 有问题 → 询问转整改工单
  if (drill.form.issues && drill.form.issues.trim()) {
    const ok = await showConfirm({ title: '转整改工单', message: '本次演练记录了发现问题，是否立即生成【演练整改】工单跟踪闭环？', confirmText: '生成工单' })
    if (ok) {
      const wr = await api.post(`/drill-records/${r.data.id}/work-order`, {})
      if (wr.code === 0) {
        toast(`整改工单已生成（${wr.data.code || 'ID ' + wr.data.work_order_id}）`)
      } else toast(wr.msg || '转工单失败', 'error')
    }
  }
  await load()
}

async function scanOverdue() {
  scanning.value = true
  const r = await api.post('/emergency-plans/scan-overdue', {})
  scanning.value = false
  if (r.code === 0) toast(r.data.notified > 0 ? `逾期检查完成，新通知 ${r.data.notified} 条` : '逾期检查完成，无新增逾期')
  else toast(r.msg || '检查失败', 'error')
  await load()
}

onMounted(async () => {
  load()
  const [sr, ur] = await Promise.all([api.get('/sites'), api.get('/users').catch(() => ({ code: 1 }))])
  if (sr.code === 0) sites.value = sr.data?.list || sr.data || []
  if (ur.code === 0) users.value = ur.data?.list || ur.data || []
})
</script>

<style scoped>
.page-head { display: flex; align-items: flex-start; justify-content: space-between; margin-bottom: 16px; }
.page-head h2 { display: flex; align-items: center; gap: 8px; margin: 0 0 6px; font-size: 20px; }
.page-head h2 svg { width: 22px; height: 22px; color: var(--accent); }
.page-head p { margin: 0; font-size: 13px; max-width: 720px; }
.muted { color: var(--muted); }
.small { font-size: 12px; }
.mono { font-family: var(--font-mono, monospace); }
.ep-kpis { display: grid; grid-template-columns: repeat(5, 1fr); gap: 12px; margin-bottom: 16px; }
.ep-kpi { background: var(--card); border: 1px solid var(--border); border-radius: 12px; padding: 14px 16px; display: flex; flex-direction: column; gap: 4px; transition: transform .18s ease; }
.ep-kpi:hover { transform: translateY(-2px); }
.ep-kpi .k-v { font-size: 26px; font-weight: 700; font-family: var(--font-mono, monospace); }
.ep-kpi .k-l { font-size: 12px; color: var(--muted); }
.ep-kpi.bad .k-v { color: #f56c6c; }
.ep-kpi.warn .k-v { color: #e6a23c; }
.toolbar { display: flex; gap: 10px; align-items: center; margin-bottom: 14px; flex-wrap: wrap; }
.chk { display: flex; align-items: center; gap: 6px; font-size: 13px; color: var(--muted); cursor: pointer; }
.card { background: var(--card); border: 1px solid var(--border); border-radius: 12px; }
.tbl-wrap { overflow-x: auto; }
.badge { display: inline-block; font-size: 11.5px; padding: 2px 8px; border-radius: 10px; border: 1px solid var(--border); color: var(--muted); white-space: nowrap; }
.badge.sc-power, .badge.sc-ups { color: #e6a23c; border-color: rgba(230, 162, 60, .4); }
.badge.sc-hvac { color: #409eff; border-color: rgba(64, 158, 255, .4); }
.badge.sc-fire { color: #f56c6c; border-color: rgba(245, 108, 108, .45); }
.badge.sc-network { color: #b37feb; border-color: rgba(179, 127, 235, .4); }
.badge.sc-water { color: #36cfc9; border-color: rgba(54, 207, 201, .4); }
.badge.ds-ok { color: #67c23a; border-color: rgba(103, 194, 58, .45); }
.badge.ds-due_soon { color: #e6a23c; border-color: rgba(230, 162, 60, .45); }
.badge.ds-overdue { color: #f56c6c; border-color: rgba(245, 108, 108, .5); background: rgba(245, 108, 108, .07); }
.badge.ds-none { color: var(--muted); }
.badge.rs-success { color: #67c23a; border-color: rgba(103, 194, 58, .45); }
.badge.rs-partial { color: #e6a23c; border-color: rgba(230, 162, 60, .45); }
.badge.rs-failed { color: #f56c6c; border-color: rgba(245, 108, 108, .45); }
.link { color: var(--accent); cursor: pointer; }
.empty-cell { text-align: center; color: var(--muted); padding: 30px 0 !important; }
.ep-meta { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 16px; }
.ep-sec { margin-bottom: 18px; }
.ep-sec > b { display: block; font-size: 13.5px; margin-bottom: 8px; color: var(--text); }
.ep-sec p { margin: 0; font-size: 13px; color: var(--muted); line-height: 1.7; }
.ep-steps { margin: 0; padding-left: 20px; }
.ep-steps li { font-size: 13px; line-height: 1.9; color: var(--text); }
.ep-drill { border: 1px solid var(--border); border-radius: 10px; padding: 10px 12px; margin-bottom: 8px; }
.ep-drill-hd { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; font-size: 13px; }
.ep-issues { margin: 6px 0 0; font-size: 12.5px; color: #e6a23c; }
.ep-result-row { display: flex; gap: 10px; }
.ep-radio { display: flex !important; flex-direction: row !important; align-items: center; gap: 6px; border: 1.5px solid var(--border); border-radius: 8px; padding: 7px 14px; cursor: pointer; font-size: 13px; transition: all .15s ease; }
.ep-radio.on { border-color: var(--accent); color: var(--accent); background: rgba(64, 158, 255, .06); }
.ep-radio input { accent-color: var(--accent); }
@media (max-width: 900px) { .ep-kpis { grid-template-columns: repeat(2, 1fr); } }
</style>
