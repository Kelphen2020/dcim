<template>
  <div><PageBar label="资产盘点" parent-label="资产台账" parent-to="/assets" />
    <div class="toolbar">
      <select class="input" style="width:130px" v-model="f.status" @change="load">
        <option value="">全部状态</option><option value="0">草稿</option><option value="1">盘点中</option><option value="2">已完成</option>
      </select>
      <span style="flex:1"></span>
      <button class="btn btn-primary" @click="openCreate">＋ 新建盘点任务</button>
    </div>
    <div class="card"><div class="table-wrap"><table class="data-table">
      <thead><tr><th>任务编号</th><th>名称</th><th>范围</th><th>账面数</th><th>进度</th><th>差异</th><th>状态</th><th>创建时间</th><th>操作</th></tr></thead>
      <tbody><tr v-for="t in list" :key="t.id">
        <td class="mono">{{ t.code }}</td>
        <td>{{ t.name }}</td>
        <td class="muted">{{ t.cabinet_id ? ('机柜 '+cabName(t.cabinet_id)) : '全站' }}</td>
        <td>{{ t.total_count }}</td>
        <td><div class="prog"><div class="prog-in" :style="{width: (t.total_count? Math.round(t.checked_count/t.total_count*100):0)+'%'}"></div></div><span class="muted small">{{ t.checked_count }}/{{ t.total_count }}</span></td>
        <td><span :class="t.diff_count>0?'diff-red':'muted'">{{ t.diff_count }}</span></td>
        <td><span class="badge" :class="'st-'+t.status">{{ STS[t.status] }}</span></td>
        <td class="muted">{{ fmt(t.created_at) }}</td>
        <td class="ops">
          <button class="btn btn-mini" @click="openDetail(t)">盘点</button>
          <button v-if="t.status===0" class="btn btn-mini" @click="del(t)">删除</button>
        </td>
      </tr>
      <tr v-if="!list.length"><td colspan="9" class="empty">暂无盘点任务</td></tr></tbody>
    </table></div></div>

    <!-- 新建任务抽屉 -->
    <div v-if="create.show" class="drawer-mask" @click.self="create.show=false"><div class="drawer">
      <div class="drawer-head"><h3>新建盘点任务</h3><button class="btn btn-mini" @click="create.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body"><fieldset class="fs"><legend><SvgIcon name="pin" />盘点范围</legend><div class="form-grid">
        <label>站点 *<select class="input" v-model.number="create.site_id"><option :value="0">请选择</option><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option></select></label>
        <label>机柜（留空=全站）<select class="input" v-model.number="create.cabinet_id"><option :value="0">全站盘点</option><option v-for="cb in cabinets" :key="cb.id" :value="cb.id">{{ cb.code }}</option></select></label>
        <label class="full">任务名称 *<input class="input" v-model="create.name" placeholder="如：2026年9月A机房季度盘点" /></label>
        <label class="full">备注<input class="input" v-model="create.remark" /></label>
      </div></fieldset>
      <p class="muted small">创建时将快照站点账面资产（不含已报废）生成盘点明细清单。</p>
      <p v-if="create.err" class="err">{{ create.err }}</p></div>
      <div class="drawer-foot"><button class="btn" @click="create.show=false">取消</button><button class="btn btn-primary" @click="doCreate">创建任务</button></div>
    </div></div>

    <!-- 盘点执行抽屉 -->
    <div v-if="detail.show" class="drawer-mask" @click.self="closeDetail"><div class="drawer" style="width:860px">
      <div class="drawer-head"><h3><SvgIcon name="clipboard" />{{ detail.task.name }} <span class="muted small">{{ detail.task.code }}</span></h3><button class="btn btn-mini" @click="closeDetail"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <div class="stat-row">
          <div class="stat"><b>{{ detail.task.total_count }}</b><span>账面数</span></div>
          <div class="stat"><b>{{ detail.task.checked_count }}</b><span>已核对</span></div>
          <div class="stat s-diff"><b>{{ detail.task.diff_count }}</b><span>差异项</span></div>
          <div class="stat"><span class="badge" :class="'st-'+detail.task.status">{{ STS[detail.task.status] }}</span><span>状态</span></div>
        </div>
        <div class="toolbar" style="margin-bottom:10px">
          <select class="input" style="width:130px" v-model="df.result" @change="loadDetail">
            <option value="">全部结果</option><option value="0">未核对</option><option value="1">相符</option><option value="2">盘亏</option><option value="3">盘盈</option><option value="4">位置不符</option>
          </select>
          <input class="input" style="width:180px" v-model="df.keyword" placeholder="搜索编码/名称" @keyup.enter="loadDetail" />
          <span style="flex:1"></span>
          <template v-if="detail.task.status===0">
            <button class="btn btn-primary" @click="startTask">▶ 开始盘点</button>
          </template>
          <template v-else-if="detail.task.status===1">
            <button class="btn" @click="openSurplus">＋ 登记盘盈</button>
            <button class="btn" @click="checkSelected(1)"><SvgIcon name="check" />选中相符</button>
            <button class="btn btn-primary" @click="applyTask(true)">差异调账并完成</button>
            <button class="btn" @click="applyTask(false)">仅完成(不调账)</button>
          </template>
        </div>
        <div class="table-wrap"><table class="data-table">
          <thead><tr><th v-if="detail.task.status===1"><input type="checkbox" @change="toggleAll" :checked="allChecked" /></th><th>资产编码</th><th>名称</th><th>账面位置</th><th>U位</th><th>账面状态</th><th>核对结果</th><th>核对人/时间</th></tr></thead>
          <tbody><tr v-for="it in detail.items" :key="it.id" :class="{'row-diff': it.result>1}">
            <td v-if="detail.task.status===1"><input type="checkbox" :value="it.id" v-model="selected" :disabled="it.result>0" /></td>
            <td class="mono">{{ it.asset_code || '(盘盈)' }}</td>
            <td>{{ it.asset_name }}</td>
            <td>{{ it.cabinet_code || '-' }}</td>
            <td>{{ it.start_u ? ('U'+it.start_u) : '-' }}</td>
            <td><span class="badge" :class="'ast-'+it.book_status">{{ AST[it.book_status] || '-' }}</span></td>
            <td>
              <template v-if="detail.task.status===1 && it.result===0">
                <select class="input input-mini" @change="setItem(it, $event)">
                  <option value="">核对…</option><option value="1">相符</option><option value="2">盘亏</option><option value="4">位置不符</option>
                </select>
              </template>
              <span v-else class="badge" :class="'rs-'+it.result">{{ RS[it.result] }}</span>
              <a v-if="it.result===2&&it.asset_id" class="link small" style="margin-left:6px" @click="toRetire(it)" title="账实不符，发起退役核销流程">转退役 ›</a>
              <span v-if="it.result===4" class="muted small"> → 实际 {{ cabName(it.actual_cabinet_id) }} U{{ it.actual_start_u }}</span>
              <span v-if="it.remark" class="muted small">（{{ it.remark }}）</span>
            </td>
            <td class="muted small">{{ it.checked_at ? fmt(it.checked_at) : '-' }}</td>
          </tr>
          <tr v-if="!detail.items.length"><td :colspan="detail.task.status===1?8:7" class="empty">暂无明细</td></tr></tbody>
        </table></div>
      </div>
    </div></div>

    <!-- 位置不符录入 -->
    <div v-if="reloc.show" class="drawer-mask" @click.self="reloc.show=false"><div class="drawer" style="width:480px">
      <div class="drawer-head"><h3>位置不符 — {{ reloc.item.asset_name }}</h3><button class="btn btn-mini" @click="reloc.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body"><div class="form-grid">
        <label>实际机柜<select class="input" v-model.number="reloc.cabinet_id"><option :value="0">请选择</option><option v-for="cb in cabinets" :key="cb.id" :value="cb.id">{{ cb.code }}</option></select></label>
        <label>实际U位<input class="input" type="number" min="0" v-model.number="reloc.start_u" /></label>
        <label class="full">备注<input class="input" v-model="reloc.remark" /></label>
      </div>
      </div>
      <div class="drawer-foot"><button class="btn" @click="reloc.show=false">取消</button><button class="btn btn-primary" @click="saveReloc">提交</button></div>
    </div></div>

    <!-- 盘盈登记 -->
    <div v-if="surplus.show" class="drawer-mask" @click.self="surplus.show=false"><div class="drawer" style="width:480px">
      <div class="drawer-head"><h3>登记盘盈设备（实物有、账面无）</h3><button class="btn btn-mini" @click="surplus.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body"><div class="form-grid">
        <label>设备编码（留空自动生成）<input class="input" v-model="surplus.asset_code" /></label>
        <label>设备名称 *<input class="input" v-model="surplus.asset_name" /></label>
        <label>所在机柜<select class="input" v-model.number="surplus.cabinet_id"><option :value="0">请选择</option><option v-for="cb in cabinets" :key="cb.id" :value="cb.id">{{ cb.code }}</option></select></label>
        <label>U位<input class="input" type="number" min="0" v-model.number="surplus.start_u" /></label>
        <label class="full">备注<input class="input" v-model="surplus.remark" /></label>
      </div>
      </div>
      <div class="drawer-foot"><button class="btn" @click="surplus.show=false">取消</button><button class="btn btn-primary" @click="saveSurplus">登记</button></div>
    </div></div>
  </div>
</template>
<script setup>
import {ref,reactive,computed,onMounted} from 'vue'
import {useRouter} from 'vue-router'
import SvgIcon from '../components/SvgIcon.vue'
import { toastAuto, showConfirm } from '../ui-dialog'
import api from '../api';import PageBar from '../components/PageBar.vue'
const STS={0:'草稿',1:'盘点中',2:'已完成'}
const router=useRouter()
// V60 盘亏一键转退役申请（reason=loss，来源留痕）
function toRetire(it){router.push(`/asset-retirement?new=1&asset_id=${it.asset_id}&reason=loss&source_type=stocktake&source_id=${it.id}`)}
const RS={0:'未核对',1:'相符',2:'盘亏',3:'盘盈',4:'位置不符'}
const AST={1:'在架',2:'在库',3:'维修',4:'报废'}
const list=ref([]),sites=ref([]),cabinets=ref([])
const f=reactive({status:''})
const create=reactive({show:false,site_id:0,cabinet_id:0,name:'',remark:'',err:''})
const detail=reactive({show:false,task:{},items:[]})
const df=reactive({result:'',keyword:''})
const selected=ref([])
const reloc=reactive({show:false,item:{},cabinet_id:0,start_u:0,remark:''})
const surplus=reactive({show:false,asset_code:'',asset_name:'',cabinet_id:0,start_u:0,remark:''})
const fmt=t=>t?String(t).replace('T',' ').slice(0,16):'-'
const cabName=id=>(cabinets.value.find(c=>c.id===id)||{}).code||'-'
const allChecked=computed(()=>{const un=detail.items.filter(i=>i.result===0);return un.length>0&&selected.value.length===un.length})
async function load(){const params={};if(f.status!=='')params.status=f.status;const r=await api.get('/stocktakes',params);list.value=r.data?.items||[]}
function openCreate(){Object.assign(create,{show:true,site_id:sites.value[0]?.id||0,cabinet_id:0,name:'',remark:'',err:''})}
async function doCreate(){const r=await api.post('/stocktakes',{site_id:create.site_id,cabinet_id:create.cabinet_id,name:create.name,remark:create.remark});if(r.code===0){create.show=false;load()}else create.err=r.msg}
async function del(t){if (!(await showConfirm({ message: '删除该草稿任务？' })))return;await api.del(`/stocktakes/${t.id}`);load()}
async function openDetail(t){detail.show=true;await refreshDetail(t.id)}
function closeDetail(){detail.show=false;load()}
async function refreshDetail(id){const params={};if(df.result!=='')params.result=df.result;if(df.keyword)params.keyword=df.keyword;params.size=500;const r=await api.get(`/stocktakes/${id||detail.task.id}`,params);detail.task=r.data?.task||{};detail.items=r.data?.items||[];selected.value=[]}
function loadDetail(){refreshDetail()}
async function startTask(){const r=await api.post(`/stocktakes/${detail.task.id}/start`);if(r.code===0)refreshDetail();else toastAuto(r.msg)}
function toggleAll(e){selected.value=e.target.checked?detail.items.filter(i=>i.result===0).map(i=>i.id):[]}
async function checkSelected(result){if(!selected.value.length){toastAuto('请先勾选明细');return}
  const r=await api.post(`/stocktakes/${detail.task.id}/check`,{items:selected.value.map(id=>({id,result}))});if(r.code===0)refreshDetail();else toastAuto(r.msg)}
async function setItem(it,ev){const v=Number(ev.target.value);if(!v)return
  if(v===4){Object.assign(reloc,{show:true,item:it,cabinet_id:it.cabinet_id||0,start_u:it.start_u||0,remark:''});return}
  const r=await api.post(`/stocktakes/${detail.task.id}/check`,{items:[{id:it.id,result:v}]});if(r.code===0)refreshDetail();else toastAuto(r.msg)}
async function saveReloc(){if(!reloc.cabinet_id){toastAuto('请选择实际机柜');return}
  const r=await api.post(`/stocktakes/${detail.task.id}/check`,{items:[{id:reloc.item.id,result:4,actual_cabinet_id:reloc.cabinet_id,actual_start_u:reloc.start_u,remark:reloc.remark}]});if(r.code===0){reloc.show=false;refreshDetail()}else toastAuto(r.msg)}
function openSurplus(){Object.assign(surplus,{show:true,asset_code:'',asset_name:'',cabinet_id:0,start_u:0,remark:''})}
async function saveSurplus(){if(!surplus.asset_name){toastAuto('请填写设备名称');return}
  const r=await api.post(`/stocktakes/${detail.task.id}/check`,{items:[{result:3,asset_code:surplus.asset_code,asset_name:surplus.asset_name,actual_cabinet_id:surplus.cabinet_id,actual_start_u:surplus.start_u,remark:surplus.remark}]});if(r.code===0){surplus.show=false;refreshDetail()}else toastAuto(r.msg)}
async function applyTask(adjust){const tip=adjust?'将按差异写回资产台账（盘亏下架待查/位置修正/盘盈入账）并完成任务，确认？':'不调账直接完成任务，确认？';if (!(await showConfirm({ message: tip })))return
  const r=await api.post(`/stocktakes/${detail.task.id}/apply`,{adjust});if(r.code===0)refreshDetail();else toastAuto(r.msg)}
onMounted(async()=>{
  const [sr,cr]=await Promise.all([api.get('/sites',{size:100}),api.get('/cabinets',{size:500})])
  sites.value=sr.data?.items||[];cabinets.value=cr.data?.items||[]
  load()
})
</script>
<style scoped>
.badge.st-0{color:#909399;border-color:rgba(144,147,153,.4)}.badge.st-1{color:#409eff;border-color:rgba(64,158,255,.4)}.badge.st-2{color:#67c23a;border-color:rgba(103,194,58,.4)}
.badge.rs-1{color:#67c23a;border-color:rgba(103,194,58,.4)}.badge.rs-2{color:#f56c6c;border-color:rgba(245,108,108,.4)}.badge.rs-3{color:#e6a23c;border-color:rgba(230,162,60,.4)}.badge.rs-4{color:#9254de;border-color:rgba(146,84,222,.4)}.badge.rs-0{color:#909399;border-color:rgba(144,147,153,.4)}
.badge.ast-1{color:#67c23a;border-color:rgba(103,194,58,.4)}.badge.ast-2{color:#909399;border-color:rgba(144,147,153,.4)}.badge.ast-3{color:#e6a23c;border-color:rgba(230,162,60,.4)}
.prog{width:80px;height:6px;background:var(--border);border-radius:3px;display:inline-block;vertical-align:middle;margin-right:6px}.prog-in{height:100%;background:var(--accent);border-radius:3px}
.diff-red{color:#f56c6c;font-weight:700}
.row-diff{background:rgba(245,108,108,.05)}
.stat-row{display:flex;gap:14px;margin-bottom:12px}.stat{flex:1;background:var(--bg2);border:1px solid var(--border);border-radius:8px;padding:10px 14px;display:flex;flex-direction:column;gap:2px;align-items:center}.stat b{font-size:20px}.stat span{font-size:12px;color:var(--muted)}.stat.s-diff b{color:#f56c6c}
.input-mini{width:90px;padding:2px 6px;font-size:12px}
.drawer-mask{position:fixed;inset:0;background:rgba(0,0,0,.35);z-index:1000;display:flex;justify-content:flex-end}.drawer{width:600px;max-width:95vw;height:100%;background:var(--bg);box-shadow:-4px 0 20px rgba(0,0,0,.15);display:flex;flex-direction:column;overflow:hidden}.drawer-head{display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--border);flex-shrink:0}.drawer-head h3{margin:0;font-size:18px}.drawer-body{flex:1;overflow-y:auto;padding:20px}.drawer-foot{padding:14px 20px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;gap:10px;flex-shrink:0}
.fs{border:1px solid var(--border);border-radius:8px;padding:16px;margin-bottom:16px}.fs legend{font-weight:600;font-size:14px;padding:0 8px;color:var(--accent)}.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.form-grid label{display:flex;flex-direction:column;gap:4px;font-size:13px;color:var(--muted)}.form-grid label.full{grid-column:1/-1}
.err{color:#f56c6c;font-size:13px}
</style>
