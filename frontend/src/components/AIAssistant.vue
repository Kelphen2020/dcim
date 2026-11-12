<template>
  <div class="ai-wrap">
    <button class="ai-btn" @click="open = !open" :title="aiTitle" data-ai-trigger>
      <SvgIcon name="spark" :size="17" />
      <span class="ai-btn-label">AI 助手</span>
      <span class="ai-mode-dot" :class="status.enabled ? 'on' : 'off'" :title="status.enabled ? '大模型已接入' : '本地规则模式'"></span>
    </button>

    <Teleport to="body">
    <div v-if="open" class="drawer-mask" @click.self="open = false">
      <div class="drawer ai-drawer" data-ai-drawer>
        <div class="drawer-head">
          <h3><SvgIcon name="robot" :size="18" /> AI 运维助手</h3>
          <div class="head-right">
            <span class="badge" :class="status.enabled ? 'hg-healthy' : 'hg-attention'" :title="status.enabled ? '回答由大模型基于平台实时数据生成' : '大模型未配置或不可用，回答由平台本地规则基于实时数据生成'">
              {{ status.enabled ? '大模型 ' + (status.model || '') : '本地模式' }}
            </span>
            <button class="btn btn-mini" @click="open = false" aria-label="关闭"><SvgIcon name="x" :size="14" /></button>
          </div>
        </div>

        <div class="drawer-body ai-body" ref="bodyEl">
          <div v-if="!msgs.length" class="ai-welcome">
            <div class="ai-welcome-title"><SvgIcon name="spark" :size="22" /> 你好，我是 DCIM 运维助手</div>
            <p class="muted">我可以基于平台实时数据回答运行状况、告警、设备健康、能耗等问题，并给出对应功能页的穿透链接。试试下面的问题：</p>
            <div class="ai-chips">
              <button v-for="q in quickQuestions" :key="q" class="ai-chip" @click="ask(q)">{{ q }}</button>
            </div>
          </div>

          <div v-for="(m, i) in msgs" :key="i" class="ai-msg" :class="m.role">
            <div class="ai-bubble" :data-role="m.role">
              <template v-if="m.role === 'assistant'">
                <div class="ai-md" v-html="m.html" @click="onMdClick"></div>
                <div v-if="m.degraded" class="ai-degraded" :title="m.reason">
                  <SvgIcon name="info" :size="12" /> 本地规则模式回答{{ m.reason ? '：' + m.reason : '' }}
                </div>
                <div v-if="m.links && m.links.length" class="ai-links">
                  <a v-for="l in m.links" :key="l.url" class="ai-link-chip" @click.prevent="go(l.url)" :href="'#' + l.url">
                    <SvgIcon name="link" :size="12" /> {{ l.label }}
                  </a>
                </div>
                <div v-if="m.report" class="ai-report-actions">
                  <button class="btn btn-mini" @click="download(m)"><SvgIcon name="download" :size="13" /> 下载 {{ m.filename }}</button>
                </div>
              </template>
              <template v-else>
                <span>{{ m.text }}</span>
              </template>
            </div>
          </div>

          <div v-if="busy" class="ai-msg assistant">
            <div class="ai-bubble ai-typing"><span></span><span></span><span></span></div>
          </div>
        </div>

        <div class="drawer-foot ai-foot">
          <div class="ai-quick">
            <button class="ai-chip sm" @click="genReport('week')"><SvgIcon name="doc" :size="12" /> 生成周报</button>
            <button class="ai-chip sm" @click="genReport('month')"><SvgIcon name="doc" :size="12" /> 生成月报</button>
          </div>
          <div class="ai-input-row">
            <input v-model="question" class="input" placeholder="输入运维问题，回车发送" maxlength="500"
              @keyup.enter="send" data-ai-input :disabled="busy" />
            <button class="btn btn-primary" @click="send" :disabled="busy || !question.trim()" data-ai-send>发送</button>
          </div>
        </div>
      </div>
    </div>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, computed, nextTick, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import api from '../api'

const router = useRouter()
const open = ref(false)
// V57.3: 路由切换自动收起抽屉（深链跳转后不遮挡目标页；会话 msgs 保留，重开可继续）
// 注意：location.hash 非响应式（watch 永不触发），必须用 useRoute()
const route = useRoute()
watch(() => route.fullPath, () => { open.value = false })
const busy = ref(false)
const question = ref('')
const msgs = ref([])
const bodyEl = ref(null)
const status = ref({ enabled: false, model: '' })

const quickQuestions = [
  '今天运行状况怎么样？',
  '有哪些严重告警？',
  '哪些设备有健康风险？',
  '能耗和 PUE 情况如何？',
  '最近工单和 SLA 情况？',
  '今天谁值班？'
]

const aiTitle = computed(() => status.value.enabled
  ? 'AI 运维助手（大模型基于平台实时数据回答）'
  : 'AI 运维助手（本地规则模式，基于平台实时数据回答）')

onMounted(async () => {
  const r = await api.get('/ai/status')
  if (r && r.code === 0 && r.data) status.value = r.data
})

function escapeHtml(s) {
  return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}

// 轻量 Markdown 渲染：转义后处理 粗体/行内代码/链接/标题/换行
function mdLite(text) {
  let s = escapeHtml(text)
  s = s.replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (m, label, url) => {
    if (/^https?:/i.test(url)) return `<a href="${url}" target="_blank" rel="noopener">${label}</a>`
    return `<a class="ai-link" data-href="${url}" href="#${url}">${label}</a>`
  })
  s = s.replace(/\*\*([^*]+)\*\*/g, '<b>$1</b>')
  s = s.replace(/`([^`]+)`/g, '<code>$1</code>')
  s = s.replace(/^### (.+)$/gm, '<b class="ai-h">$1</b>')
  s = s.replace(/^## (.+)$/gm, '<b class="ai-h">$1</b>')
  s = s.replace(/^# (.+)$/gm, '<b class="ai-h">$1</b>')
  s = s.replace(/\n/g, '<br>')
  return s
}

function onMdClick(e) {
  const a = e.target.closest('a.ai-link')
  if (!a) return
  e.preventDefault()
  const href = a.getAttribute('data-href')
  if (href) go(href)
}

function go(url) {
  open.value = false
  const [path, qs] = url.split('?')
  const query = {}
  if (qs) qs.split('&').forEach(kv => { const [k, v] = kv.split('='); if (k) query[k] = decodeURIComponent(v || '') })
  router.push({ path, query })
}

async function scrollBottom() {
  await nextTick()
  if (bodyEl.value) bodyEl.value.scrollTop = bodyEl.value.scrollHeight
}

async function send() {
  const q = question.value.trim()
  if (!q || busy.value) return
  question.value = ''
  await ask(q)
}

async function ask(q) {
  msgs.value.push({ role: 'user', text: q })
  busy.value = true
  scrollBottom()
  const r = await api.postLong('/ai/assistant', { question: q })
  busy.value = false
  if (r && r.code === 0 && r.data) {
    const d = r.data
    msgs.value.push({
      role: 'assistant', html: mdLite(d.answer || ''), links: d.links || [],
      degraded: !!d.degraded, reason: d.reason || ''
    })
  } else {
    msgs.value.push({ role: 'assistant', html: mdLite('请求失败：' + ((r && r.msg) || '未知错误'), ''), links: [], degraded: true })
  }
  scrollBottom()
}

async function genReport(period) {
  msgs.value.push({ role: 'user', text: period === 'month' ? '生成月度运行报告' : '生成周度运行报告' })
  busy.value = true
  scrollBottom()
  const r = await api.postLong('/ai/report', { period })
  busy.value = false
  if (r && r.code === 0 && r.data) {
    msgs.value.push({
      role: 'assistant', html: mdLite(r.data.markdown || ''), links: [{ label: '报表中心', url: '/reports' }],
      report: r.data.markdown, filename: r.data.filename || 'dcim-report.md', degraded: false
    })
  } else {
    msgs.value.push({ role: 'assistant', html: mdLite('报告生成失败：' + ((r && r.msg) || '未知错误'), ''), links: [], degraded: true })
  }
  scrollBottom()
}

function download(m) {
  const blob = new Blob(['\uFEFF' + m.report], { type: 'text/markdown;charset=utf-8' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = m.filename
  a.click()
  URL.revokeObjectURL(a.href)
}
</script>

<style scoped>
.ai-wrap { position: relative; display: flex; align-items: center; }
.ai-btn { display: flex; align-items: center; gap: 6px; padding: 6px 10px; border-radius: 8px; border: 1px solid var(--border); background: transparent; color: var(--text); cursor: pointer; font-size: 13px; transition: all .15s; }
.ai-btn:hover { border-color: var(--accent); color: var(--accent); }
.ai-mode-dot { width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0; }
.ai-mode-dot.on { background: #22c55e; box-shadow: 0 0 5px rgba(34,197,94,.7); }
.ai-mode-dot.off { background: var(--muted); }
.ai-drawer { width: 460px; }
.head-right { display: flex; align-items: center; gap: 8px; }
.drawer-head h3 { display: flex; align-items: center; gap: 8px; }
.ai-body { display: flex; flex-direction: column; gap: 12px; padding: 16px 20px; }
.ai-welcome { padding: 18px 4px; }
.ai-welcome-title { display: flex; align-items: center; gap: 8px; font-size: 16px; font-weight: 600; margin-bottom: 8px; color: var(--accent); }
.ai-chips { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 12px; }
.ai-chip { display: inline-flex; align-items: center; gap: 5px; padding: 6px 12px; border-radius: 16px; border: 1px solid var(--border); background: var(--bg2, transparent); color: var(--text); font-size: 13px; cursor: pointer; transition: all .15s; }
.ai-chip:hover { border-color: var(--accent); color: var(--accent); }
.ai-chip.sm { padding: 4px 10px; font-size: 12px; }
.ai-msg { display: flex; }
.ai-msg.user { justify-content: flex-end; }
.ai-msg.assistant { justify-content: flex-start; }
.ai-bubble { max-width: 88%; padding: 10px 13px; border-radius: 12px; font-size: 13.5px; line-height: 1.65; word-break: break-word; }
.ai-msg.user .ai-bubble { background: var(--accent); color: #fff; border-bottom-right-radius: 4px; }
.ai-msg.assistant .ai-bubble { background: var(--bg2, rgba(0,0,0,.04)); border: 1px solid var(--border); border-bottom-left-radius: 4px; }
.ai-md :deep(a) { color: var(--accent); cursor: pointer; text-decoration: underline; }
.ai-md :deep(code) { background: rgba(0,0,0,.07); padding: 1px 5px; border-radius: 4px; font-size: 12px; }
.ai-md :deep(.ai-h) { display: inline-block; margin: 2px 0; color: var(--accent); }
.ai-degraded { margin-top: 8px; padding-top: 7px; border-top: 1px dashed var(--border); font-size: 11.5px; color: var(--muted); display: flex; align-items: center; gap: 4px; }
.ai-links { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 9px; }
.ai-link-chip { display: inline-flex; align-items: center; gap: 4px; padding: 4px 10px; border-radius: 14px; border: 1px solid var(--accent); color: var(--accent); font-size: 12px; cursor: pointer; text-decoration: none; transition: all .15s; }
.ai-link-chip:hover { background: var(--accent); color: #fff; }
.ai-report-actions { margin-top: 9px; }
.ai-typing { display: flex; gap: 5px; padding: 13px 16px; }
.ai-typing span { width: 7px; height: 7px; border-radius: 50%; background: var(--muted); animation: aiBlink 1.2s infinite; }
.ai-typing span:nth-child(2) { animation-delay: .2s; }
.ai-typing span:nth-child(3) { animation-delay: .4s; }
@keyframes aiBlink { 0%, 80%, 100% { opacity: .25; } 40% { opacity: 1; } }
.ai-foot { display: flex; flex-direction: column; gap: 8px; align-items: stretch; }
.ai-quick { display: flex; gap: 8px; }
.ai-input-row { display: flex; gap: 8px; }
.ai-input-row .input { flex: 1; min-width: 0; }
@media (max-width: 900px) {
  .ai-btn-label { display: none; }
  .ai-drawer { width: 100% !important; }
}
</style>
