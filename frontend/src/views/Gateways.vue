<template>
  <div><div class="page-head"><h1>站点网关</h1>
    <div style="display:flex;align-items:center;gap:8px">
      <span class="live-dot" :class="list.some(g=>g.online)?'live-on':''"></span>
      <span class="muted small">{{ onlineCnt }}/{{ list.length }} 在线 · 30s 刷新</span>
    </div>
  </div>

    <!-- V16 运行统计条 -->
    <div class="stat-bar">
      <div class="sb-item"><b>{{ stats.gateway_total||0 }}</b><span>注册网关</span></div>
      <div class="sb-item"><b :class="stats.gateway_online>0?'c-ok':'c-off'">{{ stats.gateway_online||0 }}</b><span>在线</span></div>
      <div class="sb-item"><b>{{ stats.point_total||0 }}</b><span>接入测点</span></div>
      <div class="sb-item"><b>{{ fmtN(stats.readings_24h) }}</b><span>24h读数</span></div>
      <div class="sb-item"><b class="small-b">{{ stats.latest_reading?fmt(stats.latest_reading):'-' }}</b><span>最新推送</span></div>
      <div class="sb-item">
        <b :class="stats.simulate_on?'c-warn':'c-ok'" style="font-size:13px">{{ stats.simulate_on?'模拟器运行中':'真实数据' }}</b>
        <span>数据来源</span>
      </div>
    </div>

    <div class="toolbar"><span style="flex:1"></span><button class="btn btn-primary" @click="openForm()">＋ 注册网关</button></div>
    <div class="card"><div class="table-wrap"><table class="data-table">
      <thead><tr><th>编码</th><th>名称</th><th>站点</th><th>IP</th><th>版本</th><th>在线</th><th>最后心跳</th><th>操作</th></tr></thead>
      <tbody><tr v-for="g in list" :key="g.id">
        <td class="mono">{{ g.code }}</td><td>{{ g.name }}</td><td class="muted">{{ siteName(g.site_id) }}</td>
        <td class="mono">{{ g.ip||'-' }}</td><td>{{ g.version||'-' }}</td>
        <td><span class="badge" :class="g.online?'st-1':'st-0'">{{ g.online?'在线':'离线' }}</span>
            <span v-if="!g.online&&g.last_seen_at" class="muted small"> ({{ ago(g.last_seen_at) }})</span></td>
        <td class="muted">{{ fmt(g.last_seen_at) }}</td>
        <td class="ops">
          <button class="btn btn-mini" @click="openMon(g)" title="边缘 DI/DO 实时监控与继电器控制"><SvgIcon name="eye" />监控</button>
          <button class="btn btn-mini" @click="showSecret(g)" title="查看/重置接入密钥"><SvgIcon name="key" />密钥</button>
          <button class="btn btn-mini" @click="openForm(g)">编辑</button>
          <button class="btn btn-mini btn-mini-danger" @click="del(g)">删除</button>
        </td>
      </tr>
      <tr v-if="!list.length"><td colspan="8" class="empty">暂无网关，注册后边缘网关即可通过 code+secret 接入推送读数</td></tr></tbody>
    </table></div></div>

    <!-- 注册/编辑抽屉 -->
    <div v-if="form.show" class="drawer-mask" @click.self="form.show=false"><div class="drawer"><div class="drawer-head"><h3>{{ form.id?'编辑':'注册' }}网关</h3><button class="btn btn-mini" @click="form.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body"><fieldset class="fs"><legend><SvgIcon name="pin" />基本信息</legend><div class="form-grid">
        <label>编码 *<input class="input" v-model="form.code" :disabled="!!form.id" placeholder="如 GW-BJ-01" /></label>
        <label>名称<input class="input" v-model="form.name" /></label>
        <label>站点 *<select class="input" v-model.number="form.site_id"><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option></select></label>
        <label>IP<input class="input" v-model="form.ip" placeholder="网关所在主机IP" /></label>
        <label>版本<input class="input" v-model="form.version" /></label>
        <label>状态<select class="input" v-model.number="form.status"><option :value="1">启用</option><option :value="0">停用</option></select></label>
        <label class="full">备注<input class="input" v-model="form.remark" /></label>
      </div></fieldset>
      <p v-if="!form.id" class="muted small">注册成功后将生成接入密钥（secret），请立即复制保存——密钥仅在创建/重置时展示一次。</p>
      </div>
      <div class="drawer-foot"><button class="btn" @click="form.show=false">取消</button><button class="btn btn-primary" @click="save">保存</button></div>
    </div></div>

    <!-- V16 密钥抽屉（创建成功/点击密钥/重置后展示） -->
    <div v-if="sec.show" class="drawer-mask" @click.self="sec.show=false"><div class="drawer" style="width:520px">
      <div class="drawer-head"><h3><SvgIcon name="key" />网关接入密钥 — {{ sec.code }}</h3><button class="btn btn-mini" @click="sec.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <div v-if="sec.value" class="sec-box">
          <div class="sec-warn"><SvgIcon name="alert" />密钥仅此一次展示，关闭后无法再查看。请立即复制并妥善保管。</div>
          <div class="sec-val mono">{{ sec.value }}</div>
          <button class="btn btn-primary" @click="copySecret">{{ copied?'已复制':'复制密钥' }}</button>
        </div>
        <div v-else class="sec-box">
          <div class="muted" style="margin-bottom:12px">密钥不可查看（安全设计）。如已丢失，请重置生成新密钥——<b class="warn-text">重置后旧密钥立即失效</b>，使用旧密钥的边缘网关需同步更新配置。</div>
          <button class="btn btn-mini btn-mini-danger" @click="doReset"><SvgIcon name="refresh" />重置密钥</button>
        </div>
        <fieldset class="fs" style="margin-top:16px"><legend><SvgIcon name="book" />接入方法</legend>
          <div class="muted small" style="line-height:1.8">
            1. 部署包 <span class="mono">bin/dcim-gateway</span> 拷贝到边缘侧主机<br>
            2. 复制 <span class="mono">configs/gateway.example.yaml</span> 为 <span class="mono">gateway.yaml</span><br>
            3. 填入 server（本系统地址）、code=<b>{{ sec.code }}</b>、secret=上方密钥<br>
            4. 运行 <span class="mono">./dcim-gateway -c gateway.yaml</span>，注册成功后本页显示「在线」
          </div>
        </fieldset>
      </div>
    </div></div>

    <!-- V47: 边缘监控抽屉（XM-E16 DI/DO 实时状态 + 继电器控制 + 命令日志） -->
    <div v-if="mon.show" class="drawer-mask" @click.self="closeMon()"><div class="drawer" style="width:640px">
      <div class="drawer-head"><h3><SvgIcon name="eye" />边缘监控 — {{ mon.code }}</h3><button class="btn btn-mini" @click="closeMon()"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <div v-if="!mon.rt" class="muted" style="padding:20px 0">暂无运行时数据——该网关尚未部署 dcim-edge 采集器，或离线超过一个心跳周期。部署方法见「密钥」抽屉的接入说明（edge 程序同包提供）。</div>
        <template v-else>
          <fieldset class="fs"><legend><SvgIcon name="signal" />运行状态</legend>
            <div class="mon-stats">
              <div class="mon-stat"><span class="muted">运行时长</span><b>{{ fmtUp(mon.rt.uptime_sec) }}</b></div>
              <div class="mon-stat"><span class="muted">采集测点</span><b>{{ mon.rt.edge_points }}</b></div>
              <div class="mon-stat"><span class="muted">上报积压</span><b :class="mon.rt.push_fail>0?'warn-text':'ok-text'">{{ mon.rt.push_fail }}</b></div>
              <div class="mon-stat"><span class="muted">占用串口</span><b class="mono">{{ mon.rt.serials || '-' }}</b></div>
            </div>
          </fieldset>
          <fieldset class="fs"><legend><SvgIcon name="bolt" />DI 开关量输入（12路）</legend>
            <div class="io-grid">
              <div v-for="n in 12" :key="n" class="io-cell" :class="{on: diBit(n-1)}">
                <span class="io-led" :class="{on: diBit(n-1)}"></span>DI{{ n }}
                <em>{{ diBit(n-1) ? (n<=4?'漏水':n<=8?'烟感':'门磁') + '触发' : '正常' }}</em>
              </div>
            </div>
            <div class="muted small" style="margin-top:8px">亮灯 = 端子短接（干接点闭合），对应测点将按告警规则判断；标签仅供参考，实际含义以测点配置为准。</div>
          </fieldset>
          <fieldset class="fs"><legend><SvgIcon name="gear" />DO 继电器输出（4路）</legend>
            <div class="io-grid">
              <div v-for="n in 4" :key="n" class="io-cell io-do" :class="{on: doBit(n-1)}">
                <span class="io-led" :class="{on: doBit(n-1)}"></span>DO{{ n }} {{ doBit(n-1)?'闭合':'断开' }}
                <button class="btn btn-mini" :disabled="mon.busy" @click="doCmd(n, doBit(n-1)?'off':'on')">
                  {{ doBit(n-1) ? '断开' : '闭合' }}
                </button>
              </div>
            </div>
            <div class="muted small" style="margin-top:8px">控制命令经平台入队 → 边缘采集器执行 → 回读确认，全程审计。触点容量 AC 3A/250V，驱动大功率设备须经中间继电器。</div>
          </fieldset>
          <fieldset class="fs"><legend><SvgIcon name="log" />命令日志（近50条）</legend>
            <table class="data-table" style="font-size:12px">
              <thead><tr><th>#</th><th>命令</th><th>状态</th><th>结果</th><th>操作人</th><th>时间</th></tr></thead>
              <tbody>
                <tr v-for="c in mon.cmds" :key="c.id">
                  <td>{{ c.id }}</td><td class="mono">{{ c.payload }}</td>
                  <td><span class="badge" :class="{'st-1':c.status==='done','st-2':c.status==='failed','st-0':c.status==='pending'||c.status==='dispatched'}">{{ {pending:'待执行',dispatched:'已下发',done:'成功',failed:'失败'}[c.status]||c.status }}</span></td>
                  <td class="muted" style="max-width:200px">{{ c.result || '-' }}</td>
                  <td>{{ c.created_by || '-' }}</td><td class="muted">{{ fmt(c.done_at || c.created_at) }}</td>
                </tr>
                <tr v-if="!mon.cmds.length"><td colspan="6" class="empty">暂无命令</td></tr>
              </tbody>
            </table>
          </fieldset>
        </template>
      </div>
    </div></div>
  </div>
</template>
<script setup>
import {ref,reactive,computed,onMounted,onBeforeUnmount} from 'vue';import api from '../api'
import { toastAuto, showConfirm } from '../ui-dialog'
const sites=ref([]),list=ref([]),stats=ref({})
const form=reactive({show:false,id:0,code:'',name:'',site_id:0,ip:'',version:'',status:1,remark:''})
const sec=reactive({show:false,id:0,code:'',value:''})
const copied=ref(false)
let timer=null
const siteName=id=>(sites.value.find(s=>s.id===id)||{}).name||'-'
const fmt=t=>t?String(t).replace('T',' ').slice(0,16):'-'
const fmtN=n=>n>=10000?(n/10000).toFixed(1)+'万':String(n||0)
const onlineCnt=computed(()=>list.value.filter(g=>g.online).length)
function ago(t){
  const d=Date.now()-new Date(t).getTime()
  if(d<60000)return '刚刚'
  if(d<3600000)return Math.floor(d/60000)+'分钟前'
  if(d<86400000)return Math.floor(d/3600000)+'小时前'
  return Math.floor(d/86400000)+'天前'
}
async function load(){
  const [r,s]=await Promise.all([api.get('/gateways',{size:100}),api.get('/gateway-stats')])
  list.value=r.data?.items||[]
  stats.value=s.data||{}
}
function openForm(g){if(g)Object.assign(form,{show:true,id:g.id,code:g.code,name:g.name||'',site_id:g.site_id,ip:g.ip||'',version:g.version||'',status:g.status||1,remark:g.remark||''});else Object.assign(form,{show:true,id:0,code:'',name:'',site_id:sites.value[0]?.id||0,ip:'',version:'',status:1,remark:''})}
async function save(){
  const r=form.id?await api.put(`/gateways/${form.id}`,{...form}):await api.post('/gateways',{...form})
  if(r.code===0||r.code===201){
    form.show=false;load()
    // 创建成功 → 立即展示密钥(仅一次)
    if(!form.id && r.data?.secret){
      Object.assign(sec,{show:true,id:r.data.gateway?.id||0,code:r.data.gateway?.code||form.code,value:r.data.secret})
    }
  }else toastAuto(r.msg)
}
function showSecret(g){Object.assign(sec,{show:true,id:g.id,code:g.code,value:''});copied.value=false}

// ===== V47: 边缘监控 =====
const mon=reactive({show:false,id:0,code:'',rt:null,cmds:[],busy:false})
let monTimer=null
function diBit(n){ return mon.rt && mon.rt.di_state>=0 ? !!((mon.rt.di_state>>n)&1) : false }
function doBit(n){ return mon.rt && mon.rt.do_state>=0 ? !!((mon.rt.do_state>>n)&1) : false }
function fmtUp(sec){ sec=sec||0; if(sec<60)return sec+'秒'; if(sec<3600)return Math.floor(sec/60)+'分钟'; if(sec<86400)return Math.floor(sec/3600)+'小时'; return Math.floor(sec/86400)+'天'+Math.floor(sec%86400/3600)+'时' }
async function monRefresh(){
  const r=await api.get('/gateways',{size:100})
  const g=(r.data?.items||[]).find(x=>x.id===mon.id)
  if(g){ try{ mon.rt=g.runtime_state?JSON.parse(g.runtime_state):null }catch{ mon.rt=null } }
  const rc=await api.get(`/gateways/${mon.id}/commands`)
  mon.cmds=rc.data?.commands||[]
}
function openMon(g){
  Object.assign(mon,{show:true,id:g.id,code:g.code,rt:null,cmds:[],busy:false})
  try{ mon.rt=g.runtime_state?JSON.parse(g.runtime_state):null }catch{ mon.rt=null }
  monRefresh()
  monTimer=setInterval(monRefresh,5000)
}
function closeMon(){ mon.show=false; if(monTimer){clearInterval(monTimer);monTimer=null} }
async function doCmd(ch,action){
  const label=action==='on'?'闭合':'断开'
  if (!(await showConfirm({ message: `确认${label} DO${ch} 继电器？\n该操作将直接驱动现场设备（声光报警/风扇/电锁等）。` })))return
  mon.busy=true
  const r=await api.post(`/gateways/${mon.id}/command`,{type:'do_set',payload:{channel:ch,action}})
  mon.busy=false
  if(r.code!==0){toastAuto(r.msg||'命令下发失败');return}
  setTimeout(monRefresh,1500) // 等边缘执行回报后刷新
}
async function doReset(){
  if (!(await showConfirm({ message: `重置网关「${sec.code}」的接入密钥？旧密钥立即失效。` })))return
  const r=await api.post(`/gateways/${sec.id}/reset-secret`,{})
  if(r.code===0){sec.value=r.data.secret;copied.value=false}else toastAuto(r.msg)
}
async function copySecret(){
  try{await navigator.clipboard.writeText(sec.value)}catch{
    const ta=document.createElement('textarea');ta.value=sec.value;document.body.appendChild(ta);ta.select();document.execCommand('copy');ta.remove()
  }
  copied.value=true;setTimeout(()=>copied.value=false,2000)
}
async function del(g){if (!(await showConfirm({ message: `删除网关「${g.code}」？删除后该网关无法推送数据。` })))return;const r=await api.del(`/gateways/${g.id}`);if(r.code===0)load();else toastAuto(r.msg)}
onMounted(async()=>{
  const r=await api.get('/sites',{size:100});sites.value=r.data?.items||[];form.site_id=sites.value[0]?.id||0
  load();timer=setInterval(load,30000)
})
onBeforeUnmount(()=>{if(timer)clearInterval(timer);if(monTimer)clearInterval(monTimer)})
</script>
<style scoped>
.badge.st-1{color:#67c23a;border-color:rgba(103,194,58,.4)}.badge.st-0{color:#909399;border-color:rgba(144,147,153,.4)}
.live-dot{width:8px;height:8px;border-radius:50%;background:#909399;display:inline-block}
.live-dot.live-on{background:#67c23a;animation:pulse-dot 2s infinite}
@keyframes pulse-dot{0%,100%{opacity:1}50%{opacity:.4}}
.stat-bar{display:flex;align-items:center;gap:10px;background:var(--card);border:1px solid var(--border);border-radius:10px;padding:10px 16px;margin-bottom:12px}
.sb-item{display:flex;flex-direction:column;align-items:center;padding:2px 16px}
.sb-item b{font-size:18px;color:var(--text)}.sb-item b.small-b{font-size:12px;font-weight:600}
.sb-item b.c-ok{color:#67c23a}.sb-item b.c-off{color:#909399}.sb-item b.c-warn{color:#e6a23c}
.sb-item span{font-size:11px;color:var(--muted)}
.sec-box{background:var(--bg2);border:1px solid var(--border);border-radius:10px;padding:16px;text-align:center}
.sec-warn{background:rgba(230,162,60,.1);border:1px solid rgba(230,162,60,.4);color:#e6a23c;border-radius:8px;padding:8px 12px;font-size:12.5px;margin-bottom:12px;text-align:left}
.sec-val{font-size:15px;font-weight:700;letter-spacing:1px;background:var(--card);border:1px dashed var(--accent);border-radius:8px;padding:12px;margin-bottom:12px;word-break:break-all;user-select:all}
.warn-text{color:#f56c6c}
.mon-stats{display:grid;grid-template-columns:repeat(4,1fr);gap:10px}
.mon-stat{background:var(--bg2);border-radius:10px;padding:10px 12px;display:flex;flex-direction:column;gap:4px}
.mon-stat b{font-size:15px}
.ok-text{color:var(--green)}
.io-grid{display:grid;grid-template-columns:repeat(4,1fr);gap:8px}
.io-cell{display:flex;align-items:center;gap:6px;background:var(--bg2);border:1px solid var(--border);border-radius:8px;padding:8px 10px;font-size:12.5px;flex-wrap:wrap}
.io-cell.on{border-color:var(--accent);background:color-mix(in srgb,var(--accent) 8%,var(--bg2))}
.io-cell em{font-style:normal;font-size:11px;color:var(--muted);width:100%}
.io-cell.on em{color:var(--red)}
.io-led{width:10px;height:10px;border-radius:50%;background:var(--border);flex-shrink:0;transition:background .3s,box-shadow .3s}
.io-led.on{background:#22c55e;box-shadow:0 0 8px #22c55e}
.io-do .io-led.on{background:var(--accent);box-shadow:0 0 8px var(--accent)}
.drawer-mask{position:fixed;inset:0;background:rgba(0,0,0,.35);z-index:1000;display:flex;justify-content:flex-end}.drawer{width:600px;max-width:95vw;height:100%;background:var(--bg);box-shadow:-4px 0 20px rgba(0,0,0,.15);display:flex;flex-direction:column;overflow:hidden}.drawer-head{display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--border);flex-shrink:0}.drawer-head h3{margin:0;font-size:18px}.drawer-body{flex:1;overflow-y:auto;padding:20px}.drawer-foot{padding:14px 20px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;gap:10px;flex-shrink:0}
.fs{border:1px solid var(--border);border-radius:8px;padding:16px;margin-bottom:16px}.fs legend{font-weight:600;font-size:14px;padding:0 8px;color:var(--accent)}.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.form-grid label{display:flex;flex-direction:column;gap:4px;font-size:13px;color:var(--muted)}
</style>
