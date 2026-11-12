<template>
  <view class="page">
    <GlassNavBar title="巡检执行" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>
    <!-- 错误态 -->
    <view class="card err-card" v-if="err">
      <view class="err-ico"><TechIcon name="alert" color="#E5484D" :size="80" /></view>
      <view class="err-msg">{{ err }}</view>
      <button class="btn-primary" @tap="back">返回巡检页</button>
    </view>

    <!-- 签到成功 -->
    <template v-if="!err && checkin">
      <view class="card ok-card">
        <view class="ok-ico"><TechIcon name="check" color="#2FA84F" :size="52" /></view>
        <view class="grow">
          <view class="ok-title">签到成功 {{ checkedInAt }}</view>
          <view class="muted">{{ target.name || '' }} · {{ target.type === 'room' ? '房间' : '机柜' }} · 今日已巡 {{ todayCount }} 次</view>
        </view>
      </view>

      <!-- 计划选择 -->
      <view class="card" v-if="plans.length > 1">
        <view class="lbl">巡检计划</view>
        <picker :range="planNames" @change="onPlan">
          <view class="picker-val">{{ curPlan ? curPlan.name : '选择计划' }} ▾</view>
        </picker>
      </view>
      <view class="card" v-else-if="plans.length === 1">
        <view class="lbl">巡检计划</view>
        <view class="picker-val">{{ plans[0].name }}</view>
      </view>

      <!-- 逐项检查 -->
      <view class="card" v-if="items.length">
        <view class="lbl row">
          逐项检查
          <text class="grow"></text>
          <text class="prog">{{ results.filter((r) => r === 2).length ? results.filter((r) => r === 2).length + ' 项异常' : '全部正常' }}</text>
        </view>
        <view class="item" v-for="(it, i) in items" :key="i">
          <view class="item-txt">{{ i + 1 }}. {{ it }}</view>
          <view class="live" v-if="live && live[i]" :class="lvCls(live[i])">{{ lvText(live[i]) }}</view>
          <view class="row radios">
            <view class="radio" :class="{ on: results[i] === 1, ok: results[i] === 1 }" @tap="results[i] = 1"><TechIcon name="check" :color="results[i] === 1 ? '#fff' : '#2FA84F'" :size="28" /> 正常</view>
            <view class="radio" :class="{ on: results[i] === 2, bad: results[i] === 2 }" @tap="results[i] = 2"><TechIcon name="x" :color="results[i] === 2 ? '#fff' : '#E5484D'" :size="28" /> 异常</view>
          </view>
          <view v-if="results[i] === 2">
            <input class="note-in" v-model="itemNotes[i]" placeholder="异常说明" />
            <view class="photos">
              <view class="photo" v-for="(p, pi) in itemPhotos[i]" :key="pi">
                <image :src="imgUrl(p)" mode="aspectFill" class="pimg" />
                <text class="del" @tap="itemPhotos[i].splice(pi, 1)">×</text>
              </view>
              <view class="photo add" @tap="takePhoto(i)"><TechIcon name="scan" color="#8A94A6" :size="40" /></view>
            </view>
          </view>
        </view>
      </view>

      <!-- 结论 -->
      <view class="card">
        <view class="lbl">巡检结论</view>
        <view class="row radios big">
          <view class="radio" :class="{ on: status === 1, ok: status === 1 }" @tap="status = 1"><TechIcon name="check" :color="status === 1 ? '#fff' : '#2FA84F'" :size="28" /> 全部正常</view>
          <view class="radio" :class="{ on: status === 2, bad: status === 2 }" @tap="status = 2"><TechIcon name="x" :color="status === 2 ? '#fff' : '#E5484D'" :size="28" /> 发现异常</view>
        </view>
        <textarea v-if="status === 2" class="ta" v-model="abnormal" placeholder="异常描述（必填，将自动转工单）" />
        <textarea class="ta sm" v-model="remark" placeholder="补充说明（可选）" />
      </view>

      <button class="btn-primary submit" :disabled="submitting" @tap="submit">{{ submitting ? '提交中…' : '提交巡检' }}</button>
      <view class="muted fresh">签到有效期 {{ freshMin }} 分钟，超时须重新扫码（防补录，巡检时间以签到为准）</view>
    </template>

    <!-- 完成态 -->
    <view class="card done-card" v-if="done">
      <view class="done-ico"><TechIcon name="check" color="#fff" :size="60" /></view>
      <view class="done-title">巡检已提交</view>
      <view class="muted" v-if="done.report_code">报告编号：{{ done.report_code }}</view>
      <view class="wo-tip" v-if="done.work_order_id">异常已自动转工单 #{{ done.work_order_id }}</view>
      <button class="btn-primary" @tap="back">返回巡检页</button>
    </view>
    <GlassTabBar current="scan" :scrollY="scrollY" />
  </view>
</template>

<script>
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import TechIcon from '../../components/TechIcon.vue'
import { get, post, upload } from '../../api.js'
import { CONFIG } from '../../config.js'
import { requireAuth } from '../../util.js'

export default {
  components: { TechIcon , GlassNavBar, GlassTabBar },
  data() {
    return {
      scrollY: 0,
      navH: 44,
      token: '',
      err: '',
      checkin: null,
      target: {},
      plans: [],
      todayCount: 0,
      checkedInAt: '',
      planId: 0,
      items: [],
      results: [],
      itemNotes: [],
      itemPhotos: [],
      live: null,
      status: 1,
      abnormal: '',
      remark: '',
      submitting: false,
      done: null,
      freshMin: 120,
    }
  },
  computed: {
    planNames() { return this.plans.map((p) => p.name) },
    curPlan() { return this.plans.find((p) => p.id === this.planId) },
  },
  onLoad(q) {
    try { this.navH = (uni.getSystemInfoSync().statusBarHeight || 0) + 44 } catch (e) {}
    if (!requireAuth()) return
    this.token = decodeURIComponent(q.token || '')
    if (!this.token) { this.err = '缺少扫码令牌'; return }
    this.doCheckin()
    this.loadFresh()
  },
  onPageScroll(e) { this.scrollY = e.scrollTop },
  methods: {
    imgUrl(p) { return p.startsWith('http') ? p : CONFIG.BASE_URL + p },
    lvCls(x) {
      return x.range_status === 'over' || x.status === 2 ? 'lv-bad'
        : (x.range_status === 'high' || x.range_status === 'low') ? 'lv-alarm'
        : x.current_value == null ? 'lv-miss' : 'lv-ok'
    },
    lvText(x) {
      if (!x.point_code) return ''
      if (x.current_value == null) return x.point_code + ' 无读数'
      const tag = { over: ' ⚠过高', high: ' ⚠超高', low: ' ⚠过低', normal: '' }[x.range_status] ?? (x.in_range === false ? ' 越限' : '')
      return `${x.point_name || x.point_code}: ${x.current_value}${x.unit || ''}${tag}`
    },
    async loadFresh() {
      try {
        const d = await get('/public-settings', {}, { silent: true })
        if (d && d.scan_checkin_fresh_min) this.freshMin = Number(d.scan_checkin_fresh_min)
      } catch (e) { /* ignore */ }
    },
    async doCheckin() {
      try {
        const d = await post('/inspection-scan/checkin', { token: this.token }, { silent: true })
        this.checkin = d.checkin
        this.target = d.target || {}
        this.plans = d.plans || []
        this.todayCount = d.today_records || 0
        this.checkedInAt = String(d.server_time || '').slice(11, 16)
        this.planId = this.plans[0] ? this.plans[0].id : 0
        this.loadItems()
      } catch (e) {
        this.err = e.message || '签到失败：令牌无效或已过期'
      }
    },
    onPlan(e) {
      this.planId = this.plans[Number(e.detail.value)].id
      this.loadItems()
    },
    loadItems() {
      const p = this.curPlan
      this.items = p ? (p.items || '').split(/[\n|]/).map((x) => x.trim()).filter(Boolean) : []
      this.results = this.items.map(() => 1)
      this.itemNotes = this.items.map(() => '')
      this.itemPhotos = this.items.map(() => [])
      this.live = null
      if (!this.items.length) return
      post('/inspection-items-live', { items: this.items }, { silent: true })
        .then((d) => { this.live = d.items || null })
        .catch(() => {})
    },
    takePhoto(i) {
      uni.chooseImage({
        count: 3,
        sizeType: ['compressed'],
        success: async (r) => {
          uni.showLoading({ title: '上传中' })
          try {
            for (const fp of r.tempFilePaths) {
              const d = await upload(fp, { ref_type: 'inspection', ref_id: String(this.checkin.id) })
              this.itemPhotos[i].push(d.url)
            }
          } catch (e) { /* toast */ } finally { uni.hideLoading() }
        },
      })
    },
    async submit() {
      const abnCount = this.results.filter((r) => r === 2).length
      // 有异常项自动置结论为异常
      if (abnCount > 0 && this.status === 1) this.status = 2
      if (this.status === 2 && !this.abnormal.trim()) {
        // 用逐项异常说明拼接
        const notes = this.items.map((it, i) => (this.results[i] === 2 ? `${it}：${this.itemNotes[i] || '异常'}` : '')).filter(Boolean)
        this.abnormal = notes.join('；')
        if (!this.abnormal.trim()) {
          uni.showToast({ title: '请填写异常描述', icon: 'none' })
          return
        }
      }
      this.submitting = true
      try {
        const itemLines = this.items.map((it, i) => {
          let line = `${i + 1}. ${it}：${this.results[i] === 2 ? '异常' : '正常'}`
          if (this.results[i] === 2 && this.itemNotes[i]) line += `（${this.itemNotes[i]}）`
          if (this.itemPhotos[i] && this.itemPhotos[i].length) line += ` [照片:${this.itemPhotos[i].join(',')}]`
          return line
        }).join('\n')
        const result = [itemLines, this.remark].filter(Boolean).join('\n')
        const snap = this.live ? JSON.stringify(this.live.map((x) => ({ item: x.item, point: x.point_name || x.point_code, value: x.current_value, unit: x.unit, in_range: x.in_range }))) : ''
        const d = await post('/inspection-scan/submit', {
          checkin_id: this.checkin.id,
          plan_id: this.planId || undefined,
          status: this.status,
          abnormal: this.abnormal,
          result,
          point_snap: snap,
        })
        this.done = d || {}
        this.checkin = null
      } catch (e) {
        uni.showToast({ title: e.message || '提交失败', icon: 'none' })
      } finally {
        this.submitting = false
      }
    },
    back() { uni.reLaunch({ url: '/pages/scan/scan', fail: () => {} }) },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 60rpx; }
.err-card { text-align: center; padding: 60rpx 30rpx; }
.err-ico { font-size: 80rpx; }
.err-msg { font-size: 30rpx; color: #e5484d; margin: 20rpx 0 40rpx; line-height: 1.5; }
.ok-card { display: flex; align-items: center; background: #eef9f0; }
.ok-ico { font-size: 48rpx; margin-right: 18rpx; }
.ok-title { font-size: 32rpx; font-weight: 700; color: #2fa84f; }
.lbl { font-size: 26rpx; font-weight: 700; color: #1f4e9c; margin-bottom: 14rpx; }
.picker-val { font-size: 30rpx; padding: 12rpx 0; }
.prog { font-size: 24rpx; color: #8a94a6; font-weight: 400; }
.item { padding: 20rpx 0; border-bottom: 1rpx dashed #e5eaf3; }
.item:last-child { border-bottom: none; }
.item-txt { font-size: 28rpx; line-height: 1.5; }
.live { font-size: 24rpx; margin: 8rpx 0; padding: 8rpx 14rpx; border-radius: 8rpx; display: inline-block; }
.lv-ok { background: #eef9f0; color: #2fa84f; }
.lv-alarm { background: #fff7e6; color: #d48806; }
.lv-bad { background: #fff1f0; color: #e5484d; }
.lv-miss { background: #f3f4f6; color: #8a94a6; }
.radios { margin-top: 12rpx; }
.radios.big { margin-top: 0; }
.radio {
  padding: 14rpx 34rpx; border-radius: 12rpx; border: 2rpx solid #e5eaf3;
  margin-right: 20rpx; font-size: 27rpx; color: #4b5563; background: #fff;
  display: inline-flex; align-items: center; gap: 8rpx;
}
.radio.on.ok { background: #2fa84f; border-color: #2fa84f; color: #fff; }
.radio.on.bad { background: #e5484d; border-color: #e5484d; color: #fff; }
.note-in { background: #f4f6fa; border-radius: 10rpx; padding: 14rpx; font-size: 26rpx; margin-top: 12rpx; }
.photos { display: flex; flex-wrap: wrap; margin-top: 12rpx; }
.photo { position: relative; width: 120rpx; height: 120rpx; margin: 0 12rpx 12rpx 0; border-radius: 10rpx; overflow: hidden; }
.pimg { width: 100%; height: 100%; }
.photo .del { position: absolute; top: 0; right: 0; background: rgba(0, 0, 0, 0.5); color: #fff; padding: 2rpx 10rpx; font-size: 20rpx; }
.photo.add { background: #f4f6fa; border: 2rpx dashed #c9d4e8; display: flex; align-items: center; justify-content: center; font-size: 40rpx; box-sizing: border-box; }
.ta { width: 100%; height: 150rpx; background: #f4f6fa; border-radius: 12rpx; padding: 16rpx; box-sizing: border-box; font-size: 27rpx; margin-top: 16rpx; }
.ta.sm { height: 100rpx; }
.submit { margin-top: 30rpx; }
.fresh { text-align: center; margin-top: 20rpx; font-size: 22rpx; }
.done-card { text-align: center; padding: 70rpx 30rpx; }
.done-ico {
  width: 110rpx; height: 110rpx; line-height: 110rpx; border-radius: 50%;
  background: #2fa84f; color: #fff; font-size: 60rpx; margin: 0 auto 24rpx;
}
.done-title { font-size: 36rpx; font-weight: 700; margin-bottom: 12rpx; }
.wo-tip { color: #e5484d; font-size: 26rpx; margin: 16rpx 0; }
.done-card .btn-primary { margin-top: 30rpx; }
.page { padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 170rpx) !important; }
</style>
