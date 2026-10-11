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

import { formatNumber, formatTokenUnits } from '@/lib/format'

import type {
  ChartDataPoint,
  EstimatedTpsChartDataPoint,
  EstimatedTpsPhase,
  SortField,
  TokenRecordHourCell,
  TokenRecordRecentItem,
} from '../types'
import { getPhaseColorVariant, getPhaseLineDash } from './chart-colors'

function formatTimeLabel(bucketStartAt: number): string {
  const date = new Date(bucketStartAt * 1000)
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  const hour = `${date.getHours()}`.padStart(2, '0')
  return `${month}-${day} ${hour}:00`
}

export function hasVerifiedAvgTps(
  avgTps: number | null | undefined,
  sampleCount?: number
): avgTps is number {
  if (typeof avgTps !== 'number' || !Number.isFinite(avgTps) || avgTps <= 0) {
    return false
  }
  if (sampleCount !== undefined && sampleCount <= 0) {
    return false
  }
  return true
}

export function formatModelTpsSummary(
  avgTps: number | null | undefined,
  sampleCount?: number
): string {
  if (!hasVerifiedAvgTps(avgTps, sampleCount)) {
    return 'N/A'
  }
  return `${Number(avgTps.toFixed(2))}`
}

export function sortModelLogItems(
  items: TokenRecordRecentItem[],
  sortField: SortField,
  sortDirection: 'asc' | 'desc'
): TokenRecordRecentItem[] {
  const sorted = [...items]
  const multiplier = sortDirection === 'asc' ? 1 : -1

  sorted.sort((a, b) => {
    let aVal: number, bVal: number

    switch (sortField) {
      case 'total_tokens':
        aVal = a.summary.completion_tokens || 0
        bVal = b.summary.completion_tokens || 0
        break
      case 'failed_rate':
        aVal = a.summary.failed_rate || 0
        bVal = b.summary.failed_rate || 0
        break
      case 'avg_tps': {
        const aAvg = a.summary.avg_tps
        const bAvg = b.summary.avg_tps
        const aHasTps = hasVerifiedAvgTps(aAvg, a.summary.tps_sample_count)
        const bHasTps = hasVerifiedAvgTps(bAvg, b.summary.tps_sample_count)
        if (!aHasTps && !bHasTps) {
          return a.model_name.localeCompare(b.model_name)
        }
        if (!aHasTps) return 1
        if (!bHasTps) return -1
        aVal = aAvg
        bVal = bAvg
        break
      }
      default:
        return 0
    }

    if (aVal === bVal) {
      return a.model_name.localeCompare(b.model_name) * multiplier
    }
    return (aVal - bVal) * multiplier
  })

  return sorted
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

      const avgTps = cell.avg_tps
      const validTps = hasVerifiedAvgTps(avgTps, cell.tps_sample_count)
      averageTpsData.push({
        ...base,
        Value: validTps ? Number(avgTps.toFixed(2)) : null,
        SampleCount: validTps ? (cell.tps_sample_count ?? 1) : 0,
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
  avgField: 'avg_thinking_tps' | 'avg_output_tps' | 'avg_tool_tps'
  sampleCountField:
    | 'thinking_sample_count'
    | 'output_sample_count'
    | 'tool_sample_count'
}> = [
  {
    phase: 'thinking',
    avgField: 'avg_thinking_tps',
    sampleCountField: 'thinking_sample_count',
  },
  {
    phase: 'output',
    avgField: 'avg_output_tps',
    sampleCountField: 'output_sample_count',
  },
  {
    phase: 'tool',
    avgField: 'avg_tool_tps',
    sampleCountField: 'tool_sample_count',
  },
]

function resolveCellPhaseTps(
  cell: TokenRecordHourCell,
  avgField: 'avg_thinking_tps' | 'avg_output_tps' | 'avg_tool_tps',
  sampleCountField:
    | 'thinking_sample_count'
    | 'output_sample_count'
    | 'tool_sample_count'
): { value: number | null; sampleCount: number } {
  const rawAvg = cell[avgField]
  const rawSampleCount = cell[sampleCountField]
  if (hasVerifiedAvgTps(rawAvg, rawSampleCount)) {
    return {
      value: Number(rawAvg.toFixed(2)),
      sampleCount: rawSampleCount ?? 1,
    }
  }
  return { value: null, sampleCount: 0 }
}

export function buildEstimatedSeriesKey(
  modelName: string,
  phaseLabel: string
): string {
  return `${modelName} (${phaseLabel})`
}

export function transformEstimatedTpsData(
  items: TokenRecordRecentItem[],
  selectedModels: Set<string>,
  labels: PhaseLabels
): EstimatedTpsChartDataPoint[] {
  const data: EstimatedTpsChartDataPoint[] = []

  for (const item of items) {
    if (!selectedModels.has(item.model_name)) continue
    const cells = [...(item.cells ?? [])].sort(
      (left, right) => left.bucket_start_at - right.bucket_start_at
    )
    if (cells.length === 0) continue

    for (const { phase, avgField, sampleCountField } of ESTIMATED_PHASES) {
      const resolvedCells = cells.map((cell) => ({
        bucketStartAt: cell.bucket_start_at,
        ...resolveCellPhaseTps(cell, avgField, sampleCountField),
      }))
      const hasAnyValidSample = resolvedCells.some((c) => c.value !== null)
      if (!hasAnyValidSample) continue

      const phaseLabel = labels[phase]
      const seriesKey = buildEstimatedSeriesKey(item.model_name, phaseLabel)

      for (const cellPoint of resolvedCells) {
        data.push({
          Time: formatTimeLabel(cellPoint.bucketStartAt),
          Model: item.model_name,
          Phase: phase,
          PhaseLabel: phaseLabel,
          SeriesKey: seriesKey,
          Value: cellPoint.value,
          SampleCount: cellPoint.sampleCount,
        })
      }
    }
  }

  return data
}

export interface LineChartSpecOptions {
  valueFormat?: 'tokens' | 'tps' | 'percent' | 'number'
  locale?: Intl.LocalesArgument
  isDark?: boolean
}

function formatTooltipMetricValue(
  raw: unknown,
  options?: LineChartSpecOptions
): string {
  if (typeof raw !== 'number' || !Number.isFinite(raw)) {
    return 'N/A'
  }
  const format = options?.valueFormat ?? 'number'
  if (format === 'tokens') {
    const unitText = formatTokenUnits(raw)
    const exactText = formatNumber(raw, options?.locale)
    return raw >= 1000 ? `${unitText} (${exactText})` : exactText
  }
  if (format === 'percent') {
    return `${formatNumber(raw, options?.locale)}%`
  }
  if (format === 'tps') {
    return `${formatNumber(raw, options?.locale)} tok/s`
  }
  return formatNumber(raw, options?.locale)
}

export function buildLineChartSpec(
  dataId: string,
  dataValues: ChartDataPoint[],
  colorDomain: string[],
  colorRange: string[],
  options?: LineChartSpecOptions
): ISpec {
  const isTokens = options?.valueFormat === 'tokens'
  const pointFill = options?.isDark ? '#0f172a' : '#ffffff'

  return {
    type: 'line',
    data: [{ id: dataId, values: dataValues }],
    xField: 'Time',
    yField: 'Value',
    seriesField: 'Model',
    invalidType: 'break',
    legends: { visible: true },
    axes: [
      { orient: 'bottom' },
      {
        orient: 'left',
        nice: true,
        niceType: 'rough',
        ...(isTokens
          ? {
              label: {
                formatMethod: (val: unknown) => formatTokenUnits(Number(val)),
              },
            }
          : {}),
      },
    ],
    tooltip: {
      mark: {
        content: [
          {
            key: (datum: Record<string, unknown>) => datum['Model'],
            value: (datum: Record<string, unknown>) =>
              formatTooltipMetricValue(datum['Value'], options),
          },
        ],
      },
      dimension: {
        content: [
          {
            key: (datum: Record<string, unknown>) => datum['Model'],
            value: (datum: Record<string, unknown>) =>
              formatTooltipMetricValue(datum['Value'], options),
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
        fill: pointFill,
        stroke: null,
        lineWidth: 2,
      },
    },
    crosshair: {
      visible: true,
    },
  } as ISpec
}

export function buildEstimatedTpsChartSpec(
  dataId: string,
  dataValues: EstimatedTpsChartDataPoint[],
  modelColorMap: ReadonlyMap<string, string>,
  options?: { locale?: Intl.LocalesArgument; isDark?: boolean }
): ISpec {
  const seriesMeta = new Map<
    string,
    { model: string; phase: EstimatedTpsPhase }
  >()
  for (const point of dataValues) {
    if (!seriesMeta.has(point.SeriesKey)) {
      seriesMeta.set(point.SeriesKey, {
        model: point.Model,
        phase: point.Phase,
      })
    }
  }

  const colorDomain: string[] = []
  const colorRange: string[] = []
  for (const [seriesKey, meta] of seriesMeta.entries()) {
    colorDomain.push(seriesKey)
    const baseColor = modelColorMap.get(meta.model) ?? '#0ea5e9'
    colorRange.push(getPhaseColorVariant(baseColor, meta.phase))
  }

  const pointFill = options?.isDark ? '#0f172a' : '#ffffff'

  return {
    type: 'line',
    data: [{ id: dataId, values: dataValues }],
    xField: 'Time',
    yField: 'Value',
    seriesField: 'SeriesKey',
    invalidType: 'break',
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
            key: (datum: Record<string, unknown>) =>
              (datum['SeriesKey'] as string) ||
              `${datum['Model']} (${datum['PhaseLabel'] ?? datum['Phase']})`,
            value: (datum: Record<string, unknown>) =>
              formatTooltipMetricValue(datum['Value'], {
                valueFormat: 'tps',
                locale: options?.locale,
              }),
          },
        ],
      },
      dimension: {
        content: [
          {
            key: (datum: Record<string, unknown>) =>
              (datum['SeriesKey'] as string) ||
              `${datum['Model']} (${datum['PhaseLabel'] ?? datum['Phase']})`,
            value: (datum: Record<string, unknown>) =>
              formatTooltipMetricValue(datum['Value'], {
                valueFormat: 'tps',
                locale: options?.locale,
              }),
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
        lineDash: (datum: Record<string, unknown>) =>
          getPhaseLineDash((datum['Phase'] as EstimatedTpsPhase) ?? 'output'),
      },
    },
    point: {
      visible: true,
      style: {
        size: 4,
        fill: pointFill,
        stroke: null,
        lineWidth: 2,
      },
    },
    crosshair: {
      visible: true,
    },
  } as ISpec
}
