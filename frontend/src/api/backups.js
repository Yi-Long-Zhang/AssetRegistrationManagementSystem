import { request, unwrap } from './request'

export const backupsApi = {
  list() {
    return request.get('/backups').then(unwrap)
  },
  create() {
    return request.post('/backups').then(unwrap)
  },
  remove(name) {
    return request.delete(`/backups/${encodeURIComponent(name)}`).then(unwrap)
  },
  download(name) {
    return request.get(`/backups/${encodeURIComponent(name)}/download`, { responseType: 'blob' }).then(unwrap)
  },
  restore(name) {
    return request.post(`/backups/${encodeURIComponent(name)}/restore`).then(unwrap)
  }
}
