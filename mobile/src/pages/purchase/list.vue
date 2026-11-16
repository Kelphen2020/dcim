<template>
  <view class="page">
    <GlassNavBar title="采购审批" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>
    <scroll-view scroll-x class="chips">
      <view v-for="c in chips" :key="c.k" class="chip" :class="{ active: cur === c.k }" @tap="setChip(c.k)">{{ c.t }}</view>
    </scroll-view>

    <CylList :items="items" :scrollY="scrollY" :itemH="110" @pick="(row) => goDetail(row)">
      <template #empty>
        <view v-if="loading" class="empty-tip">加载中…</view>
        <view v-else class="empty-tip">没有匹配的采购申请</view>
      </template>
      <template #item="{ row: pr }">
        <view class="card c-item">
          <view class="row">
            <text class="code mono">{{ pr.code }}</text>
            <text class="grow"></text>
            <text class="tag" :class="'ps' + pr.status">{{ stText(pr.status) }}</text>
          </view>
          <view class="title">{{ pr.spare_name }} <text class="qty">×{{ pr.qty }}{{ pr.spare_unit || '' }}</text></view>
          <view class="reason muted">{{ pr.reason }}</view>
          <view class="row foot">
            <text class="muted">{{ pr.applicant }}</text>
            <text class="grow"></text>
            <text class="muted" v-if="pr.budget_yuan > 0">预算 ¥{{ pr.budget_yuan }}</text>
            <text class="muted time">{{ timeAgo(pr.created_at) }}</text>
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
import { get } from '../../api.js'
import { requireAuth, timeAgo } from '../../util.js'

export const PR_STATUS = { 1: '待审批', 2: '已批准', 3: '已驳回', 4: '已入库' }

export default {
  components: { GlassNavBar, GlassTabBar, CylList },
  data() {
    return {
      scrollY: 0, navH: 44,
      cur: '1', page: 1, size: 20, total: 0, items: [], loading: false,
      chips: [
        { k: '1', t: '待审批' },
        { k: '2', t: '已批准' },
        { k: '3', t: '已驳回' },
        { k: '4', t: '已入库' },
        { k: 'all', t: '全部' },
      ],
    }
  },
  computed: { hasMore() { return this.items.length < this.total } },
  onLoad(q) {
    try { this.navH = (uni.getSystemInfoSync().statusBarHeight || 0) + 44 } catch (e) {}
    if (q && q.status) this.cur = String(q.status)
  },
  onPageScroll(e) { this.scrollY = e.scrollTop },
  onShow() {
    if (!requireAuth()) return
    this.reload()
  },
  onPullDownRefresh() { this.reload().finally(() => uni.stopPullDownRefresh()) },
  methods: {
    timeAgo,
    stText: (s) => PR_STATUS[s] || '',
    setChip(k) { this.cur = k; this.reload() },
    async reload() { this.page = 1; await this.fetch(true) },
    async loadMore() { this.page += 1; await this.fetch(false) },
    async fetch(reset) {
      this.loading = true
      try {
        const p = { page: this.page, size: this.size }
        if (this.cur !== 'all') p.status = this.cur
        const d = await get('/purchase-requests', p)
        this.total = d.total || 0
        this.items = reset ? (d.list || []) : this.items.concat(d.list || [])
      } catch (e) { /* toast */ } finally { this.loading = false }
    },
    goDetail(pr) {
      uni.setStorageSync('dcim_pr_cur', pr)
      uni.navigateTo({ url: '/pages/purchase/detail?id=' + pr.id, fail: () => {} })
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
.title { font-size: 30rpx; font-weight: 600; margin: 10rpx 0 6rpx; line-height: 1.4; }
.qty { font-size: 26rpx; color: #1f4e9c; font-weight: 700; }
.reason { font-size: 24rpx; line-height: 1.45; overflow: hidden; text-overflow: ellipsis; display: -webkit-box; -webkit-line-clamp: 1; -webkit-box-orient: vertical; margin-bottom: 8rpx; }
.tag { font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 8rpx; }
.ps1 { background: #fff1f0; color: #e5484d; }
.ps2 { background: #eef4ff; color: #1f4e9c; }
.ps3 { background: #f3f4f6; color: #8a94a6; }
.ps4 { background: #eef9f0; color: #2fa84f; }
.foot { margin-top: 4rpx; }
.time { font-size: 22rpx; margin-left: 12rpx; }
.load-more { text-align: center; color: #1f4e9c; padding: 24rpx; font-size: 26rpx; }
.page { padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 170rpx) !important; }
</style>
