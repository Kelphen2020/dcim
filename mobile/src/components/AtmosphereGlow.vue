<template>
  <view class="atmo" :style="wrapStyle">
    <view class="atmo-core" :style="coreStyle"></view>
    <view class="atmo-halo" :style="haloStyle"></view>
    <view class="atmo-ring" :style="ringStyle"></view>
  </view>
</template>

<script>
// 氛围光：从主色（站点/主题派生）生成环境光晕，大半径柔光铺底，
// 主色变化时色值连续插值过渡（不跳变）
import { hexToRgb, mixColors, hashHue, hslHex } from '../fx.js'

export default {
  name: 'AtmosphereGlow',
  props: {
    seed: { type: String, default: 'dcim' },   // 取色种子（如站点名）
    base: { type: String, default: '#1F4E9C' }, // 基础主色
    scrollY: { type: Number, default: 0 },
  },
  data() {
    return { cur: this.base, target: this.base, timer: null }
  },
  computed: {
    wrapStyle() {
      // 随滚动轻微下移（景深远层）
      return `transform:translate3d(0,${(this.scrollY * 0.25).toFixed(1)}px,0);opacity:${(1 - Math.min(1, this.scrollY / 500) * 0.45).toFixed(3)};`
    },
    coreStyle() {
      return `background:radial-gradient(circle at 50% 45%, ${this.cur} 0%, rgba(255,255,255,0) 62%);`
    },
    haloStyle() {
      return `background:radial-gradient(circle at 50% 50%, ${this.cur} 0%, rgba(255,255,255,0) 70%);`
    },
    ringStyle() {
      return `box-shadow:0 0 90rpx 30rpx ${this.cur};`
    },
  },
  watch: {
    seed() { this.retarget() },
    base() { this.retarget() },
  },
  created() {
    this.retarget()
  },
  beforeUnmount() {
    if (this.timer) clearInterval(this.timer)
    this.timer = null
  },
  methods: {
    retarget() {
      // 由种子派生主色（色相偏移，保持同明度体系），作为插值目标
      const hue = hashHue(this.seed, 215)
      this.target = hslHex(hue, 62, 42)
      this.startTween()
    },
    startTween() {
      if (this.timer) return
      this.timer = setInterval(() => {
        const A = hexToRgb(this.cur.startsWith('#') ? this.cur : this.base)
        const B = hexToRgb(this.target)
        const dr = B.r - A.r, dg = B.g - A.g, db = B.b - A.b
        if (Math.abs(dr) + Math.abs(dg) + Math.abs(db) < 3) {
          this.cur = this.target
          clearInterval(this.timer); this.timer = null
          return
        }
        // 每帧 12% 逼近 → 连续插值过渡
        const to2 = (x) => Math.round(x).toString(16).padStart(2, '0')
        this.cur = `#${to2(A.r + dr * 0.12)}${to2(A.g + dg * 0.12)}${to2(A.b + db * 0.12)}`
      }, 40)
    },
  },
}
</script>

<style scoped>
.atmo {
  position: absolute;
  top: -180rpx; left: 50%;
  width: 1200rpx; height: 900rpx;
  margin-left: -600rpx;
  pointer-events: none;
  z-index: 0;
}
.atmo-core {
  position: absolute; inset: 0;
  opacity: 0.40;
  filter: blur(10rpx);
}
.atmo-halo {
  position: absolute;
  left: -12%; top: -8%;
  width: 124%; height: 124%;
  opacity: 0.20;
  filter: blur(60rpx);   /* 大半径柔光 */
}
.atmo-ring {
  position: absolute;
  left: 50%; top: 42%;
  width: 10rpx; height: 10rpx;
  margin: -5rpx 0 0 -5rpx;
  border-radius: 50%;
  opacity: 0.18;
  filter: blur(4rpx);
}
</style>
