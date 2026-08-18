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
import { describe, expect, it } from 'vitest'

import { getCacheRateSummary } from '../format'

describe('getCacheRateSummary', () => {
  it('uses backend input_tokens_total as the denominator', () => {
    const summary = getCacheRateSummary(100, {
      cache_tokens: 50,
      input_tokens_total: 200,
      claude: true,
    })
    expect(summary.rate).toBeCloseTo(25)
    expect(summary.totalInput).toBe(200)
  })

  it('adds cache tokens for Claude when input_tokens_total is missing', () => {
    const summary = getCacheRateSummary(100, {
      cache_tokens: 50,
      cache_creation_tokens: 10,
      claude: true,
    })
    expect(summary.totalInput).toBe(160)
    expect(summary.rate).toBeCloseTo(31.25)
  })

  it('does not double-count cache read for OpenAI semantics', () => {
    const summary = getCacheRateSummary(150, {
      cache_tokens: 50,
      claude: false,
    })
    expect(summary.totalInput).toBe(150)
    expect(summary.rate).toBeCloseTo(50 / 150 * 100)
  })

  it('returns null when there is no cache read', () => {
    const summary = getCacheRateSummary(100, { cache_creation_tokens: 20 })
    expect(summary.rate).toBeNull()
  })
})
