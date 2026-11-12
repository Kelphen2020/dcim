import axios from 'axios'

// API 基础封装：统一带 JWT、统一解包 {code, msg, data}
const http = axios.create({ baseURL: '/api/v1', timeout: 15000 })

http.interceptors.request.use(cfg => {
  const tk = localStorage.getItem('dcim_token')
  if (tk) cfg.headers.Authorization = 'Bearer ' + tk
  return cfg
})

http.interceptors.response.use(
  resp => {
    const body = resp.data
    if (body && body.code === 401) {
      localStorage.removeItem('dcim_token')
      if (!location.hash.includes('/login')) location.hash = '#/login'
      return Promise.reject(new Error(body.msg || '未登录'))
    }
    return body
  },
  err => {
    const body = err.response && err.response.data
    if (body && body.code === 401) {
      localStorage.removeItem('dcim_token')
      if (!location.hash.includes('/login')) location.hash = '#/login'
      return Promise.reject(err)
    }
    // 4xx/5xx 归一为 {code,msg}：页面可正常显示后端错误信息（如密码强度校验），
    // 而不是静默抛异常导致点击无反应
    if (body && typeof body.code !== 'undefined') return body
    return { code: err.response ? err.response.status : -1, msg: (err.response && err.response.statusText) || err.message || '网络错误' }
  }
)

const api = {
  get: (url, params) => http.get(url, { params }),
  post: (url, data) => http.post(url, data),
  postLong: (url, data) => http.post(url, data, { timeout: 60000 }), // V53 AI 助手等大模型调用
  put: (url, data) => http.put(url, data),
  del: url => http.delete(url)
}

export default api
