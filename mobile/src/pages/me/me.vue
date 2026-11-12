<template>
  <view class="page" :style="{ paddingTop: navH + 'px', paddingBottom: 'calc(env(safe-area-inset-bottom, 0px) + 150rpx)' }">
    <GlassNavBar title="我的" :scrollY="scrollY" />
    <view class="card user-card">
      <view class="avatar">{{ avatarText }}</view>
      <view class="grow">
        <view class="uname">{{ u.display_name || u.username }}</view>
        <view class="muted">@{{ u.username }} · {{ roleText(u.role) }}</view>
      </view>
    </view>

    <view class="card duty-card" v-if="duty.on_duty">
      <view class="duty-ico"><TechIcon name="clock" color="#2D6BC4" :size="44" /></view>
      <view class="grow">
        <view>当班：{{ duty.user_name }}（{{ shiftText(duty.shift) }}）</view>
        <view class="muted" v-if="duty.mine">今天轮到你值班，注意告警响应</view>
      </view>
    </view>

    <view class="card menu">
      <view class="m-item" @tap="go('/pages/summary/summary')">
        <view class="m-ico"><TechIcon name="chart" color="#2D6BC4" :size="36" /></view><text class="grow">管理摘要</text><text class="arrow">›</text>
      </view>
      <view class="m-item" v-if="isAdmin" @tap="go('/pages/change/changes')">
        <view class="m-ico"><TechIcon name="pen" color="#2D6BC4" :size="36" /></view><text class="grow">变更审批</text><text class="arrow">›</text>
      </view>
      <view class="m-item" @tap="go('/pages/asset/assets')">
        <view class="m-ico"><TechIcon name="box" color="#2D6BC4" :size="36" /></view><text class="grow">资产速查</text><text class="arrow">›</text>
      </view>
      <view class="m-item" @tap="go('/pages/stocktake/list')">
        <view class="m-ico"><TechIcon name="counter" color="#2D6BC4" :size="36" /></view><text class="grow">资产盘点</text><text class="arrow">›</text>
      </view>
      <view class="m-item" @tap="go('/pages/monitor/monitor')">
        <view class="m-ico"><TechIcon name="chart" color="#2D6BC4" :size="36" /></view><text class="grow">监控速览</text><text class="arrow">›</text>
      </view>
      <view class="m-item" @tap="go('/pages/duty/roster')">
        <view class="m-ico"><TechIcon name="cal" color="#2D6BC4" :size="36" /></view><text class="grow">值班日历</text><text class="arrow">›</text>
      </view>
      <view class="m-item" @tap="go('/pages/duty/handover')">
        <view class="m-ico"><TechIcon name="pen" color="#2D6BC4" :size="36" /></view><text class="grow">值班交接</text><text class="arrow">›</text>
      </view>
      <view class="m-item" @tap="go('/pages/maint/maint')">
        <view class="m-ico"><TechIcon name="shield" color="#2D6BC4" :size="36" /></view><text class="grow">维保与备件</text><text class="arrow">›</text>
      </view>
      <view class="m-item" @tap="go('/pages/report/report')">
        <view class="m-ico"><TechIcon name="doc" color="#2D6BC4" :size="36" /></view><text class="grow">巡检报告</text><text class="arrow">›</text>
      </view>
      <view class="m-item" @tap="go('/pages/me/push')">
        <view class="m-ico"><TechIcon name="bell" color="#2D6BC4" :size="36" /></view><text class="grow">推送设置</text><text class="arrow">›</text>
      </view>
      <view class="m-item" @tap="go('/pages/notify/notify')">
        <view class="m-ico"><TechIcon name="inbox" color="#2D6BC4" :size="36" /></view><text class="grow">消息中心</text><text class="arrow">›</text>
      </view>
      <view class="m-item" @tap="go('/pages/me/server')">
        <view class="m-ico"><TechIcon name="globe" color="#2D6BC4" :size="36" /></view><text class="grow">服务器设置</text><text class="arrow">›</text>
      </view>
      <view class="m-item" @tap="about">
        <view class="m-ico"><TechIcon name="doc" color="#2D6BC4" :size="36" /></view><text class="grow">关于</text><text class="arrow">›</text>
      </view>
    </view>

    <button class="btn-ghost logout" @tap="logout">退出登录</button>
    <view class="ver">DCIM 移动运维 {{ version }}</view>
      <GlassTabBar current="me" :scrollY="scrollY" />
  </view>
</template>

<script>
import TechIcon from '../../components/TechIcon.vue'
import { get } from '../../api.js'
import { requireAuth, user, ROLE_TEXT, SHIFT_TEXT } from '../../util.js'
import GlassNavBar from '../../components/GlassNavBar.vue'
import GlassTabBar from '../../components/GlassTabBar.vue'
import { CONFIG } from '../../config.js'

export default {
  components: { GlassNavBar, GlassTabBar, TechIcon },
  data() {
    return { u: user(), duty: { on_duty: false }, version: CONFIG.VERSION }
  },
  computed: {
    avatarText() {
      const n = this.u.display_name || this.u.username || '?'
      return n.slice(0, 1).toUpperCase()
    },
    isAdmin() {
      return ['global_admin', 'site_admin'].includes(this.u.role)
    },
  },
  onLoad() {
    try { this.navH = (uni.getSystemInfoSync().statusBarHeight || 0) + 44 } catch (e) {}
  },
  onPageScroll(e) { this.scrollY = e.scrollTop },
  onShow() {
    if (!requireAuth()) return
    this.u = user()
    this.loadDuty()
  },
  methods: {
    roleText: (r) => ROLE_TEXT[r] || r,
    shiftText: (s) => SHIFT_TEXT[s] || s,
    async loadDuty() {
      try {
        this.duty = await get('/duty/current', {}, { silent: true })
      } catch (e) { this.duty = { on_duty: false } }
    },
    go(url) { uni.navigateTo({ url }) },
    about() {
      uni.showModal({
        title: 'DCIM 移动运维',
        content: `版本 ${CONFIG.VERSION}\n数据中心基础设施管理系统移动端\n北京东青互联科技有限公司`,
        showCancel: false,
      })
    },
    logout() {
      uni.showModal({
        title: '退出登录', content: '确定退出当前账号？',
        success: (r) => {
          if (r.confirm) {
            uni.removeStorageSync('dcim_token')
            uni.removeStorageSync('dcim_user')
            uni.reLaunch({ url: '/pages/login/login', fail: () => {} })
          }
        },
      })
    },
  },
}
</script>

<style scoped>
.page { padding: 20rpx 24rpx 40rpx; }
.user-card { display: flex; align-items: center; padding: 36rpx 24rpx; }
.avatar {
  width: 110rpx; height: 110rpx; border-radius: 50%;
  background: linear-gradient(135deg, #1f4e9c, #2d6bc4);
  color: #fff; font-size: 48rpx; font-weight: 700;
  display: flex; align-items: center; justify-content: center;
  margin-right: 24rpx;
}
.uname { font-size: 34rpx; font-weight: 700; }
.duty-card { display: flex; align-items: center; }
.duty-ico { font-size: 40rpx; margin-right: 18rpx; }
.menu { padding: 8rpx 24rpx; }
.m-item { display: flex; align-items: center; padding: 30rpx 0; border-bottom: 1rpx solid #f0f3f8; font-size: 29rpx; }
.m-item:last-child { border-bottom: none; }
.m-ico { margin-right: 20rpx; font-size: 32rpx; }
.arrow { color: #c9d4e8; font-size: 34rpx; }
.logout { margin-top: 30rpx; color: #e5484d; border-color: #f0c4c6; }
.ver { text-align: center; color: #b6bfce; font-size: 22rpx; margin-top: 30rpx; }
</style>
