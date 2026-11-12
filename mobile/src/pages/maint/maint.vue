<template>
  <view class="page">
    <GlassNavBar title="维保与备件" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>
    <scroll-view scroll-x class="tabs">
      <view class="tab" :class="{ active: tab === 'cov' }" @tap="tab = 'cov'">维保覆盖</view>
      <view class="tab" :class="{ active: tab === 'exp' }" @tap="switchExp">到期预警</view>
      <view class="tab" :class="{ active: tab === 'spare' }" @tap="switchSpare">备件库存</view>
    </scroll-view>

    <!-- 覆盖分析 -->
    <template v-if="tab === 'cov'">
      <view class="kpi-grid">
        <view class="kpi"><view class="kpi-num accent-c">{{ cov.coverage_pct || 0 }}%</view><view class="kpi-label">覆盖率</view></view>
        <view class="kpi"><view class="kpi-num green">{{ cov.covered || 0 }}</view><view class="kpi-label">在保</view></view>
        <view class="kpi"><view class="kpi-num orange">{{ cov.expiring || 0 }}</view><view class="kpi-label">即将到期</view></view>
        <view class="kpi"><view class="kpi-num red">{{ cov.uncovered || 0 }}</view><view class="kpi-label">脱保</view></view>
      </view>
      <view class="cov-bar"><view class="cov-fg" :style="{ width: (cov.coverage_pct || 0) + '%' }"></view></view>
      <view class="sec-title">分类覆盖（在保/总数）</view>
      <view class="card cat" v-for="(c, cat) in catStats" :key="cat">
        <view class="row"><text class="grow">{{ catText(cat) }}</text><text class="muted">{{ c.covered }}/{{ (c.covered || 0) + (c.uncovered || 0) }} · {{ covPct(c) }}%</text></view>
        <view class="cov-bar sm"><view class="cov-fg" :style="{ width: covPct(c) + '%' }"></view></view>
      </view>
      <view v-if="!Object.keys(catStats).length" class="empty-tip">暂无分类数据</view>
    </template>

    <!-- 到期预警 -->
    <template v-if="tab === 'exp'">
      <view v-if="!expiring.length" class="empty-tip">30 天内无到期维保</view>
      <view class="card exp-item" v-for="e in expiring" :key="e.maintenance.id" @tap="goAsset(e.maintenance.asset_id)">
        <view class="row">
          <text class="grow exp-name">{{ e.asset_name || '资产#' + e.maintenance.asset_id }}</text>
          <text class="days" :class="e.days_left <= 7 ? 'urgent' : ''">{{ e.days_left }} 天</text>
        </view>
        <view class="muted">{{ e.asset_code }} · {{ e.maintenance.vendor || '无供应商' }}</view>
        <view class="muted">到期 {{ String(e.maintenance.end_at || '').slice(0, 10) }}</view>
      </view>
    </template>

    <!-- 备件库存（M6-F: 可出入库） -->
    <template v-if="tab === 'spare'">
      <view class="card row stat-bar" v-if="spareStats.total_kinds !== undefined">
        <view class="sr"><view class="sr-n accent-c">{{ spareStats.total_kinds }}</view><view class="muted">备件种类</view></view>
        <view class="sr" @tap="toggleLowOnly"><view class="sr-n" :class="spareStats.low_stock > 0 ? 'red' : 'green'">{{ lowOnly ? spareCount : spareStats.low_stock }}</view><view class="muted">{{ lowOnly ? '仅看低库存(点击取消)' : '低于安全水位(点击筛选)' }}</view></view>
        <view class="sr"><view class="sr-n">¥{{ fmtMoney(spareStats.stock_value) }}</view><view class="muted">库存总值</view></view>
      </view>
      <view v-if="!spares.length" class="empty-tip">{{ lowOnly ? '无低库存备件' : '暂无备件数据' }}</view>
      <view class="card sp-item" v-for="sp in spares" :key="sp.id" @tap="openMove(sp)">
        <view class="row">
          <text class="grow sp-name">{{ sp.name }}</text>
          <text class="stock" :class="sp.stock <= sp.safety_stock ? 'low' : ''">{{ sp.stock }}{{ sp.unit }}</text>
        </view>
        <view class="muted">{{ sp.code }} · {{ sp.category }}<text v-if="sp.spec"> · {{ sp.spec }}</text></view>
        <view class="muted" v-if="sp.location">{{ sp.location }} · 安全水位 {{ sp.safety_stock }}</view>
        <view class="row sp-ops" v-if="!isViewer" @tap.stop>
          <view class="op-btn out" @tap="openMove(sp, 'out')"><TechIcon name="arrow-down" color="#1F4E9C" :size="26" /> 领用出库</view>
          <view class="op-btn in" @tap="openMove(sp, 'in')"><TechIcon name="arrow-up" color="#2FA84F" :size="26" /> 采购入库</view>
        </view>
      </view>
      <view v-if="hasMoreSpare" class="load-more" @tap="loadMoreSpare">加载更多</view>
    </template>

    <!-- M6-F 出入库抽屉 -->
    <view v-if="moveSp" class="mv-mask" @tap.self="moveSp = null">
      <view class="mv-panel">
        <view class="mv-head">
          <text class="mv-title">{{ moveSp.name }}</text>
          <text class="mv-close" @tap="moveSp = null">×</text>
        </view>
        <view class="mv-body">
          <view class="mv-stock">当前库存 <text class="mv-stock-n" :class="{ low: moveSp.stock <= moveSp.safety_stock }">{{ moveSp.stock }}{{ moveSp.unit }}</text> · 安全水位 {{ moveSp.safety_stock }}</view>
          <view class="mv-type">
            <view class="mv-type-btn" :class="{ active: mvForm.type === 'out' }" @tap="mvForm.type = 'out'">领用出库</view>
            <view class="mv-type-btn" :class="{ active: mvForm.type === 'in' }" @tap="mvForm.type = 'in'">采购入库</view>
          </view>
          <view class="mv-row">
            <text class="mv-label">数量({{ moveSp.unit }})</text>
            <view class="mv-qty">
              <view class="qty-btn" @tap="mvForm.qty = Math.max(1, mvForm.qty - 1)">−</view>
              <input class="qty-input" type="number" v-model="mvForm.qty" />
              <view class="qty-btn" @tap="mvForm.qty = mvForm.qty + 1">＋</view>
            </view>
          </view>
          <view class="mv-row col">
            <text class="mv-label">事由</text>
            <input class="mv-input" v-model="mvForm.reason" :placeholder="mvForm.type === 'out' ? '如: 更换故障风扇(工单WO-xxx)' : '如: 采购到货补库'" />
          </view>
          <view class="mv-row col" v-if="mvForm.type === 'out'">
            <text class="mv-label">关联工单ID(可选,出库写入工单流转日志)</text>
            <input class="mv-input" type="number" v-model="mvForm.woId" placeholder="工单ID" />
          </view>
          <view class="mv-sec" v-if="mvHistory.length">
            <text class="mv-label">最近出入库</text>
            <view class="mv-hist" v-for="(m, i) in mvHistory" :key="i">
              <text :class="m.type === 'in' ? 'h-in' : 'h-out'">{{ m.type === 'in' ? '入库' : '出库' }}</text>
              <text class="grow">{{ String(m.created_at || '').slice(0, 10) }} {{ m.reason || '-' }}</text>
              <text class="muted">{{ m.qty }}{{ moveSp.unit }} 余{{ m.stock_after }}</text>
            </view>
          </view>
        </view>
        <view class="mv-foot">
          <button class="mv-cancel" @tap="moveSp = null">取消</button>
          <button class="mv-ok" :class="mvForm.type" :disabled="mvSubmitting" @tap="submitMove">确认{{ mvForm.type === 'out' ? '出库' : '入库' }}</button>
        </view>
      </view>
    </view>
    <GlassTabBar current="me" :scrollY="scrollY" />
  </view>
</template>

<script>
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import TechIcon from '../../components/TechIcon.vue'
import { get, post } from '../../api.js'
import { requireAuth, user } from '../../util.js'

export default {
  components: { GlassNavBar, GlassTabBar, TechIcon },
  data() {
    return {
      scrollY: 0,
      navH: 44,
      tab: 'cov',
      cov: {},
      expiring: [],
      spares: [], sparePage: 1, spareTotal: 0, spareStats: {},
      lowOnly: false,
      moveSp: null,
      mvForm: { type: 'out', qty: 1, reason: '', woId: '' },
      mvHistory: [],
      mvSubmitting: false,
      pendingWo: '',
    }
  },
  computed: {
    hasMoreSpare() { return this.spares.length < this.spareTotal },
    catStats() { return this.cov.category_stats || {} },
    isViewer() { return (user().role || '') === 'viewer' },
    spareCount() { return this.spareStats.low_stock || 0 },
  },
  onLoad(q) {
    try { this.navH = (uni.getSystemInfoSync().statusBarHeight || 0) + 44 } catch (e) {}
    // M6-F3 深链: ?tab=spare&low=1&wo=ID
    if (q && q.tab === 'spare') {
      this.tab = 'spare'
      this.lowOnly = q.low === '1'
      if (q.wo) this.pendingWo = q.wo
    }
  },
  onPageScroll(e) { this.scrollY = e.scrollTop },
  onShow() {
    if (!requireAuth()) return
    // #ifdef H5
    try {
      const h = location.hash || ''
      const qi = h.indexOf('?')
      if (qi > -1 && h.includes('/maint/maint')) {
        const p = new URLSearchParams(h.slice(qi + 1))
        const sig = h
        if (this._deepSig !== sig) {
          this._deepSig = sig
          if (p.get('tab') === 'spare') {
            this.tab = 'spare'
            if (p.get('low') === '1' && !this.lowOnly) { this.lowOnly = true }
            if (p.get('wo')) this.pendingWo = p.get('wo')
            this.loadSpare()
            return
          }
        }
      }
    } catch (e) { /* ignore */ }
    // #endif
    this.loadCov()
  },
  onPullDownRefresh() {
    const fns = { cov: this.loadCov, exp: this.loadExpiring, spare: this.loadSpare }
    Promise.resolve(fns[this.tab]()).finally(() => uni.stopPullDownRefresh())
  },
  methods: {
    fmtMoney(v) { return v >= 10000 ? (v / 10000).toFixed(1) + '万' : String(v || 0) },
    catText(c) {
      return { server: '服务器', network: '网络设备', storage: '存储', power: '供配电', security: '安防', other: '其他' }[c] || c || '未分类'
    },
    covPct(c) {
      const t = (c.covered || 0) + (c.uncovered || 0)
      return t ? Math.round((c.covered || 0) * 100 / t) : 0
    },
    switchExp() { this.tab = 'exp'; if (!this.expiring.length) this.loadExpiring() },
    switchSpare() { this.tab = 'spare'; if (!this.spares.length) this.loadSpare() },
    async loadCov() {
      try { this.cov = await get('/maintenances/coverage', { days: 30 }) } catch (e) { /* toast */ }
    },
    async loadExpiring() {
      try {
        const d = await get('/maintenances/expiring')
        this.expiring = d.items || []
      } catch (e) { /* toast */ }
    },
    async loadSpare() {
      this.sparePage = 1
      try {
        const params = { page: 1, size: 30 }
        if (this.lowOnly) params.low_only = 1
        const d = await get('/spare-parts', params)
        this.spares = d.items || []
        this.spareTotal = d.total || 0
        if (!this.lowOnly) {
          const s = await get('/spare-parts/stats', {}, { silent: true })
          this.spareStats = s || {}
        }
      } catch (e) { /* toast */ }
    },
    toggleLowOnly() {
      this.lowOnly = !this.lowOnly
      if (this.tab !== 'spare') this.tab = 'spare'
      this.loadSpare()
    },
    async loadMoreSpare() {
      this.sparePage += 1
      try {
        const params = { page: this.sparePage, size: 30 }
        if (this.lowOnly) params.low_only = 1
        const d = await get('/spare-parts', params)
        this.spares = this.spares.concat(d.items || [])
        this.spareTotal = d.total || 0
      } catch (e) { /* toast */ }
    },
    // M6-F1 出入库抽屉
    openMove(sp, type) {
      this.moveSp = sp
      this.mvForm = { type: type || (sp.stock <= sp.safety_stock ? 'in' : 'out'), qty: 1, reason: '', woId: this.pendingWo || '' }
      this.pendingWo = ''
      this.mvHistory = []
      get(`/spare-parts/${sp.id}/movements`, {}, { silent: true }).then((d) => {
        this.mvHistory = (d.list || []).slice(0, 5)
      }).catch(() => {})
    },
    async submitMove() {
      const f = this.mvForm
      const qty = Number(f.qty)
      if (!qty || qty <= 0) { uni.showToast({ title: '数量须大于0', icon: 'none' }); return }
      if (f.type === 'out' && qty > this.moveSp.stock) {
        uni.showToast({ title: `库存不足: 现有${this.moveSp.stock}${this.moveSp.unit}`, icon: 'none' })
        return
      }
      if (f.type === 'in' && !f.reason.trim()) { uni.showToast({ title: '请填写入库事由', icon: 'none' }); return }
      this.mvSubmitting = true
      try {
        const body = { type: f.type, qty, reason: f.reason.trim() }
        if (f.type === 'out' && f.woId) body.work_order_id = Number(f.woId)
        const d = await post(`/spare-parts/${this.moveSp.id}/move`, body)
        uni.showToast({ title: f.type === 'out' ? '出库成功' : '入库成功', icon: 'success' })
        this.moveSp = null
        this.loadSpare()
      } catch (e) { /* api 已弹错 */ }
      this.mvSubmitting = false
    },
    showMoves(sp) {
      this.openMove(sp)
    },
    goAsset(id) {
      if (!id) return
      uni.navigateTo({ url: '/pages/asset/detail?id=' + id })
    },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 40rpx; }
.tabs { white-space: nowrap; margin-bottom: 18rpx; }
.tab { display: inline-block; padding: 14rpx 36rpx; margin-right: 16rpx; background: #fff; border-radius: 30rpx; font-size: 27rpx; color: #4b5563; border: 1rpx solid #e5eaf3; }
.tab.active { background: #1f4e9c; color: #fff; border-color: #1f4e9c; font-weight: 600; }
.kpi-grid { display: flex; flex-wrap: wrap; justify-content: space-between; }
.kpi { width: 48%; box-sizing: border-box; background: #fff; border-radius: 20rpx; padding: 24rpx; margin-bottom: 18rpx; text-align: center; box-shadow: 0 4rpx 16rpx rgba(31, 78, 156, 0.06); }
.kpi-num { font-size: 46rpx; font-weight: 800; line-height: 1.15; }
.kpi-num.accent-c { color: #1f4e9c; }
.kpi-num.green { color: #2fa84f; }
.kpi-num.orange { color: #f5a623; }
.kpi-num.red { color: #e5484d; }
.kpi-label { font-size: 24rpx; color: #8a94a6; margin-top: 4rpx; }
.cov-bar { height: 20rpx; background: #eef2f8; border-radius: 10rpx; overflow: hidden; margin-bottom: 8rpx; }
.cov-bar.sm { height: 12rpx; margin-top: 10rpx; }
.cov-fg { height: 100%; background: linear-gradient(90deg, #2fa84f, #5cc97f); }
.sec-title { font-size: 28rpx; font-weight: 700; margin: 18rpx 4rpx 12rpx; }
.cat { padding: 20rpx 24rpx; font-size: 27rpx; }
.exp-name { font-size: 29rpx; font-weight: 600; }
.days { font-size: 30rpx; font-weight: 800; color: #f5a623; }
.days.urgent { color: #e5484d; }
.stat-bar { justify-content: space-between; }
.sr { flex: 1; text-align: center; }
.sr-n { font-size: 36rpx; font-weight: 800; color: #1f2937; }
.sr-n.accent-c { color: #1f4e9c; }
.sr-n.red { color: #e5484d; }
.sr-n.green { color: #2fa84f; }
.sp-name { font-size: 29rpx; font-weight: 600; }
.stock { font-size: 30rpx; font-weight: 800; color: #2fa84f; }
.stock.low { color: #e5484d; }
.load-more { text-align: center; color: #1f4e9c; padding: 24rpx; font-size: 26rpx; }
/* ---- M6-F 出入库 ---- */
.sp-ops { margin-top: 14rpx; gap: 16rpx; }
.op-btn { flex: 1; display: flex; align-items: center; justify-content: center; gap: 8rpx; font-size: 25rpx; padding: 14rpx 0; border-radius: 14rpx; }
.op-btn.out { background: #eef3fc; color: #1f4e9c; }
.op-btn.in { background: #eef9f0; color: #2fa84f; }
.mv-mask { position: fixed; inset: 0; background: rgba(10, 20, 40, 0.45); z-index: 100; display: flex; align-items: flex-end; }
.mv-panel { width: 100%; background: #fff; border-radius: 28rpx 28rpx 0 0; max-height: 82vh; display: flex; flex-direction: column; }
.mv-head { display: flex; align-items: center; justify-content: space-between; padding: 28rpx 32rpx 16rpx; }
.mv-title { font-size: 32rpx; font-weight: 700; color: #16233d; }
.mv-close { font-size: 44rpx; color: #8a94a6; line-height: 1; padding: 0 8rpx; }
.mv-body { padding: 0 32rpx; overflow-y: auto; flex: 1; }
.mv-stock { font-size: 26rpx; color: #4b5563; margin-bottom: 20rpx; }
.mv-stock-n { font-size: 34rpx; font-weight: 800; color: #2fa84f; }
.mv-stock-n.low { color: #e5484d; }
.mv-type { display: flex; gap: 16rpx; margin-bottom: 22rpx; }
.mv-type-btn { flex: 1; text-align: center; padding: 18rpx 0; border-radius: 14rpx; background: #f3f6fb; color: #4b5563; font-size: 28rpx; border: 2rpx solid transparent; }
.mv-type-btn.active { background: #eef3fc; color: #1f4e9c; font-weight: 700; border-color: #1f4e9c; }
.mv-row { display: flex; align-items: center; justify-content: space-between; margin-bottom: 20rpx; }
.mv-row.col { flex-direction: column; align-items: stretch; gap: 10rpx; }
.mv-label { font-size: 26rpx; color: #6b7686; }
.mv-qty { display: flex; align-items: center; gap: 12rpx; }
.qty-btn { width: 60rpx; height: 60rpx; border-radius: 14rpx; background: #f3f6fb; color: #1f4e9c; font-size: 34rpx; display: flex; align-items: center; justify-content: center; }
.qty-input { width: 120rpx; height: 60rpx; text-align: center; border: 2rpx solid #e5eaf3; border-radius: 14rpx; font-size: 30rpx; }
.mv-input { border: 2rpx solid #e5eaf3; border-radius: 14rpx; padding: 16rpx 20rpx; font-size: 28rpx; background: #fbfcfe; }
.mv-sec { border-top: 1rpx solid #f0f3f8; padding-top: 18rpx; margin-bottom: 18rpx; }
.mv-hist { display: flex; align-items: center; gap: 12rpx; font-size: 24rpx; padding: 10rpx 0; }
.h-in { color: #2fa84f; font-weight: 700; flex-shrink: 0; }
.h-out { color: #1f4e9c; font-weight: 700; flex-shrink: 0; }
.mv-foot { display: flex; gap: 16rpx; padding: 20rpx 32rpx calc(env(safe-area-inset-bottom, 0px) + 24rpx); border-top: 1rpx solid #f0f3f8; }
.mv-cancel { flex: 1; background: #f3f6fb; color: #4b5563; font-size: 29rpx; border-radius: 16rpx; line-height: 2.6; }
.mv-ok { flex: 2; font-size: 29rpx; border-radius: 16rpx; line-height: 2.6; color: #fff; }
.mv-ok.out { background: #1f4e9c; }
.mv-ok.in { background: #2fa84f; }
.mv-ok[disabled] { opacity: 0.5; }
.page { padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 170rpx) !important; }
</style>
