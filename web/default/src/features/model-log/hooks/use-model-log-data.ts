import { useCallback, useEffect, useState } from 'react'
import dayjs from '@/lib/dayjs'
import { getTokenRecordRecent } from '../api'
import type {
  TokenRecordRecentItem,
  TokenRecordOverallSummary,
} from '../types'

export function useModelLogData() {
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [items, setItems] = useState<TokenRecordRecentItem[]>([])
  const [summary, setSummary] = useState<TokenRecordOverallSummary | null>(null)
  const [lastUpdatedAt, setLastUpdatedAt] = useState(0)

  const fetchData = useCallback(async (withLoading = true) => {
    if (withLoading) {
      setLoading(true)
    } else {
      setRefreshing(true)
    }

    try {
      const res = await getTokenRecordRecent()
      if (res.success && res.data) {
        setItems(res.data.items ?? [])
        setSummary(res.data.summary ?? null)
      }
      setLastUpdatedAt(dayjs().unix())
    } catch {
      // api interceptor handles error toast
    } finally {
      if (withLoading) {
        setLoading(false)
      } else {
        setRefreshing(false)
      }
    }
  }, [])

  useEffect(() => {
    fetchData(true)
  }, [fetchData])

  return {
    loading,
    refreshing,
    items,
    summary,
    lastUpdatedAt,
    refreshData: () => fetchData(false),
  }
}
