<template>
  <view class="page">
    <GlassNavBar title="值班日历" back :scrollY="scrollY" />
    <view :style="{ height: navH + 'px' }"></view>
    <view class="card cur-card" v-if="cur.on_duty">
      <view class="cur-ico"><TechIcon name="clock" color="#2D6BC4" :size="48" /></view>
      <view class="grow">
        <view class="cur-t">当班：{{ cur.user_name }}（{{ shiftText(cur.shift) }}）</view>
        <view class="muted">{{ cur.duty_date }} · {{ cur.phone || '' }}</view>
      </view>
      <view class="mine-tag" v-if="cur.mine">我</view>
    </view>
    <view class="card cur-card" v-else>
      <view class="cur-ico"><TechIcon name="user" color="#8A94A6" :size="48" /></view>
      <view class="grow"><view class="cur-t">当前无人值班</view></view>
    </view>

    <view class="row month-nav">
      <text class="nav-btn" @tap="shiftMonth(-1)">‹</text>
      <text class="month">{{ month }}</text>
      <text class="nav-btn" @tap="shiftMonth(1)">›</text>
      <text class="grow"></text>
      <text class="ho-btn" @tap="goHandover">交接班 ›</text>
    </view>

    <view class="card cal">
      <view class="cal-head">
        <text v-for="w in ['一', '二', '三', '四', '五', '六', '日']" :key="w" class="cal-w">{{ w }}</text>
      </view>
      <view class="cal-grid">
        <view v-for="(cell, i) in calCells" :key="i" class="cal-cell" :class="{ today: cell.today, other: !cell.inMonth }">
          <text class="cal-d">{{ cell.day || '' }}</text>
          <view class="cal-s" v-if="cell.shifts.length">
            <text v-for="(s, j) in cell.shifts" :key="j" class="s-dot" :class="s.cls" :title="s.name">{{ s.tag }}</text>
          </view>
        </view>
      </view>
      <view class="legend">
        <text class="lg"><text class="s-dot day">白</text>白班</text>
        <text class="lg"><text class="s-dot night">夜</text>夜班</text>
        <text class="lg"><text class="s-dot full">全</text>全天</text>
      </view>
    </view>

    <view class="sec-title">本月我的值班</view>
    <view class="card" v-if="!myShifts.length"><view class="empty-tip sm">本月无排班</view></view>
    <view class="card my-item" v-for="s in myShifts" :key="s.id">
      <text>{{ s.duty_date }}</text>
      <text class="tag" :class="s.shift">{{ shiftText(s.shift) }}</text>
      <text class="muted grow right">{{ s.site_id === 0 ? '全部站点' : '站点#' + s.site_id }}</text>
    </view>
    <GlassTabBar current="me" :scrollY="scrollY" />
  </view>
</template>

<script>
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import TechIcon from '../../components/TechIcon.vue'
import { get } from '../../api.js'
import { requireAuth, user, SHIFT_TEXT } from '../../util.js'

export default {
  components: { TechIcon , GlassNavBar, GlassTabBar },
  data() {
    const now = new Date()
    return {
      scrollY: 0,
      navH: 44,
      month: `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`,
      shifts: [],
      cur: { on_duty: false },
    }
  },
  computed: {
    myShifts() {
      const uid = user().id
      return this.shifts.filter((s) => s.user_id === uid)
    },
    calCells() {
      const [y, m] = this.month.split('-').map(Number)
      const first = new Date(y, m - 1, 1)
      const startW = (first.getDay() + 6) % 7 // 周一开头
      const days = new Date(y, m, 0).getDate()
      const today = new Date()
      const todayStr = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`
      const cells = []
      for (let i = 0; i < startW; i++) cells.push({ inMonth: false, day: '', shifts: [] })
      for (let d = 1; d <= days; d++) {
        const ds = `${this.month}-${String(d).padStart(2, '0')}`
        const dayShifts = this.shifts.filter((s) => String(s.duty_date).slice(0, 10) === ds)
        cells.push({
          inMonth: true, day: d, today: ds === todayStr,
          shifts: dayShifts.slice(0, 2).map((s) => ({
            tag: { day: '白', night: '夜', full: '全' }[s.shift] || '班',
            cls: s.shift,
            name: s.user_name || '',
          })),
        })
      }
      return cells
    },
  },
  onLoad() {
    try { this.navH = (uni.getSystemInfoSync().statusBarHeight || 0) + 44 } catch (e) {}
  },
  onPageScroll(e) { this.scrollY = e.scrollTop },
  onShow() {
    if (!requireAuth()) return
    this.load()
  },
  methods: {
    shiftText: (s) => SHIFT_TEXT[s] || s,
    shiftMonth(delta) {
      const [y, m] = this.month.split('-').map(Number)
      const d = new Date(y, m - 1 + delta, 1)
      this.month = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
      this.load()
    },
    async load() {
      try {
        const d = await get('/duty-shifts', { month: this.month, size: 200 }, { silent: true })
        const items = d.items || []
        // 附用户名
        try {
          const u = await get('/users-options', {}, { silent: true })
          const um = {}
          for (const x of (u.items || u || [])) um[x.id] = x.display_name || x.username
          items.forEach((s) => { s.user_name = um[s.user_id] || '' })
        } catch (e) { /* ignore */ }
        this.shifts = items
      } catch (e) { /* ignore */ }
      try {
        this.cur = await get('/duty/current', {}, { silent: true })
      } catch (e) { /* ignore */ }
    },
    goHandover() { uni.navigateTo({ url: '/pages/duty/handover' }) },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 40rpx; }
.cur-card { display: flex; align-items: center; }
.cur-ico { font-size: 44rpx; margin-right: 18rpx; }
.cur-t { font-size: 30rpx; font-weight: 700; }
.mine-tag { background: #1f4e9c; color: #fff; font-size: 22rpx; border-radius: 8rpx; padding: 4rpx 14rpx; }
.month-nav { margin: 20rpx 4rpx 14rpx; }
.nav-btn { font-size: 40rpx; color: #1f4e9c; padding: 0 24rpx; }
.month { font-size: 30rpx; font-weight: 700; }
.ho-btn { color: #1f4e9c; font-size: 26rpx; }
.cal-head { display: flex; margin-bottom: 10rpx; }
.cal-w { flex: 1; text-align: center; font-size: 24rpx; color: #8a94a6; }
.cal-grid { display: flex; flex-wrap: wrap; }
.cal-cell { width: 14.28%; box-sizing: border-box; text-align: center; padding: 10rpx 0; min-height: 88rpx; }
.cal-cell.today { background: #eef4ff; border-radius: 12rpx; }
.cal-cell.other { opacity: 0.3; }
.cal-d { font-size: 26rpx; }
.cal-s { display: flex; justify-content: center; margin-top: 4rpx; }
.s-dot { font-size: 18rpx; width: 30rpx; height: 30rpx; line-height: 30rpx; border-radius: 50%; margin: 0 2rpx; color: #fff; text-align: center; background: #8a94a6; }
.s-dot.day { background: #f5a623; }
.s-dot.night { background: #1f4e9c; }
.s-dot.full { background: #2fa84f; }
.legend { display: flex; justify-content: center; margin-top: 12rpx; }
.lg { font-size: 22rpx; color: #8a94a6; margin: 0 20rpx; display: flex; align-items: center; }
.lg .s-dot { margin-right: 8rpx; }
.sec-title { font-size: 30rpx; font-weight: 700; margin: 22rpx 4rpx 14rpx; }
.my-item { display: flex; align-items: center; font-size: 28rpx; }
.my-item .tag { margin-left: 20rpx; font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 8rpx; color: #fff; }
.my-item .tag.day { background: #f5a623; }
.my-item .tag.night { background: #1f4e9c; }
.my-item .tag.full { background: #2fa84f; }
.right { text-align: right; }
.empty-tip.sm { padding: 30rpx 0; }
.page { padding-bottom: calc(env(safe-area-inset-bottom, 0px) + 170rpx) !important; }
</style>
