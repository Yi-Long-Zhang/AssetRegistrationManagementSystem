import { request, unwrap } from './request'

export const usersApi = {
  list() {
    return request.get('/users').then(unwrap)
  },
  create(payload) {
    return request.post('/users', payload).then(unwrap)
  },
  update(id, payload) {
    return request.put(`/users/${id}`, payload).then(unwrap)
  }
}
