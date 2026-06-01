import type { ISpec } from '@visactor/vchart'
import type { TokenRecordRecentItem, ChartDataPoint } from '../types'

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
  tpsData: ChartDataPoint[]
  failureRateData: ChartDataPoint[]
} {
  const outputTokenData: ChartDataPoint[] = []
  const tpsData: ChartDataPoint[] = []
  const failureRateData: ChartDataPoint[] = []

  for (const item of items) {
    if (!selectedModels.has(item.model_name)) continue

    for (const cell of item.cells ?? []) {
      const timeLabel = formatTimeLabel(cell.bucket_start_at)
      const base = { Time: timeLabel, Model: item.model_name }

      outputTokenData.push({ ...base, Value: cell.completion_tokens || 0 })

      tpsData.push({
        ...base,
        Value: Number(Number(cell.avg_tps || 0).toFixed(2)),
      })

      const totalRequests =
        (cell.request_count || 0) + (cell.failed_count || 0)
      const rate =
        totalRequests > 0
          ? Number(((cell.failed_count / totalRequests) * 100).toFixed(2))
          : 0
      failureRateData.push({ ...base, Value: rate })
    }
  }

  return { outputTokenData, tpsData, failureRateData }
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
