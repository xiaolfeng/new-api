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
import { Copy01Icon, Tick02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

import { formatDurationMs, prettyToolResult } from '../lib/utils'
import type { ToolLog } from '../types'
import { ToolKindBadge } from './tool-kind-badge'
import { ToolRequestIdLink } from './tool-log-cells'

interface ToolResultDialogProps {
  log: ToolLog | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onOpenChangeComplete?: (open: boolean) => void
  isAdmin?: boolean
}

function MetaRow(props: { label: string; children: ReactNode }) {
  return (
    <div className='grid grid-cols-[7.5rem_minmax(0,1fr)] items-start gap-2 text-xs'>
      <span className='text-muted-foreground pt-0.5'>{props.label}</span>
      <div className='min-w-0 break-all'>{props.children}</div>
    </div>
  )
}

export function ToolResultDialog(props: ToolResultDialogProps) {
  const { t } = useTranslation()
  const log = props.log
  const formatted = log ? prettyToolResult(log.result || '') : ''
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })
  const copied = Boolean(formatted) && copiedText === formatted
  const ok = Boolean(log) && !log?.error_code

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      onOpenChangeComplete={props.onOpenChangeComplete}
      title={t('Tool Result')}
      description={t('View the complete host tool result')}
      contentClassName='sm:max-w-2xl'
      contentHeight='auto'
      bodyClassName='space-y-3'
    >
      {log ? (
        <>
          <div className='flex flex-wrap items-center gap-1.5'>
            <ToolKindBadge kind={log.kind} />
            <span className='font-mono text-xs'>
              {log.original_name || log.canonical || '-'}
            </span>
            <StatusBadge
              label={ok ? t('Success') : log.error_code}
              variant={ok ? 'green' : 'red'}
              size='sm'
              copyable={false}
            />
            {log.truncated ? (
              <StatusBadge
                label={t('Truncated')}
                variant='warning'
                size='sm'
                copyable={false}
              />
            ) : null}
          </div>

          {log.truncated ? (
            <Alert>
              <AlertTitle>{t('Result truncated')}</AlertTitle>
              <AlertDescription>
                {t('This host tool result was truncated and may be incomplete.')}
              </AlertDescription>
            </Alert>
          ) : null}

          <div className='space-y-1.5'>
            {log.canonical ? (
              <MetaRow label={t('Canonical')}>
                <span className='font-mono'>{log.canonical}</span>
              </MetaRow>
            ) : null}
            {log.backend ? (
              <MetaRow label={t('Backend')}>
                <span className='font-mono'>{log.backend}</span>
              </MetaRow>
            ) : null}
            {log.mode ? (
              <MetaRow label={t('Mode')}>
                <span className='font-mono'>{log.mode}</span>
              </MetaRow>
            ) : null}
            <MetaRow label={t('Duration')}>
              <span className='font-mono tabular-nums'>
                {formatDurationMs(log.duration_ms)}
              </span>
            </MetaRow>
            {log.model_name ? (
              <MetaRow label={t('Model')}>{log.model_name}</MetaRow>
            ) : null}
            {log.token_name ? (
              <MetaRow label={t('Token')}>{log.token_name}</MetaRow>
            ) : null}
            {log.request_id ? (
              <MetaRow label={t('Request ID')}>
                <ToolRequestIdLink
                  requestId={log.request_id}
                  className='text-primary hover:underline'
                />
              </MetaRow>
            ) : null}
            {props.isAdmin && log.channel ? (
              <MetaRow label={t('Channel')}>
                <span className='font-mono'>
                  {log.channel_name
                    ? `${log.channel_name} #${log.channel}`
                    : `#${log.channel}`}
                </span>
              </MetaRow>
            ) : null}
            {props.isAdmin && log.ip ? (
              <MetaRow label={t('IP')}>
                <span className='font-mono'>{log.ip}</span>
              </MetaRow>
            ) : null}
            {log.query ? (
              <MetaRow label={t('Query')}>
                <span>{log.query}</span>
              </MetaRow>
            ) : null}
            {log.url ? (
              <MetaRow label={t('URL')}>
                <span className='font-mono'>{log.url}</span>
              </MetaRow>
            ) : null}
          </div>

          <div className='flex justify-end'>
            <Button
              type='button'
              variant='ghost'
              size='sm'
              disabled={!formatted}
              onClick={() => void copyToClipboard(formatted)}
            >
              <HugeiconsIcon
                icon={copied ? Tick02Icon : Copy01Icon}
                strokeWidth={2}
              />
              <span className='ml-1.5'>{copied ? t('Copied') : t('Copy')}</span>
            </Button>
          </div>
          <ScrollArea className='max-h-[480px]'>
            <pre className='bg-muted/50 rounded-md border p-3 font-mono text-xs break-all whitespace-pre-wrap'>
              {formatted || t('No result')}
            </pre>
          </ScrollArea>
        </>
      ) : null}
    </Dialog>
  )
}
