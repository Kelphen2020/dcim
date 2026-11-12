<script>
export default {
  onLaunch() {
    // 登录守卫在各页面 onShow 的 requireAuth() 统一处理，
    // onLaunch 阶段路由未就绪，勿在此 reLaunch（H5 会 fail）
  },
}
</script>

<style>
/* 全局基础样式（Light-Tech 移动端） */
page {
  background: #f4f6fa;
  font-size: 28rpx;
  color: #1f2937;
  font-family: -apple-system, 'PingFang SC', 'Helvetica Neue', Arial, sans-serif;
}
.accent { color: #1f4e9c; }
.card {
  background: #fff;
  border-radius: 24rpx;
  box-shadow: 0 4rpx 16rpx rgba(31, 78, 156, 0.06);
  padding: 24rpx;
  margin-bottom: 20rpx;
}
.muted { color: #8a94a6; font-size: 24rpx; }
.btn-primary {
  background: #1f4e9c;
  color: #fff;
  border-radius: 16rpx;
  text-align: center;
  padding: 22rpx 0;
  font-size: 30rpx;
}
.btn-primary[disabled] { opacity: 0.5; }
.btn-ghost {
  border: 1rpx solid #c9d4e8;
  color: #1f4e9c;
  border-radius: 16rpx;
  text-align: center;
  padding: 20rpx 0;
  font-size: 28rpx;
  background: #fff;
}
.tag {
  display: inline-block;
  font-size: 22rpx;
  padding: 4rpx 14rpx;
  border-radius: 8rpx;
  margin-right: 10rpx;
}
.empty-tip { text-align: center; color: #8a94a6; padding: 80rpx 0; font-size: 26rpx; }
.row { display: flex; align-items: center; }
.grow { flex: 1; }
</style>
