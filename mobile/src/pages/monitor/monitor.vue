<template>
  <view class="page">
    <GlassNavBar title="监控速览" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>
    <!-- 在线率统计条 -->
    <view class="card stat-bar">
      <view class="sb-item"><view class="sb-num">{{ total }}</view><view class="muted">测点</view></view>
      <view class="sb-item" @tap="quick('status', 1)"><view class="sb-num green">{{ onlineCount }}</view><view class="muted">正常</view></view>
      <view class="sb-item" @tap="quick('status', 2)"><view class="sb-num red">{{ alarmCount }}</view><view class="muted">告警</view></view>
      <view class="sb-item" @tap="quick('status', 0)"><view class="sb-num gray">{{ total - onlineCount - alarmCount - faultCount }}</view><view class="muted">离线</view></view>
      <view class="sb-item" @tap="quick('status', 3)"><view class="sb-num orange">{{ faultCount }}</view><view class="muted">故障</view></view>
    </view>

    <!-- 类型筛选 -->
    <scroll-view scroll-x class="chips">
      <view v-for="t in types" :key="t.k" class="chip" :class="{ active: curType === t.k }" @tap="setType(t.k)">{{ t.t }}</view>
    </scroll-view>

    <CylList :items="items" :scrollY="scrollY" :itemH="78" @pick="(p) => openTrend(p)">
      <template #empty>
        <view v-if="loading" class="empty-tip">加载中…</view>
        <view v-else class="empty-tip">没有匹配的测点</view>
      </template>
      <template #item="{ row: p }">
        <view class="card p-item">
          <view class="row">
            <text class="p-name grow">{{ p.name }}</text>
            <text class="live-val" :class="statusCls(p)">{{ liveVal(p) }}</text>
          </view>
          <view class="row sub">
            <text class="muted">{{ typeText(p.type) }} · {{ p.code }}</text>
            <text class="grow"></text>
            <text class="st-dot" :class="'sd' + p.status"></text>
            <text class="muted">{{ stText(p.status) }}</text>
          </view>
        </view>
      </template>
    </CylList>
    <view v-if="hasMore && items.length" class="load-more" @tap="loadMore">加载更多</view>

    <!-- 趋势弹层 -->
    <view class="mask" v-if="trendPoint" @tap.self="trendPoint = null">
      <view class="sheet">
        <view class="sheet-title">{{ trendPoint.name }} · 近24h</view>
        <view class="muted" v-if="trendStats">最小 {{ trendStats.min }} · 均值 {{ trendStats.avg }} · 最大 {{ trendStats.max }} {{ trendPoint.unit || '' }}</view>
        <canvas canvas-id="trendCanvas" id="trendCanvas" class="trend-canvas"></canvas>
        <view v-if="trendLoading" class="empty-tip sm">加载趋势…</view>
        <button class="btn-ghost" @tap="trendPoint = null">关闭</button>
      </view>
    </view>
    <GlassTabBar current="index" :scrollY="scrollY" />
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
    return {
      scrollY: 0,
      navH: 44,
      items: [], total: 0, page: 1, size: 30, loading: false,
      curType: '', curStatus: '',
      summary: {},
      types: [
        { k: '', t: '全部' },
        { k: 'temp_humidity', t: '温湿度' },
        { k: 'ups', t: 'UPS' },
        { k: 'power_dist', t: '配电' },
        { k: 'hvac', t: '空调' },
        { k: 'water_leak', t: '漏水' },
        { k: 'smoke', t: '烟感' },
      ],
      trendPoint: null, trendLoading: false, trendStats: null,
      liveMap: {},
      onlineCount: 0, alarmCount: 0, faultCount: 0,
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
    this.loadSummary()
  },
  onPullDownRefresh() {
    Promise.all([this.reload(), this.loadSummary()]).finally(() => {
      uni.stopPullDownRefresh()
      this.refreshLive()
    })
  },
  methods: {
    typeText(t) {
      const m = { temp_humidity: '温湿度', ups: 'UPS', power_dist: '配电', hvac: '空调', water_leak: '漏水', smoke: '烟感' }
      return m[t] || t
    },
    stText(s) { return { 0: '离线', 1: '正常', 2: '告警', 3: '故障' }[s] || '' },
    statusCls(p) { return { 0: 'gray', 1: 'green', 2: 'red', 3: 'orange' }[p.status] || '' },
    liveVal(p) {
      const lv = this.liveMap[p.id]
      if (lv && lv.current_value != null) return lv.current_value + (lv.unit || p.unit || '')
      return p.last_value != null ? p.last_value + (p.unit || '') : '--'
    },
    setType(k) { this.curType = k; this.reload() },
    quick(field, v) { this.curStatus = String(v); this.reload() },
    async loadSummary() {
      try {
        const d = await get('/monitoring-summary', {}, { silent: true })
        // 响应为六类各 {total,normal,alert,offline} + active_alerts
        let total = 0, normal = 0, alert = 0, fault = 0
        for (const k of ['power_dist', 'ups', 'hvac', 'temp_humidity', 'water_leak', 'smoke']) {
          const s = d[k] || {}
          total += s.total || 0
          normal += s.normal || 0
          alert += s.alert || 0
          fault += (s.total || 0) - (s.normal || 0) - (s.alert || 0) - (s.offline || 0)
        }
        this.total = total
        this.onlineCount = normal
        this.alarmCount = alert
        this.faultCount = Math.max(fault, 0)
      } catch (e) { /* ignore */ }
    },
    async reload() { this.page = 1; await this.fetch(true) },
    async loadMore() { this.page += 1; await this.fetch(false) },
    async fetch(reset) {
      this.loading = true
      try {
        const p = { page: this.page, size: this.size }
        if (this.curType) p.type = this.curType
        if (this.curStatus !== '') p.status = this.curStatus
        const d = await get('/monitoring-points', p)
        this.total = d.total || 0
        this.items = reset ? (d.items || []) : this.items.concat(d.items || [])
        if (reset) this.refreshLive()
      } catch (e) { /* toast */ } finally { this.loading = false }
    },
    async refreshLive() {
      const ids = this.items.map((i) => i.id)
      if (!ids.length) return
      try {
        const d = await get('/points-live', { ids: ids.join(',') }, { silent: true })
        const map = {}
        const arr = d.items || d || []
        for (const x of arr) map[x.point_id || x.id] = x
        this.liveMap = map
      } catch (e) { /* ignore */ }
    },
    async openTrend(p) {
      this.trendPoint = p
      this.trendLoading = true
      this.trendStats = null
      try {
        const d = await get('/trend-series', { point_ids: String(p.id), range: '24h' }, { silent: true })
        const series = (d.series || d || [])[0]
        this.trendLoading = false
        if (series && series.stats) {
          this.trendStats = {
            min: Number(series.stats.min ?? 0).toFixed(1),
            avg: Number(series.stats.avg ?? 0).toFixed(1),
            max: Number(series.stats.max ?? 0).toFixed(1),
          }
        }
        this.$nextTick(() => this.drawTrend(series ? series.data : []))
      } catch (e) { this.trendLoading = false }
    },
    drawTrend(data) {
      if (!data || !data.length) return
      const W = 320, H = 160, PAD = 10
      const vals = data.map((d) => d.v)
      const min = Math.min(...vals), max = Math.max(...vals)
      const span = max - min || 1
      const ctx = uni.createCanvasContext('trendCanvas', this)
      // 网格
      ctx.setStrokeStyle('#eef2f8')
      ctx.setLineWidth(1)
      for (let i = 0; i <= 4; i++) {
        const y = PAD + ((H - 2 * PAD) * i) / 4
        ctx.beginPath(); ctx.moveTo(PAD, y); ctx.lineTo(W - PAD, y); ctx.stroke()
      }
      // 折线
      ctx.setStrokeStyle('#1f4e9c')
      ctx.setLineWidth(2)
      ctx.beginPath()
      data.forEach((d, i) => {
        const x = PAD + ((W - 2 * PAD) * i) / (data.length - 1 || 1)
        const y = H - PAD - ((d.v - min) / span) * (H - 2 * PAD)
        if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y)
      })
      ctx.stroke()
      // 最大/最小标注
      ctx.setFillStyle('#8a94a6')
      ctx.setFontSize(9)
      ctx.fillText(String(max.toFixed(1)), PAD, PAD - 1)
      ctx.fillText(String(min.toFixed(1)), PAD, H - 2)
      ctx.draw()
    },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 40rpx; }
.stat-bar { display: flex; }
.sb-item { flex: 1; text-align: center; }
.sb-num { font-size: 40rpx; font-weight: 800; color: #1f2937; }
.sb-num.green { color: #2fa84f; }
.sb-num.red { color: #e5484d; }
.sb-num.orange { color: #f5a623; }
.sb-num.gray { color: #8a94a6; }
.chips { white-space: nowrap; margin-bottom: 16rpx; }
.chip { display: inline-block; padding: 10rpx 26rpx; margin-right: 14rpx; background: #fff; border-radius: 26rpx; font-size: 25rpx; color: #4b5563; border: 1rpx solid #e5eaf3; }
.chip.active { background: #1f4e9c; color: #fff; border-color: #1f4e9c; }
.p-item { padding: 22rpx 24rpx; }
.p-name { font-size: 29rpx; font-weight: 600; }
.live-val { font-size: 32rpx; font-weight: 800; font-family: monospace; }
.live-val.green { color: #2fa84f; }
.live-val.red { color: #e5484d; }
.live-val.orange { color: #f5a623; }
.live-val.gray { color: #8a94a6; }
.sub { margin-top: 8rpx; }
.st-dot { width: 14rpx; height: 14rpx; border-radius: 50%; margin-right: 8rpx; background: #8a94a6; }
.sd1 { background: #2fa84f; }
.sd2 { background: #e5484d; }
.sd3 { background: #f5a623; }
.load-more { text-align: center; color: #1f4e9c; padding: 24rpx; font-size: 26rpx; }
.mask { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.35); z-index: 100; display: flex; align-items: flex-end; }
.sheet { width: 100%; background: #fff; border-radius: 28rpx 28rpx 0 0; padding: 32rpx; box-sizing: border-box; }
.sheet-title { font-size: 32rpx; font-weight: 700; margin-bottom: 8rpx; }
.trend-canvas { width: 100%; height: 340rpx; margin: 20rpx 0; background: #fafbfd; border-radius: 12rpx; }
.empty-tip.sm { padding: 20rpx 0; }
.page { padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 170rpx) !important; }
</style>
