<template>
  <div>
    <div class="page-head"><h1>资产台账</h1></div>
    <div class="toolbar">
      <select class="input" style="width:150px" v-model.number="f.site_id" @change="load(1)"><option :value="0">全部站点</option><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option></select>
      <select class="input" style="width:120px" v-model="f.category" @change="load(1)"><option value="">全部分类</option><option v-for="(n,k) in CATS" :key="k" :value="k">{{ n }}</option></select>
      <select class="input" style="width:120px" v-model="f.status" @change="load(1)"><option value="">全部状态</option><option v-for="(n,k) in ST" :key="k" :value="k">{{ n }}</option></select>
      <select class="input" style="width:120px" v-model="f.health_grade" @change="load(1)" title="按健康等级筛选"><option value="">全部健康度</option><option v-for="(n,k) in HGS" :key="k" :value="k">{{ n }}</option></select>
      <input class="input search-input" v-model="f.keyword" placeholder="搜索编码/名称/品牌/SN/IP" @keyup.enter="load(1)" />
      <button class="btn" @click="load(1)">查询</button><span style="flex:1"></span>
      <!-- V16 批量操作 -->
      <template v-if="selIds.length">
        <span class="muted small">已选 {{ selIds.length }} 项</span>
        <button class="btn btn-mini" @click="batchStatus('to_repair')">转维修</button>
        <button class="btn btn-mini" @click="batchStatus('to_store')">转在库</button>
        <button class="btn btn-mini btn-mini-danger" @click="batchStatus('to_scrap')">报废</button>
        <button class="btn btn-mini" @click="selIds=[]">取消</button>
      </template>
      <button class="btn" @click="exportCSV"><SvgIcon name="download" />导出CSV</button>
      <button class="btn" @click="tplShow=true"><SvgIcon name="clipboard" />模板</button>
      <button class="btn btn-primary" style="margin-left:8px" @click="openForm()">＋ 新增资产</button>
    </div>
    <div class="card"><div class="table-wrap"><table class="data-table">
      <thead><tr><th style="width:30px"><input type="checkbox" :checked="pageAllChecked" @change="togglePageAll" /></th><th>编码</th><th>名称</th><th>分类</th><th>品牌/型号</th><th>位置</th><th>U位</th><th>功耗</th><th>服役</th><th>状态</th><th>维保</th><th>健康度</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="a in list" :key="a.id">
          <td><input type="checkbox" :value="a.id" v-model="selIds" /></td>
          <td class="mono">{{ a.code }}</td><td>{{ a.name }}</td><td><span class="badge">{{ CATS[a.category]||a.category }}</span></td>
          <td class="muted">{{ a.brand }} {{ a.model }}</td>
          <td><a v-if="a.cabinet_id" class="link" @click="goCapacity(a)" :title="'查看 ' + cabName(a) + ' 容量'">{{ cabName(a) }}</a><span v-else-if="a.room_id" class="link" @click="$router.push({path:'/room-3d',query:{room_id:a.room_id}})" title="在3D机房中查看">{{ roomName(a.room_id) }}</span><span v-else class="muted">-</span></td>
          <td v-if="a.cabinet_id&&a.status===1">{{ a.start_u }}~{{ a.start_u+a.height_u-1 }}U</td><td v-else class="muted">-</td>
          <td class="muted">{{ a.power_w }}W</td>
          <td><span v-if="a.purchase_at" :class="{'age-warn': serviceDays(a)>=1825}" :title="serviceDays(a)>=1825?'服役超5年，建议评估更换':''">{{ serviceYears(a) }}</span><span v-else class="muted">-</span></td>
          <td><span class="badge" :class="'st-'+a.status">{{ ST[a.status] }}</span></td>
          <td><a v-if="maintMap[a.id]" class="badge st-ok link" @click="goMaint(a)" title="查看维保记录">维保中</a><span v-else class="muted">-</span></td>
          <td><a v-if="a.health_score!=null" class="badge hg link" :class="'hg-'+a.health_grade" @click="openHealth(a)" :title="'健康分 '+a.health_score+' / 100，点击查看评估详情'">{{ a.health_score }} {{ HGS[a.health_grade]||'' }}</a><span v-else class="muted" title="尚未评估">-</span></td>
          <td class="ops">
            <button class="btn btn-mini" @click="goOrders(a)" title="查看关联工单">工单</button>
            <button class="btn btn-mini" @click="openRec(a)" title="按BOM推荐备件">备件</button>
            <button v-if="a.status===2||a.status===3" class="btn btn-mini" @click="openMount(a)">上架</button>
            <button v-if="a.status===1" class="btn btn-mini" @click="doUnmount(a)">下架</button>
            <button v-if="a.status===2" class="btn btn-mini" @click="quickStatus(a,'to_repair')">转维修</button>
            <button v-if="a.status===3" class="btn btn-mini" @click="quickStatus(a,'to_store')">修复入库</button>
            <button v-if="a.status!==4" class="btn btn-mini" @click="router.push('/asset-retirement?new=1&asset_id='+a.id)" title="发起退役申请（审批通过后自动下架并报废）">退役</button>
            <button class="btn btn-mini" @click="openForm(a)">编辑</button>
            <button class="btn btn-mini btn-mini-danger" @click="doDelete(a)">删除</button>
          </td>
        </tr>
        <tr v-if="!list.length"><td colspan="13" class="empty">暂无资产</td></tr>
      </tbody>
    </table></div><Pagination :page="page" :total="total" :page-size="size" @change="load" /></div>

    <!-- 模板选择 -->
    <div v-if="tplShow" class="drawer-mask" @click.self="tplShow=false"><div class="drawer" style="width:680px">
      <div class="drawer-head">
        <h3><SvgIcon name="clipboard" />资产模板</h3>
        <button class="btn btn-mini" @click="tplShow=false"><SvgIcon name="x" /></button>
      </div>
      <div class="drawer-body">
        <div class="tpl-grid">
          <div class="tpl-item" v-for="t in TPL" :key="t.name" @click="applyTpl(t)">
            <div class="tpl-name"><SvgIcon name="box" />{{ t.name }}</div>
            <div class="tpl-desc">{{ t.desc }}</div>
          </div>
        </div>
      </div>
      <div class="drawer-foot"><button class="btn" @click="tplShow=false">关闭</button></div>
    </div></div>

    <!-- V31 推荐备件抽屉 -->
    <div v-if="rec.show" class="drawer-mask" @click.self="rec.show=false"><div class="drawer" style="width:560px">
      <div class="drawer-head"><h3><SvgIcon name="puzzle" />推荐备件 · {{ rec.asset?.name }}</h3><button class="btn btn-mini" @click="rec.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <div class="muted" style="font-size:12.5px;margin-bottom:10px">
          按 BOM 关联推荐：{{ rec.d?.asset_category }}<span v-if="rec.d?.asset_model"> / 型号 {{ rec.d.asset_model }}</span>
          <span v-if="rec.d?.low_stock_count" class="rec-warn"><SvgIcon name="alert" />{{ rec.d.low_stock_count }} 项低库存</span>
        </div>
        <table class="data-table" v-if="(rec.d?.list||[]).length">
          <thead><tr><th>备件</th><th>匹配</th><th>建议数</th><th>库存</th><th>单价</th><th>供应商</th></tr></thead>
          <tbody><tr v-for="r in rec.d.list" :key="r.bom_id">
            <td><b>{{ r.spare_name }}</b><div class="muted" style="font-size:11px">{{ r.spare_code }}</div></td>
            <td><span class="badge" :class="r.match==='model'?'st-ok':'st-2'">{{ r.match==='model'?'型号精确':'分类通用' }}</span></td>
            <td>{{ r.qty }} {{ r.spare_unit }}</td>
            <td :class="{'rec-low': r.safety_stock>0&&r.stock<=r.safety_stock}">{{ r.stock }}</td>
            <td>{{ r.price? '¥'+r.price : '—' }}</td>
            <td class="muted">{{ r.supplier || '—' }}</td>
          </tr></tbody>
        </table>
        <div v-else class="empty" style="padding:24px;text-align:center;opacity:.6">
          该资产分类暂无 BOM 关联，去「备件台账 → BOM」配置
        </div>
      </div>
      <div class="drawer-foot"><button class="btn" @click="rec.show=false">关闭</button></div>
    </div></div>

    <!-- 新增/编辑抽屉 -->
    <div v-if="form.show" class="drawer-mask" @click.self="form.show=false"><div class="drawer">
      <div class="drawer-head"><h3>{{ form.id?'编辑资产':'新增资产' }}</h3><button class="btn btn-mini" @click="form.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <fieldset class="fs"><legend><SvgIcon name="pin" />基本信息</legend>
          <div class="form-grid">
            <label>编码 *<input class="input" v-model="form.code" :disabled="!!form.id" /></label>
            <label>名称 *<input class="input" v-model="form.name" /></label>
            <label>分类 *<select class="input" v-model="form.category"><option v-for="(n,k) in CATS" :key="k" :value="k">{{ n }}</option></select></label>
            <label>所属站点 *<select class="input" v-model.number="form.site_id" :disabled="!!form.id"><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option></select></label>
            <label>所在房间（非机架设备）<select class="input" v-model.number="form.room_id"><option :value="0">— 不适用 —</option><option v-for="r in rooms" :key="r.id" :value="r.id">{{ r.name }}</option></select></label>
            <label>品牌<input class="input" v-model="form.brand" /></label>
            <label>型号<input class="input" v-model="form.model" /></label>
            <label>序列号<input class="input" v-model="form.sn" /></label>
            <label>管理IP<input class="input" v-model="form.ip" /></label>
            <label>资产编号（财务）<input class="input" v-model="form.asset_no" /></label>
            <label>固资编号<input class="input" v-model="form.fixed_asset_no" /></label>
            <label>采购日期<input class="input" type="date" v-model="form.purchase_at" /></label>
            <label>供应商<input class="input" v-model="form.vendor" /></label>
            <label>合同编号<input class="input" v-model="form.contract_no" /></label>
            <label>责任人<input class="input" v-model="form.owner" /></label>
            <label>所属部门<input class="input" v-model="form.department" /></label>
          </div>
        </fieldset>
        <fieldset class="fs"><legend><SvgIcon name="ruler" />物理规格</legend>
          <div class="form-grid">
            <label>U高<input class="input" type="number" min="1" v-model.number="form.height_u" /></label>
            <label>额定功耗(W)<input class="input" type="number" min="0" v-model.number="form.power_w" /></label>
            <label>重量(kg)<input class="input" type="number" min="0" step="0.1" v-model.number="form.weight_kg" /></label>
            <label>尺寸 (宽×深×高 mm)<input class="input" v-model="form.dimensions" placeholder="440×600×44" /></label>
          </div>
        </fieldset>
        <fieldset class="fs"><legend><SvgIcon name="pen" />备注</legend>
          <textarea class="input" rows="3" v-model="form.remark" style="width:100%;box-sizing:border-box"></textarea>
        </fieldset>
        <p v-if="form.err" class="err">{{ form.err }}</p>
      </div>
      <div class="drawer-foot"><button class="btn" @click="form.show=false">取消</button><button class="btn btn-primary" @click="saveForm">保存</button></div>
    </div></div>

    <!-- 上架抽屉 -->
    <div v-if="mount.show" class="drawer-mask" @click.self="mount.show=false"><div class="drawer" style="width:480px">
      <div class="drawer-head"><h3>上架 · {{ mount.asset?.name }}</h3><button class="btn btn-mini" @click="mount.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <p class="muted">U高 {{ mount.asset?.height_u }}U · 功耗 {{ mount.asset?.power_w }}W · 重量 {{ mount.asset?.weight_kg }}kg</p>
        <label style="margin-top:12px">目标机柜 *<select class="input" v-model.number="mount.cabinet_id" @change="loadCap"><option :value="0">请选择</option><option v-for="c in cabinets" :key="c.id" :value="c.id">{{ c.code }}（{{ roomName(c.room_id) }}，{{ c.total_u }}U）</option></select></label>
        <label style="margin-top:8px">起始U位 *<input class="input" type="number" min="1" v-model.number="mount.start_u" /></label>
        <div v-if="cap" class="cap-summary"><span>空闲: <b>{{ cap.free_u }}</b>/{{ cap.total_u }}U</span><span>已用功率: <b>{{ cap.used_power_w }}W</b>{{ cabPow }}</span><span>已用承重: <b>{{ cap.used_weight_kg?.toFixed(1) }}kg</b>{{ cabWt }}</span></div>
        <div v-if="mount.wi" class="wi-box">
          <div class="wi-title"><SvgIcon name="gauge" /> 四维容量校验（上架前 What-if 试算）</div>
          <div v-for="d in mount.wi.checks" :key="d.dim" class="wi-row" :class="'wi-'+d.status">
            <span class="wi-ic"><SvgIcon :name="d.status==='ok'?'check-circle':d.status==='over'?'x-circle':d.status==='warn'?'alert':'info'" /></span>
            <span class="wi-label">{{ d.label }}</span>
            <span class="wi-msg">{{ d.message }}</span>
          </div>
          <div class="wi-verdict" :class="'wv-'+mount.wi.verdict">{{ mount.wi.conclusion }}</div>
          <label v-if="mount.wi.verdict==='over'" class="wi-force"><input type="checkbox" v-model="mount.force" /> 强制上架（超限操作将写入审计日志）</label>
        </div>
        <p v-if="mount.err" class="err">{{ mount.err }}</p>
      </div>
      <div class="drawer-foot"><button class="btn" @click="mount.show=false">取消</button><button class="btn btn-primary" :disabled="!mount.cabinet_id||!mount.start_u||(mount.wi&&mount.wi.verdict==='over'&&!mount.force)" @click="doMount">确认上架</button></div>
    </div></div>

    <!-- V50 健康评估抽屉 -->
    <div v-if="health.show" class="drawer-mask" @click.self="health.show=false"><div class="drawer" style="width:640px">
      <div class="drawer-head"><h3><SvgIcon name="gauge" /> 健康评估 · {{ health.asset?.name }}</h3><button class="btn btn-mini" @click="health.show=false"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <div v-if="health.err" class="hg-err">{{ health.err }}</div>
        <template v-else-if="health.d">
          <div v-if="health.d.current" class="hg-hero">
            <div class="hg-score" :class="'hg-t-'+health.d.current.grade">{{ health.d.current.score }}</div>
            <div class="hg-meta">
              <div><span class="badge hg" :class="'hg-'+health.d.current.grade">{{ health.d.current.grade_label }}</span>
                <span class="muted small" style="margin-left:8px">评估于 {{ String(health.d.current.computed_at).slice(0,16).replace('T',' ') }}</span></div>
              <div class="muted small" style="margin-top:6px">绑定测点 {{ health.d.current.point_count }} 个 · 等级阈值 健康≥{{ health.d.thresholds.healthy }} / 关注≥{{ health.d.thresholds.attention }} / 亚健康≥{{ health.d.thresholds.sub_healthy }}</div>
              <div class="hg-actions">
                <button v-if="!health.d.open_work_order && (health.d.current.grade==='risk'||health.d.current.grade==='sub_healthy')" class="btn btn-primary btn-mini" @click="createHealthWO"><SvgIcon name="wrench" /> 创建预测性维护工单</button>
                <button v-if="health.d.open_work_order" class="btn btn-mini hg-wo-link" @click="goHealthWO"><SvgIcon name="clipboard" /> 维护工单 {{ health.d.open_work_order.code }} 处理中 ›</button>
                <button v-if="health.d.current.score < 60" class="btn btn-mini" @click="openRepl(health.asset)"><SvgIcon name="order" /> 替换建议</button>
              </div>
            </div>
          </div>
          <div v-else class="hg-empty">该资产尚无评估记录（评估引擎每 15 分钟自动运行一次，管理员可在系统设置手动触发）</div>

          <fieldset class="fs" v-if="health.d.current"><legend><SvgIcon name="search" /> 扣分因子（{{ health.d.current.factors.length }} 项）</legend>
            <div v-if="!health.d.current.factors.length" class="muted small">各项指标正常，无扣分项</div>
            <div v-for="fc in health.d.current.factors" :key="fc.key" class="hg-factor">
              <div class="hg-f-head"><b>{{ fc.label }}</b><span class="hg-deduct">-{{ fc.deduct }} 分</span></div>
              <div class="muted small">{{ fc.evidence }}</div>
            </div>
          </fieldset>

          <fieldset class="fs" v-if="(health.d.history||[]).length>1"><legend><SvgIcon name="trend" /> 健康分趋势（近90天）</legend>
            <svg class="hg-spark" viewBox="0 0 300 56" preserveAspectRatio="none"><polyline :points="healthSpark" /></svg>
            <div class="muted small hg-spark-axis"><span>{{ health.d.history[0].day }}</span><span>最新 {{ health.d.history[health.d.history.length-1].score }} 分</span><span>{{ health.d.history[health.d.history.length-1].day }}</span></div>
          </fieldset>

          <fieldset class="fs" v-if="(health.d.points||[]).length"><legend><SvgIcon name="signal" /> 绑定测点（{{ health.d.points.length }}）</legend>
            <div class="hg-chips">
              <span v-for="pt in health.d.points" :key="pt.id" class="hg-chip" :class="{'hg-chip-off':pt.status===0,'hg-chip-alarm':pt.status===2}">
                {{ pt.name }} · {{ ptTypeName(pt.type) }}<template v-if="pt.current_value!=null"> · {{ Number(pt.current_value).toFixed(1) }}{{ pt.unit }}</template>
              </span>
            </div>
          </fieldset>

          <fieldset class="fs" v-if="(health.d.alerts||[]).length"><legend><SvgIcon name="alert" /> 近期关联告警（{{ health.d.alerts.length }}）</legend>
            <div v-for="al in health.d.alerts" :key="al.id" class="hg-alert">
              <span class="hg-sev" :class="'hg-sev-'+al.severity">{{ sevName(al.severity) }}</span>
              <span class="small">{{ al.point_name }}：{{ al.message }}</span>
              <span class="muted small" style="margin-left:auto;flex-shrink:0">{{ String(al.triggered_at).slice(5,16).replace('T',' ') }}</span>
            </div>
          </fieldset>

          <fieldset class="fs" v-if="(health.d.maintenances||[]).length"><legend><SvgIcon name="shield" /> 维保合同（近3份）</legend>
            <div v-for="(m,i) in health.d.maintenances" :key="i" class="small hg-maint">
              {{ m.vendor||'-' }} · 到期 {{ String(m.end_at).slice(0,10) }} · <span :class="m.status===1?'hg-ok':'hg-warn'">{{ m.status===1?'生效中':'已过期/完成' }}</span>
            </div>
          </fieldset>
        </template>
        <div v-else class="muted small">加载中…</div>
      </div>
      <div class="drawer-foot"><button class="btn" @click="health.show=false">关闭</button></div>
    </div></div>
    <ReplacementDrawer v-if="repl.show" :asset-id="repl.asset.id" :asset-name="repl.asset.name" @close="repl.show=false" @created="onReplCreated" />
  </div>
</template>

<script setup>
import { ref,reactive,computed,onMounted,watch } from 'vue'
import { showConfirm, showPrompt, toastAuto } from '../ui-dialog'
import { useRoute,useRouter } from 'vue-router'
import api from '../api'; import Pagination from '../components/Pagination.vue'
import ReplacementDrawer from '../components/ReplacementDrawer.vue'

const route=useRoute(),router=useRouter()
function goCapacity(a){router.push({path:'/capacity',query:{site_id:a.site_id,cabinet_id:a.cabinet_id}})}
function goMaint(a){router.push({path:'/maintenances',query:{asset_id:a.id}})}
function goOrders(a){router.push({path:'/work-orders',query:{asset_id:a.id}})}

const CATS={server:'服务器',network:'网络设备',storage:'存储',power:'配电',cooling:'制冷',security:'安防',other:'其他'}
const ST={1:'在架',2:'在库',3:'维修中',4:'已报废'}
const HGS={healthy:'健康',attention:'关注',sub_healthy:'亚健康',risk:'风险'}
const TPL=[
  {icon:'',name:'标准1U服务器',desc:'1U机架式服务器，双路CPU，4盘位',category:'server',height_u:1,power_w:500,weight_kg:15},
  {icon:'',name:'2U存储服务器',desc:'2U机架式，12盘位，高存储',category:'server',height_u:2,power_w:800,weight_kg:25},
  {icon:'',name:'48口交换机',desc:'48口千兆+4口万兆上联',category:'network',height_u:1,power_w:150,weight_kg:5},
  {icon:'',name:'磁盘阵列',desc:'2U 12盘位 SAS存储',category:'storage',height_u:2,power_w:600,weight_kg:30},
  {icon:'',name:'智能PDU',desc:'0U垂直安装，24口',category:'power',height_u:0,power_w:0,weight_kg:3},
  {icon:'',name:'精密空调',desc:'机柜级精密空调',category:'cooling',height_u:4,power_w:3000,weight_kg:80},
  {icon:'',name:'监控摄像头',desc:'IP摄像头，PoE供电',category:'security',height_u:0,power_w:15,weight_kg:1},
  {icon:'',name:'UPS主机',desc:'机架式UPS，3kVA',category:'power',height_u:2,power_w:0,weight_kg:20},
]

const list=ref([]),total=ref(0),page=ref(1),size=20,sites=ref([]),rooms=ref([]),cabinets=ref([]),maintMap=ref({}),tplShow=ref(false)
const f=reactive({site_id:0,category:'',status:'',keyword:'',cabinet_id:0,health_grade:''})
// V16 批量选择/服役年限/CSV导出
const selIds=ref([])
const pageAllChecked=computed(()=>list.value.length>0&&list.value.every(a=>selIds.value.includes(a.id)))
function togglePageAll(e){
  if(e.target.checked){selIds.value=[...new Set([...selIds.value,...list.value.map(a=>a.id)])]}
  else{const ids=list.value.map(a=>a.id);selIds.value=selIds.value.filter(x=>!ids.includes(x))}
}
function serviceDays(a){return a.purchase_at?Math.floor((Date.now()-new Date(a.purchase_at).getTime())/86400000):0}
function serviceYears(a){const d=serviceDays(a);return d>=365?Math.floor(d/365)+'年'+(d%365>30?Math.round(d%365/30)+'月':''):d+'天'}
async function batchStatus(action){
  const labels={to_repair:'转维修',to_store:'转在库',to_scrap:'报废'}
  const note=await showPrompt({ title: `批量${labels[action]}`, label: `批量${labels[action]} ${selIds.value.length} 项资产，备注原因（可留空）：` })
  if(note===null)return
  const r=await api.post('/assets/batch',{ids:selIds.value,action,note})
  if(r.code===0){
    selIds.value=[]
    const rej=r.data?.rejected||[]
    if(rej.length)toastAuto(`完成 ${r.data.done} 项，跳过 ${rej.length} 项：\n`+rej.slice(0,10).join('\n')+(rej.length>10?'\n…':''))
    load()
  }else toastAuto(r.msg)
}
async function quickStatus(a,action){
  const labels={to_repair:'转维修',to_store:'修复入库',to_scrap:'报废'}
  if (!(await showConfirm({ message: `将「${a.name}」${labels[action]}？` })))return
  const r=await api.post('/assets/batch',{ids:[a.id],action,note:''})
  if(r.code===0){
    const rej=r.data?.rejected||[]
    if(rej.length)toastAuto(rej[0]);else load()
  }else toastAuto(r.msg)
}
async function exportCSV(){
  // 按当前筛选导出全量(不分页, 上限2000)
  const q={page:1,size:2000}
  for(const k of ['site_id','category','status','keyword','cabinet_id','health_grade'])if(f[k]!==''&&f[k]!==0)q[k]=f[k]
  const r=await api.get('/assets',q)
  const items=r.data?.items||[]
  if(!items.length){toastAuto('无数据可导出');return}
  const head=['资产编码','名称','分类','品牌','型号','序列号','站点','机柜','起始U','高度U','功耗W','重量kg','管理IP','资产编号','供应商','合同号','部门','采购日期','责任人','状态','服役天数','健康分','健康等级','备注']
  const lines=[head.join(',')]
  for(const a of items){
    lines.push([a.code,a.name,CATS[a.category]||a.category,a.brand,a.model,a.sn,
      (sites.value.find(s=>s.id===a.site_id)||{}).name||a.site_id,
      a.cabinet_id?cabName(a):'',a.cabinet_id&&a.status===1?a.start_u:'',a.height_u,a.power_w,a.weight_kg,
      a.ip,a.asset_no,a.vendor,a.contract_no,a.department,
      a.purchase_at?String(a.purchase_at).slice(0,10):'',a.owner,ST[a.status]||a.status,
      a.purchase_at?serviceDays(a):'',a.health_score??'',a.health_score!=null?(HGS[a.health_grade]||''):'',a.remark]
      .map(x=>`"${String(x??'').replace(/"/g,'""')}"`).join(','))
  }
  const blob=new Blob(['\ufeff'+lines.join('\n')],{type:'text/csv'})
  const el=document.createElement('a')
  el.href=URL.createObjectURL(blob)
  el.download=`dcim_assets_${new Date().toISOString().slice(0,10)}.csv`
  el.click();URL.revokeObjectURL(el.href)
}
const rec=reactive({show:false,asset:null,d:null})
async function openRec(a){rec.asset=a;rec.show=true;rec.d=null
  const r=await api.get(`/spare-boms/recommend/${a.id}`);if(r.code===0)rec.d=r.data}
// V50 健康评估抽屉
const health=reactive({show:false,asset:null,d:null,err:''})
const repl=reactive({show:false,asset:null})
function openRepl(a){if(!a)return;repl.asset=a;repl.show=true}
function onReplCreated(){repl.show=false;load()}
async function openHealth(a){health.asset=a;health.show=true;health.d=null;health.err=''
  const r=await api.get(`/asset-health/${a.id}`)
  if(r.code===0)health.d=r.data;else health.err=r.msg||'加载失败'}
// 90天趋势 sparkline（SVG polyline，随主题 currentColor）
const healthSpark=computed(()=>{
  const h=health.d?.history||[]
  if(h.length<2)return ''
  const W=300,H=56
  return h.map((x,i)=>`${(i/(h.length-1)*W).toFixed(1)},${(H-4-x.score/100*(H-8)).toFixed(1)}`).join(' ')})
const healthGradeLabel=g=>HGS[g]||g
const sevName=s=>({1:'提示',2:'警告',3:'严重'}[s]||s)
const ptTypeName=t=>({temp_humidity:'温湿度',ups:'UPS',power_dist:'配电',hvac:'空调',water_leak:'漏水',smoke:'烟感'}[t]||t)
async function createHealthWO(){
  if(!health.asset)return
  if (!(await showConfirm({ message: `为「${health.asset.name}」创建预测性维护工单？` })))return
  const r=await api.post(`/asset-health/${health.asset.id}/work-order`)
  if(r.code===0){toastAuto('已创建工单 '+r.data.code);openHealth(health.asset);load()}
  else toastAuto(r.msg||'创建失败')}
function goHealthWO(){if(health.d?.open_work_order)router.push({path:'/work-orders',query:{keyword:health.d.open_work_order.code}})}
const form=reactive({show:false,id:0,code:'',name:'',category:'server',site_id:0,room_id:0,brand:'',model:'',sn:'',ip:'',asset_no:'',fixed_asset_no:'',purchase_at:'',vendor:'',contract_no:'',owner:'',department:'',height_u:1,power_w:0,weight_kg:0,dimensions:'',remark:'',err:''})
const mount=reactive({show:false,asset:null,cabinet_id:0,start_u:1,err:'',wi:null,force:false})
const cap=ref(null)
const cabById=computed(()=>Object.fromEntries(cabinets.value.map(c=>[c.id,c])))
const roomName=id=>(rooms.value.find(r=>r.id===id)||{}).name||'-'
const cabName=a=>{if(!a.cabinet_id)return'-';const c=cabById.value[a.cabinet_id];return c?c.code:'#'+a.cabinet_id}
const cabPow=computed(()=>{const c=cabById.value[mount.cabinet_id];return c&&c.rated_power_kw>0?` / 额定 ${c.rated_power_kw}kW`:''})
const cabWt=computed(()=>{const c=cabById.value[mount.cabinet_id];return c&&c.max_weight_kg>0?` / 上限 ${c.max_weight_kg}kg`:''})

async function load(p){page.value=p||1;const q={page:page.value,size};for(const k of ['site_id','category','status','keyword','cabinet_id','health_grade'])if(f[k]!==''&&f[k]!==0)q[k]=f[k];const r=await api.get('/assets',q);if(r.code===0){list.value=r.data.items||[];total.value=r.data.total||0;loadMaint()}}
async function loadMaint(){const r=await api.get('/maintenances',{size:200});if(r.code===0){const m={};for(const x of r.data.items||[])if(x.status===1)m[x.asset_id]=true;maintMap.value=m}}
async function loadBase(){const [s,ro,ca]=await Promise.all([api.get('/sites',{size:100}),api.get('/rooms',{size:200}),api.get('/cabinets',{size:200})]);sites.value=s.data?.items||[];rooms.value=ro.data?.items||[];cabinets.value=ca.data?.items||[]}

onMounted(async()=>{
  await loadBase()
  // 深链：/assets?site_id=&category=&status=&keyword=
  const q=route.query
  if(q.site_id)f.site_id=Number(q.site_id)||0
  if(q.category)f.category=q.category
  if(q.status)f.status=q.status
  if(q.keyword)f.keyword=q.keyword
  if(q.cabinet_id)f.cabinet_id=Number(q.cabinet_id)||0
  if(q.health_grade)f.health_grade=q.health_grade
  load()
  // V76 全局搜索深链：/assets?id=N&detail=1 → 打开资产详情抽屉
  if(q.id&&q.detail==='1')openAssetById(Number(q.id))
})
// V76: 按 id 拉取资产并打开详情抽屉（后端 /assets 支持 id 过滤）
async function openAssetById(id){const r=await api.get('/assets',{id,size:1});const a=(r.data?.items||[])[0];if(a)openForm(a)}
// V50: Dashboard 健康卡穿透（应用内跳转同组件不重挂载，watch 兜底）
watch(()=>route.query,q=>{
  if(q.health_grade!==undefined&&q.health_grade!==f.health_grade){f.health_grade=q.health_grade||'';load(1)}
  if(!q.health_grade&&f.health_grade&&route.path==='/assets'){/* 保持当前筛选 */}
  if(q.id&&q.detail==='1'&&route.path==='/assets')openAssetById(Number(q.id))
})

function applyTpl(t){tplShow.value=false;Object.assign(form,{show:true,id:0,code:'',name:t.name,category:t.category,site_id:sites.value[0]?.id||0,room_id:0,brand:'',model:'',sn:'',ip:'',asset_no:'',fixed_asset_no:'',purchase_at:'',vendor:'',contract_no:'',owner:'',department:'',height_u:t.height_u,power_w:t.power_w,weight_kg:t.weight_kg,dimensions:'',remark:'',err:''})}
function openForm(a){if(a)Object.assign(form,{show:true,err:'',id:a.id,code:a.code,name:a.name,category:a.category,site_id:a.site_id,room_id:a.room_id||0,brand:a.brand||'',model:a.model||'',sn:a.sn||'',ip:a.ip||'',asset_no:a.asset_no||'',fixed_asset_no:a.fixed_asset_no||'',purchase_at:(a.purchase_at||'').slice(0,10),vendor:a.vendor||'',contract_no:a.contract_no||'',owner:a.owner||'',department:a.department||'',height_u:a.height_u||1,power_w:a.power_w||0,weight_kg:a.weight_kg||0,dimensions:a.dimensions||'',remark:a.remark||''});else Object.assign(form,{show:true,err:'',id:0,code:'',name:'',category:'server',site_id:sites.value[0]?.id||0,room_id:0,brand:'',model:'',sn:'',ip:'',asset_no:'',fixed_asset_no:'',purchase_at:'',vendor:'',contract_no:'',owner:'',department:'',height_u:1,power_w:0,weight_kg:0,dimensions:'',remark:''})}
async function saveForm(){form.err='';if(!form.name||(!form.id&&!form.code)||!form.site_id){form.err='编码、名称、站点必填';return};const r=form.id?await api.put('/assets/'+form.id,{...form}):await api.post('/assets',{...form});if(r.code===0){form.show=false;load()}else form.err=r.msg}

function openMount(a){Object.assign(mount,{show:true,asset:a,cabinet_id:0,start_u:1,err:'',wi:null,force:false});cap.value=null}
async function loadCap(){cap.value=null;mount.wi=null;mount.force=false;if(!mount.cabinet_id)return;const r=await api.get(`/cabinets/${mount.cabinet_id}/capacity`);if(r.code===0)cap.value=r.data;const a=mount.asset;if(a){const w=await api.get(`/cabinets/${mount.cabinet_id}/what-if`,{height_u:a.height_u||1,power_w:a.power_w||0,weight_kg:a.weight_kg||0});if(w.code===0)mount.wi=w.data}}
async function doMount(){mount.err='';const force=(mount.wi&&mount.wi.verdict==='over'&&mount.force)?1:0;const r=await api.post(`/assets/${mount.asset.id}/mount`,{cabinet_id:mount.cabinet_id,start_u:mount.start_u,force});if(r.code===0){mount.show=false;load()}else mount.err=r.msg}
async function doUnmount(a){if (!(await showConfirm({ message: `下架「${a.name}」？` })))return;const r=await api.post(`/assets/${a.id}/unmount`);if(r.code===0)load()}
async function doDelete(a){if (!(await showConfirm({ message: `删除「${a.name}」？` })))return;const r=await api.del('/assets/'+a.id);if(r.code===0)load()}
</script>

<style scoped>
.age-warn{color:#e6a23c;font-weight:700}
.badge.st-1{color:#16a34a;border-color:rgba(34,197,94,.4)}.badge.st-2{color:#2563eb;border-color:rgba(59,130,246,.4)}.badge.st-3{color:#d97706;border-color:rgba(245,158,11,.4)}.badge.st-4{color:#64748b;border-color:rgba(148,163,184,.4)}.badge.st-ok{color:#16a34a;border-color:rgba(34,197,94,.4)}
.tpl-grid{display:grid;grid-template-columns:repeat(4,1fr);gap:10px;padding:4px}.tpl-item{border:1px solid var(--border);border-radius:10px;padding:12px;cursor:pointer;transition:all .2s}.tpl-item:hover{border-color:var(--accent);background:rgba(64,158,255,.05);transform:translateY(-2px)}.tpl-icon{font-size:24px;margin-bottom:4px}.tpl-name{font-weight:600;font-size:13px}.tpl-desc{font-size:11px;color:var(--muted);margin-top:2px}
.rec-warn{color:#ffa726;margin-left:10px;font-weight:600}
.rec-low{color:#ff5252;font-weight:700}
.badge.st-2{color:#40c4ff;border-color:rgba(64,196,255,.4)}
.drawer-mask{position:fixed;inset:0;background:rgba(0,0,0,.35);z-index:1000;display:flex;justify-content:flex-end}.drawer{width:600px;max-width:95vw;height:100%;background:var(--bg);box-shadow:-4px 0 20px rgba(0,0,0,.15);display:flex;flex-direction:column;overflow:hidden}.drawer-head{display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--border);flex-shrink:0}.drawer-head h3{margin:0;font-size:18px}.drawer-body{flex:1;overflow-y:auto;padding:20px}.drawer-foot{padding:14px 20px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;gap:10px;flex-shrink:0}
.fs{border:1px solid var(--border);border-radius:8px;padding:16px;margin-bottom:16px}.fs legend{font-weight:600;font-size:14px;padding:0 8px;color:var(--accent)}
/* V50 健康度 */
.badge.hg{font-weight:600}
.hg-healthy{color:#16a34a;border-color:rgba(34,197,94,.45)}
.hg-attention{color:#2563eb;border-color:rgba(59,130,246,.45)}
.hg-sub_healthy{color:#d97706;border-color:rgba(245,158,11,.5)}
.hg-risk{color:#dc2626;border-color:rgba(239,68,68,.55)}
.hg-hero{display:flex;gap:20px;align-items:center;border:1px solid var(--border);border-radius:12px;padding:18px 20px;margin-bottom:16px;background:linear-gradient(135deg,rgba(64,158,255,.04),transparent)}
.hg-score{font-size:52px;font-weight:800;line-height:1;font-variant-numeric:tabular-nums}
.hg-t-healthy{color:#16a34a}.hg-t-attention{color:#2563eb}.hg-t-sub_healthy{color:#d97706}.hg-t-risk{color:#dc2626}
.hg-meta{flex:1;min-width:0}
.hg-actions{margin-top:10px;display:flex;gap:8px;flex-wrap:wrap}
.hg-wo-link{color:var(--accent)}
.hg-empty{padding:24px;text-align:center;color:var(--muted);font-size:13px;border:1px dashed var(--border);border-radius:10px;margin-bottom:16px}
.hg-err{padding:16px;color:var(--red);font-size:13px}
.hg-factor{padding:10px 12px;border:1px solid var(--border);border-radius:8px;margin-bottom:8px}
.hg-f-head{display:flex;justify-content:space-between;align-items:center;font-size:13px;margin-bottom:4px}
.hg-deduct{color:#dc2626;font-weight:700;font-size:13px}
.hg-spark{width:100%;height:56px;display:block}
.hg-spark polyline{fill:none;stroke:var(--accent);stroke-width:2;stroke-linejoin:round;stroke-linecap:round}
.hg-spark-axis{display:flex;justify-content:space-between;margin-top:4px}
.hg-chips{display:flex;flex-wrap:wrap;gap:6px}
.hg-chip{font-size:12px;padding:4px 10px;border-radius:999px;border:1px solid var(--border);background:rgba(64,158,255,.05)}
.hg-chip-off{opacity:.55;border-style:dashed}
.hg-chip-alarm{border-color:rgba(239,68,68,.5);color:#dc2626;background:rgba(239,68,68,.06)}
.hg-alert{display:flex;align-items:center;gap:8px;padding:7px 0;border-bottom:1px dashed var(--border)}
.hg-alert:last-child{border-bottom:none}
.hg-sev{font-size:11px;padding:2px 8px;border-radius:999px;flex-shrink:0;font-weight:600}
.hg-sev-1{background:rgba(59,130,246,.12);color:#2563eb}
.hg-sev-2{background:rgba(245,158,11,.14);color:#d97706}
.hg-sev-3{background:rgba(239,68,68,.12);color:#dc2626}
.hg-maint{padding:4px 0}
.hg-ok{color:#16a34a}.hg-warn{color:#d97706}
.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.form-grid label{display:flex;flex-direction:column;gap:4px;font-size:13px;color:var(--muted)}
.cap-summary{display:flex;gap:18px;margin-top:12px;padding:10px 14px;background:var(--bg2);border-radius:8px;font-size:12.5px;color:var(--text2)}.cap-summary b{color:var(--text)}
.link{color:var(--accent);cursor:pointer;font-weight:600}.link:hover{text-decoration:underline}
a.badge.link{text-decoration:none}a.badge.link:hover{text-decoration:underline}
.wi-box{margin-top:12px;padding:10px 12px;border:1px solid var(--border);border-radius:8px;background:var(--bg2)}
.wi-title{display:flex;align-items:center;gap:6px;font-size:12.5px;font-weight:600;color:var(--text2);margin-bottom:8px}
.wi-row{display:flex;align-items:flex-start;gap:8px;padding:5px 0;font-size:12.5px;border-bottom:1px dashed var(--border)}
.wi-row:last-of-type{border-bottom:none}
.wi-ic{flex-shrink:0;margin-top:2px}
.wi-label{flex-shrink:0;width:34px;font-weight:600}
.wi-msg{color:var(--text2);line-height:1.5}
.wi-ok .wi-ic,.wi-ok .wi-label{color:#16a34a}
.wi-warn .wi-ic,.wi-warn .wi-label{color:#d97706}
.wi-over .wi-ic,.wi-over .wi-label{color:#dc2626}
.wi-na .wi-ic,.wi-na .wi-label{color:var(--muted)}
.wi-verdict{margin-top:8px;padding:7px 10px;border-radius:6px;font-size:12.5px;font-weight:600}
.wv-ok{background:rgba(22,163,74,.08);color:#16a34a}
.wv-warn{background:rgba(217,119,6,.08);color:#d97706}
.wv-over{background:rgba(220,38,38,.08);color:#dc2626}
.wi-force{display:flex;align-items:center;gap:6px;margin-top:8px;font-size:12.5px;color:#dc2626;cursor:pointer}
</style>