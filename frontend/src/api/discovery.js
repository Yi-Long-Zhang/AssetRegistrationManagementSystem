import { request, unwrap } from './request'

export const discoveryApi = {
  // 规则
  listRules() {
    return request.get('/discovery/rules').then(unwrap)
  },
  createRule(payload) {
    return request.post('/discovery/rules', payload).then(unwrap)
  },
  updateRule(id, payload) {
    return request.put(`/discovery/rules/${id}`, payload).then(unwrap)
  },
  removeRule(id) {
    return request.delete(`/discovery/rules/${id}`).then(unwrap)
  },
  runRule(id) {
    return request.post(`/discovery/rules/${id}/run`).then(unwrap)
  },
  testRule(id) {
    return request.post(`/discovery/rules/${id}/test`).then(unwrap)
  },
  // 运行记录
  listRuns(params) {
    return request.get('/discovery/runs', { params }).then(unwrap)
  },
  getRun(id) {
    return request.get(`/discovery/runs/${id}`).then(unwrap)
  },
  adoptHosts(runId, hostIds) {
    return request.post(`/discovery/runs/${runId}/adopt`, { hostIds }).then(unwrap)
  },
  applyHosts(runId, hostIds) {
    return request.post(`/discovery/runs/${runId}/apply`, { hostIds }).then(unwrap)
  },
  // 资产变更历史
  assetHistory(id) {
    return request.get(`/assets/${id}/history`).then(unwrap)
  }
}
