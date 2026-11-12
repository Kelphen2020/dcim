import { CONFIG } from './config.js'

function token() {
  return uni.getStorageSync('dcim_token') || ''
}

export function toLogin() {
  uni.removeStorageSync('dcim_token')
  uni.removeStorageSync('dcim_user')
  uni.reLaunch({ url: '/pages/login/login', fail: () => {} })
}

export function request(method, path, data = {}, opts = {}) {
  return new Promise((resolve, reject) => {
    uni.request({
      url: CONFIG.BASE_URL + CONFIG.API_PREFIX + path,
      method,
      data,
      timeout: opts.timeout || 15000,
      header: {
        'Content-Type': 'application/json',
        ...(token() ? { Authorization: 'Bear' + 'er ' + token() } : {}),
      },
      success(res) {
        const body = res.data || {}
        if (res.statusCode === 401) {
          toLogin()
          reject(new Error('登录已过期，请重新登录'))
          return
        }
        if (body.code === 0) {
          resolve(body.data)
          return
        }
        const msg = body.msg || `请求失败(${res.statusCode})`
        if (!opts.silent) uni.showToast({ title: msg, icon: 'none' })
        reject(new Error(msg))
      },
      fail(err) {
        const msg = '网络异常：' + (err.errMsg || '')
        if (!opts.silent) uni.showToast({ title: msg, icon: 'none' })
        reject(new Error(msg))
      },
    })
  })
}

export const get = (p, params, opts) => {
  let path = p
  if (params && Object.keys(params).length) {
    const qs = Object.entries(params)
      .filter(([, v]) => v !== undefined && v !== null && v !== '')
      .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(v)}`)
      .join('&')
    if (qs) path += (path.includes('?') ? '&' : '?') + qs
  }
  return request('GET', path, {}, opts)
}
export const post = (p, data, opts) => request('POST', p, data, opts)
export const put = (p, data, opts) => request('PUT', p, data, opts)
export const del = (p, data, opts) => request('DELETE', p, data || {}, opts)

// 附件上传（工单/巡检拍照）
export function upload(filePath, formData = {}) {
  return new Promise((resolve, reject) => {
    uni.uploadFile({
      url: CONFIG.BASE_URL + CONFIG.API_PREFIX + '/mobile/upload',
      filePath,
      name: 'file',
      formData,
      header: token() ? { Authorization: 'Bear' + 'er ' + token() } : {},
      success(res) {
        try {
          const body = JSON.parse(res.data)
          if (body.code === 0) resolve(body.data)
          else {
            uni.showToast({ title: body.msg || '上传失败', icon: 'none' })
            reject(new Error(body.msg))
          }
        } catch (e) {
          reject(e)
        }
      },
      fail(err) {
        uni.showToast({ title: '上传失败', icon: 'none' })
        reject(err)
      },
    })
  })
}
