import { useCallback, useEffect, useRef, useState } from 'react'
import { sitesApi, extractErrorMessage } from '../services/api'

export interface SiteInfo {
  id: string
  name?: string
  Name?: string
  domains?: string[]
  [key: string]: any
}

interface UseSitesOptions {
  /** 加载完成后自动选中第一个站点 */
  autoSelectFirst?: boolean
  /** 站点过滤（如 Preheat 只保留 static 模式站点） */
  filter?: (site: SiteInfo) => boolean
  /** 加载失败时的自定义提示（如 message.error / i18n 提示）；不传则仅 console */
  onFetchError?: (message: string) => void
  /** 是否立即拉取，默认 true */
  immediate?: boolean
}

/**
 * 站点列表加载 Hook：收敛各页面重复的 getSites 逻辑
 * （loading 状态 + code===200 判定 + 自动选中首站点 + 统一错误提取）
 */
export function useSites(options: UseSitesOptions = {}) {
  const { autoSelectFirst = false, filter, onFetchError, immediate = true } = options

  const [sites, setSites] = useState<SiteInfo[]>([])
  const [selectedSiteId, setSelectedSiteId] = useState<string>('')
  const [loading, setLoading] = useState(false)
  const mountedRef = useRef(true)

  useEffect(() => {
    mountedRef.current = true
    return () => { mountedRef.current = false }
  }, [])

  const fetchSites = useCallback(async () => {
    try {
      setLoading(true)
      const res = await sitesApi.getSites()
      if (!mountedRef.current) return
      if (res.code === 200 && Array.isArray(res.data)) {
        const list = filter ? res.data.filter(filter) : res.data
        setSites(list)
        if (autoSelectFirst && list.length > 0) {
          setSelectedSiteId((prev) => prev || list[0].id)
        }
      }
    } catch (error) {
      console.error('Failed to fetch sites:', error)
      onFetchError?.(extractErrorMessage(error))
    } finally {
      if (mountedRef.current) {
        setLoading(false)
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [autoSelectFirst, filter])

  useEffect(() => {
    if (immediate) {
      fetchSites()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return {
    sites,
    setSites,
    selectedSiteId,
    setSelectedSiteId,
    loading,
    fetchSites,
  }
}
