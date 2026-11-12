<template>
  <view class="page">
    <GlassNavBar title="巡检报告" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>
    <scroll-view scroll-x class="chips">
      <view v-for="c in chips" :key="c.k" class="chip" :class="{ active: cur === c.k }" @tap="setChip(c.k)">{{ c.t }}</view>
    </scroll-view>

    <CylList :items="items" :scrollY="scrollY" :itemH="92" @pick="(r) => goDetail(r.id)">
      <template #empty>
        <view v-if="loading" class="empty-tip">加载中…</view>
        <view v-else class="empty-tip">没有匹配的巡检报告</view>
      </template>
      <template #item="{ row: r }">
        <view class="card r-item">
          <view class="row">
            <text class="tag" :class="'rs' + r.status">{{ stText(r.status) }}</text>
            <text class="grow r-name">{{ r.plan_name || r.target_name || '巡检记录 #' + r.id }}</text>
            <view class="signed" v-if="r.reviewer_id"><TechIcon name="pen" color="#1F4E9C" :size="24" /> 已签发</view>
          </view>
          <view class="muted line">
            <text v-if="r.report_code" class="mono">{{ r.report_code }} · </text>
            {{ r.inspector_name || '-' }} · {{ dt(r.inspected_at) }}
            <text v-if="r.source === 'scan'"> · 扫码</text>
          </view>
          <view class="abn" v-if="r.status === 2 && r.abnormal">{{ r.abnormal }}</view>
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
import TechIcon from '../../components/TechIcon.vue'
import CylList from '../../components/CylList.vue'
import { get } from '../../api.js'
import { requireAuth } from '../../util.js'

export default {
  components: { TechIcon , GlassNavBar, GlassTabBar, CylList },
  data() {
    return {
      scrollY: 0,
      navH: 44,
      cur: 'all', page: 1, size: 20, total: 0, items: [], loading: false,
      chips: [
        { k: 'all', t: '全部' },
        { k: 'signed', t: '已签发' },
        { k: 'abnormal', t: '异常' },
        { k: 'scan', t: '扫码巡检' },
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
    stText(s) { return { 1: '正常', 2: '异常', 3: '逾期' }[s] || '' },
    dt(t) { return t ? String(t).replace('T', ' ').slice(0, 16) : '' },
    setChip(k) { this.cur = k; this.reload() },
    async reload() { this.page = 1; await this.fetch(true) },
    async loadMore() { this.page += 1; await this.fetch(false) },
    async fetch(reset) {
      this.loading = true
      try {
        const p = { page: this.page, size: this.size }
        if (this.cur === 'signed') p.signed = 1
        if (this.cur === 'abnormal') p.status = 2
        if (this.cur === 'scan') p.source = 'scan'
        const d = await get('/inspection-reports', p)
        this.total = d.total || 0
        this.items = reset ? (d.items || []) : this.items.concat(d.items || [])
      } catch (e) { /* toast */ } finally { this.loading = false }
    },
    goDetail(id) { uni.navigateTo({ url: '/pages/report/detail?id=' + id }) },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 40rpx; }
.chips { white-space: nowrap; margin-bottom: 16rpx; }
.chip { display: inline-block; padding: 12rpx 30rpx; margin-right: 16rpx; background: #fff; border-radius: 30rpx; font-size: 26rpx; color: #4b5563; border: 1rpx solid #e5eaf3; }
.chip.active { background: #1f4e9c; color: #fff; border-color: #1f4e9c; }
.r-name { font-size: 29rpx; font-weight: 600; margin: 0 12rpx; }
.tag { font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 8rpx; flex-shrink: 0; }
.rs1 { background: #eef9f0; color: #2fa84f; }
.rs2 { background: #fff1f0; color: #e5484d; }
.rs3 { background: #f3f4f6; color: #8a94a6; }
.signed { font-size: 22rpx; color: #1f4e9c; flex-shrink: 0; }
.line { margin-top: 10rpx; font-size: 24rpx; }
.mono { font-family: monospace; color: #1f4e9c; }
.abn { margin-top: 10rpx; font-size: 24rpx; color: #e5484d; background: #fff1f0; border-radius: 8rpx; padding: 10rpx 14rpx; }
.load-more { text-align: center; color: #1f4e9c; padding: 24rpx; font-size: 26rpx; }
.page { padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 170rpx) !important; }
</style>
