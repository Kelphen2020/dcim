<template>
  <div>
    <div class="page-head"><h1>巡检报告</h1></div>
    <div class="toolbar">
      <select class="input" style="width:150px" v-model.number="f.site_id" @change="load(1)"><option :value="0">全部站点</option><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option></select>
      <select class="input" style="width:120px" v-model="f.source" @change="load(1)"><option value="">全部来源</option><option value="scan">扫码巡检</option><option value="web">桌面录入</option></select>
      <select class="input" style="width:120px" v-model="f.status" @change="load(1)"><option value="">全部结论</option><option value="1">正常</option><option value="2">异常</option></select>
      <select class="input" style="width:120px" v-model="f.signed" @change="load(1)"><option value="">签发状态</option><option value="1">已签发</option><option value="0">待签发</option></select>
      <input class="input search-input" v-model="f.keyword" placeholder="搜索报告号/位置/异常" @keyup.enter="load(1)" />
      <button class="btn" @click="load(1)">查询</button>
      <span style="flex:1"></span>
      <button class="btn" @click="qrShow=true;loadQR()"><SvgIcon name="phone" />生成巡检二维码</button>
    </div>

    <div class="card"><div class="table-wrap"><table class="data-table">
      <thead><tr><th>报告号</th><th>计划</th><th>巡检位置</th><th>来源</th><th>结论</th><th>巡检人</th><th>巡检时间</th><th>签发</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="r in list" :key="r.id">
          <td class="mono"><a class="link" @click="openReport(r)">{{ r.report_code||('记录#'+r.id) }}</a></td>
          <td class="muted">{{ r.plan_name||'临时巡检' }}</td>
          <td><span v-if="r.target_name" class="badge">{{ r.target_type==='cabinet'?'':'' }} {{ r.target_name }}</span><span v-else class="muted">-</span></td>
          <td><span class="badge" :class="'src-'+r.source">{{ r.source==='scan'?'扫码':'桌面' }}</span></td>
          <td><span class="badge" :class="'rec-'+r.status">{{ RECST[r.status]||r.status }}</span></td>
          <td class="muted">{{ r.inspector_name||'-' }}</td>
          <td class="muted">{{ fmt(r.inspected_at) }}</td>
          <td><span v-if="r.reviewed_at" class="signed"><SvgIcon name="check" />{{ r.reviewer_name||'已签' }}</span><span v-else class="unsigned">待签发</span></td>
          <td class="ops">
            <button class="btn btn-mini" @click="openReport(r)">报告</button>
            <button v-if="!r.reviewed_at&&isAdmin" class="btn btn-mini btn-mini-primary" @click="openSign(r)">签发</button>
          </td>
        </tr>
        <tr v-if="!list.length"><td colspan="9" class="empty">暂无巡检报告</td></tr>
      </tbody>
    </table></div><Pagination :page="page" :total="total" :page-size="size" @change="load" /></div>

    <!-- 报告详情抽屉（可打印） -->
    <div v-if="rep.show" class="drawer-mask" @click.self="rep.show=false"><div class="drawer" style="width:680px">
      <div class="drawer-head"><h3>巡检报告 · {{ rep.data.record?.report_code||('#'+rep.id) }}</h3>
        <div><button class="btn btn-mini" @click="printReport"><SvgIcon name="printer" />打印</button> <button class="btn btn-mini" @click="rep.show=false"><SvgIcon name="x" /></button></div></div>
      <div class="drawer-body" id="report-print">
        <div class="rp-title">机房巡检报告</div>
        <div class="rp-meta">
          <div><span class="rl">报告编号</span>{{ rep.data.record?.report_code||'-' }}</div>
          <div><span class="rl">站点</span>{{ rep.data.site?.name||'-' }}</div>
          <div><span class="rl">巡检计划</span>{{ rep.data.plan?.name||'临时巡检' }}</div>
          <div><span class="rl">巡检位置</span>{{ rep.data.record?.target_name||'-' }}（{{ rep.data.record?.target_type==='cabinet'?'机柜':'房间' }}）</div>
          <div><span class="rl">巡检人</span>{{ rep.data.inspector_name||'-' }}</div>
          <div><span class="rl">巡检时间</span>{{ fmt(rep.data.record?.inspected_at) }}</div>
          <div v-if="rep.data.checkin"><span class="rl">扫码签到</span>{{ fmt(rep.data.checkin.checked_in_at) }}</div>
          <div><span class="rl">巡检结论</span><span class="badge" :class="'rec-'+rep.data.record?.status">{{ RECST[rep.data.record?.status] }}</span></div>
        </div>

        <div v-if="rep.data.result_lines?.length" class="rp-sec">
          <div class="rp-sec-t">检查项结果</div>
          <table class="rp-table"><tbody>
            <tr v-for="(l,i) in rep.data.result_lines" :key="i" :class="{'rp-abn':l.abnormal}"><td>{{ l.text }}</td></tr>
          </tbody></table>
        </div>

        <div v-if="rep.data.point_snapshot?.length" class="rp-sec">
          <div class="rp-sec-t">测点实时值快照</div>
          <table class="rp-table"><thead><tr><th>检查项</th><th>测点</th><th>读数</th><th>判定</th></tr></thead><tbody>
            <tr v-for="(p,i) in rep.data.point_snapshot" :key="i">
              <td>{{ p.item }}</td><td>{{ p.point||'-' }}</td>
              <td>{{ p.value!=null?p.value+(p.unit||''):'无读数' }}</td>
              <td><span :class="p.in_range===false?'rp-bad':'rp-ok'">{{ p.in_range===false?'越限':p.value!=null?'正常':'-' }}</span></td>
            </tr>
          </tbody></table>
        </div>

        <div v-if="rep.data.record?.abnormal" class="rp-sec">
          <div class="rp-sec-t">异常描述</div>
          <div class="rp-abn-box">{{ rep.data.record.abnormal }}</div>
          <div v-if="rep.data.work_order" class="rp-wo">已转工单：<a class="link" @click="$router.push('/work-orders?id='+rep.data.work_order.id)">{{ rep.data.work_order.code }}</a> {{ rep.data.work_order.title }}</div>
        </div>

        <div class="rp-sign">
          <div class="rp-sign-row"><span class="rl">巡检人签字</span><span class="rp-sign-line">{{ rep.data.inspector_name||'' }}</span></div>
          <div class="rp-sign-row"><span class="rl">审核人签字</span>
            <span class="rp-sign-line" :class="{'rp-signed':rep.data.record?.reviewed_at}">{{ rep.data.record?.signature||rep.data.reviewer_name||'' }}</span>
            <span v-if="rep.data.record?.reviewed_at" class="rp-sign-time">{{ fmt(rep.data.record.reviewed_at) }}</span>
            <span v-else class="rp-pending">（待签发）</span>
          </div>
        </div>
      </div>
      <div class="drawer-foot">
        <button class="btn" @click="rep.show=false">关闭</button>
        <button v-if="!rep.data.record?.reviewed_at&&isAdmin" class="btn btn-primary" @click="openSign(rep.data.record)">签发报告</button>
      </div>
    </div></div>

    <!-- 签发抽屉 -->
    <div v-if="sign.show" class="drawer-mask" @click.self="sign.show=false"><div class="drawer" style="width:420px">
      <div class="drawer-head"><h3>签发报告 · {{ sign.rec?.report_code||('#'+sign.rec?.id) }}</h3><button class="btn btn-mini" @click="sign.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <label class="full">审核签字（默认登录名）<input class="input" v-model="sign.signature" placeholder="审核人姓名" /></label>
        <p class="muted" style="font-size:12.5px;margin-top:10px">签发后报告锁定，不可再修改。签字将记入审计日志。</p>
        <p v-if="sign.err" class="err">{{ sign.err }}</p>
      </div>
      <div class="drawer-foot"><button class="btn" @click="sign.show=false">取消</button><button class="btn btn-primary" @click="doSign">确认签发</button></div>
    </div></div>

    <!-- 二维码生成抽屉 -->
    <div v-if="qrShow" class="drawer-mask" @click.self="qrShow=false"><div class="drawer" style="width:640px">
      <div class="drawer-head"><h3><SvgIcon name="phone" />生成巡检二维码</h3><button class="btn btn-mini" @click="qrShow=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <div class="qr-tabs">
          <button class="seg-btn" :class="{on:qrMode==='single'}" @click="qrMode='single'">单个</button>
          <button class="seg-btn" :class="{on:qrMode==='batch'}" @click="switchBatch">批量打印</button>
        </div>
        <template v-if="qrMode==='single'">
          <div class="form-grid">
            <label>目标类型<select class="input" v-model="qr.type" @change="qr.id=0"><option value="cabinet">机柜</option><option value="room">房间</option></select></label>
            <label>有效期<select class="input" v-model.number="qr.days"><option :value="90">90天</option><option :value="365">1年</option><option :value="1825">5年</option></select></label>
            <label class="full">{{ qr.type==='cabinet'?'选择机柜':'选择房间' }}
              <select class="input" v-model.number="qr.id" @change="genSingle">
                <option :value="0">请选择</option>
                <option v-for="t in (qr.type==='cabinet'?cabinets:rooms)" :key="t.id" :value="t.id">{{ qr.type==='cabinet'?t.code:t.name }}</option>
              </select>
            </label>
          </div>
          <div v-if="qr.dataUrl" class="qr-preview">
            <img :src="qr.dataUrl" alt="QR" />
            <div class="qr-info">{{ qr.target?.name }} · 有效期至 {{ qr.expires_at }}</div>
            <button class="btn" @click="downloadQR">下载 PNG</button>
          </div>
        </template>
        <template v-else>
          <div class="form-grid">
            <label>目标类型<select class="input" v-model="qrB.type"><option value="cabinet">机柜</option><option value="room">房间</option></select></label>
            <label>范围<select class="input" v-model.number="qrB.room_id" :disabled="qrB.type==='room'"><option :value="0">全部机柜</option><option v-for="rm in rooms" :key="rm.id" :value="rm.id">{{ rm.name }}</option></select></label>
          </div>
          <button class="btn btn-primary" style="margin:10px 0" @click="genBatch">生成二维码墙</button>
          <div v-if="qrB.list.length" class="qr-wall" id="qr-wall-print">
            <div v-for="q in qrB.list" :key="q.token" class="qr-cell">
              <img :src="q.dataUrl" alt="QR" />
              <div class="qr-cell-name">{{ q.name }}</div>
            </div>
          </div>
          <button v-if="qrB.list.length" class="btn" style="margin-top:10px" @click="printWall"><SvgIcon name="printer" />打印二维码墙</button>
        </template>
      </div>
    </div></div>
  </div>
</template>
<script setup>
import {ref,reactive,onMounted} from 'vue'
import {useRoute} from 'vue-router'
import QRCode from 'qrcode'
import api from '../api'
import Pagination from '../components/Pagination.vue'
import {useAuthStore} from '../stores/auth'
const route=useRoute(),auth=useAuthStore()
const isAdmin=auth.isAdmin
const RECST={1:'正常',2:'发现异常',3:'逾期未巡'}
const sites=ref([]),rooms=ref([]),cabinets=ref([]),list=ref([]),page=ref(1),total=ref(0),size=20
const f=reactive({site_id:0,source:'',status:'',signed:'',keyword:''})
const rep=reactive({show:false,id:0,data:{}})
const sign=reactive({show:false,rec:null,signature:'',err:''})
const fmt=t=>t?String(t).replace('T',' ').slice(0,16):'-'
async function load(p){page.value=p||1;const params={page:page.value,size};if(f.site_id)params.site_id=f.site_id;if(f.source)params.source=f.source;if(f.status)params.status=f.status;if(f.signed)params.signed=f.signed;if(f.keyword)params.keyword=f.keyword
  const r=await api.get('/inspection-reports',params);list.value=r.data?.items||[];total.value=r.data?.total||0
  if(route.query.id){openReport({id:Number(route.query.id)})}}
async function openReport(r){rep.id=r.id;rep.show=true;rep.data={};const res=await api.get(`/inspection-reports/${r.id}`);if(res.code===0)rep.data=res.data||{}}
function openSign(r){Object.assign(sign,{show:true,rec:r,signature:'',err:''})}
async function doSign(){sign.err='';const r=await api.post(`/inspection-reports/${sign.rec.id}/sign`,{signature:sign.signature});if(r.code===0){sign.show=false;rep.show=false;load(page.value)}else sign.err=r.msg}
function printReport(){window.print()}

// ===== 二维码 =====
const qrShow=ref(false),qrMode=ref('single')
const qr=reactive({type:'cabinet',id:0,days:365,dataUrl:'',target:null,expires_at:''})
const qrB=reactive({type:'cabinet',room_id:0,list:[]})
async function loadQR(){if(!rooms.value.length){const r=await api.get('/rooms',{size:200});rooms.value=r.data?.items||[]}}
async function genSingle(){qr.dataUrl='';if(!qr.id)return;const r=await api.get('/inspection-scan/qr',{type:qr.type,id:qr.id,days:qr.days});if(r.code===0){qr.dataUrl=await QRCode.toDataURL(r.data.token,{width:240,margin:1});qr.target=r.data.target;qr.expires_at=r.data.expires_at}}
async function switchBatch(){qrMode.value='batch';await loadQR();if(!cabinets.value.length){const r=await api.get('/cabinets',{size:500});cabinets.value=r.data?.items||[]}}
async function genBatch(){qrB.list=[];const params={type:qrB.type,days:365};if(qrB.type==='cabinet'&&qrB.room_id)params.room_id=qrB.room_id;const r=await api.get('/inspection-scan/qr-batch',params);if(r.code===0){for(const it of (r.data.items||[])){qrB.list.push({...it,dataUrl:await QRCode.toDataURL(it.token,{width:160,margin:1})})}}}
function downloadQR(){const a=document.createElement('a');a.href=qr.dataUrl;a.download=`巡检码_${qr.target?.name||qr.id}.png`;a.click()}
function printWall(){const w=document.getElementById('qr-wall-print');const win=window.open('','_blank');win.document.write('<html><head><title>巡检二维码</title><style>body{font-family:sans-serif}.qr-wall{display:grid;grid-template-columns:repeat(4,1fr);gap:12px}.qr-cell{text-align:center;border:1px solid #ccc;padding:8px;page-break-inside:avoid}.qr-cell img{width:100%}.qr-cell-name{font-size:13px;margin-top:4px}</style></head><body>'+w.outerHTML+'</body></html>');win.document.close();win.print()}
onMounted(async()=>{const ss=await api.get('/sites',{size:100});sites.value=ss.data?.items||[];const cs=await api.get('/cabinets',{size:500});cabinets.value=cs.data?.items||[];load()})
</script>
<style scoped>
.link{color:var(--accent);cursor:pointer;font-weight:600}.link:hover{text-decoration:underline}
.badge.src-scan{color:#409eff;border-color:rgba(64,158,255,.4)}.badge.src-web{color:#909399;border-color:rgba(144,147,153,.4)}
.badge.rec-1{color:#67c23a;border-color:rgba(103,194,58,.4)}.badge.rec-2{color:#f56c6c;border-color:rgba(245,108,108,.4)}.badge.rec-3{color:#e6a23c;border-color:rgba(230,162,60,.4)}
.signed{color:#67c23a;font-size:12.5px}.unsigned{color:#909399;font-size:12.5px}
.btn-mini-primary{color:var(--accent);border-color:var(--accent)}
.qr-tabs{display:flex;gap:8px;margin-bottom:14px}
.seg-btn{padding:6px 16px;font-size:13px;background:transparent;border:1px solid var(--border);border-radius:6px;color:var(--muted);cursor:pointer}
.seg-btn.on{background:var(--accent,#409eff);color:#fff;border-color:var(--accent)}
.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}
.form-grid label{display:flex;flex-direction:column;gap:4px;font-size:13px;color:var(--muted)}
.form-grid label.full{grid-column:1/-1}
.qr-preview{text-align:center;margin-top:16px}
.qr-preview img{border:1px solid var(--border);border-radius:8px;padding:8px;background:#fff}
.qr-info{font-size:13px;color:var(--muted);margin:10px 0}
.qr-wall{display:grid;grid-template-columns:repeat(auto-fill,minmax(120px,1fr));gap:12px;margin-top:10px}
.qr-cell{text-align:center;border:1px solid var(--border);border-radius:8px;padding:8px;background:#fff}
.qr-cell img{width:100%}
.qr-cell-name{font-size:13px;margin-top:4px;color:#333}
/* 报告样式 */
.rp-title{text-align:center;font-size:20px;font-weight:700;margin-bottom:16px}
.rp-meta{display:grid;grid-template-columns:1fr 1fr;gap:8px 16px;font-size:13.5px;border:1px solid var(--border);border-radius:8px;padding:14px;margin-bottom:16px}
.rp-meta>div{display:flex;gap:6px}.rl{display:inline-block;min-width:72px;color:var(--muted)}
.rp-sec{margin-bottom:16px}
.rp-sec-t{font-size:14px;font-weight:600;color:var(--accent);margin-bottom:8px;border-left:3px solid var(--accent);padding-left:8px}
.rp-table{width:100%;border-collapse:collapse;font-size:13px}
.rp-table th,.rp-table td{border:1px solid var(--border);padding:7px 9px;text-align:left}
.rp-table th{background:var(--bg2);font-weight:600}
.rp-abn td{color:#f56c6c}
.rp-ok{color:#67c23a}.rp-bad{color:#f56c6c;font-weight:600}
.rp-abn-box{background:rgba(245,108,108,.06);border:1px solid rgba(245,108,108,.3);border-radius:8px;padding:10px 12px;font-size:13.5px;color:#f56c6c}
.rp-wo{font-size:13px;margin-top:8px}
.rp-sign{border-top:2px solid var(--border);margin-top:20px;padding-top:16px}
.rp-sign-row{display:flex;align-items:center;gap:8px;margin-bottom:18px;font-size:14px}
.rp-sign-line{flex:1;border-bottom:1px solid var(--text2,#666);min-height:24px;font-size:18px;font-family:'STKaiti','KaiTi',cursive;padding:0 8px}
.rp-signed{color:#1a7f37;font-weight:600}
.rp-sign-time{font-size:12px;color:var(--muted)}
.rp-pending{font-size:12px;color:#e6a23c}
.drawer-mask{position:fixed;inset:0;background:rgba(0,0,0,.35);z-index:1000;display:flex;justify-content:flex-end}
.drawer{width:600px;max-width:95vw;height:100%;background:var(--bg);box-shadow:-4px 0 20px rgba(0,0,0,.15);display:flex;flex-direction:column;overflow:hidden}
.drawer-head{display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--border);flex-shrink:0}
.drawer-head h3{margin:0;font-size:17px}
.drawer-body{flex:1;overflow-y:auto;padding:20px}
.drawer-foot{padding:14px 20px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;gap:10px;flex-shrink:0}
.err{color:#f56c6c;font-size:13px}
@media (max-width:900px){.rp-meta{grid-template-columns:1fr}.form-grid{grid-template-columns:1fr}}
</style>
<style>
@media print{
  .sidebar,.topbar,.nav-backdrop,.toolbar,.drawer-head button,.drawer-foot{display:none!important}
  .drawer-mask{position:static;background:none;display:block}
  .drawer{width:100%!important;max-width:100%;box-shadow:none;height:auto}
  #report-print{padding:0}
}
</style>
