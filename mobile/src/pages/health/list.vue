<template>
  <view class="page">
    <GlassNavBar title="资产健康" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>

    <!-- 总览条 -->
    <view class="card sum-card" v-if="sum.total">
      <view class="sum-cell">
        <view class="sum-num" :class="scoreCls(sum.avg_score)">{{ sum.avg_score }}</view>
        <view class="muted">平均健康分</view>
      </view>
      <view class="sum-cell">
        <view class="sum-num">{{ sum.total }}</view>
        <view class="muted">已评估设备</view>
      </view>
      <view class="sum-cell">
        <view class="sum-num" :class="{ orange: sum.open_work_orders > 0 }">{{ sum.open_work_orders }}</view>
        <view class="muted">维护工单在办</view>
      </view>
    </view>

    <scroll-view scroll-x class="chips">
      <view v-for="c in chips" :key="c.k" class="chip" :class="{ active: cur === c.k }" @tap="setChip(c.k)">
        {{ c.t }}<text v-if="c.k !== 'all' && gradeN(c.k)" class="chip-n">{{ gradeN(c.k) }}</text>
      </view>
    </scroll-view>

    <view class="search-bar">
      <TechIcon name="search" color="#8a94a6" :size="26" />
      <input class="search-in" v-model="kw" placeholder="搜索名称 / 编码" confirm-type="search" @confirm="reload" />
      <text v-if="kw" class="search-x" @tap="clearKw">×</text>
    </view>

    <CylList :items="items" :scrollY="scrollY" :itemH="110" @pick="(row) => goDetail(row)">
      <template #empty>
        <view v-if="loading" class="empty-tip">加载中…</view>
        <view v-else class="empty-tip">没有匹配的设备</view>
      </template>
      <template #item="{ row: a }">
        <view class="card c-item">
          <view class="h-row">
            <view class="score-ring" :class="gradeCls(a.grade)">{{ a.score }}</view>
            <view class="grow h-main">
              <view class="title">{{ a.name }}</view>
              <view class="muted mono code">{{ a.code }}</view>
            </view>
            <view class="h-right">
              <text class="tag" :class="'g-' + a.grade">{{ gradeText(a.grade) }}</text>
              <text v-if="a.open_work_order_id" class="wo-badge">工单在办</text>
            </view>
          </view>
          <view class="row foot">
            <text class="muted">{{ catText(a.category) }}</text>
            <text class="grow"></text>
            <text class="muted time">评估 {{ timeAgo(a.computed_at) }}</text>
          </view>
        </view>
      </template>
    </CylList>
    <view v-if="hasMore && items.length" class="load-more" @tap="loadMore">加载更多</view>
    <GlassTabBar current="me" :scrollY="scrollY" />
  </view>
</template>

<script>
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import CylList from '../../components/CylList.vue'
import TechIcon from '../../components/TechIcon.vue'
import { get } from '../../api.js'
import { requireAuth, timeAgo } from '../../util.js'

export const GRADE_TEXT = { healthy: '健康', sub_healthy: '亚健康', attention: '关注', risk: '风险' }
export const CAT_TEXT = { server: '服务器', network: '网络设备', power: '配电', cooling: '制冷', storage: '存储', security: '安防', other: '其他' }

export default {
  components: { GlassNavBar, GlassTabBar, CylList, TechIcon },
  data() {
    return {
      scrollY: 0, navH: 44,
      cur: 'all', kw: '', page: 1, size: 20, total: 0, items: [], loading: false,
      sum: { avg_score: 0, total: 0, grades: {}, open_work_orders: 0 },
      chips: [
        { k: 'all', t: '全部' },
        { k: 'risk', t: '风险' },
        { k: 'attention', t: '关注' },
        { k: 'sub_healthy', t: '亚健康' },
        { k: 'healthy', t: '健康' },
      ],
    }
  },
  computed: { hasMore() { return this.items.length < this.total } },
  onLoad() {
    try { this.navH = (uni.getSystemInfoSync().statusBarHeight || 0) + 44 } catch (e) {}
  },
  onPageScroll(e) { this.scrollY = e.scrollTop },
  onShow() {
    if (!requireAuth()) return
    this.loadSum()
    this.reload()
  },
  onPullDownRefresh() {
    Promise.all([this.loadSum(), this.reload()]).finally(() => uni.stopPullDownRefresh())
  },
  methods: {
    timeAgo,
    gradeText: (g) => GRADE_TEXT[g] || g,
    catText: (c) => CAT_TEXT[c] || c || '设备',
    gradeCls(g) { return { healthy: 'ok', sub_healthy: 'warn', attention: 'warn2', risk: 'bad' }[g] || '' },
    scoreCls(s) { return s >= 85 ? 'green' : s >= 55 ? 'orange' : 'red' },
    gradeN(k) { return (this.sum.grades || {})[k] || 0 },
    setChip(k) { this.cur = k; this.reload() },
    clearKw() { this.kw = ''; this.reload() },
    async loadSum() {
      try { this.sum = await get('/asset-health/summary', {}, { silent: true }) } catch (e) {}
    },
    async reload() { this.page = 1; await this.fetch(true) },
    async loadMore() { this.page += 1; await this.fetch(false) },
    async fetch(reset) {
      this.loading = true
      try {
        const p = { page: this.page, size: this.size }
        if (this.cur !== 'all') p.grade = this.cur
        if (this.kw.trim()) p.keyword = this.kw.trim()
        const d = await get('/asset-health', p)
        this.total = d.total || 0
        this.items = reset ? (d.items || []) : this.items.concat(d.items || [])
      } catch (e) { /* toast */ } finally { this.loading = false }
    },
    goDetail(a) {
      uni.setStorageSync('dcim_health_cur', a)
      uni.navigateTo({ url: '/pages/health/detail?asset_id=' + a.asset_id })
    },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 40rpx; }
.sum-card { display: flex; padding: 24rpx 10rpx; margin-bottom: 16rpx; }
.sum-cell { flex: 1; text-align: center; }
.sum-num { font-size: 44rpx; font-weight: 800; color: #1f2937; }
.sum-num.green { color: #2fa84f; }
.sum-num.orange { color: #f5a623; }
.sum-num.red { color: #e5484d; }
.chips { white-space: nowrap; margin-bottom: 14rpx; }
.chip { display: inline-block; padding: 12rpx 28rpx; margin-right: 14rpx; background: #fff; border-radius: 30rpx; font-size: 26rpx; color: #4b5563; border: 1rpx solid #e5eaf3; }
.chip.active { background: #1f4e9c; color: #fff; border-color: #1f4e9c; }
.chip-n { margin-left: 8rpx; font-size: 22rpx; opacity: 0.75; }
.search-bar { display: flex; align-items: center; gap: 10rpx; background: #fff; border: 1rpx solid #e5eaf3; border-radius: 16rpx; padding: 10rpx 20rpx; margin-bottom: 16rpx; }
.search-in { flex: 1; font-size: 26rpx; }
.search-x { color: #8a94a6; font-size: 34rpx; padding: 0 8rpx; }
.h-row { display: flex; align-items: center; gap: 18rpx; }
.h-main { min-width: 0; }
.score-ring { width: 84rpx; height: 84rpx; border-radius: 50%; display: flex; align-items: center; justify-content: center; font-size: 32rpx; font-weight: 800; color: #fff; flex-shrink: 0; }
.score-ring.ok { background: #2fa84f; }
.score-ring.warn { background: #f5a623; }
.score-ring.warn2 { background: #ff7a45; }
.score-ring.bad { background: #e5484d; }
.title { font-size: 30rpx; font-weight: 600; line-height: 1.35; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.code { font-size: 22rpx; color: #8a94a6; }
.mono { font-family: monospace; }
.h-right { display: flex; flex-direction: column; align-items: flex-end; gap: 8rpx; flex-shrink: 0; }
.tag { font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 8rpx; }
.g-healthy { background: #eef9f0; color: #2fa84f; }
.g-sub_healthy { background: #fff7e6; color: #d48806; }
.g-attention { background: #fff2e8; color: #ff7a45; }
.g-risk { background: #fff1f0; color: #e5484d; }
.wo-badge { font-size: 20rpx; color: #1f4e9c; background: #eef4ff; padding: 2rpx 12rpx; border-radius: 8rpx; }
.foot { margin-top: 12rpx; }
.time { font-size: 22rpx; }
.load-more { text-align: center; color: #1f4e9c; padding: 24rpx; font-size: 26rpx; }
.page { padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 170rpx) !important; }
</style>
