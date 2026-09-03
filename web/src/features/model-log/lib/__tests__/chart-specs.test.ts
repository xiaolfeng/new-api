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

import type { TokenRecordHourCell, TokenRecordRecentItem } from '../../types'
import { transformChartData, transformEstimatedTpsData } from '../chart-specs'

function makeCell(
  bucketStartAt: number,
  values: Partial<TokenRecordHourCell>
): TokenRecordHourCell {
  return {
    bucket_start_at: bucketStartAt,
    bucket_end_at: bucketStartAt + 3599,
    request_count: 1,
    prompt_tokens: 0,
    completion_tokens: 0,
    total_tokens: 0,
    total_use_time: 0,
    avg_tps: 0,
    thinking_tokens: 0,
    thinking_duration_ms: 0,
    avg_thinking_tps: 0,
    output_tokens: 0,
    output_duration_ms: 0,
    avg_output_tps: 0,
    tool_tokens: 0,
    tool_duration_ms: 0,
    avg_tool_tps: 0,
    failed_count: 0,
    failed_detail: {},
    is_current: false,
    ...values,
  }
}

function makeItem(
  modelName: string,
  cells: TokenRecordHourCell[]
): TokenRecordRecentItem {
  return {
    model_name: modelName,
    cells,
    summary: {
      request_count: 0,
      prompt_tokens: 0,
      completion_tokens: 0,
      total_tokens: 0,
      total_use_time: 0,
      avg_tps: 0,
      thinking_tokens: 0,
      thinking_duration_ms: 0,
      avg_thinking_tps: 0,
      output_tokens: 0,
      output_duration_ms: 0,
      avg_output_tps: 0,
      tool_tokens: 0,
      tool_duration_ms: 0,
      avg_tool_tps: 0,
      failed_count: 0,
      failed_rate: 0,
      failed_detail: {},
    },
  }
}

const labels = {
  thinking: 'Thinking',
  output: 'Output',
  tool: 'Tool',
}

describe('model log TPS chart data', () => {
  it('keeps the average TPS trend grouped by model', () => {
    const items = [
      makeItem('model-a', [
        makeCell(1711454400, { completion_tokens: 40, avg_tps: 12.5 }),
      ]),
      makeItem('model-b', [
        makeCell(1711454400, { completion_tokens: 20, avg_tps: 7.25 }),
      ]),
    ]

    const result = transformChartData(items, new Set(['model-a', 'model-b']))

    expect(result.averageTpsData).toEqual([
      expect.objectContaining({ Model: 'model-a', Value: 12.5 }),
      expect.objectContaining({ Model: 'model-b', Value: 7.25 }),
    ])
  })

  it('weights estimated phase TPS by tokens and duration across selected models', () => {
    const items = [
      makeItem('model-a', [
        makeCell(1711454400, {
          thinking_tokens: 10,
          thinking_duration_ms: 1000,
          output_tokens: 20,
          output_duration_ms: 1000,
          tool_tokens: 6,
          tool_duration_ms: 300,
        }),
      ]),
      makeItem('model-b', [
        makeCell(1711454400, {
          thinking_tokens: 20,
          thinking_duration_ms: 1000,
          output_tokens: 10,
          output_duration_ms: 2000,
        }),
      ]),
    ]

    const result = transformEstimatedTpsData(
      items,
      new Set(['model-a', 'model-b']),
      labels
    )

    expect(result).toEqual([
      expect.objectContaining({ Phase: 'thinking', Value: 15 }),
      expect.objectContaining({ Phase: 'output', Value: 10 }),
      expect.objectContaining({ Phase: 'tool', Value: 20 }),
    ])
  })

  it('omits phases that have no estimated tokens or duration', () => {
    const items = [
      makeItem('model-a', [
        makeCell(1711454400, {
          output_tokens: 20,
          output_duration_ms: 1000,
        }),
      ]),
    ]

    const result = transformEstimatedTpsData(
      items,
      new Set(['model-a']),
      labels
    )

    expect(result).toHaveLength(1)
    expect(result[0]).toEqual(expect.objectContaining({ Phase: 'output' }))
  })
})
