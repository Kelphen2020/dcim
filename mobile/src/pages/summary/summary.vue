<template>
  <view class="page">
    <GlassNavBar title="管理摘要" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>
    <view class="row head">
      <text class="h-title">管理摘要</text>
      <picker class="grow" :range="dayOpts" range-key="t" @change="onDays">
        <view class="day-pick">近 {{ days }} 天 ▾</view>
      </picker>
      <text class="muted" v-if="s.generated_at">{{ timeAgo(s.generated_at) }}</text>
    </view>

    <!-- KPI 宫格 -->
    <view class="kpi-grid">
      <view class="kpi">
        <view class="kpi-num" :class="pueCls">{{ s.energy.latest_pue ? s.energy.latest_pue.toFixed(2) : '—' }}</view>
        <view class="kpi-label">最新 PUE</view>
      </view>
      <view class="kpi">
        <view class="kpi-num accent-c">{{ fmt(s.energy.period_kwh) }}</view>
        <view class="kpi-label">期间能耗 kWh</view>
      </view>
      <view class="kpi">
        <view class="kpi-num" :class="s.sla.respond_rate < 90 ? 'red' : 'green'">{{ s.sla.respond_rate }}%</view>
        <view class="kpi-label">SLA 响应达标</view>
      </view>
      <view class="kpi">
        <view class="kpi-num" :class="s.maintenance.coverage_pct < 60 ? 'orange' : 'green'">{{ s.maintenance.coverage_pct }}%</view>
        <view class="kpi-label">维保覆盖率</view>
      </view>
    </view>

    <!-- 运维态势 -->
    <view class="card row stat-row">
      <view class="sr"><view class="sr-n red">{{ s.alerts.active }}</view><view class="muted">活跃告警</view></view>
      <view class="sr"><view class="sr-n orange">{{ s.alerts.week_new }}</view><view class="muted">7日新增</view></view>
      <view class="sr"><view class="sr-n accent-c">{{ s.sla.resolve_breach }}</view><view class="muted">解决违约</view></view>
      <view class="sr"><view class="sr-n" :class="s.changes.pending_approval > 0 ? 'red' : ''">{{ s.changes.pending_approval }}</view><view class="muted">待审变更</view></view>
    </view>

    <!-- PUE 趋势 -->
    <view class="sec-title">PUE 走势（近{{ days }}天）</view>
    <view class="card">
      <canvas canvas-id="pueCanvas" id="pueCanvas" class="chart"></canvas>
      <view v-if="!(s.energy.pue_trend || []).length" class="empty-tip sm">暂无 PUE 数据</view>
    </view>

    <!-- 告警趋势 -->
    <view class="sec-title">告警趋势（近{{ days }}天）</view>
    <view class="card">
      <canvas canvas-id="alertCanvas" id="alertCanvas" class="chart"></canvas>
      <view v-if="!(s.alerts.trend || []).length" class="empty-tip sm">暂无告警</view>
    </view>

    <!-- 维保覆盖 -->
    <view class="sec-title">维保覆盖</view>
    <view class="card">
      <view class="cov-bar"><view class="cov-fg" :style="{ width: s.maintenance.coverage_pct + '%' }"></view></view>
      <view class="row cov-txt">
        <text class="grow muted">在保 {{ s.maintenance.covered }} / {{ s.maintenance.asset_total }} 台（非报废）</text>
        <text class="cov-pct">{{ s.maintenance.coverage_pct }}%</text>
      </view>
    </view>

    <!-- 资产规模 -->
    <view class="card row stat-row">
      <view class="sr"><view class="sr-n accent-c">{{ s.assets.online }}</view><view class="muted">在架资产</view></view>
      <view class="sr"><view class="sr-n">{{ s.assets.cabinets }}</view><view class="muted">机柜</view></view>
      <view class="sr"><view class="sr-n" :class="s.stocktake.running > 0 ? 'orange' : ''">{{ s.stocktake.running }}</view><view class="muted">盘点中</view></view>
      <view class="sr"><view class="sr-n">{{ s.changes.window_7d }}</view><view class="muted">7日窗口变更</view></view>
    </view>
    <GlassTabBar current="me" :scrollY="scrollY" />
  </view>
</template>

<script>
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import { get } from '../../api.js'
import { requireAuth, timeAgo } from '../../util.js'

export default {
  components: { GlassNavBar, GlassTabBar },
  data() {
    return {
      scrollY: 0,
      navH: 44,
      days: 30,
      dayOpts: [{ v: 7, t: '近 7 天' }, { v: 30, t: '近 30 天' }, { v: 90, t: '近 90 天' }],
      s: {
        energy: { latest_pue: 0, period_kwh: 0, pue_trend: [] },
        sla: { respond_rate: 0, resolve_breach: 0 },
        maintenance: { coverage_pct: 0, covered: 0, asset_total: 0 },
        alerts: { active: 0, week_new: 0, trend: [] },
        changes: { pending_approval: 0, window_7d: 0 },
        stocktake: { running: 0 },
        assets: { online: 0, cabinets: 0 },
      },
    }
  },
  computed: {
    pueCls() {
      const p = this.s.energy.latest_pue || 0
      return p > 1.6 ? 'red' : p > 1.5 ? 'orange' : 'green'
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
    fmt(n) { return n >= 10000 ? (n / 10000).toFixed(1) + '万' : String(n || 0) },
    onDays(e) { this.days = this.dayOpts[Number(e.detail.value)].v; this.load() },
    async load() {
      try {
        this.s = await get('/mobile/summary', { days: this.days })
        this.$nextTick(() => {
          this.drawLine('pueCanvas', (this.s.energy.pue_trend || []).map((x) => x.pue), (this.s.energy.pue_trend || []).map((x) => x.day), '#1f4e9c')
          this.drawBars('alertCanvas', (this.s.alerts.trend || []).map((x) => x.n), (this.s.alerts.trend || []).map((x) => x.day), '#e5484d')
        })
      } catch (e) { /* toast */ }
    },
    drawLine(id, vals, labels, color) {
      if (!vals.length) return
      const W = 320, H = 150, PAD = 14
      const min = Math.min(...vals), max = Math.max(...vals), span = max - min || 1
      const ctx = uni.createCanvasContext(id, this)
      ctx.setStrokeStyle('#eef2f8'); ctx.setLineWidth(1)
      for (let i = 0; i <= 3; i++) {
        const y = PAD + ((H - 2 * PAD) * i) / 3
        ctx.beginPath(); ctx.moveTo(PAD, y); ctx.lineTo(W - PAD, y); ctx.stroke()
      }
      ctx.setStrokeStyle(color); ctx.setLineWidth(2); ctx.beginPath()
      vals.forEach((v, i) => {
        const x = PAD + ((W - 2 * PAD) * i) / (vals.length - 1 || 1)
        const y = H - PAD - ((v - min) / span) * (H - 2 * PAD)
        i === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y)
      })
      ctx.stroke()
      ctx.setFillStyle('#8a94a6'); ctx.setFontSize(9)
      ctx.fillText(max.toFixed(2), PAD, PAD - 2)
      ctx.fillText(min.toFixed(2), PAD, H - 2)
      if (labels.length) { ctx.fillText(labels[0], PAD, H - 2); ctx.fillText(labels[labels.length - 1], W - 40, H - 2) }
      ctx.draw()
    },
    drawBars(id, vals, labels, color) {
      if (!vals.length) return
      const W = 320, H = 150, PAD = 14
      const max = Math.max(...vals) || 1
      const ctx = uni.createCanvasContext(id, this)
      const bw = (W - 2 * PAD) / vals.length
      ctx.setFillStyle(color)
      vals.forEach((v, i) => {
        const h = (v / max) * (H - 2 * PAD)
        ctx.fillRect(PAD + i * bw + 1, H - PAD - h, Math.max(bw - 2, 2), h)
      })
      ctx.setFillStyle('#8a94a6'); ctx.setFontSize(9)
      ctx.fillText(String(max), PAD, PAD - 2)
      if (labels.length) { ctx.fillText(labels[0], PAD, H - 2); ctx.fillText(labels[labels.length - 1], W - 40, H - 2) }
      ctx.draw()
    },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 40rpx; }
.head { align-items: center; margin-bottom: 16rpx; }
.h-title { font-size: 34rpx; font-weight: 800; color: #1f4e9c; margin-right: 20rpx; }
.day-pick { font-size: 25rpx; color: #4b5563; background: #fff; border-radius: 20rpx; padding: 6rpx 20rpx; display: inline-block; }
.kpi-grid { display: flex; flex-wrap: wrap; justify-content: space-between; }
.kpi { width: 48%; box-sizing: border-box; background: #fff; border-radius: 20rpx; padding: 24rpx; margin-bottom: 18rpx; box-shadow: 0 4rpx 16rpx rgba(31, 78, 156, 0.06); }
.kpi-num { font-size: 48rpx; font-weight: 800; color: #1f2937; line-height: 1.1; }
.kpi-num.red { color: #e5484d; }
.kpi-num.orange { color: #f5a623; }
.kpi-num.green { color: #2fa84f; }
.kpi-num.accent-c { color: #1f4e9c; }
.kpi-label { font-size: 24rpx; color: #8a94a6; margin-top: 6rpx; }
.stat-row { justify-content: space-between; }
.sr { flex: 1; text-align: center; }
.sr-n { font-size: 38rpx; font-weight: 800; color: #1f2937; }
.sr-n.red { color: #e5484d; }
.sr-n.orange { color: #f5a623; }
.sr-n.accent-c { color: #1f4e9c; }
.sec-title { font-size: 28rpx; font-weight: 700; margin: 20rpx 4rpx 12rpx; }
.chart { width: 100%; height: 320rpx; }
.cov-bar { height: 20rpx; background: #eef2f8; border-radius: 10rpx; overflow: hidden; }
.cov-fg { height: 100%; background: linear-gradient(90deg, #2fa84f, #5cc97f); border-radius: 10rpx; }
.cov-txt { margin-top: 12rpx; }
.cov-pct { font-size: 30rpx; font-weight: 800; color: #2fa84f; }
.empty-tip.sm { padding: 20rpx 0; }
.page { padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 170rpx) !important; }
</style>
