import { request, unwrap } from './request'

export const auditApi = {
  list(params) {
    return request.get('/audit-logs', { params }).then(unwrap)
  }
}
