<template>
  <div>
    <div class="page-head"><h1>IP 地址管理</h1>
      <div class="head-ops">
        <button class="btn" :class="{'btn-primary':tab==='pools'}" @click="tab='pools';loadPools()">IP池</button>
        <button class="btn" :class="{'btn-primary':tab==='ips'}" @click="tab='ips';load();loadPools()">IP分配</button>
      </div>
    </div>

    <!-- ══════════ IP池管理 ══════════ -->
    <template v-if="tab==='pools'">
      <div class="toolbar">
        <select class="input" style="width:150px" v-model.number="pf.site_id" @change="loadPools"><option :value="0">全部站点</option><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option></select>
        <span style="flex:1"></span>
        <button class="btn btn-primary" @click="openPoolForm()">＋ 新建IP池</button>
      </div>
      <div class="card"><div class="table-wrap"><table class="data-table">
        <thead><tr><th>名称</th><th>CIDR</th><th>类型</th><th>网关</th><th>DNS</th><th>利用率</th><th>状态</th><th>操作</th></tr></thead>
        <tbody>
          <tr v-for="p in pools" :key="p.id">
            <td><a class="link" @click="viewPool(p)">{{ p.name }}</a></td>
            <td class="mono">{{ p.cidr }}</td>
            <td><span class="badge">{{ POOL_TYPES[p.type]||p.type }}</span></td>
            <td class="mono">{{ p.gateway||'-' }}</td><td class="mono">{{ p.dns||'-' }}</td>
            <td style="min-width:140px"><div class="util"><div class="util-bar"><i :class="utilCls(p)" :style="{width:utilPct(p)+'%'}"></i></div><span class="util-txt">{{ p.used_count||0 }}/{{ poolTotal(p) }} ({{ utilPct(p) }}%)</span></div></td>
            <td><span class="badge" :class="p.status?'st-1':'st-0'">{{ p.status?'启用':'停用' }}</span></td>
            <td class="ops"><button class="btn btn-mini" @click="viewPool(p)">查看IP</button><button class="btn btn-mini" @click="openPoolForm(p)">编辑</button><button class="btn btn-mini btn-mini-danger" @click="delPool(p)">删除</button></td>
          </tr>
          <tr v-if="!pools.length"><td colspan="8" class="empty">暂无IP池</td></tr>
        </tbody>
      </table></div></div>
    </template>

    <!-- ══════════ IP分配 ══════════ -->
    <template v-else>
      <div class="toolbar">
        <select class="input" style="width:180px" v-model.number="f.pool_id" @change="load">
          <option :value="0">全部IP池</option>
          <option v-for="p in pools" :key="p.id" :value="p.id">{{ p.name }} ({{ p.cidr }})</option>
        </select>
        <select class="input" style="width:110px" v-model="f.status" @change="load"><option value="">全部状态</option><option value="1">使用中</option><option value="0">空闲</option><option value="2">保留</option></select>
        <select class="input" style="width:110px" v-model="f.type" @change="load"><option value="">全部类型</option><option v-for="(n,k) in TYPES" :key="k" :value="k">{{ n }}</option></select>
        <input class="input search-input" v-model="f.keyword" placeholder="搜索IP/主机名/MAC" @keyup.enter="load" />
        <button class="btn" @click="load">查询</button>
        <span style="flex:1"></span>
        <button class="btn btn-primary" @click="openForm()">＋ 分配IP</button>
      </div>

      <!-- 网段可视化：选中具体池时显示 -->
      <div v-if="grid.show" class="card ip-grid-card">
        <div class="ip-grid-head">
          <b>{{ grid.name }}</b> <span class="mono muted">{{ grid.cidr }}</span>
          <span class="legend"><i class="g-dot used"></i>使用中 <i class="g-dot free"></i>空闲 <i class="g-dot resv"></i>保留 <i class="g-dot na"></i>网络/广播</span>
        </div>
        <div class="ip-grid">
          <span v-for="c in grid.cells" :key="c.ip" class="g-cell" :class="c.cls" :title="c.ip + (c.host?' · '+c.host:'')" @click="c.id && openForm(c.row)">{{ c.last }}</span>
        </div>
        <div v-if="grid.tooBig" class="muted small">网段大于 /23，仅展示分配明细表</div>
      </div>

      <div class="card"><div class="table-wrap"><table class="data-table">
        <thead><tr><th>IP</th><th>主机名</th><th>类型</th><th>MAC</th><th>IP池</th><th>状态</th><th>关联</th><th>操作</th></tr></thead>
        <tbody><tr v-for="ip in list" :key="ip.id">
          <td class="mono">{{ ip.ip }}</td><td>{{ ip.hostname||'-' }}</td>
          <td><span class="badge">{{ TYPES[ip.type]||ip.type }}</span></td>
          <td class="mono">{{ ip.mac||'-' }}</td><td class="muted">{{ poolName(ip.pool_id) }}</td>
          <td><span class="badge" :class="'st-'+ip.status">{{ ST[ip.status] }}</span></td>
          <td class="muted">{{ refText(ip) }}</td>
          <td class="ops"><button class="btn btn-mini" @click="openForm(ip)">编辑</button><button class="btn btn-mini btn-mini-danger" @click="del(ip)">释放</button></td></tr>
          <tr v-if="!list.length"><td colspan="8" class="empty">暂无</td></tr></tbody>
      </table></div></div>
    </template>

    <!-- IP分配抽屉 -->
    <div v-if="form.show" class="drawer-mask" @click.self="form.show=false"><div class="drawer"><div class="drawer-head"><h3>{{ form.id?'编辑':'分配' }}IP</h3><button class="btn btn-mini" @click="form.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body"><fieldset class="fs"><legend><SvgIcon name="pin" />基本信息</legend><div class="form-grid">
        <label>IP地址 *<input class="input" v-model="form.ip" placeholder="192.168.1.10" /></label><label>主机名<input class="input" v-model="form.hostname" /></label>
        <label>IP池 *<select class="input" v-model.number="form.pool_id"><option v-for="p in pools" :key="p.id" :value="p.id">{{ p.name }} ({{ p.cidr }})</option></select></label>
        <label>类型<select class="input" v-model="form.type"><option v-for="(n,k) in TYPES" :key="k" :value="k">{{ n }}</option></select></label>
        <label>MAC地址<input class="input" v-model="form.mac" placeholder="aa:bb:cc:dd:ee:ff" /></label>
        <label>状态<select class="input" v-model.number="form.status"><option :value="1">使用中</option><option :value="0">空闲</option><option :value="2">保留</option></select></label>
        <label>关联网络设备<select class="input" v-model.number="form.device_id"><option :value="0">无</option><option v-for="d in devices" :key="d.id" :value="d.id">{{ d.name }} ({{ d.mgmt_ip }})</option></select></label>
        <label>关联资产<select class="input" v-model.number="form.asset_id"><option :value="0">无</option><option v-for="a in assets" :key="a.id" :value="a.id">{{ a.name }}</option></select></label>
        <label class="full">备注<input class="input" v-model="form.description" /></label>
      </div></fieldset>
      <p v-if="form.err" class="err">{{ form.err }}</p></div>
      <div class="drawer-foot"><button class="btn" @click="form.show=false">取消</button><button class="btn btn-primary" :disabled="form.saving" @click="save">{{ form.saving?'保存中...':'保存' }}</button></div>
    </div></div>

    <!-- IP池抽屉 -->
    <div v-if="poolForm.show" class="drawer-mask" @click.self="poolForm.show=false"><div class="drawer"><div class="drawer-head"><h3>{{ poolForm.id?'编辑':'新建' }}IP池</h3><button class="btn btn-mini" @click="poolForm.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body"><fieldset class="fs"><legend><SvgIcon name="pin" />网段信息</legend><div class="form-grid">
        <label>名称 *<input class="input" v-model="poolForm.name" /></label>
        <label>CIDR *<input class="input" v-model="poolForm.cidr" placeholder="192.168.1.0/24" /></label>
        <label>站点 *<select class="input" v-model.number="poolForm.site_id"><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option></select></label>
        <label>类型<select class="input" v-model="poolForm.type"><option v-for="(n,k) in POOL_TYPES" :key="k" :value="k">{{ n }}</option></select></label>
        <label>网关<input class="input" v-model="poolForm.gateway" placeholder="192.168.1.1" /></label>
        <label>DNS<input class="input" v-model="poolForm.dns" placeholder="223.5.5.5,114.114.114.114" /></label>
        <label>状态<select class="input" v-model.number="poolForm.status"><option :value="1">启用</option><option :value="0">停用</option></select></label>
      </div></fieldset>
      <p v-if="poolForm.err" class="err">{{ poolForm.err }}</p></div>
      <div class="drawer-foot"><button class="btn" @click="poolForm.show=false">取消</button><button class="btn btn-primary" :disabled="poolForm.saving" @click="savePool">{{ poolForm.saving?'保存中...':'保存' }}</button></div>
    </div></div>
  </div>
</template>
<script setup>
import {ref,reactive,computed,watch,onMounted} from 'vue';import {useRoute} from 'vue-router';import api from '../api'
import { toastAuto, showConfirm } from '../ui-dialog'
const route=useRoute()
const TYPES={static:'静态',dhcp:'DHCP',reserved:'保留'}
const POOL_TYPES={lan:'业务网',mgmt:'管理网',wan:'外网',dmz:'DMZ'}
const ST={0:'空闲',1:'使用中',2:'保留'}
const sites=ref([]),pools=ref([]),list=ref([]),devices=ref([]),assets=ref([])
const tab=ref('ips')
const pf=reactive({site_id:0})
const f=reactive({pool_id:0,status:'',type:'',keyword:''})
const form=reactive({show:false,saving:false,err:'',id:0,ip:'',hostname:'',pool_id:0,type:'static',mac:'',status:1,description:'',device_id:0,asset_id:0})
const poolForm=reactive({show:false,saving:false,err:'',id:0,name:'',cidr:'',site_id:0,type:'lan',gateway:'',dns:'',status:1})
const poolName=id=>(pools.value.find(p=>p.id===id)||{}).name||'-'
const refText=ip=>{const d=ip.device_id&&devices.value.find(x=>x.id===ip.device_id);const a=ip.asset_id&&assets.value.find(x=>x.id===ip.asset_id);return d?d.name:(a?a.name:'-')}

// ——— IP池 ———
async function loadPools(){const params={size:200};if(pf.site_id)params.site_id=pf.site_id;const r=await api.get('/ip-pools',params);pools.value=r.data?.items||[]}
function poolTotal(p){ // 由CIDR推算可分配主机数
  const m=String(p.cidr||'').match(/\/(\d+)$/); if(!m)return p.total_count||0
  const bits=32-parseInt(m[1]); if(bits<=1)return bits===1?2:1
  const n=Math.pow(2,bits)-2; return p.total_count||n
}
const utilPct=p=>{const t=poolTotal(p);return t?Math.min(100,Math.round((p.used_count||0)/t*100)):0}
const utilCls=p=>{const v=utilPct(p);return v>=90?'u-red':v>=70?'u-yellow':'u-green'}
function openPoolForm(p){poolForm.err=''
  if(p)Object.assign(poolForm,{show:true,id:p.id,name:p.name,cidr:p.cidr,site_id:p.site_id,type:p.type||'lan',gateway:p.gateway||'',dns:p.dns||'',status:p.status??1})
  else Object.assign(poolForm,{show:true,id:0,name:'',cidr:'',site_id:pf.site_id||sites.value[0]?.id||0,type:'lan',gateway:'',dns:'',status:1})}
async function savePool(){poolForm.err='';poolForm.saving=true
  const body={name:poolForm.name,cidr:poolForm.cidr,site_id:poolForm.site_id,type:poolForm.type,gateway:poolForm.gateway,dns:poolForm.dns,status:poolForm.status}
  const r=poolForm.id?await api.put(`/ip-pools/${poolForm.id}`,body):await api.post('/ip-pools',body)
  poolForm.saving=false
  if(r.code===0){poolForm.show=false;await loadPools();load()}else poolForm.err=r.msg||'保存失败'}
async function delPool(p){if (!(await showConfirm({ message: `删除IP池「${p.name}」将同时释放池内全部IP分配记录，确认？` })))return
  const r=await api.del(`/ip-pools/${p.id}`);if(r.code!==0){toastAuto(r.msg||'删除失败');return}
  if(f.pool_id===p.id)f.pool_id=0;await loadPools();load()}
function viewPool(p){tab.value='ips';f.pool_id=p.id;f.status='';f.type='';f.keyword='';load()}

// ——— IP分配 ———
async function load(){const params={};
  if(f.pool_id)params.pool_id=f.pool_id;if(f.status!=='')params.status=f.status;if(f.type)params.type=f.type;if(f.keyword)params.keyword=f.keyword
  const r=await api.get('/ip-assignments',params);list.value=r.data?.items||[]}
function openForm(ip){form.err=''
  if(ip&&ip.id)Object.assign(form,{show:true,id:ip.id,ip:ip.ip,hostname:ip.hostname||'',pool_id:ip.pool_id,type:ip.type||'static',mac:ip.mac||'',status:ip.status??1,description:ip.description||'',device_id:ip.device_id||0,asset_id:ip.asset_id||0})
  else Object.assign(form,{show:true,id:0,ip:'',hostname:'',pool_id:f.pool_id||pools.value[0]?.id||0,type:'static',mac:'',status:1,description:'',device_id:0,asset_id:0})}
async function save(){form.err='';form.saving=true
  const body={ip:form.ip,hostname:form.hostname,pool_id:form.pool_id,type:form.type,mac:form.mac,status:form.status,device_id:form.device_id||null,asset_id:form.asset_id||null}
  const r=form.id?await api.put(`/ip-assignments/${form.id}`,body):await api.post('/ip-assignments',body)
  form.saving=false
  if(r.code===0){form.show=false;await Promise.all([load(),loadPools()])}else form.err=r.msg||'保存失败'}
async function del(ip){if (!(await showConfirm({ message: `释放 ${ip.ip}？` })))return;const r=await api.del(`/ip-assignments/${ip.id}`);if(r.code===0){await Promise.all([load(),loadPools()])}else toastAuto(r.msg||'释放失败')}

// ——— 网段可视化网格 ———
const grid=reactive({show:false,name:'',cidr:'',cells:[],tooBig:false})
function ip2int(s){return s.split('.').reduce((a,o)=>(a<<8)+(+o),0)>>>0}
function int2ip(n){return [(n>>>24)&255,(n>>>16)&255,(n>>>8)&255,n&255].join('.')}
function buildGrid(){const p=pools.value.find(x=>x.id===(f.pool_id||null))
  if(!p||!p.cidr||!/^\d+\.\d+\.\d+\.\d+\/\d+$/.test(p.cidr)){grid.show=false;return}
  const[base,mask]=p.cidr.split('/');const m=+mask;const size=Math.pow(2,32-m)
  grid.show=true;grid.name=p.name;grid.cidr=p.cidr;grid.tooBig=size>1024
  if(size>1024){grid.cells=[];return}
  const start=(ip2int(base)>>> (32-m))<<(32-m)
  const byIp={};list.value.filter(x=>x.pool_id===p.id).forEach(x=>byIp[x.ip]=x)
  const cells=[]
  for(let i=0;i<size&&cells.length<1024;i++){const ip=int2ip((start+i)>>>0);const a=byIp[ip]
    let cls='free';if(i===0||i===size-1)cls='na';else if(a)cls=a.status===2?'resv':'used'
    cells.push({ip,last:i,cls,id:a?.id||0,host:a?.hostname||'',row:a||null})}
  grid.cells=cells}
watch(list,buildGrid)
watch(()=>f.pool_id,()=>{if(tab.value==='ips')buildGrid()})
watch(tab,v=>{if(v==='ips'){buildGrid()}else{loadPools()}})

onMounted(async()=>{
  const[sp,dv,as]=await Promise.all([api.get('/sites',{size:100}),api.get('/network-devices',{size:200}),api.get('/assets',{size:500})])
  sites.value=sp.data?.items||[];devices.value=dv.data?.items||[];assets.value=as.data?.items||[]
  await loadPools()
  const qp=+route.query.pool_id||0;if(qp){tab.value='ips';f.pool_id=qp}
  load()})
</script>
<style scoped>
.badge.st-1{color:#67c23a;border-color:rgba(103,194,58,.4)}.badge.st-0{color:#909399;border-color:rgba(144,147,153,.4)}.badge.st-2{color:#e6a23c;border-color:rgba(230,162,60,.4)}
.head-ops{display:flex;gap:8px}
.util{display:flex;align-items:center;gap:8px}.util-bar{flex:1;height:8px;border-radius:4px;background:var(--border);overflow:hidden}.util-bar i{display:block;height:100%;border-radius:4px}
.u-green{background:#67c23a}.u-yellow{background:#e6a23c}.u-red{background:#f56c6c}.util-txt{font-size:12px;color:var(--muted);white-space:nowrap}
.link{color:var(--accent);cursor:pointer;text-decoration:underline}
.ip-grid-card{padding:12px 16px;margin-bottom:12px}
.ip-grid-head{display:flex;align-items:center;gap:10px;margin-bottom:10px;font-size:13px}
.ip-grid-head .legend{margin-left:auto;display:flex;align-items:center;gap:6px;color:var(--muted);font-size:12px}
.g-dot{display:inline-block;width:10px;height:10px;border-radius:2px;margin-left:8px}
.g-dot.used{background:#67c23a}.g-dot.free{background:var(--border)}.g-dot.resv{background:#e6a23c}.g-dot.na{background:#f56c6c;opacity:.5}
.ip-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(34px,1fr));gap:3px;max-height:240px;overflow-y:auto}
.g-cell{height:22px;line-height:22px;text-align:center;font-size:10px;font-family:monospace;border-radius:3px;background:var(--border);color:var(--muted);cursor:default}
.g-cell.used{background:rgba(103,194,58,.75);color:#fff;cursor:pointer}
.g-cell.resv{background:rgba(230,162,60,.75);color:#fff;cursor:pointer}
.g-cell.na{background:rgba(245,108,108,.35);color:#fdd}
.g-cell.used:hover,.g-cell.resv:hover{filter:brightness(1.15)}
.small{font-size:12px}
.err{color:#f56c6c;font-size:13px;margin:8px 0 0}
.drawer-mask{position:fixed;inset:0;background:rgba(0,0,0,.35);z-index:1000;display:flex;justify-content:flex-end}.drawer{width:600px;max-width:95vw;height:100%;background:var(--bg);box-shadow:-4px 0 20px rgba(0,0,0,.15);display:flex;flex-direction:column;overflow:hidden}.drawer-head{display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--border);flex-shrink:0}.drawer-head h3{margin:0;font-size:18px}.drawer-body{flex:1;overflow-y:auto;padding:20px}.drawer-foot{padding:14px 20px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;gap:10px;flex-shrink:0}
.fs{border:1px solid var(--border);border-radius:8px;padding:16px;margin-bottom:16px}.fs legend{font-weight:600;font-size:14px;padding:0 8px;color:var(--accent)}.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.form-grid label{display:flex;flex-direction:column;gap:4px;font-size:13px;color:var(--muted)}.form-grid label.full{grid-column:1/-1}
</style>
