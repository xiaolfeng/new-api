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
export interface ToolLog {
  id: number
  created_at: number
  user_id: number
  username: string
  token_id: number
  token_name: string
  channel: number
  channel_name?: string
  group: string
  model_name: string
  request_id: string
  ip: string
  original_name: string
  canonical: string
  kind: string
  mode: string
  backend: string
  query: string
  url: string
  error_code: string
  duration_ms: number
  truncated: boolean
  result: string
}

export interface ToolLogFilters {
  startTime?: Date
  endTime?: Date
  kind?: string
  error?: string
  model?: string
  token?: string
  username?: string
  channel?: string
  requestId?: string
  q?: string
}

export interface GetToolLogsResponse {
  success: boolean
  message?: string
  data?: {
    items: ToolLog[]
    total: number
    page: number
    page_size: number
  }
}
