<template>
  <view class="page">
    <GlassNavBar title="服务器设置" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>
    <view class="card">
      <view class="lbl">服务器地址</view>
      <input class="in" v-model="url" placeholder="如 https://dcim.example.com" />
      <view class="muted cur">当前生效：{{ current || '（同源默认）' }}</view>
      <view class="presets">
        <text class="preset" v-for="p in presets" :key="p" @tap="url = p">{{ p }}</text>
      </view>
    </view>
    <button class="btn-primary" @tap="save">保存并重启应用</button>
    <view class="muted tip">保存后应用将重新加载并用新地址请求接口；登录状态保留，若跨环境请重新登录。</view>
    <button class="btn-ghost reset" @tap="reset">恢复默认（同源）</button>
    <GlassTabBar current="me" :scrollY="scrollY" />
  </view>
</template>

<script>
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import { CONFIG, setBaseUrl } from '../../config.js'

export default {
  components: { GlassNavBar, GlassTabBar },
  data() {
    return {
      scrollY: 0,
      navH: 44,
      url: uni.getStorageSync('dcim_base_url') || '',
      current: CONFIG.BASE_URL,
      presets: ['https://dcim.example.com', 'http://192.168.1.100'],
    }
  },
  onLoad() {
    try { this.navH = (uni.getSystemInfoSync().statusBarHeight || 0) + 44 } catch (e) {}
  },
  onPageScroll(e) { this.scrollY = e.scrollTop },
  methods: {
    save() {
      let u = (this.url || '').trim().replace(/\/+$/, '')
      if (u && !/^https?:\/\//.test(u)) u = 'http://' + u
      setBaseUrl(u)
      uni.showToast({ title: '已保存', icon: 'success' })
      setTimeout(() => {
        // #ifdef H5
        location.reload()
        // #endif
        // #ifndef H5
        uni.reLaunch({ url: '/pages/index/index', fail: () => {} })
        // #endif
      }, 600)
    },
    reset() {
      uni.removeStorageSync('dcim_base_url')
      uni.showToast({ title: '已恢复默认', icon: 'success' })
      setTimeout(() => {
        // #ifdef H5
        location.reload()
        // #endif
        // #ifndef H5
        uni.reLaunch({ url: '/pages/me/server', fail: () => {} })
        // #endif
      }, 600)
    },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 40rpx; }
.lbl { font-size: 26rpx; font-weight: 700; color: #1f4e9c; margin-bottom: 14rpx; }
.in { background: #f4f6fa; border-radius: 12rpx; padding: 20rpx; font-size: 28rpx; }
.cur { margin-top: 14rpx; }
.presets { margin-top: 16rpx; display: flex; flex-wrap: wrap; }
.preset {
  font-size: 24rpx; color: #1f4e9c; background: #eef2f8;
  border-radius: 8rpx; padding: 8rpx 18rpx; margin: 0 12rpx 12rpx 0;
}
.tip { margin-top: 20rpx; line-height: 1.5; }
.reset { margin-top: 24rpx; color: #8a94a6; }
.page { padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 170rpx) !important; }
</style>
