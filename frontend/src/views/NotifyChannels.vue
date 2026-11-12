<template>
  <div>
    <div class="page-head"><h1>通知通道</h1></div>
    <div class="toolbar">
      <span class="muted" style="font-size:13px">告警 / SLA 违约 / 派单等事件实时外发到企业微信、飞书、钉钉群机器人或自定义 Webhook</span>
      <span style="flex:1"></span>
      <button class="btn btn-primary" @click="openForm()">＋ 新建通道</button>
    </div>

    <div class="card"><div class="table-wrap"><table class="data-table">
      <thead><tr><th>名称</th><th>类型</th><th>订阅事件</th><th>状态</th><th>最近发送</th><th>备注</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="ch in list" :key="ch.id">
          <td><b>{{ ch.name }}</b></td>
          <td><span class="badge" :class="'k-'+ch.kind">{{ KINDS[ch.kind]||ch.kind }}</span></td>
          <td class="evt-cell">{{ evtText(ch.events) }}</td>
          <td>
            <label class="sw"><input type="checkbox" :checked="ch.enabled" @change="toggle(ch,$event)" /><span>启用</span></label>
            <div v-if="ch.last_status" class="ls" :class="ch.last_status==='ok'?'ls-ok':'ls-fail'">{{ ch.last_status==='ok'?'正常':ch.last_status }}</div>
          </td>
          <td class="muted">{{ fmt(ch.last_sent_at) }}</td>
          <td class="muted">{{ ch.remark||'-' }}</td>
          <td class="ops">
            <button class="btn btn-mini" @click="test(ch)" :disabled="testing===ch.id">{{ testing===ch.id?'发送中…':'测试' }}</button>
            <button class="btn btn-mini" @click="openForm(ch)">编辑</button>
            <button class="btn btn-mini btn-danger" @click="del(ch)">删除</button>
          </td>
        </tr>
        <tr v-if="!list.length"><td colspan="7" class="empty">暂无通道，点击右上角新建</td></tr>
      </tbody>
    </table></div></div>

    <!-- 事件说明卡 -->
    <div class="card" style="margin-top:14px">
      <div class="card-title"><SvgIcon name="signal" />可订阅事件</div>
      <div class="evt-grid">
        <div v-for="e in EVENTS" :key="e.k" class="evt-item"><span class="badge">{{ e.k }}</span> {{ e.label }}</div>
      </div>
    </div>

    <!-- 编辑抽屉 -->
    <div v-if="form.show" class="drawer-mask" @click.self="form.show=false"><div class="drawer">
      <div class="drawer-head"><h3>{{ form.id?'编辑通道':'新建通道' }}</h3><button class="btn btn-mini" @click="form.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <fieldset class="fs"><legend><SvgIcon name="pin" />基本信息</legend>
          <div class="form-grid">
            <label class="full">通道名称 *<input class="input" v-model="form.name" placeholder="如：运维值班群" /></label>
            <label>通道类型 *<select class="input" v-model="form.kind">
              <option v-for="(n,k) in KINDS" :key="k" :value="k">{{ n }}</option>
            </select></label>
            <label>启用<input class="input" type="checkbox" v-model="form.enabled" style="width:auto;margin-top:8px" /></label>
            <label class="full">Webhook 地址 *<input class="input" v-model="form.webhook_url" placeholder="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR-KEY" /></label>
            <label class="full">备注<input class="input" v-model="form.remark" placeholder="用途说明（可选）" /></label>
          </div>
        </fieldset>
        <fieldset class="fs"><legend><SvgIcon name="bell" />订阅事件</legend>
          <label class="chk-all"><input type="checkbox" :checked="form.events==='*'" @change="form.events = $event.target.checked?'*':''" /> 全部事件</label>
          <div class="evt-checks">
            <label v-for="e in EVENTS" :key="e.k" class="chk"><input type="checkbox" :value="e.k" v-model="form.evtList" :disabled="form.events==='*'" /> {{ e.label }}</label>
          </div>
        </fieldset>
        <div class="tip"><SvgIcon name="bulb" />{{ KIND_HINT[form.kind] }}</div>
        <p v-if="form.err" class="err">{{ form.err }}</p>
      </div>
      <div class="drawer-foot"><button class="btn" @click="form.show=false">取消</button><button class="btn btn-primary" @click="save">保存</button></div>
    </div></div>
  </div>
</template>
<script setup>
import {ref,reactive,onMounted} from 'vue'
import api from '../api'
import { toastAuto, showConfirm } from '../ui-dialog'
const KINDS={wecom:'企业微信',feishu:'飞书',dingtalk:'钉钉',webhook:'通用 Webhook'}
const KIND_HINT={
  wecom:'企业微信群机器人：群设置 → 添加群机器人 → 复制 Webhook 地址。消息以 markdown 卡片发送。',
  feishu:'飞书群自定义机器人：群设置 → 群机器人 → 添加自定义机器人 → 复制 Webhook 地址。安全设置建议选「自定义关键词」并填入 DCIM。',
  dingtalk:'钉钉群机器人：群设置 → 智能群助手 → 添加自定义机器人。安全设置建议选「自定义关键词」并填入 DCIM。',
  webhook:'通用 Webhook：POST JSON {event,title,content,source,timestamp,...扩展字段}，适合自建接收端。'
}
const EVENTS=[
  {k:'alarm',label:'告警触发'},
  {k:'alarm_resolve',label:'告警自动恢复'},
  {k:'alarm_escalation',label:'告警升级'},
  {k:'work_order',label:'工单创建/改派'},
  {k:'sla_warn',label:'SLA 即将超时'},
  {k:'sla_breach',label:'SLA 违约'},
  {k:'maintenance_due',label:'维保到期'},
  {k:'asset_health',label:'设备健康预警/恢复'},
  {k:'power_redundancy',label:'供电冗余预警/恢复'},
  {k:'capacity_forecast',label:'容量饱和预警/解除'},
  {k:'drill_overdue',label:'应急预案演练逾期'},
  {k:'asset_retire',label:'资产退役申请/审批/处置'},
  {k:'ops_report',label:'运维月报生成/发布'},
  {k:'inspection_missed',label:'巡检漏检判定'},
  {k:'alert_quality',label:'告警规则噪声预警'},
  {k:'pue_compliance',label:'PUE 合规告警/恢复'}
]
const list=ref([]),testing=ref(0)
const form=reactive({show:false,id:0,name:'',kind:'wecom',webhook_url:'',events:'*',evtList:[],enabled:true,remark:'',err:''})
const fmt=t=>t?String(t).replace('T',' ').slice(0,16):'-'
const evtText=ev=>{if(!ev||ev==='*')return '全部事件';const ks=String(ev).split(',');return ks.map(k=>(EVENTS.find(e=>e.k===k.trim())||{label:k}).label).join('、')}
async function load(){const r=await api.get('/notify-channels');list.value=r.data?.items||[]}
function openForm(ch){
  if(ch){const evts=(!ch.events||ch.events==='*')?[]:String(ch.events).split(',').map(s=>s.trim())
    Object.assign(form,{show:true,id:ch.id,name:ch.name,kind:ch.kind,webhook_url:ch.webhook_url,events:ch.events||'*',evtList:evts,enabled:ch.enabled,remark:ch.remark||'',err:''})}
  else Object.assign(form,{show:true,id:0,name:'',kind:'wecom',webhook_url:'',events:'*',evtList:[],enabled:true,remark:'',err:''})
}
async function save(){
  form.err=''
  if(!form.name||!form.webhook_url){form.err='名称与 Webhook 地址必填';return}
  const events=form.events==='*'?'*':form.evtList.join(',')
  if(events===''){form.err='请至少订阅一个事件';return}
  const body={name:form.name,kind:form.kind,webhook_url:form.webhook_url,events,enabled:form.enabled,remark:form.remark}
  const r=form.id?await api.put(`/notify-channels/${form.id}`,body):await api.post('/notify-channels',body)
  if(r.code===0){form.show=false;load()}else form.err=r.msg
}
async function toggle(ch,e){
  const r=await api.put(`/notify-channels/${ch.id}`,{enabled:e.target.checked})
  if(r.code!==0){e.target.checked=ch.enabled;toastAuto(r.msg)}else ch.enabled=e.target.checked
}
async function test(ch){
  testing.value=ch.id
  const r=await api.post(`/notify-channels/${ch.id}/test`,{})
  testing.value=0
  if(r.code===0&&r.data?.ok)toastAuto('测试消息发送成功，请到群里确认')
  else toastAuto('发送失败：'+(r.data?.error||r.msg||'未知错误'))
  load()
}
async function del(ch){
  if (!(await showConfirm({ message: `删除通道「${ch.name}」？` })))return
  const r=await api.del(`/notify-channels/${ch.id}`)
  if(r.code===0)load();else toastAuto(r.msg)
}
onMounted(load)
</script>
<style scoped>
.evt-cell{max-width:260px;font-size:12.5px;color:var(--text2)}
.sw{display:inline-flex;align-items:center;gap:5px;cursor:pointer;font-size:13px}
.ls{font-size:11px;margin-top:3px;max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.ls-ok{color:#67c23a}.ls-fail{color:#f56c6c}
.badge.k-wecom{color:#07c160;border-color:rgba(7,193,96,.4)}
.badge.k-feishu{color:#409eff;border-color:rgba(64,158,255,.4)}
.badge.k-dingtalk{color:#1677ff;border-color:rgba(22,119,255,.4)}
.badge.k-webhook{color:#909399;border-color:rgba(144,147,153,.4)}
.evt-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(230px,1fr));gap:8px}
.evt-item{font-size:13px;color:var(--text2);display:flex;align-items:center;gap:8px}
.evt-checks{display:grid;grid-template-columns:1fr 1fr;gap:8px;margin-top:8px}
.chk{display:flex;align-items:center;gap:6px;font-size:13px;color:var(--text2);cursor:pointer}
.chk-all{display:flex;align-items:center;gap:6px;font-size:13.5px;font-weight:600;cursor:pointer}
.tip{font-size:12.5px;color:var(--muted);background:var(--bg2);border-radius:8px;padding:10px 12px;line-height:1.6}
.btn-danger{color:#f56c6c;border-color:rgba(245,108,108,.4)}
.card-title{font-size:14px;font-weight:600;margin-bottom:10px}
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
@media (max-width:900px){.evt-checks{grid-template-columns:1fr}}
</style>
