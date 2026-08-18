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

import { formatDurationMs, toolTarget } from './utils'

describe('tool log helpers', () => {
  it('prefers query over url', () => {
    expect(toolTarget('筱锋', 'https://example.com')).toBe('筱锋')
    expect(toolTarget('', 'https://example.com')).toBe('https://example.com')
    expect(toolTarget('  ', '')).toBe('')
  })

  it('formats duration', () => {
    expect(formatDurationMs(0)).toBe('0ms')
    expect(formatDurationMs(240)).toBe('240ms')
    expect(formatDurationMs(1500)).toBe('1.50s')
  })
})
