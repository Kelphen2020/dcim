<template>
  <view class="page">
    <GlassNavBar title="资产速查" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>
    <view class="search-bar">
      <input class="in" v-model="kw" placeholder="搜索名称/编码/SN/品牌/型号" confirm-type="search" @confirm="reload" />
      <view class="s-btn" @tap="reload">搜索</view>
      <view class="s-btn scan" @tap="onScan">扫码</view>
    </view>
    <scroll-view scroll-x class="chips">
      <view v-for="c in chips" :key="c.k" class="chip" :class="{ active: cur === c.k }" @tap="setChip(c.k)">{{ c.t }}</view>
    </scroll-view>

    <CylList :items="items" :scrollY="scrollY" :itemH="128" @pick="(a) => goDetail(a)">
      <template #empty>
        <view v-if="loading" class="empty-tip">加载中…</view>
        <view v-else class="empty-tip">没有匹配的资产</view>
      </template>
      <template #item="{ row: a }">
        <view class="card a-item">
          <view class="a-head">
            <view class="a-ico" :style="{ background: catBg(a.category) }">
              <TechIcon :name="catIcon(a.category)" color="#fff" :size="34" />
            </view>
            <view class="grow a-hmain">
              <view class="row">
                <text class="a-name grow">{{ a.name }}</text>
                <text class="tag" :class="'as' + a.status">{{ stText(a.status) }}</text>
              </view>
              <view class="a-sub">
                <text class="cat-tag">{{ catText(a.category) }}</text>
                <text class="mono-code">{{ a.code }}</text>
              </view>
            </view>
          </view>
          <view class="a-facts">
            <view class="fact" v-if="a.brand || a.model"><TechIcon name="box" color="#8A94A6" :size="24" /><text>{{ [a.brand, a.model].filter(Boolean).join(' ') }}</text></view>
            <view class="fact" v-if="a.sn"><TechIcon name="tag" color="#8A94A6" :size="24" /><text>SN {{ a.sn }}</text></view>
            <view class="fact" v-if="locText(a)"><TechIcon name="pin" color="#8A94A6" :size="24" /><text>{{ locText(a) }}</text></view>
            <view class="fact" v-if="a.ip"><TechIcon name="globe" color="#8A94A6" :size="24" /><text class="mono-code">{{ a.ip }}</text></view>
            <view class="fact" v-if="a.power_w"><TechIcon name="bolt" color="#8A94A6" :size="24" /><text>{{ a.power_w }}W</text></view>
          </view>
        </view>
      </template>
    </CylList>
    <view v-if="hasMore && items.length" class="load-more" @tap="loadMore">加载更多</view>
    <GlassTabBar current="me" :scrollY="scrollY" />
  </view>
</template>

<script>
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import TechIcon from '../../components/TechIcon.vue'
import CylList from '../../components/CylList.vue'
import { get } from '../../api.js'
import { requireAuth } from '../../util.js'

export default {
  components: { GlassNavBar, GlassTabBar, TechIcon, CylList },
  data() {
    return {
      scrollY: 0,
      navH: 44,
      kw: '', cur: 'all', cat: '', page: 1, size: 20, total: 0, items: [], loading: false,
      chips: [
        { k: 'all', t: '全部' },
        { k: '1', t: '在架' },
        { k: '2', t: '在库' },
        { k: '3', t: '维修' },
        { k: '4', t: '报废' },
      ],
    }
  },
  computed: { hasMore() { return this.items.length < this.total } },
  onLoad(q) {
    try { this.navH = (uni.getSystemInfoSync().statusBarHeight || 0) + 44 } catch (e) {}
    if (!requireAuth()) return
    if (q && q.kw) this.kw = decodeURIComponent(q.kw)
    // M6 深链：工作台设备概览穿透（状态/分类过滤）
    if (q && q.status && ['1', '2', '3', '4'].includes(String(q.status))) this.cur = String(q.status)
    if (q && q.category) this.cat = String(q.category)
    this.reload()
  },
  onPullDownRefresh() { this.reload().finally(() => uni.stopPullDownRefresh()) },
  onShow() {
    // H5 hash 路由复用页面实例时 onLoad 不重触发，从 URL 兜底解析深链过滤
    // #ifdef H5
    try {
      const qs = (location.hash.split('?')[1] || '')
      const p = new URLSearchParams(qs)
      const st = p.get('status'), ct = p.get('category')
      const sig = (st || '') + '|' + (ct || '')
      if (sig !== '|' && sig !== this._deepSig) {
        this._deepSig = sig
        if (st && ['1', '2', '3', '4'].includes(st)) this.cur = st
        if (ct) this.cat = ct
        this.reload()
      }
    } catch (e) { /* ignore */ }
    // #endif
  },
  onPageScroll(e) { this.scrollY = e.scrollTop },
  methods: {
    stText(s) { return { 1: '在架', 2: '在库', 3: '维修', 4: '报废' }[s] || '' },
    catText(c) { return { server: '服务器', network: '网络设备', storage: '存储', power: '供配电', security: '安防', other: '其他' }[c] || c || '未分类' },
    catIcon(c) { return { server: 'server', network: 'globe', storage: 'box', power: 'bolt', security: 'shield', other: 'cpu' }[c] || 'cpu' },
    catBg(c) { return { server: '#2D6BC4', network: '#0E7490', storage: '#7C3AED', power: '#D97706', security: '#DC2626', other: '#64748B' }[c] || '#64748B' },
    locText(a) {
      const bits = []
      if (a.site_name) bits.push(a.site_name)
      if (a.room_name) bits.push(a.room_name)
      if (a.cabinet_code) bits.push(a.cabinet_code + (a.start_u ? ' U' + a.start_u : ''))
      return bits.join(' / ')
    },
    setChip(k) { this.cur = k; this.reload() },
    async reload() { this.page = 1; await this.fetch(true) },
    async loadMore() { this.page += 1; await this.fetch(false) },
    async fetch(reset) {
      this.loading = true
      try {
        const p = { page: this.page, size: this.size, enrich: 1 }
        if (this.kw.trim()) p.keyword = this.kw.trim()
        if (this.cur !== 'all') p.status = this.cur
        if (this.cat) p.category = this.cat
        const d = await get('/assets', p)
        this.total = d.total || 0
        // enrich=1 返回 {asset, site_name, room_name, cabinet_code}，展平供模板使用
        const rows = (d.items || []).map((r) => (r.asset ? { ...r.asset, site_name: r.site_name, room_name: r.room_name, cabinet_code: r.cabinet_code } : r))
        this.items = reset ? rows : this.items.concat(rows)
      } catch (e) { /* toast */ } finally { this.loading = false }
    },
    goDetail(a) {
      uni.setStorageSync('dcim_asset_cur', a)
      uni.navigateTo({ url: '/pages/asset/detail?id=' + a.id })
    },
    onScan() {
      // 资产码内容约定为资产 code 或 AST|id 形式；扫到即当关键词搜索
      // #ifdef MP-WEIXIN || APP-PLUS
      uni.scanCode({ success: (r) => this.applyScan(r.result) })
      // #endif
      // #ifdef H5
      uni.showModal({
        title: '输入资产码', editable: true, placeholderText: '粘贴资产编码',
        success: (r) => { if (r.confirm) this.applyScan(r.content) },
      })
      // #endif
    },
    applyScan(text) {
      const t = (text || '').trim()
      if (!t) return
      // DCIM-SCAN 巡检令牌 → 巡检页
      if (t.startsWith('DCIM-SCAN|')) {
        uni.navigateTo({ url: '/pages/scan/exec?token=' + encodeURIComponent(t) })
        return
      }
      this.kw = t
      this.reload()
    },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 40rpx; }
.search-bar { display: flex; align-items: center; margin-bottom: 16rpx; }
.in { flex: 1; background: #fff; border-radius: 14rpx; padding: 18rpx 22rpx; font-size: 27rpx; }
.s-btn { margin-left: 14rpx; background: #1f4e9c; color: #fff; border-radius: 14rpx; padding: 16rpx 26rpx; font-size: 26rpx; }
.s-btn.scan { background: #2fa84f; }
.chips { white-space: nowrap; margin-bottom: 16rpx; }
.chip { display: inline-block; padding: 10rpx 28rpx; margin-right: 14rpx; background: #fff; border-radius: 28rpx; font-size: 25rpx; color: #4b5563; border: 1rpx solid #e5eaf3; }
.chip.active { background: #1f4e9c; color: #fff; border-color: #1f4e9c; }
.a-name { font-size: 30rpx; font-weight: 600; }
.a-head { display: flex; align-items: center; }
.a-ico { width: 76rpx; height: 76rpx; border-radius: 20rpx; display: flex; align-items: center; justify-content: center; flex-shrink: 0; margin-right: 18rpx; }
.a-hmain { min-width: 0; }
.a-sub { display: flex; align-items: center; margin-top: 8rpx; }
.cat-tag { font-size: 21rpx; color: #2d6bc4; background: #eef4fd; border-radius: 6rpx; padding: 2rpx 12rpx; margin-right: 12rpx; }
.mono-code { font-size: 22rpx; color: #8a94a6; font-family: monospace; }
.a-facts { display: flex; flex-wrap: wrap; margin-top: 14rpx; padding-top: 14rpx; border-top: 1rpx solid #f0f3f8; }
.fact { display: flex; align-items: center; font-size: 22rpx; color: #6b7686; margin: 4rpx 22rpx 4rpx 0; }
.fact text { margin-left: 6rpx; max-width: 340rpx; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.line2 { margin: 8rpx 0; font-size: 24rpx; }
.tag { font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 8rpx; }
.as1 { background: #eef9f0; color: #2fa84f; }
.as2 { background: #eef2f8; color: #4b5563; }
.as3 { background: #fff7e6; color: #d48806; }
.as4 { background: #f3f4f6; color: #8a94a6; }
.load-more { text-align: center; color: #1f4e9c; padding: 24rpx; font-size: 26rpx; }
.page { padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 170rpx) !important; }
</style>
