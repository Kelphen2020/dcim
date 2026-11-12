<template>
  <div><PageBar label="维保记录" parent-label="资产台账" parent-to="/assets" :back="fromLink" />
    <div class="view-tabs">
      <button class="vt" :class="{on:tab==='list'}" @click="tab='list'">记录列表</button>
      <button class="vt" :class="{on:tab==='coverage'}" @click="switchCoverage">覆盖分析</button>
    </div>
    <div v-if="assetFilter" class="deep-link-bar">已筛选资产「{{ assetName(assetFilter) }}」的记录 <a class="link" @click="clearAssetFilter">清除筛选</a></div>
    <div v-if="expiring.length&&tab==='list'" class="expire-banner">
      <span class="eb-title"><SvgIcon name="clock" />{{ expiring.length }} 份维保合同将在 30 天内到期：</span>
      <span v-for="e in expiring" :key="e.maintenance.id" class="eb-item" @click="openForm(e.maintenance)">
        {{ e.asset_name || e.asset_code || ('资产#'+e.maintenance.asset_id) }}
        <b :class="e.days_left<=7?'eb-red':'eb-yellow'">{{ e.days_left===0?'今日到期':'剩 '+e.days_left+' 天' }}</b>
      </span>
      <a class="link eb-hide" @click="expiring=[]">收起</a>
    </div>
    <!-- V17 覆盖分析视图 -->
    <template v-if="tab==='coverage'">
      <div class="toolbar">
        <select class="input" style="width:130px" v-model.number="cov.days" @change="loadCoverage"><option :value="30">30天内到期</option><option :value="60">60天内到期</option><option :value="90">90天内到期</option></select>
        <select class="input" style="width:130px" v-model="covFilter" @change="void 0"><option value="">全部资产</option><option value="uncovered">仅脱保</option><option value="expiring">仅即将到期</option><option value="covered">仅在保</option></select>
        <span style="flex:1"></span>
        <button class="btn" @click="exportCovCSV"><SvgIcon name="download" />导出CSV</button>
        <button class="btn" @click="loadCoverage">⟳ 刷新</button>
      </div>
      <div v-if="cov.d" class="cov-kpis">
        <div class="ck"><b>{{ cov.d.total }}</b><span>在管资产(非报废)</span></div>
        <div class="ck ck-green"><b>{{ cov.d.covered }}</b><span>在保</span></div>
        <div class="ck ck-yellow"><b>{{ cov.d.expiring }}</b><span>{{ cov.days }}天内到期</span></div>
        <div class="ck ck-red"><b>{{ cov.d.uncovered }}</b><span>脱保</span></div>
        <div class="ck"><b :class="cov.d.coverage_pct<60?'ck-red-t':''">{{ cov.d.coverage_pct.toFixed(1) }}%</b><span>覆盖率</span></div>
      </div>
      <div class="card" v-if="cov.d&&Object.keys(cov.d.category_stats||{}).length" style="margin-bottom:12px">
        <div class="table-wrap"><table class="data-table">
          <thead><tr><th>资产分类</th><th>在保</th><th>即将到期</th><th>脱保</th><th>覆盖率</th></tr></thead>
          <tbody><tr v-for="(st,cat) in cov.d.category_stats" :key="cat">
            <td>{{ CATN[cat]||cat }}</td><td>{{ st.covered||0 }}</td><td>{{ st.expiring||0 }}</td>
            <td :class="{'ck-red-t':(st.uncovered||0)>0}">{{ st.uncovered||0 }}</td>
            <td><div class="mini-bar"><i :style="{width: catPct(st)+'%', background: catPct(st)<60?'#f56c6c':'#67c23a'}"></i></div><span class="muted small">{{ catPct(st) }}%</span></td>
          </tr></tbody>
        </table></div>
      </div>
      <div class="card"><div class="table-wrap"><table class="data-table">
        <thead><tr><th>资产编码</th><th>名称</th><th>分类</th><th>站点</th><th>机柜</th><th>状态</th><th>维保到期</th><th>剩余天数</th><th>覆盖</th><th>操作</th></tr></thead>
        <tbody><tr v-for="it in covFiltered" :key="it.asset_id">
          <td class="mono"><a class="link" @click="goAssetId(it.asset_id)">{{ it.code }}</a></td>
          <td>{{ it.name }}</td><td class="muted">{{ CATN[it.category]||it.category }}</td>
          <td class="muted">{{ it.site_name||'-' }}</td><td class="mono">{{ it.cabinet_code||'-' }}</td>
          <td><span class="badge" :class="'ast-'+it.status">{{ AST[it.status]||it.status }}</span></td>
          <td class="muted">{{ it.end_at||'-' }}</td>
          <td :class="it.days_left>=0&&it.days_left<=7?'ck-red-t':it.days_left>=0&&it.days_left<=cov.days?'ck-yellow-t':'muted'">{{ it.days_left<0?'-':it.days_left+'天' }}</td>
          <td><span class="badge" :class="'cov-'+it.coverage">{{ COVN[it.coverage] }}</span></td>
          <td class="ops"><button class="btn btn-mini" @click="newContract(it)" :title="it.coverage==='uncovered'?'为脱保资产登记维保合同':'续签维保'">{{ it.coverage==='uncovered'?'登记维保':'续签' }}</button></td>
        </tr>
        <tr v-if="!covFiltered.length"><td colspan="10" class="empty">无匹配资产</td></tr></tbody>
      </table></div></div>
    </template>
    <template v-if="tab==='list'">
    <div class="toolbar"><select class="input" style="width:120px" v-model="f.kind" @change="load"><option value="">全部类型</option><option value="contract">维保合同</option><option value="repair">维修记录</option></select><select class="input" style="width:120px" v-model="f.status" @change="load"><option value="">全部状态</option><option v-for="(n,k) in MST" :key="k" :value="k">{{ n }}</option></select><span style="flex:1"></span><button class="btn btn-primary" @click="openForm()">＋ 新增记录</button></div>
    <div class="card"><div class="table-wrap"><table class="data-table">
      <thead><tr><th>资产</th><th>类型</th><th>厂商</th><th>合同编号</th><th>开始日期</th><th>到期日期</th><th>费用</th><th>状态</th><th>操作</th></tr></thead>
      <tbody><tr v-for="m in list" :key="m.id"><td><a class="link" @click="goAsset(m.asset_id)" title="查看资产台账">{{ assetName(m.asset_id) }}</a></td><td><span class="badge" :class="'kind-'+m.kind">{{ m.kind==='contract'?'维保合同':'维修' }}</span></td><td>{{ m.vendor }}</td><td class="mono">{{ m.contract }}</td><td class="muted">{{ fmt(m.start_at) }}</td><td class="muted">{{ fmt(m.end_at) }}</td><td>¥{{ m.cost }}</td><td><span class="badge" :class="'mst-'+m.status">{{ MST[m.status] }}</span></td><td class="ops"><button class="btn btn-mini" @click="openSpares(m)">备件清单</button><button class="btn btn-mini" @click="openForm(m)">编辑</button><button class="btn btn-mini btn-mini-danger" @click="del(m)">删除</button></td></tr>
      <tr v-if="!list.length"><td colspan="9" class="empty">暂无</td></tr></tbody>
    </table></div></div>
    </template>
    <div v-if="form.show" class="drawer-mask" @click.self="form.show=false"><div class="drawer"><div class="drawer-head"><h3>{{ form.id?'编辑':'新增' }}维保记录</h3><button class="btn btn-mini" @click="form.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body"><fieldset class="fs"><legend><SvgIcon name="pin" />基本信息</legend><div class="form-grid">
        <label>关联资产 *<select class="input" v-model.number="form.asset_id"><option :value="0">请选择</option><option v-for="a in assets" :key="a.id" :value="a.id">{{ a.name }}</option></select></label>
        <label>类型 *<select class="input" v-model="form.kind"><option value="contract">维保合同</option><option value="repair">维修记录</option></select></label>
        <label>厂商<input class="input" v-model="form.vendor" /></label><label>合同编号<input class="input" v-model="form.contract" /></label>
        <label>开始日期<input class="input" type="date" v-model="form.start_at" /></label><label>结束日期<input class="input" type="date" v-model="form.end_at" /></label>
        <label>费用(元)<input class="input" type="number" step="0.01" v-model.number="form.cost" /></label><label>状态<select class="input" v-model.number="form.status"><option v-for="(n,k) in MST" :key="k" :value="k">{{ n }}</option></select></label>
        <label class="full">详情<textarea class="input" rows="3" v-model="form.detail" style="width:100%;box-sizing:border-box"></textarea></label>
      </div></fieldset></div>
      <div class="drawer-foot"><button class="btn" @click="form.show=false">取消</button><button class="btn btn-primary" @click="save">保存</button></div>
    </div></div>

    <!-- V30 备件清单抽屉 -->
    <div v-if="sp.show" class="drawer-mask" @click.self="sp.show=false"><div class="drawer"><div class="drawer-head"><h3><SvgIcon name="toolbox" />维保备件清单 · {{ sp.m?.vendor || assetName(sp.m?.asset_id) }}</h3><button class="btn btn-mini" @click="sp.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <div class="muted" style="font-size:12.5px;margin-bottom:12px">配置该维保/维修常用备件，执行时可一键按清单领用出库（可关联工单，自动写流转日志）。</div>
        <div v-for="(it,i) in sp.items" :key="i" class="sp-line">
          <select class="input" style="flex:1" v-model.number="it.spare_id">
            <option :value="0">选择备件…</option>
            <option v-for="p in spareOpts" :key="p.id" :value="p.id">{{ p.name }}({{ p.code }}) 库存{{ p.stock }}</option>
          </select>
          <input class="input" type="number" min="1" style="width:70px" v-model.number="it.qty" title="数量" />
          <input class="input" style="width:110px" v-model="it.remark" placeholder="备注" />
          <button class="btn btn-mini btn-mini-danger" @click="sp.items.splice(i,1)"><SvgIcon name="x" /></button>
        </div>
        <button class="btn btn-mini" style="margin-top:8px" @click="sp.items.push({spare_id:0,qty:1,remark:''})">＋ 添加备件</button>
        <button class="btn btn-mini" style="margin-top:8px;margin-left:8px" @click="bringBom" :disabled="!sp.m?.asset_id"><SvgIcon name="puzzle" />按资产BOM带入</button>
        <fieldset class="fs" style="margin-top:18px"><legend><SvgIcon name="bolt" />一键领用</legend>
          <label style="font-size:12.5px">关联工单（可选）
            <select class="input" v-model.number="sp.woId" style="margin-top:4px">
              <option :value="0">不关联</option>
              <option v-for="w in sp.wos" :key="w.id" :value="w.id">{{ w.code }} · {{ w.title }}</option>
            </select>
          </label>
          <button class="btn btn-primary" style="margin-top:10px;width:100%" :disabled="sp.busy||!sp.items.length" @click="consume">按清单领用出库</button>
          <div v-if="sp.result" class="sp-result"><SvgIcon name="check" />领用 {{ sp.result.consumed }} 项<span v-if="sp.result.skipped">，{{ sp.result.skipped }} 项库存不足跳过：{{ sp.result.detail.filter(d=>!d.ok).map(d=>d.spare_name+'(需'+d.qty+'有'+d.stock+')').join('、') }}</span></div>
        </fieldset>
      </div>
      <div class="drawer-foot"><button class="btn" @click="sp.show=false">关闭</button><button class="btn btn-primary" :disabled="sp.busy" @click="saveSpares">保存清单</button></div>
    </div></div>
  </div>
</template>
<script setup>
import {ref,reactive,computed,onMounted} from 'vue';import {useRoute,useRouter} from 'vue-router'
import { toastAuto, showConfirm } from '../ui-dialog'
import api from '../api';import PageBar from '../components/PageBar.vue'
const route=useRoute(),router=useRouter()
const MST={1:'生效中',2:'已过期',3:'已完成',0:'已作废'};const assets=ref([]),list=ref([])
// V17 覆盖分析
const CATN={server:'服务器',network:'网络设备',storage:'存储',power:'动力设备',hvac:'空调制冷',security:'安防',other:'其他'}
const AST={1:'在架',2:'在库',3:'维修',4:'报废'}
const COVN={covered:'在保',expiring:'即将到期',uncovered:'脱保'}
const tab=ref('list'),covFilter=ref('')
const cov=reactive({days:30,d:null})
const covFiltered=computed(()=>!cov.d?[]:cov.d.items.filter(it=>!covFilter.value||it.coverage===covFilter.value))
const catPct=st=>{const t=(st.covered||0)+(st.expiring||0)+(st.uncovered||0);return t?Math.round(((st.covered||0)+(st.expiring||0))/t*100):0}
async function loadCoverage(){const r=await api.get('/maintenances/coverage',{days:cov.days});if(r.code===0)cov.d=r.data;else toastAuto(r.msg)}
function switchCoverage(){tab.value='coverage';if(!cov.d)loadCoverage()}
function goAssetId(id){const a=assets.value.find(x=>x.id===id);if(a)router.push({path:'/assets',query:{keyword:a.code}})}
function newContract(it){tab.value='list';Object.assign(form,{show:true,id:0,asset_id:it.asset_id,kind:'contract',vendor:'',contract:'',start_at:'',end_at:'',cost:0,detail:'',status:1})}
function exportCovCSV(){
  if(!cov.d)return
  const head=['资产编码','名称','分类','站点','机柜','状态','维保到期','剩余天数','覆盖状态']
  const lines=[head.join(',')]
  for(const it of covFiltered.value)lines.push([it.code,it.name,CATN[it.category]||it.category,it.site_name||'',it.cabinet_code||'',AST[it.status]||it.status,it.end_at||'',it.days_left<0?'':it.days_left,COVN[it.coverage]].map(v=>'"'+String(v).replace(/"/g,'""')+'"').join(','))
  const blob=new Blob(['\uFEFF'+lines.join('\n')],{type:'text/csv'})
  const a=document.createElement('a');a.href=URL.createObjectURL(blob);a.download=`维保覆盖分析_${cov.days}天窗口.csv`;a.click();URL.revokeObjectURL(a.href)
}
const f=reactive({kind:'',status:''});const form=reactive({show:false,id:0,asset_id:0,kind:'contract',vendor:'',contract:'',start_at:'',end_at:'',cost:0,detail:'',status:1})
const expiring=ref([])
const assetFilter=ref(0)
const fromLink=computed(()=>!!assetFilter.value)
const assetName=id=>(assets.value.find(a=>a.id===id)||{}).name||'-';const fmt=t=>t?String(t).replace('T',' ').slice(0,10):'-'
function goAsset(id){const a=assets.value.find(x=>x.id===id);if(a)router.push({path:'/assets',query:{keyword:a.code}})}
function clearAssetFilter(){assetFilter.value=0;load()}
async function load(){const params={};if(f.kind)params.kind=f.kind;if(f.status)params.status=f.status;if(assetFilter.value)params.asset_id=assetFilter.value;const r=await api.get('/maintenances',params);list.value=r.data?.items||[]}
function openForm(m){if(m)Object.assign(form,{show:true,id:m.id,asset_id:m.asset_id,kind:m.kind||'contract',vendor:m.vendor||'',contract:m.contract||'',start_at:(m.start_at||'').slice(0,10),end_at:(m.end_at||'').slice(0,10),cost:m.cost||0,detail:m.detail||'',status:m.status||1});else Object.assign(form,{show:true,id:0,asset_id:assetFilter.value||0,kind:'contract',vendor:'',contract:'',start_at:'',end_at:'',cost:0,detail:'',status:1})}
async function save(){const r=form.id?await api.put(`/maintenances/${form.id}`,{...form}):await api.post('/maintenances',{...form});if(r.code===0){form.show=false;load()}else toastAuto(r.msg)}
async function del(m){if (!(await showConfirm({ message: '删除' })))return;await api.del(`/maintenances/${m.id}`);load()}
// V30 备件清单
const sp=reactive({show:false,m:null,items:[],woId:0,wos:[],busy:false,result:null})
const spareOpts=ref([])
async function openSpares(m){
  sp.m=m;sp.show=true;sp.result=null;sp.woId=0
  const [sr,lr,wr]=await Promise.all([
    api.get(`/maintenances/${m.id}/spares`),
    spareOpts.value.length?Promise.resolve({code:0,data:{items:spareOpts.value}}):api.get('/spare-parts',{size:200}),
    api.get('/work-orders',{size:50}),
  ])
  sp.items=(sr.code===0?(sr.data.list||[]):[]).map(x=>({spare_id:x.spare_id,qty:x.qty,remark:x.remark||''}))
  if(lr.code===0)spareOpts.value=lr.data.items||lr.data.list||[]
  sp.wos=(wr.code===0?(wr.data.items||wr.data.list||[]):[]).filter(w=>[1,2].includes(w.status))
}
async function bringBom(){
  if(!sp.m?.asset_id)return
  const r=await api.get(`/spare-boms/recommend/${sp.m.asset_id}`)
  if(r.code!==0){toastAuto(r.msg||'获取推荐失败');return}
  const list=r.data.list||[]
  if(!list.length){toastAuto('该资产分类暂无BOM关联');return}
  let added=0
  for(const rec of list){
    if(!sp.items.some(x=>x.spare_id===rec.spare_id)){sp.items.push({spare_id:rec.spare_id,qty:rec.qty,remark:'BOM推荐'});added++}
  }
  toastAuto(added?`带入 ${added} 项BOM推荐备件(记得保存清单)`:'推荐备件已在清单中')
}
async function saveSpares(){
  sp.busy=true
  const items=sp.items.filter(x=>x.spare_id>0&&x.qty>0)
  const r=await api.put(`/maintenances/${sp.m.id}/spares`,{items})
  sp.busy=false
  if(r.code===0)toastAuto(`清单已保存(${r.data.saved}项)`);else toastAuto(r.msg||'保存失败')
}
async function consume(){
  const items=sp.items.filter(x=>x.spare_id>0&&x.qty>0)
  if(!items.length){toastAuto('清单为空');return}
  if (!(await showConfirm({ message: `按清单领用 ${items.length} 项备件出库？` })))return
  sp.busy=true
  const r=await api.post(`/maintenances/${sp.m.id}/consume`,{work_order_id:sp.woId||0})
  sp.busy=false
  if(r.code===0){sp.result=r.data}else toastAuto(r.msg||'领用失败')
}
onMounted(async()=>{const r=await api.get('/assets',{size:200});assets.value=r.data?.items||[];
  // 深链：/maintenances?asset_id=3
  if(route.query.asset_id)assetFilter.value=Number(route.query.asset_id)||0
  if(route.query.kind)f.kind=route.query.kind
  if(route.query.status)f.status=route.query.status
  if(route.query.tab==='coverage'){tab.value='coverage';loadCoverage()}
  load()
  const er=await api.get('/maintenances/expiring');expiring.value=er.data?.items||[]})
</script>
<style scoped>
.badge.kind-contract{color:#67c23a;border-color:rgba(103,194,58,.4)}.badge.kind-repair{color:#e6a23c;border-color:rgba(230,162,60,.4)}
.sp-line{display:flex;gap:8px;align-items:center;margin-bottom:8px}
.sp-result{margin-top:10px;font-size:12.5px;color:#67c23a;background:rgba(103,194,58,.08);border-radius:6px;padding:8px 12px}
.badge.mst-1{color:#67c23a;border-color:rgba(103,194,58,.4)}.badge.mst-2{color:#f56c6c;border-color:rgba(245,108,108,.4)}.badge.mst-3{color:#409eff;border-color:rgba(64,158,255,.4)}.badge.mst-0{color:#909399;border-color:rgba(144,147,153,.4)}
.drawer-mask{position:fixed;inset:0;background:rgba(0,0,0,.35);z-index:1000;display:flex;justify-content:flex-end}.drawer{width:600px;max-width:95vw;height:100%;background:var(--bg);box-shadow:-4px 0 20px rgba(0,0,0,.15);display:flex;flex-direction:column;overflow:hidden}.drawer-head{display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--border);flex-shrink:0}.drawer-head h3{margin:0;font-size:18px}.drawer-body{flex:1;overflow-y:auto;padding:20px}.drawer-foot{padding:14px 20px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;gap:10px;flex-shrink:0}
.fs{border:1px solid var(--border);border-radius:8px;padding:16px;margin-bottom:16px}.fs legend{font-weight:600;font-size:14px;padding:0 8px;color:var(--accent)}.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.form-grid label{display:flex;flex-direction:column;gap:4px;font-size:13px;color:var(--muted)}.form-grid label.full{grid-column:1/-1}
.link{color:var(--accent);cursor:pointer;font-weight:600}.link:hover{text-decoration:underline}
.deep-link-bar{display:flex;align-items:center;gap:10px;padding:8px 14px;margin-bottom:12px;background:var(--bg2);border:1px solid var(--border);border-radius:8px;font-size:13px;color:var(--text2)}
.expire-banner{display:flex;align-items:center;flex-wrap:wrap;gap:6px;padding:10px 14px;margin-bottom:12px;background:rgba(230,162,60,.08);border:1px solid rgba(230,162,60,.4);border-radius:8px;font-size:13px}
.eb-title{font-weight:600;color:#e6a23c;margin-right:4px}
.eb-item{display:inline-flex;align-items:center;gap:4px;padding:2px 10px;background:var(--bg);border:1px solid var(--border);border-radius:14px;cursor:pointer;color:var(--text2,var(--muted))}
.eb-item:hover{border-color:#e6a23c}
.eb-item b.eb-red{color:#f56c6c}.eb-item b.eb-yellow{color:#e6a23c}
.eb-hide{margin-left:auto;font-size:12px}
.view-tabs{display:flex;gap:8px;margin-bottom:12px}.vt{padding:7px 18px;border:1px solid var(--border);border-radius:8px;background:var(--bg);cursor:pointer;font-size:13.5px;color:var(--muted)}.vt.on{color:var(--accent);border-color:var(--accent);font-weight:600;background:color-mix(in srgb, var(--accent) 8%, transparent)}
.cov-kpis{display:grid;grid-template-columns:repeat(5,1fr);gap:10px;margin-bottom:12px}.ck{border:1px solid var(--border);border-radius:8px;padding:12px;text-align:center;background:var(--bg)}.ck b{display:block;font-size:22px}.ck span{font-size:12px;color:var(--muted)}.ck-green b{color:#67c23a}.ck-yellow b{color:#e6a23c}.ck-red b{color:#f56c6c}
.ck-red-t{color:#f56c6c;font-weight:600}.ck-yellow-t{color:#e6a23c;font-weight:600}
.badge.cov-covered{color:#67c23a;border-color:rgba(103,194,58,.4)}.badge.cov-expiring{color:#e6a23c;border-color:rgba(230,162,60,.4)}.badge.cov-uncovered{color:#f56c6c;border-color:rgba(245,108,108,.4)}
.badge.ast-1{color:#67c23a;border-color:rgba(103,194,58,.4)}.badge.ast-2{color:#409eff;border-color:rgba(64,158,255,.4)}.badge.ast-3{color:#e6a23c;border-color:rgba(230,162,60,.4)}.badge.ast-4{color:#909399;border-color:rgba(144,147,153,.4)}
.mini-bar{display:inline-block;width:80px;height:6px;border-radius:3px;background:var(--border);vertical-align:middle;margin-right:6px}.mini-bar i{display:block;height:100%;border-radius:3px}
</style>