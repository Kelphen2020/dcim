<template>
  <div>
    <div class="page-head"><h1>值班与交接</h1></div>
    <div class="tabs">
      <button class="tab" :class="{on:tab==='duty'}" @click="tab='duty';loadDuty()">值班排班</button>
      <button class="tab" :class="{on:tab==='handover'}" @click="tab='handover';loadHandover()">交接清单</button>
      <button class="tab" :class="{on:tab==='storm'}" @click="tab='storm';loadStorms()">告警风暴</button>
    </div>

    <!-- ═══ 值班排班 ═══ -->
    <template v-if="tab==='duty'">
      <div class="card" style="margin-bottom:14px">
        <div class="duty-now">
          <div class="dn-l">
            <div class="dn-t">当前当班（{{ cur.shift==='day'?'白班 8:00-20:00':cur.shift==='night'?'夜班 20:00-8:00':'全天' }}）</div>
            <div v-if="cur.on_duty" class="dn-u"><b>{{ cur.user_name||cur.username }}</b><span v-if="cur.phone" class="muted"> · {{ cur.phone }}</span><span v-if="cur.remark" class="muted"> · {{ cur.remark }}</span></div>
            <div v-else class="dn-none"><SvgIcon name="alert" />当前时段无人值班——告警工单将不自动指派</div>
          </div>
          <button v-if="isAdmin" class="btn btn-primary" @click="openForm()">＋ 排班</button>
        </div>
      </div>
      <div class="toolbar">
        <input class="input" type="month" v-model="month" @change="loadDuty()" style="width:160px" />
        <span class="muted" style="font-size:12.5px">共 {{ dutyList.length }} 条</span>
        <span style="flex:1"></span>
        <button v-if="isAdmin" class="btn" @click="quickWeek"><SvgIcon name="bolt" />快速排本周（按周轮转）</button>
      </div>
      <div class="card"><div class="table-wrap"><table class="data-table">
        <thead><tr><th>日期</th><th>星期</th><th>班次</th><th>值班人</th><th>站点</th><th>备注</th><th v-if="isAdmin">操作</th></tr></thead>
        <tbody>
          <tr v-for="d in dutyList" :key="d.id" :class="{today: isToday(d.duty_date)}">
            <td class="mono">{{ String(d.duty_date).slice(0,10) }}<span v-if="isToday(d.duty_date)" class="badge today-b">今天</span></td>
            <td>{{ weekday(d.duty_date) }}</td>
            <td><span class="badge" :class="'sh-'+d.shift">{{ SHIFTS[d.shift]||d.shift }}</span></td>
            <td><b>{{ d.user_name }}</b></td>
            <td class="muted">{{ d.site_id ? siteName(d.site_id) : '全部站点' }}</td>
            <td class="muted">{{ d.remark||'-' }}</td>
            <td v-if="isAdmin"><button class="btn btn-mini btn-danger" @click="delDuty(d)">删除</button></td>
          </tr>
          <tr v-if="!dutyList.length"><td :colspan="isAdmin?7:6" class="empty">本月暂无排班</td></tr>
        </tbody>
      </table></div></div>
    </template>

    <!-- ═══ 交接清单 ═══ -->
    <template v-if="tab==='handover'">
      <div class="toolbar">
        <span class="muted" style="font-size:13px">生成时间 {{ ho.generated_at || '-' }}</span>
        <span style="flex:1"></span>
        <button class="btn" @click="loadHandover">⟳ 刷新</button>
        <button class="btn btn-primary" @click="printHO"><SvgIcon name="printer" />打印交接单</button>
      </div>
      <div id="handover-print">
        <div class="ho-grid">
          <div class="card">
            <div class="card-title"><SvgIcon name="bell" />未闭环告警（{{ ho.open_alert_count||0 }}）<a class="link" @click="$router.push('/alert-wall')">盯屏 ›</a></div>
            <table class="data-table"><thead><tr><th>级别</th><th>内容</th><th>触发时间</th><th>状态</th></tr></thead><tbody>
              <tr v-for="a in (ho.open_alerts||[]).slice(0,15)" :key="a.id">
                <td><span class="badge" :class="'sev-'+a.severity">{{ SEV[a.severity] }}</span></td>
                <td>{{ a.message }}</td><td class="muted">{{ fmt(a.triggered_at) }}</td>
                <td>{{ ALST[a.status] }}</td>
              </tr>
              <tr v-if="!(ho.open_alerts||[]).length"><td colspan="4" class="empty ok"><SvgIcon name="check" />无未闭环告警</td></tr>
            </tbody></table>
          </div>
          <div class="card">
            <div class="card-title"><SvgIcon name="log" />进行中工单（{{ ho.open_order_count||0 }}）<a class="link" @click="$router.push('/work-orders')">工单 ›</a></div>
            <table class="data-table"><thead><tr><th>编码</th><th>标题</th><th>优先级</th><th>SLA</th></tr></thead><tbody>
              <tr v-for="w in (ho.open_orders||[]).slice(0,15)" :key="w.id">
                <td class="mono">{{ w.code }}</td><td>{{ w.title }}</td>
                <td><span class="badge" :class="'pri-'+w.priority">{{ PRI[w.priority] }}</span></td>
                <td><span v-if="w.sla_respond_breach||w.sla_resolve_breach" class="badge sla-breach-tag">违约</span><span v-else-if="w.sla_respond_warned||w.sla_resolve_warned" class="badge sla-warn-tag">预警</span><span v-else class="muted">正常</span></td>
              </tr>
              <tr v-if="!(ho.open_orders||[]).length"><td colspan="4" class="empty ok"><SvgIcon name="check" />无进行中工单</td></tr>
            </tbody></table>
          </div>
        </div>
        <div class="ho-grid" style="margin-top:12px">
          <div class="card">
            <div class="card-title"><SvgIcon name="clipboard" />今日巡检（{{ (ho.today_inspections||[]).length }}）</div>
            <table class="data-table"><thead><tr><th>报告号</th><th>位置</th><th>结论</th><th>时间</th></tr></thead><tbody>
              <tr v-for="r in ho.today_inspections||[]" :key="r.id">
                <td class="mono">{{ r.report_code||'#'+r.id }}</td><td>{{ r.target_name||'-' }}</td>
                <td><span class="badge" :class="r.status===2?'sla-breach-tag':'sla-ok-tag'">{{ r.status===2?'异常':'正常' }}</span></td>
                <td class="muted">{{ fmt(r.inspected_at) }}</td>
              </tr>
              <tr v-if="!(ho.today_inspections||[]).length"><td colspan="4" class="empty">今日暂无巡检</td></tr>
            </tbody></table>
          </div>
          <div class="card">
            <div class="card-title"><SvgIcon name="storm" />活跃告警风暴（{{ (ho.active_storms||[]).length }}）</div>
            <div v-for="s in ho.active_storms||[]" :key="s.id" class="storm-mini" @click="tab='storm';loadStorms()">
              <b>{{ s.root_name }}</b> · {{ s.alert_count }} 条告警（抑制 {{ s.suppressed_count }}）<span class="muted"> {{ fmt(s.first_alert_at) }} 起</span>
            </div>
            <div v-if="!(ho.active_storms||[]).length" class="empty ok" style="padding:16px"><SvgIcon name="check" />无活跃风暴</div>
          </div>
        </div>
        <div class="ho-sign">
          <div class="ho-sign-row"><span>交班人签字</span><i></i></div>
          <div class="ho-sign-row"><span>接班人签字</span><i></i></div>
          <div class="ho-sign-row"><span>交接时间</span><i></i></div>
        </div>
      </div>
    </template>

    <!-- ═══ 告警风暴 ═══ -->
    <template v-if="tab==='storm'">
      <div class="card" style="margin-bottom:12px">
        <div class="card-title"><SvgIcon name="storm" />风暴抑制机制说明</div>
        <div class="muted" style="font-size:13px;line-height:1.8">
          同一供电子树（市电→变压器→UPS→PDU→机柜）<b>10 分钟内产生 ≥5 条告警</b>时自动聚合为一次「告警风暴」：
          根因=子树最上游供电节点，首条告警保留通知与自动工单，其余告警标记抑制（不再重复通知/开工单）。
          成员告警全部结束或 1 小时静默后风暴自动关闭。
        </div>
      </div>
      <div class="card"><div class="table-wrap"><table class="data-table">
        <thead><tr><th>ID</th><th>根因对象</th><th>告警数</th><th>抑制数</th><th>状态</th><th>首条告警</th><th>最后告警</th><th>结束时间</th><th>操作</th></tr></thead>
        <tbody>
          <tr v-for="s in storms" :key="s.id">
            <td class="mono">#{{ s.id }}</td>
            <td><b>{{ s.root_name }}</b><span class="muted"> ({{ s.root_kind }})</span></td>
            <td>{{ s.alert_count }}</td><td>{{ s.suppressed_count }}</td>
            <td><span class="badge" :class="s.status===0?'sla-breach-tag':'sla-ok-tag'">{{ s.status===0?'活跃':'已结束' }}</span></td>
            <td class="muted">{{ fmt(s.first_alert_at) }}</td><td class="muted">{{ fmt(s.last_alert_at) }}</td><td class="muted">{{ fmt(s.ended_at) }}</td>
            <td><button class="btn btn-mini" @click="openStorm(s)">成员告警</button></td>
          </tr>
          <tr v-if="!storms.length"><td colspan="9" class="empty ok"><SvgIcon name="check" />暂无告警风暴记录</td></tr>
        </tbody>
      </table></div></div>
    </template>

    <!-- 排班抽屉 -->
    <div v-if="form.show" class="drawer-mask" @click.self="form.show=false"><div class="drawer" style="width:480px">
      <div class="drawer-head"><h3>新增值班排班</h3><button class="btn btn-mini" @click="form.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <div class="form-grid">
          <label class="full">值班人 *<select class="input" v-model.number="form.user_id"><option :value="0">请选择</option><option v-for="u in users" :key="u.id" :value="u.id">{{ u.display_name }}（{{ u.username }}）</option></select></label>
          <label>班次<select class="input" v-model="form.shift"><option value="day">白班 8:00-20:00</option><option value="night">夜班 20:00-8:00</option><option value="full">全天</option></select></label>
          <label>站点<select class="input" v-model.number="form.site_id"><option :value="0">全部站点</option><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option></select></label>
          <label class="full">值班日期 *（多天用逗号分隔，如 2026-09-24,2026-09-25）<input class="input" v-model="form.duty_date" placeholder="2026-09-24" /></label>
          <label class="full">备注<input class="input" v-model="form.remark" /></label>
        </div>
        <p v-if="form.err" class="err">{{ form.err }}</p>
      </div>
      <div class="drawer-foot"><button class="btn" @click="form.show=false">取消</button><button class="btn btn-primary" @click="saveDuty">保存</button></div>
    </div></div>

    <!-- 风暴成员抽屉 -->
    <div v-if="storm.show" class="drawer-mask" @click.self="storm.show=false"><div class="drawer">
      <div class="drawer-head"><h3>风暴 #{{ storm.s?.id }} · {{ storm.s?.root_name }} 成员告警</h3><button class="btn btn-mini" @click="storm.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <div v-for="a in storm.alerts" :key="a.id" class="st-alert" :class="{sup: a.suppressed}">
          <span class="badge" :class="'sev-'+a.severity">{{ SEV[a.severity] }}</span>
          <span class="st-msg">{{ a.message }}</span>
          <span v-if="a.suppressed" class="badge sup-b">已抑制</span>
          <span v-else-if="a.storm_id" class="badge root-b">根因</span>
          <span class="muted">{{ fmt(a.triggered_at) }}</span>
        </div>
        <div v-if="!storm.alerts.length" class="empty">无成员告警</div>
      </div>
    </div></div>
  </div>
</template>
<script setup>
import {ref,reactive,onMounted} from 'vue'
import api from '../api'
import { toastAuto, showConfirm } from '../ui-dialog'
import {useAuthStore} from '../stores/auth'
const auth=useAuthStore()
const isAdmin=auth.isAdmin
const SEV={1:'提示',2:'警告',3:'严重'}
const ALST={0:'待处理',1:'已挂起',2:'已确认',3:'已结束'}
const PRI={1:'低',2:'中',3:'高',4:'紧急'}
const SHIFTS={day:'白班',night:'夜班',full:'全天'}
const tab=ref('duty')
const sites=ref([]),users=ref([]),dutyList=ref([]),cur=ref({}),month=ref(new Date().toISOString().slice(0,7))
const ho=ref({}),storms=ref([])
const form=reactive({show:false,user_id:0,shift:'day',site_id:0,duty_date:'',remark:'',err:''})
const storm=reactive({show:false,s:null,alerts:[]})
const fmt=t=>t?String(t).replace('T',' ').slice(5,16):'-'
const siteName=id=>(sites.value.find(s=>s.id===id)||{}).name||'-'
const weekday=d=>['日','一','二','三','四','五','六'][new Date(String(d).slice(0,10)).getDay()]
const isToday=d=>String(d).slice(0,10)===new Date().toISOString().slice(0,10)
async function loadCur(){const r=await api.get('/duty/current',{site_id:1});cur.value=r.data||{}}
async function loadDuty(){const r=await api.get('/duty-shifts',{month:month.value});dutyList.value=r.data?.items||[];loadCur()}
async function loadHandover(){const r=await api.get('/duty/handover');ho.value=r.data||{}}
async function loadStorms(){const r=await api.get('/alert-storms');storms.value=r.data?.items||[]}
function openForm(){const t=new Date();Object.assign(form,{show:true,user_id:0,shift:'day',site_id:0,duty_date:t.toISOString().slice(0,10),remark:'',err:''})}
async function saveDuty(){form.err='';if(!form.user_id||!form.duty_date){form.err='值班人与日期必填';return}
  const r=await api.post('/duty-shifts',{user_id:form.user_id,shift:form.shift,site_id:form.site_id,duty_date:form.duty_date,remark:form.remark})
  if(r.code===0||r._status===201){form.show=false;loadDuty()}else form.err=r.msg}
async function delDuty(d){if (!(await showConfirm({ message: '删除该排班？' })))return;const r=await api.del(`/duty-shifts/${d.id}`);if(r.code===0)loadDuty();else toastAuto(r.msg)}
async function quickWeek(){
  if(users.value.length<2){toastAuto('至少需要2个用户才能轮转');return}
  const lines=[];const base=new Date()
  for(let i=0;i<7;i++){const d=new Date(base);d.setDate(base.getDate()+i)
    const u=users.value[i%users.value.length]
    lines.push({user_id:u.id,shift:'full',site_id:0,duty_date:d.toISOString().slice(0,10),remark:'周轮转'})}
  let ok=0
  for(const l of lines){const r=await api.post('/duty-shifts',l);if(r.code===0||r._status===201)ok++}
  toastAuto(`已排 ${ok}/7 天`);loadDuty()}
async function openStorm(s){storm.s=s;storm.alerts=[];storm.show=true;const r=await api.get(`/alert-storms/${s.id}`);if(r.code===0)storm.alerts=r.data?.alerts||[]}
function printHO(){window.print()}
onMounted(async()=>{const[ss,uu]=await Promise.all([api.get('/sites',{size:100}),api.get('/users-options')]);sites.value=ss.data?.items||[];users.value=uu.data?.items||[]
  const q=new URLSearchParams(location.hash.split('?')[1]||'').get('tab')
  if(q==='storm'){tab.value='storm';loadStorms()}else if(q==='handover'){tab.value='handover';loadHandover()}else loadDuty()})
</script>
<style scoped>
.tabs{display:flex;gap:8px;margin-bottom:14px}.tab{padding:7px 18px;border:1px solid var(--border);border-radius:8px;background:var(--card);cursor:pointer;font-size:13.5px}.tab.on{color:var(--accent);border-color:var(--accent);font-weight:600}
.duty-now{display:flex;align-items:center;gap:16px}.dn-l{flex:1}
.dn-t{font-size:13px;color:var(--muted);margin-bottom:4px}.dn-u{font-size:17px}.dn-none{color:#e6a23c;font-size:14px}
.badge.today-b{margin-left:6px;color:#409eff;border-color:rgba(64,158,255,.4)}
.badge.sh-day{color:#e6a23c;border-color:rgba(230,162,60,.4)}.badge.sh-night{color:#9d7bff;border-color:rgba(157,123,255,.4)}.badge.sh-full{color:#409eff;border-color:rgba(64,158,255,.4)}
.badge.sev-1{color:#409eff;border-color:rgba(64,158,255,.4)}.badge.sev-2{color:#e6a23c;border-color:rgba(230,162,60,.4)}.badge.sev-3{color:#f56c6c;border-color:rgba(245,108,108,.4)}
.badge.pri-4{color:#f56c6c;border-color:rgba(245,108,108,.4)}.badge.pri-3{color:#e6a23c;border-color:rgba(230,162,60,.4)}
.sla-breach-tag{color:#f56c6c;border-color:rgba(245,108,108,.45)}.sla-warn-tag{color:#e6a23c;border-color:rgba(230,162,60,.45)}.sla-ok-tag{color:#67c23a;border-color:rgba(103,194,58,.4)}
.ho-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}
.card-title{display:flex;align-items:center;font-size:14px;font-weight:600;margin-bottom:10px}
.card-title .link{margin-left:auto;font-size:12px;font-weight:400;color:var(--accent);cursor:pointer}
.link{color:var(--accent);cursor:pointer}
.empty.ok{color:#67c23a}
.storm-mini{padding:8px 10px;background:var(--bg2);border-radius:8px;margin-bottom:6px;font-size:13px;cursor:pointer}
.storm-mini:hover{border:1px solid var(--accent)}
.ho-sign{display:flex;gap:40px;margin-top:20px;padding:16px 8px}
.ho-sign-row{display:flex;align-items:center;gap:10px;font-size:14px;flex:1}
.ho-sign-row i{flex:1;border-bottom:1px solid var(--text2,#666);min-width:100px}
.st-alert{display:flex;align-items:center;gap:8px;padding:7px 4px;border-bottom:1px dashed var(--border);font-size:13px;flex-wrap:wrap}
.st-alert.sup{opacity:.55}
.st-msg{flex:1}
.sup-b{color:#909399;border-color:rgba(144,147,153,.4)}
.root-b{color:#f56c6c;border-color:rgba(245,108,108,.4)}
.btn-danger{color:#f56c6c;border-color:rgba(245,108,108,.4)}
.drawer-mask{position:fixed;inset:0;background:rgba(0,0,0,.35);z-index:1000;display:flex;justify-content:flex-end}
.drawer{width:600px;max-width:95vw;height:100%;background:var(--bg);box-shadow:-4px 0 20px rgba(0,0,0,.15);display:flex;flex-direction:column;overflow:hidden}
.drawer-head{display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--border);flex-shrink:0}
.drawer-head h3{margin:0;font-size:17px}
.drawer-body{flex:1;overflow-y:auto;padding:20px}
.drawer-foot{padding:14px 20px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;gap:10px;flex-shrink:0}
.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}
.form-grid label{display:flex;flex-direction:column;gap:4px;font-size:13px;color:var(--muted)}
.form-grid label.full{grid-column:1/-1}
.err{color:#f56c6c;font-size:13px}
@media (max-width:900px){.ho-grid{grid-template-columns:1fr}.form-grid{grid-template-columns:1fr}}
</style>
<style>
@media print{.sidebar,.topbar,.nav-backdrop,.tabs,.toolbar{display:none!important}}
</style>
