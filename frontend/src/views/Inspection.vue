<template>
  <div>
    <div class="page-head"><h1>巡检管理</h1></div>
    <div class="tabs">
      <button class="tab" :class="{on:tab==='plans'}" @click="tab='plans'">巡检计划</button>
      <button class="tab" :class="{on:tab==='records'}" @click="tab='records';loadRecords()">巡检记录</button>
      <button class="tab" :class="{on:tab==='tasks'}" @click="tab='tasks';loadTasks()"><SvgIcon name="check-square" /> 任务治理</button>
    </div>

    <template v-if="tab==='plans'">
      <div class="toolbar">
        <select class="input" style="width:150px" v-model.number="f.site_id" @change="load"><option :value="0">全部站点</option><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option></select>
        <span style="flex:1"></span><button class="btn btn-primary" @click="openForm()">＋ 新建计划</button>
      </div>
      <div class="card"><div class="table-wrap"><table class="data-table">
        <thead><tr><th>名称</th><th>站点</th><th>房间</th><th>周期</th><th>检查项数</th><th>责任人</th><th>最近执行</th><th>状态</th><th>操作</th></tr></thead>
        <tbody>
          <tr v-for="p in list" :key="p.id">
            <td>{{ p.name }}</td><td class="muted">{{ siteName(p.site_id) }}</td>
            <td class="muted">{{ roomName(p.room_id) }}</td>
            <td><span class="badge">{{ FREQ[p.freq]||p.freq }}</span></td>
            <td>{{ (p.items||'').split('\n').filter(x=>x.trim()).length }}</td>
            <td class="muted">{{ userName(p.assignee_id) }}</td>
            <td class="muted">{{ fmt(p.last_run_at) }}</td>
            <td><span class="badge" :class="p.status===1?'st-1':'st-0'">{{ p.status===1?'启用':'停用' }}</span></td>
            <td class="ops">
              <button class="btn btn-mini btn-mini-primary" @click="openRun(p)">执行巡检</button>
              <button class="btn btn-mini" @click="tab='records';f.rec_plan=p.id;loadRecords()">记录</button>
              <button class="btn btn-mini" @click="openForm(p)">编辑</button>
              <button class="btn btn-mini btn-mini-danger" @click="del(p)">删除</button>
            </td>
          </tr>
          <tr v-if="!list.length"><td colspan="9" class="empty">暂无巡检计划</td></tr>
        </tbody>
      </table></div></div>
    </template>

    <template v-else-if="tab==='records'">
      <div class="toolbar">
        <select class="input" style="width:150px" v-model.number="f.rec_site" @change="loadRecords"><option :value="0">全部站点</option><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option></select>
        <select class="input" style="width:150px" v-model.number="f.rec_plan" @change="loadRecords"><option :value="0">全部计划</option><option v-for="p in list" :key="p.id" :value="p.id">{{ p.name }}</option></select>
      </div>
      <div class="card"><div class="table-wrap"><table class="data-table">
        <thead><tr><th>报告号</th><th>计划</th><th>位置</th><th>来源</th><th>巡检人</th><th>结果</th><th>异常说明</th><th>关联工单</th><th>巡检时间</th></tr></thead>
        <tbody>
          <tr v-for="r in records" :key="r.id">
            <td class="mono"><a v-if="r.report_code" class="link" @click="$router.push('/inspection-reports?id='+r.id)">{{ r.report_code }}</a><span v-else class="muted">#{{ r.id }}</span></td>
            <td>{{ planName(r.plan_id) }}</td>
            <td><span v-if="r.target_name" class="badge">{{ r.target_name }}</span><span v-else class="muted">-</span></td>
            <td><span class="badge" :class="'src-'+(r.source||'web')">{{ r.source==='scan'?'扫码':'桌面' }}</span></td>
            <td class="muted">{{ userName(r.inspector_id) }}</td>
            <td><span class="badge" :class="'rec-'+r.status">{{ RECST[r.status] }}</span></td>
            <td class="muted" style="max-width:260px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" :title="r.abnormal">{{ r.abnormal||'-' }}</td>
            <td><a v-if="r.work_order_id" class="link" @click="$router.push({path:'/work-orders',query:{id:r.work_order_id}})">工单#{{ r.work_order_id }}</a><span v-else class="muted">-</span></td>
            <td class="muted">{{ fmt(r.inspected_at) }}</td>
          </tr>
          <tr v-if="!records.length"><td colspan="9" class="empty">暂无巡检记录</td></tr>
        </tbody>
      </table></div></div>
    </template>

    <template v-else>
      <!-- V62 任务治理：KPI 概览 -->
      <div class="tk-kpis" v-if="tstats">
        <div class="tk-kpi" :class="rateCls(tstats.totals.done_rate)" @click="tf.status='';loadTasks()" title="窗口内 已完成/(已完成+漏检)，点击查看全部任务">
          <div class="tk-num">{{ tstats.totals.done_rate }}<i>%</i></div>
          <div class="tk-label"><SvgIcon name="gauge" /> 近{{ tstats.window_days }}天完成率</div>
        </div>
        <div class="tk-kpi" @click="tf.status='0';loadTasks()" title="当前周期内尚未执行的应巡任务">
          <div class="tk-num">{{ tstats.current.pending }}</div>
          <div class="tk-label"><SvgIcon name="clock" /> 当期待巡检</div>
        </div>
        <div class="tk-kpi" :class="{warn:tstats.totals.missed>0}" @click="tf.status='2';loadTasks()" title="周期结束仍未执行，已判定漏检">
          <div class="tk-num">{{ tstats.totals.missed }}</div>
          <div class="tk-label"><SvgIcon name="alert" /> 近{{ tstats.window_days }}天漏检</div>
        </div>
        <div class="tk-kpi" @click="tf.status='3';loadTasks()" title="已填写原因豁免的任务">
          <div class="tk-num">{{ tstats.totals.exempt }}</div>
          <div class="tk-label"><SvgIcon name="shield" /> 近{{ tstats.window_days }}天豁免</div>
        </div>
        <div class="tk-kpi" @click="tf.status='';loadTasks()" title="窗口内按计划周期自动生成的任务总数">
          <div class="tk-num">{{ tstats.totals.generated }}</div>
          <div class="tk-label"><SvgIcon name="layers" /> 近{{ tstats.window_days }}天生成</div>
        </div>
      </div>

      <div class="toolbar">
        <select class="input" style="width:110px" v-model.number="tf.days" @change="loadTasks()"><option :value="7">近 7 天</option><option :value="30">近 30 天</option><option :value="90">近 90 天</option></select>
        <select class="input" style="width:150px" v-model.number="tf.site_id" @change="loadTasks()"><option :value="0">全部站点</option><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option></select>
        <select class="input" style="width:120px" v-model="tf.status" @change="loadTasks()"><option value="">全部状态</option><option v-for="(n,k) in TKST" :key="k" :value="k">{{ n }}</option></select>
        <select class="input" style="width:150px" v-model.number="tf.assignee_id" @change="loadTasks()"><option :value="0">全部责任人</option><option v-for="u in users" :key="u.id" :value="u.id">{{ u.display_name }}（{{ u.username }}）</option></select>
        <span v-if="tf.plan_id" class="flt-chip" @click="tf.plan_id=0;loadTasks()" title="点击取消按计划过滤">计划：{{ planName(tf.plan_id) }} <SvgIcon name="x" /></span>
        <span style="flex:1"></span>
        <button v-if="canOps" class="btn" @click="scanMissed()"><SvgIcon name="search" /> 漏检扫描</button>
        <button v-if="isAdmin" class="btn btn-primary" @click="genTasks()"><SvgIcon name="refresh" /> 生成任务</button>
      </div>

      <!-- 治理面板：TOP 漏检计划 + 按责任人完成率 -->
      <div class="tk-panels" v-if="tstats">
        <div class="card tk-panel">
          <div class="tk-panel-hd"><SvgIcon name="alert" /> 漏检 TOP 计划（近{{ tstats.window_days }}天）</div>
          <div v-if="tstats.top_missed.length" class="tk-top">
            <div v-for="t in tstats.top_missed" :key="t.plan_id" class="tk-top-row" @click="tf.plan_id=t.plan_id;tf.status='2';loadTasks()" :title="'点击筛选该计划的漏检任务'">
              <span class="tk-top-name">{{ t.plan_name||('计划#'+t.plan_id) }}</span>
              <span class="tk-top-bar"><i :style="{width:Math.min(100,t.cnt/(tstats.top_missed[0].cnt||1)*100)+'%'}"></i></span>
              <span class="tk-top-cnt">{{ t.cnt }} 次</span>
            </div>
          </div>
          <div v-else class="muted tk-empty">窗口内没有漏检记录，保持得很好</div>
        </div>
        <div class="card tk-panel">
          <div class="tk-panel-hd"><SvgIcon name="user" /> 责任人完成率（近{{ tstats.window_days }}天）</div>
          <div v-if="tstats.by_assignee.length" class="table-wrap"><table class="data-table tk-atable">
            <thead><tr><th>责任人</th><th>完成</th><th>漏检</th><th style="width:38%">完成率</th></tr></thead>
            <tbody>
              <tr v-for="a in tstats.by_assignee" :key="a.assignee_id" class="tk-clickable" @click="tf.assignee_id=a.assignee_id;loadTasks()" title="点击按该责任人筛选">
                <td>{{ a.name||('用户#'+a.assignee_id) }}</td><td>{{ a.done }}</td><td :class="{missed:a.missed>0}">{{ a.missed }}</td>
                <td><span class="tk-rate"><i :class="rateCls(a.rate)" :style="{width:a.rate+'%'}"></i></span> {{ a.rate }}%</td>
              </tr>
            </tbody>
          </table></div>
          <div v-else class="muted tk-empty">窗口内暂无已完成/漏检的任务数据</div>
        </div>
      </div>

      <!-- 任务列表 -->
      <div class="card"><div class="table-wrap"><table class="data-table tk-task-table">
        <thead><tr><th>巡检计划</th><th>站点</th><th>周期</th><th>频率</th><th>责任人</th><th>状态</th><th>完成时间 / 记录</th><th>说明</th><th>操作</th></tr></thead>
        <tbody>
          <tr v-for="t in tasks" :key="t.id" :class="{'tk-hl':t.id===hlTask}">
            <td>{{ t.plan_name||('计划#'+t.plan_id) }}</td>
            <td class="muted">{{ t.site_name||siteName(t.site_id) }}</td>
            <td class="mono">{{ t.period_key }}</td>
            <td><span class="badge">{{ FREQ[t.freq]||t.freq }}</span></td>
            <td class="muted">{{ t.assignee_name||userName(t.assignee_id) }}</td>
            <td><span class="badge" :class="'tk-'+t.status">{{ TKST[t.status] }}</span></td>
            <td class="muted"><template v-if="t.status===1">{{ fmt(t.done_at) }} <a v-if="t.record_id" class="link" @click="$router.push('/inspection-reports?id='+t.record_id)">{{ t.record_code||('记录#'+t.record_id) }}</a></template><template v-else>-</template></td>
            <td class="muted" style="max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" :title="t.exempt_reason">{{ t.status===3?(t.exempt_reason||'已豁免'):(t.status===2&&!t.miss_notified?'漏检（未通知）':'-') }}</td>
            <td class="ops">
              <button v-if="t.status===1&&t.record_id" class="btn btn-mini" @click="tab='records';f.rec_plan=t.plan_id;loadRecords()">记录</button>
              <button v-if="canOps&&(t.status===0||t.status===2)" class="btn btn-mini" @click="exemptTask(t)">豁免</button>
              <button v-if="isAdmin&&t.status===0" class="btn btn-mini btn-mini-primary" @click="openAssign(t)">改派</button>
            </td>
          </tr>
          <tr v-if="!tasks.length"><td colspan="9" class="empty">窗口内暂无巡检任务——启用巡检计划后系统按周期自动生成（每小时一轮，也可点「生成任务」立即生成）</td></tr>
        </tbody>
      </table></div></div>
    </template>

    <!-- 计划编辑抽屉 -->
    <div v-if="form.show" class="drawer-mask" @click.self="form.show=false"><div class="drawer">
      <div class="drawer-head"><h3>{{ form.id?'编辑计划':'新建巡检计划' }}</h3><button class="btn btn-mini" @click="form.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <fieldset class="fs"><legend><SvgIcon name="pin" />基本信息</legend>
          <div class="form-grid">
            <label class="full">名称 *<input class="input" v-model="form.name" /></label>
            <label>站点 *<select class="input" v-model.number="form.site_id" @change="form.room_id=0"><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option></select></label>
            <label>关联房间<select class="input" v-model.number="form.room_id" title="扫码巡检时按房间自动匹配计划"><option :value="0">（全站通用）</option><option v-for="rm in siteRooms" :key="rm.id" :value="rm.id">{{ rm.name }}</option></select></label>
            <label>巡检周期 *<select class="input" v-model="form.freq"><option v-for="(n,k) in FREQ" :key="k" :value="k">{{ n }}</option></select></label>
            <label>责任人<select class="input" v-model.number="form.assignee_id"><option :value="0">（未指派）</option><option v-for="u in users" :key="u.id" :value="u.id">{{ u.display_name }}（{{ u.username }}）</option></select></label>
            <label>状态<select class="input" v-model.number="form.status"><option :value="1">启用</option><option :value="0">停用</option></select></label>
            <label class="full">检查项（每行一项；行内加 <code class="at-code">@测点编码</code> 可在执行巡检时自动显示实时值与越限提示）<textarea class="input" rows="5" v-model="form.items" placeholder="1. 检查UPS运行状态 @demo_ups_a1&#10;2. 检查空调温度 @demo_hvac_a1&#10;3. 检查机柜门锁" style="width:100%;box-sizing:border-box"></textarea></label>
            <div class="full" v-if="sitePoints.length" style="grid-column:1/-1">
              <span class="muted small">快速插入测点：</span>
              <div class="pt-chips">
                <span v-for="pt in sitePoints" :key="pt.id" class="pt-chip" @click="appendPoint(pt)" :title="'点击追加到检查项：'+pt.name+'（'+pt.code+'）'">@{{ pt.code }}<i>{{ pt.name }}</i></span>
              </div>
            </div>
          </div>
        </fieldset>
        <p v-if="form.err" class="err">{{ form.err }}</p>
      </div>
      <div class="drawer-foot"><button class="btn" @click="form.show=false">取消</button><button class="btn btn-primary" @click="save">保存</button></div>
    </div></div>

    <!-- 执行巡检抽屉 -->
    <div v-if="run.show" class="drawer-mask" @click.self="run.show=false"><div class="drawer">
      <div class="drawer-head"><h3>执行巡检 · {{ run.plan?.name }}</h3><button class="btn btn-mini" @click="run.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <fieldset class="fs"><legend><SvgIcon name="check-circle" />逐项检查</legend>
          <div v-for="(it,i) in run.items" :key="i" class="run-item">
            <span class="run-idx">{{ i+1 }}.</span><span class="run-text">{{ it }}</span>
            <span v-if="run.live&&run.live[i]&&run.live[i].point_id" class="live-val" :class="{'lv-bad':run.live[i].range_status==='over'||run.live[i].status===2,'lv-alarm':run.live[i].range_status==='high'||run.live[i].range_status==='low'}" :title="{'over':'超过过高线(严重)','high':'超过超高线','low':'低于过低线','normal':''}[run.live[i].range_status]||''">
              {{ run.live[i].current_value!=null?run.live[i].current_value:('·'+(run.live[i].point_name||run.live[i].point_code)+' 无读数') }}{{ run.live[i].unit||'' }}
              <template v-if="run.live[i].in_range===false"> <SvgIcon name="alert" />越限</template>
            </span>
            <span v-else-if="run.live&&run.live[i]&&run.live[i].point_code" class="live-val lv-miss" title="测点编码不存在">@{{ run.live[i].point_code }} 无效</span>
            <label class="radio"><input type="radio" :name="'ri'+i" :value="1" v-model.number="run.results[i]" /> 正常</label>
            <label class="radio"><input type="radio" :name="'ri'+i" :value="2" v-model.number="run.results[i]" /> 异常</label>
          </div>
          <div v-if="!run.items.length" class="muted">该计划未配置检查项，可直接填写总体结果</div>
          <div v-if="run.outCount" class="err" style="margin-top:8px"><SvgIcon name="alert" />{{ run.outCount }} 项绑定测点当前越限/告警，请核实后将对应项标记为「异常」</div>
        </fieldset>
        <fieldset class="fs"><legend><SvgIcon name="pen" />巡检结论</legend>
          <label class="full">总体结果 *<select class="input" v-model.number="run.status"><option :value="1">正常</option><option :value="2">发现异常（自动转工单）</option></select></label>
          <label class="full" style="margin-top:10px">逐项记录<textarea class="input" rows="3" v-model="run.result" style="width:100%;box-sizing:border-box" placeholder="补充说明（可选）"></textarea></label>
          <label v-if="run.status===2" class="full" style="margin-top:10px">异常描述 *<textarea class="input" rows="2" v-model="run.abnormal" style="width:100%;box-sizing:border-box" placeholder="描述异常现象，将自动创建故障工单"></textarea></label>
        </fieldset>
        <p v-if="run.err" class="err">{{ run.err }}</p>
      </div>
      <div class="drawer-foot"><button class="btn" @click="run.show=false">取消</button><button class="btn btn-primary" @click="submitRun">提交巡检</button></div>
    </div></div>

    <!-- V62 任务改派抽屉 -->
    <div v-if="assignForm.show" class="drawer-mask" @click.self="assignForm.show=false"><div class="drawer" style="width:420px">
      <div class="drawer-head"><h3>改派巡检任务</h3><button class="btn btn-mini" @click="assignForm.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <fieldset class="fs"><legend><SvgIcon name="swap" />任务信息</legend>
          <p class="tk-assign-info">计划：<b>{{ assignForm.task?.plan_name }}</b><br>周期：<span class="mono">{{ assignForm.task?.period_key }}</span>（{{ FREQ[assignForm.task?.freq]||assignForm.task?.freq }}）<br>站点：{{ assignForm.task?.site_name||siteName(assignForm.task?.site_id) }}</p>
          <div class="form-grid">
            <label class="full">新责任人 *<select class="input" v-model.number="assignForm.assignee_id"><option :value="0">（请选择）</option><option v-for="u in users" :key="u.id" :value="u.id">{{ u.display_name }}（{{ u.username }}）</option></select></label>
          </div>
          <p class="muted small" style="margin-top:8px">改派后将通知新责任人；巡检提交后任务自动核销。</p>
          <p v-if="assignForm.err" class="err">{{ assignForm.err }}</p>
        </fieldset>
      </div>
      <div class="drawer-foot"><button class="btn" @click="assignForm.show=false">取消</button><button class="btn btn-primary" @click="saveAssign">确认改派</button></div>
    </div></div>
  </div>
</template>

<script setup>
import {ref,reactive,computed,onMounted,watch} from 'vue';import api from '../api'
import { toastAuto, showConfirm, showPrompt } from '../ui-dialog'
import { useAuthStore } from '../stores/auth'
import {useRoute} from 'vue-router'
const route=useRoute()
const auth=useAuthStore()
const isAdmin=computed(()=>auth.isAdmin)
const canOps=computed(()=>['global_admin','site_admin','operator'].includes(auth.role))
// 与后端一致: freq=daily/weekly/monthly, status=1启用/0停用, items为文本(每行一项)
const FREQ={daily:'每日',weekly:'每周',monthly:'每月',quarterly:'每季'}
const RECST={1:'正常',2:'发现异常',3:'逾期未巡'}
const TKST={0:'待巡检',1:'已完成',2:'漏检',3:'已豁免'}
const tab=ref('plans')
const sites=ref([]),users=ref([]),list=ref([]),records=ref([]),points=ref([]),rooms=ref([])
const sitePoints=computed(()=>points.value.filter(p=>!form.site_id||p.site_id===form.site_id))
const siteRooms=computed(()=>rooms.value.filter(r=>!form.site_id||r.site_id===form.site_id))
const roomName=id=>{if(!id)return '全站';const rm=rooms.value.find(x=>x.id===id);return rm?rm.name:'房间#'+id}
const f=reactive({site_id:0,rec_site:0,rec_plan:0})
const form=reactive({show:false,id:0,name:'',site_id:0,room_id:0,freq:'daily',items:'',assignee_id:0,status:1,err:''})
const run=reactive({show:false,plan:null,items:[],results:[],live:null,outCount:0,status:1,result:'',abnormal:'',err:''})
const siteName=id=>(sites.value.find(s=>s.id===id)||{}).name||'-'
const planName=id=>(list.value.find(p=>p.id===id)||{}).name||'计划#'+id
const userName=id=>{if(!id)return '系统';const u=users.value.find(x=>x.id===id);return u?u.display_name:'用户#'+id}
const fmt=t=>t?String(t).replace('T',' ').slice(0,16):'-'
async function load(){const params={};if(f.site_id)params.site_id=f.site_id;const r=await api.get('/inspection-plans',params);list.value=r.data?.items||[]}
async function loadRecords(){const params={size:100};if(f.rec_site)params.site_id=f.rec_site;if(f.rec_plan)params.plan_id=f.rec_plan;const r=await api.get('/inspection-records',params);records.value=r.data?.items||[]}
function openForm(p){if(p)Object.assign(form,{show:true,id:p.id,name:p.name,site_id:p.site_id,room_id:p.room_id||0,freq:p.freq||'daily',items:p.items||'',assignee_id:p.assignee_id||0,status:p.status,err:''});else Object.assign(form,{show:true,id:0,name:'',site_id:sites.value[0]?.id||0,room_id:0,freq:'daily',items:'',assignee_id:0,status:1,err:''})}
function appendPoint(pt){
  const lines=form.items.split('\n').filter(x=>x.trim())
  const n=lines.length+1
  form.items=(form.items?form.items.replace(/\n+$/,'')+'\n':'')+`${n}. 检查${pt.name} @${pt.code}`
}
async function save(){form.err='';if(!form.name||!form.site_id){form.err='名称、站点必填';return}
  const r=form.id?await api.put(`/inspection-plans/${form.id}`,{...form}):await api.post('/inspection-plans',{...form})
  if(r.code===0){form.show=false;load()}else form.err=r.msg}
async function del(p){if (!(await showConfirm({ message: `删除计划「${p.name}」？` })))return;const r=await api.del(`/inspection-plans/${p.id}`);if(r.code===0)load();else toastAuto(r.msg)}
function openRun(p){const items=(p.items||'').split('\n').map(x=>x.trim()).filter(Boolean)
  Object.assign(run,{show:true,plan:p,items,results:items.map(()=>1),live:null,outCount:0,status:1,result:'',abnormal:'',err:''})
  loadLive(items)}
async function loadLive(items){
  if(!items.length)return
  const r=await api.post('/inspection-items-live',{items})
  if(r.code!==0||!r.data)return
  run.live=r.data.items||null
  run.outCount=(run.live||[]).filter(x=>x.in_range===false||x.status===2).length
}
async function submitRun(){run.err=''
  if(run.status===2&&!run.abnormal.trim()){run.err='发现异常时必须填写异常描述';return}
  const itemLines=run.items.map((it,i)=>`${i+1}. ${it}：${run.results[i]===2?'异常':'正常'}`).join('\n')
  const result=run.result?(itemLines?itemLines+'\n'+run.result:run.result):itemLines
  const r=await api.post('/inspection-records',{plan_id:run.plan.id,status:run.status,result,abnormal:run.abnormal})
  if(r.code!==0){run.err=r.msg;return}
  run.show=false;load()
  if(tab.value==='records')loadRecords()}
// ===== V62 任务治理 =====
const tasks=ref([]),tstats=ref(null),hlTask=ref(0)
const tf=reactive({days:30,site_id:0,status:'',assignee_id:0,plan_id:0})
const assignForm=reactive({show:false,task:null,assignee_id:0,err:''})
const rateCls=r=>r>=90?'rate-ok':(r>=70?'rate-mid':'rate-bad')
async function loadTasks(){
  const p={size:100}
  if(tf.site_id)p.site_id=tf.site_id
  if(tf.status!=='')p.status=tf.status
  if(tf.assignee_id)p.assignee_id=tf.assignee_id
  if(tf.plan_id)p.plan_id=tf.plan_id
  const r=await api.get('/inspection-tasks',p)
  tasks.value=r.data?.items||[]
  const sp={days:tf.days}
  if(tf.site_id)sp.site_id=tf.site_id
  const rs=await api.get('/inspection-tasks/stats',sp)
  tstats.value=rs.data||null
}
async function genTasks(){
  const r=await api.post('/inspection-tasks/generate',{})
  if(r.code===0){const d=r.data||{};toastAuto(`生成完成：新增 ${d.generated||0} 条，自动核销 ${d.linked||0} 条`);loadTasks()}else toastAuto(r.msg)
}
async function scanMissed(){
  if(!(await showConfirm({title:'漏检扫描',message:'将所有「周期已结束仍未执行」的任务判定为漏检，并通知责任人（幂等，已处理的不重复通知）。确认执行？'})))return
  const r=await api.post('/inspection-tasks/scan-missed',{})
  if(r.code===0){toastAuto(r.data?.missed?`扫描完成：新判定漏检 ${r.data.missed} 条`:'扫描完成：没有新的漏检');loadTasks()}else toastAuto(r.msg)
}
async function exemptTask(t){
  const reason=await showPrompt({title:'豁免巡检任务',label:`豁免「${t.plan_name}」${t.period_key} 的原因（必填，将写入审计留痕）`,multiline:true})
  if(reason===null)return
  if(!String(reason).trim()){toastAuto('豁免原因必填');return}
  const r=await api.put(`/inspection-tasks/${t.id}/exempt`,{reason:String(reason).trim()})
  if(r.code===0){toastAuto('已豁免');loadTasks()}else toastAuto(r.msg)
}
function openAssign(t){assignForm.show=true;assignForm.task=t;assignForm.assignee_id=t.assignee_id||0;assignForm.err=''}
async function saveAssign(){
  assignForm.err=''
  if(!assignForm.assignee_id){assignForm.err='请选择新责任人';return}
  const r=await api.put(`/inspection-tasks/${assignForm.task.id}/assign`,{assignee_id:assignForm.assignee_id})
  if(r.code===0){assignForm.show=false;toastAuto('已改派并通知新责任人');loadTasks()}else assignForm.err=r.msg
}
// G批: 大屏今日巡检穿透 ?plan_id=N → 记录tab按该计划过滤
function applyQuery(q){if(q&&q.plan_id){const pid=Number(q.plan_id);if(pid){tab.value='records';f.rec_plan=pid;loadRecords()}}
  if(q&&q.tab==='tasks'){tab.value='tasks'
    if(q.status!==undefined&&q.status!=='')tf.status=String(q.status)
    if(q.plan_id)tf.plan_id=Number(q.plan_id)
    loadTasks()
    if(q.id){hlTask.value=Number(q.id);setTimeout(()=>{hlTask.value=0},4000)}}}
watch(()=>route.query,(q)=>{if(route.path==='/inspection')applyQuery(q)})
onMounted(async()=>{const[ss,uu,pp,rr]=await Promise.all([api.get('/sites',{size:100}),api.get('/users-options'),api.get('/monitoring-points',{size:500}),api.get('/rooms',{size:200})]);sites.value=ss.data?.items||[];users.value=uu.data?.items||[];points.value=pp.data?.items||[];rooms.value=rr.data?.items||[];form.site_id=sites.value[0]?.id||0;load();applyQuery(route.query)})
</script>
<style scoped>
.tabs{display:flex;gap:8px;margin-bottom:14px}.tab{padding:7px 18px;border:1px solid var(--border);border-radius:8px;background:var(--card);cursor:pointer;font-size:13.5px}.tab.on{color:var(--accent);border-color:var(--accent);font-weight:600}
.badge.st-1{color:#67c23a;border-color:rgba(103,194,58,.4)}.badge.st-0{color:#909399;border-color:rgba(144,147,153,.4)}
.badge.src-scan{color:#409eff;border-color:rgba(64,158,255,.4)}.badge.src-web{color:#909399;border-color:rgba(144,147,153,.4)}
.badge.rec-1{color:#67c23a;border-color:rgba(103,194,58,.4)}.badge.rec-2{color:#f56c6c;border-color:rgba(245,108,108,.4)}.badge.rec-3{color:#e6a23c;border-color:rgba(230,162,60,.4)}
.link{color:var(--accent);cursor:pointer;font-weight:600}.link:hover{text-decoration:underline}
.btn-mini-primary{color:var(--accent);border-color:var(--accent)}
.run-item{display:flex;align-items:center;gap:12px;padding:8px 4px;border-bottom:1px dashed var(--border);font-size:13.5px}.run-idx{color:var(--muted)}.run-text{flex:1}.radio{display:inline-flex;align-items:center;gap:4px;font-size:12.5px;cursor:pointer}
.drawer-mask{position:fixed;inset:0;background:rgba(0,0,0,.35);z-index:1000;display:flex;justify-content:flex-end}.drawer{width:600px;max-width:95vw;height:100%;background:var(--bg);box-shadow:-4px 0 20px rgba(0,0,0,.15);display:flex;flex-direction:column;overflow:hidden}.drawer-head{display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--border);flex-shrink:0}.drawer-head h3{margin:0;font-size:18px}.drawer-body{flex:1;overflow-y:auto;padding:20px}.drawer-foot{padding:14px 20px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;gap:10px;flex-shrink:0}
.fs{border:1px solid var(--border);border-radius:8px;padding:16px;margin-bottom:16px}.fs legend{font-size:14px;padding:0 8px;color:var(--accent)}
.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.form-grid label{display:flex;flex-direction:column;gap:4px;font-size:13px;color:var(--muted)}.form-grid label.full{grid-column:1/-1}
.err{color:#f56c6c;font-size:13px}
.at-code{background:var(--bg2);border:1px solid var(--border);border-radius:4px;padding:0 5px;font-size:12px;color:var(--accent)}
.pt-chips{display:flex;flex-wrap:wrap;gap:6px;margin-top:6px}.pt-chip{display:inline-flex;align-items:center;gap:4px;border:1px solid var(--border);border-radius:14px;padding:3px 10px;font-size:12px;cursor:pointer;color:var(--accent);font-family:monospace}.pt-chip i{font-style:normal;color:var(--muted);font-family:inherit}.pt-chip:hover{border-color:var(--accent);background:color-mix(in srgb, var(--accent) 8%, transparent)}
.live-val{font-size:12.5px;font-family:monospace;padding:2px 8px;border-radius:10px;background:var(--bg2);border:1px solid var(--border);color:var(--muted);white-space:nowrap}
.live-val.lv-bad{color:#f56c6c;border-color:rgba(245,108,108,.5);font-weight:700}
.live-val.lv-alarm{color:#e6a23c;border-color:rgba(230,162,60,.5)}
.live-val.lv-miss{color:#909399;border-style:dashed}
/* ===== V62 任务治理 ===== */
.tk-kpis{display:grid;grid-template-columns:repeat(5,1fr);gap:12px;margin-bottom:14px}
.tk-kpi{background:var(--card);border:1px solid var(--border);border-radius:10px;padding:14px 16px;cursor:pointer;transition:border-color .15s,transform .15s}
.tk-kpi:hover{border-color:var(--accent);transform:translateY(-1px)}
.tk-num{font-size:30px;font-weight:700;line-height:1.1;font-family:'Arial',sans-serif}
.tk-num i{font-style:normal;font-size:15px;color:var(--muted);margin-left:2px}
.tk-label{margin-top:6px;font-size:12.5px;color:var(--muted);display:flex;align-items:center;gap:5px}
.tk-kpi.rate-ok .tk-num{color:#67c23a}.tk-kpi.rate-mid .tk-num{color:#e6a23c}.tk-kpi.rate-bad .tk-num{color:#f56c6c}
.tk-kpi.warn{border-color:rgba(245,108,108,.55)}.tk-kpi.warn .tk-num{color:#f56c6c}
.tk-panels{display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:14px}
.tk-panel{padding:14px 16px;margin:0}
.tk-panel-hd{font-size:13.5px;font-weight:600;color:var(--accent);display:flex;align-items:center;gap:6px;margin-bottom:10px}
.tk-top-row{display:flex;align-items:center;gap:10px;padding:6px 4px;cursor:pointer;border-radius:6px;font-size:13px}
.tk-top-row:hover{background:color-mix(in srgb,var(--accent) 6%,transparent)}
.tk-top-name{width:38%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.tk-top-bar{flex:1;height:8px;border-radius:4px;background:var(--bg2);overflow:hidden}
.tk-top-bar i{display:block;height:100%;border-radius:4px;background:linear-gradient(90deg,#f56c6c,#e6a23c)}
.tk-top-cnt{color:#f56c6c;font-weight:700;font-size:12.5px;min-width:44px;text-align:right}
.tk-empty{padding:14px 4px;font-size:13px}
.tk-atable td.missed{color:#f56c6c;font-weight:700}
.tk-clickable{cursor:pointer}.tk-clickable:hover td{background:color-mix(in srgb,var(--accent) 5%,transparent)}
.tk-rate{display:inline-block;width:60%;max-width:120px;height:8px;border-radius:4px;background:var(--bg2);overflow:hidden;vertical-align:middle;margin-right:6px}
.tk-rate i{display:block;height:100%;border-radius:4px}
.tk-rate i.rate-ok{background:#67c23a}.tk-rate i.rate-mid{background:#e6a23c}.tk-rate i.rate-bad{background:#f56c6c}
.badge.tk-0{color:var(--accent);border-color:color-mix(in srgb,var(--accent) 45%,transparent)}
.badge.tk-1{color:#67c23a;border-color:rgba(103,194,58,.4)}
.badge.tk-2{color:#f56c6c;border-color:rgba(245,108,108,.45);font-weight:700}
.badge.tk-3{color:#909399;border-color:rgba(144,147,153,.4)}
.flt-chip{display:inline-flex;align-items:center;gap:5px;border:1px solid var(--accent);color:var(--accent);border-radius:14px;padding:3px 10px;font-size:12.5px;cursor:pointer}
.flt-chip:hover{background:color-mix(in srgb,var(--accent) 8%,transparent)}
.tk-hl td{animation:tkFlash 1.2s ease-in-out 3}
@keyframes tkFlash{0%,100%{background:transparent}50%{background:color-mix(in srgb,var(--accent) 16%,transparent)}}
.tk-assign-info{font-size:13.5px;line-height:1.9;margin:0 0 10px}
.tab{display:inline-flex;align-items:center;gap:6px}
@media(max-width:900px){.tk-kpis{grid-template-columns:repeat(2,1fr)}.tk-panels{grid-template-columns:1fr}}
</style>
