<template>
  <view class="page" v-if="task.id">
    <GlassNavBar title="盘点执行" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>
    <view class="card">
      <view class="row">
        <text class="t-name grow">{{ task.name }}</text>
        <text class="tag" :class="'ts' + task.status">{{ stText(task.status) }}</text>
      </view>
      <view class="muted mono">{{ task.code }}</view>
      <view class="prog-row">
        <view class="prog-bg"><view class="prog-fg" :style="{ width: pct + '%' }"></view></view>
        <text class="prog-t">{{ task.checked_count }}/{{ task.total_count }}（{{ pct }}%）</text>
      </view>
      <view class="stats" v-if="stats.length">
        <text class="st-chip" v-for="s in stats" :key="s.result">{{ resText(s.result) }} {{ s.n }}</text>
      </view>
    </view>

    <view class="toolbar">
      <scroll-view scroll-x class="chips grow">
        <view v-for="f in filters" :key="f.k" class="chip" :class="{ active: curFilter === f.k }" @tap="setFilter(f.k)">{{ f.t }}</view>
      </scroll-view>
      <view class="s-btn" v-if="task.status === 1 && pending.length" @tap="submitAll">批量相符({{ pending.length }})</view>
    </view>

    <view v-if="!items.length" class="empty-tip">无明细</view>
    <view class="card i-item" v-for="it in items" :key="it.id" :class="{ checked: it.result > 0 }">
      <view class="row">
        <text class="i-name grow">{{ it.asset_name }}</text>
        <text class="tag res" :class="'r' + it.result" v-if="it.result > 0">{{ resText(it.result) }}</text>
      </view>
      <view class="muted mono">{{ it.asset_code }} · {{ it.cabinet_code || '?' }}柜 U{{ it.start_u }}</view>
      <view class="row i-links" v-if="it.asset_id">
        <text class="i-link" @tap="goAsset(it)">资产详情 ›</text>
      </view>
      <view class="ops" v-if="task.status === 1 && editable">
        <view class="op" :class="{ on: it._result === 1, ok: it._result === 1 }" @tap="mark(it, 1)">相符</view>
        <view class="op" :class="{ on: it._result === 2, bad: it._result === 2 }" @tap="mark(it, 2)">盘亏</view>
        <view class="op" :class="{ on: it._result === 4, warn: it._result === 4 }" @tap="mark(it, 4)">位置不符</view>
        <view class="op cam" @tap="takePhoto(it)"><TechIcon name="camera" color="#1F4E9C" :size="26" /> 拍照{{ it._photos && it._photos.length ? '(' + it._photos.length + ')' : '' }}</view>
        <input v-if="it._result === 2 || it._result === 4" class="remark-in" v-model="it._remark" placeholder="备注（可选）" />
        <view class="ph-row" v-if="it._photos && it._photos.length">
          <image v-for="(p, pi) in it._photos" :key="pi" :src="p" mode="aspectFill" class="ph-thumb" @tap="previewPhoto(it, pi)" />
        </view>
      </view>
      <!-- 已完成项的存证照片回显 -->
      <view class="ph-row ro" v-else-if="itemPhotos[it.id] && itemPhotos[it.id].length" @tap="previewSaved(it)">
        <image v-for="(p, pi) in itemPhotos[it.id]" :key="pi" :src="imgUrl(p)" mode="aspectFill" class="ph-thumb" />
        <text class="muted ph-tip">盘点存证 ×{{ itemPhotos[it.id].length }}</text>
      </view>
    </view>
    <view v-if="hasMore && items.length" class="load-more" @tap="loadMore">加载更多</view>

    <button v-if="task.status === 1" class="btn-primary submit" :disabled="!dirtyCount" @tap="saveMarks">
      保存核对结果（{{ dirtyCount }} 项待保存）
    </button>
    <GlassTabBar current="me" :scrollY="scrollY" />
  </view>
  <view v-else class="empty-tip">任务不存在</view>
</template>

<script>
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import TechIcon from '../../components/TechIcon.vue'
import { get, post, upload } from '../../api.js'
import { CONFIG } from '../../config.js'
import { requireAuth } from '../../util.js'

export default {
  components: { GlassNavBar, GlassTabBar, TechIcon },
  data() {
    return {
      scrollY: 0,
      navH: 44,
      id: 0, task: {}, stats: [], items: [], page: 1, size: 50, total: 0,
      curFilter: 'unchecked', editable: true,
      itemPhotos: {},
      filters: [
        { k: 'unchecked', t: '待核对' },
        { k: 'all', t: '全部' },
        { k: '1', t: '相符' },
        { k: '2', t: '盘亏' },
        { k: '4', t: '位置不符' },
      ],
    }
  },
  computed: {
    pct() { return this.task.total_count ? Math.round((this.task.checked_count / this.task.total_count) * 100) : 0 },
    hasMore() { return this.items.length < this.total },
    pending() { return this.items.filter((i) => !i.result) },
    dirtyCount() { return this.items.filter((i) => i._result && i._result !== i.result).length },
  },
  onLoad(q) {
    try { this.navH = (uni.getSystemInfoSync().statusBarHeight || 0) + 44 } catch (e) {}
    if (!requireAuth()) return
    this.id = Number(q.id || 0)
    this.loadTask()
    this.loadItems(true)
  },
  onPageScroll(e) { this.scrollY = e.scrollTop },
  methods: {
    stText(s) { return { 0: '草稿', 1: '盘点中', 2: '已完成' }[s] || '' },
    resText(r) { return { 0: '未核对', 1: '相符', 2: '盘亏', 3: '盘盈', 4: '位置不符' }[r] || '' },
    async loadTask() {
      try {
        const d = await get(`/stocktakes/${this.id}`, { page: 1, size: 1 })
        this.task = d.task || {}
        this.stats = d.stats || []
      } catch (e) { /* toast */ }
    },
    setFilter(k) { this.curFilter = k; this.loadItems(true) },
    async loadItems(reset) {
      if (reset) this.page = 1
      try {
        const p = { page: this.page, size: this.size }
        if (this.curFilter === 'unchecked') p.result = 0
        else if (this.curFilter !== 'all') p.result = this.curFilter
        const d = await get(`/stocktakes/${this.id}`, p)
        const list = (d.items || []).map((i) => ({ ...i, _result: i.result, _remark: i.remark || '', _photos: [] }))
        this.total = d.total || 0
        this.items = reset ? list : this.items.concat(list)
        this.loadItemPhotos()
      } catch (e) { /* toast */ }
    },
    // H2: 拉取已核对项的存证照片（ref=stocktake_item）
    async loadItemPhotos() {
      try {
        const d = await get('/mobile/attachments', { ref_type: 'stocktake_item' }, { silent: true })
        const map = {}
        for (const a of (d.items || [])) {
          if (!map[a.ref_id]) map[a.ref_id] = []
          map[a.ref_id].push(a.url)
        }
        this.itemPhotos = map
      } catch (e) { /* ignore */ }
    },
    imgUrl(u) { return u && u.startsWith('http') ? u : CONFIG.BASE_URL + u },
    goAsset(it) {
      uni.navigateTo({ url: '/pages/asset/detail?id=' + it.asset_id, fail: () => {} })
    },
    // H2: 盘点拍照（先本地暂存，随核对结果一并上传）
    takePhoto(it) {
      if (!it._photos) it._photos = []
      if (it._photos.length >= 6) { uni.showToast({ title: '每项最多6张', icon: 'none' }); return }
      uni.chooseImage({
        count: 6 - it._photos.length,
        sourceType: ['camera', 'album'],
        success: (res) => {
          it._photos = it._photos.concat(res.tempFilePaths || [])
        },
        fail: () => {},
      })
    },
    previewPhoto(it, pi) {
      uni.previewImage({ urls: it._photos, current: pi })
    },
    previewSaved(it) {
      const urls = (this.itemPhotos[it.id] || []).map((u) => this.imgUrl(u))
      if (urls.length) uni.previewImage({ urls, current: 0 })
    },
    loadMore() { this.page += 1; this.loadItems(false) },
    mark(it, r) {
      it._result = it._result === r ? 0 : r // 再点取消
    },
    submitAll() {
      uni.showModal({
        title: '批量核对', content: `将当前 ${this.pending.length} 项待核对资产标记为「相符」？`,
        success: async (res) => {
          if (!res.confirm) return
          this.pending.forEach((i) => { i._result = 1 })
          this.saveMarks()
        },
      })
    },
    async saveMarks() {
      const dirty = this.items.filter((i) => i._result && i._result !== i.result)
      if (!dirty.length) return
      const payload = dirty.map((i) => ({
        id: i.id, result: i._result, remark: i._remark || '',
      }))
      try {
        const r = await post(`/stocktakes/${this.id}/check`, { items: payload })
        // H2: 核对成功后上传存证照片（ref=stocktake_item:item_id）
        let phN = 0
        for (const i of dirty) {
          for (const p of (i._photos || [])) {
            try {
              await upload(p, { ref_type: 'stocktake_item', ref_id: String(i.id) })
              phN += 1
            } catch (e) { /* 单张失败不阻断 */ }
          }
        }
        uni.showToast({ title: phN ? `已保存 ${r.updated} 项+${phN}张照片` : `已保存 ${r.updated} 项`, icon: 'success' })
        this.loadTask()
        this.loadItems(true)
      } catch (e) { /* toast */ }
    },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 60rpx; }
.t-name { font-size: 32rpx; font-weight: 700; }
.mono { font-family: monospace; }
.tag { font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 8rpx; }
.ts0 { background: #f3f4f6; color: #8a94a6; }
.ts1 { background: #eef4ff; color: #1f4e9c; }
.ts2 { background: #eef9f0; color: #2fa84f; }
.prog-row { display: flex; align-items: center; margin-top: 14rpx; }
.prog-bg { flex: 1; height: 14rpx; background: #eef2f8; border-radius: 8rpx; overflow: hidden; }
.prog-fg { height: 100%; background: linear-gradient(90deg, #1f4e9c, #2d6bc4); }
.prog-t { margin-left: 16rpx; font-size: 24rpx; color: #4b5563; }
.stats { display: flex; flex-wrap: wrap; margin-top: 14rpx; }
.st-chip { font-size: 22rpx; background: #f4f6fa; color: #4b5563; border-radius: 8rpx; padding: 6rpx 16rpx; margin: 0 12rpx 8rpx 0; }
.toolbar { display: flex; align-items: center; margin: 16rpx 0; }
.chips { white-space: nowrap; }
.chip { display: inline-block; padding: 10rpx 24rpx; margin-right: 12rpx; background: #fff; border-radius: 26rpx; font-size: 25rpx; color: #4b5563; border: 1rpx solid #e5eaf3; }
.chip.active { background: #1f4e9c; color: #fff; border-color: #1f4e9c; }
.s-btn { background: #2fa84f; color: #fff; border-radius: 12rpx; padding: 12rpx 20rpx; font-size: 24rpx; margin-left: 12rpx; flex-shrink: 0; }
.i-item { padding: 20rpx 24rpx; }
.i-item.checked { opacity: 0.75; }
.i-name { font-size: 28rpx; font-weight: 600; }
.res { margin-left: 10rpx; }
.r1 { background: #eef9f0; color: #2fa84f; }
.r2 { background: #fff1f0; color: #e5484d; }
.r3 { background: #eef4ff; color: #1f4e9c; }
.r4 { background: #fff7e6; color: #d48806; }
.ops { display: flex; flex-wrap: wrap; align-items: center; margin-top: 14rpx; }
.op { padding: 10rpx 26rpx; border-radius: 10rpx; border: 2rpx solid #e5eaf3; margin-right: 16rpx; font-size: 25rpx; color: #4b5563; }
.op.on.ok { background: #2fa84f; border-color: #2fa84f; color: #fff; }
.op.on.bad { background: #e5484d; border-color: #e5484d; color: #fff; }
.op.on.warn { background: #f5a623; border-color: #f5a623; color: #fff; }
.remark-in { width: 100%; background: #f4f6fa; border-radius: 10rpx; padding: 12rpx; font-size: 25rpx; margin-top: 12rpx; box-sizing: border-box; }
.submit { margin-top: 24rpx; }
.i-links { margin-top: 8rpx; }
.i-link { font-size: 23rpx; color: #2d6bc4; }
.op.cam { display: flex; align-items: center; gap: 6rpx; }
.ph-row { display: flex; flex-wrap: wrap; gap: 12rpx; margin-top: 12rpx; align-items: center; }
.ph-row.ro { margin-top: 10rpx; }
.ph-thumb { width: 110rpx; height: 110rpx; border-radius: 10rpx; background: #f0f3f8; }
.ph-tip { font-size: 21rpx; }
.load-more { text-align: center; color: #1f4e9c; padding: 20rpx; font-size: 26rpx; }
.page { padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 170rpx) !important; }
</style>
