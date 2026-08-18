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
import { flexRender, type Cell, type Table } from '@tanstack/react-table'
import { Wrench } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

import type { ToolLog } from '../types'

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
            <Wrench />
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
        const cells = new Map<string, Cell<ToolLog, unknown>>()
        row.getVisibleCells().forEach((cell) => {
          cells.set(cell.column.id, cell)
        })
        const log = row.original
        return (
          <div
            key={row.id}
            className={cn(
              'border-border/40 space-y-2 border-b p-3 last:border-b-0',
              log.error_code && 'bg-rose-50/40 dark:bg-rose-950/20'
            )}
          >
            <div className='flex items-start justify-between gap-3'>
              <MobileCell cell={cells.get('created_at')} />
              <MobileCell cell={cells.get('tool')} />
            </div>
            <MobileCell cell={cells.get('target')} />
            <div className='grid grid-cols-2 gap-1.5'>
              <MobileCell cell={cells.get('model_name')} />
              <MobileCell cell={cells.get('duration_ms')} />
            </div>
          </div>
        )
      })}
    </div>
  )
}

function MobileCell({ cell }: { cell?: Cell<ToolLog, unknown> }) {
  if (!cell) return null
  return (
    <div className='min-w-0'>
      {flexRender(cell.column.columnDef.cell, cell.getContext())}
    </div>
  )
}
