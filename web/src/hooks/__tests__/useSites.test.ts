import { describe, it, expect, vi, afterEach } from 'vitest'
import { renderHook, waitFor, act } from '@testing-library/react'
import { useSites } from '../useSites'
import { sitesApi } from '../../services/api'

describe('useSites', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('加载站点列表并暴露数据', async () => {
    vi.spyOn(sitesApi, 'getSites').mockResolvedValue({
      code: 200,
      message: 'ok',
      data: [{ id: 'a', name: 'A' }],
    } as any)

    const { result } = renderHook(() => useSites())

    await waitFor(() => expect(result.current.sites).toHaveLength(1))
    expect(result.current.selectedSiteId).toBe('')
  })

  it('autoSelectFirst 时自动选中首个站点且不覆盖已有选择', async () => {
    vi.spyOn(sitesApi, 'getSites').mockResolvedValue({
      code: 200,
      message: 'ok',
      data: [{ id: 'first' }, { id: 'second' }],
    } as any)

    const { result } = renderHook(() => useSites({ autoSelectFirst: true }))

    await waitFor(() => expect(result.current.selectedSiteId).toBe('first'))

    // 用户手动切换后重新拉取不应覆盖用户选择
    act(() => {
      result.current.setSelectedSiteId('second')
    })
    await result.current.fetchSites()
    expect(result.current.selectedSiteId).toBe('second')
  })

  it('请求失败时调用 onFetchError', async () => {
    vi.spyOn(sitesApi, 'getSites').mockRejectedValue(new Error('network down'))
    const onError = vi.fn()

    const { result } = renderHook(() => useSites({ onFetchError: onError }))

    await waitFor(() => expect(onError).toHaveBeenCalled())
    expect(result.current.sites).toHaveLength(0)
  })
})
