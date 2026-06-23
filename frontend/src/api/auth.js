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
  }
}
