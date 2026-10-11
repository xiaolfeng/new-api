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
/**
 * Utility functions for usage logs feature
 */
import {
  getAllLogs,
  getUserLogs,
  getAllMidjourneyLogs,
  getUserMidjourneyLogs,
  getAllTaskLogs,
  getUserTaskLogs,
} from '../api'
import {
  LOG_TYPES,
  DISPLAYABLE_LOG_TYPES,
  TIMING_LOG_TYPES,
} from '../constants'
import type { UsageLog } from '../data/schema'
import type {
  GetLogsParams,
  GetLogsResponse,
  FetchLogsConfig,
  GetMidjourneyLogsParams,
  GetTaskLogsParams,
} from '../types'

export { buildQueryParams } from './query-params'

// 统计查询与分页无关（请求固定 page=1/pageSize=1），缓存键必须剔除分页参数，
// 否则每次翻页都会重复请求同一份统计。
export function buildLogStatsQueryKey<
  T extends { page?: number; pageSize?: number; [key: string]: unknown },
>(searchParams: T, isAdmin: boolean) {
  const { page: _page, pageSize: _pageSize, ...filters } = searchParams
  return ['usage-logs-stats', isAdmin, filters] as const
}

export type LogRowIdentityRecord = {
  row_key?: string
  log_entry_id?: string
  request_id?: string
  id?: number
  type?: number
  created_at?: number
  channel?: number
  model_name?: string
  token_name?: string
  quota?: number
  prompt_tokens?: number
  completion_tokens?: number
  use_time?: number
  is_stream?: boolean
  upstream_request_id?: string
  content?: string
  task_id?: string
  mj_id?: string
}

function hasImmutableCommonLogFields(row: LogRowIdentityRecord): boolean {
  return (
    row.type != null ||
    row.created_at != null ||
    row.channel != null ||
    Boolean(row.model_name) ||
    Boolean(row.token_name) ||
    row.quota != null ||
    row.prompt_tokens != null ||
    row.completion_tokens != null ||
    row.use_time != null ||
    row.is_stream != null ||
    Boolean(row.upstream_request_id) ||
    Boolean(row.content)
  )
}

function buildLegacyClientLogRowKey(row: LogRowIdentityRecord): string {
  const parts = [
    row.request_id ? `req=${row.request_id}` : '',
    row.type != null ? `t=${row.type}` : '',
    row.created_at != null ? `ts=${row.created_at}` : '',
    row.channel != null ? `ch=${row.channel}` : '',
    row.model_name ? `m=${row.model_name}` : '',
    row.token_name ? `tokn=${row.token_name}` : '',
    row.quota != null ? `q=${row.quota}` : '',
    row.prompt_tokens != null || row.completion_tokens != null
      ? `tok=${row.prompt_tokens ?? 0}/${row.completion_tokens ?? 0}`
      : '',
    row.use_time != null ? `ut=${row.use_time}` : '',
    row.is_stream != null ? `st=${row.is_stream ? 1 : 0}` : '',
    row.upstream_request_id ? `up=${row.upstream_request_id}` : '',
    row.content ? `c=${row.content}` : '',
  ].filter(Boolean)
  return `legacy-client:${parts.join('|')}`
}

// getLogRowId 行的稳定身份：
// 1. 优先使用服务端下发的独立事件标识 row_key / log_entry_id（避免同 request_id 重试错误与成功日志冲突）；
// 2. 兼容旧后端响应：当无 row_key 时，按 request_id、type、channel、created_at 等不可变字段组合身份；
// 3. 仅在无业务字段的极简行回退至 id 或 index。
export function getLogRowId<T extends LogRowIdentityRecord>(
  row: T,
  index: number
): string {
  if (row.row_key) return row.row_key
  if (row.log_entry_id) return row.log_entry_id
  if (row.task_id) {
    return row.id != null
      ? `task:${row.task_id}:${row.id}`
      : `task:${row.task_id}`
  }
  if (row.mj_id) {
    return row.id != null ? `mj:${row.mj_id}:${row.id}` : `mj:${row.mj_id}`
  }
  if (hasImmutableCommonLogFields(row)) {
    return buildLegacyClientLogRowKey(row)
  }
  if (row.request_id) {
    return row.id != null
      ? `req:${row.request_id}:id:${row.id}`
      : row.request_id
  }
  if (row.id) return String(row.id)
  return String(index)
}

// ensureUniqueLogRowKeys 对列表数据补齐并去重 row_key，防止旧后端或同秒同渠道完全相同重试行产生重复 React key。
export function ensureUniqueLogRowKeys<T extends LogRowIdentityRecord>(
  rows: readonly T[]
): T[] {
  if (!rows.length) return []
  const seen = new Map<string, number>()
  return rows.map((row, index) => {
    const baseKey = getLogRowId(row, index)
    const count = (seen.get(baseKey) ?? 0) + 1
    seen.set(baseKey, count)
    const uniqueKey = count > 1 ? `${baseKey}#${count}` : baseKey
    if (row.row_key === uniqueKey) {
      return row
    }
    return {
      ...row,
      row_key: uniqueKey,
    }
  })
}

// getServerLogRowKey 仅提取可作为服务端精确查询凭据的 row_key（新日志 log_entry_id 或关系库 legacy-id），
// 排除客户端或 ClickHouse 合成的历史指纹 key。
export function getServerLogRowKey(
  log: Pick<LogRowIdentityRecord, 'row_key' | 'log_entry_id'>
): string | undefined {
  const candidate = (log.row_key || log.log_entry_id || '').trim()
  if (!candidate) return undefined
  if (
    candidate.startsWith('legacy-client:') ||
    candidate.startsWith('legacy-ch:')
  ) {
    return undefined
  }
  const hashIdx = candidate.indexOf('#')
  return hashIdx > 0 ? candidate.slice(0, hashIdx) : candidate
}

// ============================================================================
// Type Checkers & Utilities
// ============================================================================

/**
 * Check if log type is displayable (has detailed info)
 */
export function isDisplayableLogType(type: number): boolean {
  return (DISPLAYABLE_LOG_TYPES as readonly number[]).includes(type)
}

/**
 * Check if log type shows timing info
 */
export function isTimingLogType(type: number): boolean {
  return (TIMING_LOG_TYPES as readonly number[]).includes(type)
}

/**
 * Get log type configuration by type number
 */
export function getLogTypeConfig(type: number) {
  return LOG_TYPES.find((t) => t.value === type) || LOG_TYPES[0]
}

/**
 * Check if log uses per-call billing
 */
export function isPerCallBilling(modelPrice?: number): boolean {
  return (modelPrice ?? 0) > 0
}

/**
 * Get default time range (today 00:00:00 to now + 1 hour)
 */
export function getDefaultTimeRange(): { start: Date; end: Date } {
  const now = new Date()
  const start = new Date(now)
  start.setHours(0, 0, 0, 0)
  const end = new Date(now.getTime() + 3600 * 1000) // +1 hour

  return { start, end }
}

/**
 * Convert milliseconds timestamp to seconds for API
 */
function timestampToSeconds(ms: number): number {
  return Math.floor(ms / 1000)
}

/**
 * Build time range parameters with default values
 * Shared logic for all log types
 */
function buildTimeRangeParams(
  searchParams: Record<string, unknown>,
  useMilliseconds: boolean
): { start_timestamp?: number; end_timestamp?: number } {
  const hasTimeParams = searchParams.startTime ?? searchParams.endTime
  const defaultTimeRange = !hasTimeParams ? getDefaultTimeRange() : null

  const convertTimestamp = (timestamp: number) =>
    useMilliseconds ? timestamp : timestampToSeconds(timestamp)

  const getTimestamp = (paramTime?: unknown, defaultTime?: Date) => {
    const time = (paramTime as number) || defaultTime?.getTime()
    return time ? convertTimestamp(time) : undefined
  }

  return {
    start_timestamp: getTimestamp(
      searchParams.startTime,
      defaultTimeRange?.start
    ),
    end_timestamp: getTimestamp(searchParams.endTime, defaultTimeRange?.end),
  }
}

/**
 * Build base parameters with time range (for drawing and task logs)
 * @param useMilliseconds - Whether to use millisecond timestamps (true for drawing logs, false for task logs)
 */
export function buildBaseParams(config: {
  page: number
  pageSize: number
  searchParams: Record<string, unknown>
  useMilliseconds?: boolean
}): {
  p: number
  page_size: number
  channel_id?: string
  start_timestamp?: number
  end_timestamp?: number
} {
  const { page, pageSize, searchParams, useMilliseconds = false } = config

  return {
    p: page,
    page_size: pageSize,
    ...(searchParams.channel
      ? {
          channel_id: String(searchParams.channel),
        }
      : {}),
    ...buildTimeRangeParams(searchParams, useMilliseconds),
  }
}

/**
 * Build API params from search params and column filters (for common logs)
 */
export function buildApiParams(config: {
  page: number
  pageSize: number
  searchParams: Record<string, unknown>
  columnFilters?: Array<{ id: string; value: unknown }>
  isAdmin: boolean
}): GetLogsParams {
  const { page, pageSize, searchParams, columnFilters = [], isAdmin } = config

  // Helper to process type parameter (single value from array)
  const processType = (value: unknown): number | undefined => {
    const parseType = (raw: unknown): number | undefined => {
      const type = Number(raw)
      return Number.isFinite(type) ? type : undefined
    }

    if (Array.isArray(value) && value.length === 1) {
      return parseType(value[0])
    }
    if (typeof value === 'string' && value !== '') {
      return parseType(value)
    }
    return undefined
  }

  // Build base params from search params
  const params: GetLogsParams = {
    p: page,
    page_size: pageSize,
    ...(searchParams.type ? { type: processType(searchParams.type) } : {}),
    ...(searchParams.model ? { model_name: String(searchParams.model) } : {}),
    ...(searchParams.token ? { token_name: String(searchParams.token) } : {}),
    ...(searchParams.group ? { group: String(searchParams.group) } : {}),
    ...(isAdmin && searchParams.channel
      ? { channel: Number(searchParams.channel) || 0 }
      : {}),
    ...(isAdmin && searchParams.username
      ? { username: String(searchParams.username) }
      : {}),
    ...(searchParams.requestId
      ? { request_id: String(searchParams.requestId) }
      : {}),
    ...(searchParams.upstreamRequestId
      ? { upstream_request_id: String(searchParams.upstreamRequestId) }
      : {}),
    ...buildTimeRangeParams(searchParams, false),
  }

  // Override with column filters if present
  if (columnFilters.length > 0) {
    columnFilters.forEach(({ id, value }) => {
      if (value === undefined || value === null || value === '') return

      switch (id) {
        case 'type':
          params.type = processType(value)
          break
        case 'model_name':
          params.model_name = String(value)
          break
        case 'token_name':
          params.token_name = String(value)
          break
        case 'group':
          params.group = String(value)
          break
        case 'channel':
          if (isAdmin) params.channel = Number(value) || 0
          break
        case 'username':
          if (isAdmin) params.username = String(value)
          break
      }
    })
  }

  return params
}

// ============================================================================
// Data Fetching
// ============================================================================

/**
 * Fetch logs based on category type
 */
export async function fetchLogsByCategory(
  config: FetchLogsConfig
): Promise<GetLogsResponse> {
  const { logCategory, isAdmin, page, pageSize, searchParams, columnFilters } =
    config

  if (logCategory === 'common') {
    const params = buildApiParams({
      page,
      pageSize,
      searchParams,
      columnFilters,
      isAdmin,
    })
    const response = isAdmin
      ? await getAllLogs(params)
      : await getUserLogs(params)
    if (response?.data?.items) {
      return {
        ...response,
        data: {
          ...response.data,
          items: ensureUniqueLogRowKeys(response.data.items as UsageLog[]),
        },
      }
    }
    return response
  }

  // For drawing and task logs
  const baseParams = buildBaseParams({
    page,
    pageSize,
    searchParams,
    useMilliseconds: logCategory === 'drawing',
  })

  const paramsWithFilter = {
    ...baseParams,
    ...(logCategory === 'drawing'
      ? { mj_id: searchParams.filter as string | undefined }
      : {}),
    ...(logCategory === 'task'
      ? { task_id: searchParams.filter as string | undefined }
      : {}),
  }

  if (logCategory === 'drawing') {
    return isAdmin
      ? await getAllMidjourneyLogs(paramsWithFilter as GetMidjourneyLogsParams)
      : await getUserMidjourneyLogs(paramsWithFilter as GetMidjourneyLogsParams)
  }

  // task logs
  return isAdmin
    ? await getAllTaskLogs(paramsWithFilter as GetTaskLogsParams)
    : await getUserTaskLogs(paramsWithFilter as GetTaskLogsParams)
}
