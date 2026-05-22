export interface TokenRecordHourCell {
  bucket_start_at: number
  bucket_end_at: number
  request_count: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  total_use_time: number
  avg_tps: number
  failed_count: number
  failed_detail: Record<string, number>
  is_current: boolean
}

export interface TokenRecordSummary {
  request_count: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  total_use_time: number
  avg_tps: number
  failed_count: number
  failed_rate: number
  failed_detail: Record<string, number>
}

export interface TokenRecordRecentItem {
  model_name: string
  summary: TokenRecordSummary
  cells: TokenRecordHourCell[]
}

export interface TokenRecordOverallSummary {
  total_request_count: number
  total_prompt_tokens: number
  total_output_tokens: number
  active_model_count: number
}

export interface TokenRecordRecentSnapshot {
  hours: { bucket_start_at: number; label: string }[]
  items: TokenRecordRecentItem[]
  summary: TokenRecordOverallSummary
}

export type SortField = 'total_tokens' | 'failed_rate' | 'avg_tps'

export type ChartTab = 'output_tokens' | 'tps' | 'failure_rate'

export interface ChartDataPoint {
  Time: string
  Model: string
  Value: number
}
