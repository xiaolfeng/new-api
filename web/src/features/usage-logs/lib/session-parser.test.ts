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
import { describe, expect, it } from 'bun:test'

import type { UsageLog } from '../data/schema'
import { parseLogSession } from './session-parser'

function createLog(overrides: Partial<UsageLog>): UsageLog {
  return {
    id: 1,
    user_id: 1,
    created_at: 0,
    type: 2,
    content: '模型价格 1.00，分组倍率 1.00',
    username: '',
    token_name: '',
    model_name: '',
    quota: 0,
    prompt_tokens: 0,
    completion_tokens: 0,
    use_time: 0,
    is_stream: false,
    channel: 0,
    channel_name: '',
    token_id: 0,
    group: '',
    ip: '',
    other: '',
    request_id: '',
    upstream_request_id: '',
    record: '',
    full_log: '',
    ...overrides,
  }
}

describe('parseLogSession', () => {
  it('prefers backend summaries from other', () => {
    const log = createLog({
      other: JSON.stringify({
        client_source: 'Codex',
        session_id: 'win_abc:1',
        session_name: 'SwiftCrystalEcho',
      }),
    })

    expect(parseLogSession(log)).toEqual({
      sessionId: 'win_abc:1',
      sessionName: 'SwiftCrystalEcho',
    })
  })

  it('falls back to record headers for Codex window id', () => {
    const log = createLog({
      record: JSON.stringify({
        headers: {
          'User-Agent': 'codex_cli_rs/0.20.0',
          'X-Codex-Window-Id': '019fbd87-1caa-7fd1-b7db-53756e002c02:0',
        },
      }),
    })

    expect(parseLogSession(log).sessionId).toBe(
      '019fbd87-1caa-7fd1-b7db-53756e002c02:0'
    )
  })

  it('falls back to full_log request headers and turn metadata', () => {
    const log = createLog({
      full_log: JSON.stringify({
        request: {
          headers: {
            'User-Agent': 'codex_cli_rs/0.20.0',
            'X-Codex-Turn-Metadata': JSON.stringify({
              session_id: 'sess_abc',
              thread_id: 'thread_abc',
              window_id: '019fbd87-1caa-7fd1-b7db-53756e002c02:1',
            }),
          },
        },
      }),
    })

    expect(parseLogSession(log).sessionId).toBe(
      '019fbd87-1caa-7fd1-b7db-53756e002c02:1'
    )
  })

  it('recognizes Codex Desktop originator and parent thread', () => {
    const log = createLog({
      record: JSON.stringify({
        headers: {
          Originator: 'codex_vscode',
          'X-Codex-Window-Id': 'win_sub:2',
          'X-Codex-Parent-Thread-Id': 'thread_parent',
        },
      }),
    })

    expect(parseLogSession(log)).toEqual({
      sessionId: 'win_sub:2',
      parentSessionId: 'thread_parent',
    })
  })

  it('falls back to generic session headers for other clients', () => {
    const log = createLog({
      record: JSON.stringify({
        headers: {
          'User-Agent': 'opencode/0.1',
          'X-Session-Affinity': 'sess_affinity',
          'X-Parent-Session-Id': 'sess_parent',
        },
      }),
    })

    expect(parseLogSession(log)).toEqual({
      sessionId: 'sess_affinity',
      parentSessionId: 'sess_parent',
    })
  })

  it('returns empty session when no headers or summaries exist', () => {
    expect(parseLogSession(createLog({}))).toEqual({})
  })
})
