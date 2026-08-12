import { request, unwrap } from './request'

export const inspectionApi = {
  listRules() {
    return request.get('/inspection/rules').then(unwrap)
  },
  createRule(payload) {
    return request.post('/inspection/rules', payload).then(unwrap)
  },
  updateRule(id, payload) {
    return request.put(`/inspection/rules/${id}`, payload).then(unwrap)
  },
  removeRule(id) {
    return request.delete(`/inspection/rules/${id}`).then(unwrap)
  },
  testRule(id) {
    return request.post(`/inspection/rules/${id}/test`).then(unwrap)
  }
}
