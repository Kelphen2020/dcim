<template>
  <view class="page" v-if="d.asset">
    <GlassNavBar title="健康详情" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>

    <!-- 评分头卡 -->
    <view class="card head-card">
      <view class="row">
        <view class="grow">
          <view class="title">{{ d.asset.name }}</view>
          <view class="muted mono code">{{ d.asset.code }} · {{ catText(d.asset.category) }}</view>
        </view>
        <view class="score-big" :class="gradeCls(cur.grade)">{{ cur.score }}</view>
      </view>
      <view class="score-bar"><view class="score-fill" :class="gradeCls(cur.grade)" :style="{ width: (cur.score || 0) + '%' }"></view></view>
      <view class="row foot">
        <text class="tag" :class="'g-' + cur.grade">{{ cur.grade_label || gradeText(cur.grade) }}</text>
        <text class="grow"></text>
        <text class="muted time">评估 {{ timeAgo(cur.computed_at) }}</text>
      </view>
    </view>

    <!-- 扣分因子 -->
    <view class="sec-title"><TechIcon name="counter" color="#F5A623" :size="30" /> 扣分因子</view>
    <view class="card">
      <view v-if="(cur.factors || []).length">
        <view class="factor" v-for="(f, i) in cur.factors" :key="i">
          <view class="row">
            <text class="f-label">{{ f.label }}</text>
            <text class="grow"></text>
            <text class="f-deduct">-{{ f.deduct }}</text>
          </view>
          <view class="muted f-ev">{{ f.evidence }}</view>
        </view>
      </view>
      <view v-else class="empty-tip sm">无扣分项，设备状态良好</view>
    </view>

    <!-- 近14天趋势 -->
    <view class="sec-title"><TechIcon name="chart" color="#2D6BC4" :size="30" /> 评分趋势（近{{ hist.length }}天）</view>
    <view class="card">
      <view v-if="hist.length" class="hist">
        <view class="hist-col" v-for="(h, i) in hist" :key="i">
          <view class="hist-bar" :class="gradeCls(h.grade)" :style="{ height: Math.max(6, h.score * 0.9) + 'rpx' }"></view>
          <view class="hist-day muted" v-if="i % 3 === 0">{{ h.day.slice(5) }}</view>
        </view>
      </view>
      <view v-else class="empty-tip sm">暂无历史评分</view>
    </view>

    <!-- 关联告警 -->
    <template v-if="(d.alerts || []).length">
      <view class="sec-title"><TechIcon name="alert" color="#E5484D" :size="30" /> 关联活跃告警</view>
      <view class="card">
        <view class="al-row" v-for="a in d.alerts" :key="a.id">
          <text class="tag" :class="'sev' + (a.severity || 1)">{{ sevText(a.severity) }}</text>
          <text class="al-msg grow">{{ a.message || a.title || '告警#' + a.id }}</text>
          <text class="muted time">{{ timeAgo(a.triggered_at || a.created_at) }}</text>
        </view>
      </view>
    </template>

    <!-- 运维动作 -->
    <view class="sec-title"><TechIcon name="wrench" color="#1F4E9C" :size="30" /> 运维动作</view>
    <view class="card" v-if="canOps">
      <button class="btn-primary op full" @tap="makeWo">创建预测性维护工单</button>
      <view class="muted op-note">按当前扣分因子自动生成工单内容；已有在办工单时不可重复创建</view>
    </view>
    <view class="card" v-else><view class="empty-tip sm">当前角色无运维操作权限</view></view>

    <!-- 替换采购建议（V75 链路移动端入口） -->
    <view class="sec-title"><TechIcon name="swap" color="#7C3AED" :size="30" /> 替换采购建议</view>
    <view class="card" v-if="sug">
      <template v-if="sug.existing">
        <view class="warn-line">已有待审批替换申请 {{ sug.existing.code }}，请勿重复提交</view>
        <button class="btn-ghost op full mt" @tap="goPurchase">查看采购审批 ›</button>
      </template>
      <template v-else-if="sug.trigger && sug.trigger.recommended">
        <view class="ok-line">已达替换触发条件（健康分 {{ cur.score }}{{ sug.trigger.retirement_approved ? ' / 退役已批准' : ' < 60' }}），建议采购同规格设备替换</view>
        <view class="muted cand-t" v-if="(sug.candidates || []).length">同规格候选（参考）</view>
        <view class="cand" v-for="c in (sug.candidates || []).slice(0, 3)" :key="c.kind + c.id">
          <view class="grow">
            <view class="cand-name">{{ c.name }} <text class="muted mono">{{ c.code }}</text></view>
            <view class="muted cand-spec">{{ c.brand }} {{ c.model }} · {{ c.height_u }}U · {{ c.power_w }}W</view>
          </view>
          <text class="tag" :class="'g-' + c.health_grade">{{ c.health_score }}</text>
        </view>
        <button class="btn-primary op full mt" @tap="makePR">生成替换采购申请</button>
        <view class="muted op-note">将创建替换件台账与待审批采购申请，审批通过后入库</view>
      </template>
      <template v-else>
        <view class="empty-tip sm">未达替换触发条件（健康分 &lt; 60 或退役审批通过）</view>
      </template>
    </view>
    <view class="card" v-else><view class="empty-tip sm">建议加载中…</view></view>

    <GlassTabBar current="me" :scrollY="scrollY" />
  </view>
  <view v-else class="empty-tip">{{ loadErr || '加载中…' }}</view>
</template>

<script>
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import TechIcon from '../../components/TechIcon.vue'
import { get, post } from '../../api.js'
import { requireAuth, timeAgo, user, SEV_TEXT } from '../../util.js'
import { GRADE_TEXT, CAT_TEXT } from './list.vue'

export default {
  components: { GlassNavBar, GlassTabBar, TechIcon },
  data() {
    return { scrollY: 0, navH: 44, assetId: 0, d: {}, sug: null, loadErr: '' }
  },
  computed: {
    cur() { return this.d.current || {} },
    hist() { return (this.d.history || []).slice(-14) },
    canOps() { return ['global_admin', 'site_admin', 'operator'].includes(user().role) },
  },
  onLoad(q) {
    try { this.navH = (uni.getSystemInfoSync().statusBarHeight || 0) + 44 } catch (e) {}
    if (!requireAuth()) return
    this.assetId = Number(q.asset_id || 0)
    this.load()
  },
  onPageScroll(e) { this.scrollY = e.scrollTop },
  methods: {
    timeAgo,
    sevText: (s) => SEV_TEXT[s] || '提示',
    gradeText: (g) => GRADE_TEXT[g] || g,
    catText: (c) => CAT_TEXT[c] || c || '设备',
    gradeCls(g) { return { healthy: 'ok', sub_healthy: 'warn', attention: 'warn2', risk: 'bad' }[g] || '' },
    async load() {
      try {
        this.d = await get(`/asset-health/${this.assetId}`)
      } catch (e) { this.loadErr = e.message || '加载失败'; return }
      try {
        this.sug = await get(`/assets/${this.assetId}/replacement-suggestion`, {}, { silent: true })
      } catch (e) { this.sug = { trigger: null, candidates: [] } }
    },
    makeWo() {
      uni.showModal({
        title: '创建预测性维护工单',
        content: `按当前健康分 ${this.cur.score} 与扣分因子生成维护工单？`,
        success: async (r) => {
          if (!r.confirm) return
          try {
            const d = await post(`/asset-health/${this.assetId}/work-order`, {})
            uni.showToast({ title: '工单 ' + d.code + ' 已创建', icon: 'none' })
            setTimeout(() => {
              uni.navigateTo({ url: '/pages/wo/detail?id=' + d.work_order_id, fail: () => {} })
            }, 800)
          } catch (e) { /* toast 已弹 */ }
        },
      })
    },
    makePR() {
      uni.showModal({
        title: '生成替换采购申请',
        content: '将创建替换件台账与待审批采购申请（数量 1），提交后进入采购审批流程',
        success: async (r) => {
          if (!r.confirm) return
          try {
            await post(`/assets/${this.assetId}/replacement-request`, {
              qty: 1,
              budget_yuan: (this.sug && this.sug.budget_yuan) || 0,
              reason: '',
            })
            uni.showToast({ title: '采购申请已提交', icon: 'success' })
            setTimeout(() => this.goPurchase(), 800)
          } catch (e) { /* toast 已弹 */ }
        },
      })
    },
    goPurchase() {
      uni.navigateTo({ url: '/pages/purchase/list', fail: () => {} })
    },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 60rpx; }
.head-card { padding: 26rpx; }
.title { font-size: 34rpx; font-weight: 700; line-height: 1.4; }
.code { font-size: 22rpx; margin-top: 6rpx; }
.mono { font-family: monospace; }
.score-big { font-size: 72rpx; font-weight: 800; line-height: 1; flex-shrink: 0; margin-left: 16rpx; }
.score-big.ok { color: #2fa84f; }
.score-big.warn { color: #f5a623; }
.score-big.warn2 { color: #ff7a45; }
.score-big.bad { color: #e5484d; }
.score-bar { height: 12rpx; background: #eef1f6; border-radius: 8rpx; margin: 20rpx 0 14rpx; overflow: hidden; }
.score-fill { height: 100%; border-radius: 8rpx; }
.score-fill.ok { background: #2fa84f; }
.score-fill.warn { background: #f5a623; }
.score-fill.warn2 { background: #ff7a45; }
.score-fill.bad { background: #e5484d; }
.tag { font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 8rpx; }
.g-healthy { background: #eef9f0; color: #2fa84f; }
.g-sub_healthy { background: #fff7e6; color: #d48806; }
.g-attention { background: #fff2e8; color: #ff7a45; }
.g-risk { background: #fff1f0; color: #e5484d; }
.sev1 { background: #eef4ff; color: #1f4e9c; }
.sev2 { background: #fff7e6; color: #d48806; }
.sev3 { background: #fff1f0; color: #e5484d; }
.time { font-size: 22rpx; }
.sec-title { font-size: 30rpx; font-weight: 700; margin: 22rpx 4rpx 14rpx; display: flex; align-items: center; gap: 8rpx; }
.factor { padding: 14rpx 0; border-bottom: 1rpx solid #f0f2f7; }
.factor:last-child { border-bottom: none; }
.f-label { font-size: 28rpx; font-weight: 600; color: #1f2937; }
.f-deduct { font-size: 28rpx; font-weight: 700; color: #e5484d; }
.f-ev { font-size: 24rpx; margin-top: 6rpx; line-height: 1.5; }
.hist { display: flex; align-items: flex-end; gap: 6rpx; height: 130rpx; padding-top: 10rpx; }
.hist-col { flex: 1; display: flex; flex-direction: column; justify-content: flex-end; align-items: center; height: 100%; }
.hist-bar { width: 100%; border-radius: 4rpx 4rpx 0 0; }
.hist-bar.ok { background: #2fa84f; }
.hist-bar.warn { background: #f5a623; }
.hist-bar.warn2 { background: #ff7a45; }
.hist-bar.bad { background: #e5484d; }
.hist-day { font-size: 18rpx; margin-top: 4rpx; }
.al-row { display: flex; align-items: center; gap: 12rpx; padding: 12rpx 0; border-bottom: 1rpx solid #f0f2f7; }
.al-row:last-child { border-bottom: none; }
.al-msg { font-size: 26rpx; color: #4b5563; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.op { font-size: 30rpx; padding: 0; line-height: 2.6; }
.op.full { margin: 0; }
.op.mt { margin-top: 16rpx; }
.op-note { text-align: center; margin-top: 10rpx; font-size: 22rpx; }
.warn-line { background: #fff7e6; color: #d48806; font-size: 25rpx; border-radius: 10rpx; padding: 14rpx 18rpx; line-height: 1.5; }
.ok-line { background: #eef9f0; color: #2fa84f; font-size: 25rpx; border-radius: 10rpx; padding: 14rpx 18rpx; line-height: 1.5; }
.cand-t { font-size: 24rpx; margin: 16rpx 0 8rpx; }
.cand { display: flex; align-items: center; gap: 12rpx; padding: 12rpx 0; border-bottom: 1rpx solid #f0f2f7; }
.cand-name { font-size: 27rpx; font-weight: 600; }
.cand-spec { font-size: 22rpx; margin-top: 4rpx; }
.empty-tip.sm { padding: 26rpx 0; }
.page { padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 170rpx) !important; }
</style>
