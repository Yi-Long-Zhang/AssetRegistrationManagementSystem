import { beforeEach, describe, expect, it, vi } from 'vitest'
import { assetsApi } from '../../src/api/assets'
import { request } from '../../src/api/request'

vi.mock('../../src/api/request', () => ({
  request: {
    get: vi.fn()
  },
  unwrap: (response) => response.data
}))

describe('assetsApi exports', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('returns the report blob instead of the Axios response', async () => {
    const report = new Blob(['asset report'], { type: 'text/csv' })
    request.get.mockResolvedValue({ data: report })

    await expect(assetsApi.exportStatsReport()).resolves.toBe(report)
    expect(request.get).toHaveBeenCalledWith('/assets/stats/export', { responseType: 'blob' })
  })
})
