import { request, unwrap } from './request'

export const authApi = {
  login(payload) {
    return request.post('/auth/login', payload).then(unwrap)
  },
  logout() {
    return request.post('/auth/logout').then(unwrap)
  },
  me() {
    return request.get('/auth/me').then(unwrap)
  },
  changePassword(oldPassword, newPassword) {
    return request.post('/auth/change-password', { oldPassword, newPassword }).then(unwrap)
  },
  sessions() {
    return request.get('/auth/sessions').then(unwrap)
  },
  revokeSession(id) {
    return request.delete(`/auth/sessions/${encodeURIComponent(id)}`).then(unwrap)
  },
  revokeAllSessions() {
    return request.post('/auth/sessions/revoke-all').then(unwrap)
  }
}
