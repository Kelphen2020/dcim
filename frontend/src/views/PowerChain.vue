<template>
  <div><PageBar label="供电链路" />
    <div class="toolbar">
      <select class="input" style="width:150px" v-model.number="f.site_id" @change="load">
        <option :value="0">全部站点</option>
        <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
      </select>
      <select class="input" style="width:130px" v-model="f.view">
        <option value="tree">树形视图</option><option value="table">表格视图</option><option value="redundancy">冗余分析</option>
      </select>
      <span style="flex:1"></span>
      <span v-if="summary.total" class="muted small">节点 {{ summary.total }} · 总额定 {{ summary.ratedKVA }} kVA · 总负载 {{ summary.loadKW.toFixed(1) }} kW · <span :class="summary.warn?'warn-text':''">过载/告警 {{ summary.warn }}</span></span>
      <button class="btn btn-primary" @click="openForm()">＋ 添加节点</button>
    </div>

    <!-- 树形视图 -->
    <div v-if="f.view==='tree'" class="card tree-card">
      <div v-if="!list.length" class="empty" style="padding:40px;text-align:center;color:var(--muted)">暂无供电链路节点，点击「添加节点」从总进线开始建链</div>
      <div v-for="root in roots" :key="root.id" class="pnode-wrap">
        <PNode :node="root" :children-map="childrenMap" :depth="0"
               @edit="openForm" @del="del" @impact="openImpact" @bind="openForm" />
      </div>
    </div>

    <!-- 表格视图 -->
    <div v-else-if="f.view==='table'" class="card"><div class="table-wrap"><table class="data-table">
      <thead><tr><th>名称</th><th>类型</th><th>上级</th><th>额定</th><th>负载</th><th>负载率</th><th>负载来源</th><th>关联</th><th>状态</th><th>操作</th></tr></thead>
      <tbody><tr v-for="p in list" :key="p.id">
        <td>{{ p.name }}</td>
        <td><span class="badge">{{ KIND[p.kind]||p.kind }}</span></td>
        <td class="muted">{{ nodeName(p.parent_id)||'总进线' }}</td>
        <td>{{ p.rated_kva||'-' }} kVA</td>
        <td>{{ p.effective_kw.toFixed(1) }} kW</td>
        <td><div class="lbar"><div class="lbar-in" :class="pctClass(p.load_pct)" :style="{width:Math.min(100,p.load_pct)+'%'}"></div></div><span class="small">{{ p.load_pct.toFixed(0) }}%</span></td>
        <td><span class="badge" :class="p.load_source==='point'?'src-live':'src-manual'">{{ p.load_source==='point'?'实时':'手工' }}</span></td>
        <td class="small muted">{{ p.asset_name||p.cabinet_code||p.point_name||'-' }}</td>
        <td><span class="badge" :class="'st-'+p.status">{{ ST[p.status] }}</span><span v-if="p.active_alerts" class="alert-dot">●{{ p.active_alerts }}</span></td>
        <td class="ops"><button class="btn btn-mini" @click="openImpact(p)">影响分析</button><button class="btn btn-mini" @click="openForm(p)">编辑</button><button class="btn btn-mini btn-mini-danger" @click="del(p)">删除</button></td>
      </tr>
      <tr v-if="!list.length"><td colspan="10" class="empty">暂无</td></tr></tbody>
    </table></div></div>

    <!-- V54 冗余分析视图 -->
    <div v-else class="red-wrap">
      <div class="red-head">
        <div class="red-sum">
          <span class="rs-item"><b :class="{ok:!(red.summary.risk>0||red.summary.warn>0)}">{{ red.list.length }}</b>站点已评估</span>
          <span class="rs-item risk"><b>{{ red.summary.risk||0 }}</b>单点风险</span>
          <span class="rs-item warn"><b>{{ red.summary.warn||0 }}</b>冗余不足</span>
          <span class="rs-item okc"><b>{{ red.summary.ok||0 }}</b>冗余充足</span>
        </div>
        <div class="red-actions">
          <button v-if="auth.isAdmin" class="btn" @click="evalRed" :disabled="red.busy">{{ red.busy?'评估中…':'立即评估' }}</button>
          <button v-if="red.detail && red.sel" class="btn btn-primary" @click="redWO" :disabled="red.busy||!!(red.detail.open_work_order_id>0)">
            {{ red.detail.open_work_order_id>0?'整改工单处理中':'开整改工单' }}
          </button>
        </div>
      </div>
      <div v-if="!red.list.length" class="empty card" style="padding:40px;text-align:center;color:var(--muted)">暂无供电链路数据的站点不参与评估。先在树形视图建链，或点「立即评估」。</div>
      <div class="red-sites">
        <div v-for="r in red.list" :key="r.site_id" class="red-card" :class="['rg-'+r.grade,{sel:red.sel===r.site_id}]" @click="openRedSite(r.site_id)">
          <div class="rc-top"><span class="rc-name">{{ r.site_name }}</span><span class="badge hg" :class="'hg-'+r.grade">{{ GRADE[r.grade]||r.grade }}</span></div>
          <div class="rc-score" :class="'sc-'+r.grade">{{ r.score }}<span class="rc-max">/100</span></div>
          <div class="rc-meta">问题 {{ r.issue_count }}<template v-if="r.high_count"> · <span class="hi">高危 {{ r.high_count }}</span></template> · 节点 {{ r.node_count }} · 受电机柜 {{ r.cabinet_count }}</div>
        </div>
      </div>
      <div v-if="red.detail" class="card red-detail">
        <div class="rd-hero" :class="'rg-'+red.detail.grade">
          <div class="rdh-score">{{ red.detail.score }}</div>
          <div class="rdh-info">
            <h3>{{ red.detail.site.name }} · {{ GRADE[red.detail.grade] }}</h3>
            <p class="muted small">T{{ red.detail.site.level||'?' }} 级站点 · 供电节点 {{ red.detail.nodes.length }} · 受电机柜 {{ red.detail.cabinet_count }} · 发现 {{ red.detail.issues.length }} 项问题</p>
          </div>
        </div>
        <fieldset class="fs" v-if="red.detail.issues.length"><legend><SvgIcon name="alert" />问题清单（按扣分排序）</legend>
          <div v-for="(is,i) in sortedIssues" :key="i" class="red-issue" :class="'sev-'+is.severity">
            <div class="ri-head"><span class="badge sev" :class="'sev-'+is.severity">{{ SEV[is.severity] }}</span><b>{{ is.label }}</b><span class="muted">— {{ is.target }}</span><span class="ri-deduct">-{{ is.deduct }}分</span></div>
            <div class="ri-ev">{{ is.evidence }}</div>
            <div class="ri-ad"><SvgIcon name="bulb" />{{ is.advice }}</div>
          </div>
        </fieldset>
        <div v-else class="rd-allok"><SvgIcon name="check-circle" />未发现问题：双路电源、机柜供电路径与负载率均在安全范围。</div>
        <fieldset class="fs"><legend><SvgIcon name="bolt" />节点负载率</legend>
          <table class="data-table"><thead><tr><th>节点</th><th>类型</th><th>额定</th><th>负载</th><th>负载率</th><th>来源</th></tr></thead>
          <tbody><tr v-for="n in red.detail.nodes" :key="n.id">
            <td>{{ n.name }}</td><td><span class="badge">{{ KIND[n.kind]||n.kind }}</span></td>
            <td>{{ n.rated_kva>0?n.rated_kva+' kVA':'-' }}</td><td>{{ n.load_kw.toFixed(1) }} kW</td>
            <td><div class="lbar"><div class="lbar-in" :class="pctClass(n.load_pct)" :style="{width:Math.min(100,n.load_pct)+'%'}"></div></div><span class="small">{{ n.rated_kva>0?n.load_pct.toFixed(0)+'%':'-' }}</span></td>
            <td><span class="badge" :class="n.source==='point'?'src-live':'src-manual'">{{ n.source==='point'?'实时':'手工' }}</span></td>
          </tr></tbody></table>
        </fieldset>
      </div>
    </div>

    <!-- 节点表单抽屉 -->
    <div v-if="form.show" class="drawer-mask" @click.self="form.show=false"><div class="drawer"><div class="drawer-head"><h3>{{ form.id?'编辑':'添加' }}供电节点</h3><button class="btn btn-mini" @click="form.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <fieldset class="fs"><legend><SvgIcon name="pin" />基本信息</legend><div class="form-grid">
          <label>名称 *<input class="input" v-model="form.name" /></label>
          <label>站点 *<select class="input" v-model.number="form.site_id"><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option></select></label>
          <label>节点类型 *<select class="input" v-model="form.kind"><option v-for="(n,k) in KIND" :key="k" :value="k">{{ n }}</option></select></label>
          <label>上级节点<select class="input" v-model.number="form.parent_id"><option :value="0">（总进线/根节点）</option><option v-for="p in list.filter(x=>x.id!==form.id)" :key="p.id" :value="p.id">{{ p.name }}（{{ KIND[p.kind]||p.kind }}）</option></select></label>
          <label>额定容量(kVA)<input class="input" type="number" step="0.1" v-model.number="form.rated_kva" /></label>
          <label>状态<select class="input" v-model.number="form.status"><option v-for="(n,k) in ST" :key="k" :value="k">{{ n }}</option></select></label>
          <label class="full">型号<input class="input" v-model="form.model" /></label>
        </div></fieldset>
        <fieldset class="fs"><legend><SvgIcon name="link" />业务关联（负载来源与影响范围）</legend><div class="form-grid">
          <label>绑定测点（实时负载）<select class="input" v-model.number="form.point_id"><option :value="0">不绑定（手工填报）</option><option v-for="p in points" :key="p.id" :value="p.id">{{ p.name }}（{{ PT[p.type]||p.type }}{{ p.current_value!=null?(' · 当前 '+Number(p.current_value).toFixed(1)+(p.unit||'')):'' }}）</option></select></label>
          <label>关联资产<select class="input" v-model.number="form.asset_id"><option :value="0">不关联</option><option v-for="a in assets" :key="a.id" :value="a.id">{{ a.name }}（{{ a.code }}）</option></select></label>
          <label class="full">供电机柜（PDU/支路节点）<select class="input" v-model.number="form.cabinet_id"><option :value="0">不指定</option><option v-for="cb in cabinets" :key="cb.id" :value="cb.id">{{ cb.code }}（{{ roomName(cb.room_id) }}）</option></select></label>
          <p class="muted small full" style="margin:0">绑定测点后负载取实时读数（<SvgIcon name="bolt" />）；关联机柜后「影响分析」可统计该节点停电波及的设备清单。</p>
        </div></fieldset>
        <fieldset class="fs"><legend><SvgIcon name="bolt" />负载</legend><div class="form-grid">
          <label>手工负载(kW)<input class="input" type="number" step="0.1" v-model.number="form.load_kw" :disabled="form.point_id>0" /><span v-if="form.point_id>0" class="muted small">已绑定测点，负载取实时值</span></label>
          <label class="full">备注<input class="input" v-model="form.remark" /></label>
        </div></fieldset>
        <p v-if="form.err" class="err">{{ form.err }}</p>
      </div>
      <div class="drawer-foot"><button class="btn" @click="form.show=false">取消</button><button class="btn btn-primary" @click="save">保存</button></div>
    </div></div>

    <!-- 影响分析抽屉 -->
    <div v-if="impact.show" class="drawer-mask" @click.self="impact.show=false"><div class="drawer" style="width:680px">
      <div class="drawer-head"><h3><SvgIcon name="alert" />停电影响分析 — {{ impact.data.root?.name }}</h3><button class="btn btn-mini" @click="impact.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body" v-if="impact.data.root">
        <div class="impact-sum">
          <div class="istat"><b>{{ impact.data.node_count }}</b><span>供电节点</span></div>
          <div class="istat"><b>{{ impact.data.cabinet_count }}</b><span>波及机柜</span></div>
          <div class="istat red"><b>{{ impact.data.asset_count }}</b><span>波及设备</span></div>
          <div class="istat red"><b>{{ (impact.data.total_power_w/1000).toFixed(2) }} kW</b><span>停电损失负载</span></div>
        </div>
        <div v-if="impact.data.upstream.length" class="upstream">
          <span class="muted small">上游供电路径：</span>
          <span v-for="(u,i) in impact.data.upstream" :key="u.id" class="up-node">{{ u.name }}<span class="up-arrow">→</span></span>
          <span class="up-node cur">{{ impact.data.root.name }}（本节点）</span>
        </div>
        <div v-else class="muted small" style="margin-bottom:10px">本节点为总进线（无上游），停电即全站失电</div>
        <fieldset class="fs"><legend><SvgIcon name="rack" />波及机柜（{{ impact.data.cabinets.length }}）</legend>
          <table class="data-table"><thead><tr><th>机柜</th><th>机房</th><th>设备数</th><th>装机功率</th></tr></thead>
          <tbody><tr v-for="cb in impact.data.cabinets" :key="cb.id">
            <td><a class="link" @click="goCapacity(cb.id)">{{ cb.code }}</a></td>
            <td class="muted">{{ cb.room_id }}</td><td>{{ cb.asset_cnt }}</td><td>{{ cb.power_kw.toFixed(2) }} kW</td>
          </tr><tr v-if="!impact.data.cabinets.length"><td colspan="4" class="empty">未关联机柜（节点未绑定供电机柜/资产）</td></tr></tbody></table>
        </fieldset>
        <fieldset class="fs"><legend><SvgIcon name="monitor" />波及设备清单（{{ impact.data.assets.length }}）</legend>
          <table class="data-table"><thead><tr><th>编码</th><th>名称</th><th>类别</th><th>位置</th><th>功率</th><th>IP</th></tr></thead>
          <tbody><tr v-for="a in impact.data.assets" :key="a.id">
            <td class="mono small">{{ a.code }}</td><td>{{ a.name }}</td><td>{{ CAT[a.category]||a.category }}</td>
            <td class="small muted">U{{ a.start_u }}</td><td>{{ a.power_w }}W</td><td class="small mono">{{ a.ip||'-' }}</td>
          </tr><tr v-if="!impact.data.assets.length"><td colspan="6" class="empty">无在架设备</td></tr></tbody></table>
        </fieldset>
        <div class="impact-tip"><SvgIcon name="bulb" />建议：停电检修前导出本清单，逐一通知设备责任人；完成检修后可按此清单核对上电恢复情况。</div>
      </div>
    </div></div>
  </div>
</template>
<script setup>
import {ref,reactive,computed,onMounted,watch} from 'vue'
import { toastAuto, showConfirm } from '../ui-dialog'
import {useRouter,useRoute} from 'vue-router'
import api from '../api';import PageBar from '../components/PageBar.vue';import PNode from '../components/PNode.vue'
import {useAuthStore} from '../stores/auth'
const router=useRouter();const route=useRoute();const auth=useAuthStore()
const GRADE={ok:'冗余充足',warn:'冗余不足',risk:'单点风险'}
const SEV={high:'高危',medium:'中危',low:'低危'}
const KIND={utility:'市电',incoming:'总进线',transformer:'变压器',ups:'UPS',pdu:'PDU',busbar:'母线',branch:'支路',other:'其他'}
const ST={1:'正常',2:'告警',3:'故障',0:'停用'}
const PT={power_dist:'配电',ups:'UPS',hvac:'空调',temp_humidity:'温湿度',water_leak:'漏水',smoke:'烟感'}
const CAT={server:'服务器',network:'网络',storage:'存储',power:'动力',cooling:'制冷',security:'安防',other:'其他'}
const sites=ref([]),rooms=ref([]),list=ref([]),points=ref([]),assets=ref([]),cabinets=ref([])
const f=reactive({site_id:0,view:'tree'})
const form=reactive({show:false,id:0,name:'',site_id:0,kind:'ups',parent_id:0,rated_kva:0,load_kw:0,status:1,asset_id:0,point_id:0,cabinet_id:0,model:'',remark:'',err:''})
const impact=reactive({show:false,data:{}})
const red=reactive({list:[],summary:{},sel:0,detail:null,busy:false})
const sortedIssues=computed(()=>red.detail?[...red.detail.issues].sort((a,b)=>b.deduct-a.deduct):[])
async function loadRed(){const r=await api.get('/power/redundancy');if(r.code===0){red.list=r.data.items||[];red.summary=r.data.summary||{}}}
async function openRedSite(sid){red.sel=sid;const r=await api.get(`/power/redundancy/${sid}`);if(r.code===0)red.detail=r.data}
async function evalRed(){red.busy=true;try{const r=await api.post('/power/redundancy/evaluate',{});if(r.code===0){await loadRed();if(red.sel)await openRedSite(red.sel)}else toastAuto(r.msg)}finally{red.busy=false}}
async function redWO(){if(!red.sel)return;red.busy=true;try{const r=await api.post(`/power/redundancy/${red.sel}/work-order`,{});if(r.code===0){toastAuto('整改工单已创建，可在运维工单页跟踪');await loadRed();await openRedSite(red.sel)}else toastAuto(r.msg)}finally{red.busy=false}}
const roomName=id=>(rooms.value.find(r=>r.id===id)||{}).name||'-'
const nodeName=id=>(list.value.find(x=>x.id===id)||{}).name||''
const childrenMap=computed(()=>{const m={};list.value.forEach(n=>{(m[n.parent_id]=m[n.parent_id]||[]).push(n)});return m})
const roots=computed(()=>list.value.filter(n=>!n.parent_id))
const summary=computed(()=>{
  const s={total:list.value.length,ratedKVA:0,loadKW:0,warn:0}
  roots.value.forEach(r=>{s.ratedKVA+=r.rated_kva||0;s.loadKW+=r.effective_kw||0})
  list.value.forEach(n=>{if(n.load_pct>80||n.status>1||n.active_alerts>0)s.warn++})
  return s
})
function pctClass(p){return p>=90?'red':p>=70?'yellow':'green'}
async function load(){const params={};if(f.site_id)params.site_id=f.site_id;const r=await api.get('/power-chains',params);list.value=r.data||[]}
function openForm(p){
  if(p)Object.assign(form,{show:true,id:p.id,name:p.name,site_id:p.site_id,kind:p.kind||'ups',parent_id:p.parent_id||0,rated_kva:p.rated_kva||0,load_kw:p.load_kw||0,status:p.status??1,asset_id:p.asset_id||0,point_id:p.point_id||0,cabinet_id:p.cabinet_id||0,model:p.model||'',remark:p.remark||'',err:''})
  else Object.assign(form,{show:true,id:0,name:'',site_id:f.site_id||sites.value[0]?.id||0,kind:'ups',parent_id:0,rated_kva:0,load_kw:0,status:1,asset_id:0,point_id:0,cabinet_id:0,model:'',remark:'',err:''})
}
async function save(){
  const body={site_id:form.site_id,parent_id:form.parent_id,name:form.name,kind:form.kind,model:form.model,rated_kva:form.rated_kva,load_kw:form.load_kw,status:form.status,asset_id:form.asset_id,point_id:form.point_id,cabinet_id:form.cabinet_id,remark:form.remark}
  const r=form.id?await api.put(`/power-chains/${form.id}`,body):await api.post('/power-chains',body)
  if(r.code===0||r.code===201){form.show=false;load()}else form.err=r.msg
}
async function del(p){
  if (!(await showConfirm({ message: `删除节点「${p.name}」？其下级节点将变为根节点。` })))return
  const r=await api.del(`/power-chains/${p.id}`)
  if(r.code===0)load();else toastAuto(r.msg)
}
async function openImpact(p){
  const r=await api.get(`/power-chains/impact/${p.id}`)
  if(r.code===0){impact.data=r.data;impact.show=true}else toastAuto(r.msg)
}
function goCapacity(cid){impact.show=false;router.push({path:'/capacity',query:{cabinet_id:cid}})}
function applyQuery(q){
  if(q.tab==='redundancy'){f.view='redundancy';loadRed().then(()=>{if(q.site_id)openRedSite(Number(q.site_id))})}
}
onMounted(async()=>{
  const [sr,rr,pr,ar,cr]=await Promise.all([api.get('/sites',{size:100}),api.get('/rooms',{size:200}),api.get('/monitoring-points',{size:500}),api.get('/assets',{size:500}),api.get('/cabinets',{size:500})])
  sites.value=sr.data?.items||[];rooms.value=rr.data?.items||[];points.value=pr.data?.items||[];assets.value=ar.data?.items||[];cabinets.value=cr.data?.items||[]
  load()
  applyQuery(route.query)
})
watch(()=>route.query,q=>applyQuery(q))
</script>
<style scoped>
.badge.st-1{color:#67c23a;border-color:rgba(103,194,58,.4)}.badge.st-2{color:#e6a23c;border-color:rgba(230,162,60,.4)}.badge.st-3{color:#f56c6c;border-color:rgba(245,108,108,.4)}.badge.st-0{color:#909399;border-color:rgba(144,147,153,.4)}
.badge.src-live{color:#409eff;border-color:rgba(64,158,255,.4)}.badge.src-manual{color:#909399;border-color:rgba(144,147,153,.4)}
.alert-dot{color:#f56c6c;font-size:11px;margin-left:4px}
.warn-text{color:#f56c6c;font-weight:700}
.lbar{display:inline-block;width:70px;height:6px;background:var(--bg2);border-radius:3px;vertical-align:middle;margin-right:6px;overflow:hidden}.lbar-in{height:100%;border-radius:3px}.lbar-in.green{background:#67c23a}.lbar-in.yellow{background:#e6a23c}.lbar-in.red{background:#f56c6c}
.tree-card{padding:18px}
.impact-sum{display:flex;gap:12px;margin-bottom:14px}.istat{flex:1;background:var(--bg2);border:1px solid var(--border);border-radius:8px;padding:12px;text-align:center}.istat b{font-size:22px;display:block}.istat span{font-size:12px;color:var(--muted)}.istat.red b{color:#f56c6c}
.upstream{background:var(--bg2);border:1px solid var(--border);border-radius:8px;padding:10px 14px;margin-bottom:14px;font-size:13px}.up-node{display:inline-block;padding:2px 10px;background:var(--card);border:1px solid var(--border);border-radius:6px;margin:2px 0}.up-node.cur{border-color:var(--accent);color:var(--accent);font-weight:600}.up-arrow{margin:0 6px;color:var(--muted)}
.impact-tip{background:rgba(64,158,255,.08);border:1px solid rgba(64,158,255,.3);border-radius:8px;padding:10px 14px;font-size:13px;color:var(--text2,var(--muted))}
.link{color:var(--accent);cursor:pointer;font-weight:600}.link:hover{text-decoration:underline}
.drawer-mask{position:fixed;inset:0;background:rgba(0,0,0,.35);z-index:1000;display:flex;justify-content:flex-end}.drawer{width:600px;max-width:95vw;height:100%;background:var(--bg);box-shadow:-4px 0 20px rgba(0,0,0,.15);display:flex;flex-direction:column;overflow:hidden}.drawer-head{display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--border);flex-shrink:0}.drawer-head h3{margin:0;font-size:18px}.drawer-body{flex:1;overflow-y:auto;padding:20px}.drawer-foot{padding:14px 20px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;gap:10px;flex-shrink:0}
.fs{border:1px solid var(--border);border-radius:8px;padding:16px;margin-bottom:16px}.fs legend{font-weight:600;font-size:14px;padding:0 8px;color:var(--accent)}.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.form-grid label{display:flex;flex-direction:column;gap:4px;font-size:13px;color:var(--muted)}.form-grid label.full{grid-column:1/-1}
.err{color:#f56c6c;font-size:13px}
.red-wrap{display:flex;flex-direction:column;gap:14px}
.red-head{display:flex;align-items:center;justify-content:space-between;gap:10px;flex-wrap:wrap}
.red-sum{display:flex;gap:16px;font-size:13px;color:var(--muted)}.rs-item b{font-size:18px;margin-right:4px;color:var(--text)}.rs-item.risk b{color:#f56c6c}.rs-item.warn b{color:#e6a23c}.rs-item.okc b{color:#67c23a}.rs-item b.ok{color:#67c23a}
.red-actions{display:flex;gap:8px}
.red-sites{display:grid;grid-template-columns:repeat(auto-fill,minmax(230px,1fr));gap:12px}
.red-card{background:var(--card);border:1px solid var(--border);border-left:4px solid var(--border);border-radius:10px;padding:14px 16px;cursor:pointer;transition:transform .15s,box-shadow .15s}
.red-card:hover{transform:translateY(-2px);box-shadow:0 4px 14px rgba(0,0,0,.08)}
.red-card.sel{border-color:var(--accent);box-shadow:0 0 0 2px rgba(64,158,255,.15)}
.red-card.rg-ok{border-left-color:#67c23a}.red-card.rg-warn{border-left-color:#e6a23c}.red-card.rg-risk{border-left-color:#f56c6c}
.rc-top{display:flex;justify-content:space-between;align-items:center;margin-bottom:6px}.rc-name{font-weight:600;font-size:14px}
.rc-score{font-size:34px;font-weight:800;line-height:1.1}.rc-score.sc-ok{color:#67c23a}.rc-score.sc-warn{color:#e6a23c}.rc-score.sc-risk{color:#f56c6c}.rc-max{font-size:13px;color:var(--muted);font-weight:400;margin-left:2px}
.rc-meta{font-size:12px;color:var(--muted);margin-top:4px}.rc-meta .hi{color:#f56c6c;font-weight:700}
.badge.hg{font-weight:700}.badge.hg-ok{color:#67c23a;border-color:rgba(103,194,58,.45)}.badge.hg-warn{color:#e6a23c;border-color:rgba(230,162,60,.45)}.badge.hg-risk{color:#f56c6c;border-color:rgba(245,108,108,.45)}
.red-detail{padding:18px}
.rd-hero{display:flex;align-items:center;gap:18px;border-radius:10px;padding:16px 20px;margin-bottom:16px;background:var(--bg2);border:1px solid var(--border)}
.rd-hero.rg-risk{border-color:rgba(245,108,108,.5);background:rgba(245,108,108,.06)}
.rd-hero.rg-warn{border-color:rgba(230,162,60,.5);background:rgba(230,162,60,.06)}
.rdh-score{font-size:46px;font-weight:800;line-height:1}
.rd-hero.rg-ok .rdh-score{color:#67c23a}.rd-hero.rg-warn .rdh-score{color:#e6a23c}.rd-hero.rg-risk .rdh-score{color:#f56c6c}
.rdh-info h3{margin:0 0 4px;font-size:17px}.rdh-info p{margin:0}
.red-issue{border:1px solid var(--border);border-left:4px solid var(--border);border-radius:8px;padding:10px 14px;margin-bottom:10px}
.red-issue.sev-high{border-left-color:#f56c6c}.red-issue.sev-medium{border-left-color:#e6a23c}.red-issue.sev-low{border-left-color:#409eff}
.ri-head{display:flex;align-items:center;gap:8px;font-size:14px}.ri-deduct{margin-left:auto;color:#f56c6c;font-weight:700;font-size:13px}
.badge.sev{font-weight:700}.badge.sev-high{color:#f56c6c;border-color:rgba(245,108,108,.45)}.badge.sev-medium{color:#e6a23c;border-color:rgba(230,162,60,.45)}.badge.sev-low{color:#409eff;border-color:rgba(64,158,255,.45)}
.ri-ev{font-size:13px;margin:6px 0 4px;color:var(--text)}
.ri-ad{font-size:12.5px;color:var(--muted);display:flex;align-items:center;gap:5px}
.rd-allok{display:flex;align-items:center;gap:8px;background:rgba(103,194,58,.08);border:1px solid rgba(103,194,58,.35);border-radius:8px;padding:14px 16px;font-size:14px;color:#4d9e2f;margin-bottom:16px}
@media (max-width:900px){.red-sites{grid-template-columns:1fr 1fr}.rd-hero{flex-direction:column;text-align:center}}
</style>
