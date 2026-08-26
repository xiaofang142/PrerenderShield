/**
 * useRealtime hook 单测。
 *
 * 覆盖: 建连携带 token、消息分发、断线指数退避重连、卸载清理。
 * WebSocket 以 mock 类注入全局环境。
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useRealtime, type RealtimeMessage } from '../hooks/useRealtime'

class MockWebSocket {
  static instances: MockWebSocket[] = []
  url: string
  closed = false
  onopen?: () => void
  onclose?: () => void
  onmessage?: (ev: { data: string }) => void

  constructor(url: string) {
    this.url = url
    MockWebSocket.instances.push(this)
  }

  close() {
    this.closed = true
    this.onclose?.()
  }

  simulateOpen() {
    this.onopen?.()
  }

  simulateMessage(msg: RealtimeMessage) {
    this.onmessage?.({ data: JSON.stringify(msg) })
  }
}

describe('useRealtime', () => {
  let originalWS: typeof WebSocket

  beforeEach(() => {
    vi.useFakeTimers()
    originalWS = global.WebSocket
    // 测试环境替换全局 WebSocket
    global.WebSocket = MockWebSocket as unknown as typeof WebSocket
    MockWebSocket.instances = []
    localStorage.setItem('token', 'test-token')
  })

  afterEach(() => {
    vi.useRealTimers()
    global.WebSocket = originalWS
    localStorage.clear()
    vi.restoreAllMocks()
  })

  it('connects to /ws/realtime with token from localStorage', () => {
    const onMessage = vi.fn()
    renderHook(() => useRealtime(onMessage))

    expect(MockWebSocket.instances).toHaveLength(1)
    const ws = MockWebSocket.instances[0]
    expect(ws.url).toContain('/ws/realtime')
    expect(ws.url).toContain('token=test-token')
  })

  it('does not connect without a token', () => {
    localStorage.removeItem('token')
    renderHook(() => useRealtime(vi.fn()))
    expect(MockWebSocket.instances).toHaveLength(0)
  })

  it('dispatches parsed messages to the callback', async () => {
    const onMessage = vi.fn()
    renderHook(() => useRealtime(onMessage))
    const ws = MockWebSocket.instances[0]

    await act(async () => {
      ws.simulateOpen()
      ws.simulateMessage({ type: 'monitor', data: { totalRequests: 42 } })
    })
    ws.simulateMessage({ type: 'alert' }) // 非 JSON 安全: 合法 JSON 但无 data

    expect(onMessage).toHaveBeenCalledTimes(2)
    expect(onMessage).toHaveBeenLastCalledWith(expect.objectContaining({ type: 'alert' }))
  })

  it('ignores malformed frames without throwing', async () => {
    const onMessage = vi.fn()
    renderHook(() => useRealtime(onMessage))
    const ws = MockWebSocket.instances[0]
    await act(async () => ws.simulateOpen())

    expect(() => ws.onmessage?.({ data: 'not-json{{' })).not.toThrow()
    expect(onMessage).not.toHaveBeenCalled()
  })

  it('reconnects with exponential backoff after close', async () => {
    renderHook(() => useRealtime(vi.fn()))
    expect(MockWebSocket.instances).toHaveLength(1)

    // 第一次断开 → ~1s 后重连
    await act(async () => MockWebSocket.instances[0].onclose?.())
    await act(async () => vi.advanceTimersByTime(1100))
    expect(MockWebSocket.instances.length).toBeGreaterThanOrEqual(2)

    // 连续快速断开 → 退避时间增长（1s→2s）
    await act(async () => MockWebSocket.instances[1].onclose?.())
    await act(async () => vi.advanceTimersByTime(1500))
    expect(MockWebSocket.instances.length).toBe(2) // 尚未到 2s 退避
    await act(async () => vi.advanceTimersByTime(600))
    expect(MockWebSocket.instances.length).toBeGreaterThanOrEqual(3)
  })

  it('closes the socket and stops reconnecting on unmount', async () => {
    const { unmount } = renderHook(() => useRealtime(vi.fn()))
    const ws = MockWebSocket.instances[0]
    await act(async () => ws.simulateOpen())

    unmount()
    expect(ws.closed).toBe(true)

    await act(async () => vi.advanceTimersByTime(10000))
    // 卸载后不再新建连接
    expect(MockWebSocket.instances).toHaveLength(1)
  })

  it('skips connecting when enabled=false', () => {
    renderHook(() => useRealtime(vi.fn(), false))
    expect(MockWebSocket.instances).toHaveLength(0)
  })
})
