import { request, unwrap } from './request'

export const licensesApi = {
  list() {
    return request.get('/licenses').then(unwrap)
  },
  create(payload) {
    return request.post('/licenses', payload).then(unwrap)
  },
  update(id, payload) {
    return request.put(`/licenses/${id}`, payload).then(unwrap)
  },
  remove(id) {
    return request.delete(`/licenses/${id}`).then(unwrap)
  },
  reveal(id) {
    return request.post(`/licenses/${id}/reveal`).then(unwrap)
  },
  import(payload) {
    return request.post('/licenses/import', payload).then(unwrap)
  },
  template(params) {
    return request.get('/licenses/template', { params, responseType: 'blob' }).then(unwrap)
  },
  export(params) {
    return request.get('/licenses/export', { params, responseType: 'blob' }).then(unwrap)
  },
  attachments(id) {
    return request.get(`/licenses/${id}/attachments`).then(unwrap)
  },
  uploadAttachment(id, payload) {
    return request.post(`/licenses/${id}/attachments`, payload).then(unwrap)
  },
  downloadAttachment(id, attachmentId) {
    return request.get(`/licenses/${id}/attachments/${attachmentId}/download`, { responseType: 'blob' }).then(unwrap)
  },
  removeAttachment(id, attachmentId) {
    return request.delete(`/licenses/${id}/attachments/${attachmentId}`).then(unwrap)
  }
}
