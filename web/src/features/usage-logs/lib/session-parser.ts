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
 * Session extraction for usage logs.
 *
 * The backend usually pre-computes `client_source` / `session_id` / `session_name`
 * into `other`. For historical rows or older binaries that lack those summaries,
 * fall back to the raw `record` / `full_log` / `content` JSON so Codex window
 * sessions (e.g. `x-codex-window-id: <thread_id>:<window_number>`) still surface.
 */
import type { UsageLog } from '../data/schema'
import type { LogOtherData } from '../types'
import { parseLogOther } from './format'
import { parseClientSource } from './source-parser'

export interface ParsedLogSession {
  sessionId?: string
  sessionName?: string
  parentSessionId?: string
  parentSessionName?: string
  agentId?: string
  agentName?: string
}

type HeaderMap = Record<string, string>

function getHeader(headers: HeaderMap, target: string): string | undefined {
  const normalized = target.toLowerCase()
  const key = Object.keys(headers).find(
    (candidate) => candidate.toLowerCase() === normalized
  )
  const value = key ? headers[key] : ''
  return value.trim() || undefined
}

function parseJsonRecord(raw: string): Record<string, unknown> | null {
  if (!raw) return null
  try {
    const parsed: unknown = JSON.parse(raw)
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed)
      ? (parsed as Record<string, unknown>)
      : null
  } catch {
    return null
  }
}

function extractHeadersFromLog(log: UsageLog): HeaderMap {
  for (const raw of [log.record, log.full_log, log.content]) {
    const record = parseJsonRecord(raw)
    if (!record) continue
    const request =
      record.request &&
      typeof record.request === 'object' &&
      !Array.isArray(record.request)
        ? (record.request as Record<string, unknown>)
        : {}
    const headers = request.headers ?? record.headers
    if (headers && typeof headers === 'object' && !Array.isArray(headers)) {
      return headers as HeaderMap
    }
  }
  return {}
}

function parseCodexWindowIdFromTurnMetadata(
  headers: HeaderMap
): string | undefined {
  const raw = getHeader(headers, 'x-codex-turn-metadata')
  if (!raw) return undefined
  const metadata = parseJsonRecord(raw)
  if (!metadata) return undefined
  const windowId = metadata.window_id
  return typeof windowId === 'string' && windowId.trim()
    ? windowId.trim()
    : undefined
}

function detectSource(headers: HeaderMap): string {
  const userAgent =
    getHeader(headers, 'user-agent') ?? getHeader(headers, 'originator') ?? ''
  return parseClientSource(userAgent).name
}

/**
 * Resolve the session info for a usage log, preferring the backend summaries
 * in `other` and falling back to request headers embedded in the raw logs.
 */
export function parseLogSession(log: UsageLog): ParsedLogSession {
  const other: LogOtherData | null = parseLogOther(log.other)
  const session: ParsedLogSession = {
    sessionId: other?.session_id || undefined,
    sessionName: other?.session_name || undefined,
    parentSessionId: other?.parent_session_id || undefined,
    parentSessionName: other?.parent_session_name || undefined,
    agentId: other?.agent_id || undefined,
    agentName: other?.agent_name || undefined,
  }

  const hasSummarizedSession =
    session.sessionId ||
    session.sessionName ||
    session.parentSessionId ||
    session.parentSessionName ||
    session.agentId ||
    session.agentName
  if (hasSummarizedSession) return session

  const headers = extractHeadersFromLog(log)
  if (Object.keys(headers).length === 0) return session

  const source = detectSource(headers)
  if (source === 'Codex') {
    session.sessionId =
      getHeader(headers, 'x-codex-window-id') ??
      parseCodexWindowIdFromTurnMetadata(headers)
    session.parentSessionId = getHeader(headers, 'x-codex-parent-thread-id')
    return session
  }

  session.sessionId =
    getHeader(headers, 'x-session-affinity') ??
    getHeader(headers, 'x-claude-code-session-id') ??
    getHeader(headers, 'x-grok-session-id') ??
    getHeader(headers, 'x-session-id')
  session.parentSessionId =
    // Grok Build: X-Grok-Agent-Id 是 Agent 父会话
    getHeader(headers, 'x-grok-agent-id') ??
    getHeader(headers, 'x-parent-session-id')
  session.agentId = getHeader(headers, 'x-claude-code-agent-id')
  return session
}
