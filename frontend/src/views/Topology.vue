<template>
  <div>
    <div class="page-head"><h1>基础设施层级</h1>
      <div class="head-ops">
        <router-link to="/capacity" class="btn"><SvgIcon name="chart" />机柜容量总览</router-link>
        <router-link to="/room-3d" class="btn"><SvgIcon name="cube" />3D机房</router-link>
      </div>
    </div>

    <!-- ===== 站点 ===== -->
    <div class="toolbar">
      <input class="search-input input" v-model="siteKw" placeholder="搜索站点（名称/编码/地址）" @input="loadSites" />
      <button v-if="auth.isGlobalAdmin" class="btn btn-primary" @click="openSiteForm()">＋ 新增站点</button>
    </div>
    <div class="card">
      <table>
        <thead><tr>
          <th>站点编码</th><th>名称</th><th>等级</th><th>地址</th><th>负责人</th><th>状态</th><th>备注</th><th style="width:170px">操作</th>
        </tr></thead>
        <tbody>
          <tr v-for="s in sites" :key="s.id" :class="{ 'row-active': curSite && curSite.id === s.id }">
            <td><code>{{ s.code }}</code></td>
            <td><a class="link" @click="selectSite(s)">{{ s.name }}</a></td>
            <td><span class="tag">{{ LEVELS[s.level] || '-' }}</span></td>
            <td class="muted">{{ s.address || '-' }}</td>
            <td class="muted">{{ s.manager || '-' }}</td>
            <td><span class="tag" :class="s.status === 1 ? 'tag-on' : 'tag-off'">{{ s.status === 1 ? '在用' : '停用' }}</span></td>
            <td class="muted">{{ s.remark || '-' }}</td>
            <td>
              <button class="btn btn-sm btn-outline" @click="selectSite(s)">下级</button>
              <button v-if="auth.isGlobalAdmin" class="btn btn-sm btn-outline" @click="openSiteForm(s)">编辑</button>
              <button v-if="auth.isGlobalAdmin" class="btn btn-sm btn-ghost" @click="delSite(s)">删除</button>
            </td>
          </tr>
          <tr v-if="!sites.length"><td colspan="8" class="empty-cell">暂无站点数据</td></tr>
        </tbody>
      </table>
    </div>

    <!-- ===== 选中站点的下级：房间 → 机柜列 → 机柜 ===== -->
    <template v-if="curSite">
      <div class="page-head" style="margin-top:28px"><h1>{{ curSite.name }} · 房间</h1></div>
      <div class="toolbar">
        <button v-if="auth.isAdmin" class="btn btn-primary" @click="openRoomForm()">＋ 新增房间</button>
      </div>
      <div class="card">
        <table>
          <thead><tr>
            <th>房间</th><th>楼层</th><th>面积(㎡)</th><th>防护等级</th><th>机柜列</th><th>机柜</th><th>测点</th><th>活跃告警</th><th style="width:200px">操作</th>
          </tr></thead>
          <tbody>
            <tr v-for="r in rooms" :key="r.id" :class="{ 'row-active': curRoom && curRoom.id === r.id }">
              <td><a class="link" @click="selectRoom(r)">{{ r.name }}</a></td>
              <td>{{ r.floor || '-' }}</td>
              <td>{{ r.area || '-' }}</td>
              <td>{{ r.grade || '-' }}</td>
              <td>{{ roomStat(r.id).cabinet_cnt ? rowCount(r.id) + ' 列' : rowCount(r.id) + ' 列' }}</td>
              <td>{{ roomStat(r.id).cabinet_cnt }}</td>
              <td><span class="muted">{{ roomStat(r.id).point_cnt }}</span></td>
              <td><span v-if="roomStat(r.id).alert_cnt" class="alert-badge" @click="$router.push({path:'/alert-wall'})" title="点击进入告警盯屏"><SvgIcon name="bell" />{{ roomStat(r.id).alert_cnt }}</span><span v-else class="muted">-</span></td>
              <td>
                <button class="btn btn-sm btn-outline" @click="selectRoom(r)">下级</button>
                <button v-if="auth.isAdmin" class="btn btn-sm btn-outline" @click="openRoomForm(r)">编辑</button>
                <button v-if="auth.isAdmin" class="btn btn-sm btn-ghost" @click="delRoom(r)">删除</button>
              </td>
            </tr>
            <tr v-if="!rooms.length"><td colspan="9" class="empty-cell">该站点暂无房间</td></tr>
          </tbody>
        </table>
      </div>
    </template>

    <template v-if="curRoom">
      <div class="page-head" style="margin-top:28px"><h1>{{ curRoom.name }} · 机柜列与机柜</h1></div>
      <div class="toolbar">
        <button v-if="auth.isAdmin" class="btn btn-primary" @click="openRowForm()">＋ 新增机柜列</button>
        <button v-if="auth.isAdmin" class="btn btn-outline" @click="openCabForm()">＋ 新增机柜</button>
        <span style="flex:1"></span>
        <button class="btn" @click="loadTopoStats">↻ 刷新统计</button>
      </div>
      <div class="card">
        <table>
          <thead><tr>
            <th>机柜列</th><th>机柜数</th><th>备注</th><th style="width:150px">操作</th>
          </tr></thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id">
              <td>{{ row.name }}</td>
              <td>{{ cabinets.filter(c=>c.row_id===row.id).length }}</td>
              <td class="muted">{{ row.remark || '-' }}</td>
              <td>
                <button v-if="auth.isAdmin" class="btn btn-sm btn-outline" @click="openRowForm(row)">编辑</button>
                <button v-if="auth.isAdmin" class="btn btn-sm btn-ghost" @click="delRow(row)">删除</button>
              </td>
            </tr>
            <tr v-if="!rows.length"><td colspan="4" class="empty-cell">该房间暂无机柜列</td></tr>
          </tbody>
        </table>
      </div>
      <div class="card" style="margin-top:16px">
        <div class="card-title">机柜清单（{{ cabinets.length }}）</div>
        <table>
          <thead><tr>
            <th>机柜编号</th><th>所属列</th><th>型号</th><th>U位占用</th><th>功率</th><th>设备</th><th>额定(kW)</th><th>承重(kg)</th><th>状态</th><th style="width:230px">操作</th>
          </tr></thead>
          <tbody>
            <tr v-for="c in cabinets" :key="c.id">
              <td><code>{{ c.code }}</code></td>
              <td>{{ rowName(c.row_id) || '-' }}</td>
              <td class="muted">{{ c.model || '-' }}</td>
              <td>
                <div class="u-bar"><div class="u-bar-in" :class="uPctClass(c)" :style="{width: cabStat(c.id).uPct+'%'}"></div></div>
                <span class="u-txt">{{ cabStat(c.id).used_u }}/{{ cabStat(c.id).total_u||c.total_u }}U</span>
              </td>
              <td><span :class="kwClass(c)">{{ cabStat(c.id).used_kw.toFixed(2) }}kW</span></td>
              <td>{{ cabStat(c.id).asset_cnt }}</td>
              <td>{{ c.rated_power_kw || '-' }}</td>
              <td>{{ c.max_weight_kg || '-' }}</td>
              <td><span class="tag" :class="c.status === 1 ? 'tag-on' : 'tag-off'">{{ c.status === 1 ? '在用' : '停用' }}</span></td>
              <td>
                <button class="btn btn-sm btn-outline" @click="goCapacity(c)">容量</button>
                <button class="btn btn-sm btn-outline" @click="go3D(c)">3D</button>
                <button class="btn btn-sm btn-outline" @click="goAssets(c)">设备</button>
                <button v-if="auth.isAdmin" class="btn btn-sm btn-outline" @click="openCabForm(c)">编辑</button>
                <button v-if="auth.isAdmin" class="btn btn-sm btn-ghost" @click="delCab(c)">删除</button>
              </td>
            </tr>
            <tr v-if="!cabinets.length"><td colspan="10" class="empty-cell">该房间暂无机柜</td></tr>
          </tbody>
        </table>
      </div>
    </template>

    <!-- ===== V16 抽屉：站点 ===== -->
    <div v-if="siteForm.show" class="drawer-mask" @click.self="siteForm.show=false">
      <div class="drawer"><div class="drawer-head"><h3>{{ siteForm.id ? '编辑站点' : '新增站点' }}</h3><button class="btn btn-mini" @click="siteForm.show=false"><SvgIcon name="x" /></button></div>
        <div class="drawer-body"><fieldset class="fs"><legend><SvgIcon name="pin" />站点信息</legend><div class="form-grid">
          <label>编码 *<input class="input" v-model="siteForm.code" placeholder="如 BJ-DC1" :disabled="!!siteForm.id" /></label>
          <label>名称 *<input class="input" v-model="siteForm.name" placeholder="如 北京一号数据中心" /></label>
          <label>等级<select class="input" v-model.number="siteForm.level"><option v-for="(n,k) in LEVELS" :key="k" :value="Number(k)">{{ n }}</option></select></label>
          <label>状态<select class="input" v-model.number="siteForm.status"><option :value="1">在用</option><option :value="0">停用</option></select></label>
          <label class="full">地址<input class="input" v-model="siteForm.address" /></label>
          <label class="full">所属省份（能效合规政策匹配）<select class="input" v-model="siteForm.province"><option value="">未设置</option><option value="北京">北京</option><option value="上海">上海</option><option value="天津">天津</option><option value="重庆">重庆</option><option value="河北">河北</option><option value="山西">山西</option><option value="辽宁">辽宁</option><option value="吉林">吉林</option><option value="黑龙江">黑龙江</option><option value="江苏">江苏</option><option value="浙江">浙江</option><option value="安徽">安徽</option><option value="福建">福建</option><option value="江西">江西</option><option value="山东">山东</option><option value="河南">河南</option><option value="湖北">湖北</option><option value="湖南">湖南</option><option value="广东">广东</option><option value="海南">海南</option><option value="四川">四川</option><option value="贵州">贵州</option><option value="云南">云南</option><option value="陕西">陕西</option><option value="甘肃">甘肃</option><option value="青海">青海</option><option value="内蒙古">内蒙古</option><option value="广西">广西</option><option value="西藏">西藏</option><option value="宁夏">宁夏</option><option value="新疆">新疆</option><option value="香港">香港</option><option value="澳门">澳门</option><option value="台湾">台湾</option></select></label>
          <label>负责人<input class="input" v-model="siteForm.manager" /></label>
          <label>联系电话<input class="input" v-model="siteForm.phone" /></label>
          <label class="full">备注<input class="input" v-model="siteForm.remark" /></label>
        </div></fieldset>
        <div class="err" v-if="siteForm.err">{{ siteForm.err }}</div></div>
        <div class="drawer-foot"><button class="btn" @click="siteForm.show=false">取消</button><button class="btn btn-primary" @click="saveSite">保存</button></div>
      </div>
    </div>

    <!-- ===== V16 抽屉：房间 ===== -->
    <div v-if="roomForm.show" class="drawer-mask" @click.self="roomForm.show=false">
      <div class="drawer"><div class="drawer-head"><h3>{{ roomForm.id ? '编辑房间' : '新增房间' }}</h3><button class="btn btn-mini" @click="roomForm.show=false"><SvgIcon name="x" /></button></div>
        <div class="drawer-body"><fieldset class="fs"><legend><SvgIcon name="pin" />房间信息</legend><div class="form-grid">
          <label class="full">名称 *<input class="input" v-model="roomForm.name" placeholder="如 A栋1层主机房" /></label>
          <label>楼层<input class="input" v-model="roomForm.floor" placeholder="如 1F" /></label>
          <label>面积(㎡)<input class="input" type="number" step="0.1" v-model.number="roomForm.area" /></label>
          <label>防护等级<input class="input" v-model="roomForm.grade" placeholder="如 机房A级" /></label>
          <label class="full">备注<input class="input" v-model="roomForm.remark" /></label>
        </div></fieldset>
        <div class="err" v-if="roomForm.err">{{ roomForm.err }}</div></div>
        <div class="drawer-foot"><button class="btn" @click="roomForm.show=false">取消</button><button class="btn btn-primary" @click="saveRoom">保存</button></div>
      </div>
    </div>

    <!-- ===== V16 抽屉：机柜列 ===== -->
    <div v-if="rowForm.show" class="drawer-mask" @click.self="rowForm.show=false">
      <div class="drawer"><div class="drawer-head"><h3>{{ rowForm.id ? '编辑机柜列' : '新增机柜列' }}</h3><button class="btn btn-mini" @click="rowForm.show=false"><SvgIcon name="x" /></button></div>
        <div class="drawer-body"><fieldset class="fs"><legend><SvgIcon name="pin" />机柜列信息</legend><div class="form-grid">
          <label>名称 *<input class="input" v-model="rowForm.name" placeholder="如 A列" /></label>
          <label>通道类型<select class="input" v-model="rowForm.channel"><option value="">未指定</option><option value="cold">冷通道</option><option value="hot">热通道</option><option value="none">无</option></select></label>
          <label class="full">备注<input class="input" v-model="rowForm.remark" /></label>
        </div></fieldset>
        <p class="muted small">通道类型用于 3D 机房冷热通道布局展示（两列面对面=冷通道封闭）。</p>
        <div class="err" v-if="rowForm.err">{{ rowForm.err }}</div></div>
        <div class="drawer-foot"><button class="btn" @click="rowForm.show=false">取消</button><button class="btn btn-primary" @click="saveRow">保存</button></div>
      </div>
    </div>

    <!-- ===== V16 抽屉：机柜 ===== -->
    <div v-if="cabForm.show" class="drawer-mask" @click.self="cabForm.show=false">
      <div class="drawer"><div class="drawer-head"><h3>{{ cabForm.id ? '编辑机柜' : '新增机柜' }}</h3><button class="btn btn-mini" @click="cabForm.show=false"><SvgIcon name="x" /></button></div>
        <div class="drawer-body"><fieldset class="fs"><legend><SvgIcon name="pin" />机柜信息</legend><div class="form-grid">
          <label>编号 *<input class="input" v-model="cabForm.code" placeholder="如 A01" :disabled="!!cabForm.id" /></label>
          <label>所属列 *<select class="input" v-model.number="cabForm.row_id"><option v-for="r in rows" :key="r.id" :value="r.id">{{ r.name }}</option></select></label>
          <label>U位数<input class="input" type="number" v-model.number="cabForm.total_u" /></label>
          <label>型号<input class="input" v-model="cabForm.model" placeholder="如 42U标准服务器柜" /></label>
          <label>额定功率(kW)<input class="input" type="number" step="0.1" v-model.number="cabForm.rated_power_kw" /></label>
          <label>承重上限(kg)<input class="input" type="number" v-model.number="cabForm.max_weight_kg" /></label>
          <label>状态<select class="input" v-model.number="cabForm.status"><option :value="1">在用</option><option :value="2">预留</option><option :value="0">停用</option></select></label>
          <label class="full">备注<input class="input" v-model="cabForm.remark" /></label>
        </div></fieldset>
        <p v-if="cabForm.id && cabStat(cabForm.id).used_u>0" class="muted small">当前已用 {{ cabStat(cabForm.id).used_u }}U，U位数不可小于该值。</p>
        <div class="err" v-if="cabForm.err">{{ cabForm.err }}</div></div>
        <div class="drawer-foot"><button class="btn" @click="cabForm.show=false">取消</button><button class="btn btn-primary" @click="saveCab">保存</button></div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import { showConfirm, toastAuto } from '../ui-dialog'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()

const LEVELS = { 1: '核心级', 2: '区域级', 3: '边缘级' }

const siteKw = ref('')
const sites = ref([])
const curSite = ref(null)
const rooms = ref([])
const curRoom = ref(null)
const rows = ref([])
const cabinets = ref([])
// V16 统计: 房间(测点/告警/机柜数) + 机柜(U占用/功率/设备数)
const roomStats = ref({})
const cabStats = ref({})


function errMsg(e) { return e.response?.data?.msg || e.message || '操作失败' }
const roomStat = id => roomStats.value[id] || { point_cnt: 0, alert_cnt: 0, cabinet_cnt: 0 }
function cabStat(id) {
  const s = cabStats.value[id]
  if (!s) return { used_u: 0, total_u: 0, asset_cnt: 0, used_kw: 0, uPct: 0 }
  const pct = s.total_u > 0 ? Math.min(100, Math.round(s.used_u / s.total_u * 100)) : 0
  return { ...s, uPct: pct }
}
function uPctClass(c) {
  const p = cabStat(c.id).uPct
  return p >= 90 ? 'u-hot' : p >= 70 ? 'u-warm' : 'u-ok'
}
function kwClass(c) {
  const s = cabStat(c.id)
  if (!c.rated_power_kw) return 'muted'
  const p = s.used_kw / c.rated_power_kw * 100
  return p >= 90 ? 'c-hot' : p >= 70 ? 'c-warm' : ''
}

async function loadTopoStats() {
  const params = {}
  if (curSite.value) params.site_id = curSite.value.id
  const r = await api.get('/topology-stats', params)
  if (r.code !== 0) return
  const rm = {}, cm = {}
  for (const x of (r.data?.rooms || [])) rm[x.room_id] = x
  for (const x of (r.data?.cabinets || [])) cm[x.cabinet_id] = x
  roomStats.value = rm
  cabStats.value = cm
}

// ---------- 站点 ----------
async function loadSites() {
  const res = await api.get('/sites', { page: 1, size: 200, keyword: siteKw.value || undefined })
  if (res.code === 0) sites.value = res.data.items || []
}
function selectSite(s) {
  curSite.value = s
  curRoom.value = null
  rows.value = []
  cabinets.value = []
  loadRooms()
  loadTopoStats()
}
function openSiteForm(s) {
  siteForm.value = { show: true, err: '', id: s?.id, code: s?.code || '', name: s?.name || '', level: s?.level || 1, address: s?.address || '', province: s?.province || '', manager: s?.manager || '', phone: s?.phone || '', status: s?.status ?? 1, remark: s?.remark || '' }
}
const siteForm = ref({ show: false })
async function saveSite() {
  const f = siteForm.value
  f.err = ''
  const payload = { code: f.code.trim(), name: f.name.trim(), level: f.level, address: f.address, province: f.province || '', lng: 0, lat: 0, manager: f.manager, phone: f.phone, status: f.status, remark: f.remark }
  if (!payload.code || !payload.name) { f.err = '编码与名称必填'; return }
  try {
    const res = f.id ? await api.put('/sites/' + f.id, payload) : await api.post('/sites', payload)
    if (res.code !== 0) { f.err = res.msg; return }
    f.show = false
    loadSites()
  } catch (e) { f.err = errMsg(e) }
}
async function delSite(s) {
  if (!(await showConfirm({ title: '删除站点', message: `确认删除站点「${s.name}」？站点下存在房间/机柜/资产/测点等业务数据时将被拒绝。` }))) return
  try {
    const res = await api.del('/sites/' + s.id)
    if (res.code !== 0) toastAuto(res.msg)
    else { if (curSite.value?.id === s.id) { curSite.value = null; rooms.value = [] } loadSites() }
  } catch (e) { toastAuto(errMsg(e)) }
}

// ---------- 房间 ----------
async function loadRooms() {
  const res = await api.get('/rooms', { page: 1, size: 200, site_id: curSite.value.id })
  if (res.code === 0) rooms.value = res.data.items || []
}
const roomForm = ref({ show: false })
function openRoomForm(r) {
  roomForm.value = { show: true, err: '', id: r?.id, name: r?.name || '', floor: r?.floor || '', area: r?.area, grade: r?.grade || '', remark: r?.remark || '' }
}
async function saveRoom() {
  const f = roomForm.value
  f.err = ''
  const payload = { site_id: curSite.value.id, name: f.name.trim(), floor: f.floor, area: f.area, grade: f.grade, remark: f.remark }
  if (!payload.name) { f.err = '名称必填'; return }
  try {
    const res = f.id ? await api.put('/rooms/' + f.id, payload) : await api.post('/rooms', payload)
    if (res.code !== 0) { f.err = res.msg; return }
    f.show = false
    loadRooms()
    loadTopoStats()
  } catch (e) { f.err = errMsg(e) }
}
async function delRoom(r) {
  if (!(await showConfirm({ title: '删除房间', message: `确认删除房间「${r.name}」？房间下存在机柜列/机柜/测点时将被拒绝。` }))) return
  try {
    const res = await api.del('/rooms/' + r.id)
    if (res.code !== 0) toastAuto(res.msg)
    else { if (curRoom.value?.id === r.id) { curRoom.value = null; rows.value = []; cabinets.value = [] } loadRooms(); loadTopoStats() }
  } catch (e) { toastAuto(errMsg(e)) }
}

// ---------- 机柜列 ----------
async function loadRows() {
  const res = await api.get('/rows', { page: 1, size: 200, room_id: curRoom.value.id })
  if (res.code === 0) rows.value = res.data.items || []
}
const rowForm = ref({ show: false })
function openRowForm(r) {
  rowForm.value = { show: true, err: '', id: r?.id, name: r?.name || '', channel: r?.channel || '', remark: r?.remark || '' }
}
async function saveRow() {
  const f = rowForm.value
  f.err = ''
  const payload = { room_id: curRoom.value.id, site_id: curSite.value.id, name: f.name.trim(), channel: f.channel, remark: f.remark }
  if (!payload.name) { f.err = '名称必填'; return }
  try {
    const res = f.id ? await api.put('/rows/' + f.id, payload) : await api.post('/rows', payload)
    if (res.code !== 0) { f.err = res.msg; return }
    f.show = false
    loadRows()
  } catch (e) { f.err = errMsg(e) }
}
async function delRow(r) {
  if (!(await showConfirm({ title: '删除机柜列', message: `确认删除机柜列「${r.name}」？列下存在机柜时将被拒绝。` }))) return
  try {
    const res = await api.del('/rows/' + r.id)
    if (res.code !== 0) toastAuto(res.msg)
    else loadRows()
  } catch (e) { toastAuto(errMsg(e)) }
}
function rowCount(roomId) {
  return rows.value.filter(r => r.room_id === roomId).length || (roomStats.value[roomId]?.cabinet_cnt ? '—' : 0)
}
function rowName(id) {
  const r = rows.value.find(x => x.id === id)
  return r ? r.name : ''
}

// ---------- 机柜 ----------
async function loadCabinets() {
  const res = await api.get('/cabinets', { page: 1, size: 500, room_id: curRoom.value.id })
  if (res.code === 0) cabinets.value = res.data.items || []
}
const cabForm = ref({ show: false })
function openCabForm(c) {
  cabForm.value = { show: true, err: '', id: c?.id, code: c?.code || '', row_id: c?.row_id || (rows.value[0]?.id), total_u: c?.total_u || 42, rated_power_kw: c?.rated_power_kw, max_weight_kg: c?.max_weight_kg, model: c?.model || '', status: c?.status ?? 1, remark: c?.remark || '' }
  if (!rows.value.length) cabForm.value.err = '请先创建机柜列'
}
function goCapacity(c) {
  router.push({ path: '/capacity', query: { site_id: curSite.value.id, cabinet_id: c.id } })
}
function go3D(c) {
  router.push({ path: '/room-3d', query: { cabinet_id: c.id } })
}
function goAssets(c) {
  router.push({ path: '/assets', query: { cabinet_id: c.id } })
}
async function saveCab() {
  const f = cabForm.value
  f.err = ''
  const payload = { room_id: curRoom.value.id, site_id: curSite.value.id, row_id: f.row_id, code: f.code.trim(), model: f.model, total_u: f.total_u, rated_power_kw: f.rated_power_kw, max_weight_kg: f.max_weight_kg, status: f.status, remark: f.remark }
  if (!payload.code) { f.err = '编号必填'; return }
  if (!payload.row_id) { f.err = '请选择所属机柜列'; return }
  try {
    const res = f.id ? await api.put('/cabinets/' + f.id, payload) : await api.post('/cabinets', payload)
    if (res.code !== 0) { f.err = res.msg; return }
    f.show = false
    loadCabinets()
    loadTopoStats()
  } catch (e) { f.err = errMsg(e) }
}
async function delCab(c) {
  if (!(await showConfirm({ title: '删除机柜', message: `确认删除机柜「${c.code}」？柜内有在架资产/关联测点/供电绑定时将被拒绝。` }))) return
  try {
    const res = await api.del('/cabinets/' + c.id)
    if (res.code !== 0) toastAuto(res.msg)
    else { loadCabinets(); loadTopoStats() }
  } catch (e) { toastAuto(errMsg(e)) }
}
function selectRoom(r) {
  curRoom.value = r
  loadRows()
  loadCabinets()
  loadTopoStats()
}

onMounted(loadSites)
</script>

<style scoped>
.search-input { width: 260px; }
.row-active { background: var(--accent-glow); }
.empty-cell { text-align: center; color: var(--muted); padding: 24px !important; }
.link { color: var(--accent); cursor: pointer; font-weight: 600; }
.link:hover { text-decoration: underline; }
.muted { color: var(--muted); }
code { background: var(--card2); padding: 1px 6px; border-radius: 4px; font-size: 12px; }
.page-head { display: flex; align-items: center; justify-content: space-between; }
.head-ops { display: flex; gap: 8px; }
.head-ops .btn { text-decoration: none; }
/* V16 U位占用条 */
.u-bar { display: inline-block; width: 60px; height: 6px; background: var(--bg2); border-radius: 3px; overflow: hidden; vertical-align: middle; margin-right: 6px; }
.u-bar-in { height: 100%; border-radius: 3px; }
.u-bar-in.u-ok { background: #67c23a; } .u-bar-in.u-warm { background: #e6a23c; } .u-bar-in.u-hot { background: #f56c6c; }
.u-txt { font-size: 11.5px; color: var(--muted); }
.c-hot { color: #f56c6c; font-weight: 700; } .c-warm { color: #e6a23c; }
.alert-badge { background: rgba(245,108,108,.12); color: #f56c6c; border-radius: 10px; padding: 2px 9px; font-size: 12px; font-weight: 700; cursor: pointer; }
.alert-badge:hover { background: rgba(245,108,108,.22); }
/* V16 抽屉(统一规范) */
.drawer-mask { position: fixed; inset: 0; background: rgba(0,0,0,.35); z-index: 1000; display: flex; justify-content: flex-end; }
.drawer { width: 600px; max-width: 95vw; height: 100%; background: var(--bg); box-shadow: -4px 0 20px rgba(0,0,0,.15); display: flex; flex-direction: column; overflow: hidden; }
.drawer-head { display: flex; align-items: center; justify-content: space-between; padding: 16px 20px; border-bottom: 1px solid var(--border); flex-shrink: 0; }
.drawer-head h3 { margin: 0; font-size: 18px; }
.drawer-body { flex: 1; overflow-y: auto; padding: 20px; }
.drawer-foot { padding: 14px 20px; border-top: 1px solid var(--border); display: flex; justify-content: flex-end; gap: 10px; flex-shrink: 0; }
.fs { border: 1px solid var(--border); border-radius: 8px; padding: 16px; margin-bottom: 16px; }
.fs legend { font-weight: 600; font-size: 14px; padding: 0 8px; color: var(--accent); }
.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.form-grid label { display: flex; flex-direction: column; gap: 4px; font-size: 13px; color: var(--muted); }
.form-grid label.full { grid-column: 1 / -1; }
.err { color: #f56c6c; font-size: 13px; margin-top: 8px; }
</style>
