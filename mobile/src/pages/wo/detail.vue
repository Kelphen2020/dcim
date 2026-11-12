<template>
  <view class="page" v-if="w.id">
    <GlassNavBar title="工单详情" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>
    <view class="card">
      <view class="row">
        <text class="code">{{ w.code }}</text>
        <text class="grow"></text>
        <text class="st" :class="'ws' + w.status">{{ stText(w.status) }}</text>
      </view>
      <view class="title">{{ w.title }}</view>
      <view class="row tags">
        <text class="tag" :class="prioCls(w.priority)">{{ prioText(w.priority) }}</text>
        <text class="tag type">{{ w.type || 'task' }}</text>
        <text class="sla" :class="slaCls(w)" v-if="slaText(w)">{{ slaText(w) }}</text>
      </view>
      <view class="desc" v-if="w.description">{{ w.description }}</view>
      <view class="muted meta">
        <view v-if="w.resolution">处理结果：{{ w.resolution }}</view>
        <view>创建：{{ dt(w.created_at) }}</view>
        <view v-if="w.accepted_at">受理：{{ dt(w.accepted_at) }}</view>
        <view v-if="w.finished_at">完成：{{ dt(w.finished_at) }}</view>
      </view>
    </view>

    <!-- 现场照片（附件真闭环：服务器持久化，重进可见） -->
    <template v-if="attached.length">
      <view class="sec-title">现场照片（{{ attached.length }}）</view>
      <view class="card">
        <view class="photos">
          <view class="photo" v-for="(a, i) in attached" :key="a.id">
            <image :src="imgUrl(a.url)" mode="aspectFill" class="pimg" @tap="previewAttached(i)" />
          </view>
        </view>
        <view class="muted att-meta">照片已存服务器，随时可查</view>
      </view>
    </template>

    <!-- 操作区 -->
    <view class="card" v-if="w.status === 1 || w.status === 2">
      <view class="ops">
        <button v-if="w.status === 1" class="btn-primary op" @tap="accept">受理工单</button>
        <button v-if="w.status === 2" class="btn-primary op" @tap="finishPanel = true">完成工单</button>
        <button class="btn-ghost op" @tap="openReassign">改派</button>
        <button class="btn-ghost op" @tap="goSpare">领用备件</button>
        <button class="btn-ghost op danger" @tap="cancel">取消工单</button>
      </view>
    </view>

    <!-- 流转日志 -->
    <view class="sec-title">流转日志</view>
    <view class="card">
      <view v-if="!logs.length" class="empty-tip">暂无流转记录</view>
      <view class="log-item" v-for="l in logs" :key="l.id">
        <view class="dot"></view>
        <view class="grow">
          <view>{{ l.username || '系统' }} · {{ actText(l.action) }}</view>
          <view class="muted" v-if="l.content">{{ l.content }}</view>
          <view class="muted">{{ dt(l.created_at) }}</view>
        </view>
      </view>
    </view>

    <!-- 完成面板 -->
    <view class="mask" v-if="finishPanel" @tap.self="finishPanel = false">
      <view class="sheet">
        <view class="sheet-title">完成工单</view>
        <textarea v-model="resolution" class="ta" placeholder="处理说明（必填）：故障原因、处理过程、更换部件…" maxlength="500" />
        <view class="photos">
          <view class="photo" v-for="(p, i) in photos" :key="i">
            <image :src="imgUrl(p)" mode="aspectFill" class="pimg" />
            <text class="del" @tap="photos.splice(i, 1)">×</text>
          </view>
          <view class="photo add" v-if="photos.length < 9" @tap="takePhoto"><TechIcon name="scan" color="#8A94A6" :size="44" /></view>
        </view>
        <view class="row sheet-ops">
          <button class="btn-ghost grow" @tap="finishPanel = false">取消</button>
          <button class="btn-primary grow" :disabled="!resolution.trim()" @tap="doFinish">提交完成</button>
        </view>
      </view>
    </view>

    <!-- 改派面板 -->
    <view class="mask" v-if="reassignPanel" @tap.self="reassignPanel = false">
      <view class="sheet">
        <view class="sheet-title">改派给</view>
        <view class="user-item" v-for="u in users" :key="u.id" @tap="doReassign(u.id)">
          <text>{{ u.display_name || u.username }}</text>
          <text class="muted">{{ u.username }} · {{ roleText(u.role) }}</text>
        </view>
        <view v-if="!users.length" class="empty-tip">无可选人员</view>
      </view>
    </view>
    <GlassTabBar current="wo" :scrollY="scrollY" />
  </view>
  <view v-else class="empty-tip">工单不存在</view>
</template>

<script>
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import TechIcon from '../../components/TechIcon.vue'
import { get, post, upload } from '../../api.js'
import { CONFIG } from '../../config.js'
import { requireAuth, WO_STATUS, WO_PRIORITY, ROLE_TEXT, slaLeft } from '../../util.js'

export default {
  components: { TechIcon , GlassNavBar, GlassTabBar },
  data() {
    return {
      scrollY: 0,
      navH: 44,
      id: 0, w: {}, logs: [], users: [], attached: [],
      finishPanel: false, resolution: '', photos: [],
      reassignPanel: false,
    }
  },
  onLoad(q) {
    try { this.navH = (uni.getSystemInfoSync().statusBarHeight || 0) + 44 } catch (e) {}
    if (!requireAuth()) return
    this.id = Number(q.id || 0)
    const cached = uni.getStorageSync('dcim_wo_cur')
    if (cached && cached.id === this.id) this.w = cached
    this.load()
  },
  onPageScroll(e) { this.scrollY = e.scrollTop },
  methods: {
    stText: (s) => WO_STATUS[s] || '',
    prioText: (p) => WO_PRIORITY[p] || '',
    roleText: (r) => ROLE_TEXT[r] || r,
    prioCls(p) { return { 1: 'p-urgent', 2: 'p-high', 3: 'p-mid', 4: 'p-low' }[p] || 'p-low' },
    dt(t) { return t ? String(t).replace('T', ' ').slice(0, 19) : '' },
    slaText(w) {
      if (w.sla_respond_breach || w.sla_resolve_breach) return 'SLA 已违约'
      const due = w.status === 1 ? w.sla_respond_due : w.sla_resolve_due
      return slaLeft(due)
    },
    slaCls(w) {
      if (w.sla_respond_breach || w.sla_resolve_breach) return 'breach'
      if (w.sla_respond_warned || w.sla_resolve_warned) return 'warn'
      return ''
    },
    actText(a) {
      return { create: '创建', accept: '受理', finish: '完成', close: '关闭', cancel: '取消', reassign: '改派', reopen: '重开' }[a] || a
    },
    imgUrl(p) { return p.startsWith('http') ? p : CONFIG.BASE_URL + p },
    async load() {
      try {
        const d = await get('/work-orders', { id: this.id })
        const hit = (d.items || [])[0]
        if (hit) { this.w = hit; uni.setStorageSync('dcim_wo_cur', hit) }
      } catch (e) { /* ignore */ }
      try {
        const l = await get(`/work-orders-logs/${this.id}`)
        this.logs = l.items || l || []
      } catch (e) { /* ignore */ }
      // M6: 拉取服务器持久化的附件（照片真闭环）
      try {
        const a = await get('/mobile/attachments', { ref_type: 'work_order', ref_id: String(this.id) }, { silent: true })
        this.attached = a.items || []
      } catch (e) { /* ignore */ }
    },
    previewAttached(i) {
      uni.previewImage({ urls: this.attached.map((a) => this.imgUrl(a.url)), current: i })
    },
    async accept() {
      try {
        await post(`/work-orders/${this.id}/accept`, {})
        uni.showToast({ title: '已受理', icon: 'success' })
        this.load()
      } catch (e) { /* toast */ }
    },
    async doFinish() {
      let res = this.resolution.trim()
      if (this.photos.length) res += '\n[现场照片] ' + this.photos.join(' ')
      try {
        await post(`/work-orders/${this.id}/finish`, { resolution: res })
        uni.showToast({ title: '工单已完成', icon: 'success' })
        this.finishPanel = false
        this.resolution = ''; this.photos = []
        this.load() // 重新拉附件列表，照片墙立即可见
      } catch (e) { /* toast */ }
    },
    async cancel() {
      const r = await new Promise((res) => uni.showModal({ title: '取消工单', content: '确定取消该工单？', success: res, fail: () => res({ confirm: false }) }))
      if (!r.confirm) return
      try {
        await post(`/work-orders/${this.id}/cancel`, {})
        uni.showToast({ title: '已取消', icon: 'success' })
        this.load()
      } catch (e) { /* toast */ }
    },
    async openReassign() {
      this.reassignPanel = true
      if (!this.users.length) {
        try {
          const d = await get('/users-options')
          this.users = d.items || d || []
        } catch (e) { /* ignore */ }
      }
    },
    // M6-F2: 领用备件（跳备件tab，预填工单ID）
    goSpare() {
      uni.navigateTo({ url: '/pages/maint/maint?tab=spare&wo=' + this.w.id, fail: () => {} })
    },
    async doReassign(uid) {
      try {
        await post(`/work-orders/${this.id}/reassign`, { assignee_id: uid })
        uni.showToast({ title: '已改派', icon: 'success' })
        this.reassignPanel = false
        this.load()
      } catch (e) { /* toast */ }
    },
    takePhoto() {
      uni.chooseImage({
        count: 9 - this.photos.length,
        sizeType: ['compressed'],
        success: async (r) => {
          uni.showLoading({ title: '上传中' })
          try {
            for (const fp of r.tempFilePaths) {
              const d = await upload(fp, { ref_type: 'work_order', ref_id: String(this.id) })
              this.photos.push(d.url)
            }
          } catch (e) { /* toast */ } finally {
            uni.hideLoading()
          }
        },
      })
    },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 40rpx; }
.code { font-size: 24rpx; color: #8a94a6; font-family: monospace; }
.title { font-size: 34rpx; font-weight: 700; margin: 12rpx 0; line-height: 1.4; }
.tags { margin-bottom: 12rpx; }
.tag { color: #fff; background: #8a94a6; }
.p-urgent { background: #e5484d; }
.p-high { background: #f5a623; }
.p-mid { background: #3b82f6; }
.p-low { background: #8a94a6; }
.tag.type { background: #eef2f8; color: #4b5563; }
.st { font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 8rpx; background: #eef2f8; color: #4b5563; }
.ws1 { background: #fff1f0; color: #e5484d; }
.ws2 { background: #fff7e6; color: #d48806; }
.ws3 { background: #eef9f0; color: #2fa84f; }
.sla { font-size: 24rpx; color: #2fa84f; margin-left: auto; }
.sla.warn { color: #f5a623; font-weight: 700; }
.sla.breach { color: #e5484d; font-weight: 700; }
.desc { font-size: 28rpx; color: #4b5563; line-height: 1.5; background: #f4f6fa; border-radius: 12rpx; padding: 16rpx; margin-bottom: 12rpx; }
.meta view { margin-top: 6rpx; }
.sec-title { font-size: 30rpx; font-weight: 700; margin: 20rpx 4rpx 14rpx; }
.att-meta { font-size: 22rpx; margin-top: 10rpx; }
.ops { display: flex; flex-wrap: wrap; }
.op { flex: 1; min-width: 260rpx; margin: 8rpx; font-size: 28rpx; padding: 0; line-height: 2.6; }
.op.danger { color: #e5484d; border-color: #e5484d; }
.log-item { display: flex; padding: 14rpx 0; border-bottom: 1rpx solid #f0f3f8; }
.log-item:last-child { border-bottom: none; }
.dot { width: 14rpx; height: 14rpx; border-radius: 50%; background: #1f4e9c; margin: 12rpx 18rpx 0 4rpx; flex-shrink: 0; }
.mask { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.35); z-index: 100; display: flex; align-items: flex-end; }
.sheet { width: 100%; background: #fff; border-radius: 28rpx 28rpx 0 0; padding: 32rpx; box-sizing: border-box; max-height: 80vh; overflow-y: auto; }
.sheet-title { font-size: 32rpx; font-weight: 700; margin-bottom: 20rpx; }
.ta { width: 100%; height: 180rpx; background: #f4f6fa; border-radius: 12rpx; padding: 16rpx; box-sizing: border-box; font-size: 28rpx; }
.photos { display: flex; flex-wrap: wrap; margin: 20rpx 0; }
.photo { position: relative; width: 150rpx; height: 150rpx; margin: 0 16rpx 16rpx 0; border-radius: 12rpx; overflow: hidden; }
.pimg { width: 100%; height: 100%; }
.photo .del { position: absolute; top: 0; right: 0; background: rgba(0, 0, 0, 0.5); color: #fff; padding: 4rpx 12rpx; font-size: 22rpx; }
.photo.add { background: #f4f6fa; border: 2rpx dashed #c9d4e8; display: flex; align-items: center; justify-content: center; font-size: 48rpx; box-sizing: border-box; }
.sheet-ops { gap: 20rpx; margin-top: 10rpx; }
.sheet-ops button { margin: 0; }
.user-item { display: flex; justify-content: space-between; padding: 24rpx 8rpx; border-bottom: 1rpx solid #f0f3f8; font-size: 28rpx; }
.page { padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 170rpx) !important; }
</style>
