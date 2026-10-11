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

You should have received a copy of the GNU Affero General Public
License along with this program. If not, see <https://www.gnu.org/licenses/>.
For commercial licensing, please contact support@quantumnous.com.
*/
import { describe, expect, test } from 'vitest'

import {
  ensureUniqueLogRowKeys,
  getLogRowId,
  getServerLogRowKey,
} from '../utils'

describe('getLogRowId and ensureUniqueLogRowKeys', () => {
  test('prefers server row_key or log_entry_id as the unique row identity', () => {
    expect(
      getLogRowId(
        {
          row_key: 'le_20261011001',
          log_entry_id: 'le_20261011001',
          request_id: 'req-shared',
          id: 1,
          type: 2,
        },
        0
      )
    ).toBe('le_20261011001')

    expect(
      getLogRowId(
        {
          log_entry_id: 'le_20261011002',
          request_id: 'req-shared',
          id: 2,
          type: 5,
        },
        1
      )
    ).toBe('le_20261011002')
  })

  test('disambiguates legacy rows sharing request_id when row_key is missing', () => {
    const retryFailure = {
      id: 1,
      request_id: 'req-shared',
      type: 5,
      channel: 10,
      created_at: 1700000000,
      model_name: 'gpt-4o',
      content: 'upstream 502',
    }
    const finalSuccess = {
      id: 2,
      request_id: 'req-shared',
      type: 2,
      channel: 11,
      created_at: 1700000001,
      model_name: 'gpt-4o',
      prompt_tokens: 100,
      completion_tokens: 50,
      content: 'ok',
    }

    const id1 = getLogRowId(retryFailure, 0)
    const id2 = getLogRowId(finalSuccess, 1)
    expect(id1).not.toBe('req-shared')
    expect(id2).not.toBe('req-shared')
    expect(id1).not.toBe(id2)
  })

  test('ensureUniqueLogRowKeys deduplicates identical same-second same-channel legacy retry rows', () => {
    const duplicateError = {
      id: 1,
      request_id: 'req-dup',
      type: 5,
      channel: 3,
      created_at: 1700000005,
      model_name: 'claude-3-5-sonnet',
      content: 'rate limit',
    }
    const rows = ensureUniqueLogRowKeys([
      { ...duplicateError, id: 1 },
      { ...duplicateError, id: 2 },
      {
        id: 3,
        request_id: 'req-dup',
        type: 2,
        channel: 4,
        created_at: 1700000006,
        model_name: 'claude-3-5-sonnet',
        content: 'succeeded',
      },
    ])

    const rowKeys = rows.map((row, idx) => getLogRowId(row, idx))
    expect(new Set(rowKeys).size).toBe(3)
    expect(rowKeys[1]).toBe(`${rowKeys[0]}#2`)
  })

  test('getServerLogRowKey keeps verifiable server keys and excludes synthetic fingerprints', () => {
    expect(getServerLogRowKey({ row_key: 'le_abc123' })).toBe('le_abc123')
    expect(getServerLogRowKey({ row_key: 'legacy-id:42' })).toBe('legacy-id:42')
    expect(
      getServerLogRowKey({ row_key: 'legacy-ch:1234567890abcdef#2' })
    ).toBeUndefined()
    expect(
      getServerLogRowKey({ row_key: 'legacy-client:req=req-1|t=5' })
    ).toBeUndefined()
  })

  test('falls back to id or position when no log fields exist', () => {
    expect(getLogRowId({ id: 7 }, 1)).toBe('7')
    expect(getLogRowId({ id: 0 }, 3)).toBe('3')
  })
})
