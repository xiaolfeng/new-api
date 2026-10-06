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

You should have received a copy of the GNU Affero General Public
License along with this program. If not, see <https://www.gnu.org/licenses/>.
For commercial licensing, please contact support@quantumnous.com
*/
import { describe, expect, test } from 'vitest'

import dayjs from '@/lib/dayjs'

import type { QuotaDataItem } from '../../types'
import { processChartData } from '../charts'

function item(date: string, quota: number): QuotaDataItem {
  return {
    model_name: 'gpt-5',
    created_at: dayjs(date).unix(),
    quota,
    count: 1,
    token_used: 100,
  }
}

describe('processChartData sparse time buckets', () => {
  test('keeps every existing sparse bucket when padding to the minimum', () => {
    const processed = processChartData(
      [item('2026-08-01', 100), item('2026-08-20', 50)],
      'day'
    )

    const barValues = processed.spec_line.data[0].values as Array<{
      Time: string
    }>
    const times = new Set(barValues.map((v) => v.Time))
    // 补点只能新增空桶：两个稀疏数据点都必须出现在趋势轴上，
    // 不能被“最后数据点前的连续 7 桶”窗口丢弃。
    expect(times.has('08-01')).toBe(true)
    expect(times.has('08-20')).toBe(true)

    const areaValues = processed.spec_area.data[0].values as Array<{
      Time: string
    }>
    const areaTimes = new Set(areaValues.map((v) => v.Time))
    expect(areaTimes.has('08-01')).toBe(true)
    expect(areaTimes.has('08-20')).toBe(true)
  })
})
