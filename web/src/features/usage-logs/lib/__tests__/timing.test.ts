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
import { describe, expect, it } from 'vitest'

import type { DeliveryTiming } from '../../types'
import { parseLogOther } from '../format'
import { getLogTiming } from '../timing'

const log = { use_time: 7, is_stream: true, completion_tokens: 50 }
const delivery: DeliveryTiming = {
  version: 1,
  source: 'server_delivery',
  total_ms: 7109,
  ttft_ms: 7079,
  status: 'completed',
}

describe('delivery timing extraction and compatibility', () => {
  it('prefers delivery milliseconds without clamping to the old whole-second duration', () => {
    const metrics = getLogTiming(log, {
      delivery_timing: delivery,
      frt: 9999,
      tps: 800,
      bamboo_timing: { total_ms: 1000, ttft_ms: 500, output_tps: 300 },
    })
    expect(metrics.totalLabel).toBe('7.109s')
    expect(metrics.firstLabel).toBe('7.079s')
    if (metrics.firstMs === null || metrics.totalMs === null) {
      throw new Error('Expected valid delivery timing')
    }
    expect(metrics.firstMs).toBeLessThanOrEqual(metrics.totalMs)
    expect(metrics.legacy).toBe(false)
    expect(metrics.tokensPerSecond).toBeNull()
  })

  it('does not clamp or backfill legacy records', () => {
    const metrics = getLogTiming(log, { frt: 7079 })
    expect(metrics.legacy).toBe(true)
    expect(metrics.totalLabel).toBe('7.0s')
    expect(metrics.firstLabel).toBe('7.1s')
    expect(metrics.firstMs).toBe(7079)
    expect(metrics.tokensPerSecond).toBeNull()
  })

  it('preserves legacy desktop stored TPS and mobile averages', () => {
    const metrics = getLogTiming(log, { tps: 81.5, frt: 1000 })
    expect(metrics.averageTps).toBe(50 / 6)
    expect(metrics.tokensPerSecond).toBe(81.5)
    expect(metrics.mobileTokensPerSecond).toBe(50 / 7)
  })

  it.each([0, null])(
    'distinguishes TTFT %s without falling back to frt',
    (ttft) => {
      const metrics = getLogTiming(log, {
        delivery_timing: { ...delivery, ttft_ms: ttft },
        frt: 300,
      })
      expect(metrics.firstMs).toBe(ttft)
      expect(metrics.firstLabel).toBe(ttft === null ? null : '0.000s')
    }
  )

  it('keeps a zero total duration valid but TPS unavailable', () => {
    const metrics = getLogTiming(log, {
      delivery_timing: { ...delivery, total_ms: 0, ttft_ms: 0 },
    })
    expect(metrics.legacy).toBe(false)
    expect(metrics.totalLabel).toBe('0.000s')
    expect(metrics.firstLabel).toBe('0.000s')
    expect(metrics.tokensPerSecond).toBeNull()
  })

  it.each([
    null,
    {},
    { ...delivery, version: 2 },
    { ...delivery, source: 'sdk' },
    { ...delivery, status: 'unknown' },
    { ...delivery, total_ms: -1 },
    { ...delivery, total_ms: '7109' },
    { ...delivery, ttft_ms: -1 },
    { ...delivery, ttft_ms: 7110 },
    { ...delivery, ttft_ms: undefined },
  ])('falls back for malformed or unknown schema: %j', (value) => {
    const metrics = getLogTiming(
      log,
      parseLogOther(
        JSON.stringify({
          delivery_timing: value,
          frt: 7079,
        })
      )
    )
    expect(metrics.legacy).toBe(true)
    expect(metrics.totalLabel).toBe('7.0s')
    expect(metrics.firstLabel).toBe('7.1s')
  })

  it.each([undefined, delivery])(
    'never falls back for an invalid delivery marker (%j)',
    (timing) => {
      const metrics = getLogTiming(log, {
        delivery_timing_invalid: true,
        delivery_timing: timing,
        frt: 1,
        tps: 100,
        bamboo_timing: { total_ms: 1000, ttft_ms: 1, output_tokens: 100 },
      })
      expect(metrics.legacy).toBe(false)
      expect(metrics.totalMs).toBeNull()
      expect(metrics.firstMs).toBeNull()
      expect(metrics.totalLabel).toBeNull()
      expect(metrics.firstLabel).toBeNull()
      expect(metrics.tokensPerSecond).toBeNull()
      expect(metrics.mobileTokensPerSecond).toBeNull()
    }
  )

  it('does not mistake a false marker for an invariant failure', () => {
    expect(getLogTiming(log, { delivery_timing_invalid: false }).legacy).toBe(
      true
    )
  })
})

describe('delivery average TPS reliability', () => {
  it.each([
    [true, 7100, 7000, 50, 500],
    [true, 7100, 7001, 50, null],
    [true, 7100, 7100, 50, null],
    [true, 7100, null, 50, null],
    [true, 1000, 0, 50, 50],
    [false, 2000, 1900, 50, 25],
    [false, 1, 0, 50, 50000],
    [false, 2000, null, 50, null],
    [false, 0, 0, 50, null],
    [true, 1000, 0, 0, null],
    [true, 1000, 0, -1, null],
    [true, 1000, 0, Number.NaN, null],
  ])(
    'stream=%s total=%s first=%s tokens=%s => %s',
    (stream, total, first, tokens, expected) => {
      const metrics = getLogTiming(
        { ...log, is_stream: stream, completion_tokens: tokens },
        {
          delivery_timing: { ...delivery, total_ms: total, ttft_ms: first },
          tps: 10000,
          bamboo_timing: { output_tokens: 1000, output_tps: 999 },
        }
      )
      expect(metrics.averageTps).toBe(expected)
      expect(metrics.tokensPerSecond).toBe(expected)
      expect(metrics.mobileTokensPerSecond).toBe(expected)
    }
  )
})
