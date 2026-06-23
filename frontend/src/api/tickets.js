import { request, unwrap } from './request'

export const ticketsApi = {
  list(params) {
    return request.get('/tickets', { params }).then(unwrap)
  },
  detail(id) {
    return request.get(`/tickets/${id}`).then(unwrap)
  },
  create(payload) {
    return request.post('/tickets', payload).then(unwrap)
  },
  action(id, action, payload) {
    return request.post(`/tickets/${id}/${action}`, payload).then(unwrap)
  },
  comments(id) {
    return request.get(`/tickets/${id}/comments`).then(unwrap)
  },
  createComment(id, payload) {
    return request.post(`/tickets/${id}/comments`, payload).then(unwrap)
  },
  attachments(id) {
    return request.get(`/tickets/${id}/attachments`).then(unwrap)
  },
  uploadAttachment(id, payload) {
    return request.post(`/tickets/${id}/attachments`, payload).then(unwrap)
  },
  downloadAttachment(id, attachmentId) {
    return request.get(`/tickets/${id}/attachments/${attachmentId}/download`, { responseType: 'blob' }).then(unwrap)
  }
}
