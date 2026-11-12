<template>
  <div>
    <div class="page-head"><h1>故障案例库</h1></div>
    <div class="toolbar">
      <select class="input" style="width:130px" v-model="f.category" @change="load(1)"><option value="">全部分类</option><option v-for="(n,k) in CATS" :key="k" :value="k">{{ n }}</option></select>
      <select class="input" style="width:130px" v-model="f.point_type" @change="load(1)"><option value="">全部测点类型</option><option v-for="(n,k) in PTYPES" :key="k" :value="k">{{ n }}</option></select>
      <input class="input search-input" v-model="f.keyword" placeholder="搜索标题/现象/原因/方案/关键词" @keyup.enter="load(1)" />
      <button class="btn" @click="load(1)">查询</button>
      <span style="flex:1"></span>
      <button class="btn btn-primary" @click="openForm()">＋ 新建案例</button>
    </div>

    <div class="card"><div class="table-wrap"><table class="data-table">
      <thead><tr><th>编号</th><th>标题</th><th>分类</th><th>测点类型</th><th>关联资产</th><th>历时</th><th>来源工单</th><th>被引用</th><th>创建时间</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="fc in list" :key="fc.id">
          <td class="mono"><a class="link" @click="openDetail(fc)">{{ fc.code }}</a></td>
          <td>{{ fc.title }}</td>
          <td><span class="badge">{{ CATS[fc.category]||fc.category }}</span></td>
          <td class="muted">{{ PTYPES[fc.point_type]||fc.point_type||'-' }}</td>
          <td class="muted">{{ fc.asset_id ? assetName(fc.asset_id) : '-' }}</td>
          <td class="muted">{{ fc.downtime_min ? fc.downtime_min+'分钟' : '-' }}</td>
          <td><a v-if="fc.source_wo_id" class="link" @click="$router.push('/work-orders?id='+fc.source_wo_id)">工单#{{ fc.source_wo_id }}</a><span v-else class="muted">手工</span></td>
          <td class="muted">{{ fc.usage_count }} 次</td>
          <td class="muted">{{ fmt(fc.created_at) }}</td>
          <td class="ops">
            <button class="btn btn-mini" @click="openDetail(fc)">详情</button>
            <button class="btn btn-mini" @click="openForm(fc)">编辑</button>
            <button class="btn btn-mini btn-danger" @click="del(fc)">删除</button>
          </td>
        </tr>
        <tr v-if="!list.length"><td colspan="10" class="empty">暂无案例——在工单详情里点「沉淀为案例」，或手工新建</td></tr>
      </tbody>
    </table></div><Pagination :page="page" :total="total" :page-size="size" @change="load" /></div>

    <!-- 详情抽屉 -->
    <div v-if="detail.show" class="drawer-mask" @click.self="detail.show=false"><div class="drawer">
      <div class="drawer-head"><h3>案例 {{ detail.fc?.code }}</h3><button class="btn btn-mini" @click="detail.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <template v-if="detail.fc">
          <h3 style="margin:0 0 12px">{{ detail.fc.title }}</h3>
          <div class="fc-meta">
            <span class="badge">{{ CATS[detail.fc.category]||detail.fc.category }}</span>
            <span v-if="detail.fc.point_type" class="badge">{{ PTYPES[detail.fc.point_type] }}</span>
            <span class="muted">历时 {{ detail.fc.downtime_min||'-' }} 分钟</span>
            <span class="muted">被引用 {{ detail.fc.usage_count }} 次</span>
          </div>
          <div class="fc-sec"><div class="fc-t"><SvgIcon name="search" />故障现象</div><div class="fc-c">{{ detail.fc.symptom || '-' }}</div></div>
          <div class="fc-sec"><div class="fc-t"><SvgIcon name="info" />根本原因</div><div class="fc-c">{{ detail.fc.cause || '-' }}</div></div>
          <div class="fc-sec"><div class="fc-t"><SvgIcon name="wrench" />解决方案</div><div class="fc-c">{{ detail.fc.solution || '-' }}</div></div>
          <div class="fc-sec" v-if="detail.fc.keywords"><div class="fc-t"><SvgIcon name="tag" />关键词</div><div class="fc-c">{{ detail.fc.keywords }}</div></div>
          <div class="fc-sec" v-if="detail.fc.source_wo_id"><div class="fc-t"><SvgIcon name="clipboard" />来源工单</div>
            <a class="link" @click="$router.push('/work-orders?id='+detail.fc.source_wo_id)">工单 #{{ detail.fc.source_wo_id }} ›</a></div>
          <div class="fc-sec" v-if="detail.similar.length"><div class="fc-t"><SvgIcon name="book" />相似案例</div>
            <div v-for="s in detail.similar" :key="s.id" class="fc-sim" @click="openDetail(s)"><a class="link">{{ s.code }} {{ s.title }}</a><span class="muted">（匹配度 {{ s.score }}）</span></div>
          </div>
        </template>
      </div>
      <div class="drawer-foot"><button class="btn" @click="detail.show=false">关闭</button><button class="btn btn-primary" @click="openForm(detail.fc)">编辑</button></div>
    </div></div>

    <!-- 编辑抽屉 -->
    <div v-if="form.show" class="drawer-mask" @click.self="form.show=false"><div class="drawer">
      <div class="drawer-head"><h3>{{ form.id?'编辑案例':'新建案例' }}</h3><button class="btn btn-mini" @click="form.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <fieldset class="fs"><legend><SvgIcon name="pin" />基本信息</legend>
          <div class="form-grid">
            <label class="full">标题 *<input class="input" v-model="form.title" placeholder="如: UPS-1# 电池组放电异常导致机柜掉电" /></label>
            <label>分类<select class="input" v-model="form.category"><option v-for="(n,k) in CATS" :key="k" :value="k">{{ n }}</option></select></label>
            <label>测点类型<select class="input" v-model="form.point_type"><option value="">（无）</option><option v-for="(n,k) in PTYPES" :key="k" :value="k">{{ n }}</option></select></label>
            <label>所属站点<select class="input" v-model.number="form.site_id"><option :value="0">全站通用</option><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option></select></label>
            <label>关联资产<select class="input" v-model.number="form.asset_id"><option :value="0">（无）</option><option v-for="a in assets" :key="a.id" :value="a.id">{{ a.name }}（{{ a.code }}）</option></select></label>
            <label>故障历时(分钟)<input class="input" type="number" min="0" v-model.number="form.downtime_min" /></label>
            <label>来源工单ID<input class="input" type="number" min="0" v-model.number="form.source_wo_id" placeholder="0=手工" /></label>
          </div>
        </fieldset>
        <fieldset class="fs"><legend><SvgIcon name="search" />现象 → 原因 → 方案</legend>
          <div class="form-grid">
            <label class="full">故障现象<textarea class="input" rows="3" v-model="form.symptom" style="width:100%;box-sizing:border-box"></textarea></label>
            <label class="full">根本原因<textarea class="input" rows="3" v-model="form.cause" style="width:100%;box-sizing:border-box"></textarea></label>
            <label class="full">解决方案<textarea class="input" rows="3" v-model="form.solution" style="width:100%;box-sizing:border-box"></textarea></label>
            <label class="full">关键词（逗号分隔，推荐匹配用）<input class="input" v-model="form.keywords" placeholder="如: UPS,电池,掉电" /></label>
          </div>
        </fieldset>
        <p v-if="form.err" class="err">{{ form.err }}</p>
      </div>
      <div class="drawer-foot"><button class="btn" @click="form.show=false">取消</button><button class="btn btn-primary" @click="save">保存</button></div>
    </div></div>
  </div>
</template>
<script setup>
import {ref,reactive,onMounted} from 'vue'
import {useRoute} from 'vue-router'
import api from '../api'
import { toastAuto, showConfirm } from '../ui-dialog'
import Pagination from '../components/Pagination.vue'
const route=useRoute()
const CATS={power:'供配电',ups:'UPS',hvac:'空调制冷',network:'网络',server:'服务器',storage:'存储',security:'安防',other:'其他'}
const PTYPES={temp_humidity:'温湿度',hvac:'空调',ups:'UPS',power_dist:'配电',water_leak:'漏水',smoke:'烟感'}
const sites=ref([]),assets=ref([]),list=ref([]),page=ref(1),total=ref(0),size=20
const f=reactive({category:'',point_type:'',keyword:''})
const detail=reactive({show:false,fc:null,similar:[]})
const form=reactive({show:false,id:0,site_id:0,title:'',symptom:'',cause:'',solution:'',category:'power',point_type:'',asset_id:0,keywords:'',source_wo_id:0,downtime_min:0,err:''})
const fmt=t=>t?String(t).replace('T',' ').slice(0,16):'-'
const assetName=id=>{const a=assets.value.find(x=>x.id===id);return a?a.name:'资产#'+id}
async function load(p){page.value=p||1;const params={page:page.value,size};if(f.category)params.category=f.category;if(f.point_type)params.point_type=f.point_type;if(f.keyword)params.keyword=f.keyword
  const r=await api.get('/fault-cases',params);list.value=r.data?.items||[];total.value=r.data?.total||0}
async function openDetail(fc){const r=await api.get(`/fault-cases/${fc.id}`);detail.fc=r.code===0?r.data:fc;detail.similar=[];detail.show=true
  const sr=await api.get('/fault-cases/recommend',{text:fc.title+' '+fc.symptom,point_type:fc.point_type||'',category:fc.category,limit:5})
  detail.similar=(sr.data?.items||[]).filter(x=>x.id!==fc.id).slice(0,4)}
function openForm(fc){if(fc)Object.assign(form,{show:true,id:fc.id,site_id:fc.site_id||0,title:fc.title,symptom:fc.symptom||'',cause:fc.cause||'',solution:fc.solution||'',category:fc.category||'power',point_type:fc.point_type||'',asset_id:fc.asset_id||0,keywords:fc.keywords||'',source_wo_id:fc.source_wo_id||0,downtime_min:fc.downtime_min||0,err:''})
  else Object.assign(form,{show:true,id:0,site_id:sites.value[0]?.id||0,title:'',symptom:'',cause:'',solution:'',category:'power',point_type:'',asset_id:0,keywords:'',source_wo_id:0,downtime_min:0,err:''})}
async function save(){form.err='';if(!form.title.trim()){form.err='标题必填';return}
  const body={site_id:form.site_id,title:form.title,symptom:form.symptom,cause:form.cause,solution:form.solution,category:form.category,point_type:form.point_type,asset_id:form.asset_id,keywords:form.keywords,source_wo_id:form.source_wo_id,downtime_min:form.downtime_min}
  const r=form.id?await api.put(`/fault-cases/${form.id}`,body):await api.post('/fault-cases',body)
  if(r.code===0||r._status===201){form.show=false;load(1)}else form.err=r.msg}
async function del(fc){if (!(await showConfirm({ message: `删除案例「${fc.title}」？` })))return;const r=await api.del(`/fault-cases/${fc.id}`);if(r.code===0)load(page.value);else toastAuto(r.msg)}
onMounted(async()=>{const[ss,aa]=await Promise.all([api.get('/sites',{size:100}),api.get('/assets',{size:300})]);sites.value=ss.data?.items||[];assets.value=aa.data?.items||[];load()})
</script>
<style scoped>
.link{color:var(--accent);cursor:pointer;font-weight:600}.link:hover{text-decoration:underline}
.btn-danger{color:#f56c6c;border-color:rgba(245,108,108,.4)}
.fc-meta{display:flex;gap:10px;align-items:center;margin-bottom:14px;flex-wrap:wrap}
.fc-sec{margin-bottom:14px}.fc-t{font-size:13px;font-weight:600;color:var(--accent);margin-bottom:5px}
.fc-c{font-size:13.5px;line-height:1.7;color:var(--text2);white-space:pre-wrap;background:var(--bg2);border-radius:8px;padding:10px 12px}
.fc-sim{padding:5px 0;font-size:13px;cursor:pointer}
.drawer-mask{position:fixed;inset:0;background:rgba(0,0,0,.35);z-index:1000;display:flex;justify-content:flex-end}
.drawer{width:600px;max-width:95vw;height:100%;background:var(--bg);box-shadow:-4px 0 20px rgba(0,0,0,.15);display:flex;flex-direction:column;overflow:hidden}
.drawer-head{display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--border);flex-shrink:0}
.drawer-head h3{margin:0;font-size:18px}
.drawer-body{flex:1;overflow-y:auto;padding:20px}
.drawer-foot{padding:14px 20px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;gap:10px;flex-shrink:0}
.fs{border:1px solid var(--border);border-radius:8px;padding:16px;margin-bottom:16px}
.fs legend{font-size:14px;padding:0 8px;color:var(--accent)}
.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}
.form-grid label{display:flex;flex-direction:column;gap:4px;font-size:13px;color:var(--muted)}
.form-grid label.full{grid-column:1/-1}
.err{color:#f56c6c;font-size:13px}
@media (max-width:900px){.form-grid{grid-template-columns:1fr}}
</style>
