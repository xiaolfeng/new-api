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
import { describe, expect, test } from 'vitest'

import {
  HOST_TOOL_CANONICAL_FETCH,
  HOST_TOOL_CANONICAL_SEARCH,
  getUsageActivityTagDetails,
} from '../format'

const searchTag = { id: 'web_search' as const, labelKey: 'Web Search' }
const fetchTag = { id: 'web_fetch' as const, labelKey: 'WebFetch' }
const imageTag = {
  id: 'image_recognize' as const,
  labelKey: 'Image recognition',
}

describe('getUsageActivityTagDetails', () => {
  test('describes public host tool tags with call count and canonical fallback', () => {
    const details = getUsageActivityTagDetails(
      {
        usage_tags: ['web_search'],
        web_search: true,
        web_search_call_count: 2,
      },
      searchTag
    )

    expect(details.kind).toBe('host_tool')
    expect(details.count).toBe(2)
    expect(details.names).toEqual(['web_search'])
    expect(details.canonicals).toEqual([HOST_TOOL_CANONICAL_SEARCH])
    expect(details.backends).toEqual([])
  })

  test('prefers admin exec original names and backend for the matching host tool', () => {
    const details = getUsageActivityTagDetails(
      {
        usage_tags: ['web_search', 'web_fetch'],
        web_search: true,
        web_search_call_count: 1,
        web_fetch: true,
        web_fetch_call_count: 1,
        admin_info: {
          host_tools: {
            execs: [
              {
                original_name: 'WebSearch',
                canonical: HOST_TOOL_CANONICAL_SEARCH,
                backend: 'searxng',
                duration_ms: 240,
              },
              {
                original_name: 'WebFetch',
                canonical: HOST_TOOL_CANONICAL_FETCH,
                backend: 'http',
                duration_ms: 80,
              },
            ],
          },
        },
      },
      searchTag
    )

    expect(details.names).toEqual(['WebSearch'])
    expect(details.canonicals).toEqual([HOST_TOOL_CANONICAL_SEARCH])
    expect(details.backends).toEqual(['searxng'])
    expect(details.durationMs).toEqual([240])
  })

  test('describes image recognition as a hop, not a host tool', () => {
    const details = getUsageActivityTagDetails(
      {
        image_recognize: true,
        image_recognize_image_count: 3,
      },
      imageTag
    )

    expect(details.kind).toBe('image_recognize')
    expect(details.count).toBe(3)
    expect(details.names).toEqual([])
    expect(details.canonicals).toEqual([])
  })

  test('does not mix fetch execs into a search tooltip', () => {
    const details = getUsageActivityTagDetails(
      {
        usage_tags: ['web_fetch'],
        web_fetch: true,
        web_fetch_call_count: 1,
        admin_info: {
          host_tools: {
            execs: [
              {
                original_name: 'WebFetch',
                canonical: HOST_TOOL_CANONICAL_FETCH,
              },
            ],
          },
        },
      },
      fetchTag
    )

    expect(details.names).toEqual(['WebFetch'])
    expect(details.canonicals).toEqual([HOST_TOOL_CANONICAL_FETCH])
  })
})
