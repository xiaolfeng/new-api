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
      'Single Turn': 'Single Turn',
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

  test('renders the single turn badge from precomputed chinese value', () => {
    render(
      <InteractionTypeCell
        log={createLog({
          other: JSON.stringify({ interaction_type: '单轮' }),
        })}
      />
    )

    expect(screen.getByText('Single Turn')).toBeInTheDocument()
  })

  test('still shows a compact tool trigger when the interaction type is missing', () => {
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
    expect(screen.queryByText('Web Search')).not.toBeInTheDocument()
    expect(screen.getByLabelText('Used host tool')).toHaveAttribute(
      'data-usage-activity-tag',
      'summary'
    )
  })

  test('keeps the interaction type and tool details on one row inside the tooltip', async () => {
    const user = userEvent.setup()
    render(
      <InteractionTypeCell
        log={createLog({
          other: JSON.stringify({
            interaction_type: 'callback',
            usage_tags: ['web_fetch', 'image_recognize'],
            web_fetch: true,
            web_fetch_call_count: 1,
            image_recognize: true,
            image_recognize_image_count: 2,
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
    expect(screen.queryByText('WebFetch')).not.toBeInTheDocument()
    expect(screen.queryByText('Image recognition')).not.toBeInTheDocument()
    const row = screen.getByText('Callback').parentElement
    expect(row?.className).toContain('whitespace-nowrap')
    expect(row?.className).toContain('inline-flex')

    await user.hover(screen.getByLabelText('Used host tool'))
    expect(await screen.findByText('WebFetch')).toBeInTheDocument()
    expect(screen.getByText('WebFetch → host.web_fetch')).toBeInTheDocument()
    expect(screen.getByText(/1 calls/)).toBeInTheDocument()
    expect(screen.getByText('Image recognition')).toBeInTheDocument()
    expect(screen.getByText(/2 images/)).toBeInTheDocument()
  })

  test('returns nothing when there is no type and no tool activity', () => {
    const { container } = render(<InteractionTypeCell log={createLog({})} />)
    expect(container).toBeEmptyDOMElement()
  })
})
