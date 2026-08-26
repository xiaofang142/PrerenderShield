import { useState, useEffect } from 'react'
import dayjs from 'dayjs'
import { firewallApi } from '../../services/api'

export interface AccessLog {
  id?: string | number
  time: string
  ip: string
  method: string
  path: string
  status: number
  duration: number
}

export interface LogStats {
  topIPs: Array<{ ip: string; count: number }>
  topURLs: Array<{ url: string; count: number }>
  methodStats: Array<{ method: string; count: number }>
  statusStats: Array<{ status: number; count: number }>
}

const emptyStats: LogStats = {
  topIPs: [],
  topURLs: [],
  methodStats: [],
  statusStats: [],
}

/** 单次遍历统计 IP/URL/Method/Status，替代原 4 次独立 forEach */
export function calculateLogStats(logs: AccessLog[]): LogStats {
  const ipCount = new Map<string, number>()
  const urlCount = new Map<string, number>()
  const methodCount = new Map<string, number>()
  const statusCount = new Map<number, number>()

  for (const log of logs) {
    if (log.ip) ipCount.set(log.ip, (ipCount.get(log.ip) ?? 0) + 1)
    if (log.path) urlCount.set(log.path, (urlCount.get(log.path) ?? 0) + 1)
    if (log.method) methodCount.set(log.method, (methodCount.get(log.method) ?? 0) + 1)
    if (log.status) statusCount.set(log.status, (statusCount.get(log.status) ?? 0) + 1)
  }

  function toSortedList<K extends string | number>(
    map: Map<K, number>,
    keyName: K extends string ? ('ip' | 'url' | 'method') : 'status',
    limit: number
  ): Array<{ count: number } & Record<string, string | number>> {
    return Array.from(map.entries())
      .map(([key, count]) => ({ [keyName]: key, count }))
      .sort((a, b) => b.count - a.count)
      .slice(0, limit)
  }

  return {
    topIPs: toSortedList(ipCount, 'ip', 10) as LogStats['topIPs'],
    topURLs: toSortedList(urlCount, 'url', 10) as LogStats['topURLs'],
    methodStats: toSortedList(methodCount, 'method', Number.MAX_SAFE_INTEGER) as LogStats['methodStats'],
    statusStats: toSortedList(statusCount, 'status', Number.MAX_SAFE_INTEGER) as LogStats['statusStats'],
  }
}

export interface UseLogsOptions {
  pageSize: number
}

/** 日志数据 + 统计的统一数据源 Hook */
export function useLogs({ pageSize }: UseLogsOptions) {
  const [logs, setLogs] = useState<AccessLog[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [currentPage, setCurrentPage] = useState(1)
  const [stats, setStats] = useState<LogStats>(emptyStats)

  const fetchLogs = async (page = 1): Promise<void> => {
    try {
      setLoading(true)
      const res = await firewallApi.getAccessLogs({ page, limit: pageSize })
      if (res.code === 200) {
        const logData: AccessLog[] = res.data.logs || []
        setLogs(logData)
        setTotal(res.data.total || 0)
        setCurrentPage(page)
        setStats(calculateLogStats(logData))
      }
    } catch (error) {
      console.error('Failed to fetch access logs:', error)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void fetchLogs()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pageSize])

  return { logs, total, loading, currentPage, stats, fetchLogs, setLoading }
}

export interface LogFilters {
  ip: string
  method: string
  status: string
}

/** 客户端筛选（纯函数，便于测试） */
export function filterLogs<T extends AccessLog>(logs: T[], filters: LogFilters): T[] {
  return logs.filter((log) => {
    if (filters.ip && !log.ip?.includes(filters.ip)) return false
    if (filters.method && log.method !== filters.method) return false
    if (filters.status && log.status !== parseInt(filters.status)) return false
    return true
  })
}

/** 导出 CSV 行格式化 */
export function formatLogForExport(log: AccessLog): Record<string, string | number> {
  return {
    时间: dayjs(log.time).format('YYYY-MM-DD HH:mm:ss'),
    IP: log.ip,
    方法: log.method,
    路径: log.path,
    状态码: log.status,
    耗时ms: log.duration,
  }
}
