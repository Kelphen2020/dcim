<template>
  <div class="scan-page">
    <!-- 步骤1: 签到 -->
    <template v-if="stage==='checkin'">
      <div class="sp-head"><SvgIcon name="camera" />扫码巡检签到</div>
      <div class="sp-card">
        <p v-if="!token" class="sp-err">缺少扫码令牌，请扫描机柜/房间上的巡检二维码</p>
        <p v-else-if="err" class="sp-err">{{ err }}</p>
        <p v-else class="muted">正在签到…</p>
      </div>
    </template>

    <!-- 步骤2: 执行巡检 -->
    <template v-if="stage==='run'">
      <div class="sp-head">
        <span class="sp-target">{{ target.type==='cabinet'?'机柜':'房间' }} {{ target.name }}</span>
        <span class="sp-time">签到 {{ checkedInAt }}</span>
      </div>
      <div v-if="todayCount>0" class="sp-note">今日该位置已巡检 {{ todayCount }} 次</div>

      <div class="sp-card" v-if="plans.length">
        <div class="sp-label">巡检计划</div>
        <select class="input" v-model.number="planId">
          <option v-for="p in plans" :key="p.id" :value="p.id">{{ p.name }}（{{ FREQ[p.freq]||p.freq }}）</option>
        </select>
      </div>
      <div class="sp-card" v-else>
        <div class="sp-label">临时巡检（无匹配计划）</div>
      </div>

      <div class="sp-card" v-if="items.length">
        <div class="sp-label">检查项（{{ items.length }}）<span v-if="outCount" class="sp-out"><SvgIcon name="alert" />{{ outCount }} 项越限</span></div>
        <div v-for="(it,i) in items" :key="i" class="sp-item">
          <div class="sp-item-text">{{ it }}</div>
          <div v-if="live && live[i]" class="sp-live">
            <span class="lv" :class="lvCls(live[i])">{{ lvText(live[i]) }}</span>
          </div>
          <div class="sp-radios">
            <label><input type="radio" :value="1" v-model="results[i]" /> <SvgIcon name="check" />正常</label>
            <label><input type="radio" :value="2" v-model="results[i]" /> <SvgIcon name="x-circle" />异常</label>
          </div>
        </div>
      </div>

      <div class="sp-card">
        <div class="sp-label">巡检结论</div>
        <div class="sp-radios big">
          <label><input type="radio" :value="1" v-model="status" /> <SvgIcon name="check" />全部正常</label>
          <label><input type="radio" :value="2" v-model="status" /> <SvgIcon name="x-circle" />发现异常</label>
        </div>
        <textarea v-if="status===2" class="input" rows="3" v-model="abnormal" placeholder="异常描述（必填，将自动转工单）" style="margin-top:10px;width:100%;box-sizing:border-box"></textarea>
        <textarea class="input" rows="2" v-model="remark" placeholder="补充说明（可选）" style="margin-top:10px;width:100%;box-sizing:border-box"></textarea>
        <p v-if="err" class="sp-err">{{ err }}</p>
      </div>

      <button class="sp-submit" :disabled="submitting" @click="submit">{{ submitting?'提交中…':'提交巡检' }}</button>
      <p class="sp-fresh muted">签到有效期 {{ freshMin }} 分钟，超时须重新扫码（防补录）</p>
    </template>

    <!-- 步骤3: 完成 -->
    <template v-if="stage==='done'">
      <div class="sp-done">
        <div class="sp-done-icon"><SvgIcon name="check" /></div>
        <h2>巡检已提交</h2>
        <p class="muted">报告编号：{{ doneRec.report_code || '（待生成）' }}</p>
        <p v-if="doneRec.work_order_id" class="sp-wo"><SvgIcon name="alert" />异常已自动转工单 #{{ doneRec.work_order_id }}</p>
        <a class="sp-link" :href="'#/inspection-reports/'+doneRec.id">查看巡检报告 ›</a>
        <button class="sp-submit" style="margin-top:20px" @click="stage='checkin';doCheckin()">继续巡检下一处</button>
      </div>
    </template>
  </div>
</template>
<script setup>
import {ref,onMounted} from 'vue'
import {useRoute} from 'vue-router'
import api from '../api'
const route=useRoute()
const FREQ={daily:'每日',weekly:'每周',monthly:'每月'}
const stage=ref('checkin')
const token=ref(''),err=ref('')
const checkin=ref(null),target=ref({}),plans=ref([]),todayCount=ref(0),checkedInAt=ref('')
const planId=ref(0),items=ref([]),results=ref([]),live=ref(null),outCount=ref(0)
const status=ref(1),abnormal=ref(''),remark=ref(''),submitting=ref(false)
const doneRec=ref({}),freshMin=ref(120)
const lvCls=x=>x.range_status==='over'||x.status===2?'lv-bad':(x.range_status==='high'||x.range_status==='low')?'lv-alarm':x.current_value==null?'lv-miss':'lv-ok'
const lvText=x=>{if(!x.point_code)return '';if(x.current_value==null)return x.point_code+' 无读数';const tag={over:' 过高',high:' 超高',low:' 过低',normal:''}[x.range_status]??(x.in_range===false?' 越限':'');return `${x.point_name||x.point_code}: ${x.current_value}${x.unit||''}${tag}`}

async function doCheckin(){
  err.value=''
  const tk=route.query.token||token.value
  if(!tk){err.value='缺少扫码令牌，请扫描机柜/房间上的巡检二维码';return}
  token.value=tk
  const r=await api.post('/inspection-scan/checkin',{token:tk})
  if(r.code!==0){err.value=r.msg||'签到失败';return}
  checkin.value=r.data.checkin
  target.value=r.data.target||{}
  plans.value=r.data.plans||[]
  todayCount.value=r.data.today_records||0
  checkedInAt.value=String(r.data.server_time||'').slice(11,16)
  planId.value=plans.value[0]?.id||0
  loadItems()
  stage.value='run'
}
async function loadItems(){
  const p=plans.value.find(x=>x.id===planId.value)
  items.value=p?(p.items||'').split('\n').map(x=>x.trim()).filter(Boolean):[]
  results.value=items.value.map(()=>1)
  live.value=null;outCount.value=0
  if(!items.value.length)return
  const r=await api.post('/inspection-items-live',{items:items.value})
  if(r.code===0&&r.data){live.value=r.data.items||null;outCount.value=(live.value||[]).filter(x=>x.in_range===false||x.status===2).length}
}
import {watch} from 'vue'
watch(planId,loadItems)
async function submit(){
  err.value=''
  if(status.value===2&&!abnormal.value.trim()){err.value='发现异常时必须填写异常描述';return}
  submitting.value=true
  const itemLines=items.value.map((it,i)=>`${i+1}. ${it}：${results.value[i]===2?'异常':'正常'}`).join('\n')
  const result=[itemLines,remark.value].filter(Boolean).join('\n')
  const snap=live.value?JSON.stringify(live.value.map(x=>({item:x.item,point:x.point_name||x.point_code,value:x.current_value,unit:x.unit,in_range:x.in_range}))):''
  const r=await api.post('/inspection-scan/submit',{checkin_id:checkin.value.id,plan_id:planId.value||undefined,status:status.value,abnormal:abnormal.value,result,point_snap:snap})
  submitting.value=false
  if(r.code!==0){err.value=r.msg||'提交失败';return}
  doneRec.value=r.data||{}
  stage.value='done'
}
onMounted(()=>{
  const auth=JSON.parse(localStorage.getItem('dcim_user')||'null')
  if(!localStorage.getItem('dcim_token')){location.hash='#/login';return}
  api.get('/system-settings').then(()=>{}).catch(()=>{})
  doCheckin()
})
</script>
<style scoped>
.scan-page{max-width:560px;margin:0 auto;padding:12px 14px 40px;font-size:14px}
.sp-head{display:flex;justify-content:space-between;align-items:baseline;padding:10px 2px;position:sticky;top:0;background:var(--bg);z-index:5}
.sp-target{font-size:17px;font-weight:700}
.sp-time{font-size:12px;color:var(--muted)}
.sp-note{font-size:12.5px;color:#e6a23c;background:rgba(230,162,60,.08);border:1px solid rgba(230,162,60,.3);border-radius:8px;padding:7px 10px;margin-bottom:10px}
.sp-card{background:var(--card);border:1px solid var(--border);border-radius:10px;padding:12px 14px;margin-bottom:12px}
.sp-label{font-size:13px;font-weight:600;color:var(--accent);margin-bottom:8px;display:flex;justify-content:space-between;align-items:center}
.sp-out{color:#f56c6c;font-size:12px}
.sp-item{padding:9px 0;border-bottom:1px dashed var(--border)}
.sp-item:last-child{border-bottom:none}
.sp-item-text{font-size:13.5px;line-height:1.5}
.sp-live{margin:4px 0}
.lv{font-size:12px;font-family:monospace;padding:2px 8px;border-radius:10px;border:1px solid var(--border);background:var(--bg2)}
.lv-ok{color:#67c23a}.lv-bad{color:#f56c6c;border-color:rgba(245,108,108,.5);font-weight:700}.lv-alarm{color:#e6a23c}.lv-miss{color:#909399;border-style:dashed}
.sp-radios{display:flex;gap:16px;margin-top:6px}
.sp-radios label{display:inline-flex;align-items:center;gap:5px;font-size:13px;cursor:pointer}
.sp-radios.big label{font-size:15px;font-weight:600}
.sp-radios input{width:18px;height:18px}
.sp-submit{display:block;width:100%;padding:14px;font-size:16px;font-weight:700;color:#fff;background:var(--accent,#409eff);border:none;border-radius:10px;cursor:pointer}
.sp-submit:disabled{opacity:.6}
.sp-fresh{text-align:center;font-size:12px;margin-top:10px}
.sp-err{color:#f56c6c;font-size:14px;text-align:center;padding:16px 0}
.sp-done{text-align:center;padding:50px 20px}
.sp-done-icon{width:64px;height:64px;border-radius:50%;background:#67c23a;color:#fff;font-size:34px;line-height:64px;margin:0 auto 16px}
.sp-wo{color:#e6a23c;font-size:13.5px;margin-top:6px}
.sp-link{display:inline-block;margin-top:14px;color:var(--accent);font-size:14px}
</style>
