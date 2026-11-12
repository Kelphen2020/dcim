<template>
  <view class="page">
    <GlassNavBar title="值班交接" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>
    <view class="card head-card">
      <view class="row">
        <view class="grow">
          <view class="h-title">值班交接单</view>
          <view class="muted">生成时间 {{ ho.generated_at || '--' }} · 数据实时</view>
        </view>
        <view class="ho-badge" :class="signedToday ? 'ok' : 'todo'">
          <TechIcon :name="signedToday ? 'check-circle' : 'clock'" :color="signedToday ? '#B9F5C8' : '#FFE3A3'" :size="30" />
          {{ signedToday ? '今日已交接' : '今日未交接' }}
        </view>
      </view>
    </view>

    <!-- 未闭环告警 -->
    <view class="sec-title"><TechIcon name="bell" color="#E5484D" :size="30" /> 未闭环告警 <text class="cnt">({{ ho.open_alert_count || 0 }})</text></view>
    <view class="card" v-if="!(ho.open_alerts || []).length"><view class="empty-tip sm">无</view></view>
    <view class="card li clickable" v-for="a in (ho.open_alerts || []).slice(0, 10)" :key="'a' + a.id" @tap="goAlert(a)">
      <view class="row">
        <text class="sev" :class="'s' + a.severity">{{ sevText(a.severity) }}</text>
        <text class="grow li-t">{{ a.message }}</text>
        <text class="go-arrow">›</text>
      </view>
    </view>

    <!-- 进行中工单 -->
    <view class="sec-title"><TechIcon name="wrench" color="#2D6BC4" :size="30" /> 进行中工单 <text class="cnt">({{ ho.open_order_count || 0 }})</text></view>
    <view class="card" v-if="!(ho.open_orders || []).length"><view class="empty-tip sm">无</view></view>
    <view class="card li clickable" v-for="w in (ho.open_orders || []).slice(0, 10)" :key="'w' + w.id" @tap="goWo(w)">
      <view class="row">
        <text class="mono muted">{{ w.code }}</text>
        <text class="grow li-t">{{ w.title }}</text>
        <text class="sla" :class="(w.sla_respond_breach || w.sla_resolve_breach) ? 'breach' : ''">
          {{ (w.sla_respond_breach || w.sla_resolve_breach) ? '违约' : (slaLeft(w.sla_resolve_due) || '-') }}
        </text>
        <text class="go-arrow">›</text>
      </view>
    </view>

    <!-- 今日巡检 -->
    <view class="sec-title"><TechIcon name="doc" color="#2FA84F" :size="30" /> 今日巡检 <text class="cnt">({{ (ho.today_inspections || []).length }})</text></view>
    <view class="card" v-if="!(ho.today_inspections || []).length"><view class="empty-tip sm">今日暂无巡检记录</view></view>
    <view class="card li" v-for="r in (ho.today_inspections || []).slice(0, 8)" :key="'r' + r.id">
      <view class="row">
        <text class="tag" :class="'rs' + r.status">{{ recSt(r.status) }}</text>
        <text class="grow li-t">{{ r.target_name || '巡检' }}</text>
        <text class="muted">{{ String(r.inspected_at).slice(11, 16) }}</text>
      </view>
    </view>

    <!-- 活跃风暴 -->
    <view class="sec-title" v-if="(ho.active_storms || []).length"><TechIcon name="alert" color="#7C3AED" :size="30" /> 活跃风暴 ({{ ho.active_storms.length }})</view>
    <view class="card li" v-for="s in (ho.active_storms || [])" :key="'s' + s.id">
      <view>#{{ s.id }} 根因: {{ s.root_cause || '-' }} · 成员 {{ s.alert_count || 0 }}</view>
    </view>

    <!-- 签字区 -->
    <view class="sec-title"><TechIcon name="pen" color="#1F4E9C" :size="30" /> 交接确认</view>
    <view class="card">
      <view class="row field-row">
        <text class="f-lbl">接班人</text>
        <picker class="grow" :range="userNames" @change="onPick">
          <view class="picker-v">{{ toUser ? toUser.display_name || toUser.username : '选择接班人 ▾' }}</view>
        </picker>
      </view>
      <view class="f-lbl sig-lbl">交班人签字（下方手写）</view>
      <view class="sig-wrap">
        <canvas canvas-id="sigPad" id="sigPad" class="sig-pad"
          @touchstart="tsStart" @touchmove="tsMove" @touchend="tsEnd"
          disable-scroll="true"></canvas>
        <view class="sig-clear" @tap="clearSig">清除</view>
      </view>
      <input class="note-in" v-model="note" placeholder="交接备注（可选）" />
      <button class="btn-primary" :disabled="submitting || !hasInk || !toUser" @tap="submit">
        {{ submitting ? '提交中…' : '签字确认交接' }}
      </button>
      <view class="muted tip" v-if="!toUser">请先选择接班人</view>
      <view class="muted tip" v-else-if="!hasInk">请在签字区手写签名</view>
    </view>

    <!-- 历史签字 -->
    <view class="sec-title">历史交接 <text class="cnt">({{ signs.length }})</text></view>
    <view class="card li" v-for="s in signs" :key="s.id">
      <view class="row">
        <text class="grow">{{ String(s.handover_date).slice(0, 10) }} {{ shiftText(s.shift) }}</text>
        <text class="muted">{{ s.from_user_name }} → {{ s.to_user_name || '-' }}</text>
      </view>
      <image v-if="s.signature_url" :src="imgUrl(s.signature_url)" mode="aspectFit" class="sig-thumb" />
    </view>
    <view class="card" v-if="!signs.length"><view class="empty-tip sm">暂无交接记录</view></view>
    <GlassTabBar current="me" :scrollY="scrollY" />
  </view>
</template>

<script>
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import TechIcon from '../../components/TechIcon.vue'
import { get, post, upload } from '../../api.js'
import { CONFIG } from '../../config.js'
import { requireAuth, SEV_TEXT, SHIFT_TEXT, slaLeft } from '../../util.js'

export default {
  components: { TechIcon , GlassNavBar, GlassTabBar },
  data() {
    return {
      scrollY: 0,
      navH: 44,
      ho: {}, users: [], toUser: null, note: '', signs: [],
      ctx: null, drawing: false, hasInk: false, submitting: false,
    }
  },
  computed: {
    userNames() { return this.users.map((u) => `${u.display_name || u.username}(${u.username})`) },
    // H1: 今日是否已有交接签字
    signedToday() {
      const today = String(new Date()).slice(0, 10)
      return (this.signs || []).some((s) => String(s.handover_date || s.created_at || '').slice(0, 10) === today)
    },
  },
  onLoad() {
    try { this.navH = (uni.getSystemInfoSync().statusBarHeight || 0) + 44 } catch (e) {}
    if (!requireAuth()) return
    this.load()
    this.$nextTick(() => {
      this.ctx = uni.createCanvasContext('sigPad', this)
      this.ctx.setStrokeStyle('#1f2937')
      this.ctx.setLineWidth(3)
      this.ctx.setLineCap('round')
    })
  },
  onPageScroll(e) { this.scrollY = e.scrollTop },
  methods: {
    sevText: (s) => SEV_TEXT[s] || '',
    shiftText: (s) => SHIFT_TEXT[s] || s,
    recSt(s) { return { 1: '正常', 2: '异常', 3: '逾期' }[s] || '' },
    slaLeft,
    // H1: 交接单待办穿透
    goAlert(a) {
      uni.setStorageSync('dcim_alert_cur', a)
      uni.navigateTo({ url: '/pages/alert/detail?id=' + a.id, fail: () => {} })
    },
    goWo(w) {
      uni.setStorageSync('dcim_wo_cur', w)
      uni.navigateTo({ url: '/pages/wo/detail?id=' + w.id, fail: () => {} })
    },
    imgUrl(u) { return u && u.startsWith('http') ? u : CONFIG.BASE_URL + u },
    async load() {
      try { this.ho = await get('/duty/handover', {}, { silent: true }) } catch (e) { this.ho = {} }
      try {
        const u = await get('/users-options', {}, { silent: true })
        this.users = u.items || u || []
      } catch (e) { /* ignore */ }
      try {
        const s = await get('/duty/handover/signs', {}, { silent: true })
        this.signs = s.items || []
      } catch (e) { /* ignore */ }
    },
    onPick(e) { this.toUser = this.users[Number(e.detail.value)] },
    // 手写签字（触摸轨迹）
    pos(e) {
      const t = e.touches && e.touches[0]
      return t ? { x: t.x, y: t.y } : null
    },
    tsStart(e) {
      const p = this.pos(e)
      if (!p || !this.ctx) return
      this.drawing = true
      this.ctx.beginPath()
      this.ctx.moveTo(p.x, p.y)
    },
    tsMove(e) {
      const p = this.pos(e)
      if (!p || !this.drawing || !this.ctx) return
      this.ctx.lineTo(p.x, p.y)
      this.ctx.stroke()
      this.ctx.draw(true)
      this.ctx.moveTo(p.x, p.y)
      this.hasInk = true
    },
    tsEnd() { this.drawing = false },
    clearSig() {
      if (!this.ctx) return
      this.ctx.clearRect(0, 0, 999, 999)
      this.ctx.draw()
      this.hasInk = false
    },
    async submit() {
      this.submitting = true
      try {
        // 1) canvas 导出为图片
        const res = await new Promise((resolve, reject) => {
          uni.canvasToTempFilePath({
            canvasId: 'sigPad',
            fileType: 'png',
            success: resolve, fail: reject,
          }, this)
        })
        // 2) 上传签字图
        const up = await upload(res.tempFilePath, { ref_type: 'duty_handover', ref_id: '0' })
        // 3) 落库
        await post('/duty/handover/sign', {
          site_id: 0,
          shift: '',
          to_user_id: this.toUser.id,
          signature_url: up.url,
          note: this.note,
        })
        uni.showToast({ title: '交接完成', icon: 'success' })
        this.note = ''
        this.clearSig()
        this.load()
      } catch (e) {
        uni.showToast({ title: (e && e.message) || '提交失败', icon: 'none' })
      } finally {
        this.submitting = false
      }
    },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 60rpx; }
.head-card { background: linear-gradient(135deg, #1f4e9c, #2d6bc4); color: #fff; }
.h-title { font-size: 36rpx; font-weight: 800; }
.head-card .muted { color: rgba(255, 255, 255, 0.8); }
.ho-badge { display: flex; align-items: center; gap: 8rpx; font-size: 22rpx; padding: 8rpx 18rpx; border-radius: 24rpx; flex-shrink: 0; }
.ho-badge.ok { background: rgba(47, 168, 79, 0.25); color: #d6ffe0; }
.ho-badge.todo { background: rgba(245, 166, 35, 0.25); color: #ffeccd; }
.sec-title { font-size: 30rpx; font-weight: 700; margin: 22rpx 4rpx 14rpx; }
.cnt { color: #8a94a6; font-size: 24rpx; font-weight: 400; }
.li { padding: 18rpx 24rpx; }
.li.clickable:active { background: #f0f5fc; }
.go-arrow { color: #c2ccdc; font-size: 30rpx; flex-shrink: 0; margin-left: 8rpx; }
.li-t { font-size: 27rpx; margin: 0 14rpx; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.sev { font-size: 20rpx; color: #fff; border-radius: 6rpx; padding: 3rpx 12rpx; flex-shrink: 0; }
.s1 { background: #3b82f6; }
.s2 { background: #f5a623; }
.s3 { background: #e5484d; }
.mono { font-family: monospace; font-size: 22rpx; flex-shrink: 0; }
.sla { font-size: 22rpx; color: #2fa84f; flex-shrink: 0; }
.sla.breach { color: #e5484d; font-weight: 700; }
.tag { font-size: 20rpx; padding: 3rpx 12rpx; border-radius: 6rpx; flex-shrink: 0; }
.rs1 { background: #eef9f0; color: #2fa84f; }
.rs2 { background: #fff1f0; color: #e5484d; }
.rs3 { background: #f3f4f6; color: #8a94a6; }
.field-row { padding: 14rpx 0; border-bottom: 1rpx solid #f0f3f8; }
.f-lbl { font-size: 27rpx; color: #4b5563; width: 160rpx; flex-shrink: 0; }
.picker-v { font-size: 28rpx; color: #1f2937; }
.sig-lbl { margin: 20rpx 0 12rpx; }
.sig-wrap { position: relative; }
.sig-pad { width: 100%; height: 300rpx; background: #fafbfd; border: 2rpx dashed #c9d4e8; border-radius: 14rpx; box-sizing: border-box; }
.sig-clear { position: absolute; top: 10rpx; right: 14rpx; font-size: 22rpx; color: #8a94a6; background: rgba(255, 255, 255, 0.8); border-radius: 8rpx; padding: 4rpx 14rpx; }
.note-in { background: #f4f6fa; border-radius: 12rpx; padding: 18rpx; font-size: 27rpx; margin: 20rpx 0; }
.tip { text-align: center; margin-top: 12rpx; }
.sig-thumb { width: 240rpx; height: 100rpx; margin-top: 10rpx; background: #fafbfd; border-radius: 8rpx; }
.empty-tip.sm { padding: 26rpx 0; }
.page { padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 170rpx) !important; }
</style>
