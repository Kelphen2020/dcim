<template>
  <Teleport to="body">
    <div v-if="open" class="gs-mask" @click.self="close">
      <div class="gs-panel" role="dialog" aria-label="全局搜索">
        <div class="gs-input-row">
          <svg class="gs-lead" width="18" height="18" aria-hidden="true"><use href="#i-inspect" /></svg>
          <input ref="inputEl" v-model="q" class="gs-input" type="text"
            placeholder="搜索资产 / 测点 / 工单 / 告警 / 网络设备 / 备件，或输入页面名跳转"
            @keydown="onKey" />
          <span class="gs-kbd">Esc</span>
        </div>
        <div class="gs-body" ref="bodyEl">
          <div v-if="loading" class="gs-status">搜索中…</div>
          <div v-else-if="error" class="gs-status gs-err">{{ error }}</div>
          <template v-else>
            <template v-if="recentEntries.length">
              <div class="gs-sec">最近搜索</div>
              <div v-for="e in recentEntries" :key="'r' + e.label" class="gs-item"
                :class="{ on: entries.indexOf(e) === activeIdx }" :data-eidx="entries.indexOf(e)"
                @mouseenter="activeIdx = entries.indexOf(e)" @click="activate(e)">
                <svg class="gs-ico" width="16" height="16"><use href="#i-inspect" /></svg>
                <div class="gs-main"><div class="gs-title">{{ e.label }}</div></div>
                <span class="gs-go">重新搜索</span>
              </div>
            </template>
            <template v-if="pageEntries.length">
              <div class="gs-sec">{{ q.trim() ? '页面' : '页面导航' }}</div>
              <div v-for="e in pageEntries" :key="'p' + e.to" class="gs-item"
                :class="{ on: entries.indexOf(e) === activeIdx }" :data-eidx="entries.indexOf(e)"
                @mouseenter="activeIdx = entries.indexOf(e)" @click="activate(e)">
                <svg class="gs-ico" width="16" height="16"><use :href="'#' + e.ico" /></svg>
                <div class="gs-main">
                  <div class="gs-title">{{ e.label }}</div>
                  <div class="gs-sub">{{ e.group }} · {{ e.to }}</div>
                </div>
                <span class="gs-go">跳转</span>
              </div>
            </template>
            <template v-for="s in objSections" :key="'g' + s.type">
              <div class="gs-sec">{{ s.label }}<span class="gs-count">{{ s.total }} 条匹配</span></div>
              <div v-for="e in s.items" :key="'o' + s.type + e.id" class="gs-item"
                :class="{ on: entries.indexOf(e) === activeIdx }" :data-eidx="entries.indexOf(e)"
                @mouseenter="activeIdx = entries.indexOf(e)" @click="activate(e)">
                <svg class="gs-ico" width="16" height="16"><use :href="TYPE_ICO[s.type] || '#i-box'" /></svg>
                <div class="gs-main">
                  <div class="gs-title">{{ e.title }}</div>
                  <div class="gs-sub">{{ e.subtitle }}</div>
                </div>
                <span v-if="e.tag" class="gs-tag" :class="tagCls(e.tag)">{{ e.tag }}</span>
              </div>
            </template>
            <div v-if="q.trim() && !entries.length" class="gs-status">未找到与「{{ q.trim() }}」匹配的对象或页面</div>
          </template>
        </div>
        <div class="gs-foot">
          <span><b>↑ ↓</b> 选择</span><span><b>Enter</b> 打开</span><span><b>Esc</b> 关闭</span>
          <span v-if="tookMs !== null" class="gs-took">{{ tookMs }} ms</span>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
// V76 B5-2: 全局搜索命令面板（Ctrl/Cmd+K）
// 六类对象统一检索（后端 /search，SiteScope+菜单组权限过滤）+ 页面快跳 + 最近搜索
// 结果携带深链 route，直达目标页详情抽屉
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'

const props = defineProps({ pages: { type: Array, default: () => [] } })
const router = useRouter()

const open = ref(false)
const q = ref('')
const loading = ref(false)
const error = ref('')
const groups = ref([])
const tookMs = ref(null)
const activeIdx = ref(0)
const inputEl = ref(null)
const bodyEl = ref(null)

const TYPE_ICO = { asset: '#i-asset', point: '#i-gear', work_order: '#i-order', alert: '#i-bell', network_device: '#i-gateway', spare_part: '#i-box' }
const RECENT_KEY = 'dcim_gs_recent'
const recent = ref([])
try { recent.value = JSON.parse(localStorage.getItem(RECENT_KEY) || '[]').slice(0, 6) } catch { recent.value = [] }
function pushRecent(v) {
  if (!v) return
  recent.value = [v, ...recent.value.filter(x => x !== v)].slice(0, 6)
  try { localStorage.setItem(RECENT_KEY, JSON.stringify(recent.value)) } catch { /* ignore */ }
}

const flatPages = computed(() => (props.pages || []).flatMap(g => g.items.map(m => ({ ...m, group: g.label }))))
const pageHits = computed(() => {
  const v = q.value.trim().toLowerCase()
  if (!v) return []
  return flatPages.value.filter(p => p.label.toLowerCase().includes(v) || p.to.toLowerCase().includes(v)).slice(0, 6)
})
const entries = computed(() => {
  if (!q.value.trim()) {
    return [...recent.value.map(r => ({ kind: 'recent', label: r })), ...flatPages.value.map(p => ({ kind: 'page', ...p }))]
  }
  const objs = []
  for (const g of groups.value) {
    if (!g.total) continue
    for (const it of g.items || []) objs.push({ kind: 'obj', type: g.type, glabel: g.label, ...it })
  }
  return [...pageHits.value.map(p => ({ kind: 'page', ...p })), ...objs]
})
const recentEntries = computed(() => entries.value.filter(e => e.kind === 'recent'))
const pageEntries = computed(() => entries.value.filter(e => e.kind === 'page'))
const objSections = computed(() => {
  const out = []
  for (const e of entries.value) {
    if (e.kind !== 'obj') continue
    let s = out.find(x => x.type === e.type)
    if (!s) {
      const g = groups.value.find(x => x.type === e.type)
      s = { type: e.type, label: e.glabel, total: g ? g.total : 0, items: [] }
      out.push(s)
    }
    s.items.push(e)
  }
  return out
})

function tagCls(t) {
  if (['严重', '故障', '低库存', '已报废'].includes(t)) return 'danger'
  if (['警告', '离线', '维修中', '维护', '已挂起'].includes(t)) return 'warn'
  return ''
}

let searchTimer = null, seq = 0
watch(q, () => {
  activeIdx.value = 0
  if (searchTimer) clearTimeout(searchTimer)
  const v = q.value.trim()
  if (!v) { groups.value = []; loading.value = false; error.value = ''; tookMs.value = null; return }
  loading.value = true
  searchTimer = setTimeout(async () => {
    const my = ++seq
    try {
      const r = await api.get('/search', { q: v, limit: 5 })
      if (my !== seq) return
      if (r.code === 0) { groups.value = r.data?.groups || []; tookMs.value = r.data?.took_ms ?? null; error.value = '' }
      else { groups.value = []; error.value = r.msg || '搜索失败' }
    } catch {
      if (my === seq) { groups.value = []; error.value = '网络异常，请重试' }
    } finally {
      if (my === seq) loading.value = false
    }
  }, 250)
})
watch(() => entries.value.length, () => { activeIdx.value = 0 })

function openPal() {
  open.value = true
  nextTick(() => inputEl.value?.focus())
}
function close() {
  open.value = false
  q.value = ''
  groups.value = []
  error.value = ''
  tookMs.value = null
  activeIdx.value = 0
}
function activate(e) {
  if (!e) return
  if (e.kind === 'recent') { q.value = e.label; nextTick(() => inputEl.value?.focus()); return }
  if (e.kind === 'page') { router.push(e.to); close(); return }
  pushRecent(q.value.trim())
  router.push({ path: e.route.path, query: e.route.query })
  close()
}
function scrollActive() {
  nextTick(() => {
    bodyEl.value?.querySelector(`[data-eidx="${activeIdx.value}"]`)?.scrollIntoView({ block: 'nearest' })
  })
}
function onKey(e) {
  if (e.key === 'ArrowDown') { e.preventDefault(); if (entries.value.length) { activeIdx.value = (activeIdx.value + 1) % entries.value.length; scrollActive() } }
  else if (e.key === 'ArrowUp') { e.preventDefault(); if (entries.value.length) { activeIdx.value = (activeIdx.value - 1 + entries.value.length) % entries.value.length; scrollActive() } }
  else if (e.key === 'Enter') { e.preventDefault(); activate(entries.value[activeIdx.value]) }
  else if (e.key === 'Escape') { e.preventDefault(); close() }
}
function onGlobalKey(e) {
  if ((e.ctrlKey || e.metaKey) && (e.key === 'k' || e.key === 'K')) {
    e.preventDefault()
    if (open.value) close(); else openPal()
  } else if (e.key === 'Escape' && open.value) close()
}
onMounted(() => window.addEventListener('keydown', onGlobalKey))
onUnmounted(() => { window.removeEventListener('keydown', onGlobalKey); if (searchTimer) clearTimeout(searchTimer) })
defineExpose({ openPal, close })
</script>

<style scoped>
.gs-mask { position: fixed; inset: 0; z-index: 3000; background: rgba(10, 20, 40, .45); backdrop-filter: blur(3px); display: flex; justify-content: center; align-items: flex-start; padding-top: 12vh; }
.gs-panel { width: 640px; max-width: calc(100vw - 24px); max-height: 72vh; display: flex; flex-direction: column; background: var(--card, var(--bg-card, #fff)); border: 1px solid var(--border, #e5e7eb); border-radius: 14px; box-shadow: 0 18px 60px rgba(10, 25, 60, .28); overflow: hidden; }
.gs-input-row { display: flex; align-items: center; gap: 10px; padding: 14px 16px; border-bottom: 1px solid var(--border, #e5e7eb); }
.gs-lead { color: var(--muted, #6b7280); flex-shrink: 0; }
.gs-input { flex: 1; border: none; outline: none; background: transparent; font-size: 15px; color: var(--text, #1f2937); }
.gs-input::placeholder { color: var(--muted, #9ca3af); font-size: 13px; }
.gs-kbd { font-size: 11px; color: var(--muted, #6b7280); border: 1px solid var(--border, #e5e7eb); border-radius: 5px; padding: 2px 7px; flex-shrink: 0; }
.gs-body { overflow-y: auto; padding: 6px 8px 10px; min-height: 120px; }
.gs-sec { font-size: 11px; color: var(--muted, #6b7280); letter-spacing: .06em; padding: 10px 10px 4px; display: flex; align-items: center; gap: 8px; }
.gs-count { font-size: 10.5px; opacity: .8; }
.gs-item { display: flex; align-items: center; gap: 10px; padding: 8px 10px; border-radius: 9px; cursor: pointer; }
.gs-item.on { background: var(--accent-glow, rgba(37, 99, 235, .08)); }
.gs-ico { color: var(--primary, var(--accent, #2563eb)); flex-shrink: 0; }
.gs-main { flex: 1; min-width: 0; }
.gs-title { font-size: 13.5px; color: var(--text, #1f2937); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.gs-sub { font-size: 11.5px; color: var(--muted, #6b7280); margin-top: 2px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.gs-tag { font-size: 11px; padding: 2px 8px; border-radius: 10px; border: 1px solid var(--border, #e5e7eb); color: var(--muted, #6b7280); flex-shrink: 0; }
.gs-tag.danger { color: #dc2626; border-color: rgba(220, 38, 38, .4); background: rgba(220, 38, 38, .06); }
.gs-tag.warn { color: #d97706; border-color: rgba(217, 119, 6, .4); background: rgba(217, 119, 6, .06); }
.gs-go { font-size: 11px; color: var(--muted, #9ca3af); flex-shrink: 0; }
.gs-status { padding: 28px 12px; text-align: center; font-size: 13px; color: var(--muted, #6b7280); }
.gs-status.gs-err { color: #dc2626; }
.gs-foot { display: flex; gap: 16px; align-items: center; padding: 9px 16px; border-top: 1px solid var(--border, #e5e7eb); font-size: 11.5px; color: var(--muted, #6b7280); }
.gs-foot b { font-weight: 600; border: 1px solid var(--border, #e5e7eb); border-radius: 4px; padding: 1px 5px; margin-right: 3px; }
.gs-took { margin-left: auto; font-variant-numeric: tabular-nums; }
@media (max-width: 900px) {
  .gs-mask { padding-top: 8vh; }
  .gs-panel { max-height: 80vh; }
  .gs-foot span:nth-child(1), .gs-foot span:nth-child(2) { display: none; }
}
</style>
