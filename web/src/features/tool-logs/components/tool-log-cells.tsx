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
import { Link, getRouteApi } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { GroupBadge } from '@/components/group-badge'
import { StatusBadge } from '@/components/status-badge'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

import {
  previewToolResult,
  toolTarget,
  usageLogsSearchForRequest,
} from '../lib/utils'
import type { ToolLog } from '../types'
import { ToolKindBadge } from './tool-kind-badge'
import { ToolResultDialog } from './tool-result-dialog'

const route = getRouteApi('/_authenticated/tool-logs/')

export function ToolIdentityCell(props: { log: ToolLog }) {
  const { t } = useTranslation()
  const log = props.log
  const name = log.original_name || log.canonical || '-'

  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger
          render={
            <div className='flex min-w-0 cursor-help flex-col gap-0.5' />
          }
        >
          <ToolKindBadge kind={log.kind} />
          <span className='text-muted-foreground truncate font-mono text-[11px]'>
            {name}
          </span>
        </TooltipTrigger>
        <TooltipContent className='max-w-xs space-y-0.5'>
          <p>{t('Used host tool')}</p>
          <p className='font-mono text-xs'>
            {log.canonical && log.canonical !== name
              ? `${name} → ${log.canonical}`
              : name}
          </p>
          <p className='text-muted-foreground'>
            {[log.mode, log.backend].filter(Boolean).join(' · ') ||
              log.canonical}
          </p>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

export function ToolTargetCell(props: { log: ToolLog }) {
  const target = toolTarget(props.log.query, props.log.url, props.log.kind)
  if (!target) {
    return <span className='text-muted-foreground text-xs'>-</span>
  }

  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger
          render={
            <span className='line-clamp-2 cursor-help text-xs break-all' />
          }
        >
          {target}
        </TooltipTrigger>
        <TooltipContent className='max-w-sm break-all'>{target}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

export function ToolTokenCell(props: { log: ToolLog }) {
  const tokenName = props.log.token_name
  if (!tokenName && !props.log.group) {
    return <span className='text-muted-foreground text-xs'>-</span>
  }

  return (
    <div className='flex max-w-[160px] flex-col gap-0.5'>
      {tokenName ? (
        <StatusBadge
          label={tokenName}
          size='sm'
          showDot={false}
          className='border-border/60 bg-muted/30 text-foreground h-6 max-w-full gap-1.5 overflow-hidden rounded-md border px-2 py-0.5 [font-family:var(--font-body)]'
        />
      ) : null}
      {props.log.group ? (
        <GroupBadge
          group={props.log.group}
          type='text'
          size='sm'
          className='inline align-baseline text-xs leading-none [&>span]:leading-none'
        />
      ) : null}
    </div>
  )
}

export function ToolRequestIdCell(props: { log: ToolLog }) {
  const search = route.useSearch()
  const requestId = props.log.request_id
  if (!requestId) {
    return <span className='text-muted-foreground text-xs'>-</span>
  }

  return (
    <Link
      to='/usage-logs/$section'
      params={{ section: 'common' }}
      search={usageLogsSearchForRequest({
        requestId,
        startTime: search.startTime,
        endTime: search.endTime,
      })}
      className='text-primary block max-w-[160px] truncate font-mono text-xs hover:underline'
    >
      {requestId}
    </Link>
  )
}

export function ToolResultCell(props: { log: ToolLog; isAdmin: boolean }) {
  const [open, setOpen] = useState(false)
  const search = route.useSearch()
  const preview = previewToolResult(props.log.result || '')

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
        log={props.log}
        open={open}
        onOpenChange={setOpen}
        isAdmin={props.isAdmin}
        usageLogsSearch={{
          startTime: search.startTime,
          endTime: search.endTime,
        }}
      />
    </>
  )
}

export function ToolStatusStack(props: { log: ToolLog }) {
  const { t } = useTranslation()
  const ok = !props.log.error_code

  return (
    <div className='flex flex-wrap items-center gap-1'>
      <StatusBadge
        label={ok ? t('Success') : props.log.error_code}
        variant={ok ? 'green' : 'red'}
        size='sm'
        copyable={false}
        className='-ml-1.5 !text-xs [&_span]:!text-xs'
      />
      {props.log.truncated ? (
        <TooltipProvider>
          <Tooltip>
            <TooltipTrigger
              render={
                <StatusBadge
                  label={t('Truncated')}
                  variant='warning'
                  size='sm'
                  copyable={false}
                  className='cursor-help !text-xs [&_span]:!text-xs'
                />
              }
            />
            <TooltipContent>
              {t('This host tool result was truncated and may be incomplete.')}
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>
      ) : null}
    </div>
  )
}
