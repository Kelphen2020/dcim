<template>
  <view class="page">
    <GlassNavBar title="变更审批" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>
    <scroll-view scroll-x class="chips">
      <view v-for="c in chips" :key="c.k" class="chip" :class="{ active: cur === c.k }" @tap="setChip(c.k)">{{ c.t }}</view>
    </scroll-view>

    <CylList :items="items" :scrollY="scrollY" :itemH="100" @pick="(ch) => goDetail(ch)">
      <template #empty>
        <view v-if="loading" class="empty-tip">加载中…</view>
        <view v-else class="empty-tip">没有匹配的变更单</view>
      </template>
      <template #item="{ row: ch }">
        <view class="card c-item">
          <view class="row">
            <text class="code mono">{{ ch.code }}</text>
            <text class="grow"></text>
            <text class="tag" :class="'cs' + ch.status">{{ stText(ch.status) }}</text>
          </view>
          <view class="title">{{ ch.title }}</view>
          <view class="row foot">
            <text class="tag risk" :class="'r' + ch.risk">{{ riskText(ch.risk) }}</text>
            <text class="grow"></text>
            <text class="muted" v-if="ch.window_start">窗口 {{ dt(ch.window_start) }}</text>
          </view>
          <view class="muted time">申请 {{ timeAgo(ch.created_at) }}</view>
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
import { get } from '../../api.js'
import { requireAuth, timeAgo } from '../../util.js'

export const CHG_STATUS = { 1: '草稿', 2: '待审批', 3: '已批准', 4: '已驳回', 5: '执行中', 6: '已完成', 7: '已回滚' }

export default {
  components: { GlassNavBar, GlassTabBar, CylList },
  data() {
    return {
      scrollY: 0,
      navH: 44,
      cur: 'pending', page: 1, size: 20, total: 0, items: [], loading: false,
      chips: [
        { k: 'pending', t: '待我审批' },
        { k: 'approved', t: '待执行' },
        { k: 'running', t: '执行中' },
        { k: 'all', t: '全部' },
        { k: 'done', t: '已完成' },
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
    this.reload()
  },
  onPullDownRefresh() { this.reload().finally(() => uni.stopPullDownRefresh()) },
  methods: {
    timeAgo,
    stText: (s) => CHG_STATUS[s] || '',
    riskText: (r) => ({ 1: '低风险', 2: '中风险', 3: '高风险' }[r] || ''),
    dt(t) { return t ? String(t).replace('T', ' ').slice(5, 16) : '' },
    setChip(k) { this.cur = k; this.reload() },
    async reload() { this.page = 1; await this.fetch(true) },
    async loadMore() { this.page += 1; await this.fetch(false) },
    async fetch(reset) {
      this.loading = true
      try {
        const p = { page: this.page, size: this.size }
        if (this.cur === 'pending') p.status = 2
        else if (this.cur === 'approved') p.status = 3
        else if (this.cur === 'running') p.status = 5
        else if (this.cur === 'done') p.status = 6
        const d = await get('/changes', p)
        this.total = d.total || 0
        this.items = reset ? (d.items || []) : this.items.concat(d.items || [])
      } catch (e) { /* toast */ } finally { this.loading = false }
    },
    goDetail(ch) {
      uni.setStorageSync('dcim_chg_cur', ch)
      uni.navigateTo({ url: '/pages/change/detail?id=' + ch.id })
    },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 40rpx; }
.chips { white-space: nowrap; margin-bottom: 16rpx; }
.chip { display: inline-block; padding: 12rpx 30rpx; margin-right: 16rpx; background: #fff; border-radius: 30rpx; font-size: 26rpx; color: #4b5563; border: 1rpx solid #e5eaf3; }
.chip.active { background: #1f4e9c; color: #fff; border-color: #1f4e9c; }
.code { font-size: 22rpx; color: #8a94a6; }
.mono { font-family: monospace; }
.title { font-size: 30rpx; font-weight: 600; margin: 10rpx 0; line-height: 1.4; }
.foot { margin-bottom: 6rpx; }
.tag { font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 8rpx; }
.cs1 { background: #f3f4f6; color: #8a94a6; }
.cs2 { background: #fff1f0; color: #e5484d; }
.cs3 { background: #eef4ff; color: #1f4e9c; }
.cs4 { background: #f3f4f6; color: #8a94a6; }
.cs5 { background: #fff7e6; color: #d48806; }
.cs6 { background: #eef9f0; color: #2fa84f; }
.cs7 { background: #fff7e6; color: #d48806; }
.risk { color: #fff; }
.r1 { background: #2fa84f; }
.r2 { background: #f5a623; }
.r3 { background: #e5484d; }
.time { font-size: 22rpx; }
.load-more { text-align: center; color: #1f4e9c; padding: 24rpx; font-size: 26rpx; }
.page { padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 170rpx) !important; }
</style>
