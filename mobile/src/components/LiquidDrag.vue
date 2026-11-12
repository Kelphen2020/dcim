<template>
  <view class="liquid-wrap">
    <canvas class="liquid-canvas" :canvas-id="cid" :id="cid"
      :style="{ width: w + 'px', height: h + 'px' }" disable-scroll="true"></canvas>
    <view class="liquid-dot origin" :style="originStyle">
      <slot name="origin"><TechIcon :name="originIcon" color="#fff" :size="34" /></slot>
    </view>
    <view class="liquid-dot handle" :style="handleStyle"
      @touchstart="onStart" @touchmove.stop.prevent="onMove" @touchend="onEnd">
      <slot><TechIcon :name="icon" color="#fff" :size="38" /></slot>
    </view>
    <view class="liquid-tip" v-if="broken">已断开</view>
  </view>
</template>

<script>
// 液滴粘连拖拽：拖动元素与原位之间生成液态连接（贝塞尔颈部），
// 距离增大颈部收缩，超过阈值断开，两端回弹成圆
import TechIcon from './TechIcon.vue'

export default {
  name: 'LiquidDrag',
  components: { TechIcon },
  props: {
    icon: { type: String, default: 'scan' },
    originIcon: { type: String, default: 'pin' },
    w: { type: Number, default: 320 },
    h: { type: Number, default: 160 },
    threshold: { type: Number, default: 120 }, // px 断裂阈值
  },
  emits: ['drag', 'break', 'reset'],
  data() {
    return {
      cid: 'liquid-' + Math.random().toString(36).slice(2, 8),
      ox: 40, oy: 80,           // 原点
      hx: 40, hy: 80,           // 手柄当前位置
      sx: 40, sy: 80,           // 手柄起始
      dragging: false,
      broken: false,
      ctx: null,
      raf: null,
    }
  },
  computed: {
    originStyle() {
      return `left:${this.ox - 20}px;top:${this.oy - 20}px;`
    },
    handleStyle() {
      const op = this.broken ? 0.55 : 1
      return `left:${this.hx - 22}px;top:${this.hy - 22}px;opacity:${op};`
    },
  },
  mounted() {
    this.$nextTick(() => {
      this.ctx = uni.createCanvasContext(this.cid, this)
      this.draw()
    })
  },
  beforeUnmount() {
    if (this.raf) cancelAnimationFrame(this.raf)
  },
  methods: {
    dist() { return Math.hypot(this.hx - this.ox, this.hy - this.oy) },
    onStart(e) {
      const t = e.touches[0]
      this.dragging = true
      this.broken = false
      this.sx = this.hx; this.sy = this.hy
      this._startClientX = t.clientX
      this._startClientY = t.clientY
      this.draw()
    },
    onMove(e) {
      if (!this.dragging) return
      const t = e.touches[0]
      // 相对位移驱动（起点手柄位 + 手指位移）
      this.hx = this.sx + (t.clientX - this._startClientX)
      this.hy = this.sy + (t.clientY - this._startClientY)
      // 限制在画布内
      this.hx = Math.max(24, Math.min(this.w - 24, this.hx))
      this.hy = Math.max(24, Math.min(this.h - 24, this.hy))
      const d = this.dist()
      if (d > this.threshold && !this.broken) {
        this.broken = true
        this.$emit('break', { x: this.hx, y: this.hy })
      }
      this.$emit('drag', { x: this.hx, y: this.hy, dist: d, broken: this.broken })
      this.draw()
    },
    onEnd() {
      if (!this.dragging) return
      this.dragging = false
      // 回弹动画：断开→弹回起始位；未断开→吸回原点
      const fromX = this.hx, fromY = this.hy
      const toX = this.broken ? this.sx : this.ox
      const toY = this.broken ? this.sy : this.oy
      const t0 = Date.now(), dur = 380
      const step = () => {
        const p = Math.min(1, (Date.now() - t0) / dur)
        const e = 1 - Math.pow(1 - p, 3) // easeOutCubic（回弹感）
        this.hx = fromX + (toX - fromX) * e
        this.hy = fromY + (toY - fromY) * e
        this.draw()
        if (p < 1) this.raf = (typeof requestAnimationFrame === 'function' ? requestAnimationFrame(step) : setTimeout(step, 16))
        else {
          if (!this.broken) { this.hx = this.ox; this.hy = this.oy }
          this.broken = false
          this.draw()
          this.$emit('reset')
        }
      }
      step()
    },
    draw() {
      const ctx = this.ctx
      if (!ctx) return
      ctx.clearRect(0, 0, this.w, this.h)
      const d = this.dist()
      const maxD = this.threshold
      // 颈部半径随距离收缩
      const t = Math.min(1, d / maxD)
      const rO = 20 * (1 - t * 0.7)
      const rH = 22
      const color = this.broken ? 'rgba(229,72,77,0.55)' : 'rgba(45,107,196,0.55)'

      // 液态桥（两圆外公切线 + 贝塞尔）——未断裂且距离>0 才画
      if (!this.broken && d > 2) {
        const ang = Math.atan2(this.hy - this.oy, this.hx - this.ox)
        const perp = ang + Math.PI / 2
        const o1x = this.ox + Math.cos(perp) * rO, o1y = this.oy + Math.sin(perp) * rO
        const o2x = this.ox - Math.cos(perp) * rO, o2y = this.oy - Math.sin(perp) * rO
        const h1x = this.hx + Math.cos(perp) * rH, h1y = this.hy + Math.sin(perp) * rH
        const h2x = this.hx - Math.cos(perp) * rH, h2y = this.hy - Math.sin(perp) * rH
        const mx = (this.ox + this.hx) / 2, my = (this.oy + this.hy) / 2
        // 颈部内凹控制点
        const neck = Math.min(rO, rH) * (1 - t) * 0.9
        const c1x = mx + Math.cos(perp) * neck, c1y = my + Math.sin(perp) * neck
        const c2x = mx - Math.cos(perp) * neck, c2y = my - Math.sin(perp) * neck
        ctx.setFillStyle(color)
        ctx.beginPath()
        ctx.moveTo(o1x, o1y)
        ctx.quadraticCurveTo(c1x, c1y, h1x, h1y)
        ctx.lineTo(h2x, h2y)
        ctx.quadraticCurveTo(c2x, c2y, o2x, o2y)
        ctx.closePath()
        ctx.fill()
      }
      // 原点圆
      ctx.setFillStyle(this.broken ? 'rgba(150,160,180,0.5)' : 'rgba(45,107,196,0.75)')
      ctx.beginPath()
      ctx.arc(this.ox, this.oy, this.broken ? 14 : rO + 2, 0, Math.PI * 2)
      ctx.fill()
      ctx.draw()
    },
  },
}
</script>

<style scoped>
.liquid-wrap { position: relative; width: 100%; height: 320rpx; }
.liquid-canvas { position: absolute; left: 0; top: 0; }
.liquid-dot {
  position: absolute;
  display: flex; align-items: center; justify-content: center;
  border-radius: 50%;
  color: #fff;
}
.liquid-dot.origin {
  width: 80rpx; height: 80rpx;
  background: rgba(45, 107, 196, 0.18);
  border: 2rpx solid rgba(45, 107, 196, 0.5);
}
.liquid-dot.handle {
  width: 88rpx; height: 88rpx;
  background: linear-gradient(135deg, #2D6BC4, #1F4E9C);
  box-shadow: 0 8rpx 24rpx rgba(31, 78, 156, 0.4);
}
.liquid-tip {
  position: absolute; right: 16rpx; top: 12rpx;
  font-size: 22rpx; color: #e5484d;
}
</style>
