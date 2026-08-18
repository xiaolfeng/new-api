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
import { useNavigate } from '@tanstack/react-router'
import { useCallback, useEffect } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { UserInfoDialog } from '@/features/usage-logs/components/dialogs/user-info-dialog'

import { ToolLogsTable } from './components/tool-logs-table'
import { ToolResultDialog } from './components/tool-result-dialog'
import {
  ToolLogsProvider,
  useToolLogsContext,
  useToolLogsViewScope,
  type LogsViewScope,
} from './components/tool-logs-provider'

function ToolLogsContent() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { canManageScope, viewScope, setViewScope, isAdminView } =
    useToolLogsViewScope()
  const {
    selectedUserId,
    userInfoDialogOpen,
    setUserInfoDialogOpen,
    selectedLog,
    resultDialogOpen,
    setResultDialogOpen,
    takePendingUsageLogsSearch,
  } = useToolLogsContext()

  const flushPendingUsageLogsNavigation = useCallback(
    (open: boolean) => {
      if (open) return
      const search = takePendingUsageLogsSearch()
      if (!search) return
      void navigate({
        to: '/usage-logs/$section',
        params: { section: 'common' },
        search,
      })
    },
    [navigate, takePendingUsageLogsSearch]
  )

  useEffect(() => {
    if (resultDialogOpen || userInfoDialogOpen) return
    const timer = window.setTimeout(() => {
      flushPendingUsageLogsNavigation(false)
    }, 300)
    return () => window.clearTimeout(timer)
  }, [
    flushPendingUsageLogsNavigation,
    resultDialogOpen,
    userInfoDialogOpen,
  ])

  const handleViewScopeChange = useCallback(
    (scope: string) => {
      if (scope === 'all' || scope === 'self') {
        setViewScope(scope as LogsViewScope)
      }
    },
    [setViewScope]
  )

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>{t('Tool Logs')}</SectionPageLayout.Title>
        {canManageScope && (
          <SectionPageLayout.Actions>
            <Tabs value={viewScope} onValueChange={handleViewScopeChange}>
              <TabsList>
                <TabsTrigger value='all'>{t('All')}</TabsTrigger>
                <TabsTrigger value='self'>{t('Only Mine')}</TabsTrigger>
              </TabsList>
            </Tabs>
          </SectionPageLayout.Actions>
        )}
        <SectionPageLayout.Content>
          <div className='flex h-full min-h-0 flex-col'>
            <div className='min-h-0 flex-1'>
              <ToolLogsTable />
            </div>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>
      <UserInfoDialog
        userId={selectedUserId}
        open={userInfoDialogOpen}
        onOpenChange={setUserInfoDialogOpen}
        onOpenChangeComplete={flushPendingUsageLogsNavigation}
      />
      <ToolResultDialog
        log={selectedLog}
        open={resultDialogOpen}
        onOpenChange={setResultDialogOpen}
        onOpenChangeComplete={flushPendingUsageLogsNavigation}
        isAdmin={isAdminView}
      />
    </>
  )
}

export function ToolLogsPage() {
  return (
    <ToolLogsProvider>
      <ToolLogsContent />
    </ToolLogsProvider>
  )
}
