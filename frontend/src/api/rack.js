import { request, unwrap } from './request'

export const rackApi = {
  // 机房
  listRooms() {
    return request.get('/rooms').then(unwrap)
  },
  createRoom(payload) {
    return request.post('/rooms', payload).then(unwrap)
  },
  updateRoom(id, payload) {
    return request.put(`/rooms/${id}`, payload).then(unwrap)
  },
  removeRoom(id) {
    return request.delete(`/rooms/${id}`).then(unwrap)
  },
  // 机柜
  listRacks(params) {
    return request.get('/racks', { params }).then(unwrap)
  },
  createRack(payload) {
    return request.post('/racks', payload).then(unwrap)
  },
  updateRack(id, payload) {
    return request.put(`/racks/${id}`, payload).then(unwrap)
  },
  removeRack(id) {
    return request.delete(`/racks/${id}`).then(unwrap)
  }
}
