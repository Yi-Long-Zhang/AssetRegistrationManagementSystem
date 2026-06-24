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
  },
  import(payload) {
    return request.post('/assets/import', payload).then(unwrap)
  },
  export(params) {
    return request.get('/assets/export', { params, responseType: 'blob' }).then(unwrap)
  },
  stats(params) {
    return request.get('/assets/stats', { params }).then(unwrap)
  },
  template(params) {
    return request.get('/assets/template', { params, responseType: 'blob' }).then(unwrap)
  }
}
