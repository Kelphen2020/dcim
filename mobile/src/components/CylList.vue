<template>
  <view class="cyl-viewport">
    <slot v-if="!items.length" name="empty"></slot>
    <view class="cyl-item" v-for="(it, i) in items" :key="keyOf(it, i)" :style="styleAt(i)" @tap="$emit('pick', it)">
      <slot name="item" :row="it" :idx="i"></slot>
    </view>
  </view>
</template>

<script>
// 通用圆柱卷收列表（M5 设计效果④的可复用版）
// 用法：<CylList :items="rows" :scrollY="scrollY" @pick="go"><template #item="{row}">…</template></CylList>
import { cylStyle } from '../fx.js'

export default {
  name: 'CylList',
  props: {
    items: { type: Array, default: () => [] },
    scrollY: { type: Number, default: 0 },
    itemH: { type: Number, default: 82 },     // 单行估高 px
    listTop: { type: Number, default: 0 },     // 列表顶部页面偏移（0=挂载后自动测量）
    keyField: { type: String, default: 'id' },
    winH: { type: Number, default: 0 },
  },
  emits: ['pick'],
  data() {
    return { measuredTop: null, win: 0, rects: [] }
  },
  mounted() {
    try {
      const sys = uni.getSystemInfoSync()
      this.win = sys.windowHeight || 700
    } catch (e) { this.win = 700 }
    this.$nextTick(() => this.measureItems())
  },
  watch: {
    items() { this.$nextTick(() => this.measureItems()) },
  },
  methods: {
    // 逐项实测真实位置与高度（卡片高度可变，固定 itemH 估算会逐行累计偏移导致特效错位）
    measureItems() {
      const q = uni.createSelectorQuery().in(this)
      q.selectAll('.cyl-item').boundingClientRect((rs) => {
        if (rs && rs.length) {
          const sy = this.scrollY
          this.rects = rs.map((r) => ({ top: r.top + sy, h: r.height }))
        }
        if (!this.listTop && this.measuredTop == null) {
          const q2 = uni.createSelectorQuery().in(this)
          q2.select('.cyl-viewport').boundingClientRect((rect) => {
            if (rect && rect.top != null) this.measuredTop = rect.top + this.scrollY
          }).exec()
        }
      }).exec()
    },
    keyOf(it, i) {
      const k = it && it[this.keyField]
      return k != null ? k : i
    },
    styleAt(i) {
      const r = this.rects[i]
      if (r) return cylStyle(r.top, r.h || this.itemH, this.scrollY, this.winH || this.win || 700)
      const top = this.listTop || this.measuredTop || 300
      return cylStyle(top + i * this.itemH, this.itemH, this.scrollY, this.winH || this.win || 700)
    },
  },
}
</script>

<style scoped>
.cyl-viewport { perspective: 900px; }
.cyl-item { transform-style: preserve-3d; margin-bottom: 16rpx; transition: opacity 0.1s linear; }
</style>
