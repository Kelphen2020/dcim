// 公共工具
export function timeAgo(iso) {
  if (!iso) return ''
  const t = new Date(iso.replace(/-/g, '/').replace(/\.\d+/, '').replace('T', ' ').replace(/\+08:00$/, ''))
  if (isNaN(t.getTime())) return iso
  const diff = (Date.now() - t.getTime()) / 1000
  if (diff < 60) return '刚刚'
  if (diff < 3600) return Math.floor(diff / 60) + '分钟前'
  if (diff < 86400) return Math.floor(diff / 3600) + '小时前'
  if (diff < 172800) return '昨天 ' + fmtHM(t)
  return `${t.getMonth() + 1}-${String(t.getDate()).padStart(2, '0')} ${fmtHM(t)}`
}
function fmtHM(t) {
  return `${String(t.getHours()).padStart(2, '0')}:${String(t.getMinutes()).padStart(2, '0')}`
}

// SLA 剩余时间：due 为 null 返回 ''；超时返回「超时Xh」
export function slaLeft(due) {
  if (!due) return ''
  const t = new Date(due.replace(/-/g, '/').replace(/\.\d+/, '').replace('T', ' ').replace(/\+08:00$/, ''))
  if (isNaN(t.getTime())) return ''
  const ms = t.getTime() - Date.now()
  const abs = Math.abs(ms)
  const h = Math.floor(abs / 3600000)
  const m = Math.floor((abs % 3600000) / 60000)
  const txt = h >= 24 ? `${Math.floor(h / 24)}d${h % 24}h` : h > 0 ? `${h}h${m}m` : `${m}m`
  return ms >= 0 ? `剩 ${txt}` : `超时 ${txt}`
}

export const SEV_TEXT = { 1: '提示', 2: '警告', 3: '严重' }
export const WO_STATUS = { 1: '待受理', 2: '处理中', 3: '已完成', 4: '已关闭', 5: '已取消' }
export const WO_PRIORITY = { 1: '紧急', 2: '高', 3: '中', 4: '低' }
export const ROLE_TEXT = {
  global_admin: '全局管理员',
  site_admin: '站点管理员',
  operator: '运维工程师',
  viewer: '只读用户',
}
export const SHIFT_TEXT = { day: '白班', night: '夜班', full: '全天' }

export function user() {
  return uni.getStorageSync('dcim_user') || {}
}

export function requireAuth() {
  if (!uni.getStorageSync('dcim_token')) {
    uni.reLaunch({ url: '/pages/login/login', fail: () => {} })
    return false
  }
  return true
}

// 扫码分流：识别 DCIM-SCAN 令牌 → 巡检执行页
export function handleScanResult(text) {
  const content = (text || '').trim()
  if (!content) return false
  if (content.startsWith('DCIM-SCAN|')) {
    uni.navigateTo({ url: '/pages/scan/exec?token=' + encodeURIComponent(content) })
    return true
  }
  uni.showToast({ title: '不是有效的 DCIM 巡检码', icon: 'none' })
  return false
}

export function doScan() {
  // #ifdef MP-WEIXIN || APP-PLUS
  uni.scanCode({
    onlyFromCamera: false,
    success: (res) => handleScanResult(res.result),
    fail: () => {},
  })
  // #endif
  // #ifdef H5
  // H5 无摄像头扫码 API：弹输入框粘贴令牌（现场用微信扫一扫也可直开小程序）
  uni.showModal({
    title: '输入巡检码',
    editable: true,
    placeholderText: '粘贴 DCIM-SCAN|... 令牌',
    success: (r) => {
      if (r.confirm) handleScanResult(r.content)
    },
  })
  // #endif
}
