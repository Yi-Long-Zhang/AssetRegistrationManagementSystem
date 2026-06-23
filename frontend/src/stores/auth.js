import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { authApi } from '../api'
import { hasAnyRole } from '../utils/permissions'

function readUser() {
  try {
    return JSON.parse(localStorage.getItem('user') || 'null')
  } catch {
    return null
  }
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token') || '')
  const user = ref(readUser())
  const loaded = ref(false)

  const isLoggedIn = computed(() => Boolean(token.value))
  const role = computed(() => user.value?.role || '')

  function setSession(payload) {
    token.value = payload.token
    user.value = payload.user
    loaded.value = true
    localStorage.setItem('token', payload.token)
    localStorage.setItem('user', JSON.stringify(payload.user))
  }

  function clearSession() {
    token.value = ''
    user.value = null
    loaded.value = false
    localStorage.removeItem('token')
    localStorage.removeItem('user')
  }

  async function login(payload) {
    const data = await authApi.login(payload)
    setSession(data)
    return data
  }

  async function fetchMe() {
    if (!token.value) return null
    const data = await authApi.me()
    user.value = data.user || data
    loaded.value = true
    localStorage.setItem('user', JSON.stringify(user.value))
    return user.value
  }

  async function logout() {
    try {
      if (token.value) await authApi.logout()
    } finally {
      clearSession()
    }
  }

  function canAccess(roles) {
    return hasAnyRole(user.value, roles)
  }

  return { token, user, loaded, isLoggedIn, role, login, fetchMe, logout, clearSession, canAccess }
})
