<template>
  <div>
    <div class="page-head"><h1>配置备份</h1></div>
    <div class="toolbar">
      <select class="input" style="width:200px" v-model.number="f.device_id" @change="load"><option :value="0">全部设备</option><option v-for="d in devices" :key="d.id" :value="d.id">{{ d.name }} ({{ d.code }})</option></select>
      <span style="flex:1"></span>
      <button class="btn btn-primary" @click="openBackup()">＋ 手动备份</button>
    </div>
    <div class="card"><div class="table-wrap"><table class="data-table">
      <thead><tr><th>文件名</th><th>设备</th><th>版本</th><th>方式</th><th>大小</th><th>状态</th><th>备份时间</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="b in list" :key="b.id">
          <td class="mono">{{ b.filename }}</td>
          <td>{{ deviceName(b.device_id) }}</td>
          <td class="muted">{{ b.version||'-' }}</td>
          <td><span class="badge">{{ b.backup_type==='scheduled'?'定时':'手动' }}</span></td>
          <td class="mono">{{ fmtSize(b.size) }}</td>
          <td><span class="badge" :class="b.status?'st-1':'st-3'">{{ b.status?'成功':'失败' }}</span></td>
          <td class="muted">{{ fmt(b.backed_up_at) }}</td>
          <td class="ops"><button class="btn btn-mini" @click="view(b)">查看</button><button class="btn btn-mini" @click="download(b)">下载</button><button class="btn btn-mini btn-mini-danger" @click="del(b)">删除</button></td>
        </tr>
        <tr v-if="!list.length"><td colspan="8" class="empty">暂无备份记录</td></tr>
      </tbody>
    </table></div></div>

    <!-- 备份内容查看 -->
    <div v-if="viewer.show" class="drawer-mask" @click.self="viewer.show=false"><div class="drawer drawer-wide">
      <div class="drawer-head"><h3><SvgIcon name="doc" />{{ viewer.filename }}</h3><button class="btn btn-mini" @click="viewer.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body"><pre class="cfg-pre">{{ viewer.content }}</pre></div>
      <div class="drawer-foot"><button class="btn" @click="viewer.show=false">关闭</button></div>
    </div></div>

    <!-- 手动备份 -->
    <div v-if="form.show" class="drawer-mask" @click.self="form.show=false"><div class="drawer">
      <div class="drawer-head"><h3>手动备份配置</h3><button class="btn btn-mini" @click="form.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body"><fieldset class="fs"><legend><SvgIcon name="pin" />备份信息</legend><div class="form-grid">
        <label>设备 *<select class="input" v-model.number="form.device_id"><option v-for="d in devices" :key="d.id" :value="d.id">{{ d.name }} ({{ d.mgmt_ip||'无IP' }})</option></select></label>
        <label>版本标识<input class="input" v-model="form.version" placeholder="如 v1.2.3 或 变更前" /></label>
        <label class="full">配置内容 *<textarea class="input cfg-input" rows="14" v-model="form.content" placeholder="粘贴设备配置文本，或从设备导出后上传"></textarea></label>
      </div></fieldset>
      <p v-if="form.err" class="err">{{ form.err }}</p></div>
      <div class="drawer-foot"><button class="btn" @click="form.show=false">取消</button><button class="btn btn-primary" :disabled="form.saving" @click="save">{{ form.saving?'保存中...':'保存备份' }}</button></div>
    </div></div>
  </div>
</template>
<script setup>
import {ref,reactive,onMounted} from 'vue';import {useRoute} from 'vue-router';import api from '../api'
import { toastAuto, showConfirm } from '../ui-dialog'
const route=useRoute()
const devices=ref([]),list=ref([])
const f=reactive({device_id:0})
const form=reactive({show:false,saving:false,err:'',device_id:0,version:'',content:''})
const viewer=reactive({show:false,filename:'',content:''})
const deviceName=id=>(devices.value.find(d=>d.id===id)||{}).name||('设备#'+id)
const fmt=t=>t?String(t).replace('T',' ').slice(0,16):'-'
const fmtSize=n=>n>=1024?(n/1024).toFixed(1)+' KB':n+' B'
async function load(){const params={};if(f.device_id)params.device_id=f.device_id;const r=await api.get('/config-backups',params);list.value=r.data?.items||[]}
function openBackup(){form.err='';Object.assign(form,{show:true,device_id:f.device_id||devices.value[0]?.id||0,version:'',content:''})}
async function save(){form.err=''
  if(!form.device_id){form.err='请选择设备';return}
  if(!form.content.trim()){form.err='配置内容不能为空';return}
  form.saving=true
  const r=await api.post('/config-backups',{device_id:form.device_id,content:form.content,version:form.version})
  form.saving=false
  if(r.code===0){form.show=false;load()}else form.err=r.msg||'备份失败'}
async function view(b){const r=await api.get(`/config-backups/${b.id}`)
  if(r.code===0){viewer.show=true;viewer.filename=b.filename;viewer.content=r.data?.content||'(空)'}else toastAuto(r.msg||'加载失败')}
function download(b){
  api.get(`/config-backups/${b.id}`).then(r=>{
    const content=r.data?.content||''
    const url=URL.createObjectURL(new Blob([content],{type:'text/plain;charset=utf-8'}))
    const a=document.createElement('a');a.href=url;a.download=b.filename;a.click();URL.revokeObjectURL(url)})}
async function del(b){if (!(await showConfirm({ message: `删除备份 ${b.filename}？` })))return;const r=await api.del(`/config-backups/${b.id}`);if(r.code===0)load();else toastAuto(r.msg||'删除失败')}
onMounted(async()=>{const r=await api.get('/network-devices',{size:200});devices.value=r.data?.items||[]
  const qd=+route.query.device_id||0;if(qd)f.device_id=qd
  load()})
</script>
<style scoped>
.badge.st-1{color:#67c23a;border-color:rgba(103,194,58,.4)}.badge.st-3{color:#f56c6c;border-color:rgba(245,108,108,.4)}
.drawer-wide{width:860px}
.cfg-pre{background:var(--panel,#0d1526);border:1px solid var(--border);border-radius:6px;padding:14px;font-size:12px;line-height:1.6;white-space:pre-wrap;word-break:break-all;font-family:monospace;margin:0}
.cfg-input{font-family:monospace;font-size:12px;resize:vertical;min-height:200px}
.err{color:#f56c6c;font-size:13px;margin:8px 0 0}
.drawer-mask{position:fixed;inset:0;background:rgba(0,0,0,.35);z-index:1000;display:flex;justify-content:flex-end}.drawer{width:600px;max-width:95vw;height:100%;background:var(--bg);box-shadow:-4px 0 20px rgba(0,0,0,.15);display:flex;flex-direction:column;overflow:hidden}.drawer-head{display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--border);flex-shrink:0}.drawer-head h3{margin:0;font-size:18px}.drawer-body{flex:1;overflow-y:auto;padding:20px}.drawer-foot{padding:14px 20px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;gap:10px;flex-shrink:0}
.fs{border:1px solid var(--border);border-radius:8px;padding:16px;margin-bottom:16px}.fs legend{font-weight:600;font-size:14px;padding:0 8px;color:var(--accent)}.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.form-grid label{display:flex;flex-direction:column;gap:4px;font-size:13px;color:var(--muted)}.form-grid label.full{grid-column:1/-1}
</style>
