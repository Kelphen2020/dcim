<template>
  <view class="glass-nav" :style="navStyle">
    <!-- 折射高光层：随滚动位移 -->
    <view class="glass-refract" :style="refractStyle"></view>
    <view class="glass-inner">
      <view class="nav-left" v-if="back" @tap="goBack">
        <TechIcon name="x" :color="fg" size="0" />
        <text class="back-t" :style="{ color: fg }">‹ 返回</text>
      </view>
      <view class="nav-left" v-else @tap="$emit('leftTap')">
        <slot name="left"></slot>
      </view>
      <view class="nav-title" :style="{ color: fg }">{{ title }}</view>
      <view class="nav-right" @tap="$emit('rightTap')">
        <slot name="right"></slot>
      </view>
    </view>
    <view class="glass-edge" :style="{ opacity: edgeOpacity }"></view>
  </view>
</template>

<script>
// 液态玻璃导航栏：
// - backdrop-filter 毛玻璃 + 轻微折射层（斜向渐变，随滚动位移）
// - 边缘高光强度随滚动进度变化
// - 玻璃/文字颜色按下方内容亮度自适应（dark prop 或页面驱动）
import TechIcon from './TechIcon.vue'

export default {
  name: 'GlassNavBar',
  components: { TechIcon },
  props: {
    title: { type: String, default: '' },
    back: { type: Boolean, default: false },
    scrollY: { type: Number, default: 0 },
    dark: { type: Boolean, default: false }, // true=下方内容深色→亮字深玻璃
  },
  emits: ['leftTap', 'rightTap'],
  computed: {
    fg() { return this.dark ? '#F2F6FF' : '#16233D' },
    navStyle() {
      // 滚动加深玻璃浓度
      const p = Math.min(1, this.scrollY / 240)
      const bg = this.dark
        ? `rgba(10,18,38,${(0.42 + p * 0.3).toFixed(3)})`
        : `rgba(255,255,255,${(0.42 + p * 0.32).toFixed(3)})`
      return `background:${bg};`
    },
    refractStyle() {
      // 折射高光随滚动水平位移
      const dx = (this.scrollY * 0.35) % 400
      return `transform:translateX(${(dx - 120).toFixed(1)}px);`
    },
    edgeOpacity() {
      return (0.25 + Math.min(1, this.scrollY / 200) * 0.55).toFixed(2)
    },
  },
  methods: {
    goBack() {
      const pages = getCurrentPages()
      if (pages.length > 1) uni.navigateBack()
      else uni.reLaunch({ url: '/pages/index/index', fail: () => {} })
    },
  },
}
</script>

<style scoped>
.glass-nav {
  position: fixed;
  top: 0; left: 0; right: 0;
  z-index: 90;
  height: calc(var(--status-bar-height, 0px) + 88rpx);
  padding-top: var(--status-bar-height, 0px);
  backdrop-filter: blur(24px) saturate(1.6);
  -webkit-backdrop-filter: blur(24px) saturate(1.6);
  overflow: hidden;
  transition: background 0.25s ease;
}
.glass-refract {
  position: absolute; top: -30%; left: 0;
  width: 240px; height: 160%;
  background: linear-gradient(105deg, transparent 0%, rgba(255, 255, 255, 0.16) 42%, rgba(180, 214, 255, 0.22) 50%, rgba(255, 255, 255, 0.14) 58%, transparent 100%);
  filter: blur(6px);
  pointer-events: none;
}
.glass-inner {
  position: relative;
  height: 88rpx;
  display: flex; align-items: center;
  padding: 0 24rpx;
}
.nav-left { min-width: 140rpx; display: flex; align-items: center; }
.back-t { font-size: 28rpx; margin-left: -6rpx; }
.nav-title {
  flex: 1; text-align: center;
  font-size: 32rpx; font-weight: 700; letter-spacing: 1rpx;
}
.nav-right { min-width: 140rpx; display: flex; align-items: center; justify-content: flex-end; }
.glass-edge {
  position: absolute; bottom: 0; left: 0; right: 0; height: 2rpx;
  background: linear-gradient(90deg, transparent, rgba(120, 180, 255, 0.9), rgba(255, 255, 255, 0.95), rgba(120, 180, 255, 0.9), transparent);
  pointer-events: none;
}
</style>
