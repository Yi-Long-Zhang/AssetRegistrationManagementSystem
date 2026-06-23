import { request, unwrap } from './request'

export const adApi = {
  config() {
    return request.get('/ad/config').then(unwrap)
  },
  saveConfig(payload) {
    return request.put('/ad/config', payload).then(unwrap)
  },
  test() {
    return request.post('/ad/test').then(unwrap)
  },
  lookupUser(payload) {
    return request.post('/ad/lookup-user', payload).then(unwrap)
  },
  importUser(payload) {
    return request.post('/ad/import-user', payload).then(unwrap)
  }
}
