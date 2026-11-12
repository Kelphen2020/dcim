<template>
  <view class="login-page">
    <!-- 氛围光（深色封面取色） -->
    <view class="login-atmo">
      <AtmosphereGlow seed="dcim-login" base="#2D6BC4" :scrollY="0" />
    </view>
    <view class="logo-area">
      <view class="logo-badge">
        <TechIcon name="cpu" color="#fff" :size="56" />
      </view>
      <view class="logo">DCIM</view>
      <view class="sub">数据中心基础设施管理</view>
      <view class="sub-en">MOBILE OPS CONSOLE</view>
    </view>
    <view class="form glass-form">
      <view class="field">
        <view class="icon"><TechIcon name="user" color="#8A94A6" :size="34" /></view>
        <input v-model="username" placeholder="用户名" placeholder-class="ph" />
      </view>
      <view class="field">
        <view class="icon"><TechIcon name="lock" color="#8A94A6" :size="34" /></view>
        <input v-model="password" :password="!showPw" placeholder="密码" placeholder-class="ph" />
        <view class="eye" @tap="showPw = !showPw"><TechIcon :name="showPw ? 'eye-off' : 'eye'" color="#8A94A6" :size="32" /></view>
      </view>
      <view class="field">
        <view class="icon"><TechIcon name="cpu" color="#8A94A6" :size="34" /></view>
        <input v-model="captcha" type="number" placeholder="验证码答案" placeholder-class="ph" />
        <text class="cap-q" @tap="loadCaptcha">{{ capQuestion || '点击加载' }}</text>
      </view>
      <button class="btn-primary login-btn" :disabled="loading" @tap="doLogin">
        {{ loading ? '登录中…' : '登 录' }}
      </button>
      <!-- #ifdef MP-WEIXIN -->
      <view v-if="wxMode" class="wx-hint">首次使用：请输入上方账号密码，完成微信绑定</view>
      <button v-if="!wxMode" class="btn-wx" :disabled="loading" @tap="doWxLogin">
        <TechIcon name="user" color="#fff" :size="30" /> 微信一键登录
      </button>
      <button v-else class="btn-wx" :disabled="loading" @tap="doWxBind">
        <TechIcon name="lock" color="#fff" :size="30" /> 绑定微信并登录
      </button>
      <!-- #endif -->
      <view class="ver">版本 {{ version }} · 北京东青互联科技</view>
    </view>
  </view>
</template>

<script>
import TechIcon from '../../components/TechIcon.vue'
import AtmosphereGlow from '../../components/AtmosphereGlow.vue'
import { get, post } from '../../api.js'
import { CONFIG } from '../../config.js'

export default {
  components: { TechIcon, AtmosphereGlow },
  data() {
    return {
      username: '',
      password: '',
      captcha: '',
      capId: '',
      capQuestion: '',
      showPw: false,
      loading: false,
      version: CONFIG.VERSION,
      wxMode: false,     // 小程序端：true=待绑定态（输入账号密码绑定微信）
      wxTicket: '',      // 后端下发的一次性绑定票据
    }
  },
  onLoad() {
    this.loadCaptcha()
  },
  methods: {
    async loadCaptcha() {
      try {
        const d = await get('/captcha', {}, { silent: true })
        this.capId = d.id
        this.capQuestion = d.question
      } catch (e) {
        this.capQuestion = '加载失败，点击重试'
      }
    },
    solve(q) {
      const nums = (q.match(/\d+/g) || []).map(Number)
      if (nums.length < 2) return ''
      if (q.includes('+')) return String(nums[0] + nums[1])
      if (q.includes('-') || q.includes('−')) return String(nums[0] - nums[1])
      if (q.includes('×') || q.includes('*')) return String(nums[0] * nums[1])
      return ''
    },
    async doLogin() {
      if (!this.username || !this.password) {
        uni.showToast({ title: '请输入用户名密码', icon: 'none' })
        return
      }
      let ans = this.captcha.trim()
      if (!ans) {
        uni.showToast({ title: '请输入验证码答案', icon: 'none' })
        return
      }
      this.loading = true
      try {
        const d = await post('/login', {
          username: this.username,
          password: this.password,
          captcha: String(ans),
          captcha_id: this.capId,
        }, { silent: true })
        uni.setStorageSync('dcim_token', d.token)
        uni.setStorageSync('dcim_user', d.user)
        uni.reLaunch({ url: '/pages/index/index', fail: () => {} })
      } catch (e) {
        uni.showToast({ title: e.message || '登录失败', icon: 'none' })
        this.loadCaptcha()
        this.captcha = ''
      } finally {
        this.loading = false
      }
    },
    // #ifdef MP-WEIXIN
    // 微信一键登录：wx.login 拿 code → 后端换 openid；未绑定则进入绑定态
    doWxLogin() {
      this.loading = true
      uni.login({
        provider: 'weixin',
        success: async (res) => {
          try {
            const d = await post('/wx-login', { code: res.code }, { silent: true })
            if (d.bound) {
              this.onLoginOk(d)
            } else {
              this.wxMode = true
              this.wxTicket = d.wx_ticket
              uni.showToast({ title: '请先绑定账号密码', icon: 'none' })
            }
          } catch (e) {
            uni.showToast({ title: e.message || '微信登录失败', icon: 'none' })
          } finally {
            this.loading = false
          }
        },
        fail: () => {
          this.loading = false
          uni.showToast({ title: '微信登录取消', icon: 'none' })
        },
      })
    },
    // 绑定微信：票据 + 账号密码 → 写 openid 并登录
    async doWxBind() {
      if (!this.username || !this.password) {
        uni.showToast({ title: '请输入账号密码完成绑定', icon: 'none' })
        return
      }
      this.loading = true
      try {
        const d = await post('/wx-bind', {
          wx_ticket: this.wxTicket,
          username: this.username,
          password: this.password,
        }, { silent: true })
        this.onLoginOk(d)
      } catch (e) {
        uni.showToast({ title: e.message || '绑定失败', icon: 'none' })
        this.wxMode = false
        this.wxTicket = ''
      } finally {
        this.loading = false
      }
    },
    onLoginOk(d) {
      uni.setStorageSync('dcim_token', d.token)
      uni.setStorageSync('dcim_user', d.user)
      uni.showToast({ title: '登录成功', icon: 'success' })
      setTimeout(() => uni.reLaunch({ url: '/pages/index/index', fail: () => {} }), 300)
    },
    // #endif
  },
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  background: linear-gradient(165deg, #0E2247 0%, #16335F 34%, #1F4E9C 52%, #F4F6FA 52.1%);
  padding: 0 48rpx;
  position: relative;
  overflow: hidden;
}
.login-atmo { position: absolute; top: 0; left: 0; right: 0; height: 52vh; overflow: hidden; pointer-events: none; }
.logo-area { position: relative; padding: 130rpx 0 70rpx; text-align: center; z-index: 1; }
.logo-badge {
  width: 110rpx; height: 110rpx; margin: 0 auto 24rpx;
  border-radius: 32rpx;
  background: rgba(255, 255, 255, 0.14);
  border: 1rpx solid rgba(255, 255, 255, 0.28);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  display: flex; align-items: center; justify-content: center;
  box-shadow: 0 12rpx 40rpx rgba(0, 0, 0, 0.2);
}
.logo { font-size: 84rpx; font-weight: 800; color: #fff; letter-spacing: 10rpx; text-shadow: 0 6rpx 30rpx rgba(0, 0, 0, 0.25); }
.sub { color: rgba(220, 233, 255, 0.9); font-size: 28rpx; margin-top: 12rpx; letter-spacing: 4rpx; }
.sub-en { color: rgba(160, 190, 235, 0.6); font-size: 20rpx; margin-top: 10rpx; letter-spacing: 6rpx; }
.form.glass-form {
  position: relative; z-index: 1;
  background: rgba(255, 255, 255, 0.86);
  backdrop-filter: blur(24px) saturate(1.5);
  -webkit-backdrop-filter: blur(24px) saturate(1.5);
  border: 1rpx solid rgba(255, 255, 255, 0.7);
  border-radius: 32rpx;
  padding: 48rpx 36rpx;
  box-shadow: 0 16rpx 60rpx rgba(14, 34, 71, 0.22);
}
.field {
  display: flex;
  align-items: center;
  border-bottom: 1rpx solid #e5eaf3;
  padding: 24rpx 0;
}
.icon { margin-right: 16rpx; font-size: 30rpx; display: flex; }
.field input { flex: 1; font-size: 30rpx; }
.ph { color: #b6bfce; }
.eye { padding: 0 8rpx; display: flex; }
.cap-q { color: #1f4e9c; font-size: 28rpx; font-weight: 600; max-width: 220rpx; text-align: right; }
.login-btn {
  margin-top: 48rpx;
  background: linear-gradient(135deg, #1F4E9C, #2D6BC4);
  box-shadow: 0 10rpx 30rpx rgba(31, 78, 156, 0.35);
}
.ver { text-align: center; color: #b6bfce; font-size: 22rpx; margin-top: 32rpx; }
.btn-wx {
  margin-top: 20rpx;
  background: linear-gradient(135deg, #07C160, #0AA85A);
  color: #fff;
  border-radius: 16rpx;
  font-size: 30rpx;
  font-weight: 600;
  box-shadow: 0 8rpx 24rpx rgba(7, 193, 96, 0.3);
  display: flex; align-items: center; justify-content: center; gap: 10rpx;
}
.wx-hint { text-align: center; color: #1f4e9c; font-size: 24rpx; margin-top: 20rpx; }
</style>
