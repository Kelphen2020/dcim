<template>
  <view class="page" v-if="ch.id">
    <GlassNavBar title="变更详情" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>
    <view class="card">
      <view class="row">
        <text class="code mono muted">{{ ch.code }}</text>
        <text class="grow"></text>
        <text class="tag" :class="'cs' + ch.status">{{ stText(ch.status) }}</text>
      </view>
      <view class="title">{{ ch.title }}</view>
      <view class="row tags">
        <text class="tag risk" :class="'r' + ch.risk">{{ riskText(ch.risk) }}</text>
        <text class="muted" v-if="ch.window_start">窗口 {{ dt(ch.window_start) }} ~ {{ dt(ch.window_end) }}</text>
      </view>
    </view>

    <!-- 影响预分析 -->
    <view class="sec-title"><TechIcon name="bolt" color="#F5A623" :size="30" /> 影响预分析</view>
    <view class="card" v-if="impact">
      <view class="row imp-row">
        <view class="imp-cell"><view class="imp-num" :class="impact.cabinet_count > 0 ? 'orange' : ''">{{ impact.cabinet_count || 0 }}</view><view class="muted">波及机柜</view></view>
        <view class="imp-cell"><view class="imp-num" :class="impact.asset_count > 0 ? 'orange' : ''">{{ impact.asset_count || 0 }}</view><view class="muted">波及设备</view></view>
        <view class="imp-cell"><view class="imp-num">{{ impact.total_power_w ? (impact.total_power_w / 1000).toFixed(1) : '0' }}</view><view class="muted">功率(kW)</view></view>
        <view class="imp-cell"><view class="imp-num" :class="riskCls(impact.suggested_risk)">{{ riskText(impact.suggested_risk) }}</view><view class="muted">建议风险级</view></view>
      </view>
      <view class="warn-line" v-if="impact.suggested_risk > ch.risk">系统建议风险级（{{ riskText(impact.suggested_risk) }}）高于申报（{{ riskText(ch.risk) }}），请复核</view>
      <view class="muted up" v-if="(impact.power_nodes || []).length">关联供电节点：{{ (impact.power_nodes || []).map((n) => n.name || n.kind).join('、') }}</view>
    </view>
    <view class="card" v-else><view class="empty-tip sm">加载中…</view></view>

    <!-- 方案 -->
    <view class="sec-title"><TechIcon name="doc" color="#2D6BC4" :size="30" /> 变更内容</view>
    <view class="card">
      <view class="blk" v-if="ch.description"><view class="blk-t">变更描述</view><view class="blk-c">{{ ch.description }}</view></view>
      <view class="blk" v-if="ch.impact"><view class="blk-t">影响范围（申报）</view><view class="blk-c">{{ ch.impact }}</view></view>
      <view class="blk" v-if="ch.plan"><view class="blk-t">实施方案</view><view class="blk-c">{{ ch.plan }}</view></view>
      <view class="blk" v-if="ch.rollback_plan"><view class="blk-t">回退方案</view><view class="blk-c">{{ ch.rollback_plan }}</view></view>
      <view class="blk" v-if="ch.approve_note"><view class="blk-t">审批意见</view><view class="blk-c">{{ ch.approve_note }}</view></view>
      <view class="blk" v-if="ch.finish_note"><view class="blk-t">完成说明</view><view class="blk-c">{{ ch.finish_note }}</view></view>
    </view>

    <!-- 审批操作（仅管理员，status=2） -->
    <view class="card" v-if="ch.status === 2 && isAdmin">
      <view class="ops">
        <button class="btn-primary op" @tap="askNote('approve')">批 准</button>
        <button class="btn-ghost op danger" @tap="askNote('reject')">驳 回</button>
      </view>
      <view class="muted op-note">审批将记录你的账号与时间，不可撤销</view>
    </view>
    <view class="card" v-else-if="ch.status === 2 && !isAdmin">
      <view class="empty-tip sm">待管理员审批（你的角色无审批权限）</view>
    </view>
    <!-- 执行操作 -->
    <view class="card" v-if="ch.status === 3">
      <button class="btn-primary op full" @tap="act('execute')">开始执行</button>
    </view>
    <view class="card" v-if="ch.status === 5">
      <view class="ops">
        <button class="btn-primary op" @tap="askNote('finish')">完成</button>
        <button class="btn-ghost op danger" @tap="act('rollback')">回滚</button>
      </view>
    </view>
    <GlassTabBar current="me" :scrollY="scrollY" />
  </view>
  <view v-else class="empty-tip">变更不存在</view>
</template>

<script>
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import { get, post } from '../../api.js'
import { requireAuth, user } from '../../util.js'

export default {
  components: { GlassNavBar, GlassTabBar },
  data() {
    return { id: 0, ch: {}, impact: null }
  },
  computed: {
    isAdmin() { return ['global_admin', 'site_admin'].includes(user().role) },
  },
  onLoad(q) {
    try { this.navH = (uni.getSystemInfoSync().statusBarHeight || 0) + 44 } catch (e) {}
    if (!requireAuth()) return
    this.id = Number(q.id || 0)
    const cached = uni.getStorageSync('dcim_chg_cur')
    if (cached && cached.id === this.id) this.ch = cached
    this.load()
  },
  onPageScroll(e) { this.scrollY = e.scrollTop },
  methods: {
    stText(s) { return { 1: '草稿', 2: '待审批', 3: '已批准', 4: '已驳回', 5: '执行中', 6: '已完成', 7: '已回滚' }[s] || '' },
    riskText(r) { return { 1: '低风险', 2: '中风险', 3: '高风险' }[r] || '-' },
    riskCls(r) { return r >= 3 ? 'red' : r === 2 ? 'orange' : 'green' },
    dt(t) { return t ? String(t).replace('T', ' ').slice(5, 16) : '' },
    async load() {
      try {
        const d = await get('/changes', { page: 1, size: 1, keyword: '' }, { silent: true })
        // 列表接口无 ?id 过滤：从缓存或翻页取；优先直接拉 impact（含 change 全量）
        const imp = await get(`/changes/${this.id}/impact`, {}, { silent: true })
        if (imp && imp.change) {
          this.ch = imp.change
          this.impact = imp
          uni.setStorageSync('dcim_chg_cur', this.ch)
          return
        }
      } catch (e) { /* ignore */ }
      if (!this.ch.id) {
        // 兜底：翻列表找
        for (let p = 1; p <= 3; p++) {
          try {
            const d = await get('/changes', { page: p, size: 100 })
            const hit = (d.items || []).find((x) => x.id === this.id)
            if (hit) { this.ch = hit; break }
            if ((d.items || []).length < 100) break
          } catch (e) { break }
        }
      }
    },
    askNote(action) {
      const titles = { approve: '批准变更', reject: '驳回变更', finish: '完成变更' }
      uni.showModal({
        title: titles[action] || action,
        editable: true,
        placeholderText: action === 'reject' ? '驳回原因（必填）' : '审批意见（可选）',
        success: async (r) => {
          if (!r.confirm) return
          if (action === 'reject' && !(r.content || '').trim()) {
            uni.showToast({ title: '驳回须填写原因', icon: 'none' })
            return
          }
          await this.act(action, r.content || '')
        },
      })
    },
    async act(action, note = '') {
      try {
        await post(`/changes/${this.id}/${action}`, { note })
        uni.showToast({ title: '操作成功', icon: 'success' })
        this.load()
      } catch (e) { /* toast 已弹 */ }
    },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 60rpx; }
.code { font-size: 22rpx; }
.mono { font-family: monospace; }
.title { font-size: 34rpx; font-weight: 700; margin: 12rpx 0; line-height: 1.4; }
.tags { margin-bottom: 4rpx; }
.tag { font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 8rpx; margin-right: 10rpx; }
.cs1, .cs4 { background: #f3f4f6; color: #8a94a6; }
.cs2 { background: #fff1f0; color: #e5484d; }
.cs3 { background: #eef4ff; color: #1f4e9c; }
.cs5 { background: #fff7e6; color: #d48806; }
.cs6 { background: #eef9f0; color: #2fa84f; }
.cs7 { background: #fff7e6; color: #d48806; }
.risk { color: #fff; }
.r1 { background: #2fa84f; }
.r2 { background: #f5a623; }
.r3 { background: #e5484d; }
.sec-title { font-size: 30rpx; font-weight: 700; margin: 22rpx 4rpx 14rpx; display: flex; align-items: center; gap: 8rpx; }
.imp-row { display: flex; }
.imp-cell { flex: 1; text-align: center; }
.imp-num { font-size: 40rpx; font-weight: 800; color: #1f2937; }
.imp-num.orange { color: #f5a623; }
.imp-num.red { color: #e5484d; font-size: 30rpx; line-height: 2.4; }
.imp-num.green { color: #2fa84f; font-size: 30rpx; line-height: 2.4; }
.warn-line { margin-top: 16rpx; background: #fff7e6; color: #d48806; font-size: 24rpx; border-radius: 10rpx; padding: 12rpx 16rpx; }
.up { margin-top: 12rpx; font-size: 24rpx; }
.blk { margin-bottom: 18rpx; }
.blk:last-child { margin-bottom: 0; }
.blk-t { font-size: 24rpx; color: #1f4e9c; font-weight: 700; margin-bottom: 6rpx; }
.blk-c { font-size: 27rpx; color: #4b5563; line-height: 1.55; }
.ops { display: flex; }
.op { flex: 1; margin: 8rpx; font-size: 30rpx; padding: 0; line-height: 2.6; }
.op.full { margin: 0; }
.op.danger { color: #e5484d; border-color: #e5484d; }
.op-note { text-align: center; margin-top: 8rpx; }
.empty-tip.sm { padding: 26rpx 0; }
.page { padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 170rpx) !important; }
</style>
