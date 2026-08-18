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
import { useCallback } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { UserInfoDialog } from '@/features/usage-logs/components/dialogs/user-info-dialog'

import { ToolLogsTable } from './components/tool-logs-table'
import {
  ToolLogsProvider,
  useToolLogsContext,
  useToolLogsViewScope,
  type LogsViewScope,
} from './components/tool-logs-provider'

function ToolLogsContent() {
  const { t } = useTranslation()
  const { canManageScope, viewScope, setViewScope } = useToolLogsViewScope()
  const { selectedUserId, userInfoDialogOpen, setUserInfoDialogOpen } =
    useToolLogsContext()

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
