import { useEffect, useRef } from 'react'

export interface RealtimeMessage {
  type: string
  channel?: string
  data?: unknown
  timestamp?: number
}

const MAX_RETRY_DELAY = 30000

/**
 * useRealtime 订阅后端 /ws/realtime WebSocket 实时推送。
 * 自动携带 JWT（查询参数，浏览器 WS 无法设置请求头）、断线指数退避重连、卸载清理。
 * onMessage 通过 ref 持有，无需调用方稳定回调引用。
 */
export function useRealtime(onMessage: (msg: RealtimeMessage) => void, enabled = true) {
  const handlerRef = useRef(onMessage)
  handlerRef.current = onMessage

  useEffect(() => {
    if (!enabled) return

    let ws: WebSocket | null = null
    let disposed = false
    let retries = 0
    let timer: ReturnType<typeof setTimeout> | undefined

    const connect = () => {
      const token = localStorage.getItem('token')
      if (!token || disposed) return

      const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
      ws = new WebSocket(
        `${proto}://${window.location.host}/ws/realtime?token=${encodeURIComponent(token)}`
      )

      ws.onopen = () => {
        retries = 0
      }

      ws.onmessage = (ev: MessageEvent) => {
        try {
          handlerRef.current(JSON.parse(ev.data as string) as RealtimeMessage)
        } catch {
          // 忽略非 JSON 帧
        }
      }

      ws.onclose = () => {
        if (disposed) return
        const delay = Math.min(1000 * 2 ** retries, MAX_RETRY_DELAY)
        retries += 1
        timer = setTimeout(connect, delay)
      }
    }

    connect()

    return () => {
      disposed = true
      if (timer) clearTimeout(timer)
      ws?.close()
    }
  }, [enabled])
}
