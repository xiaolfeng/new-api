/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
/* eslint-disable react-refresh/only-export-components */
import { useQueryClient } from '@tanstack/react-query'
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from 'react'

import { useIsAdmin } from '@/hooks/use-admin'

import type { ChannelAffinityInfo } from '../types'

const AUTO_REFRESH_INTERVAL_SECONDS = 30

export type LogsViewScope = 'all' | 'self'

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
  disableAutoRefresh: () => void
  isDetailOpen: boolean
  setIsDetailOpen: (open: boolean) => void
  viewScope: LogsViewScope
  setViewScope: (scope: LogsViewScope) => void
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
  const [isDetailOpen, setIsDetailOpen] = useState(false)
  const [pageVisible, setPageVisible] = useState(true)
  const consecutiveFailuresRef = useRef(0)
  const queryClient = useQueryClient()
  const isPausedRef = useRef(false)
  const [viewScope, setViewScope] = useState<LogsViewScope>('all')

  const setAutoRefresh = useCallback((enabled: boolean) => {
    setAutoRefreshState(enabled)
    if (!enabled) {
      setCountdown(AUTO_REFRESH_INTERVAL_SECONDS)
    }
  }, [])

  const disableAutoRefresh = useCallback(() => {
    setAutoRefreshState(false)
    setCountdown(AUTO_REFRESH_INTERVAL_SECONDS)
  }, [])

  useEffect(() => {
    isPausedRef.current = isDetailOpen || !pageVisible
  }, [isDetailOpen, pageVisible])

  useEffect(() => {
    const handler = () => setPageVisible(document.visibilityState === 'visible')
    document.addEventListener('visibilitychange', handler)
    return () => document.removeEventListener('visibilitychange', handler)
  }, [])

  useEffect(() => {
    if (!autoRefresh) return

    setCountdown(AUTO_REFRESH_INTERVAL_SECONDS)

    const refreshIntervalId = setInterval(() => {
      if (isPausedRef.current) return
      Promise.all([
        queryClient.invalidateQueries({ queryKey: ['logs'] }),
        queryClient.invalidateQueries({ queryKey: ['usage-logs-stats'] }),
      ])
        .then(() => {
          consecutiveFailuresRef.current = 0
        })
        .catch(() => {
          const next = consecutiveFailuresRef.current + 1
          consecutiveFailuresRef.current = next
          if (next >= 3) {
            setTimeout(() => {
              setAutoRefreshState(false)
              setCountdown(AUTO_REFRESH_INTERVAL_SECONDS)
            }, 0)
          }
        })
      setCountdown(AUTO_REFRESH_INTERVAL_SECONDS)
    }, AUTO_REFRESH_INTERVAL_SECONDS * 1000)

    const countdownIntervalId = setInterval(() => {
      if (isPausedRef.current) return
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
        disableAutoRefresh,
        isDetailOpen,
        setIsDetailOpen,
        viewScope,
        setViewScope,
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

/**
 * Resolves the effective admin scope for usage logs: whether the current
 * user is allowed to view all users' logs (`canManageScope`), and whether
 * their current view preference (`viewScope`) has that scope active
 * (`isAdminView`). Data fetching and admin-only UI should key off
 * `isAdminView` rather than raw role, so an admin who switches to "only
 * mine" is treated exactly like a regular user for that view.
 */
export function useLogsViewScope() {
  const canManageScope = useIsAdmin()
  const { viewScope, setViewScope } = useUsageLogsContext()

  return {
    canManageScope,
    viewScope,
    setViewScope,
    isAdminView: canManageScope && viewScope === 'all',
  }
}
