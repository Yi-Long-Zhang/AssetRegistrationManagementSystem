import { request, unwrap } from './request'

export const credentialsApi = {
  list() {
    return request.get('/credentials').then(unwrap)
  },
  create(payload) {
    return request.post('/credentials', payload).then(unwrap)
  },
  update(id, payload) {
    return request.put(`/credentials/${id}`, payload).then(unwrap)
  },
  remove(id) {
    return request.delete(`/credentials/${id}`).then(unwrap)
  },
  reveal(id) {
    return request.post(`/credentials/${id}/reveal`).then(unwrap)
  }
}
