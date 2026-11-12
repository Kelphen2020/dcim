<template>
  <view class="page" :style="{ paddingTop: navH + 'px', paddingBottom: 'calc(env(safe-area-inset-bottom, 0px) + 150rpx)' }">
    <GlassNavBar title="运维工单" :scrollY="scrollY">
      <template #right>
        <view class="nav-add" @tap="goCreate"><TechIcon name="pen" color="#1F4E9C" :size="36" /></view>
      </template>
    </GlassNavBar>
    <scroll-view scroll-x class="chips">
      <view v-for="c in chips" :key="c.k" class="chip" :class="{ active: cur === c.k }" @tap="setChip(c.k)">{{ c.t }}</view>
    </scroll-view>

    <CylList :items="items" :scrollY="scrollY" :itemH="104" @pick="(w) => goDetail(w)">
      <template #empty>
        <view v-if="loading" class="empty-tip">加载中…</view>
        <view v-else class="empty-tip">没有匹配的工单</view>
      </template>
      <template #item="{ row: w }">
        <view class="card wo">
          <view class="row">
            <text class="code">{{ w.code }}</text>
            <text class="grow"></text>
            <text class="st" :class="'ws' + w.status">{{ stText(w.status) }}</text>
          </view>
          <view class="title">{{ w.title }}</view>
          <view class="row foot">
            <text class="tag" :class="prioCls(w.priority)">{{ prioText(w.priority) }}</text>
            <text class="tag type">{{ w.type || 'task' }}</text>
            <text class="grow"></text>
            <text class="sla" :class="slaCls(w)">{{ slaText(w) }}</text>
          </view>
          <view class="muted time">{{ timeAgo(w.created_at) }}</view>
        </view>
      </template>
    </CylList>

    <view v-if="hasMore && items.length" class="load-more" @tap="loadMore">加载更多</view>
      <GlassTabBar current="wo" :scrollY="scrollY" />
  </view>
</template>

<script>
import { get } from '../../api.js'
import { requireAuth, timeAgo, slaLeft, WO_STATUS, WO_PRIORITY, user } from '../../util.js'
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import CylList from '../../components/CylList.vue'
import TechIcon from '../../components/TechIcon.vue'

export default {
  components: { GlassNavBar, GlassTabBar, CylList, TechIcon },
  data() {
    return {
      scrollY: 0,
      navH: 44,
      cur: 'mine',
      page: 1,
      size: 20,
      total: 0,
      items: [],
      loading: false,
      chips: [
        { k: 'mine', t: '待我处理' },
        { k: 'breach', t: 'SLA违约' },
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
    stText: (s) => WO_STATUS[s] || '',
    prioText: (p) => WO_PRIORITY[p] || '',
    prioCls(p) { return { 1: 'p-urgent', 2: 'p-high', 3: 'p-mid', 4: 'p-low' }[p] || 'p-low' },
    slaText(w) {
      if (w.sla_respond_breach || w.sla_resolve_breach) return 'SLA 已违约'
      const due = w.status === 1 ? w.sla_respond_due : w.sla_resolve_due
      return slaLeft(due)
    },
    slaCls(w) {
      if (w.sla_respond_breach || w.sla_resolve_breach) return 'breach'
      if (w.sla_respond_warned || w.sla_resolve_warned) return 'warn'
      return ''
    },
    setChip(k) { this.cur = k; this.reload() },
    async reload() { this.page = 1; await this.fetch(true) },
    async loadMore() { this.page += 1; await this.fetch(false) },
    async fetch(reset) {
      this.loading = true
      try {
        const p = { page: this.page, size: this.size }
        if (this.cur === 'breach') p.sla = 'breach'
        if (this.cur === 'done') p.status = 3
        const d = await get('/work-orders', p)
        let items = d.items || []
        this.total = d.total || 0
        // 「待我处理」前端过滤：指派给我(或未指派)且状态1/2
        if (this.cur === 'mine') {
          const uid = user().id
          items = items.filter((w) => (w.status === 1 || w.status === 2) && (!w.assignee_id || w.assignee_id === uid))
        }
        this.items = reset ? items : this.items.concat(items)
      } catch (e) { /* ignore */ } finally { this.loading = false }
    },
    goDetail(w) {
      uni.setStorageSync('dcim_wo_cur', w)
      uni.navigateTo({ url: '/pages/wo/detail?id=' + w.id })
    },
    goCreate() { uni.navigateTo({ url: '/pages/wo/create' }) },
  },
}
</script>

<style scoped>
.page { padding: 0 24rpx 40rpx; }
.chips { white-space: nowrap; padding: 20rpx 0; }
.nav-add { padding: 6rpx 10rpx; }
.chip {
  display: inline-block; padding: 12rpx 30rpx; margin-right: 16rpx;
  background: #fff; border-radius: 30rpx; font-size: 26rpx; color: #4b5563; border: 1rpx solid #e5eaf3;
}
.chip.active { background: #1f4e9c; color: #fff; border-color: #1f4e9c; }
.wo .code { font-size: 24rpx; color: #8a94a6; font-family: monospace; }
.wo .title { font-size: 30rpx; font-weight: 600; margin: 12rpx 0; line-height: 1.4; }
.st { font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 8rpx; background: #eef2f8; color: #4b5563; }
.ws1 { background: #fff1f0; color: #e5484d; }
.ws2 { background: #fff7e6; color: #d48806; }
.ws3 { background: #eef9f0; color: #2fa84f; }
.foot { margin-bottom: 8rpx; }
.tag { color: #fff; background: #8a94a6; }
.p-urgent { background: #e5484d; }
.p-high { background: #f5a623; }
.p-mid { background: #3b82f6; }
.p-low { background: #8a94a6; }
.tag.type { background: #eef2f8; color: #4b5563; }
.sla { font-size: 22rpx; color: #2fa84f; }
.sla.warn { color: #f5a623; font-weight: 700; }
.sla.breach { color: #e5484d; font-weight: 700; }
.time { font-size: 22rpx; }
.load-more { text-align: center; color: #1f4e9c; padding: 24rpx; font-size: 26rpx; }
</style>
