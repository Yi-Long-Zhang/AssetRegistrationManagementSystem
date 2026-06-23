import { request, unwrap } from './request'

export const settingsApi = {
  ticketTypeApprovers() {
    return request.get('/ticket-type-approvers').then(unwrap)
  },
  saveTicketTypeApprover(type, payload) {
    return request.put(`/ticket-type-approvers/${type}`, payload).then(unwrap)
  }
}
