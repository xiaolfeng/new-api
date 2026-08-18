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
import { render, screen } from '@testing-library/react'
import i18next from 'i18next'
import { beforeAll, describe, expect, test } from 'vitest'

import type { ToolLog } from '../../types'
import { ToolKindBadge } from '../tool-kind-badge'
import { ToolStatusStack } from '../tool-log-cells'

function createLog(overrides: Partial<ToolLog>): ToolLog {
  return {
    id: 1,
    created_at: 0,
    user_id: 1,
    username: 'alice',
    token_id: 1,
    token_name: 'demo',
    channel: 3,
    channel_name: 'OpenAI',
    group: 'default',
    model_name: 'claude-sonnet',
    request_id: 'req_1',
    ip: '1.1.1.1',
    original_name: 'WebSearch',
    canonical: 'host.web_search',
    kind: 'search',
    mode: 'loop',
    backend: 'searxng',
    query: '筱锋',
    url: '',
    error_code: '',
    duration_ms: 240,
    truncated: false,
    result: 'ok',
    ...overrides,
  }
}

describe('tool log status cells', () => {
  beforeAll(() => {
    i18next.addResourceBundle('en', 'translation', {
      Success: 'Success',
      Truncated: 'Truncated',
      Search: 'Search',
      Fetch: 'Fetch',
      'This host tool result was truncated and may be incomplete.':
        'This host tool result was truncated and may be incomplete.',
    })
  })

  test('shows success without a truncated marker', () => {
    render(<ToolStatusStack log={createLog({})} />)
    expect(screen.getByText('Success')).toBeInTheDocument()
    expect(screen.queryByText('Truncated')).not.toBeInTheDocument()
  })

  test('shows the error code and truncated marker', () => {
    render(
      <ToolStatusStack
        log={createLog({ error_code: 'timeout', truncated: true })}
      />
    )
    expect(screen.getByText('timeout')).toBeInTheDocument()
    expect(screen.getByText('Truncated')).toBeInTheDocument()
  })

  test('labels search and fetch kinds', () => {
    const { rerender } = render(<ToolKindBadge kind='search' />)
    expect(screen.getByText('Search')).toBeInTheDocument()
    rerender(<ToolKindBadge kind='fetch' />)
    expect(screen.getByText('Fetch')).toBeInTheDocument()
  })
})
