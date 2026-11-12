<template>
  <div>
    <div class="page-head"><h1>变更管理</h1></div>
    <div class="toolbar">
      <select class="input" style="width:150px" v-model.number="f.site_id" @change="load"><option :value="0">全部站点</option><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option></select>
      <select class="input" style="width:120px" v-model="f.status" @change="load"><option value="">全部状态</option><option v-for="(n,k) in CST" :key="k" :value="k">{{ n }}</option></select>
      <span style="flex:1"></span><button class="btn btn-primary" @click="openForm()">＋ 新建变更</button>
    </div>
    <div class="card"><div class="table-wrap"><table class="data-table">
      <thead><tr><th>编码</th><th>标题</th><th>站点</th><th>风险</th><th>状态</th><th>窗口</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="c in list" :key="c.id">
          <td class="mono"><a class="link" @click="openDetail(c)">{{ c.code }}</a></td>
          <td>{{ c.title }}</td><td class="muted">{{ siteName(c.site_id) }}</td>
          <td><span class="badge" :class="'risk-'+c.risk">{{ RISK[c.risk] }}</span></td>
          <td><span class="badge" :class="'cst-'+c.status">{{ CST[c.status] }}</span></td>
          <td class="muted">{{ fmt(c.window_start) }} ~ {{ fmt(c.window_end) }}</td>
          <td class="ops">
            <button class="btn btn-mini" @click="openImpact(c)" :title="c.cabinet_ids||c.power_node_ids?'评估变更波及范围':'需先在编辑中关联机柜/供电节点'">影响分析</button>
            <button v-if="c.status===1||c.status===4" class="btn btn-mini btn-mini-primary" @click="act(c,'submit','提审')">提审</button>
            <button v-if="c.status===2&&isAdmin" class="btn btn-mini btn-mini-primary" @click="act(c,'approve','批准')">批准</button>
            <button v-if="c.status===2&&isAdmin" class="btn btn-mini btn-mini-danger" @click="act(c,'reject','驳回')">驳回</button>
            <button v-if="c.status===3" class="btn btn-mini btn-mini-primary" @click="act(c,'execute','开始执行')">执行</button>
            <button v-if="c.status===5" class="btn btn-mini btn-mini-primary" @click="act(c,'finish','完成')">完成</button>
            <button v-if="c.status===5||c.status===6" class="btn btn-mini btn-mini-danger" @click="act(c,'rollback','回滚')">回滚</button>
            <button class="btn btn-mini" @click="openDetail(c)">详情</button>
            <button v-if="c.status===1" class="btn btn-mini" @click="openForm(c)">编辑</button>
            <button v-if="c.status===1" class="btn btn-mini btn-mini-danger" @click="del(c)">删除</button>
          </td>
        </tr>
        <tr v-if="!list.length"><td colspan="7" class="empty">暂无变更</td></tr>
      </tbody>
    </table></div></div>

    <!-- 详情抽屉 -->
    <div v-if="detail.show" class="drawer-mask" @click.self="detail.show=false"><div class="drawer">
      <div class="drawer-head"><h3>变更详情 · {{ detail.c?.code }}</h3><button class="btn btn-mini" @click="detail.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body" v-if="detail.c">
        <div class="detail-grid">
          <div><span class="dl">标题</span>{{ detail.c.title }}</div>
          <div><span class="dl">状态</span><span class="badge" :class="'cst-'+detail.c.status">{{ CST[detail.c.status] }}</span></div>
          <div><span class="dl">风险等级</span>{{ RISK[detail.c.risk] }}</div>
          <div><span class="dl">站点</span>{{ siteName(detail.c.site_id) }}</div>
          <div><span class="dl">变更窗口</span>{{ fmt(detail.c.window_start) }} ~ {{ fmt(detail.c.window_end) }}</div>
          <div><span class="dl">审批人</span>{{ userName(detail.c.approver_id) }}</div>
          <div><span class="dl">执行人</span>{{ userName(detail.c.executor_id) }}</div>
          <div><span class="dl">完成时间</span>{{ fmt(detail.c.finished_at) }}</div>
          <div class="full"><span class="dl">变更内容</span>{{ detail.c.description || '-' }}</div>
          <div class="full"><span class="dl">影响范围</span>{{ detail.c.impact || '-' }}</div>
          <div class="full"><span class="dl">实施方案</span>{{ detail.c.plan || '-' }}</div>
          <div class="full"><span class="dl">回退方案</span>{{ detail.c.rollback_plan || '-' }}</div>
          <div v-if="detail.c.approve_note" class="full"><span class="dl">审批意见</span>{{ detail.c.approve_note }}</div>
          <div v-if="detail.c.finish_note" class="full"><span class="dl">完成说明</span>{{ detail.c.finish_note }}</div>
        </div>
        <!-- B2-3 关联网络设备与配置备份 -->
        <fieldset class="fs" v-if="detail.bk&&detail.bk.network_devices.length" style="margin-top:14px"><legend><SvgIcon name="link" />关联网络设备（{{ detail.bk.network_devices.length }}）</legend>
          <div v-for="nd in detail.bk.network_devices" :key="nd.id" class="il-row">
            <b>{{ nd.name }}</b><span class="muted small mono">{{ nd.mgmt_ip||'-' }}</span>
            <router-link class="link small" :to="'/network-devices?id='+nd.id+'&detail=1'" @click="detail.show=false">设备详情</router-link>
          </div>
        </fieldset>
        <fieldset class="fs" v-if="detail.bk&&detail.bk.network_devices.length" style="margin-top:14px"><legend><SvgIcon name="layers" />配置备份记录（{{ detail.bk.config_backups.length }}）</legend>
          <div v-if="!detail.bk.config_backups.length" class="muted small" style="padding:4px 0">尚无备份记录——变更「开始执行」时将自动为每台关联设备生成备份。</div>
          <div v-for="b in detail.bk.config_backups" :key="b.id" class="il-row">
            <span class="badge" :class="'bkt-'+b.backup_type">{{ BKT[b.backup_type]||b.backup_type }}</span>
            <b class="mono small">{{ b.version||'-' }}</b>
            <span class="muted small">{{ fmt(b.backed_up_at) }} · {{ b.filename }}</span>
            <router-link class="link small" :to="'/config-backups?device_id='+b.device_id" @click="detail.show=false">备份页</router-link>
          </div>
        </fieldset>
      </div>
      <div class="drawer-foot"><button class="btn" @click="detail.show=false">关闭</button></div>
    </div></div>

    <!-- 编辑抽屉 -->
    <div v-if="form.show" class="drawer-mask" @click.self="form.show=false"><div class="drawer">
      <div class="drawer-head"><h3>{{ form.id?'编辑变更':'新建变更' }}</h3><button class="btn btn-mini" @click="form.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <fieldset class="fs"><legend><SvgIcon name="pin" />基本信息</legend>
          <div class="form-grid">
            <label class="full">标题 *<input class="input" v-model="form.title" /></label>
            <label>站点 *<select class="input" v-model.number="form.site_id"><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option></select></label>
            <label>风险等级 *<select class="input" v-model.number="form.risk"><option v-for="(n,k) in RISK" :key="k" :value="Number(k)">{{ n }}</option></select></label>
            <label>变更窗口开始<input class="input" type="datetime-local" v-model="form.window_start" /></label>
            <label>变更窗口结束<input class="input" type="datetime-local" v-model="form.window_end" /></label>
            <label class="full">变更内容 *<textarea class="input" rows="3" v-model="form.description" style="width:100%;box-sizing:border-box"></textarea></label>
            <label class="full">影响范围<textarea class="input" rows="2" v-model="form.impact" style="width:100%;box-sizing:border-box"></textarea></label>
            <label class="full">实施方案<textarea class="input" rows="2" v-model="form.plan" style="width:100%;box-sizing:border-box"></textarea></label>
            <label class="full">回退方案<textarea class="input" rows="2" v-model="form.rollback_plan" style="width:100%;box-sizing:border-box"></textarea></label>
          </div>
        </fieldset>
        <fieldset class="fs"><legend><SvgIcon name="link" />关联对象（用于影响预分析）</legend>
          <label class="full" style="font-size:13px;color:var(--muted);display:block;margin-bottom:6px">涉及机柜（勾选后保存，审批前可一键评估波及设备）</label>
          <div class="chip-wrap">
            <label v-for="cb in siteCabinets" :key="cb.id" class="chip" :class="{on: form.cabSel.includes(cb.id)}">
              <input type="checkbox" :value="cb.id" v-model="form.cabSel" />{{ cb.code }}
            </label>
            <span v-if="!siteCabinets.length" class="muted small">该站点暂无机柜</span>
          </div>
          <label class="full" style="font-size:13px;color:var(--muted);display:block;margin:10px 0 6px">涉及供电节点（UPS/PDU/支路，将递归评估下级波及）</label>
          <div class="chip-wrap">
            <label v-for="pc in sitePowerNodes" :key="pc.id" class="chip" :class="{on: form.pcSel.includes(pc.id)}">
              <input type="checkbox" :value="pc.id" v-model="form.pcSel" />{{ pc.name }}（{{ PCK[pc.kind]||pc.kind }}）
            </label>
            <span v-if="!sitePowerNodes.length" class="muted small">该站点暂无供电链路节点</span>
          </div>
          <label class="full" style="font-size:13px;color:var(--muted);display:block;margin:10px 0 6px">涉及网络设备（变更开始执行时自动触发配置备份，详情可查备份记录）</label>
          <div class="chip-wrap">
            <label v-for="nd in siteNetdevs" :key="nd.id" class="chip" :class="{on: form.ndSel.includes(nd.id)}">
              <input type="checkbox" :value="nd.id" v-model="form.ndSel" />{{ nd.name }}<span v-if="nd.mgmt_ip" class="muted small">（{{ nd.mgmt_ip }}）</span>
            </label>
            <span v-if="!siteNetdevs.length" class="muted small">该站点暂无网络设备</span>
          </div>
        </fieldset>
        <p v-if="form.err" class="err">{{ form.err }}</p>
      </div>
      <div class="drawer-foot"><button class="btn" @click="form.show=false">取消</button><button class="btn btn-primary" @click="save">保存</button></div>
    </div></div>

    <!-- V17 影响分析抽屉 -->
    <div v-if="impact.show" class="drawer-mask" @click.self="impact.show=false"><div class="drawer" style="width:640px">
      <div class="drawer-head"><h3>变更影响分析 · {{ impact.c?.code }}</h3><button class="btn btn-mini" @click="impact.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <div v-if="impact.loading" class="muted" style="padding:20px">加载中…</div>
        <template v-else-if="impact.d">
          <p v-if="!impact.d.cabinet_count&&!impact.d.power_nodes.length" class="muted" style="padding:8px 0">该变更未关联机柜/供电节点，请先编辑变更勾选涉及对象。</p>
          <div class="impact-kpis">
            <div class="ik"><b>{{ impact.d.cabinet_count }}</b><span>波及机柜</span></div>
            <div class="ik"><b>{{ impact.d.asset_count }}</b><span>波及设备</span></div>
            <div class="ik"><b>{{ (impact.d.total_power_w/1000).toFixed(1) }} kW</b><span>涉及负载</span></div>
            <div class="ik" :class="{'ik-warn': impact.riskMismatch}"><b>{{ RISK[impact.d.suggested_risk] }}</b><span>建议风险级{{ impact.riskMismatch?'（申报:'+RISK[impact.d.change.risk]+'）':'' }}</span></div>
          </div>
          <p v-if="impact.riskMismatch" class="err" style="margin:8px 0"><SvgIcon name="alert" />申报风险（{{ RISK[impact.d.change.risk] }}）低于按波及面推算的建议值（{{ RISK[impact.d.suggested_risk] }}），请审批时复核。</p>
          <fieldset class="fs" v-if="impact.d.power_nodes.length"><legend><SvgIcon name="bolt" />涉及供电节点</legend>
            <div v-for="n in impact.d.power_nodes" :key="n.id" class="il-row">{{ n.name }}（{{ PCK[n.kind]||n.kind }}）<span class="muted small" v-if="n.cabinet_id"> → 绑定机柜 #{{ n.cabinet_id }}</span></div>
          </fieldset>
          <fieldset class="fs" v-if="impact.d.cabinets.length"><legend><SvgIcon name="rack" />波及机柜（含供电节点下级递归）</legend>
            <div v-for="cb in impact.d.cabinets" :key="cb.id" class="il-row">
              <b class="mono">{{ cb.code }}</b>
              <span class="muted small">{{ cb.asset_cnt }} 台在架 · {{ cb.power_kw.toFixed(2) }} kW</span>
              <router-link class="link small" :to="'/capacity?cabinet_id='+cb.id" @click="impact.show=false">容量</router-link>
            </div>
          </fieldset>
          <fieldset class="fs" v-if="impact.d.assets.length"><legend><SvgIcon name="monitor" />波及设备（在架）</legend>
            <div class="table-wrap" style="max-height:280px;overflow-y:auto"><table class="data-table">
              <thead><tr><th>编码</th><th>名称</th><th>U位</th><th>功耗</th><th>IP</th></tr></thead>
              <tbody><tr v-for="a in impact.d.assets" :key="a.id"><td class="mono">{{ a.code }}</td><td>{{ a.name }}</td><td>{{ a.start_u }}U</td><td>{{ a.power_w }}W</td><td class="muted">{{ a.ip||'-' }}</td></tr></tbody>
            </table></div>
          </fieldset>
        </template>
      </div>
      <div class="drawer-foot"><button class="btn" @click="impact.show=false">关闭</button></div>
    </div></div>
  </div>
</template>

<script setup>
import {ref,reactive,computed,onMounted} from 'vue';import api from '../api'
import { showConfirm, showPrompt, toastAuto } from '../ui-dialog'
// 与后端一致: risk 1低/2中/3高; status 1草稿 2待审批 3已批准 4已驳回 5执行中 6已完成 7已回滚
const RISK={1:'低',2:'中',3:'高'}
const CST={1:'草稿',2:'待审批',3:'已批准',4:'已驳回',5:'执行中',6:'已完成',7:'已回滚'}
const PCK={utility:'市电',incoming:'总进线',transformer:'变压器',ups:'UPS',pdu:'PDU',busbar:'母线',branch:'支路',other:'其他'}
const BKT={manual:'手动',scheduled:'定时',change:'变更触发'}
const sites=ref([]),users=ref([]),list=ref([])
const allCabinets=ref([]),allPowerNodes=ref([]),allNetdevs=ref([])  // V17 关联对象选项 + B2-3 网络设备
const siteCabinets=computed(()=>allCabinets.value.filter(c=>!form.site_id||c.site_id===form.site_id))
const sitePowerNodes=computed(()=>allPowerNodes.value.filter(p=>!form.site_id||p.site_id===form.site_id))
const siteNetdevs=computed(()=>allNetdevs.value.filter(n=>!form.site_id||n.site_id===form.site_id))
const me=JSON.parse(localStorage.getItem('dcim_user')||'{}')
const isAdmin=computed(()=>me.role==='global_admin'||me.role==='site_admin')
const f=reactive({site_id:0,status:''})
const form=reactive({show:false,id:0,title:'',site_id:0,risk:2,window_start:'',window_end:'',description:'',impact:'',plan:'',rollback_plan:'',cabSel:[],pcSel:[],ndSel:[],err:''})
const detail=reactive({show:false,c:null,bk:null})
const impact=reactive({show:false,loading:false,c:null,d:null,riskMismatch:false})
async function openImpact(c){
  impact.c=c;impact.show=true;impact.loading=true;impact.d=null
  const r=await api.get(`/changes/${c.id}/impact`)
  impact.loading=false
  if(r.code!==0){toastAuto(r.msg);impact.show=false;return}
  impact.d=r.data
  impact.riskMismatch=(r.data.suggested_risk||1)>(r.data.change.risk||1)
}
const siteName=id=>(sites.value.find(s=>s.id===id)||{}).name||'-'
const userName=id=>{if(!id)return '-';const u=users.value.find(x=>x.id===id);return u?u.display_name:'用户#'+id}
const fmt=t=>t?String(t).replace('T',' ').slice(0,16):'-'
// datetime-local 值 <-> RFC3339
const toLocal=s=>s?String(s).replace('T',' ').slice(0,16).replace(' ','T'):''
const toRFC=s=>s?new Date(s).toISOString():''
async function load(){const params={};if(f.site_id)params.site_id=f.site_id;if(f.status)params.status=f.status;const r=await api.get('/changes',params);list.value=r.data?.items||[]}
async function act(c,action,label){
  let note=''
  if(action==='reject'||action==='rollback'){note=await showPrompt({ title: label, label: `${label}原因：` });if(note===null)return}
  else if(action==='approve'||action==='finish'){const v=await showPrompt({ title: label, label: `${label}说明（可选）：` });if(v===null)return;note=v}
  else if (!(await showConfirm({ message: `确认${label}变更 ${c.code}？` })))return
  const r=await api.post(`/changes/${c.id}/${action}`,{note})
  if(r.code===0){
    if(action==='execute'&&c.network_device_ids){
      const n=String(c.network_device_ids).split(',').filter(Boolean).length
      if(n)toastAuto('变更已开始执行，已触发 '+n+' 台网络设备配置备份')
    }
    load()
  }else toastAuto(r.msg)
}
function openDetail(c){detail.c=c;detail.bk=null;detail.show=true;api.get(`/changes/${c.id}/backups`).then(r=>{if(r.code===0)detail.bk=r.data})}
function openForm(c){if(c)Object.assign(form,{show:true,id:c.id,title:c.title,site_id:c.site_id,risk:c.risk||2,window_start:toLocal(c.window_start),window_end:toLocal(c.window_end),description:c.description||'',impact:c.impact||'',plan:c.plan||'',rollback_plan:c.rollback_plan||'',cabSel:(c.cabinet_ids||'').split(',').filter(Boolean).map(Number),pcSel:(c.power_node_ids||'').split(',').filter(Boolean).map(Number),ndSel:(c.network_device_ids||'').split(',').filter(Boolean).map(Number),err:''});else Object.assign(form,{show:true,id:0,title:'',site_id:sites.value[0]?.id||0,risk:2,window_start:'',window_end:'',description:'',impact:'',plan:'',rollback_plan:'',cabSel:[],pcSel:[],ndSel:[],err:''})}
async function save(){form.err='';if(!form.title||!form.site_id||!form.description){form.err='标题、站点、变更内容必填';return}
  const body={...form,cabinet_ids:form.cabSel.join(','),power_node_ids:form.pcSel.join(','),network_device_ids:form.ndSel.join(','),window_start:toRFC(form.window_start),window_end:toRFC(form.window_end)}
  const r=form.id?await api.put(`/changes/${form.id}`,body):await api.post('/changes',body)
  if(r.code===0){form.show=false;load()}else form.err=r.msg}
async function del(c){if (!(await showConfirm({ message: `删除变更「${c.title}」？` })))return;const r=await api.del(`/changes/${c.id}`);if(r.code===0)load();else toastAuto(r.msg)}
onMounted(async()=>{const[ss,uu,cbs,pcs,nds]=await Promise.all([api.get('/sites',{size:100}),api.get('/users-options'),api.get('/cabinets',{size:500}),api.get('/power-chains'),api.get('/network-devices',{size:200})]);sites.value=ss.data?.items||[];users.value=uu.data?.items||[];allCabinets.value=cbs.data?.items||[];allPowerNodes.value=Array.isArray(pcs.data)?pcs.data:(pcs.data?.items||[]);allNetdevs.value=nds.data?.items||[];form.site_id=sites.value[0]?.id||0;load()})
</script>
<style scoped>
.badge.risk-3{color:#f56c6c;border-color:rgba(245,108,108,.4)}.badge.risk-2{color:#e6a23c;border-color:rgba(230,162,60,.4)}.badge.risk-1{color:#909399;border-color:rgba(144,147,153,.4)}
.badge.cst-1{color:#909399;border-color:rgba(144,147,153,.4)}.badge.cst-2{color:#e6a23c;border-color:rgba(230,162,60,.4)}.badge.cst-3{color:#67c23a;border-color:rgba(103,194,58,.4)}.badge.cst-4{color:#f56c6c;border-color:rgba(245,108,108,.4)}.badge.cst-5{color:#409eff;border-color:rgba(64,158,255,.4)}.badge.cst-6{color:#16a34a;border-color:rgba(34,197,94,.4)}.badge.cst-7{color:#909399;border-color:rgba(144,147,153,.4)}
.badge.bkt-change{color:var(--accent);border-color:var(--accent)}.badge.bkt-manual{color:#909399;border-color:rgba(144,147,153,.4)}.badge.bkt-scheduled{color:#409eff;border-color:rgba(64,158,255,.4)}
.link{color:var(--accent);cursor:pointer;font-weight:600}.link:hover{text-decoration:underline}
.btn-mini-primary{color:var(--accent);border-color:var(--accent)}
.detail-grid{display:grid;grid-template-columns:1fr 1fr;gap:10px 16px;font-size:13.5px}.detail-grid .full{grid-column:1/-1}.dl{display:inline-block;min-width:80px;color:var(--muted)}
.drawer-mask{position:fixed;inset:0;background:rgba(0,0,0,.35);z-index:1000;display:flex;justify-content:flex-end}.drawer{width:600px;max-width:95vw;height:100%;background:var(--bg);box-shadow:-4px 0 20px rgba(0,0,0,.15);display:flex;flex-direction:column;overflow:hidden}.drawer-head{display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--border);flex-shrink:0}.drawer-head h3{margin:0;font-size:18px}.drawer-body{flex:1;overflow-y:auto;padding:20px}.drawer-foot{padding:14px 20px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;gap:10px;flex-shrink:0}
.fs{border:1px solid var(--border);border-radius:8px;padding:16px;margin-bottom:16px}.fs legend{font-size:14px;padding:0 8px;color:var(--accent)}
.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.form-grid label{display:flex;flex-direction:column;gap:4px;font-size:13px;color:var(--muted)}.form-grid label.full{grid-column:1/-1}
.err{color:#f56c6c;font-size:13px}
.chip-wrap{display:flex;flex-wrap:wrap;gap:6px}.chip{display:inline-flex;align-items:center;gap:4px;border:1px solid var(--border);border-radius:14px;padding:3px 10px;font-size:12.5px;cursor:pointer;color:var(--muted);user-select:none}.chip input{margin:0}.chip.on{color:var(--accent);border-color:var(--accent);background:color-mix(in srgb, var(--accent) 8%, transparent)}
.impact-kpis{display:grid;grid-template-columns:repeat(4,1fr);gap:10px;margin-bottom:12px}.ik{border:1px solid var(--border);border-radius:8px;padding:10px;text-align:center}.ik b{display:block;font-size:20px}.ik span{font-size:12px;color:var(--muted)}.ik-warn{border-color:#e6a23c}.ik-warn b{color:#e6a23c}
.il-row{display:flex;align-items:center;gap:8px;padding:5px 0;border-bottom:1px dashed var(--border);font-size:13px}.il-row:last-child{border-bottom:none}.il-row .muted{flex:1}
</style>
