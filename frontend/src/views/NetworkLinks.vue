<template>
  <div><PageBar label="链路台账" parent-label="网络拓扑" parent-to="/network-topology" />
    <div class="toolbar">
      <select class="input" style="width:140px" v-model.number="f.site_id" @change="load"><option :value="0">全部站点</option><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option></select>
      <select class="input" style="width:130px" v-model="f.status" @change="load"><option value="">全部状态</option><option value="1">UP</option><option value="0">DOWN</option></select>
      <span style="flex:1"></span>
      <span class="muted small">共 {{ filtered.length }} 条链路</span>
    </div>
    <div class="card"><div class="table-wrap"><table class="data-table">
      <thead><tr><th>ID</th><th>本端（设备 / 端口）</th><th>对端（设备 / 端口）</th><th>类型</th><th>带宽</th><th>状态</th><th>标签</th><th>操作</th></tr></thead>
      <tbody><tr v-for="l in filtered" :key="l.id">
        <td class="muted">{{ l.id }}</td>
        <td>{{ endLabel(l.port_a_id) }}</td>
        <td>{{ endLabel(l.port_b_id) }}</td>
        <td><span class="badge">{{ LT[l.link_type] || l.link_type || '-' }}</span></td>
        <td>{{ l.bandwidth || '-' }}</td>
        <td><span class="badge" :class="l.status===1?'lk-up':'lk-down'">{{ l.status===1?'UP':'DOWN' }}</span></td>
        <td class="muted">{{ l.label || '-' }}</td>
        <td class="ops"><button class="btn btn-mini" @click="openForm(l)">编辑</button></td>
      </tr>
      <tr v-if="!filtered.length"><td colspan="8" class="empty">暂无链路</td></tr></tbody>
    </table></div></div>

    <div v-if="form.show" class="drawer-mask" @click.self="form.show=false"><div class="drawer">
      <div class="drawer-head"><h3>编辑链路 #{{ form.id }}</h3><button class="btn btn-mini" @click="form.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body"><fieldset class="fs"><legend><SvgIcon name="link" />连接两端</legend><div class="form-grid">
        <label>本端端口<select class="input" v-model.number="form.port_a_id"><option v-for="p in ports" :key="p.id" :value="p.id">{{ portLabel(p) }}</option></select></label>
        <label>对端端口<select class="input" v-model.number="form.port_b_id"><option v-for="p in ports" :key="p.id" :value="p.id">{{ portLabel(p) }}</option></select></label>
      </div></fieldset>
      <fieldset class="fs"><legend><SvgIcon name="gear" />链路属性</legend><div class="form-grid">
        <label>类型<select class="input" v-model="form.link_type"><option value="ethernet">以太网</option><option value="fiber">光纤</option><option value="trunk">Trunk聚合</option></select></label>
        <label>带宽<select class="input" v-model="form.bandwidth"><option value="">未指定</option><option v-for="b in ['100M','1G','10G','40G','100G']" :key="b" :value="b">{{ b }}</option></select></label>
        <label>状态<select class="input" v-model.number="form.status"><option :value="1">UP</option><option :value="0">DOWN</option></select></label>
        <label class="full">标签<input class="input" v-model="form.label" placeholder="如：核心-汇聚 主链路" /></label>
      </div></fieldset>
      <p v-if="form.err" class="err">{{ form.err }}</p></div>
      <div class="drawer-foot"><button class="btn" @click="form.show=false">取消</button><button class="btn btn-primary" @click="save">保存</button></div>
    </div></div>
  </div>
</template>
<script setup>
import {ref,reactive,computed,onMounted} from 'vue'
import api from '../api';import PageBar from '../components/PageBar.vue'
const LT={ethernet:'以太网',fiber:'光纤',trunk:'Trunk'}
const sites=ref([]),links=ref([]),ports=ref([]),devices=ref([])
const f=reactive({site_id:0,status:''})
const form=reactive({show:false,id:0,port_a_id:0,port_b_id:0,link_type:'ethernet',bandwidth:'',status:1,label:'',err:''})
const filtered=computed(()=>f.status===''?links.value:links.value.filter(l=>String(l.status)===f.status))
const portLabel=p=>`${(devices.value.find(d=>d.id===p.device_id)||{}).name||('设备#'+p.device_id)} / ${p.name}`
function endLabel(pid){const p=ports.value.find(x=>x.id===pid);return p?portLabel(p):('端口#'+pid)}
async function load(){const params={};if(f.site_id)params.site_id=f.site_id;const r=await api.get('/network-links',params);links.value=r.data?.items||[]}
function openForm(l){Object.assign(form,{show:true,id:l.id,port_a_id:l.port_a_id,port_b_id:l.port_b_id,link_type:l.link_type||'ethernet',bandwidth:l.bandwidth||'',status:l.status,label:l.label||'',err:''})}
async function save(){const r=await api.put(`/network-links/${form.id}`,{port_a_id:form.port_a_id,port_b_id:form.port_b_id,link_type:form.link_type,bandwidth:form.bandwidth,status:form.status,label:form.label});if(r.code===0){form.show=false;load()}else form.err=r.msg}
onMounted(async()=>{
  const [sr,tr]=await Promise.all([api.get('/sites',{size:100}),api.get('/network-topology')])
  sites.value=sr.data?.items||[]
  ports.value=tr.data?.ports||[];devices.value=tr.data?.devices||[]
  load()
})
</script>
<style scoped>
.badge.lk-up{color:#67c23a;border-color:rgba(103,194,58,.4)}.badge.lk-down{color:#f56c6c;border-color:rgba(245,108,108,.4)}
.drawer-mask{position:fixed;inset:0;background:rgba(0,0,0,.35);z-index:1000;display:flex;justify-content:flex-end}.drawer{width:600px;max-width:95vw;height:100%;background:var(--bg);box-shadow:-4px 0 20px rgba(0,0,0,.15);display:flex;flex-direction:column;overflow:hidden}.drawer-head{display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--border);flex-shrink:0}.drawer-head h3{margin:0;font-size:18px}.drawer-body{flex:1;overflow-y:auto;padding:20px}.drawer-foot{padding:14px 20px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;gap:10px;flex-shrink:0}
.fs{border:1px solid var(--border);border-radius:8px;padding:16px;margin-bottom:16px}.fs legend{font-weight:600;font-size:14px;padding:0 8px;color:var(--accent)}.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.form-grid label{display:flex;flex-direction:column;gap:4px;font-size:13px;color:var(--muted)}.form-grid label.full{grid-column:1/-1}
.err{color:#f56c6c;font-size:13px}
</style>
