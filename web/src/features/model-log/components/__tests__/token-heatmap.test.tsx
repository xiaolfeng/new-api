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
For commercial licensing, please contact support@quantumnous.com
*/
import { fireEvent, render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import en from '@/i18n/locales/en.json'

const apiGetMock = vi.hoisted(() => vi.fn())

vi.mock('@/lib/api', () => ({
  api: { get: apiGetMock },
}))

import { TokenHeatmap } from '../token-heatmap'

const i18n = createInstance()

beforeEach(async () => {
  await i18n.init({
    lng: 'en',
    resources: { en },
    interpolation: { escapeValue: false },
  })
  apiGetMock.mockReset()
  apiGetMock.mockResolvedValue({
    data: {
      success: true,
      data: [
        { date: '2026-10-05', prompt_tokens: 10, completion_tokens: 5, total_tokens: 15 },
      ],
    },
  })
})

describe('TokenHeatmap tooltip positioning', () => {
  test('tooltip renders through a body portal, escaping ancestor containing blocks', async () => {
    const { container } = render(
      <I18nextProvider i18n={i18n}>
        <TokenHeatmap />
      </I18nextProvider>
    )

    const cell = (await screen.findAllByRole('gridcell'))[0]
    fireEvent.mouseEnter(cell)

    const tooltip = await screen.findByRole('tooltip')
    expect(document.body.contains(tooltip)).toBe(true)
    expect(container.contains(tooltip)).toBe(false)
  })
})
