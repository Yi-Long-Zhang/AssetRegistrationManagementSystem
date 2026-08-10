import { request, unwrap } from './request'

export const settingsApi = {
  mailConfig() {
    return request.get('/settings/mail').then(unwrap)
  },
  saveMailConfig(payload) {
    return request.put('/settings/mail', payload).then(unwrap)
  },
  testMailConfig(payload) {
    return request.post('/settings/mail/test', payload).then(unwrap)
  },
  imConfig() {
    return request.get('/settings/im').then(unwrap)
  },
  saveIMConfig(payload) {
    return request.put('/settings/im', payload).then(unwrap)
  },
  testIMConfig() {
    return request.post('/settings/im/test').then(unwrap)
  },
  imBindings() {
    return request.get('/settings/im/bindings').then(unwrap)
  },
  saveIMBinding(payload) {
    return request.put('/settings/im/bindings', payload).then(unwrap)
  },
  deleteIMBinding(userId) {
    return request.delete(`/settings/im/bindings/${userId}`).then(unwrap)
  },
  ticketTypeApprovers() {
    return request.get('/ticket-type-approvers').then(unwrap)
  },
  saveTicketTypeApprover(type, payload) {
    return request.put(`/ticket-type-approvers/${type}`, payload).then(unwrap)
  }
}
