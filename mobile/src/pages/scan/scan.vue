<template>
  <view class="page" :style="{ paddingTop: navH + 'px', paddingBottom: 'calc(env(safe-area-inset-bottom, 0px) + 150rpx)' }">
    <GlassNavBar title="扫码巡检" :scrollY="scrollY" />
    <view class="scan-hero" @tap="onScan">
      <view class="hero-ico"><TechIcon name="scan" color="#fff" :size="88" /></view>
      <view class="hero-txt">扫一扫签到巡检</view>
      <view class="hero-sub">扫描机柜/房间二维码，防补录签到</view>
    </view>

    <view class="sec-title">今日巡检</view>
    <view class="card row today">
      <view class="grow">
        <view class="today-num">{{ doneToday }}<text class="of">/{{ plans.length }}</text></view>
        <view class="muted">计划完成进度</view>
      </view>
      <view class="ring"><TechIcon :name="allDone ? 'check' : 'clock'" :color="allDone ? '#2FA84F' : '#F5A623'" :size="56" /></view>
    </view>

    <view class="sec-title">启用中的巡检计划</view>
    <view class="card plan" v-for="p in plans" :key="p.id">
      <view class="row">
        <view class="grow">
          <view class="plan-name">{{ p.name }}</view>
          <view class="muted">{{ freqText(p.freq) }} · {{ itemCount(p) }} 项 · 上次 {{ p.last_run_at ? timeAgo(p.last_run_at) : '未执行' }}</view>
        </view>
        <text class="tag" :class="ranToday(p) ? 'ok' : 'todo'">{{ ranToday(p) ? '今日已巡' : '待巡' }}</text>
      </view>
    </view>
    <view v-if="!plans.length" class="empty-tip">暂无启用中的巡检计划</view>

    <view class="sec-title">最近巡检记录</view>
    <view class="card rec" v-for="r in records" :key="r.id">
      <view class="row">
        <text class="rec-st" :class="'rs' + r.status">{{ recSt(r.status) }}</text>
        <view class="grow rec-body">
          <view>{{ r.target_name || planName(r.plan_id) || '巡检记录' }}<text v-if="r.source === 'scan'" class="scan-tag">扫码</text></view>
          <view class="muted">{{ dt(r.inspected_at) }}<text v-if="r.report_code"> · {{ r.report_code }}</text></view>
        </view>
      </view>
      <view class="abn" v-if="r.abnormal && r.status === 2">{{ r.abnormal }}</view>
    </view>
    <view v-if="!records.length" class="empty-tip">暂无巡检记录</view>
      <GlassTabBar current="scan" :scrollY="scrollY" />
  </view>
</template>

<script>
import TechIcon from '../../components/TechIcon.vue'
import { get } from '../../api.js'
import { requireAuth, timeAgo, doScan } from '../../util.js'
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'

export default {
  components: { GlassNavBar, GlassTabBar, TechIcon },
  data() {
    return { plans: [], records: [] }
  },
  computed: {
    doneToday() {
      return this.plans.filter((p) => this.ranToday(p)).length
    },
    allDone() {
      return this.plans.length > 0 && this.doneToday >= this.plans.length
    },
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
    timeAgo,
    freqText(f) { return { daily: '每日', weekly: '每周', monthly: '每月' }[f] || f },
    recSt(s) { return { 1: '正常', 2: '异常', 3: '逾期' }[s] || '' },
    dt(t) { return t ? String(t).replace('T', ' ').slice(0, 16) : '' },
    itemCount(p) { return (p.items || '').split(/[|\n]/).filter(Boolean).length },
    planName(id) { const p = this.plans.find((x) => x.id === id); return p ? p.name : '' },
    ranToday(p) {
      if (!p.last_run_at) return false
      const d = String(p.last_run_at).slice(0, 10)
      const now = new Date()
      const t = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`
      return d === t
    },
    async load() {
      try {
        const d = await get('/inspection-plans', { page: 1, size: 100 })
        this.plans = (d.items || []).filter((p) => p.status === 1)
      } catch (e) { /* ignore */ }
      try {
        const r = await get('/inspection-records', { page: 1, size: 10 })
        this.records = r.items || []
      } catch (e) { /* ignore */ }
    },
    onScan() { doScan() },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 40rpx; }
.scan-hero {
  background: linear-gradient(135deg, #1f4e9c, #2d6bc4);
  border-radius: 24rpx; padding: 60rpx 0; text-align: center; color: #fff;
  box-shadow: 0 8rpx 24rpx rgba(31, 78, 156, 0.25); margin-bottom: 10rpx;
}
.hero-ico { font-size: 80rpx; }
.hero-txt { font-size: 36rpx; font-weight: 700; margin-top: 12rpx; }
.hero-sub { font-size: 24rpx; opacity: 0.8; margin-top: 8rpx; }
.sec-title { font-size: 30rpx; font-weight: 700; margin: 24rpx 4rpx 14rpx; }
.today { align-items: center; }
.today-num { font-size: 52rpx; font-weight: 800; color: #1f4e9c; }
.today-num .of { font-size: 26rpx; color: #8a94a6; font-weight: 400; }
.ring { font-size: 56rpx; }
.plan-name { font-size: 30rpx; font-weight: 600; margin-bottom: 6rpx; }
.tag { font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 8rpx; }
.tag.ok { background: #eef9f0; color: #2fa84f; }
.tag.todo { background: #fff7e6; color: #d48806; }
.rec-st { font-size: 22rpx; padding: 6rpx 14rpx; border-radius: 8rpx; margin-right: 16rpx; }
.rs1 { background: #eef9f0; color: #2fa84f; }
.rs2 { background: #fff1f0; color: #e5484d; }
.rs3 { background: #f3f4f6; color: #8a94a6; }
.rec-body view:first-child { font-size: 28rpx; font-weight: 600; }
.scan-tag { font-size: 20rpx; color: #1f4e9c; border: 1rpx solid #1f4e9c; border-radius: 6rpx; padding: 0 8rpx; margin-left: 10rpx; }
.abn { margin-top: 10rpx; font-size: 24rpx; color: #e5484d; background: #fff1f0; border-radius: 8rpx; padding: 10rpx 14rpx; }
</style>
