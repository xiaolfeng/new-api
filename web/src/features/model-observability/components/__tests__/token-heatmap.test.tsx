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

import { SummaryCards } from '../summary-cards'
import { TokenHeatmap } from '../token-heatmap'

const apiGetMock = vi.hoisted(() => vi.fn())

vi.mock('@/lib/api', () => ({
  api: { get: apiGetMock },
}))

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
        {
          date: '2026-10-05',
          prompt_tokens: 100000,
          completion_tokens: 500,
          total_tokens: 100500,
        },
      ],
    },
  })
})

describe('TokenHeatmap and SummaryCards token unit formatting', () => {
  test('renders k/M/B token units in header and tooltip while preserving exact integers', async () => {
    const { container } = render(
      <I18nextProvider i18n={i18n}>
        <TokenHeatmap />
      </I18nextProvider>
    )

    // Header shows abbreviated 100.5k tokens with exact integer in title
    const headerTotal = await screen.findByText('100.5k tokens')
    expect(headerTotal).toHaveAttribute('title', '100,500 tokens')

    const cells = await screen.findAllByRole('gridcell')
    const activeCell = cells.find((el) =>
      el.getAttribute('aria-label')?.startsWith('2026-10-05:')
    )
    expect(activeCell).toBeDefined()
    expect(activeCell).toHaveAttribute(
      'aria-label',
      '2026-10-05: 100.5k (100,500) tokens'
    )

    if (!activeCell) {
      throw new Error('Expected active heatmap cell')
    }
    fireEvent.mouseEnter(activeCell)

    const tooltip = await screen.findByRole('tooltip')
    expect(document.body.contains(tooltip)).toBe(true)
    expect(container.contains(tooltip)).toBe(false)
    expect(tooltip).toHaveTextContent('100.5k (100,500) tokens')
    expect(tooltip).toHaveTextContent('100k (100,000)')
    expect(tooltip).toHaveTextContent('500')
  })

  test('formats SummaryCards input/output tokens with k/M/B and keeps request count on compact number format', () => {
    render(
      <I18nextProvider i18n={i18n}>
        <SummaryCards
          summary={{
            total_request_count: 1200,
            total_prompt_tokens: 10450000,
            total_output_tokens: 2345000000,
            active_model_count: 6,
          }}
        />
      </I18nextProvider>
    )

    const inputVal = screen.getByText('10.45M')
    expect(inputVal).toHaveAttribute('title', '10,450,000')

    const outputVal = screen.getByText('2.345B')
    expect(outputVal).toHaveAttribute('title', '2,345,000,000')
  })
})
