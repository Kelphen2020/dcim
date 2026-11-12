<template>
  <view class="page" v-if="a.id">
    <GlassNavBar title="资产详情" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>
    <view class="card">
      <view class="row head">
        <view class="grow">
          <view class="a-name">{{ a.name }}</view>
          <view class="muted mono">{{ a.code }}</view>
        </view>
        <text class="tag big" :class="'as' + a.status">{{ stText(a.status) }}</text>
      </view>
      <view class="loc-line" v-if="locText"><TechIcon name="pin" color="#8A94A6" :size="26" /> {{ locText }}</view>
      <view class="head-ops">
        <button v-if="a.status === 1 || a.status === 3" class="btn-ghost op-sm" @tap="toWo"><TechIcon name="wrench" color="#1F4E9C" :size="28" /> 报修开工单</button>
      </view>
    </view>

    <view class="sec-title">基本信息</view>
    <view class="card kv">
      <view class="kvi" v-for="(v, k) in infoFields" :key="k" v-show="v">
        <text class="k">{{ infoLabel[k] }}</text><text class="v">{{ v }}</text>
      </view>
    </view>

    <view class="sec-title">维保记录 <text class="muted">({{ maints.length }})</text></view>
    <view class="card" v-if="!maints.length"><view class="empty-tip sm">暂无维保记录</view></view>
    <view class="card m-item" v-for="m in maints" :key="m.id">
      <view class="row">
        <text class="tag" :class="m.status === 1 ? 'as1' : 'as2'">{{ m.status === 1 ? '生效' : '失效' }}</text>
        <text class="grow m-kind">{{ m.kind === 'contract' ? '维保合同' : '维修' }} {{ m.vendor || '' }}</text>
      </view>
      <view class="muted" v-if="m.end_at">到期 {{ String(m.end_at).slice(0, 10) }}<text v-if="daysLeft(m.end_at) !== null" :class="daysLeft(m.end_at) < 30 ? 'warn-t' : ''"> · 剩 {{ daysLeft(m.end_at) }} 天</text></view>
    </view>

    <view class="sec-title">关联工单 <text class="muted">({{ wos.length }})</text></view>
    <view class="card" v-if="!wos.length"><view class="empty-tip sm">暂无关联工单</view></view>
    <view class="card w-item" v-for="w in wos" :key="w.id" @tap="goWo(w)">
      <view class="row">
        <text class="mono muted">{{ w.code }}</text>
        <text class="grow"></text>
        <text class="tag" :class="'ws' + w.status">{{ woSt(w.status) }}</text>
      </view>
      <view class="w-title">{{ w.title }}</view>
    </view>
    <GlassTabBar current="me" :scrollY="scrollY" />
  </view>
  <view v-else class="empty-tip">资产不存在</view>
</template>

<script>
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import TechIcon from '../../components/TechIcon.vue'
import { get } from '../../api.js'
import { requireAuth, WO_STATUS } from '../../util.js'

export default {
  components: { GlassNavBar, GlassTabBar, TechIcon },
  data() {
    return {
      scrollY: 0,
      navH: 44,
      id: 0, a: {}, maints: [], wos: [],
      locText: '',
      infoLabel: {
        category: '分类', brand: '品牌', model: '型号', sn: '序列号', ip: '管理IP',
        asset_no: '资产编号', fixed_asset_no: '固资编号', vendor: '供应商',
        department: '所属部门', contract_no: '合同编号', dimensions: '尺寸(mm)',
        power_w: '额定功率(W)', weight_kg: '重量(kg)', owner: '负责人',
        purchase_at: '采购日期', remark: '备注',
      },
    }
  },
  computed: {
    infoFields() {
      const a = this.a
      return {
        category: a.category, brand: a.brand, model: a.model, sn: a.sn, ip: a.ip,
        asset_no: a.asset_no, fixed_asset_no: a.fixed_asset_no, vendor: a.vendor,
        department: a.department, contract_no: a.contract_no, dimensions: a.dimensions,
        power_w: a.power_w, weight_kg: a.weight_kg, owner: a.owner,
        purchase_at: a.purchase_at ? String(a.purchase_at).slice(0, 10) : '',
        remark: a.remark,
      }
    },
  },
  onLoad(q) {
    try { this.navH = (uni.getSystemInfoSync().statusBarHeight || 0) + 44 } catch (e) {}
    if (!requireAuth()) return
    this.id = Number(q.id || 0)
    const cached = uni.getStorageSync('dcim_asset_cur')
    if (cached && cached.id === this.id) this.a = cached
    this.load()
  },
  onPageScroll(e) { this.scrollY = e.scrollTop },
  methods: {
    stText(s) { return { 1: '在架', 2: '在库', 3: '维修', 4: '报废' }[s] || '' },
    woSt(s) { return WO_STATUS[s] || '' },
    daysLeft(end) {
      if (!end) return null
      const t = new Date(String(end).slice(0, 10).replace(/-/g, '/'))
      if (isNaN(t.getTime())) return null
      return Math.ceil((t.getTime() - Date.now()) / 86400000)
    },
    async load() {
      try {
        const d = await get('/assets', { id: this.id, enrich: 1 })
        const hit = (d.items || [])[0]
        if (hit) {
          const flat = hit.asset ? { ...hit.asset, site_name: hit.site_name, room_name: hit.room_name, cabinet_code: hit.cabinet_code } : hit
          this.a = flat
          uni.setStorageSync('dcim_asset_cur', flat)
          const bits = []
          if (flat.site_name) bits.push(flat.site_name)
          if (flat.room_name) bits.push(flat.room_name)
          if (flat.cabinet_code) bits.push(flat.cabinet_code + (flat.start_u ? ' · U' + flat.start_u : ''))
          this.locText = bits.join(' / ')
        }
      } catch (e) { /* ignore */ }
      try {
        const m = await get('/maintenances', { asset_id: this.id, size: 50 }, { silent: true })
        this.maints = m.items || []
      } catch (e) { /* ignore */ }
      try {
        const w = await get('/work-orders', { asset_id: this.id, size: 20 }, { silent: true })
        this.wos = w.items || []
      } catch (e) { /* ignore */ }
    },
    goWo(w) {
      uni.setStorageSync('dcim_wo_cur', w)
      uni.navigateTo({ url: '/pages/wo/detail?id=' + w.id })
    },
    // M6-D：资产一键报修（预填资产上下文）
    toWo() {
      const a = this.a
      const q = [
        'site_id=' + (a.site_id || 0),
        'type=repair',
        'asset_id=' + a.id,
        'asset_name=' + encodeURIComponent(a.name || ''),
        'cabinet_id=' + (a.cabinet_id || 0),
        'title=' + encodeURIComponent(a.name + ' 报修'),
        'description=' + encodeURIComponent('资产：' + (a.name || '') + '（' + (a.code || '') + '）' + (this.locText ? '\n位置：' + this.locText : '')),
      ].join('&')
      uni.navigateTo({ url: '/pages/wo/create?' + q })
    },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 40rpx; }
.head { align-items: flex-start; }
.a-name { font-size: 34rpx; font-weight: 700; }
.mono { font-family: monospace; }
.tag { font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 8rpx; }
.tag.big { font-size: 24rpx; padding: 6rpx 18rpx; }
.loc-line { display: flex; align-items: center; gap: 8rpx; font-size: 24rpx; color: #6b7686; margin-top: 14rpx; }
.head-ops { margin-top: 16rpx; }
.op-sm { display: flex; align-items: center; justify-content: center; gap: 8rpx; font-size: 26rpx; padding: 0 24rpx; line-height: 2.4; }
.as1 { background: #eef9f0; color: #2fa84f; }
.as2 { background: #eef2f8; color: #4b5563; }
.as3 { background: #fff7e6; color: #d48806; }
.as4 { background: #f3f4f6; color: #8a94a6; }
.sec-title { font-size: 30rpx; font-weight: 700; margin: 22rpx 4rpx 14rpx; }
.kv { padding: 8rpx 24rpx; }
.kvi { display: flex; padding: 16rpx 0; border-bottom: 1rpx solid #f0f3f8; font-size: 27rpx; }
.kvi:last-child { border-bottom: none; }
.kvi .k { width: 200rpx; color: #8a94a6; flex-shrink: 0; }
.kvi .v { flex: 1; color: #1f2937; word-break: break-all; }
.m-kind { font-size: 27rpx; }
.warn-t { color: #e5484d; font-weight: 700; }
.w-title { font-size: 28rpx; margin-top: 8rpx; }
.ws1 { background: #fff1f0; color: #e5484d; }
.ws2 { background: #fff7e6; color: #d48806; }
.ws3 { background: #eef9f0; color: #2fa84f; }
.empty-tip.sm { padding: 30rpx 0; }
.page { padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 170rpx) !important; }
</style>
