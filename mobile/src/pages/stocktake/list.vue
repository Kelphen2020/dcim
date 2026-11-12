<template>
  <view class="page">
    <GlassNavBar title="资产盘点" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>
    <view class="sec-title">盘点任务</view>
    <CylList :items="tasks" :scrollY="scrollY" :itemH="110" @pick="(t) => openTask(t)">
      <template #empty>
        <view class="empty-tip">暂无盘点任务</view>
      </template>
      <template #item="{ row: t }">
        <view class="card t-item">
          <view class="row">
            <text class="t-name grow">{{ t.name }}</text>
            <text class="tag" :class="'ts' + t.status">{{ stText(t.status) }}</text>
          </view>
          <view class="muted mono">{{ t.code }} · {{ String(t.created_at).slice(0, 10) }}</view>
          <view class="prog-row">
            <view class="prog-bg"><view class="prog-fg" :style="{ width: pct(t) + '%' }"></view></view>
            <text class="prog-t">{{ t.checked_count }}/{{ t.total_count }}</text>
          </view>
          <view class="muted diff-t" v-if="t.diff_count > 0">差异 {{ t.diff_count }} 项</view>
        </view>
      </template>
    </CylList>
    <GlassTabBar current="me" :scrollY="scrollY" />
  </view>
</template>

<script>
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import CylList from '../../components/CylList.vue'
import { get } from '../../api.js'
import { requireAuth } from '../../util.js'

export default {
  components: { GlassNavBar, GlassTabBar, CylList },
  data() {
    return { scrollY: 0, navH: 44, tasks: [] }
  },
  onLoad() {
    try { this.navH = (uni.getSystemInfoSync().statusBarHeight || 0) + 44 } catch (e) {}
  },
  onPageScroll(e) { this.scrollY = e.scrollTop },
  onShow() {
    if (!requireAuth()) return
    this.load()
  },
  onPullDownRefresh() { this.load().finally(() => uni.stopPullDownRefresh()) },
  methods: {
    stText(s) { return { 0: '草稿', 1: '盘点中', 2: '已完成' }[s] || '' },
    pct(t) { return t.total_count ? Math.round((t.checked_count / t.total_count) * 100) : 0 },
    async load() {
      try {
        const d = await get('/stocktakes', { page: 1, size: 50 })
        this.tasks = d.items || []
      } catch (e) { /* toast */ }
    },
    openTask(t) {
      uni.setStorageSync('dcim_stocktake_cur', t)
      uni.navigateTo({ url: '/pages/stocktake/exec?id=' + t.id })
    },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 40rpx; }
.sec-title { font-size: 30rpx; font-weight: 700; margin: 8rpx 4rpx 14rpx; }
.t-name { font-size: 30rpx; font-weight: 600; }
.mono { font-family: monospace; margin: 6rpx 0; }
.tag { font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 8rpx; }
.ts0 { background: #f3f4f6; color: #8a94a6; }
.ts1 { background: #eef4ff; color: #1f4e9c; }
.ts2 { background: #eef9f0; color: #2fa84f; }
.prog-row { display: flex; align-items: center; margin-top: 12rpx; }
.prog-bg { flex: 1; height: 14rpx; background: #eef2f8; border-radius: 8rpx; overflow: hidden; }
.prog-fg { height: 100%; background: linear-gradient(90deg, #1f4e9c, #2d6bc4); border-radius: 8rpx; }
.prog-t { margin-left: 16rpx; font-size: 24rpx; color: #4b5563; }
.page { padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 170rpx) !important; }
</style>
