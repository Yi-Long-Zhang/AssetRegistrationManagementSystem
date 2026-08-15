import { request, unwrap } from './request'

export const ipSegmentsApi = {
  list() {
    return request.get('/ip-segments').then(unwrap)
  },
  create(payload) {
    return request.post('/ip-segments', payload).then(unwrap)
  },
  update(id, payload) {
    return request.put(`/ip-segments/${id}`, payload).then(unwrap)
  },
  remove(id) {
    return request.delete(`/ip-segments/${id}`).then(unwrap)
  },
  usage(id) {
    return request.get(`/ip-segments/${id}/usage`).then(unwrap)
  }
}
