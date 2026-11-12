<template>
  <view class="page">
    <GlassNavBar title="消息中心" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>
    <view class="top-ops">
      <view class="muted grow">共 {{ total }} 条 · 未读 {{ unreadCount }}</view>
      <text class="read-all" @tap="readAll">全部已读</text>
    </view>

    <CylList :items="items" :scrollY="scrollY" :itemH="96" @pick="(n) => readOne(n)">
      <template #empty>
        <view v-if="loading" class="empty-tip">加载中…</view>
        <view v-else class="empty-tip">暂无消息</view>
      </template>
      <template #item="{ row: n }">
        <view class="card n-item" :class="{ unread: !n.read }">
          <view class="row">
            <view class="dot" v-if="!n.read"></view>
            <text class="n-title" :class="{ bold: !n.read }">{{ n.title }}</text>
            <text class="grow"></text>
            <text class="muted">{{ timeAgo(n.created_at) }}</text>
          </view>
          <view class="n-content">{{ n.content }}</view>
          <view class="muted kind">{{ kindText(n.kind) }}<text v-if="drillable(n)" class="drill-t"> · 查看详情 ›</text></view>
        </view>
      </template>
    </CylList>

    <view v-if="hasMore && items.length" class="load-more" @tap="loadMore">加载更多</view>
    <GlassTabBar current="index" :scrollY="scrollY" />
  </view>
</template>

<script>
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import CylList from '../../components/CylList.vue'
import { get, post } from '../../api.js'
import { requireAuth, timeAgo } from '../../util.js'

const KIND = {
  alarm: '告警', alarm_resolve: '告警恢复', alarm_escalation: '告警升级',
  work_order: '工单', maintenance_due: '维保到期', trend_forecast: '趋势预警',
  sla_warn: 'SLA预警', sla_breach: 'SLA违约', inspection: '巡检',
}

export default {
  components: { GlassNavBar, GlassTabBar, CylList },
  data() {
    return { scrollY: 0, navH: 44, items: [], total: 0, page: 1, size: 30, loading: false }
  },
  computed: {
    hasMore() { return this.items.length < this.total },
    unreadCount() { return this.items.filter((n) => !n.read).length },
  },
  onLoad() {
    try { this.navH = (uni.getSystemInfoSync().statusBarHeight || 0) + 44 } catch (e) {}
    if (!requireAuth()) return
    this.reload()
  },
  onPageScroll(e) { this.scrollY = e.scrollTop },
  methods: {
    timeAgo,
    kindText(k) { return KIND[k] || k },
    drillable(n) { return !!n.ref_id && ['alert', 'work_order', 'asset', 'change_request', 'maintenance', 'duty_handover', 'inspection_plan'].includes(n.ref_type) },
    async reload() { this.page = 1; await this.fetch(true) },
    async loadMore() { this.page += 1; await this.fetch(false) },
    async fetch(reset) {
      this.loading = true
      try {
        const d = await get('/notifications', { page: this.page, size: this.size })
        this.total = d.total || 0
        this.items = reset ? (d.items || []) : this.items.concat(d.items || [])
      } catch (e) { /* ignore */ } finally { this.loading = false }
    },
    async readOne(n) {
      if (!n.read) {
        try {
          await post(`/notifications-read/${n.id}`, {}, { silent: true })
          n.read = true
        } catch (e) { /* ignore */ }
      }
      this.drill(n)
    },
    // M6-E1：按 ref_type 穿透到业务详情页
    drill(n) {
      const rt = n.ref_type, rid = n.ref_id
      if (!rt || !rid) return
      if (rt === 'alert') {
        uni.navigateTo({ url: '/pages/alert/detail?id=' + rid })
      } else if (rt === 'work_order') {
        uni.navigateTo({ url: '/pages/wo/detail?id=' + rid })
      } else if (rt === 'asset') {
        uni.navigateTo({ url: '/pages/asset/detail?id=' + rid })
      } else if (rt === 'change_request') {
        uni.navigateTo({ url: '/pages/change/detail?id=' + rid })
      } else if (rt === 'maintenance') {
        uni.navigateTo({ url: '/pages/maint/maint' })
      } else if (rt === 'duty_handover') {
        uni.navigateTo({ url: '/pages/duty/handover' })
      } else if (rt === 'inspection_plan') {
        uni.navigateTo({ url: '/pages/scan/scan' })
      }
    },
    async readAll() {
      const unread = this.items.filter((n) => !n.read)
      if (!unread.length) { uni.showToast({ title: '没有未读消息', icon: 'none' }); return }
      try {
        for (const n of unread) {
          await post(`/notifications-read/${n.id}`, {}, { silent: true })
          n.read = true
        }
        uni.showToast({ title: '已全部标记已读', icon: 'success' })
      } catch (e) { /* ignore */ }
    },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 40rpx; }
.top-ops { display: flex; align-items: center; margin-bottom: 16rpx; padding: 0 4rpx; }
.read-all { color: #1f4e9c; font-size: 26rpx; }
.n-item.unread { border-left: 8rpx solid #1f4e9c; }
.dot { width: 14rpx; height: 14rpx; border-radius: 50%; background: #e5484d; margin-right: 12rpx; flex-shrink: 0; }
.n-title { font-size: 28rpx; }
.n-title.bold { font-weight: 700; }
.n-content { font-size: 26rpx; color: #4b5563; line-height: 1.5; margin: 10rpx 0; }
.kind { font-size: 22rpx; }
.drill-t { color: #2d6bc4; }
.load-more { text-align: center; color: #1f4e9c; padding: 24rpx; font-size: 26rpx; }
.page { padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 170rpx) !important; }
</style>
