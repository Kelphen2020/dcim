<template>
  <div>
    <div class="page-head"><h1>网络设备</h1></div>
    <div class="toolbar">
      <select class="input" style="width:150px" v-model.number="f.site_id" @change="load(1)"><option :value="0">全部站点</option><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option></select>
      <select class="input" style="width:120px" v-model="f.type" @change="load(1)"><option value="">全部类型</option><option v-for="(n,k) in TYPES" :key="k" :value="k">{{ n }}</option></select>
      <input class="input search-input" v-model="f.keyword" placeholder="搜索名称/编码/IP" @keyup.enter="load(1)" />
      <button class="btn" @click="load(1)">查询</button><span style="flex:1"></span>
      <button class="btn" @click="openForm(null)" title="从资产台账选择网络类资产，一键预填并强关联；资产退役批准后设备自动离线">从资产创建</button>
      <button class="btn btn-primary" @click="openForm()">＋ 添加设备</button>
    </div>
    <div class="card"><div class="table-wrap"><table class="data-table">
      <thead><tr><th>编码</th><th>名称</th><th>类型</th><th>厂商/型号</th><th>管理IP</th><th>端口</th><th>状态</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="d in list" :key="d.id">
          <td class="mono">{{ d.code }}<div v-if="d.asset_id" class="muted" style="font-size:11px">{{ assetLabel(d.asset_id) }}</div></td><td>{{ d.name }}</td><td><span class="badge">{{ TYPES[d.type]||d.type }}</span></td>
          <td class="muted">{{ d.vendor }} {{ d.model }}</td><td class="mono">{{ d.mgmt_ip }}</td><td>{{ d.port_count }}</td>
          <td><span class="badge" :class="'st-'+d.status">{{ ST[d.status] }}</span></td>
          <td class="ops"><button class="btn btn-mini" @click="openDetail(d)">详情</button><button class="btn btn-mini" @click="openForm(d)">编辑</button><button class="btn btn-mini" @click="goBackup(d)">备份</button><button class="btn btn-mini btn-mini-danger" @click="del(d)">删除</button></td>
        </tr>
        <tr v-if="!list.length"><td colspan="8" class="empty">暂无设备</td></tr>
      </tbody>
    </table></div><Pagination :page="page" :total="total" :page-size="size" @change="load" /></div>

    <!-- V66 B2-1: 设备详情抽屉（含关联测点与实时读数） -->
    <div v-if="detail.show" class="drawer-mask" @click.self="detail.show=false"><div class="drawer">
      <div class="drawer-head"><h3>设备详情 · {{ detail.d?.name }}</h3><button class="btn btn-mini" @click="detail.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body" v-if="detail.d">
        <div class="dt-grid">
          <div><span class="dl">编码</span><span class="mono">{{ detail.d.code }}</span></div>
          <div><span class="dl">类型</span>{{ TYPES[detail.d.type]||detail.d.type }}</div>
          <div><span class="dl">管理IP</span><span class="mono">{{ detail.d.mgmt_ip||'-' }}</span></div>
          <div><span class="dl">状态</span><span class="badge" :class="'st-'+detail.d.status">{{ ST[detail.d.status] }}</span></div>
          <div><span class="dl">厂商/型号</span>{{ (detail.d.vendor||'')+' '+(detail.d.model||'')||'-' }}</div>
          <div><span class="dl">端口数</span>{{ detail.d.port_count||0 }}</div>
          <div><span class="dl">最后心跳</span>{{ detail.d.last_seen_at ? String(detail.d.last_seen_at).replace('T',' ').slice(0,16) : '从未上报' }}</div>
          <div v-if="detail.d.asset_id"><span class="dl">关联资产</span><a class="lnk" href="javascript:;" @click="$router.push({path:'/assets',query:{id:detail.d.asset_id,detail:'1'}})">{{ assetLabel(detail.d.asset_id) }}</a><span class="muted" style="font-size:11.5px;margin-left:6px">退役批准将自动离线</span></div>
        </div>
        <h4 class="sec-title"><SvgIcon name="gauge" />关联测点（{{ detail.total }}）
          <span v-if="detail.alarming" class="badge st-3" style="margin-left:8px">告警 {{ detail.alarming }}</span>
          <span v-if="detail.offline" class="badge st-0" style="margin-left:6px">离线 {{ detail.offline }}</span>
        </h4>
        <div class="pt-list">
          <div v-for="p in detail.points" :key="p.id" class="pt-item" @click="$router.push({path:'/monitoring-points', query:{id:p.id, detail:'1'}})">
            <div class="pt-main">
              <span class="pt-name">{{ p.name }}</span>
              <span class="badge pt-type">{{ PTT[p.type]||p.type }}</span>
            </div>
            <div class="pt-side">
              <strong class="pt-val">{{ p.current_value!==null&&p.current_value!==undefined ? p.current_value+(p.unit||'') : '-' }}</strong>
              <span class="badge" :class="'st-'+p.status">{{ PTST[p.status] }}</span>
            </div>
          </div>
          <div v-if="!detail.points.length" class="muted" style="font-size:12.5px;padding:8px 0">暂无绑定测点 —— 在「测点管理」编辑测点时可绑定本设备</div>
        </div>
      </div>
      <div class="drawer-foot"><button class="btn" @click="detail.show=false">关闭</button><button class="btn" @click="heartbeat" title="上报一次心跳：刷新最后在线时间，离线设备自动恢复上线">上报心跳</button><button class="btn btn-primary" @click="openForm(detail.d)">编辑设备</button></div>
    </div></div>

    <div v-if="form.show" class="drawer-mask" @click.self="form.show=false"><div class="drawer">
      <div class="drawer-head"><h3>{{ form.id?'编辑':'添加' }}网络设备</h3><button class="btn btn-mini" @click="form.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <fieldset class="fs" v-if="!form.id"><legend><SvgIcon name="layers" />从资产创建（可选）</legend>
          <div class="form-grid">
            <label class="full">选择台账资产
              <select class="input" v-model.number="form.picked" @change="applyAsset">
                <option :value="0">— 不关联，手工填写 —</option>
                <option v-for="a in assetOpts" :key="a.id" :value="a.id">{{ a.code }} · {{ a.name }}{{ a.ip ? ' · '+a.ip : '' }}</option>
              </select>
            </label>
            <div class="full muted" style="font-size:12px;line-height:1.6" v-if="form.picked">已预填编码/名称/厂商/型号/序列号/管理IP，可再修改。保存后与资产强关联：资产退役批准时本设备自动置离线。</div>
            <div class="full muted" style="font-size:12px" v-else-if="!assetOpts.length">当前站点暂无可关联的未绑定资产（在架/在库且未被其他网络设备绑定）</div>
          </div>
        </fieldset>
        <fieldset class="fs"><legend><SvgIcon name="pin" />基本信息</legend>
          <div class="form-grid">
            <label>编码 *<input class="input" v-model="form.code" /></label><label>名称 *<input class="input" v-model="form.name" /></label>
            <label>类型 *<select class="input" v-model="form.type"><option v-for="(n,k) in TYPES" :key="k" :value="k">{{ n }}</option></select></label>
            <label>站点 *<select class="input" v-model.number="form.site_id" @change="form.picked=0;loadAssetOpts()"><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option></select></label>
            <label>厂商<input class="input" v-model="form.vendor" /></label><label>型号<input class="input" v-model="form.model" /></label>
            <label>管理IP<input class="input" v-model="form.mgmt_ip" /></label><label>固件版本<input class="input" v-model="form.firmware" /></label>
            <label>序列号<input class="input" v-model="form.serial_no" /></label><label>状态<select class="input" v-model.number="form.status"><option v-for="(n,k) in ST" :key="k" :value="k">{{ n }}</option></select></label>
            <label class="full" v-if="form.id&&form.asset_id">关联资产
              <span style="display:flex;gap:8px;align-items:center"><input class="input" :value="assetLabel(form.asset_id)" disabled style="flex:1" /><button type="button" class="btn btn-mini" @click="form.asset_id=null">解绑</button></span>
            </label>
            <label class="full">描述<input class="input" v-model="form.description" /></label>
          </div>
        </fieldset>
        <fieldset class="fs"><legend><SvgIcon name="plug" />SNMP 凭证（可选）</legend>
          <div class="form-grid">
            <label>SNMP 版本<select class="input" v-model="form.snmp_ver"><option value="v2c">v2c</option><option value="v1">v1</option></select></label>
            <label>Community<input class="input" v-model="form.snmp_community" placeholder="public" /></label>
            <label>SSH 端口<input class="input" type="number" v-model.number="form.ssh_port" placeholder="22" /></label>
            <label>SSH 用户名<input class="input" v-model="form.ssh_user" /></label>
          </div>
        </fieldset>
      </div>
      <div class="drawer-foot"><button class="btn" @click="form.show=false">取消</button><button class="btn btn-primary" @click="save">保存</button></div>
    </div></div>
  </div>
</template>

<script setup>
import {ref,reactive,onMounted,watch} from 'vue';import {useRouter,useRoute} from 'vue-router';import api from '../api';import Pagination from '../components/Pagination.vue'
import { toastAuto, showConfirm } from '../ui-dialog'
const router=useRouter()
const route=useRoute()
const PTT={temp_humidity:'温湿度',ups:'UPS',power_dist:'配电',hvac:'空调',water_leak:'漏水',smoke:'烟感',cdu:'CDU',leak_rope:'漏绳'}
const PTST={0:'离线',1:'正常',2:'告警',3:'故障'}
const detail=reactive({show:false,d:null,points:[],total:0,alarming:0,offline:0})
async function openDetail(d){
  detail.d=d;detail.points=[];detail.total=0;detail.alarming=0;detail.offline=0;detail.show=true
  const r=await api.get(`/network-devices/${d.id}/points`)
  if(r.code===0){if(r.data?.device)detail.d=r.data.device;detail.points=r.data?.points||[];detail.total=r.data?.total||0;detail.alarming=r.data?.alarming||0;detail.offline=r.data?.offline||0}
}
// V67/B2-2: 心跳上报（离线判定数据源；离线设备上报后自动回上线）
async function heartbeat(){
  const d=detail.d; if(!d) return
  const r=await api.post(`/network-devices/${d.id}/heartbeat`)
  if(r.code===0){ toastAuto(r.data.back_online?'心跳已上报，设备恢复在线':'心跳已上报'); openDetail(d); load() }
  else toastAuto(r.msg||'心跳上报失败')
}
async function applyDeep(){
  const q=route.query
  if(q.id&&q.detail==='1'){
    const dr=await api.get(`/network-devices/${q.id}/points`)
    if(dr.code===0){detail.d=dr.data.device;detail.points=dr.data.points||[];detail.total=dr.data.total||0;detail.alarming=dr.data.alarming||0;detail.offline=dr.data.offline||0;detail.show=true}
  } else detail.show=false
}
watch(()=>route.query,()=>{if(route.path==='/network-devices')applyDeep()})
const goBackup=d=>router.push({path:'/config-backups',query:{device_id:d.id}})
const TYPES={switch:'交换机',router:'路由器',firewall:'防火墙',ap:'无线AP',controller:'控制器'}
const ST={1:'在线',0:'离线',2:'维护',3:'故障'}
const sites=ref([]),list=ref([]),page=ref(1),total=ref(0),size=20
const f=reactive({site_id:0,type:'',keyword:''})
const form=reactive({show:false,id:0,code:'',name:'',type:'switch',site_id:0,vendor:'',model:'',mgmt_ip:'',firmware:'',serial_no:'',status:1,description:'',snmp_ver:'v2c',snmp_community:'public',ssh_port:22,ssh_user:'admin',asset_id:null,picked:0})
// V69/B2-4: 从资产创建（强关联）
const assetOpts=ref([]),assetMap=ref({})
async function loadAssetOpts(){const r=await api.get('/network-devices/asset-options',{site_id:form.site_id||0});assetOpts.value=r.data?.items||[]}
function assetLabel(id){const a=assetMap.value[id];return a?(a.code+' · '+a.name):('资产 #'+id)}
function applyAsset(){const a=assetOpts.value.find(x=>x.id===form.picked);if(!a){form.asset_id=null;return}
  form.asset_id=a.id
  if(!form.code)form.code=a.code
  form.name=a.name;form.vendor=a.brand||a.vendor||'';form.model=a.model||'';form.serial_no=a.sn||''
  if(a.ip)form.mgmt_ip=a.ip
  if(a.site_id)form.site_id=a.site_id
  const cat=String(a.category||'').toLowerCase();if(TYPES[cat])form.type=cat}
async function load(p){page.value=p||1;const params={page:page.value,size};if(f.site_id)params.site_id=f.site_id;if(f.type)params.type=f.type;if(f.keyword)params.keyword=f.keyword;const r=await api.get('/network-devices',params);list.value=r.data?.items||[];total.value=r.data?.total||0;assetMap.value=r.data?.asset_map||{}}
function openForm(d){if(d)Object.assign(form,{show:true,id:d.id,code:d.code,name:d.name,type:d.type,site_id:d.site_id,vendor:d.vendor||'',model:d.model||'',mgmt_ip:d.mgmt_ip||'',firmware:d.firmware||'',serial_no:d.serial_no||'',status:d.status||1,description:d.description||'',snmp_ver:'v2c',snmp_community:'public',ssh_port:22,ssh_user:'admin',asset_id:d.asset_id||null,picked:0});else{Object.assign(form,{show:true,id:0,code:'',name:'',type:'switch',site_id:f.site_id||sites.value[0]?.id||0,vendor:'',model:'',mgmt_ip:'',firmware:'',serial_no:'',status:1,description:'',snmp_ver:'v2c',snmp_community:'public',ssh_port:22,ssh_user:'admin',asset_id:null,picked:0});loadAssetOpts()}}
async function save(){const r=form.id?await api.put(`/network-devices/${form.id}`,{...form}):await api.post('/network-devices',{...form});if(r.code===0){form.show=false;load()}else toastAuto(r.msg)}
async function del(d){if (!(await showConfirm({ message: '删除' })))return;await api.del(`/network-devices/${d.id}`);load()}
onMounted(async()=>{const r=await api.get('/sites',{size:100});sites.value=r.data?.items||[];form.site_id=sites.value[0]?.id||0;load();applyDeep()})
</script>
<style scoped>
.badge.st-1{color:#67c23a;border-color:rgba(103,194,58,.4)}.badge.st-0{color:#909399;border-color:rgba(144,147,153,.4)}.badge.st-2{color:#e6a23c;border-color:rgba(230,162,60,.4)}.badge.st-3{color:#f56c6c;border-color:rgba(245,108,108,.4)}
.drawer-mask{position:fixed;inset:0;background:rgba(0,0,0,.35);z-index:1000;display:flex;justify-content:flex-end}.drawer{width:600px;max-width:95vw;height:100%;background:var(--bg);box-shadow:-4px 0 20px rgba(0,0,0,.15);display:flex;flex-direction:column;overflow:hidden}.drawer-head{display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--border);flex-shrink:0}.drawer-head h3{margin:0;font-size:18px}.drawer-body{flex:1;overflow-y:auto;padding:20px}.drawer-foot{padding:14px 20px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;gap:10px;flex-shrink:0}
.fs{border:1px solid var(--border);border-radius:8px;padding:16px;margin-bottom:16px}.fs legend{font-weight:600;font-size:14px;padding:0 8px;color:var(--accent)}
.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.form-grid label{display:flex;flex-direction:column;gap:4px;font-size:13px;color:var(--muted)}.form-grid label.full{grid-column:1/-1}
.dt-grid{display:grid;grid-template-columns:1fr 1fr;gap:10px;font-size:13.5px;margin-bottom:16px}.dt-grid .dl{display:inline-block;min-width:70px;color:var(--muted);font-size:12.5px}
.sec-title{display:flex;align-items:center;gap:6px;font-size:14px;margin:14px 0 8px;color:var(--text)}
.pt-list{display:flex;flex-direction:column;gap:6px}
.pt-item{display:flex;align-items:center;justify-content:space-between;gap:10px;padding:9px 12px;border:1px solid var(--border);border-radius:8px;cursor:pointer;transition:border-color .15s}
.pt-item:hover{border-color:var(--accent)}
.pt-main{display:flex;align-items:center;gap:8px;min-width:0}.pt-name{font-size:13.5px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.pt-type{font-size:11px;flex-shrink:0}
.pt-side{display:flex;align-items:center;gap:8px;flex-shrink:0}.pt-val{font-size:14px;font-variant-numeric:tabular-nums}
.lnk{color:var(--accent);cursor:pointer;text-decoration:none}.lnk:hover{text-decoration:underline}
</style>