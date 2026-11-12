<template>
  <div class="login-wrap">
    <div class="login-box">
      <h2>{{ settings.site_title }}</h2>
      <p class="login-sub">Data Center Infrastructure Management · 北京东青互联科技</p>
      <div class="err" v-if="err">{{ err }}</div>
      <input v-model="username" placeholder="用户名" @keyup.enter="doLogin" autocomplete="username" />
      <div class="pw-row">
      <input v-model="password" :type="showPw ? 'text' : 'password'" placeholder="密码" @keyup.enter="doLogin" autocomplete="current-password" />
      <span class="pw-toggle" @click="showPw=!showPw" :title="showPw ? '隐藏密码' : '显示密码'"><SvgIcon :name="showPw ? 'eye-off' : 'eye'" /></span>
    </div>
      <div class="captcha-row">
        <input v-model="captcha" placeholder="验证码" @keyup.enter="doLogin" />
        <div class="captcha-q" @click="loadCaptcha" title="点击换一题">{{ captchaQ || '…' }}</div>
      </div>
      <button class="btn btn-primary" @click="doLogin" :disabled="loading">
        {{ loading ? '登录中...' : '登 录' }}
      </button>
      <p class="login-tip">初始账号请联系系统管理员获取</p>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useSettingsStore } from '../stores/settings'
import api from '../api'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const settings = useSettingsStore()

const username = ref('')
const password = ref('')
const captcha = ref('')
const captchaQ = ref('')
const captchaId = ref('')
const err = ref('')
const loading = ref(false)
const showPw = ref(false)

async function loadCaptcha() {
  try {
    const res = await api.get('/captcha')
    if (res.code === 0) {
      captchaQ.value = res.data.question
      captchaId.value = res.data.id
    }
  } catch (e) { /* 网络异常时保留旧题 */ }
}

onMounted(() => {
  settings.load()
  loadCaptcha()
})

async function doLogin() {
  err.value = ''
  if (loading.value) return
  if (!username.value || !password.value) { err.value = '请输入用户名和密码'; return }
  if (!captcha.value) { err.value = '请输入验证码'; return }
  loading.value = true
  try {
    await auth.login(username.value, password.value, captcha.value, captchaId.value)
    localStorage.setItem('dcim_last_username', username.value)
    router.push(route.query.redirect || '/dashboard')
  } catch (e) {
    err.value = e.response?.data?.msg || e.message || '登录失败'
    captcha.value = ''
    loadCaptcha()
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.captcha-row { display: flex; gap: 10px; margin-bottom: 14px; }
.captcha-row input { flex: 1; margin-bottom: 0 !important; }
.captcha-q {
  min-width: 96px; padding: 10px 12px; text-align: center; cursor: pointer;
  border: 1px solid var(--border); border-radius: var(--radius, 8px);
  background: var(--card-bg, var(--bg-soft)); font-size: 13px; font-weight: 600;
  user-select: none; letter-spacing: 1px;
}
.captcha-q:hover { border-color: var(--accent); }
.login-tip { text-align: center; color: var(--muted); font-size: 12px; margin-top: 12px; }
.pw-row { position: relative; }
.pw-row input { padding-right: 36px; }
.pw-toggle { position: absolute; right: 10px; top: 50%; transform: translateY(-50%); cursor: pointer; font-size: 16px; user-select: none; }
</style>
