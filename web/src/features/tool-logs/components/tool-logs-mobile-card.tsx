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
import { Wrench01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { flexRender, type Table } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'

import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import { formatDurationMs, toolTarget } from '../lib/utils'
import type { ToolLog } from '../types'
import { ToolKindBadge } from './tool-kind-badge'
import {
  ToolRequestIdCell,
  ToolResultCell,
  ToolStatusStack,
} from './tool-log-cells'

interface ToolLogsMobileListProps {
  table: Table<ToolLog>
  isLoading?: boolean
}

export function ToolLogsMobileList(props: ToolLogsMobileListProps) {
  const { t } = useTranslation()
  const rows = props.table.getRowModel().rows

  if (props.isLoading) {
    return (
      <div className='border-border/50 bg-card overflow-hidden rounded-lg border'>
        {[1, 2, 3].map((i) => (
          <div
            key={i}
            className='border-border/40 space-y-2 border-b p-3 last:border-b-0'
          >
            <Skeleton className='h-5 w-40 rounded-md' />
            <Skeleton className='h-4 w-full rounded-md' />
          </div>
        ))}
      </div>
    )
  }

  if (rows.length === 0) {
    return (
      <Empty className='border'>
        <EmptyHeader>
          <EmptyMedia variant='icon'>
            <HugeiconsIcon icon={Wrench01Icon} strokeWidth={2} />
          </EmptyMedia>
          <EmptyTitle>{t('No tool logs yet')}</EmptyTitle>
          <EmptyDescription>
            {t('Host tool calls will appear here after they run.')}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  return (
    <div className='border-border/50 bg-card overflow-hidden rounded-lg border'>
      {rows.map((row) => {
        const log = row.original
        const target = toolTarget(log.query, log.url, log.kind)
        const userCell = row
          .getVisibleCells()
          .find((cell) => cell.column.id === 'user')
        return (
          <div
            key={row.id}
            className={cn(
              'border-border/40 space-y-2 border-b p-3 last:border-b-0',
              log.error_code && 'bg-rose-50/40 dark:bg-rose-950/20'
            )}
          >
            <div className='flex items-start justify-between gap-3'>
              <div className='min-w-0 space-y-1'>
                <div className='font-mono text-xs tabular-nums'>
                  {formatTimestampToDate(log.created_at)}
                </div>
                <ToolStatusStack log={log} />
              </div>
              <div className='flex min-w-0 flex-col items-end gap-0.5'>
                <ToolKindBadge kind={log.kind} />
                <span className='text-muted-foreground truncate font-mono text-[11px]'>
                  {log.original_name || log.canonical || '-'}
                </span>
              </div>
            </div>

            <div className='text-sm break-all'>
              {target || (
                <span className='text-muted-foreground text-xs'>-</span>
              )}
            </div>

            <div className='text-muted-foreground flex flex-wrap items-center gap-x-2 gap-y-1 text-xs'>
              {log.model_name ? (
                <span className='font-mono'>{log.model_name}</span>
              ) : null}
              <span className='font-mono tabular-nums'>
                {formatDurationMs(log.duration_ms)}
              </span>
              {userCell
                ? flexRender(
                    userCell.column.columnDef.cell,
                    userCell.getContext()
                  )
                : null}
              {log.token_name ? <span>{log.token_name}</span> : null}
            </div>

            {log.request_id ? <ToolRequestIdCell log={log} /> : null}
            <ToolResultCell log={log} />
          </div>
        )
      })}
    </div>
  )
}
