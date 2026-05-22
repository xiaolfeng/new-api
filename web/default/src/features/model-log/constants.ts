import type { SortField, ChartTab } from './types'

export const SORT_OPTIONS: { value: SortField; labelKey: string }[] = [
  { value: 'total_tokens', labelKey: 'Output Tokens' },
  { value: 'failed_rate', labelKey: 'Failure Rate' },
  { value: 'avg_tps', labelKey: 'Output TPS' },
]

export const CHART_TABS: { value: ChartTab; labelKey: string }[] = [
  { value: 'output_tokens', labelKey: 'Output Token Trend' },
  { value: 'tps', labelKey: 'Output TPS Trend' },
  { value: 'failure_rate', labelKey: 'Failure Rate Trend' },
]
