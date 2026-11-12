<template>
  <view class="glass-tabbar">
    <view class="tab-bg" :style="{ background: bgStyle }"></view>
    <view class="tab-refract" :style="refractStyle"></view>
    <view class="tab-items">
      <view class="tab-item" v-for="t in tabs" :key="t.url" @tap="go(t.url)">
        <view class="ico-wrap" :class="{ on: t.key === current }">
          <TechIcon :name="t.icon" :color="t.key === current ? activeColor : '#8A94A6'" :size="46" />
          <view class="ico-glow" v-if="t.key === current" :style="{ background: glowStyle }"></view>
        </view>
        <text class="tab-label" :class="{ on: t.key === current }" :style="t.key === current ? { color: activeColor } : {}">{{ t.label }}</text>
      </view>
    </view>
    <view class="tab-edge"></view>
  </view>
</template>

<script>
// 自定义液态玻璃 TabBar：大字体 + 科技感 SVG 图标 + 选中辉光
import TechIcon from './TechIcon.vue'

export default {
  name: 'GlassTabBar',
  components: { TechIcon },
  props: {
    current: { type: String, default: 'index' },
    scrollY: { type: Number, default: 0 },
  },
  data() {
    return {
      activeColor: '#2D6BC4',
      tabs: [
        { key: 'index', label: '工作台', icon: 'grid', url: '/pages/index/index' },
        { key: 'alert', label: '告警', icon: 'alert', url: '/pages/alert/alert' },
        { key: 'wo', label: '工单', icon: 'ticket', url: '/pages/wo/wo' },
        { key: 'scan', label: '巡检', icon: 'scan', url: '/pages/scan/scan' },
        { key: 'me', label: '我的', icon: 'user', url: '/pages/me/me' },
      ],
    }
  },
  computed: {
    bgStyle() {
      const p = Math.min(1, this.scrollY / 240)
      return `rgba(255,255,255,${(0.72 + p * 0.22).toFixed(3)})`
    },
    refractStyle() {
      const dx = (this.scrollY * -0.3) % 360
      return `transform:translateX(${(dx - 80).toFixed(1)}px);`
    },
    glowStyle() {
      return `radial-gradient(circle, rgba(45,107,196,0.35) 0%, transparent 70%)`
    },
  },
  methods: {
    go(url) {
      if (url === '/' + getCurrentPages().slice(-1)[0].route) return
      uni.reLaunch({ url, fail: () => {} })
    },
  },
}
</script>

<style scoped>
.glass-tabbar {
  position: fixed;
  left: 0; right: 0; bottom: 0;
  z-index: 95;
  height: calc(env(safe-area-inset-bottom, 0px) + 112rpx);
  padding-bottom: env(safe-area-inset-bottom, 0px);
  backdrop-filter: blur(26px) saturate(1.7);
  -webkit-backdrop-filter: blur(26px) saturate(1.7);
  overflow: hidden;
  box-shadow: 0 -6rpx 30rpx rgba(20, 40, 80, 0.10);
}
.tab-bg { position: absolute; inset: 0; transition: background 0.25s ease; }
.tab-refract {
  position: absolute; bottom: -40%; left: 0;
  width: 200px; height: 180%;
  background: linear-gradient(75deg, transparent, rgba(120, 180, 255, 0.16), rgba(255, 255, 255, 0.22), rgba(120, 180, 255, 0.16), transparent);
  filter: blur(8px);
  pointer-events: none;
}
.tab-items { position: relative; display: flex; height: 112rpx; }
.tab-item { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; }
.ico-wrap { position: relative; display: flex; align-items: center; justify-content: center; }
.ico-glow {
  position: absolute; width: 76rpx; height: 76rpx; border-radius: 50%;
  top: 50%; left: 50%; transform: translate(-50%, -50%);
  pointer-events: none;
}
.tab-label {
  font-size: 28rpx;            /* 加大：原原生 ~20rpx */
  margin-top: 6rpx;
  color: #8A94A6;
  font-weight: 500;
  letter-spacing: 1rpx;
}
.tab-label.on { font-size: 29rpx; font-weight: 800; }
.tab-edge {
  position: absolute; top: 0; left: 0; right: 0; height: 2rpx;
  background: linear-gradient(90deg, transparent, rgba(120, 180, 255, 0.75), rgba(255, 255, 255, 0.9), rgba(120, 180, 255, 0.75), transparent);
}
</style>
