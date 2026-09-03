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
import type { ISpec } from '@visactor/vchart'

import type {
  ChartDataPoint,
  EstimatedTpsChartDataPoint,
  EstimatedTpsPhase,
  TokenRecordRecentItem,
} from '../types'

function formatTimeLabel(bucketStartAt: number): string {
  const date = new Date(bucketStartAt * 1000)
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  const hour = `${date.getHours()}`.padStart(2, '0')
  return `${month}-${day} ${hour}:00`
}

export function transformChartData(
  items: TokenRecordRecentItem[],
  selectedModels: Set<string>
): {
  outputTokenData: ChartDataPoint[]
  averageTpsData: ChartDataPoint[]
  failureRateData: ChartDataPoint[]
} {
  const outputTokenData: ChartDataPoint[] = []
  const averageTpsData: ChartDataPoint[] = []
  const failureRateData: ChartDataPoint[] = []

  for (const item of items) {
    if (!selectedModels.has(item.model_name)) continue

    for (const cell of item.cells ?? []) {
      const timeLabel = formatTimeLabel(cell.bucket_start_at)
      const base = { Time: timeLabel, Model: item.model_name }

      outputTokenData.push({ ...base, Value: cell.completion_tokens || 0 })

      averageTpsData.push({
        ...base,
        Value: Number(Number(cell.avg_tps || 0).toFixed(2)),
      })

      const totalRequests = (cell.request_count || 0) + (cell.failed_count || 0)
      const rate =
        totalRequests > 0
          ? Number(((cell.failed_count / totalRequests) * 100).toFixed(2))
          : 0
      failureRateData.push({ ...base, Value: rate })
    }
  }

  return { outputTokenData, averageTpsData, failureRateData }
}

type PhaseLabels = Record<EstimatedTpsPhase, string>

const ESTIMATED_PHASES: Array<{
  phase: EstimatedTpsPhase
  tokensField: 'thinking_tokens' | 'output_tokens' | 'tool_tokens'
  durationField:
    | 'thinking_duration_ms'
    | 'output_duration_ms'
    | 'tool_duration_ms'
}> = [
  {
    phase: 'thinking',
    tokensField: 'thinking_tokens',
    durationField: 'thinking_duration_ms',
  },
  {
    phase: 'output',
    tokensField: 'output_tokens',
    durationField: 'output_duration_ms',
  },
  {
    phase: 'tool',
    tokensField: 'tool_tokens',
    durationField: 'tool_duration_ms',
  },
]

export function transformEstimatedTpsData(
  items: TokenRecordRecentItem[],
  selectedModels: Set<string>,
  labels: PhaseLabels
): EstimatedTpsChartDataPoint[] {
  const buckets = new Map<
    number,
    Record<EstimatedTpsPhase, { tokens: number; durationMs: number }>
  >()

  for (const item of items) {
    if (!selectedModels.has(item.model_name)) continue

    for (const cell of item.cells ?? []) {
      const aggregate = buckets.get(cell.bucket_start_at) ?? {
        thinking: { tokens: 0, durationMs: 0 },
        output: { tokens: 0, durationMs: 0 },
        tool: { tokens: 0, durationMs: 0 },
      }
      for (const { phase, tokensField, durationField } of ESTIMATED_PHASES) {
        aggregate[phase].tokens += Number(cell[tokensField] || 0)
        aggregate[phase].durationMs += Number(cell[durationField] || 0)
      }
      buckets.set(cell.bucket_start_at, aggregate)
    }
  }

  const data: EstimatedTpsChartDataPoint[] = []
  for (const [bucketStartAt, aggregate] of [...buckets.entries()].sort(
    ([left], [right]) => left - right
  )) {
    for (const { phase } of ESTIMATED_PHASES) {
      const { tokens, durationMs } = aggregate[phase]
      if (tokens <= 0 || durationMs <= 0) continue
      data.push({
        Time: formatTimeLabel(bucketStartAt),
        Model: labels[phase],
        Phase: phase,
        Value: Number((tokens / (durationMs / 1000)).toFixed(2)),
      })
    }
  }

  return data
}

export function buildLineChartSpec(
  dataId: string,
  dataValues: ChartDataPoint[],
  colorDomain: string[],
  colorRange: string[]
): ISpec {
  return {
    type: 'line',
    data: [{ id: dataId, values: dataValues }],
    xField: 'Time',
    yField: 'Value',
    seriesField: 'Model',
    legends: { visible: true },
    axes: [
      { orient: 'bottom' },
      {
        orient: 'left',
        nice: true,
        niceType: 'rough',
      },
    ],
    tooltip: {
      mark: {
        content: [
          {
            key: (datum: Record<string, unknown>) => datum['Model'],
            value: (datum: Record<string, unknown>) =>
              typeof datum['Value'] === 'number'
                ? datum['Value'].toLocaleString()
                : datum['Value'],
          },
        ],
      },
    },
    color: {
      type: 'ordinal',
      domain: colorDomain,
      range: colorRange,
    },
    line: {
      style: {
        curveType: 'monotone',
        lineWidth: 2,
      },
    },
    point: {
      visible: true,
      style: {
        size: 4,
        fill: '#fff',
        stroke: null,
        lineWidth: 2,
      },
    },
    crosshair: {
      visible: true,
    },
  } as ISpec
}
