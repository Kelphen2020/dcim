<template>
  <view class="page" v-if="pr.id">
    <GlassNavBar title="采购详情" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>

    <view class="card">
      <view class="row">
        <text class="code mono muted">{{ pr.code }}</text>
        <text class="grow"></text>
        <text class="tag" :class="'ps' + pr.status">{{ stText(pr.status) }}</text>
      </view>
      <view class="title">{{ pr.spare_name }} <text class="qty">×{{ pr.qty }}{{ pr.spare_unit || '' }}</text></view>
      <view class="muted mono code">备件编码 {{ pr.spare_code }}</view>
    </view>

    <view class="sec-title"><TechIcon name="doc" color="#2D6BC4" :size="30" /> 申请信息</view>
    <view class="card">
      <view class="kv"><text class="k">申请人</text><text class="v">{{ pr.applicant }}</text></view>
      <view class="kv"><text class="k">申请时间</text><text class="v">{{ dt(pr.created_at) }}</text></view>
      <view class="kv" v-if="pr.supplier"><text class="k">供应商</text><text class="v">{{ pr.supplier }}</text></view>
      <view class="kv" v-if="pr.price > 0"><text class="k">单价</text><text class="v">¥{{ pr.price }}</text></view>
      <view class="kv" v-if="pr.budget_yuan > 0"><text class="k">预算</text><text class="v">¥{{ pr.budget_yuan }}</text></view>
      <view class="kv"><text class="k">当前库存</text><text class="v" :class="{ red: pr.stock_now <= 0 }">{{ pr.stock_now }} / 安全 {{ pr.safety_stock }}</text></view>
      <view class="blk" v-if="pr.reason"><view class="blk-t">申请理由</view><view class="blk-c">{{ pr.reason }}</view></view>
    </view>

    <template v-if="pr.status >= 2 && pr.approver">
      <view class="sec-title"><TechIcon name="check" color="#2FA84F" :size="30" /> 审批信息</view>
      <view class="card">
        <view class="kv"><text class="k">审批人</text><text class="v">{{ pr.approver }}</text></view>
        <view class="kv"><text class="k">审批时间</text><text class="v">{{ dt(pr.approved_at) }}</text></view>
        <view class="blk" v-if="pr.approve_note"><view class="blk-t">审批意见</view><view class="blk-c">{{ pr.approve_note }}</view></view>
        <view class="kv" v-if="pr.stocked_at"><text class="k">入库时间</text><text class="v">{{ dt(pr.stocked_at) }}</text></view>
      </view>
    </template>

    <!-- 审批操作（仅管理员，status=1） -->
    <view class="card" v-if="pr.status === 1 && isAdmin">
      <view class="ops">
        <button class="btn-primary op" @tap="askReview(true)">批 准</button>
        <button class="btn-ghost op danger" @tap="askReview(false)">驳 回</button>
      </view>
      <view class="muted op-note">审批将记录你的账号与时间，并向申请人发送通知，不可撤销</view>
    </view>
    <view class="card" v-else-if="pr.status === 1 && !isAdmin">
      <view class="empty-tip sm">待管理员审批（你的角色无审批权限）</view>
    </view>

    <!-- 入库操作（仅管理员，status=2） -->
    <view class="card" v-if="pr.status === 2 && isAdmin">
      <button class="btn-primary op full" @tap="askStock">执行入库（+{{ pr.qty }}{{ pr.spare_unit || '' }}）</button>
      <view class="muted op-note">入库将增加备件库存并生成流水，申请状态置为已入库</view>
    </view>

    <GlassTabBar current="me" :scrollY="scrollY" />
  </view>
  <view v-else class="empty-tip">{{ loadErr || '加载中…' }}</view>
</template>

<script>
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import TechIcon from '../../components/TechIcon.vue'
import { get, post } from '../../api.js'
import { requireAuth, user } from '../../util.js'
import { PR_STATUS } from './list.vue'

export default {
  components: { GlassNavBar, GlassTabBar, TechIcon },
  data() {
    return { scrollY: 0, navH: 44, id: 0, pr: {}, loadErr: '' }
  },
  computed: {
    isAdmin() { return ['global_admin', 'site_admin'].includes(user().role) },
  },
  onLoad(q) {
    try { this.navH = (uni.getSystemInfoSync().statusBarHeight || 0) + 44 } catch (e) {}
    if (!requireAuth()) return
    this.id = Number(q.id || 0)
    const cached = uni.getStorageSync('dcim_pr_cur')
    if (cached && cached.id === this.id) this.pr = cached
    if (!this.pr.id) this.findInList()
  },
  onPageScroll(e) { this.scrollY = e.scrollTop },
  methods: {
    stText: (s) => PR_STATUS[s] || '',
    dt(t) { return t ? String(t).replace('T', ' ').slice(0, 16) : '' },
    async findInList() {
      // 列表接口无 ?id 过滤：翻页查找兜底
      for (let p = 1; p <= 5; p++) {
        try {
          const d = await get('/purchase-requests', { page: p, size: 100 }, { silent: true })
          const hit = (d.list || []).find((x) => x.id === this.id)
          if (hit) { this.pr = hit; uni.setStorageSync('dcim_pr_cur', hit); return }
          if ((d.list || []).length < 100) break
        } catch (e) { break }
      }
      if (!this.pr.id) this.loadErr = '采购申请不存在'
    },
    askReview(approve) {
      uni.showModal({
        title: approve ? '批准采购申请' : '驳回采购申请',
        editable: true,
        placeholderText: approve ? '审批意见（可选）' : '驳回原因（必填）',
        success: async (r) => {
          if (!r.confirm) return
          const note = (r.content || '').trim()
          if (!approve && !note) {
            uni.showToast({ title: '驳回须填写原因', icon: 'none' })
            return
          }
          try {
            const d = await post(`/purchase-requests/${this.id}/review`, { approve, note })
            this.pr = d
            uni.setStorageSync('dcim_pr_cur', d)
            uni.showToast({ title: approve ? '已批准，通知已发送' : '已驳回，通知已发送', icon: 'none' })
          } catch (e) { /* toast 已弹 */ }
        },
      })
    },
    askStock() {
      uni.showModal({
        title: '执行入库',
        content: `确认入库 ${this.pr.spare_name} ×${this.pr.qty}？库存将增加并生成流水`,
        success: async (r) => {
          if (!r.confirm) return
          try {
            const d = await post(`/purchase-requests/${this.id}/stock`, {})
            if (d && d.id) { this.pr = d; uni.setStorageSync('dcim_pr_cur', d) }
            else await this.findInList()
            uni.showToast({ title: '入库完成', icon: 'success' })
          } catch (e) { /* toast 已弹 */ }
        },
      })
    },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 60rpx; }
.code { font-size: 22rpx; }
.mono { font-family: monospace; }
.title { font-size: 34rpx; font-weight: 700; margin: 12rpx 0 6rpx; line-height: 1.4; }
.qty { font-size: 28rpx; color: #1f4e9c; }
.tag { font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 8rpx; }
.ps1 { background: #fff1f0; color: #e5484d; }
.ps2 { background: #eef4ff; color: #1f4e9c; }
.ps3 { background: #f3f4f6; color: #8a94a6; }
.ps4 { background: #eef9f0; color: #2fa84f; }
.sec-title { font-size: 30rpx; font-weight: 700; margin: 22rpx 4rpx 14rpx; display: flex; align-items: center; gap: 8rpx; }
.kv { display: flex; padding: 10rpx 0; border-bottom: 1rpx solid #f0f2f7; }
.kv:last-child { border-bottom: none; }
.k { width: 160rpx; font-size: 26rpx; color: #8a94a6; flex-shrink: 0; }
.v { flex: 1; font-size: 26rpx; color: #1f2937; }
.v.red { color: #e5484d; font-weight: 700; }
.blk { margin-top: 16rpx; }
.blk-t { font-size: 24rpx; color: #1f4e9c; font-weight: 700; margin-bottom: 6rpx; }
.blk-c { font-size: 27rpx; color: #4b5563; line-height: 1.55; }
.ops { display: flex; }
.op { flex: 1; margin: 8rpx; font-size: 30rpx; padding: 0; line-height: 2.6; }
.op.full { margin: 0; }
.op.danger { color: #e5484d; border-color: #e5484d; }
.op-note { text-align: center; margin-top: 8rpx; font-size: 22rpx; }
.empty-tip.sm { padding: 26rpx 0; }
.page { padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 170rpx) !important; }
</style>
