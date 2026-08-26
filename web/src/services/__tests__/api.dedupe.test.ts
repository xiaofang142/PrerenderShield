import { describe, it, expect, afterEach } from 'vitest'
import type { InternalAxiosRequestConfig, AxiosResponse } from 'axios'
import api from '../api'

// 使用自定义 adapter 统计真实网络请求数，验证 GET 去重逻辑
let networkCalls = 0

const delayedAdapter = (config: InternalAxiosRequestConfig): Promise<AxiosResponse> => {
  networkCalls++
  return new Promise((resolve) => {
    setTimeout(() => {
      resolve({
        data: { code: 200, message: 'ok', data: { seq: networkCalls } },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      })
    }, 30)
  })
}

describe('GET request deduplication', () => {
  afterEach(() => {
    networkCalls = 0
  })

  it('并发相同 GET 只发出一次网络请求', async () => {
    api.defaults.adapter = delayedAdapter

    const [resA, resB] = await Promise.all([
      api.get<{ seq: number }>('/__dedupe_test__'),
      api.get<{ seq: number }>('/__dedupe_test__'),
    ])

    expect(networkCalls).toBe(1)
    expect(resA.code).toBe(200)
    expect(resB.data.seq).toBe(1)
  })

  it('不同 params 的 GET 不合并', async () => {
    api.defaults.adapter = delayedAdapter

    await Promise.all([
      api.get('/__dedupe_test2__', { params: { a: 1 } }),
      api.get('/__dedupe_test2__', { params: { a: 2 } }),
    ])

    expect(networkCalls).toBe(2)
  })

  it('前序请求完成后，后续相同 GET 正常发出（不误伤时序请求）', async () => {
    api.defaults.adapter = delayedAdapter

    await api.get('/__dedupe_seq__')
    await api.get('/__dedupe_seq__')

    expect(networkCalls).toBe(2)
  })
})
