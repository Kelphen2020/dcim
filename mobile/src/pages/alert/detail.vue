<template>
  <view class="page" v-if="a.id">
    <GlassNavBar title="告警详情" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>
    <view class="card head" :style="{ borderLeft: '12rpx solid ' + sevColor(a.severity) }">
      <view class="row">
        <text class="sev-tag" :style="{ background: sevColor(a.severity) }">{{ sevText(a.severity) }}</text>
        <text class="st-tag" :class="'st' + a.status">{{ stText(a.status) }}</text>
        <text class="grow"></text>
        <text class="muted">#{{ a.id }}</text>
      </view>
      <view class="msg">{{ a.message }}</view>
      <view class="muted meta">
        <view v-if="a.point_name" class="meta-line"><TechIcon name="pin" color="#8A94A6" :size="26" /> 测点：{{ a.point_name }}（{{ a.point_code || '-' }}）</view>
        <view class="meta-line"><TechIcon name="clock" color="#8A94A6" :size="26" /> 触发：{{ a.triggered_at ? a.triggered_at.replace('T', ' ').slice(0, 19) : '' }}（{{ timeAgo(a.triggered_at) }}）</view>
        <view v-if="a.wo_code" class="meta-line"><TechIcon name="wrench" color="#8A94A6" :size="26" /> 关联工单：{{ a.wo_code }}</view>
        <view v-if="a.storm_id">所属风暴 #{{ a.storm_id }}<text v-if="a.suppressed">（被抑制成员，仅根因通知）</text></view>
        <view v-if="a.escalated">已升级</view>
      </view>
      <view class="detail-json" v-if="detailText">{{ detailText }}</view>
    </view>

    <!-- 测点近24h趋势（M6-E3） -->
    <template v-if="a.point_id">
      <view class="sec-title"><TechIcon name="chart" color="#16233D" :size="30" /> 近24h趋势<text class="muted" v-if="trendStats"> · 最小 {{ trendStats.min }} / 均值 {{ trendStats.avg }} / 最大 {{ trendStats.max }}</text></view>
      <view class="card trend-card">
        <canvas canvas-id="alertTrendCanvas" id="alertTrendCanvas" class="trend-canvas"></canvas>
        <view v-if="trendLoading" class="empty-tip sm">加载趋势…</view>
        <view v-else-if="trendEmpty" class="empty-tip sm">近24h无读数</view>
      </view>
    </template>

    <!-- 相似案例 -->
    <view class="sec-title" v-if="cases.length"><TechIcon name="bolt" color="#F5A623" :size="30" /> 相似故障案例</view>
    <view class="card case" v-for="fc in cases" :key="fc.id">
      <view class="row">
        <text class="fc-code">{{ fc.code }}</text>
        <text class="grow"></text>
        <text class="score">匹配 {{ fc.score }}%</text>
      </view>
      <view class="fc-title">{{ fc.title }}</view>
      <view class="fc-sec" v-if="fc.expanded">
        <view><text class="k">现象：</text>{{ fc.symptom }}</view>
        <view><text class="k">原因：</text>{{ fc.cause }}</view>
        <view><text class="k">方案：</text>{{ fc.solution }}</view>
      </view>
      <view class="fc-toggle" @tap="fc.expanded = !fc.expanded">{{ fc.expanded ? '收起 ▲' : '展开详情 ▼' }}</view>
    </view>

    <!-- 操作 -->
    <view class="sec-title">处理操作</view>
    <view class="card">
      <view class="ops">
        <button v-if="a.status === 0" class="btn-primary op" @tap="act('acknowledge')">确认告警</button>
        <button v-if="a.status === 0" class="btn-ghost op" @tap="act('suspend')">挂起</button>
        <button v-if="a.status === 1" class="btn-ghost op" @tap="act('resume')">恢复待处理</button>
        <button v-if="a.status < 3" class="btn-ghost op danger" @tap="act('resolve')">结束告警</button>
        <button v-if="!a.wo_code && a.status < 3" class="btn-ghost op" @tap="toWo">转工单</button>
        <button class="btn-ghost op" @tap="act(a.watched ? 'unwatch' : 'watch')">{{ a.watched ? '取消关注' : '关注' }}</button>
      </view>
      <view class="muted op-note" v-if="a.status === 3">该告警已结束（{{ a.resolved_at ? timeAgo(a.resolved_at) : '' }}）</view>
    </view>

    <!-- 留痕 -->
    <view class="sec-title">操作留痕</view>
    <view class="card">
      <view v-if="!logs.length" class="empty-tip">暂无操作记录</view>
      <view class="log-item" v-for="l in logs" :key="l.id">
        <view class="dot"></view>
        <view class="grow">
          <view>{{ l.action_name || l.action }} · {{ l.username || '系统' }}</view>
          <view class="muted">{{ l.created_at ? l.created_at.replace('T', ' ').slice(0, 19) : '' }}<text v-if="l.reason"> · {{ l.reason }}</text></view>
        </view>
      </view>
    </view>
    <GlassTabBar current="alert" :scrollY="scrollY" />
  </view>
  <view v-else class="empty-tip">告警不存在或已删除</view>
</template>

<script>
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import TechIcon from '../../components/TechIcon.vue'
import { get, post } from '../../api.js'
import { requireAuth, timeAgo, SEV_TEXT } from '../../util.js'

export default {
  components: { TechIcon , GlassNavBar, GlassTabBar },
  data() {
    return { id: 0, a: {}, logs: [], cases: [], detailText: '', trendStats: null, trendLoading: false, trendEmpty: false, scrollY: 0, navH: 44 }
  },
  onLoad(q) {
    try { this.navH = (uni.getSystemInfoSync().statusBarHeight || 0) + 44 } catch (e) {}
    if (!requireAuth()) return
    this.id = Number(q.id || 0)
    // 列表/工作台已把告警对象存入 storage
    const cached = uni.getStorageSync('dcim_alert_cur')
    if (cached && cached.id === this.id) {
      this.a = cached
      this.parseDetail()
      this.loadTrend()
    } else {
      this.fetchFromList()
    }
    this.loadLogs()
    this.loadCases()
  },
  onPageScroll(e) { this.scrollY = e.scrollTop },
  methods: {
    timeAgo,
    sevText: (s) => SEV_TEXT[s] || '',
    sevColor(s) { return { 1: '#3B82F6', 2: '#F5A623', 3: '#E5484D' }[s] || '#8a94a6' },
    stText(s) { return { 0: '待处理', 1: '已挂起', 2: '已确认', 3: '已结束' }[s] || '' },
    parseDetail() {
      try {
        const d = JSON.parse(this.a.detail || '{}')
        const parts = []
        if (d.current_value !== undefined) parts.push(`当前值 ${d.current_value}${d.unit || ''}`)
        if (d.threshold !== undefined) parts.push(`阈值 ${d.operator || ''} ${d.threshold}${d.unit || ''}`)
        this.detailText = parts.join(' · ')
      } catch (e) { this.detailText = '' }
    },
    async fetchFromList() {
      // 直进详情（推送跳转）无缓存时：翻页查找
      for (let p = 1; p <= 5; p++) {
        try {
          const d = await get('/alerts', { page: p, size: 100 })
          const hit = (d.items || []).find((x) => x.id === this.id)
          if (hit) { this.a = hit; this.parseDetail(); this.loadTrend(); return }
          if ((d.items || []).length < 100) break
        } catch (e) { break }
      }
    },
    async loadLogs() {
      try {
        const d = await get(`/alerts/${this.id}/logs`)
        this.logs = d.items || []
      } catch (e) { /* ignore */ }
    },
    // M6-E3：测点近24h趋势迷你图
    async loadTrend() {
      if (!this.a.point_id) return
      this.trendLoading = true
      this.trendEmpty = false
      try {
        const d = await get('/trend-series', { point_ids: String(this.a.point_id), range: '24h' }, { silent: true })
        const series = (d.series || d || [])[0]
        this.trendLoading = false
        if (series && series.stats) {
          this.trendStats = {
            min: Number(series.stats.min ?? 0).toFixed(1),
            avg: Number(series.stats.avg ?? 0).toFixed(1),
            max: Number(series.stats.max ?? 0).toFixed(1),
          }
        }
        const data = series ? series.data : []
        if (!data || !data.length) { this.trendEmpty = true; return }
        this.$nextTick(() => this.drawTrend(data))
      } catch (e) { this.trendLoading = false; this.trendEmpty = true }
    },
    drawTrend(data) {
      if (!data || !data.length) return
      const W = 320, H = 140, PAD = 10
      const vals = data.map((d) => d.v)
      const min = Math.min(...vals), max = Math.max(...vals)
      const span = max - min || 1
      const ctx = uni.createCanvasContext('alertTrendCanvas', this)
      ctx.setStrokeStyle('#eef2f8')
      ctx.setLineWidth(1)
      for (let i = 0; i <= 4; i++) {
        const y = PAD + ((H - 2 * PAD) * i) / 4
        ctx.beginPath(); ctx.moveTo(PAD, y); ctx.lineTo(W - PAD, y); ctx.stroke()
      }
      ctx.setStrokeStyle('#1f4e9c')
      ctx.setLineWidth(2)
      ctx.beginPath()
      data.forEach((d, i) => {
        const x = PAD + ((W - 2 * PAD) * i) / (data.length - 1 || 1)
        const y = H - PAD - ((d.v - min) / span) * (H - 2 * PAD)
        if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y)
      })
      ctx.stroke()
      ctx.setFillStyle('#8a94a6')
      ctx.setFontSize(9)
      ctx.fillText(String(max.toFixed(1)), PAD, PAD - 1)
      ctx.fillText(String(min.toFixed(1)), PAD, H - 2)
      ctx.draw()
    },
    async loadCases() {
      try {
        const d = await get('/fault-cases/recommend', { alert_id: this.id })
        this.cases = (d.items || d || []).map((x) => ({ ...x, expanded: false }))
      } catch (e) { /* ignore */ }
    },
    // M6-D：一键转工单（预填告警上下文）
    toWo() {
      const a = this.a
      const q = [
        'site_id=' + (a.site_id || 0),
        'type=alarm',
        'priority=' + (a.severity === 3 ? 1 : a.severity === 2 ? 2 : 3),
        'alert_id=' + a.id,
        'title=' + encodeURIComponent('告警处理：' + (a.point_name ? a.point_name + ' ' : '') + (a.message || '')),
        'description=' + encodeURIComponent('告警 #' + a.id + '（' + this.sevText(a.severity) + '）\n' + (a.message || '') + (this.detailText ? '\n' + this.detailText : '')),
      ].join('&')
      uni.navigateTo({ url: '/pages/wo/create?' + q })
    },
    async act(action) {
      let reason = ''
      if (action === 'resolve' || action === 'suspend') {
        const r = await new Promise((res) => {
          uni.showModal({ title: action === 'resolve' ? '结束告警' : '挂起告警', editable: true, placeholderText: '备注原因（可选）', success: res, fail: () => res({ confirm: false }) })
        })
        if (!r.confirm) return
        reason = r.content || ''
      }
      try {
        await post(`/alerts/${this.id}/action`, { action, reason })
        uni.showToast({ title: '操作成功', icon: 'success' })
        // 刷新状态
        this.a = { ...this.a }
        if (action === 'acknowledge') { this.a.status = 2; this.a.watched = this.a.watched }
        if (action === 'suspend') this.a.status = 1
        if (action === 'resume') this.a.status = 0
        if (action === 'resolve') this.a.status = 3
        if (action === 'watch') this.a.watched = true
        if (action === 'unwatch') this.a.watched = false
        uni.setStorageSync('dcim_alert_cur', this.a)
        this.loadLogs()
      } catch (e) { /* toast 已弹 */ }
    },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 40rpx; }
.head .msg { font-size: 32rpx; font-weight: 700; margin: 16rpx 0 12rpx; line-height: 1.45; }
.meta view { margin-top: 6rpx; }
.meta-line { display: flex; align-items: center; margin-top: 6rpx; }
.meta-line .tech-icon { margin-right: 10rpx; }
.sev-tag { color: #fff; font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 8rpx; margin-right: 12rpx; }
.st-tag { font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 8rpx; background: #eef2f8; color: #4b5563; }
.st0 { background: #fff1f0; color: #e5484d; }
.st2 { background: #fff7e6; color: #d48806; }
.st3 { background: #eef9f0; color: #2fa84f; }
.detail-json { margin-top: 14rpx; background: #f4f6fa; border-radius: 12rpx; padding: 14rpx 18rpx; font-size: 24rpx; color: #4b5563; }
.sec-title { font-size: 30rpx; font-weight: 700; margin: 20rpx 4rpx 14rpx; }
.case .fc-code { color: #1f4e9c; font-size: 24rpx; font-weight: 700; }
.score { color: #f5a623; font-size: 22rpx; }
.fc-title { font-size: 28rpx; font-weight: 600; margin: 10rpx 0; }
.fc-sec { font-size: 26rpx; line-height: 1.6; color: #4b5563; }
.fc-sec .k { color: #1f4e9c; font-weight: 600; }
.fc-toggle { color: #1f4e9c; font-size: 24rpx; margin-top: 12rpx; }
.ops { display: flex; flex-wrap: wrap; }
.op { flex: 1; min-width: 260rpx; margin: 8rpx; font-size: 28rpx; padding: 0; line-height: 2.6; }
.op.danger { color: #e5484d; border-color: #e5484d; }
.op-note { text-align: center; margin-top: 10rpx; }
.log-item { display: flex; padding: 14rpx 0; border-bottom: 1rpx solid #f0f3f8; }
.log-item:last-child { border-bottom: none; }
.dot { width: 14rpx; height: 14rpx; border-radius: 50%; background: #1f4e9c; margin: 12rpx 18rpx 0 4rpx; flex-shrink: 0; }
/* ---- M6-E3 趋势图 ---- */
.trend-card { padding: 20rpx; }
.trend-canvas { width: 100%; height: 280rpx; }
.empty-tip.sm { font-size: 24rpx; padding: 16rpx 0; }
.page { padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 170rpx) !important; }
</style>
