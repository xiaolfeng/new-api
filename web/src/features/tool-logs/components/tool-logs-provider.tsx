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
import {
  createContext,
  useContext,
  useState,
  type ReactNode,
} from 'react'

import { useIsAdmin } from '@/hooks/use-admin'

export type LogsViewScope = 'all' | 'self'

interface ToolLogsContextValue {
  selectedUserId: number | null
  setSelectedUserId: (userId: number | null) => void
  userInfoDialogOpen: boolean
  setUserInfoDialogOpen: (open: boolean) => void
  viewScope: LogsViewScope
  setViewScope: (scope: LogsViewScope) => void
}

const ToolLogsContext = createContext<ToolLogsContextValue | undefined>(
  undefined
)

export function ToolLogsProvider({ children }: { children: ReactNode }) {
  const [selectedUserId, setSelectedUserId] = useState<number | null>(null)
  const [userInfoDialogOpen, setUserInfoDialogOpen] = useState(false)
  const [viewScope, setViewScope] = useState<LogsViewScope>('all')

  return (
    <ToolLogsContext.Provider
      value={{
        selectedUserId,
        setSelectedUserId,
        userInfoDialogOpen,
        setUserInfoDialogOpen,
        viewScope,
        setViewScope,
      }}
    >
      {children}
    </ToolLogsContext.Provider>
  )
}

export function useToolLogsContext() {
  const context = useContext(ToolLogsContext)
  if (!context) {
    throw new Error('useToolLogsContext must be used within ToolLogsProvider')
  }
  return context
}

export function useToolLogsViewScope() {
  const canManageScope = useIsAdmin()
  const { viewScope, setViewScope } = useToolLogsContext()
  return {
    canManageScope,
    viewScope,
    setViewScope,
    isAdminView: canManageScope && viewScope === 'all',
  }
}
