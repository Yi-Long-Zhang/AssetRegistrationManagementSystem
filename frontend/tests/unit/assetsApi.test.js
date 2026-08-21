import { beforeEach, describe, expect, it, vi } from 'vitest'
import { assetsApi } from '../../src/api/assets'
import { request } from '../../src/api/request'

describe('assetsApi', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('unwraps the asset statistics export blob', async () => {
    const blob = new Blob(['assetNo,hostname'])
    const get = vi.spyOn(request, 'get').mockResolvedValue({ data: blob })

    await expect(assetsApi.exportStatsReport()).resolves.toBe(blob)
    expect(get).toHaveBeenCalledWith('/assets/stats/export', { responseType: 'blob' })
  })
})
