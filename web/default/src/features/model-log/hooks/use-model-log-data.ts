import { useCallback, useEffect, useRef, useState } from 'react'
import dayjs from '@/lib/dayjs'
import { getTokenRecordRecent } from '../api'
import type {
  TokenRecordRecentItem,
  TokenRecordOverallSummary,
} from '../types'

const AUTO_REFRESH_INTERVAL_SECONDS = 30
const MAX_CONSECUTIVE_FAILURES = 3

export function useModelLogData() {
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [items, setItems] = useState<TokenRecordRecentItem[]>([])
  const [summary, setSummary] = useState<TokenRecordOverallSummary | null>(null)
  const [lastUpdatedAt, setLastUpdatedAt] = useState(0)
  const [timeRange, setTimeRange] = useState(24)
  const [autoRefresh, setAutoRefreshState] = useState(false)
  const [countdown, setCountdown] = useState(AUTO_REFRESH_INTERVAL_SECONDS)
  const [pageVisible, setPageVisible] = useState(true)
  const consecutiveFailuresRef = useRef(0)

  const setAutoRefresh = useCallback((enabled: boolean) => {
    setAutoRefreshState(enabled)
    if (!enabled) {
      setCountdown(AUTO_REFRESH_INTERVAL_SECONDS)
      consecutiveFailuresRef.current = 0
    }
  }, [])

  const fetchData = useCallback(
    async (withLoading = true) => {
      if (withLoading) {
        setLoading(true)
      } else {
        setRefreshing(true)
      }

      try {
        const res = await getTokenRecordRecent(timeRange)
        if (res.success && res.data) {
          setItems(res.data.items ?? [])
          setSummary(res.data.summary ?? null)
        }
        setLastUpdatedAt(dayjs().unix())
        consecutiveFailuresRef.current = 0
      } catch {
        // api interceptor handles error toast
        consecutiveFailuresRef.current += 1
        if (consecutiveFailuresRef.current >= MAX_CONSECUTIVE_FAILURES) {
          setAutoRefreshState(false)
          setCountdown(AUTO_REFRESH_INTERVAL_SECONDS)
        }
      } finally {
        if (withLoading) {
          setLoading(false)
        } else {
          setRefreshing(false)
        }
      }
    },
    [timeRange]
  )

  // Initial load and reload when timeRange changes
  useEffect(() => {
    fetchData(true)
  }, [fetchData])

  // Page Visibility API
  useEffect(() => {
    const handler = () => {
      setPageVisible(document.visibilityState === 'visible')
    }
    document.addEventListener('visibilitychange', handler)
    return () => document.removeEventListener('visibilitychange', handler)
  }, [])

  // Auto-refresh interval and countdown
  useEffect(() => {
    if (!autoRefresh || !pageVisible) return

    setCountdown(AUTO_REFRESH_INTERVAL_SECONDS)

    const refreshIntervalId = setInterval(() => {
      fetchData(false)
      setCountdown(AUTO_REFRESH_INTERVAL_SECONDS)
    }, AUTO_REFRESH_INTERVAL_SECONDS * 1000)

    const countdownIntervalId = setInterval(() => {
      setCountdown((prev) => {
        if (prev <= 1) {
          return AUTO_REFRESH_INTERVAL_SECONDS
        }
        return prev - 1
      })
    }, 1000)

    return () => {
      clearInterval(refreshIntervalId)
      clearInterval(countdownIntervalId)
    }
  }, [autoRefresh, pageVisible, fetchData])

  return {
    loading,
    refreshing,
    items,
    summary,
    lastUpdatedAt,
    refreshData: () => fetchData(false),
    timeRange,
    setTimeRange,
    autoRefresh,
    setAutoRefresh,
    countdown,
  }
}
