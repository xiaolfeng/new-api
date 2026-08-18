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
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DataTablePage, useDataTable } from '@/components/data-table'
import { useMediaQuery } from '@/hooks'
import { useTableUrlState } from '@/hooks/use-table-url-state'

import { fetchToolLogs } from '../api'
import { DEFAULT_TOOL_LOGS_DATA } from '../constants'
import { getDefaultTimeRange, timestampToSeconds } from '../lib/utils'
import type { ToolLog } from '../types'
import { useToolLogsColumns } from './tool-logs-columns'
import { ToolLogsFilterBar } from './tool-logs-filter-bar'
import { ToolLogsMobileList } from './tool-logs-mobile-card'
import { useToolLogsViewScope } from './tool-logs-provider'

const route = getRouteApi('/_authenticated/tool-logs/')

export function ToolLogsTable() {
  const { t } = useTranslation()
  const { isAdminView: isAdmin } = useToolLogsViewScope()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const searchParams = route.useSearch()

  const {
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: isMobile ? 20 : 100 },
    globalFilter: { enabled: false },
    columnFilters: [
      { columnId: 'model_name', searchKey: 'model', type: 'string' },
      { columnId: 'token_name', searchKey: 'token', type: 'string' },
      ...(isAdmin
        ? [
            { columnId: 'channel', searchKey: 'channel', type: 'string' as const },
            { columnId: 'username', searchKey: 'username', type: 'string' as const },
          ]
        : []),
    ],
  })

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'tool-logs',
      isAdmin,
      pagination.pageIndex + 1,
      pagination.pageSize,
      searchParams,
    ],
    queryFn: async () => {
      const defaultRange = getDefaultTimeRange()
      const result = await fetchToolLogs(
        {
          p: pagination.pageIndex + 1,
          page_size: pagination.pageSize,
          start_timestamp: timestampToSeconds(
            searchParams.startTime || defaultRange.start.getTime()
          ),
          end_timestamp: timestampToSeconds(
            searchParams.endTime || defaultRange.end.getTime()
          ),
          kind: searchParams.kind,
          error: searchParams.error,
          model_name: searchParams.model,
          token_name: searchParams.token,
          username: isAdmin ? searchParams.username : undefined,
          channel: isAdmin ? searchParams.channel : undefined,
          request_id: searchParams.requestId,
          group: searchParams.group,
          q: searchParams.q,
        },
        isAdmin
      )
      if (!result?.success) {
        toast.error(result?.message || t('Failed to load logs'))
        return DEFAULT_TOOL_LOGS_DATA
      }
      return result.data || DEFAULT_TOOL_LOGS_DATA
    },
  })

  const logs = data?.items || []
  const columns = useToolLogsColumns(isAdmin)
  const isLoadingData = isLoading || (isFetching && !data)

  const { table } = useDataTable({
    data: logs as unknown as Record<string, unknown>[],
    columns: columns as ColumnDef<Record<string, unknown>>[],
    columnFilters,
    columnVisibilityStorageKey: `tool-logs:${isAdmin ? 'admin' : 'user'}:column-visibility`,
    initialColumnVisibility: {
      backend: false,
      mode: false,
      canonical: false,
      ip: false,
    },
    pagination,
    enableRowSelection: false,
    onPaginationChange,
    onColumnFiltersChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: data?.total || 0,
    ensurePageInRange,
  })

  return (
    <DataTablePage
      table={table}
      columns={columns as ColumnDef<Record<string, unknown>>[]}
      isLoading={isLoadingData}
      isFetching={isFetching}
      emptyTitle={t('No tool logs yet')}
      emptyDescription={t('Host tool calls will appear here after they run.')}
      skeletonKeyPrefix='tool-log-skeleton'
      applyHeaderSize
      tableClassName='[&_[data-slot=table]]:text-[13px] [&_[data-slot=table]_td]:text-[13px] [&_[data-slot=table]_th]:text-[13px]'
      mobile={
        <ToolLogsMobileList
          table={table as unknown as import('@tanstack/react-table').Table<ToolLog>}
          isLoading={isLoadingData}
        />
      }
      toolbar={<ToolLogsFilterBar table={table} />}
    />
  )
}
