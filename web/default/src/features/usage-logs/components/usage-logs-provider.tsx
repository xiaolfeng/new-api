/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import type { ChannelAffinityInfo } from '../types'

const AUTO_REFRESH_INTERVAL_SECONDS = 30

interface UsageLogsContextValue {
  selectedUserId: number | null
  setSelectedUserId: (userId: number | null) => void
  userInfoDialogOpen: boolean
  setUserInfoDialogOpen: (open: boolean) => void
  affinityTarget: ChannelAffinityInfo | null
  setAffinityTarget: (target: ChannelAffinityInfo | null) => void
  affinityDialogOpen: boolean
  setAffinityDialogOpen: (open: boolean) => void
  sensitiveVisible: boolean
  setSensitiveVisible: (visible: boolean) => void
  autoRefresh: boolean
  setAutoRefresh: (enabled: boolean) => void
  countdown: number
}

const UsageLogsContext = createContext<UsageLogsContextValue | undefined>(
  undefined
)

export function UsageLogsProvider({ children }: { children: ReactNode }) {
  const [selectedUserId, setSelectedUserId] = useState<number | null>(null)
  const [userInfoDialogOpen, setUserInfoDialogOpen] = useState(false)
  const [affinityTarget, setAffinityTarget] =
    useState<ChannelAffinityInfo | null>(null)
  const [affinityDialogOpen, setAffinityDialogOpen] = useState(false)
  const [sensitiveVisible, setSensitiveVisible] = useState(true)
  const [autoRefresh, setAutoRefreshState] = useState(false)
  const [countdown, setCountdown] = useState(AUTO_REFRESH_INTERVAL_SECONDS)

  const queryClient = useQueryClient()

  const setAutoRefresh = useCallback((enabled: boolean) => {
    setAutoRefreshState(enabled)
    if (!enabled) {
      setCountdown(AUTO_REFRESH_INTERVAL_SECONDS)
    }
  }, [])

  useEffect(() => {
    if (!autoRefresh) return

    setCountdown(AUTO_REFRESH_INTERVAL_SECONDS)

    const refreshIntervalId = setInterval(() => {
      queryClient.invalidateQueries({ queryKey: ['logs'] })
      queryClient.invalidateQueries({ queryKey: ['usage-logs-stats'] })
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
  }, [autoRefresh, queryClient])

  return (
    <UsageLogsContext.Provider
      value={{
        selectedUserId,
        setSelectedUserId,
        userInfoDialogOpen,
        setUserInfoDialogOpen,
        affinityTarget,
        setAffinityTarget,
        affinityDialogOpen,
        setAffinityDialogOpen,
        sensitiveVisible,
        setSensitiveVisible,
        autoRefresh,
        setAutoRefresh,
        countdown,
      }}
    >
      {children}
    </UsageLogsContext.Provider>
  )
}

export function useUsageLogsContext() {
  const context = useContext(UsageLogsContext)
  if (!context) {
    throw new Error('useUsageLogsContext must be used within UsageLogsProvider')
  }
  return context
}
