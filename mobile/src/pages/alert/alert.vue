<template>
  <view class="page" :style="{ paddingTop: navH + 'px', paddingBottom: 'calc(env(safe-area-inset-bottom, 0px) + 150rpx)' }">
    <GlassNavBar title="告警中心" :scrollY="scrollY" />
    <!-- 快捷筛选 -->
    <scroll-view scroll-x class="chips">
      <view v-for="c in chips" :key="c.k" class="chip" :class="{ active: cur === c.k }" @tap="setChip(c.k)">{{ c.t }}</view>
    </scroll-view>

    <CylList :items="items" :scrollY="scrollY" :itemH="96" @pick="(a) => goDetail(a.id)">
      <template #empty>
        <view v-if="loading" class="empty-tip">加载中…</view>
        <view v-else class="empty-tip">没有匹配的告警</view>
      </template>
      <template #item="{ row: a }">
        <view class="alert-item">
          <view class="sev-bar" :style="{ background: sevColor(a.severity) }"></view>
          <view class="grow ab">
            <view class="row top">
              <text class="sev-tag" :style="{ background: sevColor(a.severity) }">{{ sevText(a.severity) }}</text>
              <text class="st-tag" :class="'st' + a.status">{{ stText(a.status) }}</text>
              <text v-if="a.storm_id" class="storm">风暴</text>
              <text v-if="a.suppressed" class="sup">抑制</text>
              <text class="grow"></text>
              <text class="muted">{{ timeAgo(a.triggered_at) }}</text>
            </view>
            <view class="msg">{{ a.message }}</view>
            <view class="muted" v-if="a.point_name || a.wo_code">
              <text v-if="a.point_name">{{ a.point_name }}</text>
              <text v-if="a.wo_code"> · {{ a.wo_code }}</text>
            </view>
          </view>
        </view>
      </template>
    </CylList>

    <view v-if="hasMore && items.length" class="load-more" @tap="loadMore">加载更多</view>
    <GlassTabBar current="alert" :scrollY="scrollY" />
  </view>
</template>

<script>
import { get } from '../../api.js'
import { requireAuth, timeAgo, SEV_TEXT } from '../../util.js'
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import CylList from '../../components/CylList.vue'

export default {
  components: { GlassNavBar, GlassTabBar, CylList },
  data() {
    return {
      scrollY: 0,
      navH: 44,
      cur: 'active',
      page: 1,
      size: 20,
      total: 0,
      items: [],
      loading: false,
      chips: [
        { k: 'all', t: '全部' },
        { k: 'active', t: '待处理' },
        { k: 'ack', t: '已确认' },
        { k: 'sev3', t: '严重' },
        { k: 'sev2', t: '警告' },
        { k: 'closed', t: '已结束' },
      ],
    }
  },
  computed: {
    hasMore() { return this.items.length < this.total },
  },
  onShow() {
    if (!requireAuth()) return
    // #ifdef H5
    // hash 路由复用页面实例时 onLoad 不重触发，从 URL 兜底解析分级深链
    try {
      const qs = location.hash.split('?')[1] || ''
      const sv = new URLSearchParams(qs).get('sev')
      if (sv && ['2', '3'].includes(sv)) {
        const sig = 'sev' + sv
        if (sig !== this._deepSig) { this._deepSig = sig; this.cur = sig }
      }
    } catch (e) { /* ignore */ }
    // #endif
    this.reload()
    // M6-D: 30s 自动轮询（告警盯屏），页面隐藏时暂停
    this.stopPoll()
    this._poll = setInterval(() => {
      // #ifdef H5
      if (document.hidden) return
      // #endif
      this.reload()
    }, 30000)
  },
  onHide() { this.stopPoll() },
  onUnload() { this.stopPoll() },
  onLoad(q) {
    try { this.navH = (uni.getSystemInfoSync().statusBarHeight || 0) + 44 } catch (e) {}
    // M6-C：工作台告警分级穿透（?sev=3 严重 / ?sev=2 警告）
    if (q && q.sev && ['2', '3'].includes(String(q.sev))) this.cur = 'sev' + q.sev
  },
  onPageScroll(e) { this.scrollY = e.scrollTop },
  onPullDownRefresh() {
    this.reload().finally(() => uni.stopPullDownRefresh())
  },
  methods: {
    timeAgo,
    sevText: (s) => SEV_TEXT[s] || '',
    sevColor(s) { return { 1: '#3B82F6', 2: '#F5A623', 3: '#E5484D' }[s] || '#8a94a6' },
    stText(s) { return { 0: '待处理', 1: '已挂起', 2: '已确认', 3: '已结束' }[s] || '' },
    setChip(k) { this.cur = k; this.reload() },
    params() {
      const p = { page: this.page, size: this.size }
      if (this.cur === 'active') p.status = 0
      else if (this.cur === 'ack') p.status = 2
      else if (this.cur === 'closed') p.status = 3
      else if (this.cur === 'sev3') p.severity = 3
      else if (this.cur === 'sev2') p.severity = 2
      return p
    },
    async reload() {
      this.page = 1
      await this.fetch(true)
    },
    async loadMore() {
      this.page += 1
      await this.fetch(false)
    },
    async fetch(reset) {
      this.loading = true
      try {
        const d = await get('/alerts', this.params())
        this.total = d.total || 0
        this.items = reset ? (d.items || []) : this.items.concat(d.items || [])
      } catch (e) { /* ignore */ } finally {
        this.loading = false
      }
    },
    goDetail(a) {
      uni.setStorageSync('dcim_alert_cur', a)
      uni.navigateTo({ url: '/pages/alert/detail?id=' + a.id })
    },
    stopPoll() {
      if (this._poll) { clearInterval(this._poll); this._poll = null }
    },
  },
}
</script>

<style scoped>
.page { padding: 0 24rpx 40rpx; }
.chips { white-space: nowrap; padding: 20rpx 0; }
.chip {
  display: inline-block; padding: 12rpx 30rpx; margin-right: 16rpx;
  background: #fff; border-radius: 30rpx; font-size: 26rpx; color: #4b5563;
  border: 1rpx solid #e5eaf3;
}
.chip.active { background: #1f4e9c; color: #fff; border-color: #1f4e9c; }
.alert-item {
  display: flex; background: #fff; border-radius: 16rpx; margin-bottom: 16rpx;
  overflow: hidden; box-shadow: 0 4rpx 16rpx rgba(31, 78, 156, 0.05);
}
.sev-bar { width: 10rpx; flex-shrink: 0; }
.ab { padding: 22rpx 24rpx; }
.top { margin-bottom: 10rpx; }
.sev-tag { color: #fff; font-size: 20rpx; padding: 3rpx 12rpx; border-radius: 6rpx; margin-right: 10rpx; }
.st-tag { font-size: 20rpx; padding: 3rpx 12rpx; border-radius: 6rpx; background: #eef2f8; color: #4b5563; }
.st0 { background: #fff1f0; color: #e5484d; }
.st2 { background: #fff7e6; color: #d48806; }
.st3 { background: #eef9f0; color: #2fa84f; }
.storm { margin-left: 10rpx; font-size: 22rpx; }
.sup { margin-left: 8rpx; font-size: 20rpx; color: #7c3aed; border: 1rpx solid #7c3aed; border-radius: 6rpx; padding: 0 8rpx; }
.msg { font-size: 28rpx; line-height: 1.45; margin-bottom: 8rpx; }
.load-more { text-align: center; color: #1f4e9c; padding: 24rpx; font-size: 26rpx; }
</style>
