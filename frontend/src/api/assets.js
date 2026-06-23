import { request, unwrap } from './request'

export const assetsApi = {
  list(params) {
    return request.get('/assets', { params }).then(unwrap)
  },
  create(payload) {
    return request.post('/assets', payload).then(unwrap)
  },
  update(id, payload) {
    return request.put(`/assets/${id}`, payload).then(unwrap)
  },
  remove(id) {
    return request.delete(`/assets/${id}`).then(unwrap)
  }
}
