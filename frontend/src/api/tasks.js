import { request, unwrap } from './request'

export const tasksApi = {
  list(params) {
    return request.get('/tasks', { params }).then(unwrap)
  },
  get(id) {
    return request.get(`/tasks/${id}`).then(unwrap)
  },
  retry(id) {
    return request.post(`/tasks/${id}/retry`).then(unwrap)
  },
  acknowledge(id) {
    return request.post(`/tasks/${id}/acknowledge`).then(unwrap)
  }
}
