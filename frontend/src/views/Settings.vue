<template>
  <div>
    <div class="page-head"><h1>系统设置</h1><p class="muted">系统参数（管理员）。个人资料、密码与主题偏好已移至<a href="#/user-center" style="color:var(--accent)">用户中心</a>（点击顶栏头像进入）。</p></div>

    <!-- 系统参数：仅管理员可见 -->
    <div v-if="auth.isGlobalAdmin" class="card">
      <div class="card-title"><SvgIcon name="gear" />系统参数</div>
      <p class="muted" style="margin-bottom:14px">修改后点击「保存参数」生效，所有用户共享。审计日志会记录每次变更。</p>
      <div class="form-grid">
        <label>站点标题<input class="input" v-model="sys.site_title" placeholder="DCIM 数据中心基础设施管理" /></label>
        <label>单位名（关于信息）<input class="input" v-model="sys.company_name" placeholder="北京东青互联科技" /></label>
        <label>默认主题
          <select class="input" v-model="sys.default_theme">
            <option value="">（跟随本机）</option>
            <option v-for="t in THEME_PRESETS" :key="t.key" :value="t.key">{{ t.name }}</option>
          </select>
        </label>
        <label>版本号（关于信息）<input class="input" v-model="sys.system_version" placeholder="如 V7.0" /></label>
        <label class="full">登录页公告
          <textarea class="input" rows="2" v-model="sys.login_banner" placeholder="登录页展示的欢迎/提示文案" style="width:100%;box-sizing:border-box"></textarea>
        </label>
        <label>密码最小长度
          <select class="input" v-model="sys.password_min_length">
            <option v-for="n in [8,10,12,16]" :key="n" :value="String(n)">{{ n }} 位</option>
          </select>
        </label>
        <label>仪表盘自动刷新间隔（秒）<input class="input" type="number" v-model.number="sys.dashboard_refresh" min="10" /></label>
        <label>运维联系电话<input class="input" v-model="sys.maintenance_contact" placeholder="故障与告警通知联系人" /></label>
        <label>电价（元/kWh）<input class="input" v-model="sys.energy_price_kwh" placeholder="默认 0.8，用于能耗费用估算" /></label>
        <label>电网排放因子（kgCO₂/kWh）<input class="input" v-model="sys.co2_factor" placeholder="默认 0.5703，用于碳排放估算" /></label>
        <label>健康等级阈值<input class="input" v-model="sys.health_grade_thresholds" placeholder="默认 85,70,55（健康/关注/亚健康分界线）" /></label>
        <label>风险级自动开工单
          <select class="input" v-model="sys.health_auto_work_order">
            <option value="1">开启（健康分跌入风险级自动创建预测性维护工单）</option>
            <option value="0">关闭（仅发送通知）</option>
          </select>
        </label>
        <label>供电冗余自动开工单
          <select class="input" v-model="sys.power_redundancy_auto_work_order">
            <option value="1">开启（站点供电评为单点风险自动创建整改工单）</option>
            <option value="0">关闭（仅发送通知）</option>
          </select>
        </label>
        <label>容量饱和自动开工单
          <select class="input" v-model="sys.capacity_forecast_auto_work_order">
            <option value="1">开启（预测90天内饱和自动创建扩容规划工单）</option>
            <option value="0">关闭（仅发送通知）</option>
          </select>
        </label>
        <label>演练逾期通知
          <select class="input" v-model="sys.drill_overdue_notify">
            <option value="1">开启（应急预案超过演练周期未演练自动通知责任人）</option>
            <option value="0">关闭</option>
          </select>
        </label>
        <label>运维月报自动生成
          <select class="input" v-model="sys.auto_monthly_report">
            <option value="1">开启（每月1日自动生成上月全站月报并通知）</option>
            <option value="0">关闭（仅手动生成）</option>
          </select>
        </label>
        <label>巡检任务自动生成
          <select class="input" v-model="sys.inspection_task_auto">
            <option value="1">开启（按巡检计划周期自动生成应执行任务，未执行到期判漏检）</option>
            <option value="0">关闭（仅在任务治理页手动生成）</option>
          </select>
        </label>
        <label>巡检漏检通知
          <select class="input" v-model="sys.inspection_missed_notify">
            <option value="1">开启（周期结束未执行判定漏检并通知责任人+Webhook外发）</option>
            <option value="0">关闭（仅记录漏检状态，不发通知）</option>
          </select>
        </label>
        <label>告警噪声预警
          <select class="input" v-model="sys.alert_quality_notify">
            <option value="1">开启（近7天触发≥5次且疑似误报率≥60%的规则，通知管理员+Webhook外发，7天幂等）</option>
            <option value="0">关闭（仅质量治理页查看，不发通知）</option>
          </select>
        </label>
        <label>低库存自动采购申请
          <select class="input" v-model="sys.spare_auto_purchase">
            <option value="1">开启（每日扫描低库存备件，自动生成待审批采购申请并通知，同备件待审批唯一）</option>
            <option value="0">关闭（仅低库存预警，不自动生成采购申请）</option>
          </select>
        </label>
        <label>PUE 合规主动告警
          <select class="input" v-model="sys.pue_over_notify">
            <option value="1">开启（连续越线达设定时长后生成合规告警并通知+Webhook，同站点同状态 24 小时去重）</option>
            <option value="0">关闭（仍记录合规告警，仅不发通知）</option>
          </select>
        </label>
        <label>PUE 越线持续时间（小时）
          <input class="input" type="number" min="1" max="24" v-model="sys.pue_over_sustain_hours" title="最近 N 小时内所有 PUE 采样均越线（至少 2 个采样点）才触发，默认 2 小时" />
        </label>
        <label>网络设备心跳超时（分钟）
          <input class="input" type="number" min="1" max="1440" v-model="sys.netdev_offline_min" title="网络设备最后心跳距今超过该分钟数即判定离线并告警；心跳来源=设备心跳上报/配置备份成功" />
        </label>
        <label class="full">会话/安全说明
          <textarea class="input" rows="2" v-model="sys.session_note" placeholder="如：30分钟无操作自动登出，密码须含大小写字母、数字与特殊字符" style="width:100%;box-sizing:border-box"></textarea>
        </label>
      </div>
      <p v-if="sysMsg" :class="sysOk?'ok':'err'" style="margin-top:10px">{{ sysMsg }}</p>
      <div style="margin-top:14px"><button class="btn btn-primary" @click="saveSys" :disabled="sysSaving">{{ sysSaving?'保存中…':'保存参数' }}</button></div>
    </div>

    <div class="card" style="margin-top:20px">
      <div class="card-title"><SvgIcon name="gauge" />设备健康评估</div>
      <p class="muted" style="margin-bottom:12px">评估引擎每 15 分钟自动运行：从告警史、阈值裕度、劣化趋势、测点在线率、服役年限、维保覆盖等 8 个维度为每台在架设备打分（100 分制扣分模型，每项扣分附证据）。等级阈值与自动开工单开关在上方「系统参数」中配置。</p>
      <div v-if="healthCfg.factors" class="hg-factor-list">
        <div v-for="fc in healthCfg.factors" :key="fc.key" class="hg-factor-item">
          <b>{{ fc.label }}</b><span class="muted">{{ fc.desc }}</span>
        </div>
      </div>
      <div style="margin-top:14px;display:flex;align-items:center;gap:12px;flex-wrap:wrap">
        <button class="btn btn-primary" @click="evalNow" :disabled="evalBusy">{{ evalBusy ? '评估中…' : '立即评估' }}</button>
        <span v-if="evalStats" class="muted small">上次手动评估：{{ evalStats.evaluated }} 台 · 风险 {{ evalStats.risk }} · 亚健康 {{ evalStats.sub_healthy }} · 新通知 {{ evalStats.notified }} · 自动工单 {{ evalStats.work_orders }}</span>
      </div>
    </div>

    <!-- V58 AI 大模型配置 -->
    <div v-if="auth.isGlobalAdmin" class="card" style="margin-top:20px">
      <div class="card-title" style="display:flex;align-items:center;gap:8px;flex-wrap:wrap">
        <SvgIcon name="robot" /><span>AI 大模型</span>
        <span class="badge" :class="aiCfg.enabled ? 'hg-healthy' : 'hg-attention'">{{ aiCfg.enabled ? '已接入' : '未接入 · 本地规则模式' }}</span>
        <span v-if="aiCfg.source === 'db'" class="badge ai-src">界面配置生效中</span>
        <span v-else-if="aiCfg.source === 'env'" class="badge ai-src">服务器环境配置生效中</span>
        <span style="flex:1"></span>
        <button v-if="aiCfg.source === 'db'" class="btn btn-mini" @click="resetAI">恢复环境配置</button>
      </div>
      <p class="muted" style="margin-bottom:14px">为 AI 运维助手配置大模型（OpenAI 兼容接口）。下拉选择供应商后自动填入接口地址与常用模型，也可切换到自定义模型名/端点。<b>保存后立即热生效，无需重启服务</b>；密钥仅存服务器，界面全程掩码不回显。注意：Token Plan / Coding Plan 与按量付费的 Key 和 Base URL 完全隔离，必须配套使用。</p>

      <div class="form-grid">
        <label>供应商
          <select class="input" v-model="aiForm.provider" @change="onProviderChange">
            <optgroup v-for="g in providerGroups" :key="g.name" :label="g.name">
              <option v-for="p in g.items" :key="p.key" :value="p.key">{{ p.name }}</option>
            </optgroup>
          </select>
        </label>
        <label>模型名称
          <select class="input mono" v-model="modelSel">
            <option value="" disabled>请选择模型</option>
            <option v-for="m in curModels" :key="m" :value="m">{{ m }}</option>
            <option value="__custom__">自定义模型名…</option>
          </select>
          <input v-if="modelIsCustom" class="input mono" style="margin-top:6px" v-model="aiForm.model" placeholder="输入模型 ID，如 qwen3.8-omni-flash" />
        </label>
        <p v-if="curProvider" class="full muted small ai-prov-hint">
          {{ curProvider.desc }}<template v-if="curProvider.keyHint"> · 密钥格式：{{ curProvider.keyHint }}</template>
        </p>
        <label class="full">接口地址（base_url，OpenAI 兼容）
          <input class="input mono" v-model="aiForm.base_url" placeholder="https://…/v1" />
        </label>
        <label class="full">API Key
          <input class="input mono" type="password" v-model="aiForm.api_key"
                 :placeholder="aiCfg.api_key_set ? ('已配置 ' + aiCfg.api_key_mask + ' · 留空保持不变') : (curProvider && curProvider.keyHint ? curProvider.keyHint : 'sk-…（本地 Ollama 可留空）')" />
        </label>
      </div>

      <div v-if="aiTest.done" class="ai-test" :class="aiTest.ok ? 'ok' : 'bad'">
        <SvgIcon :name="aiTest.ok ? 'check-circle' : 'x-circle'" />
        <span v-if="aiTest.ok">连接成功 · {{ aiTest.latency_ms }}ms · 应答模型 <b class="mono">{{ aiTest.model_echo || aiForm.model }}</b><template v-if="aiTest.reply"> · 回复「{{ aiTest.reply }}」</template></span>
        <span v-else>连接失败：{{ aiTest.error }}</span>
      </div>

      <div style="margin-top:14px;display:flex;gap:10px;align-items:center;flex-wrap:wrap">
        <button class="btn" @click="testAI" :disabled="aiTesting"><SvgIcon name="plug" />{{ aiTesting ? '测试中…' : '测试连接' }}</button>
        <button class="btn btn-primary" @click="saveAI" :disabled="aiSaving"><SvgIcon name="save" />{{ aiSaving ? '保存中…' : '保存配置' }}</button>
        <span class="muted small">建议先「测试连接」通过后再保存</span>
      </div>
    </div>

    <div class="card" style="margin-top:20px">
      <div class="card-title"><SvgIcon name="palette" />界面主题（站点默认）</div>
      <p class="muted" style="margin-bottom:14px">此处设置的是<b>站点默认主题</b>，对新用户/未选过主题的浏览器生效；个人主题请到「用户中心」设置，只影响自己。</p>
    </div>

    <div class="card" style="margin-top:20px">
      <div class="card-title"><SvgIcon name="info" />关于</div>
      <p class="muted" style="margin:0">DCIM 数据中心基础设施管理系统 · {{ sys.system_version || 'V7.0' }} · {{ sys.company_name || '北京东青互联科技' }}</p>
      <p class="muted" style="margin:6px 0 0;font-size:12px">涵盖基础设施层级、资产台账、维保、工单、巡检、变更、动环监控、能耗、3D 可视化、网络管理、报表与开放 API。</p>
    </div>
  </div>
</template>

<script setup>
import { reactive, ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useSettingsStore, THEME_PRESETS } from '../stores/settings'
import { useAuthStore } from '../stores/auth'
import api from '../api'
import { toastAuto, toast, showConfirm } from '../ui-dialog'

const settings = useSettingsStore()
const auth = useAuthStore()
const router = useRouter()
const ROLE_LABELS = { global_admin: '全局管理员', site_admin: '站点管理员', operator: '值班员', viewer: '只读' }

// ==================== 系统参数 ====================
const SYS_KEYS = ['site_title','company_name','default_theme','system_version','login_banner','password_min_length','dashboard_refresh','maintenance_contact','session_note','energy_price_kwh','co2_factor','health_grade_thresholds','health_auto_work_order','power_redundancy_auto_work_order','capacity_forecast_auto_work_order','drill_overdue_notify','auto_monthly_report','inspection_task_auto','inspection_missed_notify','alert_quality_notify','netdev_offline_min','spare_auto_purchase','pue_over_notify','pue_over_sustain_hours']
const SYS_DEFAULTS = { password_min_length: '8', inspection_task_auto: '1', inspection_missed_notify: '1', alert_quality_notify: '1', netdev_offline_min: '10', spare_auto_purchase: '1', pue_over_notify: '1', pue_over_sustain_hours: '2' }
const sys = reactive(Object.fromEntries(SYS_KEYS.map(k => [k, SYS_DEFAULTS[k] || ''])))
const sysMsg = ref(''), sysOk = ref(false), sysSaving = ref(false)
async function loadSys() {
  if (!auth.isGlobalAdmin) return
  try {
    const r = await api.get('/system-settings')
    const d = r.data || {}
    SYS_KEYS.forEach(k => { if (d[k] !== undefined && d[k] !== null) sys[k] = k === 'dashboard_refresh' ? Number(d[k]) || 30 : String(d[k]) })
    if (!sys.dashboard_refresh) sys.dashboard_refresh = 30
  } catch { /* ignore */ }
}
async function saveSys() {
  sysSaving.value = true; sysMsg.value = ''; sysOk.value = false
  let okCount = 0, failMsg = ''
  for (const k of SYS_KEYS) {
    const value = k === 'dashboard_refresh' ? String(sys.dashboard_refresh || 30) : String(sys[k] ?? '')
    const r = await api.put('/system-setting', { key: k, value })
    if (r.code === 0) okCount++
    else { failMsg = r.msg || k; break }
  }
  sysSaving.value = false
  if (okCount === SYS_KEYS.length) { sysOk.value = true; sysMsg.value = '已保存全部参数' }
  else { sysOk.value = false; sysMsg.value = '保存失败：' + failMsg }
  settings.load()
}

// ==================== V58 AI 大模型配置（V72 下拉列表重设计） ====================
// 供应商与模型清单 2026-11 联网核实：Token Plan 专属端点(sk-sp- Key)、百炼 qwen3.8 系列、
// DeepSeek V4.1、GLM-5.3、Kimi K3.1、豆包 Seed 2.1、MiniMax M3、GPT-5.4 系列。
const DEFAULT_PROVIDER = 'bailian'
const DEFAULT_MODEL = 'qwen3.8-omni-flash'
const AI_PROVIDERS = [
  { key: 'tokenplan',  group: '订阅套餐',   name: '阿里云百炼 · Token Plan', desc: '套餐订阅制，Credits 统一计量，千问+三方模型（实测清单，无 omni 系列）', base: 'https://token-plan.cn-beijing.maas.aliyuncs.com/compatible-mode/v1', models: ['qwen3.8-max', 'qwen3.8-flash', 'qwen3.7-plus', 'qwen3.7-max', 'qwen3.6-flash', 'glm-5.3', 'deepseek-v4.1-flash', 'auto'], keyHint: 'sk-sp- 开头（Token Plan 控制台生成，与百炼通用 Key 不通用）' },
  { key: 'bailian',    group: '国内 · 按量', name: '阿里云百炼（按量付费）', desc: '通义千问全系列，按 token 计费', base: 'https://dashscope.aliyuncs.com/compatible-mode/v1', models: ['qwen3.8-omni-flash', 'qwen3.8-max', 'qwen3.8-flash', 'qwen3.7-plus', 'qwen-plus', 'qwen-turbo'], keyHint: 'sk- 开头（百炼控制台 API Key）' },
  { key: 'deepseek',   group: '国内 · 按量', name: 'DeepSeek 深度求索',      desc: '1M 上下文，思考/非思考双模式', base: 'https://api.deepseek.com', models: ['deepseek-flash', 'deepseek-v4-pro'] },
  { key: 'zhipu',      group: '国内 · 按量', name: '智谱 GLM',               desc: 'GLM 旗舰系列，长程 Agent 任务', base: 'https://open.bigmodel.cn/api/paas/v4', models: ['glm-5.3', 'glm-5.2', 'glm-4.7-flash'] },
  { key: 'moonshot',   group: '国内 · 按量', name: '月之暗面 Kimi',          desc: 'Kimi K3.1 系列，推理强度可调', base: 'https://api.moonshot.cn/v1', models: ['kimi-k3.1-pro', 'kimi-k3.1-pro-highspeed', 'kimi-k2.6'] },
  { key: 'minimax',    group: '国内 · 按量', name: 'MiniMax',                desc: '百万上下文 Agent 推理', base: 'https://api.minimaxi.com/v1', models: ['MiniMax-M3', 'MiniMax-M2.5'] },
  { key: 'volcengine', group: '国内 · 按量', name: '火山引擎 · 豆包',        desc: '豆包 Seed 系列（火山方舟）', base: 'https://ark.cn-beijing.volces.com/api/v3', models: ['doubao-seed-2-1-pro-260915', 'doubao-seed-2-1-lite-260915', 'doubao-seed-evolving'] },
  { key: 'siliconflow', group: '国内 · 按量', name: '硅基流动 SiliconFlow',   desc: '一个 Key 聚合 200+ 开源/商业模型', base: 'https://api.siliconflow.cn/v1', models: ['deepseek-ai/DeepSeek-V4.1-Flash', 'Qwen/Qwen3-8B', 'zai-org/GLM-4.6'] },
  { key: 'openai',     group: '海外',       name: 'OpenAI',                 desc: 'GPT 系列', base: 'https://api.openai.com/v1', models: ['gpt-5.4-mini', 'gpt-5.4-nano', 'gpt-5.1'] },
  { key: 'openrouter', group: '海外',       name: 'OpenRouter',             desc: '海外模型聚合路由，自动选路', base: 'https://openrouter.ai/api/v1', models: ['openrouter/auto'] },
  { key: 'ollama',     group: '本地/自定义', name: 'Ollama 本地',            desc: '本地部署模型，无需密钥', base: 'http://localhost:11434/v1', models: ['qwen3:8b', 'llama3.1:8b'] },
  { key: 'custom',     group: '本地/自定义', name: '自定义',                 desc: '任意 OpenAI 兼容端点，手动填写地址与模型', base: '', models: [] },
]
const aiCfg = ref({ enabled: false, source: 'none', api_key_set: false, api_key_mask: '' })
const aiForm = reactive({ provider: DEFAULT_PROVIDER, base_url: '', model: '', api_key: '' })
const aiTest = reactive({ done: false, ok: false, latency_ms: 0, reply: '', model_echo: '', error: '' })
const aiTesting = ref(false), aiSaving = ref(false)
const modelCustom = ref(false)
const curProvider = computed(() => AI_PROVIDERS.find(p => p.key === aiForm.provider) || null)
const curModels = computed(() => (curProvider.value || {}).models || [])
const providerGroups = computed(() => {
  const gs = []
  for (const p of AI_PROVIDERS) {
    const g = gs.find(x => x.name === p.group)
    if (g) g.items.push(p); else gs.push({ name: p.group, items: [p] })
  }
  return gs
})
// 模型下拉：已保存模型不在当前供应商清单中时自动落入「自定义」态，值不丢
const modelIsCustom = computed(() => modelCustom.value || (!!aiForm.model && !curModels.value.includes(aiForm.model)))
const modelSel = computed({
  get() {
    if (modelIsCustom.value) return '__custom__'
    return aiForm.model || ''
  },
  set(v) {
    if (v === '__custom__') { modelCustom.value = true; aiTest.done = false; return }
    modelCustom.value = false
    aiForm.model = v
    aiTest.done = false
  }
})
function onProviderChange() {
  const p = curProvider.value
  modelCustom.value = false
  aiTest.done = false
  if (!p) return
  if (p.key === 'custom') { aiForm.model = ''; return }
  if (p.base) aiForm.base_url = p.base
  aiForm.model = p.models.length ? p.models[0] : ''
}
async function loadAI() {
  if (!auth.isGlobalAdmin) return
  const r = await api.get('/settings/ai')
  if (r.code !== 0) return
  aiCfg.value = r.data || {}
  const d = r.data || {}
  if (!d.base_url && !d.model) {
    // 无任何已存配置：给默认（Token Plan + qwen3.8-omni-flash）
    aiForm.provider = DEFAULT_PROVIDER
    aiForm.base_url = (AI_PROVIDERS.find(p => p.key === DEFAULT_PROVIDER) || {}).base || ''
    aiForm.model = DEFAULT_MODEL
    aiForm.api_key = ''
    return
  }
  const known = AI_PROVIDERS.find(p => p.key === d.provider)
  if (known) aiForm.provider = known.key
  else {
    const byBase = AI_PROVIDERS.find(p => p.base && d.base_url && d.base_url.startsWith(p.base))
    aiForm.provider = byBase ? byBase.key : 'custom'
  }
  aiForm.base_url = d.base_url || (AI_PROVIDERS.find(p => p.key === aiForm.provider) || {}).base || ''
  aiForm.model = d.model || ''
  aiForm.api_key = ''
  modelCustom.value = false
}
async function testAI() {
  aiTesting.value = true; aiTest.done = false
  try {
    const r = await api.post('/settings/ai/test', { base_url: aiForm.base_url, model: aiForm.model, api_key: aiForm.api_key })
    const d = r.data || {}
    Object.assign(aiTest, { done: true, ok: !!d.ok, latency_ms: d.latency_ms || 0, reply: d.reply || '', model_echo: d.model_echo || '', error: d.error || (r.msg || '未知错误') })
  } catch (e) { Object.assign(aiTest, { done: true, ok: false, error: String(e), latency_ms: 0, reply: '', model_echo: '' }) }
  aiTesting.value = false
}
async function saveAI() {
  if (!aiForm.base_url || !aiForm.model) { toast('接口地址与模型名称必填', 'error'); return }
  aiSaving.value = true
  const r = await api.put('/settings/ai', { provider: aiForm.provider, base_url: aiForm.base_url, model: aiForm.model, api_key: aiForm.api_key })
  aiSaving.value = false
  if (r.code === 0) {
    toast('大模型配置已保存并热生效，AI 助手即刻可用新模型')
    aiForm.api_key = ''
    await loadAI()
  } else toast(r.msg || '保存失败', 'error')
}
async function resetAI() {
  if (!(await showConfirm({ title: '恢复环境配置', message: '清除界面保存的大模型配置，回退到服务器环境变量（ai.env）配置？', tone: 'warn', confirmText: '恢复' }))) return
  const r = await api.del('/settings/ai')
  if (r.code === 0) { toast('已恢复为服务器环境配置'); await loadAI() } else toast(r.msg || '操作失败', 'error')
}

// ==================== V50 设备健康评估 ====================
const healthCfg = ref({})
const evalBusy = ref(false), evalStats = ref(null)
async function loadHealthCfg() {
  if (!auth.isGlobalAdmin) return
  try {
    const r = await api.get('/asset-health/config')
    if (r.code === 0) healthCfg.value = r.data || {}
  } catch { /* ignore */ }
}
async function evalNow() {
  evalBusy.value = true; evalStats.value = null
  try {
    const r = await api.post('/asset-health/evaluate')
    if (r.code === 0) evalStats.value = r.data
    else toastAuto(r.msg || '评估失败')
  } finally { evalBusy.value = false }
}

// ==================== 个人资料 ====================
const profile = reactive({ display_name: '', phone: '' })
const pfMsg = ref(''), pfOk = ref(false)
function loadProfile() {
  profile.display_name = auth.user?.display_name || ''
  profile.phone = auth.user?.phone || ''
}
async function saveProfile() {
  pfMsg.value = ''; pfOk.value = false
  const r = await api.put('/me/profile', { display_name: profile.display_name, phone: profile.phone })
  if (r.code === 0) {
    pfOk.value = true; pfMsg.value = '资料已保存'
    const u = r.data?.user
    if (u) {
      auth.user = { ...auth.user, display_name: u.display_name, phone: u.phone }
      localStorage.setItem('dcim_user', JSON.stringify(auth.user))
    }
  } else { pfOk.value = false; pfMsg.value = r.msg || '保存失败' }
}

// ==================== 修改密码 ====================
const pwd = reactive({ old: '', new: '', confirm: '', err: '', ok: false })
async function changePwd() {
  pwd.err = ''; pwd.ok = false
  if (!pwd.old || !pwd.new) { pwd.err = '请填写原密码与新密码'; return }
  if (pwd.new.length < 8) { pwd.err = '新密码至少8位'; return }
  if (!/[A-Z]/.test(pwd.new) || !/[a-z]/.test(pwd.new) || !/[0-9]/.test(pwd.new) || !/[^A-Za-z0-9]/.test(pwd.new)) { pwd.err = '密码须包含大写字母、小写字母、数字与特殊字符'; return }
  if (pwd.new !== pwd.confirm) { pwd.err = '两次输入的新密码不一致'; return }
  try {
    const res = await api.post('/me/password', { old_password: pwd.old, new_password: pwd.new })
    if (res.code !== 0) { pwd.err = res.msg; return }
    pwd.ok = true
    setTimeout(() => { auth.logout(); router.push('/login') }, 1200)
  } catch (e) { pwd.err = e.response?.data?.msg || '修改失败' }
}

onMounted(() => { loadSys(); loadProfile(); loadHealthCfg(); loadAI() })
</script>

<style scoped>
.muted { color: var(--muted); }
.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.form-grid label { display: flex; flex-direction: column; gap: 4px; font-size: 13px; color: var(--muted); }
.form-grid label.full { grid-column: 1/-1; }
.ok { color: #00915b; font-size: 13px; }
.hg-factor-list { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
.hg-factor-item { border: 1px solid var(--border); border-radius: 8px; padding: 8px 12px; font-size: 12.5px; display: flex; flex-direction: column; gap: 3px; }
.hg-factor-item b { font-size: 13px; }
@media (max-width: 900px) { .hg-factor-list { grid-template-columns: 1fr; } }
.err { color: #f56c6c; font-size: 13px; }
.theme-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(190px, 1fr)); gap: 14px; }
.theme-card { border: 1px solid var(--border); border-radius: 10px; padding: 12px; cursor: pointer; transition: all .2s; }
.theme-card:hover { border-color: var(--accent); transform: translateY(-2px); }
.theme-card.active { border-color: var(--accent); box-shadow: 0 0 0 2px var(--accent-glow); }
.theme-preview { position: relative; height: 72px; border-radius: 6px; overflow: hidden; margin-bottom: 10px; background: linear-gradient(135deg, #eef4ff, #dbe7ff); }
.theme-preview.dark { background: linear-gradient(135deg, #101828, #1a2438); }
.theme-preview[data-theme-preview="emerald-green"] { background: linear-gradient(135deg, #ecfdf5, #d1fae5); }
.theme-preview[data-theme-preview="royal-purple"] { background: linear-gradient(135deg, #f3efff, #e4d9ff); }
.theme-preview[data-theme-preview="midnight-dark"] { background: linear-gradient(135deg, #0d1424, #16203a); }
.theme-preview[data-theme-preview="obsidian-black"] { background: linear-gradient(135deg, #0a0a0c, #1c1a14); }
.theme-preview[data-theme-preview="nebula-purple"] { background: linear-gradient(135deg, #1a1030, #2d1b4e); }
.theme-preview[data-theme-preview="klein-blue"] { background: linear-gradient(135deg, #f0f3fd, #dce4fa); }
.theme-preview[data-theme-preview="klein-blue-dark"] { background: linear-gradient(135deg, #070D24, #15224E); }
.tp-bar { position: absolute; top: 0; left: 0; right: 0; height: 14px; background: rgba(59,130,246,.75); }
.theme-preview.dark .tp-bar { background: rgba(96,165,250,.5); }
.theme-preview[data-theme-preview="klein-blue"] .tp-bar { background: rgba(2,42,153,.8); }
.theme-preview[data-theme-preview="klein-blue-dark"] .tp-bar { background: rgba(76,125,255,.7); }
.tp-side { position: absolute; top: 14px; left: 0; bottom: 0; width: 26%; background: rgba(59,130,246,.28); }
.theme-preview[data-theme-preview="klein-blue"] .tp-side { background: rgba(2,42,153,.25); }
.theme-preview[data-theme-preview="klein-blue-dark"] .tp-side { background: rgba(76,125,255,.25); }
.tp-main { position: absolute; top: 22px; left: 32%; width: 56%; height: 16px; border-radius: 3px; background: rgba(59,130,246,.5); }
.theme-preview[data-theme-preview="klein-blue"] .tp-main { background: rgba(2,42,153,.45); }
.theme-preview[data-theme-preview="klein-blue-dark"] .tp-main { background: rgba(76,125,255,.4); }
.theme-name { font-size: 13px; font-weight: 600; }
.theme-desc { font-size: 11px; color: var(--muted); margin-top: 2px; }
.ai-prov-hint { grid-column: 1/-1; margin: -4px 0 0; font-size: 12px; line-height: 1.5; }
.ai-test { margin-top: 14px; display: flex; align-items: center; gap: 8px; padding: 10px 14px; border-radius: 8px; font-size: 13px; }
.ai-test svg { width: 16px; height: 16px; flex-shrink: 0; }
.ai-test.ok { color: #67c23a; background: rgba(103, 194, 58, .08); border: 1px solid rgba(103, 194, 58, .3); }
.ai-test.bad { color: #f56c6c; background: rgba(245, 108, 108, .06); border: 1px solid rgba(245, 108, 108, .3); word-break: break-all; }
.badge.ai-src { color: #409eff; border-color: rgba(64, 158, 255, .4); }
.badge.hg-healthy { color: #67c23a; border-color: rgba(103, 194, 58, .45); }
.badge.hg-attention { color: #e6a23c; border-color: rgba(230, 162, 60, .45); }
</style>
