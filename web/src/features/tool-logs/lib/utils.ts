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
export function getDefaultTimeRange(): { start: Date; end: Date } {
  const now = new Date()
  const start = new Date(now)
  start.setHours(0, 0, 0, 0)
  const end = new Date(now.getTime() + 3600 * 1000)
  return { start, end }
}

export function timestampToSeconds(ms: number): number {
  return Math.floor(ms / 1000)
}

export function formatDurationMs(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return '0ms'
  if (ms < 1000) return `${Math.round(ms)}ms`
  return `${(ms / 1000).toFixed(2)}s`
}

export function toolTarget(
  query: string,
  url: string,
  kind?: string
): string {
  const q = query.trim()
  const u = url.trim()
  if (kind === 'fetch') return u || q
  if (kind === 'search') return q || u
  if (q) return q
  return u
}

export function previewToolResult(result: string, maxLength = 80): string {
  const text = result.trim().replaceAll(/\s+/g, ' ')
  if (text === '') return ''
  if (text.length <= maxLength) return text
  return `${text.slice(0, maxLength)}…`
}

export function prettyToolResult(result: string): string {
  const trimmed = result.trim()
  if (trimmed === '') return ''
  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2)
  } catch {
    return result
  }
}

export function usageLogsSearchForRequest(params: {
  requestId: string
  startTime?: number
  endTime?: number
}): {
  requestId: string
  startTime?: number
  endTime?: number
} {
  return {
    requestId: params.requestId,
    startTime: params.startTime,
    endTime: params.endTime,
  }
}
