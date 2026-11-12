<template>
  <div class="page">
    <div class="page-head">
      <h2><SvgIcon name="user" />用户中心</h2>
      <span class="muted">管理个人资料、登录密码与界面主题偏好</span>
    </div>

    <div class="uc-grid">
      <!-- 左：账号信息卡 -->
      <div class="card uc-acct">
        <div class="uc-avatar">{{ (profile.display_name || profile.username || '?').slice(0, 1) }}</div>
        <div class="uc-name">{{ profile.display_name || profile.username }}</div>
        <div class="uc-sub mono">@{{ profile.username }}</div>
        <span class="role-tag" :class="'role-' + profile.role">{{ roleLabel }}</span>
        <div class="uc-meta">
          <div><span class="muted">所属站点</span><b>{{ siteText }}</b></div>
          <div><span class="muted">上次登录</span><b>{{ fmt(profile.last_login_at) }}</b></div>
          <div><span class="muted">微信绑定</span><b :class="profile.wx_bound ? 'ok-text' : 'muted'">{{ profile.wx_bound ? '已绑定' : '未绑定' }}</b></div>
        </div>
        <button v-if="profile.wx_bound" class="btn btn-sm uc-unbind" @click="unbindWx">解除微信绑定</button>
      </div>

      <!-- 右：设置区 -->
      <div class="uc-main">
        <div class="card">
          <div class="uc-sec-title"><SvgIcon name="pen" />基本资料</div>
          <div class="uc-form">
            <label>显示名称
              <input class="input" v-model="editName" maxlength="50" placeholder="用于界面展示的名称" />
            </label>
            <label>手机号码
              <input class="input" v-model="editPhone" maxlength="20" placeholder="接收告警通知用，如 13800001234" />
            </label>
            <label>电子邮箱
              <input class="input" v-model="editEmail" maxlength="100" placeholder="name@company.com" />
            </label>
            <div class="uc-actions">
              <button class="btn btn-primary" :disabled="savingProfile" @click="saveProfile">
                {{ savingProfile ? '保存中…' : '保存资料' }}
              </button>
              <span v-if="profileMsg" class="uc-msg" :class="profileMsg.ok ? 'ok-text' : 'err-text'">{{ profileMsg.text }}</span>
            </div>
          </div>
        </div>

        <div class="card">
          <div class="uc-sec-title"><SvgIcon name="lock" />修改密码</div>
          <div class="uc-theme-hint muted">密码至少 8 位，须包含大写字母、小写字母、数字与特殊字符；修改成功后需重新登录。</div>
          <div class="uc-form">
            <label>原密码
              <input class="input" type="password" v-model="pwd.old" placeholder="当前使用的登录密码" />
            </label>
            <label>新密码
              <input class="input" type="password" v-model="pwd.new1" placeholder="至少8位，含大小写字母和数字" />
            </label>
            <label>确认新密码
              <input class="input" type="password" v-model="pwd.new2" placeholder="再输入一次新密码" />
            </label>
            <div class="uc-actions">
              <button class="btn btn-primary" :disabled="savingPwd" @click="savePwd">
                {{ savingPwd ? '提交中…' : '修改密码' }}
              </button>
              <span v-if="pwdMsg" class="uc-msg" :class="pwdMsg.ok ? 'ok-text' : 'err-text'">{{ pwdMsg.text }}</span>
            </div>
          </div>
        </div>

        <div class="card">
          <div class="uc-sec-title"><SvgIcon name="color" />界面主题</div>
          <div class="uc-theme-hint muted">仅影响您自己的浏览器；管理员可在「系统设置」修改站点默认主题。</div>
          <div class="uc-theme-grid">
            <div v-for="t in themes" :key="t.key"
                 class="uc-theme-card" :class="{ active: settings.theme === t.key }"
                 @click="pickTheme(t.key)" :title="t.desc + ' · ' + t.scene">
              <div class="uc-swatch">
                <span v-for="(c, i) in t.palette.slice(0, 4)" :key="i" :style="{ background: c }"></span>
              </div>
              <div class="uc-theme-name">{{ t.name }}<em v-if="settings.theme === t.key">使用中</em></div>
              <div class="muted small">{{ t.dark ? '深色' : '浅色' }} · {{ t.scene }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { reactive, ref, computed, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import { useSettingsStore, THEME_PRESETS } from '../stores/settings'
import SvgIcon from '../components/SvgIcon.vue'
import api from '../api'
import { toastAuto, showConfirm } from '../ui-dialog'

const auth = useAuthStore()
const settings = useSettingsStore()
const themes = THEME_PRESETS

const ROLE_LABELS = { global_admin: '全局管理员', site_admin: '站点管理员', operator: '值班员', viewer: '只读' }
const profile = reactive({ username: '', display_name: '', role: '', phone: '', email: '', last_login_at: null, wx_bound: false })
const roleLabel = computed(() => ROLE_LABELS[profile.role] || profile.role)
const siteText = ref('全部站点')
async function loadSites() {
  const ids = auth.site_ids || []
  if (!ids.length) { siteText.value = '全部站点'; return }
  try {
    const r = await api.get('/sites', { size: 100 })
    const all = r.data?.items || []
    const names = ids.map(id => (all.find(s => s.id === id) || {}).name || ('站点' + id))
    siteText.value = names.join('、')
  } catch { siteText.value = ids.length + ' 个站点' }
}
const fmt = t => t ? String(t).replace('T', ' ').slice(0, 16) : '-'

const editName = ref(''); const editPhone = ref(''); const editEmail = ref('')
const savingProfile = ref(false); const profileMsg = ref(null)
const pwd = reactive({ old: '', new1: '', new2: '' })
const savingPwd = ref(false); const pwdMsg = ref(null)

async function load() {
  const r = await api.get('/me')
  if (r.code === 0) {
    Object.assign(profile, r.data.user)
    profile.wx_bound = !!r.data.wx_bound
    editName.value = profile.display_name || ''
    editPhone.value = profile.phone || ''
    editEmail.value = profile.email || ''
    // 同步本地缓存（头像/名称在顶栏即时更新）
    auth.user = { ...auth.user, ...r.data.user }
    localStorage.setItem('dcim_user', JSON.stringify(auth.user))
  }
}
onMounted(() => { load(); loadSites() })

async function saveProfile() {
  savingProfile.value = true; profileMsg.value = null
  try {
    const body = {}
    if (editName.value.trim() !== (profile.display_name || '')) body.display_name = editName.value.trim()
    if (editPhone.value.trim() !== (profile.phone || '')) body.phone = editPhone.value.trim()
    if (editEmail.value.trim() !== (profile.email || '')) body.email = editEmail.value.trim()
    if (!Object.keys(body).length) { profileMsg.value = { ok: false, text: '没有修改任何内容' }; return }
    const r = await api.put('/me/profile', body)
    if (r.code === 0) {
      profileMsg.value = { ok: true, text: '已保存' }
      await load()
    } else profileMsg.value = { ok: false, text: r.msg || '保存失败' }
  } finally { savingProfile.value = false }
}

async function savePwd() {
  savingPwd.value = true; pwdMsg.value = null
  try {
    if (!pwd.old || !pwd.new1) { pwdMsg.value = { ok: false, text: '请填写原密码和新密码' }; return }
    if (pwd.new1.length < 8) { pwdMsg.value = { ok: false, text: '新密码至少 8 位' }; return }
    if (!/[A-Z]/.test(pwd.new1) || !/[a-z]/.test(pwd.new1) || !/[0-9]/.test(pwd.new1) || !/[^A-Za-z0-9]/.test(pwd.new1)) {
      pwdMsg.value = { ok: false, text: '密码须包含大写字母、小写字母、数字与特殊字符' }; return
    }
    if (pwd.new1 !== pwd.new2) { pwdMsg.value = { ok: false, text: '两次输入的新密码不一致' }; return }
    const r = await api.post('/me/password', { old_password: pwd.old, new_password: pwd.new1 })
    if (r.code === 0) {
      pwdMsg.value = { ok: true, text: '密码已修改，1.5秒后请重新登录' }
      pwd.old = pwd.new1 = pwd.new2 = ''
      setTimeout(() => { auth.logout(); location.hash = '#/login'; location.reload() }, 1500)
    } else pwdMsg.value = { ok: false, text: r.msg || '修改失败' }
  } finally { savingPwd.value = false }
}

function pickTheme(key) {
  settings.updateLocal({ theme: key })
}

async function unbindWx() {
  if (!(await showConfirm({ message: '确认解除微信绑定？解除后小程序端将无法一键登录。' }))) return
  const r = await api.post('/me/wx-unbind', {})
  if (r.code === 0) { profile.wx_bound = false; toastAuto('已解除绑定') } else toastAuto(r.msg || '解绑失败')
}
</script>

<style scoped>
.page-head { display: flex; align-items: baseline; gap: 12px; margin-bottom: 16px; }
.uc-grid { display: grid; grid-template-columns: 280px 1fr; gap: 16px; align-items: start; }
.uc-acct { text-align: center; padding: 28px 20px; }
.uc-avatar { width: 64px; height: 64px; border-radius: 50%; background: var(--accent); color: #fff; display: flex; align-items: center; justify-content: center; font-size: 26px; font-weight: 600; margin: 0 auto 12px; }
.uc-name { font-size: 17px; font-weight: 600; }
.uc-sub { font-size: 12px; color: var(--muted); margin: 2px 0 10px; }
.role-tag { display: inline-block; font-size: 11px; padding: 2px 10px; border-radius: 10px; background: var(--accent-glow, rgba(22,119,255,.1)); color: var(--accent); }
.uc-meta { margin-top: 18px; text-align: left; display: flex; flex-direction: column; gap: 10px; border-top: 1px solid var(--border); padding-top: 16px; }
.uc-meta > div { display: flex; justify-content: space-between; font-size: 13px; gap: 8px; }
.uc-meta b { font-weight: 500; text-align: right; }
.uc-unbind { margin-top: 14px; color: var(--red, #f56c6c); border-color: rgba(245,108,108,.4); }
.uc-main { display: flex; flex-direction: column; gap: 16px; }
.card { background: var(--card, #fff); border: 1px solid var(--border); border-radius: 12px; padding: 18px 20px; }
.uc-sec-title { font-size: 14px; font-weight: 600; display: flex; align-items: center; gap: 6px; margin-bottom: 14px; }
.uc-form { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; }
.uc-form label { display: flex; flex-direction: column; gap: 6px; font-size: 12.5px; color: var(--muted); }
.uc-actions { grid-column: 1 / -1; display: flex; align-items: center; gap: 12px; }
.uc-msg { font-size: 12.5px; }
.ok-text { color: var(--green, #00b96b); }
.err-text { color: var(--red, #f56c6c); }
.uc-theme-hint { font-size: 12px; margin-bottom: 12px; }
.uc-theme-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 10px; }
.uc-theme-card { border: 2px solid var(--border); border-radius: 10px; padding: 10px 12px; cursor: pointer; transition: border-color .2s, transform .15s; }
.uc-theme-card:hover { transform: translateY(-2px); }
.uc-theme-card.active { border-color: var(--accent); }
.uc-swatch { display: flex; gap: 4px; margin-bottom: 8px; }
.uc-swatch span { width: 22px; height: 12px; border-radius: 3px; }
.uc-theme-name { font-size: 13px; font-weight: 600; display: flex; justify-content: space-between; align-items: center; }
.uc-theme-name em { font-style: normal; font-size: 10.5px; color: var(--accent); border: 1px solid currentColor; border-radius: 8px; padding: 0 6px; }
@media (max-width: 1000px) {
  .uc-grid { grid-template-columns: 1fr; }
  .uc-form { grid-template-columns: 1fr; }
}
</style>
