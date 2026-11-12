<template>
  <view class="page">
    <GlassNavBar title="推送设置" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>
    <view class="card">
      <view class="lbl">当前设备推送绑定</view>
      <view v-if="!channels.length" class="muted none">本设备尚未绑定推送通道</view>
      <view class="ch-item" v-for="c in channels" :key="c.id">
        <view class="grow">
          <view>{{ chText(c.channel) }} · {{ c.platform || '-' }}</view>
          <view class="muted">{{ c.device_id }} · 事件: {{ c.events || '*' }}</view>
        </view>
        <switch :checked="c.enabled" color="#1f4e9c" @change="toggle(c, $event)" />
        <text class="unbind" @tap="unbind(c)">解绑</text>
      </view>
    </view>

    <view class="card">
      <view class="lbl">订阅事件（本机绑定）</view>
      <view class="ev" v-for="e in eventDefs" :key="e.k">
        <text>{{ e.t }}</text>
        <switch :checked="evSet.includes(e.k)" color="#1f4e9c" @change="toggleEvent(e.k, $event)" />
      </view>
    </view>

    <button class="btn-primary" :disabled="binding" @tap="bind">
      {{ binding ? '绑定中…' : (myChannel ? '更新本机绑定' : '绑定本机推送') }}
    </button>
    <view class="muted tip">绑定后告警/工单等事件将通过所选通道推送到本设备。App 端集成厂商推送后自动升级为离线推送。</view>
    <GlassTabBar current="me" :scrollY="scrollY" />
  </view>
</template>

<script>
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import { get, post, del } from '../../api.js'
import { requireAuth } from '../../util.js'

const CH = { unipush: 'App 推送', wxsub: '微信订阅消息', feishu: '飞书卡片' }

export default {
  components: { GlassNavBar, GlassTabBar },
  data() {
    return {
      scrollY: 0,
      navH: 44,
      channels: [],
      deviceId: '',
      events: ['alarm', 'work_order', 'sla', 'maintenance'],
      binding: false,
      eventDefs: [
        { k: 'alarm', t: '告警触发/恢复/升级' },
        { k: 'work_order', t: '工单创建/改派' },
        { k: 'sla', t: 'SLA 预警/违约' },
        { k: 'maintenance', t: '维保到期' },
      ],
    }
  },
  computed: {
    evSet() { return this.events },
    myChannel() { return this.channels.find((c) => c.device_id === this.deviceId) },
  },
  onLoad() {
    try { this.navH = (uni.getSystemInfoSync().statusBarHeight || 0) + 44 } catch (e) {}
    if (!requireAuth()) return
    // 稳定的设备标识（首次生成后持久化）
    let dev = uni.getStorageSync('dcim_device_id')
    if (!dev) {
      // #ifdef MP-WEIXIN
      dev = 'wxmp-' + Math.random().toString(36).slice(2, 10) + Date.now().toString(36)
      // #endif
      // #ifdef H5
      dev = 'h5-' + Math.random().toString(36).slice(2, 10) + Date.now().toString(36)
      // #endif
      // #ifdef APP-PLUS
      dev = 'app-' + (plus.device.uuid || Math.random().toString(36).slice(2))
      // #endif
      if (!dev) dev = 'dev-' + Date.now().toString(36)
      uni.setStorageSync('dcim_device_id', dev)
    }
    this.deviceId = dev
    const mc = this.myChannel
    if (mc && mc.events && mc.events !== '*') this.events = mc.events.split(',')
    this.load()
  },
  onPageScroll(e) { this.scrollY = e.scrollTop },
  methods: {
    chText(c) { return CH[c] || c },
    async load() {
      try {
        const d = await get('/mobile/push-channels')
        this.channels = d.items || []
      } catch (e) { /* ignore */ }
    },
    platform() {
      // #ifdef MP-WEIXIN
      return 'wxmp'
      // #endif
      // #ifdef H5
      return 'h5'
      // #endif
      // #ifdef APP-PLUS
      return uni.getSystemInfoSync().platform || 'app'
      // #endif
      return 'unknown'
    },
    channelType() {
      // #ifdef MP-WEIXIN
      return 'wxsub'
      // #endif
      return 'unipush'
    },
    toggleEvent(k, e) {
      const on = e.detail.value
      if (on && !this.events.includes(k)) this.events.push(k)
      if (!on) this.events = this.events.filter((x) => x !== k)
    },
    async bind() {
      this.binding = true
      try {
        await post('/mobile/push-channels', {
          device_id: this.deviceId,
          channel: this.channelType(),
          platform: this.platform(),
          events: this.events.length ? this.events.join(',') : '*',
          token: 'pending-native-integration',
        })
        uni.showToast({ title: '绑定成功', icon: 'success' })
        this.load()
      } catch (e) { /* toast */ } finally { this.binding = false }
    },
    async toggle(c, e) {
      try {
        await post('/mobile/push-channels', {
          device_id: c.device_id, channel: c.channel, platform: c.platform, enabled: e.detail.value,
        })
        c.enabled = e.detail.value
      } catch (err) { /* toast */ }
    },
    unbind(c) {
      uni.showModal({
        title: '解绑', content: `解绑 ${c.device_id}？`,
        success: async (r) => {
          if (!r.confirm) return
          try {
            await del(`/mobile/push-channels/${c.id}`)
            uni.showToast({ title: '已解绑', icon: 'success' })
            this.load()
          } catch (e) { /* toast */ }
        },
      })
    },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 40rpx; }
.lbl { font-size: 26rpx; font-weight: 700; color: #1f4e9c; margin-bottom: 14rpx; }
.none { padding: 10rpx 0 20rpx; }
.ch-item { display: flex; align-items: center; padding: 20rpx 0; border-bottom: 1rpx solid #f0f3f8; font-size: 27rpx; }
.ch-item:last-child { border-bottom: none; }
.unbind { color: #e5484d; font-size: 24rpx; margin-left: 16rpx; }
.ev { display: flex; justify-content: space-between; align-items: center; padding: 16rpx 0; font-size: 27rpx; border-bottom: 1rpx solid #f0f3f8; }
.ev:last-child { border-bottom: none; }
.tip { margin-top: 20rpx; line-height: 1.5; }
.page { padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 170rpx) !important; }
</style>
