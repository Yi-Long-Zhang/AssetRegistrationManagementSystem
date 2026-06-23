import axios from 'axios'
import { ElMessage } from 'element-plus'

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
  (error) => {
    if (error.response?.status === 401) {
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
