import { defineStore } from 'pinia'
import api from '../api'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('dcim_token') || '',
    user: JSON.parse(localStorage.getItem('dcim_user') || 'null'),
    site_ids: JSON.parse(localStorage.getItem('dcim_site_ids') || '[]'),
    lastActivity: Date.now()
  }),
  getters: {
    loggedIn: s => !!s.token,
    role: s => s.user ? s.user.role : '',
    isGlobalAdmin: s => s.user && s.user.role === 'global_admin',
    isAdmin: s => s.user && ['global_admin', 'site_admin'].includes(s.user.role)
  },
  actions: {
    async login(username, password, captcha, captchaId) {
      const res = await api.post('/login', { username, password, captcha, captcha_id: captchaId })
      if (res.code !== 0) throw new Error(res.msg || '登录失败')
      this.token = res.data.token
      this.user = res.data.user
      this.site_ids = res.data.site_ids || []
      localStorage.setItem('dcim_token', this.token)
      localStorage.setItem('dcim_user', JSON.stringify(this.user))
      localStorage.setItem('dcim_site_ids', JSON.stringify(this.site_ids))
      return res.data
    },
    logout() {
      this.token = ''
      this.user = null
      this.site_ids = []
      localStorage.removeItem('dcim_token')
      localStorage.removeItem('dcim_user')
      localStorage.removeItem('dcim_site_ids')
    }
  }
})
