<template>
  <view class="page" :style="{ paddingTop: navH + 'px' }">
    <!-- 液态玻璃导航栏 -->
    <GlassNavBar :scrollY="scrollY" :dark="scrollY < 120">
      <template #left>
        <view class="site-pick" @tap="pickSite">
          <TechIcon name="layers" :color="scrollY < 120 ? '#DCE9FF' : '#1F4E9C'" :size="30" />
          <text class="site-t" :style="{ color: scrollY < 120 ? '#F2F6FF' : '#16233D' }">{{ currentSiteName }}</text>
          <text class="caret" :style="{ color: scrollY < 120 ? '#9FB8E0' : '#8A94A6' }">▾</text>
        </view>
      </template>
      <template #right>
        <view class="bell-wrap" @tap="goNotify">
          <TechIcon name="bell" :color="scrollY < 120 ? '#DCE9FF' : '#1F4E9C'" :size="38" />
          <text v-if="ov.unread > 0" class="badge">{{ ov.unread > 99 ? '99+' : ov.unread }}</text>
        </view>
      </template>
    </GlassNavBar>

    <!-- 封面氛围光（主色随站点插值过渡） -->
    <view class="hero" :style="depthStyle(scrollY, 0.55)">
      <AtmosphereGlow :seed="currentSiteName" base="#1F4E9C" :scrollY="scrollY" />
      <view class="hero-content">
        <view class="hero-hi">DCIM · {{ greeting }}</view>
        <view class="hero-site">{{ currentSiteName }}</view>
        <view class="hero-sub">
          <text class="hs-chip" :class="{ warn: ov.alerts.total > 0 }">
            {{ ov.alerts.total > 0 ? ov.alerts.total + ' 条活跃告警' : '运行平稳 · 无活跃告警' }}
          </text>
          <text class="hs-chip" v-if="ov.duty && ov.duty.on_duty">当班 {{ ov.duty.user_name }}</text>
        </view>
      </view>
    </view>

    <!-- KPI 宫格（景深中层） -->
    <view class="mid-layer" :style="depthStyle(scrollY, 0.82)">
      <view class="kpi-grid">
        <view class="kpi glass-card" @tap="goAlert">
          <view class="kpi-head">
            <TechIcon name="alert" :color="ov.alerts.total > 0 ? '#E5484D' : '#2FA84F'" :size="34" />
            <text class="kpi-label">活跃告警</text>
          </view>
          <view class="kpi-num" :class="ov.alerts.total > 0 ? 'red' : 'green'">{{ ov.alerts.total }}</view>
          <view class="kpi-sub"><text class="sub-link" @tap.stop="goAlertSev(3)">严重 {{ sev(3) }}</text> · <text class="sub-link" @tap.stop="goAlertSev(2)">警告 {{ sev(2) }}</text></view>
        </view>
        <view class="kpi glass-card" @tap="goWo">
          <view class="kpi-head">
            <TechIcon name="ticket" color="#2D6BC4" :size="34" />
            <text class="kpi-label">待我处理</text>
          </view>
          <view class="kpi-num" :class="ov.work_orders.my_todo_breach > 0 ? 'red' : 'accent-c'">{{ ov.work_orders.my_todo }}</view>
          <view class="kpi-sub breach" v-if="ov.work_orders.my_todo_breach > 0"><TechIcon name="alert" color="#E5484D" :size="24" /> {{ ov.work_orders.my_todo_breach }} 单违约</view>
          <view class="kpi-sub" v-else>全站未闭环 {{ ov.work_orders.open_total }}</view>
        </view>
        <view class="kpi glass-card" @tap="goScan">
          <view class="kpi-head">
            <TechIcon name="scan" color="#2FA84F" :size="34" />
            <text class="kpi-label">今日巡检</text>
          </view>
          <view class="kpi-num accent-c">{{ ov.inspection.done_today }}<text class="of">/{{ ov.inspection.plan_total }}</text></view>
          <view class="kpi-sub">{{ inspDone ? '已完成' : '待执行' }}</view>
        </view>
        <view class="kpi glass-card" @tap="goMonitor">
          <view class="kpi-head">
            <TechIcon name="cpu" :color="ov.points.online_rate < 90 ? '#F5A623' : '#2FA84F'" :size="34" />
            <text class="kpi-label">测点在线率</text>
          </view>
          <view class="kpi-num" :class="ov.points.online_rate < 90 ? 'orange' : 'green'">{{ ov.points.online_rate }}%</view>
          <view class="kpi-sub">{{ ov.points.online }}/{{ ov.points.total }} 在线</view>
        </view>
      </view>
    </view>

    <!-- 设备概览（M6：数量/状态/分类/机柜占用，全可穿透） -->
    <view class="sec-title" @tap="goAssets('')"><TechIcon name="server" color="#16233D" :size="30" /> 设备概览 <text class="muted link-t">资产速查 ›</text></view>
    <view class="glass-card dev-card">
      <view class="dev-top">
        <view class="dev-total">
          <view class="dev-n">{{ assets.total }}</view>
          <view class="muted">设备总数</view>
        </view>
        <view class="dev-cabs">
          <view class="dev-n sm">{{ assets.cabinet_used }}<text class="of">/{{ assets.cabinet_total }}</text></view>
          <view class="muted">机柜在用</view>
        </view>
      </view>
      <!-- 状态分布条（穿透到资产页带过滤） -->
      <view class="dev-bar">
        <view v-for="s in statusSegs" :key="s.k" class="dev-seg" :style="{ width: s.pct + '%', background: s.color }" @tap.stop="goAssets(s.k)"></view>
      </view>
      <view class="dev-legend">
        <view class="lg-item" v-for="s in statusSegs" :key="'l' + s.k" @tap="goAssets(s.k)">
          <view class="lg-dot" :style="{ background: s.color }"></view>
          <text>{{ s.t }}</text><text class="lg-n">{{ s.n }}</text>
        </view>
      </view>
      <!-- 分类分布（穿透） -->
      <view class="dev-cats" v-if="catSegs.length">
        <view class="cat-chip" v-for="c in catSegs" :key="c.k" @tap="goAssetsCat(c.k)">
          <TechIcon :name="catIcon(c.k)" color="#2D6BC4" :size="26" />
          <text class="cat-t">{{ catText(c.k) }}</text>
          <text class="cat-n">{{ c.n }}</text>
        </view>
      </view>
    </view>

    <!-- 今日巡检计划（M6-E2） -->
    <template v-if="todayPlans.length">
      <view class="sec-title" @tap="goScan"><TechIcon name="scan" color="#16233D" :size="30" /> 今日巡检 <text class="muted link-t">去扫码 ›</text></view>
      <view class="glass-card plan-card">
        <view class="plan-row" v-for="p in todayPlans" :key="p.id" :class="{ done: p.done }">
          <view class="plan-chk">
            <TechIcon :name="p.done ? 'check-circle' : 'clock'" :color="p.done ? '#2FA84F' : '#F5A623'" :size="34" />
          </view>
          <view class="grow">
            <view class="plan-name">{{ p.name }}</view>
            <view class="muted plan-freq">{{ freqText(p.freq) }} · {{ p.done ? '已执行' : '待巡检' }}</view>
          </view>
          <text v-if="p.done" class="plan-tag ok">完成</text>
          <text v-else class="plan-tag todo">待办</text>
        </view>
      </view>
    </template>

    <!-- 备件低库存预警（M6-E4） -->
    <template v-if="spareWarn.low_count > 0">
      <view class="sec-title" @tap="goMaint"><TechIcon name="box" color="#16233D" :size="30" /> 备件低库存 <text class="spare-badge">{{ spareWarn.low_count }}</text></view>
      <view class="glass-card spare-card">
        <view class="spare-row" v-for="s in spareWarn.top" :key="s.id" @tap="goMaint">
          <view class="grow">
            <view class="spare-name">{{ s.name }}</view>
            <view class="muted mono">{{ s.code }}</view>
          </view>
          <view class="spare-stock">
            <text class="st-n" :class="{ zero: s.stock <= 0 }">{{ s.stock }}</text>
            <text class="muted st-safe">/ 安全 {{ s.safety_stock }}</text>
          </view>
        </view>
        <view class="spare-more muted" v-if="spareWarn.low_count > spareWarn.top.length">另有 {{ spareWarn.low_count - spareWarn.top.length }} 项低库存</view>
      </view>
    </template>

    <!-- 健康与采购（M7-V77）：数字可穿透到过滤列表 -->
    <template v-if="!isViewer()">
      <view class="sec-title"><TechIcon name="gauge" color="#16233D" :size="30" /> 健康与采购</view>
      <view class="hp-row">
        <view class="glass-card hp-card" @tap="goHealth()">
          <view class="hp-t"><TechIcon name="shield" color="#2FA84F" :size="26" /> 资产健康</view>
          <view class="hp-num" :class="hpCls()">{{ healthSum.total ? healthSum.avg_score : '--' }}</view>
          <view class="hp-sub muted">平均健康分 · {{ healthSum.total || 0 }} 台</view>
          <view class="hp-chips">
            <text class="hp-chip ok" @tap.stop="goHealth('healthy')">健康 {{ gradeN('healthy') }}</text>
            <text class="hp-chip warn" v-if="gradeN('attention') + gradeN('sub_healthy')" @tap.stop="goHealth('attention')">关注 {{ gradeN('attention') + gradeN('sub_healthy') }}</text>
            <text class="hp-chip bad" v-if="gradeN('risk')" @tap.stop="goHealth('risk')">风险 {{ gradeN('risk') }}</text>
          </view>
        </view>
        <view class="glass-card hp-card" @tap="goPurchase()">
          <view class="hp-t"><TechIcon name="box" color="#1F4E9C" :size="26" /> 采购审批</view>
          <view class="hp-num" :class="{ orange: prPending.total > 0 }">{{ prPending.total }}</view>
          <view class="hp-sub muted">待审批申请</view>
          <view class="hp-chips">
            <text class="hp-chip link" v-if="prPending.latest">{{ prPending.latest.code }} ›</text>
            <text class="hp-chip" v-else>暂无待办</text>
          </view>
        </view>
      </view>
    </template>

    <!-- 液滴粘连扫码按钮 -->
    <view class="scan-zone glass-card">
      <LiquidDrag icon="scan" origin-icon="pin" :w="liquidW" :h="150" :threshold="110"
        @break="onLiquidBreak" @reset="onLiquidReset" />
      <view class="scan-hint">{{ liquidHint }}</view>
    </view>

    <!-- 待办工单横滑 -->
    <template v-if="ov.work_orders.my_list && ov.work_orders.my_list.length">
      <view class="sec-title"><TechIcon name="ticket" color="#16233D" :size="30" /> 待办工单 <text class="muted">({{ ov.work_orders.my_todo }})</text></view>
      <scroll-view scroll-x class="wo-scroll">
        <view class="wo-card glass-card" v-for="w in ov.work_orders.my_list" :key="w.id" @tap="goWoDetail(w.id)">
          <view class="wo-code">{{ w.code }}</view>
          <view class="wo-title">{{ w.title }}</view>
          <view class="wo-foot">
            <text class="tag" :class="prioCls(w.priority)">{{ prioText(w.priority) }}</text>
            <text class="sla" :class="w.sla_breach ? 'breach' : ''">{{ w.sla_breach ? '已违约' : slaLeft(w.sla_resolve_due) }}</text>
          </view>
        </view>
      </scroll-view>
    </template>

    <!-- 最新告警：圆柱卷收列表 -->
    <view class="sec-title"><TechIcon name="alert" color="#16233D" :size="30" /> 最新告警</view>
    <view class="cyl-viewport">
      <view v-if="!ov.alerts.latest || !ov.alerts.latest.length" class="glass-card empty-glass">
        <TechIcon name="check" color="#2FA84F" :size="52" />
        <view class="empty-t">运行平稳 · 无活跃告警</view>
      </view>
      <view class="cyl-item" v-for="(a, i) in ov.alerts.latest" :key="a.id"
        :style="cylItemStyle(i)" @tap="goAlertDetail(a)">
        <view class="glass-card alert-row">
          <view class="sev-chip" :style="{ background: sevColor(a.severity) }">
            <TechIcon name="alert" color="#fff" :size="26" />
          </view>
          <view class="grow alert-body">
            <view class="alert-msg">{{ a.point_name ? a.point_name + ' · ' : '' }}{{ a.message }}</view>
            <view class="muted a-sub">
              <text v-if="a.storm_id" class="storm">风暴</text>
              {{ timeAgo(a.triggered_at) }} · {{ sevText(a.severity) }}
            </view>
          </view>
          <view class="sev-bar-v" :style="{ background: sevColor(a.severity) }"></view>
        </view>
      </view>
    </view>

    <!-- PUE 卡 -->
    <view class="glass-card pue-card">
      <view class="row">
        <TechIcon name="gauge" color="#1F4E9C" :size="44" />
        <view class="grow pue-mid">
          <view class="muted">最新 PUE</view>
          <view class="pue-num" :class="pueCls">{{ ov.pue.pue ? ov.pue.pue.toFixed(3) : '—' }}</view>
        </view>
        <view class="pue-right">
          <view class="muted">总功率</view>
          <view class="pue-kw">{{ ov.pue.total_power ? ov.pue.total_power.toFixed(1) : '0' }} kW</view>
          <view class="muted" v-if="ov.pue.recorded_at">{{ timeAgo(ov.pue.recorded_at) }}</view>
        </view>
      </view>
    </view>

    <view class="foot-time" v-if="ov.generated_at">数据 {{ timeAgo(ov.generated_at) }} · 下拉刷新</view>

    <!-- 自定义液态玻璃 TabBar -->
    <GlassTabBar current="index" :scrollY="scrollY" />
  </view>
</template>

<script>
import { get } from '../../api.js'
import { requireAuth, user, timeAgo, slaLeft, SEV_TEXT, WO_PRIORITY, SHIFT_TEXT, doScan } from '../../util.js'
import { cylStyle, depthStyle } from '../../fx.js'
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import AtmosphereGlow from '../../components/AtmosphereGlow.vue'
import LiquidDrag from '../../components/LiquidDrag.vue'
import TechIcon from '../../components/TechIcon.vue'

export default {
  components: { GlassNavBar, GlassTabBar, AtmosphereGlow, LiquidDrag, TechIcon },
  data() {
    return {
      siteId: 0,
      scrollY: 0,
      navH: 44,
      liquidW: 320,
      liquidHint: '拖出水滴至断裂，或直接点击图标 → 进入扫码作业',
      ov: {
        alerts: { total: 0, by_severity: {}, storm_count: 0, latest: [] },
        work_orders: { my_todo: 0, my_todo_breach: 0, open_total: 0, my_list: [] },
        inspection: { plan_total: 0, done_today: 0 },
        duty: { on_duty: false },
        points: { total: 0, online: 0, online_rate: 0 },
        pue: { pue: 0, total_power: 0 },
        unread: 0,
        sites: [],
      },
      assets: { total: 0, rack_on: 0, rack_off: 0, repair: 0, scrapped: 0, by_category: [], cabinet_total: 0, cabinet_used: 0 },
      todayPlans: [],
      spareWarn: { low_count: 0, top: [] },
      healthSum: { avg_score: 0, total: 0, grades: {}, open_work_orders: 0 },
      prPending: { total: 0, latest: null },
    }
  },
  computed: {
    siteNames() {
      return ['全部站点', ...this.ov.sites.map((s) => s.name)]
    },
    currentSiteName() {
      if (!this.siteId) return '全部站点'
      const s = this.ov.sites.find((x) => x.id === this.siteId)
      return s ? s.name : '全部站点'
    },
    inspDone() {
      return this.ov.inspection.plan_total > 0 && this.ov.inspection.done_today >= this.ov.inspection.plan_total
    },
    pueCls() {
      const p = this.ov.pue.pue || 0
      return p > 1.6 ? 'red' : p > 1.5 ? 'orange' : 'green'
    },
    greeting() {
      const h = new Date().getHours()
      return h < 6 ? '凌晨值守' : h < 12 ? '上午好' : h < 18 ? '下午好' : '晚上好'
    },
    // M6 设备概览：状态分段（含百分比）
    statusSegs() {
      const a = this.assets
      const defs = [
        { k: '1', t: '在架', n: a.rack_on, color: '#2FA84F' },
        { k: '2', t: '在库', n: a.rack_off, color: '#3B82F6' },
        { k: '3', t: '维修', n: a.repair, color: '#F5A623' },
        { k: '4', t: '报废', n: a.scrapped, color: '#C4CAD4' },
      ]
      const total = a.total || 1
      return defs.filter((d) => d.n > 0).map((d) => ({ ...d, pct: Math.max(4, Math.round((d.n / total) * 100)) }))
    },
    catSegs() {
      return (this.assets.by_category || []).slice(0, 6).map((c) => ({ k: c.category, n: c.count }))
    },
  },
  onLoad() {
    try {
      const sys = uni.getSystemInfoSync()
      const sb = sys.statusBarHeight || 0
      this.navH = sb + 44
      this.liquidW = Math.max(280, Math.min(360, (sys.windowWidth || 375) - 60))
      this.winH = sys.windowHeight || 700
    } catch (e) { /* defaults */ }
  },
  onShow() {
    if (!requireAuth()) return
    this.load()
    // M6-D: 30s 自动轮询（值班盯屏场景，页面隐藏时暂停）
    this.stopPoll()
    this._poll = setInterval(() => {
      // #ifdef H5
      if (document.hidden) return
      // #endif
      this.load()
    }, 30000)
  },
  onHide() { this.stopPoll() },
  onUnload() { this.stopPoll() },
  onPageScroll(e) {
    this.scrollY = e.scrollTop
  },
  onPullDownRefresh() {
    this.load().finally(() => uni.stopPullDownRefresh())
  },
  methods: {
    timeAgo, slaLeft, depthStyle,
    sevText: (s) => SEV_TEXT[s] || '',
    freqText(f) { return { daily: '每日', weekly: '每周', monthly: '每月', quarterly: '每季度' }[f] || (f || '周期') },
    goMaint() { uni.navigateTo({ url: '/pages/maint/maint?tab=spare&low=1' }) },
    prioText: (p) => WO_PRIORITY[p] || '',
    prioCls(p) { return ['p-low', 'p-urgent', 'p-high', 'p-mid', 'p-low'][p] || 'p-low' },
    sev(n) {
      const b = this.ov.alerts.by_severity || {}
      return b[n] || b[String(n)] || 0
    },
    sevColor(s) { return { 1: '#3B82F6', 2: '#F5A623', 3: '#E5484D' }[s] || '#8a94a6' },
    // 圆柱卷收：按列表项序号+滚动进度计算形变（列表顶部固定视区，itemH≈150rpx≈75px）
    cylItemStyle(i) {
      const itemH = 82 // px 估高
      const listTop = this.listTopBase()
      return cylStyle(listTop + i * itemH, itemH, this.scrollY, this.winH || 700)
    },
    listTopBase() {
      // 告警列表在页面中的大致偏移（hero+kpi+scan区），随内容变化取固定估值
      return this._alertListTop || 620
    },
    async load() {
      try {
        const d = await get('/mobile/overview', { site_id: this.siteId })
        this.ov = d
        if (d.assets) this.assets = d.assets
        this.todayPlans = d.today_plans || []
        if (d.spare_warn) this.spareWarn = d.spare_warn
      } catch (e) { /* toast 已弹 */ }
      // M7-V77 健康与采购聚合（viewer 无权限时静默跳过）
      if (!this.isViewer()) {
        const [hs, pr] = await Promise.all([
          get('/asset-health/summary', { site_id: this.siteId }, { silent: true }).catch(() => null),
          get('/purchase-requests', { status: 1, page: 1, size: 1 }, { silent: true }).catch(() => null),
        ])
        if (hs) this.healthSum = hs
        if (pr) this.prPending = { total: pr.total || 0, latest: (pr.list || [])[0] || null }
      }
      // 精确测告警列表位置（H5/小程序均支持）
      this.$nextTick(() => {
        const q = uni.createSelectorQuery().in(this)
        q.select('.cyl-viewport').boundingClientRect((rect) => {
          if (rect && rect.top != null) {
            this._alertListTop = rect.top + this.scrollY
          }
        }).exec()
      })
    },
    pickSite() {
      uni.showActionSheet({
        itemList: this.siteNames,
        success: (r) => {
          const idx = r.tapIndex
          this.siteId = idx === 0 ? 0 : this.ov.sites[idx - 1].id
          this.load()
        },
        fail: () => {},
      })
    },
    onLiquidBreak() {
      this.liquidHint = '水滴已断开 → 进入扫码作业'
      setTimeout(() => {
        this.liquidHint = '拖出水滴至断裂，或直接点击图标 → 进入扫码作业'
        doScan()
      }, 450)
    },
    onLiquidReset() {
      this.liquidHint = '拖出水滴至断裂，或直接点击图标 → 进入扫码作业'
    },
    goNotify() { uni.navigateTo({ url: '/pages/notify/notify' }) },
    goAlert() { uni.reLaunch({ url: '/pages/alert/alert' }) },
    goAlertSev(n) { uni.reLaunch({ url: '/pages/alert/alert?sev=' + n }) },
    goWo() { uni.reLaunch({ url: '/pages/wo/wo' }) },
    goScan() { doScan() },
    goMonitor() { uni.navigateTo({ url: '/pages/monitor/monitor' }) },
    // M6 穿透：设备概览 → 资产速查（带状态过滤）
    goAssets(status) {
      const q = status ? '?status=' + status : ''
      uni.navigateTo({ url: '/pages/asset/assets' + q })
    },
    goAssetsCat(cat) {
      uni.navigateTo({ url: '/pages/asset/assets?category=' + cat })
    },
    // M7-V77 健康与采购穿透
    isViewer() { return user().role === 'viewer' },
    gradeN(k) { return (this.healthSum.grades || {})[k] || 0 },
    hpCls() { const v = this.healthSum.avg_score || 0; return v >= 85 ? 'green' : v >= 55 ? 'orange' : 'red' },
    goHealth(grade) { uni.navigateTo({ url: '/pages/health/list' + (grade ? '?grade=' + grade : ''), fail: () => {} }) },
    goPurchase() { uni.navigateTo({ url: '/pages/purchase/list?status=1', fail: () => {} }) },
    catText(c) {
      return { server: '服务器', network: '网络设备', storage: '存储', power: '供配电', security: '安防', other: '其他' }[c] || c
    },
    catIcon(c) {
      return { server: 'server', network: 'globe', storage: 'box', power: 'bolt', security: 'shield', other: 'cpu' }[c] || 'box'
    },
    goAlertDetail(a) {
      uni.setStorageSync('dcim_alert_cur', a)
      uni.navigateTo({ url: '/pages/alert/detail?id=' + a.id })
    },
    goWoDetail(id) { uni.navigateTo({ url: '/pages/wo/detail?id=' + id }) },
    stopPoll() {
      if (this._poll) { clearInterval(this._poll); this._poll = null }
    },
  },
}
</script>

<style scoped>
.page {
  padding: 0 24rpx calc(env(safe-area-inset-bottom, 0px) + 150rpx);
  min-height: 100vh;
  background: linear-gradient(180deg, #0E2247 0%, #16335F 26%, #F4F6FA 46%);
  overflow-x: hidden;
}
/* ---- 封面氛围区 ---- */
.hero { position: relative; padding: 40rpx 12rpx 110rpx; overflow: hidden; }
.hero-content { position: relative; z-index: 1; }
.hero-hi { color: rgba(220, 233, 255, 0.75); font-size: 26rpx; letter-spacing: 2rpx; }
.hero-site { color: #fff; font-size: 52rpx; font-weight: 800; margin: 10rpx 0 18rpx; letter-spacing: 1rpx; text-shadow: 0 4rpx 24rpx rgba(0, 0, 0, 0.25); }
.hero-sub { display: flex; flex-wrap: wrap; }
.hs-chip {
  font-size: 23rpx; color: #DCE9FF;
  background: rgba(255, 255, 255, 0.12);
  border: 1rpx solid rgba(255, 255, 255, 0.18);
  backdrop-filter: blur(10px);
  border-radius: 30rpx; padding: 8rpx 22rpx; margin: 0 14rpx 10rpx 0;
}
.hs-chip.warn { background: rgba(229, 72, 77, 0.30); border-color: rgba(255, 120, 120, 0.4); }
/* ---- 玻璃卡片体系 ---- */
.glass-card {
  background: rgba(255, 255, 255, 0.82);
  backdrop-filter: blur(18px) saturate(1.4);
  -webkit-backdrop-filter: blur(18px) saturate(1.4);
  border: 1rpx solid rgba(255, 255, 255, 0.65);
  border-radius: 24rpx;
  box-shadow: 0 8rpx 32rpx rgba(18, 44, 92, 0.10);
  padding: 24rpx;
  margin-bottom: 20rpx;
}
.mid-layer { position: relative; margin-top: -70rpx; z-index: 2; }
/* ---- KPI ---- */
.kpi-grid { display: flex; flex-wrap: wrap; justify-content: space-between; }
.kpi { width: 48.5%; box-sizing: border-box; padding: 26rpx 24rpx; }
.kpi-head { display: flex; align-items: center; margin-bottom: 12rpx; }
.kpi-label { font-size: 25rpx; color: #4b5563; margin-left: 12rpx; font-weight: 600; }
.kpi-num { font-size: 54rpx; font-weight: 800; color: #16233D; line-height: 1.1; }
.kpi-num .of { font-size: 26rpx; color: #8a94a6; font-weight: 400; }
.kpi-num.red { color: #e5484d; }
.kpi-num.orange { color: #f5a623; }
.kpi-num.green { color: #2fa84f; }
.kpi-num.accent-c { color: #1f4e9c; }
.kpi-sub { font-size: 22rpx; color: #8a94a6; margin-top: 6rpx; }
.kpi-sub .sub-link { color: #4b5563; text-decoration: underline; text-underline-offset: 4rpx; }
.kpi-sub.breach { display: flex; align-items: center; gap: 6rpx; color: #e5484d; }
/* ---- 液滴扫码 ---- */
/* ---- M7-V77 健康与采购 ---- */
.hp-row { display: flex; gap: 16rpx; }
.hp-card { flex: 1; padding: 22rpx 22rpx 18rpx; min-width: 0; }
.hp-t { font-size: 26rpx; font-weight: 700; color: #1f2937; display: flex; align-items: center; gap: 8rpx; }
.hp-num { font-size: 56rpx; font-weight: 800; color: #1f2937; line-height: 1.2; margin-top: 8rpx; }
.hp-num.green { color: #2fa84f; }
.hp-num.orange { color: #f5a623; }
.hp-num.red { color: #e5484d; }
.hp-sub { font-size: 22rpx; margin-bottom: 12rpx; }
.hp-chips { display: flex; flex-wrap: wrap; gap: 8rpx; }
.hp-chip { font-size: 20rpx; padding: 4rpx 12rpx; border-radius: 8rpx; background: #f3f4f6; color: #6b7280; }
.hp-chip.ok { background: #eef9f0; color: #2fa84f; }
.hp-chip.warn { background: #fff7e6; color: #d48806; }
.hp-chip.bad { background: #fff1f0; color: #e5484d; }
.hp-chip.link { background: #eef4ff; color: #1f4e9c; font-family: monospace; }
.scan-zone { padding: 20rpx 24rpx 24rpx; }
.scan-hint { text-align: center; font-size: 23rpx; color: #8a94a6; margin-top: 4rpx; }
/* ---- 导航栏内 ---- */
.site-pick { display: flex; align-items: center; }
.site-t { font-size: 30rpx; font-weight: 700; margin-left: 10rpx; max-width: 300rpx; overflow: hidden; white-space: nowrap; }
.caret { font-size: 22rpx; margin-left: 6rpx; }
.bell-wrap { position: relative; padding: 8rpx; }
.badge {
  position: absolute; top: -4rpx; right: -12rpx;
  background: #e5484d; color: #fff; font-size: 18rpx;
  border-radius: 20rpx; padding: 2rpx 10rpx; line-height: 1.4;
}
/* ---- 工单横滑 ---- */
.sec-title { display: flex; align-items: center; font-size: 30rpx; font-weight: 700; color: #16233D; margin: 22rpx 4rpx 16rpx; }
.sec-title .muted { font-weight: 400; margin-left: 6rpx; }
.wo-scroll { white-space: nowrap; }
.wo-card { display: inline-block; width: 420rpx; vertical-align: top; margin-right: 20rpx; white-space: normal; }
.wo-code { font-size: 22rpx; color: #8a94a6; font-family: monospace; }
.wo-title { font-size: 28rpx; font-weight: 600; margin: 8rpx 0 16rpx; line-height: 1.4; color: #16233D; }
.wo-foot { display: flex; align-items: center; justify-content: space-between; }
.sla { font-size: 22rpx; color: #2fa84f; }
.sla.breach { color: #e5484d; font-weight: 700; }
.tag { font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 8rpx; color: #fff; }
.p-urgent { background: #e5484d; }
.p-high { background: #f5a623; }
.p-mid { background: #3b82f6; }
.p-low { background: #8a94a6; }
/* ---- 圆柱卷收列表 ---- */
.cyl-viewport { perspective: 900px; }
.cyl-item { transform-style: preserve-3d; margin-bottom: 16rpx; transition: opacity 0.1s linear; }
.alert-row { display: flex; align-items: center; padding: 22rpx 24rpx; margin-bottom: 0; }
.sev-chip { width: 56rpx; height: 56rpx; border-radius: 16rpx; display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
.alert-body { padding: 0 18rpx; }
.alert-msg { font-size: 27rpx; line-height: 1.45; color: #16233D; }
.a-sub { margin-top: 6rpx; font-size: 22rpx; }
.storm { color: #7c3aed; border: 1rpx solid #7c3aed; border-radius: 6rpx; padding: 0 8rpx; margin-right: 8rpx; font-size: 20rpx; }
.sev-bar-v { width: 8rpx; align-self: stretch; border-radius: 4rpx; flex-shrink: 0; }
.empty-glass { text-align: center; padding: 44rpx 0; }
.empty-t { color: #2fa84f; font-size: 27rpx; margin-top: 14rpx; font-weight: 600; }
/* ---- 设备概览（M6） ---- */
.link-t { color: #2d6bc4; font-size: 24rpx; margin-left: auto; }
/* ---- 今日巡检（M6-E2） ---- */
.plan-card { padding: 8rpx 24rpx; }
.plan-row { display: flex; align-items: center; padding: 18rpx 0; border-bottom: 1rpx solid #f0f3f8; }
.plan-row:last-child { border-bottom: none; }
.plan-chk { margin-right: 16rpx; display: flex; }
.plan-name { font-size: 28rpx; font-weight: 600; color: #16233d; }
.plan-row.done .plan-name { color: #6b7686; font-weight: 500; }
.plan-freq { font-size: 22rpx; margin-top: 4rpx; }
.plan-tag { font-size: 21rpx; padding: 4rpx 16rpx; border-radius: 8rpx; flex-shrink: 0; }
.plan-tag.ok { background: #eef9f0; color: #2fa84f; }
.plan-tag.todo { background: #fff7e6; color: #d48806; }
/* ---- 备件低库存（M6-E4） ---- */
.spare-badge { margin-left: auto; background: #e5484d; color: #fff; font-size: 22rpx; padding: 2rpx 14rpx; border-radius: 20rpx; font-weight: 700; }
.spare-card { padding: 8rpx 24rpx 16rpx; }
.spare-row { display: flex; align-items: center; padding: 16rpx 0; border-bottom: 1rpx solid #f0f3f8; }
.spare-name { font-size: 27rpx; font-weight: 600; color: #16233d; }
.spare-stock { text-align: right; flex-shrink: 0; margin-left: 16rpx; }
.st-n { font-size: 32rpx; font-weight: 800; color: #f5a623; }
.st-n.zero { color: #e5484d; }
.st-safe { font-size: 21rpx; margin-left: 6rpx; }
.spare-more { font-size: 22rpx; text-align: center; padding-top: 14rpx; }
.dev-card { padding: 24rpx; }
.dev-top { display: flex; align-items: flex-end; margin-bottom: 18rpx; }
.dev-total { flex: 1; }
.dev-cabs { text-align: right; }
.dev-n { font-size: 56rpx; font-weight: 800; color: #16233d; line-height: 1.1; }
.dev-n.sm { font-size: 40rpx; color: #1f4e9c; }
.dev-n .of { font-size: 26rpx; color: #8a94a6; font-weight: 500; }
.dev-bar { display: flex; height: 18rpx; border-radius: 9rpx; overflow: hidden; background: #eef2f8; }
.dev-seg { height: 100%; }
.dev-legend { display: flex; flex-wrap: wrap; margin-top: 14rpx; }
.lg-item { display: flex; align-items: center; margin-right: 24rpx; margin-top: 6rpx; font-size: 23rpx; color: #4b5563; }
.lg-dot { width: 14rpx; height: 14rpx; border-radius: 4rpx; margin-right: 8rpx; }
.lg-n { font-weight: 700; margin-left: 6rpx; color: #16233d; }
.dev-cats { display: flex; flex-wrap: wrap; margin-top: 16rpx; }
.cat-chip { display: flex; align-items: center; background: #f4f7fc; border: 1rpx solid #e3eaf6; border-radius: 12rpx; padding: 10rpx 16rpx; margin: 0 12rpx 12rpx 0; }
.cat-t { font-size: 23rpx; color: #4b5563; margin-left: 8rpx; }
.cat-n { font-size: 24rpx; font-weight: 700; color: #1f4e9c; margin-left: 8rpx; }
/* ---- PUE ---- */
.pue-card .pue-mid { padding-left: 22rpx; }
.pue-num { font-size: 50rpx; font-weight: 800; }
.pue-num.red { color: #e5484d; }
.pue-num.orange { color: #f5a623; }
.pue-num.green { color: #2fa84f; }
.pue-right { text-align: right; }
.pue-kw { font-size: 34rpx; font-weight: 700; color: #1f4e9c; }
.foot-time { text-align: center; color: #b6bfce; font-size: 22rpx; margin: 26rpx 0 10rpx; }
.row { display: flex; align-items: center; }
.grow { flex: 1; }
.muted { color: #8a94a6; font-size: 24rpx; }
</style>
