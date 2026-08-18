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
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { beforeAll, describe, expect, test } from 'vitest'

import type { UsageLog } from '../../data/schema'
import { InteractionTypeCell } from '../interaction-type-cell'

function createLog(overrides: Partial<UsageLog>): UsageLog {
  return {
    id: 1,
    user_id: 1,
    created_at: 0,
    type: 2,
    content: '',
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

describe('InteractionTypeCell', () => {
  beforeAll(() => {
    i18next.addResourceBundle('en', 'translation', {
      Input: 'Input',
      Output: 'Output',
      Callback: 'Callback',
      'Web Search': 'Web Search',
      WebFetch: 'WebFetch',
      'Image recognition': 'Image recognition',
      'Used host tool': 'Used host tool',
      '{{count}} calls': '{{count}} calls',
      'Image recognition hop': 'Image recognition hop',
      '{{count}} images': '{{count}} images',
    })
  })

  test('renders the interaction type without inventing tool tags', () => {
    render(
      <InteractionTypeCell
        log={createLog({
          other: JSON.stringify({ interaction_type: 'output' }),
        })}
      />
    )

    expect(screen.getByText('Output')).toBeInTheDocument()
    expect(screen.queryByText('Web Search')).not.toBeInTheDocument()
  })

  test('still shows host tool tags when the interaction type is missing', () => {
    render(
      <InteractionTypeCell
        log={createLog({
          other: JSON.stringify({
            usage_tags: ['web_search'],
            web_search: true,
            web_search_call_count: 2,
          }),
        })}
      />
    )

    expect(screen.queryByText('Input')).not.toBeInTheDocument()
    expect(
      screen.getByText('Web Search').closest('[data-usage-activity-tag]')
    ).toHaveAttribute('data-usage-activity-tag', 'web_search')
  })

  test('describes the host tool in the activity tag tooltip', async () => {
    const user = userEvent.setup()
    render(
      <InteractionTypeCell
        log={createLog({
          other: JSON.stringify({
            interaction_type: 'callback',
            usage_tags: ['web_fetch'],
            web_fetch: true,
            web_fetch_call_count: 1,
            admin_info: {
              host_tools: {
                execs: [
                  {
                    original_name: 'WebFetch',
                    canonical: 'host.web_fetch',
                    backend: 'http',
                    duration_ms: 80,
                  },
                ],
              },
            },
          }),
        })}
      />
    )

    expect(screen.getByText('Callback')).toBeInTheDocument()
    await user.hover(screen.getByText('WebFetch'))
    expect(await screen.findByText('Used host tool')).toBeInTheDocument()
    expect(screen.getByText('WebFetch → host.web_fetch')).toBeInTheDocument()
    expect(screen.getByText(/1 calls/)).toBeInTheDocument()
    expect(screen.getByText(/http/)).toBeInTheDocument()
  })

  test('returns nothing when there is no type and no tool activity', () => {
    const { container } = render(<InteractionTypeCell log={createLog({})} />)
    expect(container).toBeEmptyDOMElement()
  })
})
