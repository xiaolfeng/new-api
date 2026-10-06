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
import { CircleAlert } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  dotColorMap,
  textColorMap,
  type StatusVariant,
} from '@/components/status-badge'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

import { getFirstResponseTimeColor, getResponseTimeColor } from '../lib/format'
import { getLogTiming, LEGACY_TIMING_NOTE, type TimingLog } from '../lib/timing'
import type { LogOtherData } from '../types'

/**
 * Softened fills for the full-height timing bar. The bar sits directly beside
 * dense numeric text, so the saturated `dotColorMap` tones (tuned for small
 * dots and badges) read as too high-contrast at that size; a translucent fill
 * keeps the status legible while matching the page's muted palette.
 */
const barColorMap: Record<StatusVariant, string> = {
  ...dotColorMap,
  success: 'bg-success/90',
  warning: 'bg-warning/80',
  danger: 'bg-destructive/80',
  neutral: 'bg-neutral/80',
}

interface PhaseTiming {
  thinkingMs?: number | null
  contentMs?: number | null
  toolMs?: number | null
}

interface TimingMetricsCellProps {
  log: TimingLog
  other?: LogOtherData | null
  className?: string
  /**
   * `bar` (default) draws a full-height color segment beside the labels,
   * matching the dense desktop table. `dot` swaps that segment for small
   * status dots inline with each label, matching the lighter-weight status
   * indicator used elsewhere on the mobile card.
   */
  indicator?: 'bar' | 'dot'
  compact?: boolean
}

export function TimingMetricsCell(props: TimingMetricsCellProps) {
  const { t } = useTranslation()
  const indicator = props.indicator ?? 'bar'
  const metrics = getLogTiming(props.log, props.other)
  const showFirstToken = props.log.is_stream
  const firstTokenSeconds =
    metrics.firstMs == null ? null : metrics.firstMs / 1000
  const firstTokenVariant: StatusVariant =
    firstTokenSeconds == null
      ? 'neutral'
      : getFirstResponseTimeColor(firstTokenSeconds)
  const totalTimeVariant: StatusVariant =
    metrics.totalMs === null
      ? 'neutral'
      : getResponseTimeColor(
          metrics.totalMs / 1000,
          props.log.completion_tokens
        )
  const firstTokenLabel = metrics.firstLabel ?? t('N/A')
  const totalTimeLabel = metrics.totalLabel ?? t('N/A')

  const phase: PhaseTiming | undefined = metrics.phaseTiming
  const thinkingMs =
    typeof phase?.thinkingMs === 'number' && phase.thinkingMs > 0
      ? phase.thinkingMs
      : null
  const contentMs =
    typeof phase?.contentMs === 'number' && phase.contentMs > 0
      ? phase.contentMs
      : null
  const toolMs =
    typeof phase?.toolMs === 'number' && phase.toolMs > 0 ? phase.toolMs : null
  const hasPhaseTiming =
    thinkingMs != null || contentMs != null || toolMs != null

  const labels = (
    <div
      className={cn(
        'flex min-h-8 min-w-0 flex-col justify-center gap-0.5 text-xs leading-tight',
        props.compact &&
          'min-h-0 flex-row flex-wrap items-center gap-x-2.5 gap-y-1'
      )}
    >
      {showFirstToken && (
        <div className='flex items-baseline gap-1.5'>
          {indicator === 'dot' && (
            <span
              aria-hidden
              className={cn(
                'size-1.5 shrink-0 rounded-full',
                dotColorMap[firstTokenVariant]
              )}
            />
          )}
          <span className='text-muted-foreground shrink-0'>
            {t('First token')}
          </span>
          <span className={cn('tabular-nums', textColorMap[firstTokenVariant])}>
            {firstTokenLabel}
          </span>
        </div>
      )}
      <div className='flex items-baseline gap-1.5'>
        {indicator === 'dot' && (
          <span
            aria-hidden
            className={cn(
              'size-1.5 shrink-0 rounded-full',
              dotColorMap[totalTimeVariant]
            )}
          />
        )}
        <span className='text-muted-foreground shrink-0'>{t('Duration')}</span>
        <span className={cn('tabular-nums', textColorMap[totalTimeVariant])}>
          {totalTimeLabel}
        </span>
      </div>
      {metrics.legacy && (
        <span className='text-muted-foreground/60 basis-full text-[10px] leading-tight whitespace-normal'>
          {t(LEGACY_TIMING_NOTE)}
        </span>
      )}
    </div>
  )

  if (indicator === 'dot') {
    const cell = (
      <div className={cn('flex items-stretch', props.className)}>{labels}</div>
    )
    if (!hasPhaseTiming) return cell
    return (
      <TooltipProvider>
        <Tooltip>
          <TooltipTrigger render={cell} />
          <PhaseTimingTooltipContent
            thinkingMs={thinkingMs}
            contentMs={contentMs}
            toolMs={toolMs}
          />
        </Tooltip>
      </TooltipProvider>
    )
  }

  const cell = (
    <div className={cn('flex items-stretch gap-2', props.className)}>
      <span
        aria-hidden
        className={cn(
          'flex w-1 shrink-0 flex-col overflow-hidden rounded-full',
          !showFirstToken && barColorMap[totalTimeVariant]
        )}
      >
        {showFirstToken && (
          <>
            <span className={cn('flex-1', barColorMap[firstTokenVariant])} />
            <span className={cn('flex-1', barColorMap[totalTimeVariant])} />
          </>
        )}
      </span>
      {labels}
    </div>
  )
  if (!hasPhaseTiming) return cell
  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger render={cell} />
        <PhaseTimingTooltipContent
          thinkingMs={thinkingMs}
          contentMs={contentMs}
          toolMs={toolMs}
        />
      </Tooltip>
    </TooltipProvider>
  )
}

function PhaseTimingTooltipContent({
  thinkingMs,
  contentMs,
  toolMs,
}: {
  thinkingMs: number | null
  contentMs: number | null
  toolMs: number | null
}) {
  const { t } = useTranslation()
  return (
    <TooltipContent side='bottom' className='max-w-[220px] p-2'>
      <div className='space-y-1 text-xs'>
        <p className='text-muted-foreground'>{t('Upstream timing')}</p>
        {thinkingMs != null && (
          <div className='flex items-center gap-1.5'>
            <span className='text-violet-500 dark:text-violet-400'>◆</span>
            <span className='text-muted-foreground'>{t('Thinking')}</span>
            <span className='ml-auto font-mono text-violet-600 tabular-nums dark:text-violet-400'>
              {(thinkingMs / 1000).toFixed(2)}s
            </span>
          </div>
        )}
        {contentMs != null && (
          <div className='flex items-center gap-1.5'>
            <span className='text-sky-500 dark:text-sky-400'>◆</span>
            <span className='text-muted-foreground'>{t('Output')}</span>
            <span className='ml-auto font-mono text-sky-600 tabular-nums dark:text-sky-400'>
              {(contentMs / 1000).toFixed(2)}s
            </span>
          </div>
        )}
        {toolMs != null && (
          <div className='flex items-center gap-1.5'>
            <span className='text-amber-500 dark:text-amber-400'>◆</span>
            <span className='text-muted-foreground'>{t('Tool')}</span>
            <span className='ml-auto font-mono text-amber-600 tabular-nums dark:text-amber-400'>
              {(toolMs / 1000).toFixed(2)}s
            </span>
          </div>
        )}
      </div>
    </TooltipContent>
  )
}

interface StreamTpsCellProps {
  isStream: boolean
  compact?: boolean
  /** Task logs are asynchronous jobs; stream vs non-stream does not apply. */
  isTask?: boolean
  /** The task request returned its result inline, so it is a synchronous call. */
  isSyncTask?: boolean
  tokensPerSecond?: number | null
  deliveryTiming?: boolean
  streamStatus?: LogOtherData['stream_status']
  className?: string
}

export function StreamTpsCell(props: StreamTpsCellProps) {
  const { t } = useTranslation()
  const showStreamError =
    props.isStream && props.streamStatus && props.streamStatus.status !== 'ok'
  let tpsLabel = props.deliveryTiming ? t('N/A') : '—'
  if (props.tokensPerSecond != null) {
    const rate = props.deliveryTiming
      ? props.tokensPerSecond.toFixed(1)
      : Math.round(props.tokensPerSecond)
    tpsLabel = `${rate} t/s`
  }
  let streamLabel = props.isStream ? t('Stream') : t('Non-stream')
  if (props.isTask) {
    streamLabel = props.isSyncTask ? t('Sync') : t('Async')
  }

  return (
    <div
      className={cn(
        'flex shrink-0 flex-col items-start justify-center gap-0.5 text-xs leading-tight',
        props.compact && 'flex-row flex-wrap items-center gap-1.5',
        props.className
      )}
    >
      <span
        className={cn(
          'inline-flex items-center gap-1 font-medium',
          props.isStream ? 'text-info' : 'text-muted-foreground'
        )}
      >
        {streamLabel}
        {showStreamError && (
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger
                render={<CircleAlert className='text-destructive size-3' />}
              />
              <TooltipContent>
                <div className='space-y-0.5 text-xs'>
                  <p>
                    {t('Stream Status')}: {t('Error')}
                  </p>
                  <p>{props.streamStatus?.end_reason || 'unknown'}</p>
                  {(props.streamStatus?.error_count ?? 0) > 0 && (
                    <p>
                      {t('Soft Errors')}: {props.streamStatus?.error_count}
                    </p>
                  )}
                </div>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        )}
      </span>
      {(!props.compact ||
        props.deliveryTiming ||
        (props.isStream && props.tokensPerSecond != null)) && (
        <span className='text-muted-foreground/60 px-0.5 tabular-nums'>
          {tpsLabel}
        </span>
      )}
    </div>
  )
}
