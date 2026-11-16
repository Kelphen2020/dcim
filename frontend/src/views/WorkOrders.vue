<template>
  <div>
    <div class="page-head"><h1>运维工单</h1></div>
    <div class="tabs">
      <button class="tab" :class="{on:mainTab==='list'}" @click="mainTab='list'"><SvgIcon name="log" />工单列表</button>
      <button class="tab" :class="{on:mainTab==='sla'}" @click="switchSla"><SvgIcon name="clock" />SLA 考核</button>
    </div>
    <template v-if="mainTab==='list'">
    <div class="toolbar">
      <select class="input" style="width:150px" v-model.number="f.site_id" @change="load(1)"><option :value="0">全部站点</option><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option></select>
      <select class="input" style="width:120px" v-model="f.status" @change="load(1)"><option value="">全部状态</option><option v-for="(n,k) in WOST" :key="k" :value="k">{{ n }}</option></select>
      <select class="input" style="width:120px" v-model="f.priority" @change="load(1)"><option value="">全部优先级</option><option v-for="(n,k) in PRI" :key="k" :value="k">{{ n }}</option></select>
      <select class="input" style="width:120px" v-model="f.sla" @change="load(1)"><option value="">SLA 全部</option><option value="breach">已违约</option><option value="warn">预警中</option><option value="ok">正常</option></select>
      <input class="input search-input" v-model="f.keyword" placeholder="搜索标题/编码" @keyup.enter="load(1)" />
      <button class="btn" @click="load(1)">查询</button><span style="flex:1"></span>
      <button class="btn" @click="openSla"><SvgIcon name="clock" />SLA 时限</button>
      <button class="btn btn-primary" @click="openForm()">＋ 新建工单</button>
    </div>
    <div class="card"><div class="table-wrap"><table class="data-table">
      <thead><tr><th>编码</th><th>标题</th><th>类型</th><th>优先级</th><th>状态</th><th>SLA</th><th>关联资产</th><th>处理人</th><th>创建时间</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="w in list" :key="w.id">
          <td class="mono"><a class="link" @click="openDetail(w)">{{ w.code }}</a></td>
          <td>{{ w.title }}</td><td><span class="badge">{{ TYPES[w.type]||w.type }}</span></td>
          <td><span class="badge" :class="'pri-'+w.priority">{{ PRI[w.priority] }}</span></td>
          <td><span class="badge" :class="'wost-'+w.status">{{ WOST[w.status] }}</span></td>
          <td><span v-if="w.status>=3" class="sla-done" :class="{'sla-breach-tag':w.sla_respond_breach||w.sla_resolve_breach}">{{ (w.sla_respond_breach||w.sla_resolve_breach)?'已违约':'达标' }}</span>
              <template v-else><span class="sla-badge" :class="slaCls(w)">{{ slaText(w) }}</span></template></td>
          <td><a v-if="w.asset_id" class="link" @click="goAsset(w.asset_id)" title="查看关联资产">{{ assetName(w.asset_id) }}</a><span v-else class="muted">-</span></td>
          <td class="muted">{{ userName(w.assignee_id) }}</td><td class="muted">{{ fmt(w.created_at) }}</td>
          <td class="ops">
            <button v-if="w.status===1" class="btn btn-mini btn-mini-primary" @click="act(w,'accept')">受理</button>
            <button v-if="w.status===2" class="btn btn-mini btn-mini-primary" @click="act(w,'finish')">完成</button>
            <button v-if="w.status===1||w.status===2" class="btn btn-mini" @click="openReassign(w)">改派</button>
            <button v-if="w.status===3" class="btn btn-mini" @click="act(w,'close')">关闭</button>
            <button v-if="w.status<3" class="btn btn-mini" @click="act(w,'cancel')">取消</button>
            <button class="btn btn-mini" @click="openDetail(w)">详情</button>
            <button class="btn btn-mini" @click="openForm(w)">编辑</button>
          </td>
        </tr>
        <tr v-if="!list.length"><td colspan="10" class="empty">暂无工单</td></tr>
      </tbody>
    </table></div><Pagination :page="page" :total="total" :page-size="size" @change="load" /></div>
    </template>

    <!-- V63 SLA 违约分析与考核 -->
    <div v-if="mainTab==='sla'" class="sla-panel">
      <div class="toolbar">
        <select class="input" style="width:130px" v-model.number="sla.days" @change="loadSla"><option :value="7">近 7 天</option><option :value="30">近 30 天</option><option :value="90">近 90 天</option></select>
        <select class="input" style="width:150px" v-model.number="sla.site_id" @change="loadSla"><option :value="0">全部站点</option><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option></select>
        <button class="btn" @click="loadSla">刷新</button><span style="flex:1"></span>
        <button class="btn" @click="openSla">SLA 时限配置</button>
        <button class="btn" @click="exportSlaCsv"><SvgIcon name="download" />导出考核 CSV</button>
      </div>
      <div v-if="!sla.data" class="card muted" style="padding:24px;text-align:center">加载中…</div>
      <template v-else>
      <div class="sla-kpis">
        <div class="sla-kpi"><span class="sk-label">工单总数</span><span class="sk-val">{{ sla.data.kpi.total }}</span><span class="sk-sub">已完成 {{ sla.data.kpi.finished }}</span></div>
        <div class="sla-kpi" :class="rateCls(sla.data.kpi.respond_rate)"><span class="sk-label">响应达标率</span><span class="sk-val">{{ sla.data.kpi.respond_rate }}%</span><span class="sk-sub">响应违约 {{ sla.data.kpi.respond_breach }}</span></div>
        <div class="sla-kpi" :class="rateCls(sla.data.kpi.resolve_rate)"><span class="sk-label">解决达标率</span><span class="sk-val">{{ sla.data.kpi.resolve_rate }}%</span><span class="sk-sub">解决违约 {{ sla.data.kpi.resolve_breach }}</span></div>
        <div class="sla-kpi clickable" :class="sla.data.kpi.breach_total>0?'bad':''" @click="drillBreach" title="点击到列表页查看已违约工单"><span class="sk-label">违约工单</span><span class="sk-val">{{ sla.data.kpi.breach_total }}</span><span class="sk-sub">点击穿透清单 ›</span></div>
        <div class="sla-kpi" :class="sla.data.kpi.open_overdue>0?'bad':''"><span class="sk-label">进行中已超时</span><span class="sk-val">{{ sla.data.kpi.open_overdue }}</span><span class="sk-sub">待扫描器判定</span></div>
        <div class="sla-kpi"><span class="sk-label">平均响应</span><span class="sk-val">{{ sla.data.kpi.avg_respond_h }}h</span><span class="sk-sub">平均解决 {{ sla.data.kpi.avg_resolve_h }}h</span></div>
      </div>
      <div class="sla-cols">
        <div class="card">
          <h4 class="sec-title"><SvgIcon name="user" />按处理人考核<span class="muted" style="font-weight:400;font-size:12px;margin-left:8px">违约多者优先 · 含未指派</span></h4>
          <div class="table-wrap"><table class="data-table">
            <thead><tr><th>处理人</th><th>指派</th><th>完成</th><th>进行中</th><th>违约</th><th style="min-width:140px">违约率</th><th>平均解决(h)</th></tr></thead>
            <tbody>
              <tr v-for="u in sla.data.by_user" :key="u.user_id" :class="{'row-breach':u.breach>0}">
                <td>{{ u.display_name || u.username }}<span v-if="!u.user_id" class="muted">（未指派）</span></td>
                <td>{{ u.assigned }}</td><td>{{ u.finished }}</td><td>{{ u.open }}</td>
                <td :class="u.breach>0?'td-breach':''">{{ u.breach }}</td>
                <td><div class="rate-bar"><div class="rate-fill" :class="rateCls(100-u.breach_rate)" :style="{width:Math.min(100,u.breach_rate)+'%'}"></div><span class="rate-txt">{{ u.breach_rate }}%</span></div></td>
                <td>{{ u.avg_resolve_h || '-' }}</td>
              </tr>
              <tr v-if="!sla.data.by_user.length"><td colspan="7" class="empty">期内暂无工单</td></tr>
            </tbody>
          </table></div>
        </div>
        <div class="sla-side">
          <div class="card">
            <h4 class="sec-title"><SvgIcon name="layers" />按优先级</h4>
            <div class="table-wrap"><table class="data-table">
              <thead><tr><th>优先级</th><th>工单</th><th>响应违约</th><th>解决违约</th><th>响应达标率</th></tr></thead>
              <tbody>
                <tr v-for="p in sla.data.by_priority" :key="p.priority">
                  <td><span class="badge" :class="'pri-'+p.priority">{{ p.label }}</span></td>
                  <td>{{ p.total }}</td><td :class="p.resp_breach>0?'td-breach':''">{{ p.resp_breach }}</td><td :class="p.resolve_breach>0?'td-breach':''">{{ p.resolve_breach }}</td>
                  <td>{{ p.respond_rate }}%</td>
                </tr>
                <tr v-if="!sla.data.by_priority.length"><td colspan="5" class="empty">暂无数据</td></tr>
              </tbody>
            </table></div>
          </div>
          <div class="card">
            <h4 class="sec-title"><SvgIcon name="chart" />逐日趋势<span class="muted" style="font-weight:400;font-size:12px;margin-left:8px">柱=当日创建 · 红=其中违约</span></h4>
            <div class="trend-bars">
              <div v-for="t in sla.data.trend" :key="t.date" class="tb" :title="t.date+'：创建 '+t.created+' · 违约 '+t.breached+' · 完成 '+t.finished">
                <div class="tb-col"><div class="tb-fill" :style="{height:barH(t.created)+'%'}"></div><div v-if="t.breached" class="tb-breach" :style="{height:barH(t.breached)+'%'}"></div></div>
              </div>
            </div>
          </div>
        </div>
      </div>
      <div class="card">
        <h4 class="sec-title"><SvgIcon name="alert" />最近违约工单（TOP {{ sla.data.top_breaches.length }}）</h4>
        <div class="table-wrap"><table class="data-table">
          <thead><tr><th>编码</th><th>标题</th><th>优先级</th><th>违约类型</th><th>状态</th><th>站点</th><th>处理人</th><th>创建时间</th></tr></thead>
          <tbody>
            <tr v-for="t in sla.data.top_breaches" :key="t.id">
              <td class="mono"><a class="link" @click="openDetail(t.wo)">{{ t.code }}</a></td>
              <td>{{ t.title }}</td>
              <td><span class="badge" :class="'pri-'+t.priority">{{ PRI[t.priority] }}</span></td>
              <td><span v-for="k in t.breach_kinds" :key="k" class="badge sla-breach-tag" style="margin-right:4px">{{ k }}</span></td>
              <td><span class="badge" :class="'wost-'+t.status">{{ WOST[t.status] }}</span></td>
              <td class="muted">{{ t.site_name || '-' }}</td><td class="muted">{{ t.assignee_name }}</td><td class="muted">{{ fmt(t.created_at) }}</td>
            </tr>
            <tr v-if="!sla.data.top_breaches.length"><td colspan="8" class="empty">期内无违约工单</td></tr>
          </tbody>
        </table></div>
      </div>
      </template>
    </div>

    <!-- 详情抽屉（含流转日志） -->
    <div v-if="detail.show" class="drawer-mask" @click.self="detail.show=false"><div class="drawer">
      <div class="drawer-head"><h3>工单详情 · {{ detail.wo?.code }}</h3><button class="btn btn-mini" @click="detail.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <template v-if="detail.wo">
          <div class="detail-grid">
            <div><span class="dl">标题</span>{{ detail.wo.title }}</div>
            <div><span class="dl">类型</span>{{ TYPES[detail.wo.type]||detail.wo.type }}</div>
            <div><span class="dl">优先级</span>{{ PRI[detail.wo.priority] }}</div>
            <div><span class="dl">状态</span><span class="badge" :class="'wost-'+detail.wo.status">{{ WOST[detail.wo.status] }}</span></div>
            <div><span class="dl">站点</span>{{ siteName(detail.wo.site_id) }}</div>
            <div><span class="dl">处理人</span>{{ userName(detail.wo.assignee_id) }}</div>
            <div v-if="detail.wo.asset_id"><span class="dl">关联资产</span><a class="link" @click="goAsset(detail.wo.asset_id)">{{ assetName(detail.wo.asset_id) }}</a></div>
            <div><span class="dl">受理时间</span>{{ fmt(detail.wo.accepted_at) }}</div>
            <div><span class="dl">完成时间</span>{{ fmt(detail.wo.finished_at) }}</div>
            <div><span class="dl">响应时限</span>{{ fmt(detail.wo.sla_respond_due) }} <span v-if="detail.wo.sla_respond_breach" class="badge sla-breach-tag">违约</span></div>
            <div><span class="dl">解决时限</span>{{ fmt(detail.wo.sla_resolve_due) }} <span v-if="detail.wo.sla_resolve_breach" class="badge sla-breach-tag">违约</span></div>
            <div class="full"><span class="dl">描述</span>{{ detail.wo.description || '-' }}</div>
            <div v-if="detail.wo.resolution" class="full"><span class="dl">处理结果</span>{{ detail.wo.resolution }}</div>
          </div>
          <template v-if="detail.wo.status>=3">
            <h4 class="sec-title"><SvgIcon name="book" />历史案例推荐 <button v-if="!detail.promoted" class="btn btn-mini btn-mini-primary" style="float:right" @click="promote(detail.wo)"><SvgIcon name="upload" />沉淀为案例</button><span v-else class="muted" style="float:right;font-size:12px">已沉淀 <SvgIcon name="check" /></span></h4>
            <div v-if="detail.cases.length" class="case-list">
              <div v-for="fc in detail.cases" :key="fc.id" class="case-item" @click="$router.push('/fault-cases')">
                <b>{{ fc.code }}</b> {{ fc.title }}<span class="muted">（匹配 {{ fc.score }}）</span>
                <div class="case-sol" v-if="fc.solution">方案: {{ String(fc.solution).slice(0,60) }}</div>
              </div>
            </div>
            <div v-else class="muted" style="font-size:12.5px">暂无匹配案例——完成工单后可沉淀为案例供下次参考</div>
          </template>
          <h4 class="sec-title"><SvgIcon name="log" />流转日志</h4>
          <div class="logs">
            <div v-for="l in detail.logs" :key="l.id" class="log-item">
              <span class="log-time">{{ fmt(l.created_at) }}</span>
              <span class="log-act">{{ LOGACT[l.action]||l.action }}</span>
              <span class="log-user">{{ l.username || (l.user_id ? '用户#'+l.user_id : '系统') }}</span>
              <span class="log-content">{{ l.content }}</span>
            </div>
            <div v-if="!detail.logs.length" class="muted">暂无日志</div>
          </div>
        </template>
      </div>
      <div class="drawer-foot"><button class="btn" @click="detail.show=false">关闭</button></div>
    </div></div>

    <!-- 改派抽屉 -->
    <div v-if="reassign.show" class="drawer-mask" @click.self="reassign.show=false"><div class="drawer" style="width:420px">
      <div class="drawer-head"><h3>改派工单 · {{ reassign.wo?.code }}</h3><button class="btn btn-mini" @click="reassign.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <label class="full">改派给 *<select class="input" v-model.number="reassign.assignee_id"><option :value="0">请选择</option><option v-for="u in users" :key="u.id" :value="u.id">{{ u.display_name }}（{{ u.username }}）</option></select>
          <span class="duty-hint" v-if="duty.on_duty">当班：{{ duty.user_name }} · {{ duty.shift_label }}<button type="button" class="link" @click.prevent="reassign.assignee_id=duty.user_id">用当班人</button></span>
        </label>
        <label class="full" style="margin-top:12px">备注<input class="input" v-model="reassign.note" placeholder="改派原因（可选）" /></label>
        <p v-if="reassign.err" class="err">{{ reassign.err }}</p>
      </div>
      <div class="drawer-foot"><button class="btn" @click="reassign.show=false">取消</button><button class="btn btn-primary" @click="doReassign">确认改派</button></div>
    </div></div>

    <div v-if="form.show" class="drawer-mask" @click.self="form.show=false"><div class="drawer">
      <div class="drawer-head"><h3>{{ form.id?'编辑工单':'新建工单' }}</h3><button class="btn btn-mini" @click="form.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <fieldset class="fs"><legend><SvgIcon name="pin" />基本信息</legend>
          <div class="form-grid">
            <label class="full">标题 *<input class="input" v-model="form.title" /></label>
            <label>类型 *<select class="input" v-model="form.type"><option v-for="(n,k) in TYPES" :key="k" :value="k">{{ n }}</option></select></label>
            <label>优先级 *<select class="input" v-model.number="form.priority"><option v-for="(n,k) in PRI" :key="k" :value="k">{{ n }}</option></select></label>
            <label>所属站点 *<select class="input" v-model.number="form.site_id" @change="onFormSiteChange"><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option></select></label>
            <label>处理人<select class="input" v-model.number="form.assignee_id"><option :value="0">（未指派）</option><option v-for="u in users" :key="u.id" :value="u.id">{{ u.display_name }}（{{ u.username }}）</option></select>
              <span class="duty-hint" v-if="!form.id && duty.on_duty">当班：{{ duty.user_name }} · {{ duty.shift_label }}<button type="button" class="link" @click.prevent="useDuty">用当班人</button></span>
              <span class="duty-hint" v-else-if="!form.id && !duty.loading">当前无当班人，请手动选择</span>
            </label>
            <label>关联资产<select class="input" v-model.number="form.asset_id"><option :value="0">（无）</option><option v-for="a in assets" :key="a.id" :value="a.id">{{ a.name }}（{{ a.code }}）</option></select></label>
            <label class="full">描述<textarea class="input" rows="3" v-model="form.description" style="width:100%;box-sizing:border-box"></textarea></label>
          </div>
        </fieldset>
        <p v-if="form.err" class="err">{{ form.err }}</p>
      </div>
      <div class="drawer-foot"><button class="btn" @click="form.show=false">取消</button><button class="btn btn-primary" @click="save">保存</button></div>
    </div></div>

    <!-- V18 SLA 时限配置抽屉 -->
    <div v-if="slaForm.show" class="drawer-mask" @click.self="slaForm.show=false"><div class="drawer" style="width:520px">
      <div class="drawer-head"><h3><SvgIcon name="clock" />SLA 时限配置</h3><button class="btn btn-mini" @click="slaForm.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <p class="muted" style="font-size:13px;margin:0 0 12px">按工单优先级设定「响应时限」（创建→受理）与「解决时限」（创建→完成）。剩余不足 20% 时预警，超时判违约并推通知/外发。</p>
        <table class="data-table sla-table">
          <thead><tr><th>优先级</th><th>响应时限（小时）</th><th>解决时限（小时）</th></tr></thead>
          <tbody>
            <tr v-for="(lab,i) in ['低','中','高','紧急']" :key="i">
              <td><span class="badge" :class="'pri-'+(i+1)">{{ lab }}</span></td>
              <td><input class="input sla-in" type="number" min="0.5" step="0.5" v-model.number="slaForm.respond[i]" /></td>
              <td><input class="input sla-in" type="number" min="1" step="1" v-model.number="slaForm.resolve[i]" /></td>
            </tr>
          </tbody>
        </table>
        <p v-if="slaForm.err" class="err">{{ slaForm.err }}</p>
      </div>
      <div class="drawer-foot"><button class="btn" @click="slaForm.show=false">取消</button><button class="btn btn-primary" @click="saveSla">保存</button></div>
    </div></div>
  </div>
</template>

<script setup>
import {ref,reactive,onMounted,onBeforeUnmount} from 'vue';import {useRoute,useRouter} from 'vue-router'
import { showConfirm, showPrompt, toastAuto } from '../ui-dialog'
import api from '../api';import Pagination from '../components/Pagination.vue'
const route=useRoute(),router=useRouter()
const TYPES={repair:'故障维修',alarm:'告警工单',inspection:'巡检异常',change:'变更工单',task:'任务',other:'其他'}
const PRI={1:'低',2:'中',3:'高',4:'紧急'}
// 与后端状态机一致: 1待受理 2处理中 3已完成 4已关闭 5已取消
const WOST={1:'待受理',2:'处理中',3:'已完成',4:'已关闭',5:'已取消'}
const LOGACT={create:'创建',accept:'受理',progress:'进展',finish:'完成',close:'关闭',cancel:'取消',reassign:'改派'}
const sites=ref([]),assets=ref([]),users=ref([]),list=ref([]),page=ref(1),total=ref(0),size=20
const f=reactive({site_id:0,status:'',priority:'',sla:'',keyword:''})
const form=reactive({show:false,id:0,title:'',type:'repair',priority:2,site_id:0,assignee_id:0,asset_id:0,description:'',err:''})
const duty=reactive({on_duty:false,user_id:0,user_name:'',shift_label:'',scope:'none',loading:false})
async function loadDuty(siteId){duty.loading=true;try{const r=await api.get('/work-orders/assignee-suggestion',{site_id:siteId});const d=r.data||{};Object.assign(duty,{on_duty:!!d.on_duty,user_id:d.user_id||0,user_name:d.user_name||'',shift_label:d.shift_label||'',scope:d.scope||'none'})}catch(e){Object.assign(duty,{on_duty:false,user_id:0,user_name:'',shift_label:'',scope:'none'})}finally{duty.loading=false}}
function useDuty(){if(duty.on_duty)form.assignee_id=duty.user_id}
async function onFormSiteChange(){const prev=duty.user_id;await loadDuty(form.site_id);if(duty.on_duty&&(!form.assignee_id||form.assignee_id===prev))form.assignee_id=duty.user_id}
const detail=reactive({show:false,wo:null,logs:[],cases:[],promoted:false})
const reassign=reactive({show:false,wo:null,assignee_id:0,note:'',err:''})
const fmt=t=>t?String(t).replace('T',' ').slice(0,16):'-'
const assetName=id=>{const a=assets.value.find(x=>x.id===id);return a?a.name+'（'+a.code+'）':'资产#'+id}
const siteName=id=>(sites.value.find(s=>s.id===id)||{}).name||'-'
const userName=id=>{if(!id)return '-';const u=users.value.find(x=>x.id===id);return u?u.display_name:'用户#'+id}
function goAsset(id){const a=assets.value.find(x=>x.id===id);router.push({path:'/assets',query:a?{keyword:a.code}:{}})}
async function load(p){page.value=p||1;const params={page:page.value,size};if(f.site_id)params.site_id=f.site_id;if(f.status)params.status=f.status;if(f.priority)params.priority=f.priority;if(f.sla)params.sla=f.sla;if(f.keyword)params.keyword=f.keyword;if(route.query.asset_id)params.asset_id=route.query.asset_id;const r=await api.get('/work-orders',params);list.value=r.data?.items||[];total.value=r.data?.total||0
  // 深链 ?id= 直接打开详情（V76: 不在当前页时按 id 兜底拉取）
  if(route.query.id)openWoById(Number(route.query.id))}
// V76 全局搜索深链：先查当前列表，未命中再按 id 单查（后端 /work-orders 支持 id 过滤）
async function openWoById(id){let w=list.value.find(x=>x.id===id);if(!w){const r=await api.get('/work-orders',{id,size:1});w=(r.data?.items||[])[0]}if(w)openDetail(w)}
async function act(w,action){
  if(action==='finish'){const res=await showPrompt({ title: '完成工单', label: '填写处理结果：', multiline: true });if(res===null)return;const r=await api.post(`/work-orders/${w.id}/finish`,{resolution:res});if(r.code!==0)return toastAuto(r.msg)}
  else if(action==='cancel'){if (!(await showConfirm({ message: `取消工单 ${w.code}？` })))return;const r=await api.post(`/work-orders/${w.id}/cancel`,{});if(r.code!==0)return toastAuto(r.msg)}
  else{const r=await api.post(`/work-orders/${w.id}/${action}`,{});if(r.code!==0)return toastAuto(r.msg)}
  load(page.value)
}
async function openReassign(w){Object.assign(reassign,{show:true,wo:w,assignee_id:w.assignee_id||0,note:'',err:''});await loadDuty(w.site_id);if(!reassign.assignee_id&&duty.on_duty)reassign.assignee_id=duty.user_id}
async function doReassign(){reassign.err='';if(!reassign.assignee_id){reassign.err='请选择改派目标';return}
  const r=await api.post(`/work-orders/${reassign.wo.id}/reassign`,{assignee_id:reassign.assignee_id,note:reassign.note})
  if(r.code===0){reassign.show=false;load(page.value)}else reassign.err=r.msg}
async function openDetail(w){detail.wo=w;detail.logs=[];detail.cases=[];detail.promoted=false;detail.show=true;const r=await api.get(`/work-orders-logs/${w.id}`);if(r.code===0)detail.logs=r.data?.items||r.data||[]
  // V22 案例推荐(完成/关闭的工单)
  if(w.status>=3){const rc=await api.get('/fault-cases/recommend',{text:(w.title||'')+' '+(w.description||'')+' '+(w.resolution||''),limit:4});detail.cases=(rc.data?.items||[]).slice(0,4)
    detail.promoted=detail.cases.some(x=>x.source_wo_id===w.id)}
  // 已有沉淀?
  if(!detail.promoted){const rp=await api.get('/fault-cases',{keyword:w.code,size:5});detail.promoted=(rp.data?.items||[]).some(x=>x.source_wo_id===w.id)}}
async function promote(w){const r=await api.post(`/fault-cases/from-work-order/${w.id}`,{});if(r.code===0||r._status===201){toastAuto('已沉淀为案例 '+r.data.code+'，请到故障案例库补充「根本原因」');detail.promoted=true}else toastAuto(r.msg||'沉淀失败')}
// ===== V18 SLA =====
const slaCfg=ref(null),slaForm=reactive({show:false,respond:[],resolve:[],err:''})
async function loadSlaCfg(){if(!slaCfg.value){const r=await api.get('/sla-config');if(r.code===0)slaCfg.value=r.data}}
function slaDeadline(w){ // 取当前仍生效的最近时限
  const now=Date.now()
  if(w.status===1&&w.sla_respond_due)return new Date(w.sla_respond_due).getTime()
  if(w.sla_resolve_due)return new Date(w.sla_resolve_due).getTime()
  return 0}
function slaText(w){
  if(w.sla_respond_breach)return '响应违约'
  if(w.sla_resolve_breach)return '解决违约'
  const d=slaDeadline(w);if(!d)return '-'
  const ms=d-Date.now()
  if(ms<=0)return '已超时'
  const h=Math.floor(ms/3600000),m=Math.floor(ms%3600000/60000)
  return '剩 '+(h>=24?Math.floor(h/24)+'天'+(h%24)+'时':h>0?h+'时'+m+'分':m+'分')}
function slaCls(w){
  if(w.sla_respond_breach||w.sla_resolve_breach)return 'sla-breach'
  const d=slaDeadline(w);if(!d)return ''
  const total=(d-Date.now())
  if(total<=0)return 'sla-breach'
  if(w.sla_respond_warned||w.sla_resolve_warned)return 'sla-warn'
  return 'sla-ok'}
function openSla(){loadSlaCfg().then(()=>{if(slaCfg.value){Object.assign(slaForm,{show:true,respond:[...slaCfg.value.respond_hours],resolve:[...slaCfg.value.resolve_hours],err:''})}})}
// ===== V63 SLA 考核 =====
const mainTab=ref('list')
const sla=reactive({days:30,site_id:0,data:null})
async function loadSla(){const r=await api.get('/work-orders/sla-analysis',{days:sla.days,site_id:sla.site_id||''});if(r.code===0)sla.data=r.data;else toastAuto(r.msg||'加载失败')}
function switchSla(){mainTab.value='sla';if(!sla.data)loadSla()}
function rateCls(v){return v>=95?'good':v>=80?'mid':'bad'}
function barH(v){if(!sla.data)return 0;const mx=Math.max(1,...sla.data.trend.map(t=>t.created));return Math.max(v>0?6:0,Math.round(v/mx*100))}
function drillBreach(){mainTab.value='list';f.sla='breach';f.status='';f.priority='';load(1)}
function exportSlaCsv(){if(!sla.data)return
  const d=sla.data;const L=[]
  L.push('SLA 考核报告,近'+d.days+'天,'+(sla.site_id?siteName(sla.site_id):'全部站点'))
  L.push('')
  L.push('指标,数值')
  L.push('工单总数,'+d.kpi.total);L.push('已完成,'+d.kpi.finished)
  L.push('响应达标率(%),'+d.kpi.respond_rate);L.push('解决达标率(%),'+d.kpi.resolve_rate)
  L.push('响应违约,'+d.kpi.respond_breach);L.push('解决违约,'+d.kpi.resolve_breach);L.push('违约工单合计,'+d.kpi.breach_total)
  L.push('进行中已超时,'+d.kpi.open_overdue);L.push('平均响应(h),'+d.kpi.avg_respond_h);L.push('平均解决(h),'+d.kpi.avg_resolve_h)
  L.push('');L.push('处理人考核')
  L.push('处理人,指派,完成,进行中,违约,违约率(%),平均解决(h)')
  for(const u of d.by_user)L.push([u.display_name||u.username,u.assigned,u.finished,u.open,u.breach,u.breach_rate,u.avg_resolve_h].join(','))
  L.push('');L.push('按优先级')
  L.push('优先级,工单,响应违约,解决违约,响应达标率(%)')
  for(const p of d.by_priority)L.push([p.label,p.total,p.resp_breach,p.resolve_breach,p.respond_rate].join(','))
  const blob=new Blob(['\uFEFF'+L.join('\n')],{type:'text/csv;charset=utf-8'})
  const a=document.createElement('a');a.href=URL.createObjectURL(blob);a.download='dcim_sla_'+d.days+'d.csv';a.click();URL.revokeObjectURL(a.href)
  toastAuto('已导出考核 CSV')}
async function saveSla(){
  slaForm.err=''
  const r=await api.put('/sla-config',{respond_hours:slaForm.respond.map(Number),resolve_hours:slaForm.resolve.map(Number)})
  if(r.code===0){slaForm.show=false;slaCfg.value=null;load(1)}else slaForm.err=r.msg}
async function openForm(w){if(w){Object.assign(form,{show:true,id:w.id,title:w.title,type:w.type,priority:w.priority,site_id:w.site_id,assignee_id:w.assignee_id||0,asset_id:w.asset_id||0,description:w.description||'',err:''})}else{Object.assign(form,{show:true,id:0,title:'',type:'repair',priority:2,site_id:sites.value[0]?.id||0,assignee_id:0,asset_id:Number(route.query.asset_id)||0,description:'',err:''});await loadDuty(form.site_id);if(duty.on_duty)form.assignee_id=duty.user_id}}
async function save(){form.err='';if(!form.title||!form.site_id){form.err='标题、站点必填';return};const r=form.id?await api.put(`/work-orders/${form.id}`,{...form}):await api.post('/work-orders',{...form});if(r.code===0){form.show=false;load()}else form.err=r.msg}
let slaTimer=null
onBeforeUnmount(()=>{if(slaTimer)clearInterval(slaTimer)})
// V63 深链 ?tab=sla（双路径：onMounted + watch route.query）
import {watch} from 'vue'
function applyDeepTab(){if(route.query.tab==='sla'){mainTab.value='sla';if(route.query.days)sla.days=Number(route.query.days)||30;if(!sla.data)loadSla();else loadSla()}}
watch(()=>route.query,q=>{if(q.tab==='sla')applyDeepTab();if(q.id&&route.path==='/work-orders')openWoById(Number(q.id))})
onMounted(async()=>{slaTimer=setInterval(()=>{ /* 触发倒计时刷新 */ list.value=[...list.value] },60000)
const[ss,aa,uu]=await Promise.all([api.get('/sites',{size:100}),api.get('/assets',{size:200}),api.get('/users-options')]);sites.value=ss.data?.items||[];assets.value=aa.data?.items||[];users.value=uu.data?.items||[];form.site_id=sites.value[0]?.id||0;
  // 深链：/work-orders?status=1&priority=3
  if(route.query.status)f.status=route.query.status
  if(route.query.priority)f.priority=route.query.priority
  load()
  if(route.query.tab==='sla')applyDeepTab()})
</script>
<style scoped>
.duty-hint{display:flex;align-items:center;gap:8px;font-size:12px;color:var(--muted);margin-top:2px}
.duty-hint .link{font-size:12px;padding:0;background:none;border:none;color:var(--primary);cursor:pointer}
.badge.pri-4{color:#f56c6c;border-color:rgba(245,108,108,.4)}.badge.pri-3{color:#e6a23c;border-color:rgba(230,162,60,.4)}.badge.pri-2{color:#409eff;border-color:rgba(64,158,255,.4)}.badge.pri-1{color:#909399;border-color:rgba(144,147,153,.4)}
.badge.wost-1{color:#409eff;border-color:rgba(64,158,255,.4)}.badge.wost-2{color:#e6a23c;border-color:rgba(230,162,60,.4)}.badge.wost-3{color:#67c23a;border-color:rgba(103,194,58,.4)}.badge.wost-4{color:#16a34a;border-color:rgba(34,197,94,.4)}.badge.wost-5{color:#909399;border-color:rgba(144,147,153,.4)}
.link{color:var(--accent);cursor:pointer;font-weight:600}.link:hover{text-decoration:underline}
.btn-mini-primary{color:var(--accent);border-color:var(--accent)}
.detail-grid{display:grid;grid-template-columns:1fr 1fr;gap:10px 16px;font-size:13.5px}.detail-grid .full{grid-column:1/-1}.dl{display:inline-block;min-width:70px;color:var(--muted)}
.sec-title{margin:20px 0 10px;font-size:15px}
.logs{display:flex;flex-direction:column;gap:8px}.log-item{display:flex;gap:10px;font-size:12.5px;padding:8px 10px;background:var(--bg2);border-radius:8px;flex-wrap:wrap}.log-time{color:var(--muted)}.log-act{font-weight:700;color:var(--accent)}.log-user{color:var(--text2)}.log-content{flex-basis:100%;color:var(--text2)}
.drawer-mask{position:fixed;inset:0;background:rgba(0,0,0,.35);z-index:1000;display:flex;justify-content:flex-end}.drawer{width:600px;max-width:95vw;height:100%;background:var(--bg);box-shadow:-4px 0 20px rgba(0,0,0,.15);display:flex;flex-direction:column;overflow:hidden}.drawer-head{display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--border);flex-shrink:0}.drawer-head h3{margin:0;font-size:18px}.drawer-body{flex:1;overflow-y:auto;padding:20px}.drawer-foot{padding:14px 20px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;gap:10px;flex-shrink:0}
.fs{border:1px solid var(--border);border-radius:8px;padding:16px;margin-bottom:16px}.fs legend{font-size:14px;padding:0 8px;color:var(--accent)}
.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.form-grid label{display:flex;flex-direction:column;gap:4px;font-size:13px;color:var(--muted)}.form-grid label.full{grid-column:1/-1}
.err{color:#f56c6c;font-size:13px}
.sla-badge{display:inline-block;font-size:11.5px;padding:2px 8px;border-radius:10px;border:1px solid;white-space:nowrap}
.sla-ok{color:#67c23a;border-color:rgba(103,194,58,.4)}
.sla-warn{color:#e6a23c;border-color:rgba(230,162,60,.45);background:rgba(230,162,60,.07)}
.sla-breach{color:#f56c6c;border-color:rgba(245,108,108,.45);background:rgba(245,108,108,.08);font-weight:600}
.sla-done{font-size:12px;color:#67c23a}
.sla-breach-tag{color:#f56c6c;border-color:rgba(245,108,108,.45)}
.sla-table{width:100%}.sla-in{width:110px}
.case-list{display:flex;flex-direction:column;gap:6px}
.case-item{padding:8px 10px;background:var(--bg2);border-radius:8px;font-size:13px;cursor:pointer;border:1px solid transparent}
.case-item:hover{border-color:var(--accent)}
.case-sol{font-size:12px;color:var(--muted);margin-top:3px}
/* ===== V63 SLA 考核 ===== */
.tabs{display:flex;gap:8px;margin-bottom:14px}.tab{padding:7px 18px;border:1px solid var(--border);border-radius:8px;background:var(--card);cursor:pointer;font-size:13.5px;display:inline-flex;align-items:center;gap:6px;color:var(--text)}.tab.on{color:var(--accent);border-color:var(--accent);font-weight:600}
.sla-kpis{display:grid;grid-template-columns:repeat(6,1fr);gap:12px;margin-bottom:14px}
.sla-kpi{background:var(--card);border:1px solid var(--border);border-radius:10px;padding:14px 16px;display:flex;flex-direction:column;gap:4px}
.sla-kpi .sk-label{font-size:12.5px;color:var(--muted)}
.sla-kpi .sk-val{font-size:26px;font-weight:700;line-height:1.1}
.sla-kpi .sk-sub{font-size:11.5px;color:var(--muted)}
.sla-kpi.good .sk-val{color:#67c23a}.sla-kpi.mid .sk-val{color:#e6a23c}.sla-kpi.bad .sk-val{color:#f56c6c}
.sla-kpi.clickable{cursor:pointer}.sla-kpi.clickable:hover{border-color:var(--accent)}
.sla-cols{display:grid;grid-template-columns:1.5fr 1fr;gap:12px;margin-bottom:14px}
.sla-side{display:flex;flex-direction:column;gap:12px}
.row-breach td:first-child{font-weight:600}
.td-breach{color:#f56c6c;font-weight:700}
.rate-bar{position:relative;height:14px;background:var(--bg2);border-radius:7px;overflow:hidden;min-width:110px}
.rate-fill{position:absolute;left:0;top:0;bottom:0;border-radius:7px;opacity:.75}
.rate-fill.good{background:#67c23a}.rate-fill.mid{background:#e6a23c}.rate-fill.bad{background:#f56c6c}
.rate-txt{position:relative;z-index:1;font-size:11px;line-height:14px;padding-left:6px}
.trend-bars{display:flex;gap:2px;align-items:flex-end;height:90px;padding:4px 2px 0}
.tb{flex:1;height:100%;display:flex;align-items:flex-end}
.tb-col{width:100%;height:100%;position:relative;display:flex;align-items:flex-end;justify-content:center}
.tb-fill{width:70%;background:var(--accent);opacity:.55;border-radius:2px 2px 0 0}
.tb-breach{position:absolute;bottom:0;width:70%;background:#f56c6c;border-radius:2px 2px 0 0}
@media(max-width:1100px){.sla-kpis{grid-template-columns:repeat(3,1fr)}.sla-cols{grid-template-columns:1fr}}
@media(max-width:900px){.sla-kpis{grid-template-columns:repeat(2,1fr)}}
</style>
