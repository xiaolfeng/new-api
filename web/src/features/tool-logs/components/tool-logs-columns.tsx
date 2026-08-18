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
import type { ColumnDef } from '@tanstack/react-table'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { getUserAvatarFallback, getUserAvatarStyle } from '@/lib/avatar'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import type { ToolLog } from '../types'
import { formatDurationMs, toolTarget } from '../lib/utils'
import { ToolResultDialog } from './tool-result-dialog'
import { useToolLogsContext } from './tool-logs-provider'

export function useToolLogsColumns(isAdmin: boolean): ColumnDef<ToolLog>[] {
  const { t } = useTranslation()
  const columns: ColumnDef<ToolLog>[] = [
    {
      accessorKey: 'created_at',
      header: t('Time'),
      cell: ({ row }) => {
        const log = row.original
        const ok = !log.error_code
        return (
          <div className='flex min-w-0 flex-col gap-0.5'>
            <span className='truncate font-mono text-xs tabular-nums'>
              {formatTimestampToDate(log.created_at)}
            </span>
            <StatusBadge
              label={ok ? t('Success') : log.error_code}
              variant={ok ? 'green' : 'red'}
              size='sm'
              copyable={false}
              className='-ml-1.5 !text-xs [&_span]:!text-xs'
            />
          </div>
        )
      },
      enableHiding: false,
      size: 170,
    },
  ]

  if (isAdmin) {
    columns.push(
      {
        id: 'channel',
        header: t('Channel'),
        cell: ({ row }) => {
          const log = row.original
          if (!log.channel) {
            return <span className='text-muted-foreground text-xs'>-</span>
          }
          return (
            <StatusBadge
              label={`#${log.channel}`}
              autoColor={String(log.channel)}
              copyText={String(log.channel)}
              size='sm'
              showDot={false}
              className='font-mono'
            />
          )
        },
        size: 90,
      },
      {
        id: 'user',
        header: t('User'),
        cell: function UserCell({ row }) {
          const { setSelectedUserId, setUserInfoDialogOpen } =
            useToolLogsContext()
          const log = row.original
          if (!log.username) {
            return <span className='text-muted-foreground text-xs'>-</span>
          }
          return (
            <button
              type='button'
              className='flex min-w-0 items-center gap-1.5 text-left'
              onClick={() => {
                setSelectedUserId(log.user_id)
                setUserInfoDialogOpen(true)
              }}
            >
              <Avatar className='ring-border/60 size-6 shrink-0 ring-1'>
                <AvatarFallback
                  className='text-[11px] font-semibold'
                  style={getUserAvatarStyle(log.username)}
                >
                  {getUserAvatarFallback(log.username)}
                </AvatarFallback>
              </Avatar>
              <span className='truncate text-xs'>{log.username}</span>
            </button>
          )
        },
        size: 120,
      }
    )
  }

  columns.push(
    {
      accessorKey: 'token_name',
      header: t('Token'),
      cell: ({ row }) => (
        <span className='truncate text-xs'>{row.original.token_name || '-'}</span>
      ),
      size: 110,
    },
    {
      accessorKey: 'model_name',
      header: t('Model'),
      cell: ({ row }) => (
        <span className='truncate font-mono text-xs'>
          {row.original.model_name || '-'}
        </span>
      ),
      size: 150,
    },
    {
      id: 'tool',
      header: t('Tool'),
      cell: ({ row }) => {
        const log = row.original
        const isFetch = log.kind === 'fetch'
        return (
          <div className='flex min-w-0 flex-col gap-0.5'>
            <StatusBadge
              label={isFetch ? t('Fetch') : t('Search')}
              variant={isFetch ? 'orange' : 'blue'}
              size='sm'
              copyable={false}
            />
            <span className='text-muted-foreground truncate font-mono text-[11px]'>
              {log.original_name || log.canonical || '-'}
            </span>
          </div>
        )
      },
      size: 120,
    },
    {
      id: 'target',
      header: t('Query'),
      cell: ({ row }) => {
        const target = toolTarget(row.original.query, row.original.url)
        if (!target) {
          return <span className='text-muted-foreground text-xs'>-</span>
        }
        return (
          <span className='line-clamp-2 text-xs break-all' title={target}>
            {target}
          </span>
        )
      },
      size: 220,
    },
    {
      accessorKey: 'backend',
      header: t('Backend'),
      cell: ({ row }) => (
        <span className='text-muted-foreground font-mono text-xs'>
          {row.original.backend || '-'}
        </span>
      ),
      size: 90,
    },
    {
      accessorKey: 'duration_ms',
      header: t('Duration'),
      cell: ({ row }) => (
        <span className='font-mono text-xs tabular-nums'>
          {formatDurationMs(row.original.duration_ms)}
        </span>
      ),
      size: 80,
    },
    {
      id: 'details',
      header: t('Details'),
      cell: function DetailsCell({ row }) {
        const [open, setOpen] = useState(false)
        const log = row.original
        const preview = log.result?.trim()
        return (
          <>
            <button
              type='button'
              className={cn(
                'max-w-[180px] truncate text-left text-xs hover:underline',
                preview
                  ? 'text-foreground'
                  : 'text-muted-foreground/50 cursor-default hover:no-underline'
              )}
              onClick={() => preview && setOpen(true)}
            >
              {preview || '—'}
            </button>
            <ToolResultDialog
              result={log.result || ''}
              open={open}
              onOpenChange={setOpen}
            />
          </>
        )
      },
      size: 180,
    }
  )

  return columns
}
