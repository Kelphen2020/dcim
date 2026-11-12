<template>
  <view class="page" v-if="rec.id">
    <GlassNavBar title="报告详情" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>
    <view class="card head" :class="{ signed: rec.reviewer_id }">
      <view class="row">
        <view class="grow">
          <view class="rp-code mono" v-if="rec.report_code">{{ rec.report_code }}</view>
          <view class="rp-title">{{ plan.name || rec.target_name || '巡检报告' }}</view>
        </view>
        <text class="tag big" :class="'rs' + rec.status">{{ stText(rec.status) }}</text>
      </view>
      <view class="muted meta">
        <view class="meta-line"><TechIcon name="pin" color="#8A94A6" :size="26" /> {{ site.name }}</view>
        <view class="meta-line"><TechIcon name="clock" color="#8A94A6" :size="26" /> 巡检时间 {{ dt(rec.inspected_at) }}<text v-if="rec.source === 'scan'"> · 扫码签到</text></view>
        <view class="meta-line"><TechIcon name="user" color="#8A94A6" :size="26" /> 巡检人 {{ inspector_name || '-' }}</view>
      </view>
    </view>

    <!-- 签发 -->
    <view class="card sign-card" v-if="rec.reviewer_id">
      <view class="row">
        <view class="sign-ico"><TechIcon name="pen" color="#1F4E9C" :size="44" /></view>
        <view class="grow">
          <view>已签发：{{ reviewer_name }}</view>
          <view class="muted">{{ dt(rec.reviewed_at) }}<text v-if="rec.signature"> · 签字「{{ rec.signature }}」</text></view>
        </view>
      </view>
    </view>
    <view class="card sign-card unsigned" v-else>
      <text class="muted">未签发（管理员在 PC 端签发后锁定）</text>
    </view>

    <!-- 逐项结果 -->
    <view class="sec-title" v-if="(resultLines || []).length">逐项检查结果</view>
    <view class="card" v-if="(resultLines || []).length">
      <view class="rl" v-for="(l, i) in resultLines" :key="i" :class="{ bad: /异常/.test(l) }">{{ l }}</view>
    </view>
    <view class="card" v-else-if="rec.result">
      <view class="rl" v-for="(l, i) in rec.result.split('\n')" :key="i" :class="{ bad: /异常/.test(l) }">{{ l }}</view>
    </view>

    <!-- 测点快照 -->
    <view class="sec-title" v-if="(snap || []).length">测点实时值快照</view>
    <view class="card" v-if="(snap || []).length">
      <view class="snap-row" v-for="(s, i) in snap" :key="i">
        <text class="grow">{{ s.item || s.point || '-' }}</text>
        <text class="mono" :class="s.in_range === false ? 'bad' : 'ok'">{{ s.value != null ? s.value + (s.unit || '') : '无读数' }}</text>
      </view>
    </view>

    <!-- 现场照片（M6-C：巡检拍照真闭环，签发核验依据） -->
    <template v-if="photos.length">
      <view class="sec-title"><TechIcon name="scan" color="#16233D" :size="30" /> 现场照片（{{ photos.length }}）</view>
      <view class="card">
        <view class="photos">
          <view class="photo" v-for="(p, i) in photos" :key="p.id">
            <image :src="imgUrl(p.url)" mode="aspectFill" class="pimg" @tap="preview(i)" />
          </view>
        </view>
        <view class="muted att-meta">巡检执行时拍摄，已存服务器</view>
      </view>
    </template>

    <!-- 异常与工单 -->
    <view class="sec-title" v-if="rec.abnormal">异常描述</view>
    <view class="card abn" v-if="rec.abnormal">{{ rec.abnormal }}</view>
    <view class="card wo-link" v-if="wo.id" @tap="goWo">
      <view class="grow row"><TechIcon name="wrench" color="#2D6BC4" :size="30" /><text style="margin-left:8rpx">关联工单 {{ wo.code }}</text></view>
      <text class="tag" :class="'ws' + wo.status">{{ woStText(wo.status) }}</text>
    </view>
    <GlassTabBar current="me" :scrollY="scrollY" />
  </view>
  <view v-else class="empty-tip">报告不存在</view>
</template>

<script>
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import TechIcon from '../../components/TechIcon.vue'
import { get } from '../../api.js'
import { CONFIG } from '../../config.js'
import { requireAuth, WO_STATUS } from '../../util.js'

export default {
  components: { TechIcon , GlassNavBar, GlassTabBar },
  data() {
    return { id: 0, rec: {}, plan: {}, site: {}, wo: {}, snap: [], resultLines: [], inspector_name: '', reviewer_name: '', photos: [] }
  },
  onLoad(q) {
    try { this.navH = (uni.getSystemInfoSync().statusBarHeight || 0) + 44 } catch (e) {}
    if (!requireAuth()) return
    this.id = Number(q.id || 0)
    this.load()
  },
  onPageScroll(e) { this.scrollY = e.scrollTop },
  methods: {
    stText(s) { return { 1: '正常', 2: '异常', 3: '逾期' }[s] || '' },
    woStText(s) { return WO_STATUS[s] || '' },
    dt(t) { return t ? String(t).replace('T', ' ').slice(0, 16) : '' },
    async load() {
      try {
        const d = await get(`/inspection-reports/${this.id}`)
        this.rec = d.record || {}
        this.plan = d.plan || {}
        this.site = d.site || {}
        this.wo = d.work_order || {}
        this.snap = Array.isArray(d.point_snapshot) ? d.point_snapshot : []
        this.resultLines = d.result_lines || []
        this.inspector_name = d.inspector_name || ''
        this.reviewer_name = d.reviewer_name || ''
        this.loadPhotos()
      } catch (e) { /* toast */ }
    },
    // 巡检拍照附件 ref = inspection:checkin_id
    async loadPhotos() {
      const cid = this.rec.checkin_id
      if (!cid) { this.photos = []; return }
      try {
        const d = await get('/mobile/attachments', { ref_type: 'inspection', ref_id: String(cid) }, { silent: true })
        this.photos = d.items || []
      } catch (e) { this.photos = [] }
    },
    imgUrl(p) { return p && p.startsWith('http') ? p : CONFIG.BASE_URL + p },
    preview(i) {
      uni.previewImage({ urls: this.photos.map((p) => this.imgUrl(p.url)), current: i })
    },
    goWo() {
      uni.navigateTo({ url: '/pages/wo/detail?id=' + this.wo.id })
    },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 40rpx; }
.head { border-left: 12rpx solid #2fa84f; }
.head.signed { border-left-color: #1f4e9c; }
.rp-code { font-size: 24rpx; color: #1f4e9c; font-weight: 700; }
.rp-title { font-size: 34rpx; font-weight: 700; margin: 8rpx 0 12rpx; }
.meta view { margin-top: 6rpx; }
.meta-line { display: flex; align-items: center; margin-top: 6rpx; }
.meta-line .tech-icon { margin-right: 10rpx; }
.row { display: flex; align-items: center; }
.tag { font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 8rpx; }
.tag.big { font-size: 24rpx; }
.rs1 { background: #eef9f0; color: #2fa84f; }
.rs2 { background: #fff1f0; color: #e5484d; }
.rs3 { background: #f3f4f6; color: #8a94a6; }
.sign-card { display: flex; background: #eef4ff; }
.sign-card.unsigned { background: #f4f6fa; }
.sign-ico { font-size: 40rpx; margin-right: 18rpx; }
.sec-title { font-size: 28rpx; font-weight: 700; margin: 20rpx 4rpx 12rpx; display: flex; align-items: center; gap: 8rpx; }
.photos { display: flex; flex-wrap: wrap; }
.photo { position: relative; width: 160rpx; height: 160rpx; margin: 0 12rpx 12rpx 0; border-radius: 12rpx; overflow: hidden; background: #f4f6fa; }
.pimg { width: 100%; height: 100%; }
.att-meta { font-size: 22rpx; margin-top: 4rpx; }
.rl { font-size: 27rpx; line-height: 1.7; color: #4b5563; }
.rl.bad { color: #e5484d; font-weight: 600; }
.snap-row { display: flex; padding: 12rpx 0; border-bottom: 1rpx solid #f0f3f8; font-size: 26rpx; }
.snap-row:last-child { border-bottom: none; }
.mono { font-family: monospace; }
.mono.ok { color: #2fa84f; }
.mono.bad { color: #e5484d; font-weight: 700; }
.abn { color: #e5484d; background: #fff1f0; font-size: 27rpx; }
.wo-link { display: flex; align-items: center; font-size: 28rpx; }
.ws1 { background: #fff1f0; color: #e5484d; }
.ws2 { background: #fff7e6; color: #d48806; }
.ws3 { background: #eef9f0; color: #2fa84f; }
.page { padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 170rpx) !important; }
</style>
