import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'

export const request = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1'
})

request.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

request.interceptors.response.use(
  (response) => response,
  async (error) => {
    const response = error.response
    const original = error.config
    if (
      response?.status === 428 &&
      response.data?.code === 'reauth_required' &&
      !original?._reauthRetried &&
      !original?._skipReauth
    ) {
      original._reauthRetried = true
      try {
        const { value } = await ElMessageBox.prompt('请输入当前密码以继续敏感操作', '二次认证', {
          inputType: 'password',
          inputPlaceholder: '当前密码',
          confirmButtonText: '验证',
          cancelButtonText: '取消',
          inputValidator: (password) => Boolean(password) || '请输入密码'
        })
        await request.post('/auth/reauth', { password: value }, { _skipReauth: true })
        return request(original)
      } catch {
        return Promise.reject(error)
      }
    }
    if (response?.status === 401) {
      localStorage.removeItem('token')
      localStorage.removeItem('user')
      if (location.pathname !== '/login') {
        ElMessage.error('登录已失效，请重新登录')
        location.href = '/login'
      }
    }
    return Promise.reject(error)
  }
)

export function unwrap(response) {
  return response.data
}
