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
import { useQueryClient } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import type { Table } from '@tanstack/react-table'
import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { CompactDateTimeRangePicker } from '@/features/usage-logs/components/compact-date-time-range-picker'
import {
  LogsFilterField,
  LogsFilterInput,
  LogsFilterToolbar,
} from '@/features/usage-logs/components/logs-filter-toolbar'

import {
  TOOL_KIND_ALL,
  TOOL_KIND_FILTERS,
  TOOL_STATUS_ALL,
  TOOL_STATUS_FILTERS,
} from '../constants'
import { getDefaultTimeRange } from '../lib/utils'
import type { ToolLogFilters } from '../types'
import { useToolLogsViewScope } from './tool-logs-provider'

const route = getRouteApi('/_authenticated/tool-logs/')

interface ToolLogsFilterBarProps<TData> {
  table: Table<TData>
}

export function ToolLogsFilterBar<TData>(props: ToolLogsFilterBarProps<TData>) {
  const { t } = useTranslation()
  const navigate = route.useNavigate()
  const queryClient = useQueryClient()
  const search = route.useSearch()
  const { isAdminView: isAdmin } = useToolLogsViewScope()

  const defaultRange = getDefaultTimeRange()
  const [filters, setFilters] = useState<ToolLogFilters>(() => ({
    startTime: search.startTime
      ? new Date(search.startTime)
      : defaultRange.start,
    endTime: search.endTime ? new Date(search.endTime) : defaultRange.end,
    kind: search.kind || TOOL_KIND_ALL,
    error: search.error || TOOL_STATUS_ALL,
    model: search.model || '',
    token: search.token || '',
    username: search.username || '',
    channel: search.channel || '',
    requestId: search.requestId || '',
    group: search.group || '',
    q: search.q || '',
  }))

  const update = useCallback(
    <K extends keyof ToolLogFilters>(key: K, value: ToolLogFilters[K]) => {
      setFilters((prev) => ({ ...prev, [key]: value }))
    },
    []
  )

  const handleApply = useCallback(() => {
    void navigate({
      to: '/tool-logs',
      search: {
        page: 1,
        startTime: filters.startTime?.getTime(),
        endTime: filters.endTime?.getTime(),
        kind:
          !filters.kind || filters.kind === TOOL_KIND_ALL
            ? undefined
            : filters.kind,
        error:
          !filters.error || filters.error === TOOL_STATUS_ALL
            ? undefined
            : filters.error,
        model: filters.model || undefined,
        token: filters.token || undefined,
        username: isAdmin ? filters.username || undefined : undefined,
        channel: isAdmin ? filters.channel || undefined : undefined,
        requestId: filters.requestId || undefined,
        group: filters.group || undefined,
        q: filters.q || undefined,
      },
    })
    void queryClient.invalidateQueries({ queryKey: ['tool-logs'] })
  }, [filters, isAdmin, navigate, queryClient])

  const handleReset = useCallback(() => {
    const { start, end } = getDefaultTimeRange()
    setFilters({
      startTime: start,
      endTime: end,
      kind: TOOL_KIND_ALL,
      error: TOOL_STATUS_ALL,
    })
    void navigate({
      to: '/tool-logs',
      search: {
        page: 1,
        startTime: start.getTime(),
        endTime: end.getTime(),
      },
    })
    void queryClient.invalidateQueries({ queryKey: ['tool-logs'] })
  }, [navigate, queryClient])

  const hasActiveFilters = Boolean(
    (filters.kind && filters.kind !== TOOL_KIND_ALL) ||
      (filters.error && filters.error !== TOOL_STATUS_ALL) ||
      filters.model ||
      filters.token ||
      filters.username ||
      filters.channel ||
      filters.requestId ||
      filters.group ||
      filters.q
  )

  return (
    <LogsFilterToolbar
      table={props.table}
      hasActiveFilters={hasActiveFilters}
      onReset={handleReset}
      onSearch={handleApply}
      primaryFilters={
        <>
          <LogsFilterField wide>
            <CompactDateTimeRangePicker
              start={filters.startTime}
              end={filters.endTime}
              onChange={({ start, end }) => {
                update('startTime', start)
                update('endTime', end)
              }}
            />
          </LogsFilterField>
          <LogsFilterField>
            <Select
              value={filters.kind || TOOL_KIND_ALL}
              onValueChange={(value) => update('kind', value ?? TOOL_KIND_ALL)}
            >
              <SelectTrigger>
                <SelectValue placeholder={t('All Types')} />
              </SelectTrigger>
              <SelectContent>
                {TOOL_KIND_FILTERS.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {t(item.label)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </LogsFilterField>
          <LogsFilterField>
            <Select
              value={filters.error || TOOL_STATUS_ALL}
              onValueChange={(value) =>
                update('error', value ?? TOOL_STATUS_ALL)
              }
            >
              <SelectTrigger>
                <SelectValue placeholder={t('All Statuses')} />
              </SelectTrigger>
              <SelectContent>
                {TOOL_STATUS_FILTERS.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {t(item.label)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </LogsFilterField>
          <LogsFilterField>
            <LogsFilterInput
              placeholder={t('Query')}
              value={filters.q || ''}
              onChange={(e) => update('q', e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleApply()
              }}
            />
          </LogsFilterField>
        </>
      }
      advancedFilters={
        <>
          <LogsFilterField>
            <LogsFilterInput
              placeholder={t('Model Name')}
              value={filters.model || ''}
              onChange={(e) => update('model', e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleApply()
              }}
            />
          </LogsFilterField>
          <LogsFilterField>
            <LogsFilterInput
              placeholder={t('Token Name')}
              value={filters.token || ''}
              onChange={(e) => update('token', e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleApply()
              }}
            />
          </LogsFilterField>
          <LogsFilterField>
            <LogsFilterInput
              placeholder={t('Request ID')}
              value={filters.requestId || ''}
              onChange={(e) => update('requestId', e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleApply()
              }}
            />
          </LogsFilterField>
          <LogsFilterField>
            <LogsFilterInput
              placeholder={t('Group')}
              value={filters.group || ''}
              onChange={(e) => update('group', e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleApply()
              }}
            />
          </LogsFilterField>
          {isAdmin && (
            <>
              <LogsFilterField>
                <LogsFilterInput
                  placeholder={t('Username')}
                  value={filters.username || ''}
                  onChange={(e) => update('username', e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') handleApply()
                  }}
                />
              </LogsFilterField>
              <LogsFilterField>
                <LogsFilterInput
                  placeholder={t('Channel')}
                  value={filters.channel || ''}
                  onChange={(e) => update('channel', e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') handleApply()
                  }}
                />
              </LogsFilterField>
            </>
          )}
        </>
      }
    />
  )
}
