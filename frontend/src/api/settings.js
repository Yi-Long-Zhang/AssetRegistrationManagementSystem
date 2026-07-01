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
  ticketTypeApprovers() {
    return request.get('/ticket-type-approvers').then(unwrap)
  },
  saveTicketTypeApprover(type, payload) {
    return request.put(`/ticket-type-approvers/${type}`, payload).then(unwrap)
  }
}
