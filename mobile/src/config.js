// 运行环境配置
// H5：默认同源相对路径（部署在 nginx /m/ 下，/api 已被反代到后端）
// 小程序/App：必须完整域名
let base = ''
// #ifdef H5
base = typeof window !== 'undefined' && window.location && window.location.origin ? '' : ''
// #endif
// #ifndef H5
// 小程序/App 需完整域名：改为你的实际部署地址（须 ICP 备案 + HTTPS），或在「我的-服务器设置」中配置
base = 'https://dcim.example.com'
// #endif

const saved = uni.getStorageSync('dcim_base_url')

export const CONFIG = {
  // storage 里保存的服务器地址优先（"我的-服务器设置"可改）
  BASE_URL: saved || base,
  API_PREFIX: '/api/v1',
  VERSION: 'M1-v35',
}

export function setBaseUrl(url) {
  uni.setStorageSync('dcim_base_url', url)
  CONFIG.BASE_URL = url
}
