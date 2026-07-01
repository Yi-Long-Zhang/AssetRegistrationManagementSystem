import { request, unwrap } from './request'

export const workflowsApi = {
  list() {
    return request.get('/workflows').then(unwrap)
  },
  detail(type) {
    return request.get(`/workflows/${type}`).then(unwrap)
  },
  save(type, payload) {
    return request.put(`/workflows/${type}`, payload).then(unwrap)
  },
  enable(type) {
    return request.post(`/workflows/${type}/enable`).then(unwrap)
  }
}
