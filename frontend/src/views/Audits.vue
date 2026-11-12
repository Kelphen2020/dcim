<template>
  <div>
    <div class="page-head"><h1>审计日志</h1>
      <div class="toolbar" style="margin-top:8px">
        <input class="input" style="width:220px" v-model="kw" placeholder="搜索操作/对象/用户/详情" @keyup.enter="load(1)" />
        <select class="input" style="width:150px" v-model="action" @change="load(1)">
          <option value="">全部操作</option>
          <option v-for="(n,k) in ACTION_LABELS" :key="k" :value="k">{{ n }}</option>
        </select>
        <input class="input" type="date" style="width:150px" v-model="start" @change="load(1)" title="开始日期" />
        <span class="muted">至</span>
        <input class="input" type="date" style="width:150px" v-model="end" @change="load(1)" title="结束日期" />
        <button class="btn" @click="load(1)">查询</button>
        <button v-if="kw||action||start||end" class="btn" @click="kw='';action='';start='';end='';load(1)">清空</button>
        <span style="flex:1"></span>
        <span class="muted small">共 {{ total }} 条</span>
        <button class="btn" @click="exportCSV">导出 CSV</button>
      </div>
    </div>
    <div class="card">
      <table class="data-table">
        <thead><tr><th>时间</th><th>用户</th><th>操作</th><th>对象</th><th>详情</th><th>IP</th><th></th></tr></thead>
        <tbody>
          <tr v-for="a in list" :key="a.id">
            <td class="muted mono">{{ fmt(a.created_at) }}</td>
            <td>{{ a.username }}</td>
            <td><span class="tag" :class="'act-'+actGroup(a.action)">{{ ACTION_LABELS[a.action] || a.action }}</span></td>
            <td>{{ a.target || '-' }}</td>
            <td class="muted detail-cell">{{ a.detail || '-' }}</td>
            <td class="muted mono">{{ a.ip }}</td>
            <td class="ops"><button class="btn btn-mini" @click="view(a)">详情</button></td>
          </tr>
          <tr v-if="!list.length"><td colspan="7" class="empty-cell">暂无审计记录</td></tr>
        </tbody>
      </table>
      <Pagination v-if="total > pageSize" :total="total" :page="page" :page-size="pageSize" @change="load" />
    </div>

    <div v-if="sel" class="drawer-mask" @click.self="sel=null"><div class="drawer" style="width:480px">
      <div class="drawer-head"><h3>审计详情</h3><button class="btn btn-mini" @click="sel=null"><SvgIcon name="x" /></button></div>
      <div class="drawer-body">
        <table class="kv-table">
          <tr><td>时间</td><td class="mono">{{ fmt(sel.created_at) }}</td></tr>
          <tr><td>用户</td><td>{{ sel.username }} (ID {{ sel.user_id }})</td></tr>
          <tr><td>操作</td><td><span class="tag">{{ ACTION_LABELS[sel.action] || sel.action }}</span> <span class="muted mono">{{ sel.action }}</span></td></tr>
          <tr><td>对象</td><td>{{ sel.target || '-' }}</td></tr>
          <tr><td>IP</td><td class="mono">{{ sel.ip || '-' }}</td></tr>
          <tr><td>记录ID</td><td class="mono">#{{ sel.id }}</td></tr>
        </table>
        <div class="sec-label">详情</div>
        <pre class="detail-pre">{{ sel.detail || '(无)' }}</pre>
      </div>
      <div class="drawer-foot"><button class="btn" @click="sel=null">关闭</button></div>
    </div></div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'
import Pagination from '../components/Pagination.vue'

const ACTION_LABELS = {
  login: '登录', logout: '退出', change_password: '修改密码', reset_password: '重置密码',
  create_site: '创建站点', update_site: '更新站点', delete_site: '删除站点',
  create_room: '创建房间', update_room: '更新房间', delete_room: '删除房间',
  create_row: '创建机柜列', update_row: '更新机柜列', delete_row: '删除机柜列',
  create_cabinet: '创建机柜', update_cabinet: '更新机柜', delete_cabinet: '删除机柜',
  create_gateway: '注册网关', update_gateway: '更新网关', delete_gateway: '删除网关',
  create_user: '创建用户', update_user: '更新用户', delete_user: '删除用户',
  create_asset: '新增资产', update_asset: '更新资产', delete_asset: '删除资产',
  create_work_order: '创建工单', update_work_order: '更新工单',
  ack_alert: '确认告警', resolve_alert: '结束告警',
  update_setting: '更新配置'
}
const actGroup = a => /^(login|logout)$/.test(a) ? 'auth' : /^delete/.test(a) ? 'del' : /^(create|reset)/.test(a) ? 'add' : 'upd'

const list = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const kw = ref(''), action = ref(''), start = ref(''), end = ref('')
const sel = ref(null)

function fmt(ts) {
  const d = new Date(ts)
  return `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,'0')}-${String(d.getDate()).padStart(2,'0')} ${String(d.getHours()).padStart(2,'0')}:${String(d.getMinutes()).padStart(2,'0')}:${String(d.getSeconds()).padStart(2,'0')}`
}

async function load(p) {
  page.value = p || 1
  const params = { page: page.value, size: pageSize }
  if (kw.value) params.keyword = kw.value
  if (action.value) params.action = action.value
  if (start.value) params.start = start.value
  if (end.value) params.end = end.value
  const res = await api.get('/audits', params)
  if (res.code === 0) { list.value = res.data.items || []; total.value = res.data.total || 0 }
}

function view(a) { sel.value = a }

async function exportCSV() {
  // 导出当前过滤条件下全部记录（最多2000条）
  const params = { page: 1, size: 2000 }
  if (kw.value) params.keyword = kw.value
  if (action.value) params.action = action.value
  if (start.value) params.start = start.value
  if (end.value) params.end = end.value
  const res = await api.get('/audits', params)
  const items = res.data?.items || []
  const rows = [['时间','用户','操作','对象','详情','IP']]
  items.forEach(a => rows.push([fmt(a.created_at), a.username, ACTION_LABELS[a.action]||a.action, a.target||'', (a.detail||'').replace(/\n/g,' '), a.ip||'']))
  const csv = '\uFEFF' + rows.map(r => r.map(x => `"${String(x??'').replace(/"/g,'""')}"`).join(',')).join('\n')
  const url = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }))
  const el = document.createElement('a'); el.href = url; el.download = `dcim_audit_${new Date().toISOString().slice(0,10)}.csv`; el.click()
  URL.revokeObjectURL(url)
}

onMounted(() => load(1))
</script>
<style scoped>
.muted { color: var(--muted); font-size: 12px; }
.small { font-size: 12px; }
.empty-cell { text-align: center; color: var(--muted); padding: 24px !important; }
.detail-cell { max-width: 260px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tag { display: inline-block; padding: 2px 8px; border: 1px solid var(--border); border-radius: 4px; font-size: 12px; }
.tag.act-auth { color: #409eff; border-color: rgba(64,158,255,.4); }
.tag.act-add { color: #67c23a; border-color: rgba(103,194,58,.4); }
.tag.act-upd { color: #e6a23c; border-color: rgba(230,162,60,.4); }
.tag.act-del { color: #f56c6c; border-color: rgba(245,108,108,.4); }
.kv-table { width: 100%; border-collapse: collapse; font-size: 13px; }
.kv-table td { padding: 8px 10px; border-bottom: 1px solid var(--border); }
.kv-table td:first-child { color: var(--muted); width: 80px; }
.sec-label { font-size: 13px; font-weight: 600; margin: 16px 0 6px; color: var(--text2, var(--muted)); }
.detail-pre { background: rgba(0,0,0,.2); border: 1px solid var(--border); border-radius: 6px; padding: 12px; font-size: 12px; line-height: 1.6; white-space: pre-wrap; word-break: break-all; font-family: monospace; margin: 0; }
.drawer-mask{position:fixed;inset:0;background:rgba(0,0,0,.35);z-index:1000;display:flex;justify-content:flex-end}.drawer{max-width:95vw;height:100%;background:var(--bg);box-shadow:-4px 0 20px rgba(0,0,0,.15);display:flex;flex-direction:column;overflow:hidden}.drawer-head{display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--border);flex-shrink:0}.drawer-head h3{margin:0;font-size:18px}.drawer-body{flex:1;overflow-y:auto;padding:20px}.drawer-foot{padding:14px 20px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;gap:10px;flex-shrink:0}
</style>
