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
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { ModelBadge } from '@/features/usage-logs/components/model-badge'
import { getUserAvatarFallback, getUserAvatarStyle } from '@/lib/avatar'
import { formatTimestampToDate } from '@/lib/format'

import type { ToolLog } from '../types'
import { formatDurationMs } from '../lib/utils'
import {
  ToolIdentityCell,
  ToolRequestIdCell,
  ToolResultCell,
  ToolStatusStack,
  ToolTargetCell,
  ToolTokenCell,
} from './tool-log-cells'
import { useToolLogsContext } from './tool-logs-provider'

export function useToolLogsColumns(isAdmin: boolean): ColumnDef<ToolLog>[] {
  const { t } = useTranslation()
  const columns: ColumnDef<ToolLog>[] = [
    {
      accessorKey: 'created_at',
      header: t('Time'),
      cell: ({ row }) => {
        const log = row.original
        return (
          <div className='flex min-w-0 flex-col gap-0.5'>
            <span className='truncate font-mono text-xs tabular-nums'>
              {formatTimestampToDate(log.created_at)}
            </span>
            <ToolStatusStack log={log} />
          </div>
        )
      },
      enableHiding: false,
      size: 180,
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
          const label = log.channel_name
            ? `${log.channel_name} #${log.channel}`
            : `#${log.channel}`
          return (
            <StatusBadge
              label={label}
              autoColor={String(log.channel)}
              copyText={String(log.channel)}
              size='sm'
              showDot={false}
              className='font-mono'
            />
          )
        },
        size: 140,
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
      cell: ({ row }) => <ToolTokenCell log={row.original} />,
      size: 130,
    },
    {
      accessorKey: 'model_name',
      header: t('Model'),
      cell: ({ row }) =>
        row.original.model_name ? (
          <ModelBadge modelName={row.original.model_name} />
        ) : (
          <span className='text-muted-foreground text-xs'>-</span>
        ),
      size: 160,
    },
    {
      id: 'tool',
      header: t('Tool'),
      cell: ({ row }) => <ToolIdentityCell log={row.original} />,
      size: 130,
    },
    {
      id: 'target',
      header: t('Query'),
      cell: ({ row }) => <ToolTargetCell log={row.original} />,
      size: 220,
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
      accessorKey: 'request_id',
      header: t('Request ID'),
      cell: ({ row }) => <ToolRequestIdCell log={row.original} />,
      size: 150,
    },
    {
      id: 'details',
      header: t('Details'),
      cell: ({ row }) => (
        <ToolResultCell log={row.original} isAdmin={isAdmin} />
      ),
      size: 180,
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
      accessorKey: 'mode',
      header: t('Mode'),
      cell: ({ row }) => (
        <span className='text-muted-foreground font-mono text-xs'>
          {row.original.mode || '-'}
        </span>
      ),
      size: 80,
    },
    {
      accessorKey: 'canonical',
      header: t('Canonical'),
      cell: ({ row }) => (
        <span className='text-muted-foreground font-mono text-xs'>
          {row.original.canonical || '-'}
        </span>
      ),
      size: 140,
    },
    {
      accessorKey: 'ip',
      header: t('IP'),
      cell: ({ row }) => (
        <span className='text-muted-foreground font-mono text-xs'>
          {row.original.ip || '-'}
        </span>
      ),
      size: 110,
    }
  )

  return columns
}
