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
import { useTranslation } from 'react-i18next'

import { formatNumber } from '@/lib/format'

import { formatTimingMs, getLogTiming, type TimingLog } from '../../lib/timing'
import type { DeliveryTiming, LogOtherData, UpstreamTiming } from '../../types'
import { TimingMetricsCell } from '../timing-metrics-cell'
import { DetailRow, DetailSection } from './log-detail-layout'

const deliveryStatusLabels: Record<DeliveryTiming['status'], string> = {
  completed: 'Completed',
  cancelled: 'Cancelled',
  write_error: 'Write error',
  upstream_error: 'Upstream error',
}

export function LogTimingDetails(props: {
  log: TimingLog
  other: LogOtherData | null
}) {
  const { t } = useTranslation()
  const metrics = getLogTiming(props.log, props.other)
  const rawHops = props.other?.bamboo_timing_hops
  const hops = Array.isArray(rawHops)
    ? rawHops.filter(
        (hop) => hop && Number.isInteger(hop.hop_index) && hop.hop_index > 0
      )
    : []
  const singleTiming = props.other?.bamboo_timing

  return (
    <>
      <DetailRow
        label={metrics.legacy ? t('Response Time') : t('Service-side delivery')}
        value={
          <TimingMetricsCell
            log={props.log}
            other={props.other}
            indicator='dot'
          />
        }
      />
      {!metrics.legacy && (
        <>
          {metrics.delivery && (
            <DetailRow
              label={t('Delivery status')}
              value={t(deliveryStatusLabels[metrics.delivery.status])}
            />
          )}
          <DetailRow
            label={t('TPS')}
            value={
              metrics.tokensPerSecond === null
                ? t('N/A')
                : `${metrics.tokensPerSecond.toFixed(1)} t/s`
            }
            mono
          />
        </>
      )}
      {(hops.length > 1 || props.other?.bamboo_timing) && (
        <DetailSection label={t('Upstream timing')}>
          {hops.length > 1
            ? hops.map((hop) => (
                <div
                  key={hop.hop_index}
                  className='space-y-1 not-first:border-t not-first:pt-2'
                >
                  <p className='text-muted-foreground text-xs font-medium'>
                    {t('Upstream hop {{index}}', { index: hop.hop_index })}
                  </p>
                  <UpstreamTimingRows timing={hop} />
                </div>
              ))
            : singleTiming && (
                <div className='space-y-1'>
                  <p className='text-muted-foreground text-xs font-medium'>
                    {t('Last upstream hop')}
                  </p>
                  <UpstreamTimingRows timing={singleTiming} />
                </div>
              )}
        </DetailSection>
      )}
    </>
  )
}

function UpstreamTimingRows(props: { timing: UpstreamTiming }) {
  const { t } = useTranslation()
  const durations: [string, number | undefined][] = [
    ['Duration', props.timing.total_ms],
    ['First token', props.timing.ttft_ms],
    ['Thinking', props.timing.thinking_ms],
    ['Output', props.timing.content_ms],
    ['Tool', props.timing.tool_ms],
  ]
  const rates: [string, number | undefined][] = [
    ['Thinking TPS', props.timing.thinking_tps],
    ['Output TPS', props.timing.output_tps],
    ['Tool TPS', props.timing.tool_tps],
  ]
  const tokens: [string, number | undefined][] = [
    ['Thinking Tokens', props.timing.thinking_tokens],
    ['Output Tokens', props.timing.output_tokens],
    ['Tool Tokens', props.timing.tool_tokens],
  ]
  return (
    <>
      {durations.map(([label, ms]) => (
        <DetailRow
          key={label}
          label={t(label)}
          value={
            typeof ms === 'number' && Number.isFinite(ms) && ms >= 0
              ? formatTimingMs(ms)
              : t('N/A')
          }
          mono
        />
      ))}
      {rates.map(([label, rate]) => (
        <DetailRow
          key={label}
          label={t(label)}
          value={
            typeof rate === 'number' && Number.isFinite(rate)
              ? `${rate < 0 ? '~' : ''}${Math.abs(rate).toFixed(1)} t/s`
              : t('N/A')
          }
          mono
        />
      ))}
      {tokens.map(([label, count]) => (
        <DetailRow
          key={label}
          label={t(label)}
          value={
            typeof count === 'number' && Number.isFinite(count) && count >= 0
              ? formatNumber(count)
              : t('N/A')
          }
          mono
        />
      ))}
    </>
  )
}
