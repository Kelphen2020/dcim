<template>
  <view class="page">
    <GlassNavBar title="新建工单" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>

    <view class="card">
      <view class="fld">
        <view class="lbl">站点 <text class="req">*</text></view>
        <view class="picker" @tap="pickSite">
          <text :class="{ ph: !siteId }">{{ siteName || '选择站点' }}</text>
          <text class="caret">▾</text>
        </view>
      </view>
      <view class="fld">
        <view class="lbl">标题 <text class="req">*</text></view>
        <input class="in" v-model="title" placeholder="故障/任务简述" maxlength="100" />
      </view>
      <view class="fld">
        <view class="lbl">类型</view>
        <view class="chips-row">
          <view v-for="(t, k) in TYPES" :key="k" class="chip" :class="{ active: type === k }" @tap="type = k">{{ t }}</view>
        </view>
      </view>
      <view class="fld">
        <view class="lbl">优先级</view>
        <view class="chips-row">
          <view v-for="(p, i) in PRIOS" :key="i" class="chip" :class="[prioCls(i + 1), { active: priority === i + 1 }]" @tap="priority = i + 1">{{ p }}</view>
        </view>
      </view>
      <view class="fld">
        <view class="lbl">指派给</view>
        <view class="picker" @tap="pickAssignee">
          <text :class="{ ph: !assigneeId }">{{ assigneeName || '不指派（值班自动接管）' }}</text>
          <text class="caret">▾</text>
        </view>
      </view>
      <view class="fld" v-if="ctxText">
        <view class="lbl">关联对象</view>
        <view class="ctx-line"><TechIcon name="link" color="#2D6BC4" :size="26" /> {{ ctxText }}</view>
      </view>
      <view class="fld">
        <view class="lbl">描述</view>
        <textarea class="ta" v-model="description" placeholder="现象、位置、已采取措施…" maxlength="1000" />
      </view>
      <view class="fld">
        <view class="lbl">现场照片（{{ photos.length }}/9）</view>
        <view class="photos">
          <view class="photo" v-for="(p, i) in photos" :key="i">
            <image :src="p" mode="aspectFill" class="pimg" />
            <text class="del" @tap="photos.splice(i, 1)">×</text>
          </view>
          <view class="photo add" v-if="photos.length < 9" @tap="takePhoto"><TechIcon name="scan" color="#8A94A6" :size="40" /></view>
        </view>
      </view>
    </view>

    <button class="btn-primary submit" :disabled="submitting || !canSubmit" @tap="submit">
      {{ submitting ? '提交中…' : '创建工单' }}
    </button>
    <GlassTabBar current="wo" :scrollY="scrollY" />
  </view>
</template>

<script>
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import TechIcon from '../../components/TechIcon.vue'
import { get, post, upload } from '../../api.js'
import { requireAuth, user } from '../../util.js'

export default {
  components: { GlassNavBar, GlassTabBar, TechIcon },
  data() {
    return {
      scrollY: 0,
      navH: 44,
      TYPES: { repair: '故障维修', alarm: '告警工单', inspection: '巡检异常', change: '变更工单', task: '任务', other: '其他' },
      PRIOS: ['紧急', '高', '中', '低'],
      sites: [], siteId: 0,
      title: '', type: 'repair', priority: 3, description: '',
      assigneeId: 0, assigneeName: '',
      users: [],
      // 预填上下文（深链带入）
      assetId: 0, assetName: '', cabinetId: 0, alertId: 0,
      photos: [],
      submitting: false,
    }
  },
  computed: {
    siteName() {
      const s = this.sites.find((x) => x.id === this.siteId)
      return s ? s.name : ''
    },
    canSubmit() { return this.siteId > 0 && this.title.trim().length >= 2 },
    ctxText() {
      const bits = []
      if (this.assetName) bits.push('资产 ' + this.assetName)
      if (this.alertId) bits.push('告警 #' + this.alertId)
      return bits.join(' · ')
    },
  },
  onLoad(q) {
    try { this.navH = (uni.getSystemInfoSync().statusBarHeight || 0) + 44 } catch (e) {}
    if (!requireAuth()) return
    // 深链预填：?site_id=&title=&description=&asset_id=&asset_name=&alert_id=&type=&priority=
    if (q) {
      this.siteId = Number(q.site_id || 0)
      if (q.title) this.title = decodeURIComponent(q.title)
      if (q.description) this.description = decodeURIComponent(q.description)
      this.assetId = Number(q.asset_id || 0)
      if (q.asset_name) this.assetName = decodeURIComponent(q.asset_name)
      this.cabinetId = Number(q.cabinet_id || 0)
      this.alertId = Number(q.alert_id || 0)
      if (q.type && this.TYPES[q.type]) this.type = q.type
      if (q.priority) this.priority = Math.min(4, Math.max(1, Number(q.priority) || 3))
    }
    this.loadSites()
    this.loadUsers()
  },
  onPageScroll(e) { this.scrollY = e.scrollTop },
  methods: {
    prioCls(p) { return ['p-urgent', 'p-high', 'p-mid', 'p-low'][p - 1] || 'p-low' },
    async loadSites() {
      try {
        const d = await get('/sites', { size: 100 }, { silent: true })
        this.sites = d.items || []
        if (!this.siteId && this.sites.length === 1) this.siteId = this.sites[0].id
      } catch (e) { /* ignore */ }
    },
    async loadUsers() {
      try {
        const d = await get('/users-options', {}, { silent: true })
        this.users = d.items || d || []
      } catch (e) { /* ignore */ }
    },
    pickSite() {
      if (!this.sites.length) { uni.showToast({ title: '无可用站点', icon: 'none' }); return }
      uni.showActionSheet({
        itemList: this.sites.map((s) => s.name),
        success: (r) => { this.siteId = this.sites[r.tapIndex].id },
        fail: () => {},
      })
    },
    pickAssignee() {
      const names = ['不指派', ...this.users.map((u) => u.display_name || u.username)]
      uni.showActionSheet({
        itemList: names,
        success: (r) => {
          if (r.tapIndex === 0) { this.assigneeId = 0; this.assigneeName = '' }
          else {
            const u = this.users[r.tapIndex - 1]
            this.assigneeId = u.id
            this.assigneeName = u.display_name || u.username
          }
        },
        fail: () => {},
      })
    },
    takePhoto() {
      uni.chooseImage({
        count: 9 - this.photos.length,
        sizeType: ['compressed'],
        success: (r) => {
          // 先暂存本地路径，创建成功后统一上传（此时才有 ref_id）
          this.photos = this.photos.concat(r.tempFilePaths)
        },
      })
    },
    async submit() {
      if (!this.canSubmit || this.submitting) return
      this.submitting = true
      try {
        const body = {
          site_id: this.siteId,
          title: this.title.trim(),
          type: this.type,
          priority: this.priority,
          description: this.description.trim(),
        }
        if (this.assetId) body.asset_id = this.assetId
        if (this.cabinetId) body.cabinet_id = this.cabinetId
        if (this.assigneeId) body.assignee_id = this.assigneeId
        const d = await post('/work-orders', body)
        const wid = d.id || (d.work_order && d.work_order.id)
        // 照片上传（真闭环：ref 指向新工单）
        let upN = 0
        for (const fp of this.photos) {
          try {
            await upload(fp, { ref_type: 'work_order', ref_id: String(wid) })
            upN++
          } catch (e) { /* 单张失败不阻断 */ }
        }
        uni.showToast({ title: upN ? `工单已建·${upN}张照片` : '工单已创建', icon: 'success' })
        setTimeout(() => {
          if (wid) uni.redirectTo({ url: '/pages/wo/detail?id=' + wid, fail: () => uni.navigateBack({ fail: () => {} }) })
          else uni.navigateBack({ fail: () => {} })
        }, 800)
      } catch (e) { /* toast 已弹 */ } finally {
        this.submitting = false
      }
    },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx calc(env(safe-area-inset-bottom, 0px) + 170rpx); }
.fld { margin-bottom: 24rpx; }
.lbl { font-size: 26rpx; color: #4b5563; font-weight: 600; margin-bottom: 12rpx; }
.req { color: #e5484d; }
.in { background: #f4f6fa; border-radius: 12rpx; padding: 18rpx 22rpx; font-size: 28rpx; }
.ta { background: #f4f6fa; border-radius: 12rpx; padding: 18rpx 22rpx; font-size: 27rpx; width: 100%; box-sizing: border-box; min-height: 150rpx; }
.picker { display: flex; align-items: center; justify-content: space-between; background: #f4f6fa; border-radius: 12rpx; padding: 18rpx 22rpx; font-size: 28rpx; }
.picker .ph { color: #b6bfce; }
.caret { color: #8a94a6; }
.chips-row { display: flex; flex-wrap: wrap; }
.chip { padding: 10rpx 24rpx; border-radius: 24rpx; font-size: 25rpx; background: #f4f6fa; color: #4b5563; margin: 0 12rpx 12rpx 0; border: 1rpx solid transparent; }
.chip.active { background: #1f4e9c; color: #fff; border-color: #1f4e9c; }
.chip.p-urgent.active { background: #e5484d; border-color: #e5484d; }
.chip.p-high.active { background: #f5a623; border-color: #f5a623; }
.chip.p-mid.active { background: #3b82f6; border-color: #3b82f6; }
.ctx-line { display: flex; align-items: center; gap: 8rpx; font-size: 26rpx; color: #2d6bc4; background: #eef4fd; border-radius: 10rpx; padding: 12rpx 16rpx; }
.photos { display: flex; flex-wrap: wrap; }
.photo { position: relative; width: 140rpx; height: 140rpx; margin: 0 12rpx 12rpx 0; border-radius: 12rpx; overflow: hidden; background: #f4f6fa; }
.pimg { width: 100%; height: 100%; }
.photo.add { display: flex; align-items: center; justify-content: center; border: 2rpx dashed #d5dce8; }
.del { position: absolute; top: 0; right: 0; background: rgba(0,0,0,.55); color: #fff; font-size: 24rpx; padding: 2rpx 12rpx; border-radius: 0 0 0 12rpx; }
.submit { margin-top: 10rpx; }
</style>
