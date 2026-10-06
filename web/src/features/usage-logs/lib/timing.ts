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
import { formatUseTime } from '@/lib/format'

import type { UsageLog } from '../data/schema'
import type { DeliveryTiming, LogOtherData } from '../types'

export const LEGACY_TIMING_NOTE =
  'Legacy precision: duration is in whole seconds'

export type TimingLog = Pick<
  UsageLog,
  'use_time' | 'completion_tokens' | 'is_stream'
>

function isNonnegativeNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0
}

function isDeliveryTiming(value: unknown): value is DeliveryTiming {
  if (!value || typeof value !== 'object') return false
  const timing = value as Record<string, unknown>
  return (
    timing.version === 1 &&
    timing.source === 'server_delivery' &&
    isNonnegativeNumber(timing.total_ms) &&
    (timing.ttft_ms === null ||
      (isNonnegativeNumber(timing.ttft_ms) &&
        timing.ttft_ms <= timing.total_ms)) &&
    typeof timing.status === 'string' &&
    ['completed', 'cancelled', 'write_error', 'upstream_error'].includes(
      timing.status
    )
  )
}

export function formatTimingMs(
  ms: number | null,
  legacy = false
): string | null {
  if (ms === null) return null
  return legacy ? formatUseTime(ms / 1000) : `${(ms / 1000).toFixed(3)}s`
}

export function getLogTiming(log: TimingLog, other?: LogOtherData | null) {
  const invalid = other?.delivery_timing_invalid === true
  const delivery =
    !invalid && isDeliveryTiming(other?.delivery_timing)
      ? other.delivery_timing
      : null
  const legacy = delivery === null && !invalid
  let totalMs: number | null = null
  if (delivery) totalMs = delivery.total_ms
  else if (legacy) totalMs = log.use_time * 1000
  let firstMs: number | null = null
  if (delivery) {
    firstMs = delivery.ttft_ms
  } else if (legacy && typeof other?.frt === 'number' && other.frt > 0) {
    firstMs = other.frt
  }

  let averageTps: number | null = null
  if (delivery) {
    let generationMs: number | null = delivery.total_ms
    if (log.is_stream) {
      generationMs = firstMs === null ? null : delivery.total_ms - firstMs
    }
    if (
      generationMs !== null &&
      firstMs !== null &&
      generationMs > 0 &&
      (!log.is_stream || generationMs >= 100) &&
      Number.isFinite(log.completion_tokens) &&
      log.completion_tokens > 0
    ) {
      const rate = log.completion_tokens / (generationMs / 1000)
      averageTps = Number.isFinite(rate) ? rate : null
    }
  } else if (legacy) {
    // 保留旧记录的桌面 TPS 算法，不用上游数据补齐新交付指标。
    const bt = other?.bamboo_timing
    const ttftMs = bt?.ttft_ms && bt.ttft_ms > 0 ? bt.ttft_ms : firstMs
    const tokens =
      log.completion_tokens > 0
        ? log.completion_tokens
        : (bt?.output_tokens ?? 0)
    const seconds =
      log.is_stream && ttftMs && ttftMs > 0
        ? log.use_time - ttftMs / 1000
        : log.use_time
    if (seconds > 0 && tokens > 0) averageTps = tokens / seconds
  }

  const tokensPerSecond =
    legacy && typeof other?.tps === 'number' && other.tps > 0
      ? other.tps
      : averageTps
  // 移动端旧记录原本按整个请求耗时计算，保持历史数值不变。
  let mobileTokensPerSecond = tokensPerSecond
  if (legacy) {
    mobileTokensPerSecond =
      log.use_time > 0 && log.completion_tokens > 0
        ? log.completion_tokens / log.use_time
        : null
  }
  const bt = other?.bamboo_timing

  return {
    legacy,
    delivery,
    totalMs,
    firstMs,
    totalLabel: formatTimingMs(totalMs, legacy),
    firstLabel: formatTimingMs(firstMs, legacy),
    averageTps,
    tokensPerSecond: other?.host_tool_internal ? null : tokensPerSecond,
    mobileTokensPerSecond: other?.host_tool_internal
      ? null
      : mobileTokensPerSecond,
    phaseTiming: bt
      ? {
          thinkingMs: bt.thinking_ms,
          contentMs: bt.content_ms,
          toolMs: bt.tool_ms,
        }
      : undefined,
  }
}
