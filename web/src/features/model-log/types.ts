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
export interface TokenRecordHourCell {
  bucket_start_at: number
  bucket_end_at: number
  request_count: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  total_use_time: number
  avg_tps: number
  thinking_tokens: number
  thinking_duration_ms: number
  avg_thinking_tps: number
  output_tokens: number
  output_duration_ms: number
  avg_output_tps: number
  tool_tokens: number
  tool_duration_ms: number
  avg_tool_tps: number
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
  thinking_tokens: number
  thinking_duration_ms: number
  avg_thinking_tps: number
  output_tokens: number
  output_duration_ms: number
  avg_output_tps: number
  tool_tokens: number
  tool_duration_ms: number
  avg_tool_tps: number
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

export type EstimatedTpsPhase = 'thinking' | 'output' | 'tool'

export interface EstimatedTpsChartDataPoint extends ChartDataPoint {
  Phase: EstimatedTpsPhase
}

export interface TokenRecordDailyItem {
  date: string
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
}
