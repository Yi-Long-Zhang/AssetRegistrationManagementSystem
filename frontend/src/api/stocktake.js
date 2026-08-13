import { request, unwrap } from './request'

export const stocktakeApi = {
  list() {
    return request.get('/stocktakes').then(unwrap)
  },
  create(payload) {
    return request.post('/stocktakes', payload).then(unwrap)
  },
  detail(id) {
    return request.get(`/stocktakes/${id}`).then(unwrap)
  },
  checkItem(taskId, itemId, payload) {
    return request.put(`/stocktakes/${taskId}/items/${itemId}`, payload).then(unwrap)
  },
  close(id) {
    return request.post(`/stocktakes/${id}/close`).then(unwrap)
  },
  exportCsv(id) {
    return request.get(`/stocktakes/${id}/export`, { responseType: 'blob' })
  }
}
