<template>
  <div><div class="page-head"><h1>VLAN 管理</h1></div>
    <div class="toolbar"><span style="flex:1"></span><button class="btn btn-primary" @click="openForm()">＋ 新建VLAN</button></div>
    <div class="card"><div class="table-wrap"><table class="data-table">
      <thead><tr><th>VID</th><th>名称</th><th>子网</th><th>站点</th><th>颜色</th><th>操作</th></tr></thead>
      <tbody><tr v-for="v in list" :key="v.id"><td class="mono">{{ v.vid }}</td><td><span :style="{color:v.color}">●</span> {{ v.name }}</td><td class="mono">{{ v.subnet }}</td><td class="muted">{{ siteName(v.site_id) }}</td><td><span class="color-dot" :style="{background:v.color}"></span></td><td class="ops"><button class="btn btn-mini" @click="openForm(v)">编辑</button><button class="btn btn-mini btn-mini-danger" @click="del(v)">删除</button></td></tr>
      <tr v-if="!list.length"><td colspan="6" class="empty">暂无</td></tr></tbody>
    </table></div></div>
    <div v-if="form.show" class="drawer-mask" @click.self="form.show=false"><div class="drawer"><div class="drawer-head"><h3>{{ form.id?'编辑':'新建' }}VLAN</h3><button class="btn btn-mini" @click="form.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body"><fieldset class="fs"><legend><SvgIcon name="pin" />基本信息</legend><div class="form-grid">
        <label>VID *<input class="input" type="number" min="1" max="4094" v-model.number="form.vid" /></label><label>名称 *<input class="input" v-model="form.name" /></label>
        <label>站点 *<select class="input" v-model.number="form.site_id"><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option></select></label>
        <label>子网<input class="input" v-model="form.subnet" placeholder="192.168.1.0/24" /></label>
        <label>颜色<input class="input" type="color" v-model="form.color" style="height:36px;padding:2px" /></label><label>描述<input class="input" v-model="form.description" /></label>
      </div></fieldset></div>
      <div class="drawer-foot"><button class="btn" @click="form.show=false">取消</button><button class="btn btn-primary" @click="save">保存</button></div>
    </div></div>
  </div>
</template>
<script setup>
import {ref,reactive,onMounted} from 'vue';import api from '../api'
import { toastAuto, showConfirm } from '../ui-dialog'
const sites=ref([]),list=ref([]);const form=reactive({show:false,id:0,vid:1,name:'',site_id:0,subnet:'',color:'#409eff',description:''})
const siteName=id=>(sites.value.find(s=>s.id===id)||{}).name||'-'
async function load(){const r=await api.get('/vlans',{size:200});list.value=r.data?.items||[]}
function openForm(v){if(v)Object.assign(form,{show:true,id:v.id,vid:v.vid,name:v.name,site_id:v.site_id,subnet:v.subnet||'',color:v.color||'#409eff',description:v.description||''});else Object.assign(form,{show:true,id:0,vid:1,name:'',site_id:sites.value[0]?.id||0,subnet:'',color:'#409eff',description:''})}
async function save(){const r=form.id?await api.put(`/vlans/${form.id}`,{...form}):await api.post('/vlans',{...form});if(r.code===0){form.show=false;load()}else toastAuto(r.msg)}
async function del(v){if (!(await showConfirm({ message: '删除' })))return;await api.del(`/vlans/${v.id}`);load()}
onMounted(async()=>{const r=await api.get('/sites',{size:100});sites.value=r.data?.items||[];form.site_id=sites.value[0]?.id||0;load()})
</script>
<style scoped>
.color-dot{display:inline-block;width:14px;height:14px;border-radius:50%}
.drawer-mask{position:fixed;inset:0;background:rgba(0,0,0,.35);z-index:1000;display:flex;justify-content:flex-end}.drawer{width:600px;max-width:95vw;height:100%;background:var(--bg);box-shadow:-4px 0 20px rgba(0,0,0,.15);display:flex;flex-direction:column;overflow:hidden}.drawer-head{display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--border);flex-shrink:0}.drawer-head h3{margin:0;font-size:18px}.drawer-body{flex:1;overflow-y:auto;padding:20px}.drawer-foot{padding:14px 20px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;gap:10px;flex-shrink:0}
.fs{border:1px solid var(--border);border-radius:8px;padding:16px;margin-bottom:16px}.fs legend{font-weight:600;font-size:14px;padding:0 8px;color:var(--accent)}.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.form-grid label{display:flex;flex-direction:column;gap:4px;font-size:13px;color:var(--muted)}
</style>