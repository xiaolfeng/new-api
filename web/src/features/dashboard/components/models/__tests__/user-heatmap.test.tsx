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
import { fireEvent, render, screen, within } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import en from '@/i18n/locales/en.json'

const apiGetMock = vi.hoisted(() => vi.fn())

vi.mock('@/lib/api', () => ({
  api: { get: apiGetMock },
}))

import { UserHeatmap } from '../user-heatmap'

const i18n = createInstance()

beforeEach(async () => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date(2026, 9, 15, 12))
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
          prompt_tokens: 10,
          completion_tokens: 5,
          total_tokens: 15,
        },
      ],
    },
  })
})

afterEach(() => {
  vi.useRealTimers()
})

describe('UserHeatmap tooltip positioning', () => {
  test('tooltip renders through a body portal, escaping ancestor containing blocks', async () => {
    const { container } = render(
      <I18nextProvider i18n={i18n}>
        <UserHeatmap />
      </I18nextProvider>
    )

    const cell = (await screen.findAllByRole('gridcell'))[0]
    fireEvent.mouseEnter(cell)

    const tooltip = await screen.findByRole('tooltip')
    // 坐标取自视口，tooltip 必须挂在 body 下，不能留在可能建立
    // fixed 包含块（filter/transform 动画）或 overflow 裁剪的祖先内。
    expect(document.body.contains(tooltip)).toBe(true)
    expect(container.contains(tooltip)).toBe(false)
  })

  test('grid dates align to Monday-first rows', async () => {
    render(
      <I18nextProvider i18n={i18n}>
        <UserHeatmap />
      </I18nextProvider>
    )

    await screen.findAllByRole('gridcell')
    const firstCell = screen.getAllByRole('gridcell')[0]
    // 第一行标签是 Monday：首个日期必须是周一，行序才与星期标签一致。
    const label = firstCell.getAttribute('aria-label') ?? ''
    const dateStr = label.split(':')[0]
    const [y, m, d] = dateStr.split('-').map(Number)
    const dow = new Date(y, m - 1, d).getDay()
    expect(dow).toBe(1)
  })
})

describe('UserHeatmap token formatting', () => {
  test.each([
    {
      prompt: 600,
      completion: 399,
      total: 999,
      compact: '999',
      exact: '999',
      promptDisplay: '600',
      promptExact: '600',
      completionDisplay: '399',
      completionExact: '399',
    },
    {
      prompt: 1000,
      completion: 0,
      total: 1000,
      compact: '1k',
      exact: '1,000',
      promptDisplay: '1k (1,000)',
      promptExact: '1,000',
      completionDisplay: '0',
      completionExact: '0',
    },
    {
      prompt: 1_200_000,
      completion: 34_567,
      total: 1_234_567,
      compact: '1.23M',
      exact: '1,234,567',
      promptDisplay: '1.2M (1,200,000)',
      promptExact: '1,200,000',
      completionDisplay: '34.6k (34,567)',
      completionExact: '34,567',
    },
    {
      prompt: 1_200_000_000,
      completion: 34_567_890,
      total: 1_234_567_890,
      compact: '1.235B',
      exact: '1,234,567,890',
      promptDisplay: '1.2B (1,200,000,000)',
      promptExact: '1,200,000,000',
      completionDisplay: '34.57M (34,567,890)',
      completionExact: '34,567,890',
    },
  ])(
    'formats $total tokens with exact values for reconciliation',
    async (item) => {
      apiGetMock.mockResolvedValue({
        data: {
          success: true,
          data: [
            {
              date: '2026-10-05',
              prompt_tokens: item.prompt,
              completion_tokens: item.completion,
              total_tokens: item.total,
            },
          ],
        },
      })
      render(
        <I18nextProvider i18n={i18n}>
          <UserHeatmap />
        </I18nextProvider>
      )

      const heatmap = await screen.findByTestId('user-heatmap')
      expect(
        within(heatmap).getByTitle(`${item.exact} tokens`)
      ).toHaveTextContent(`${item.compact} tokens`)
      const cell = within(heatmap).getByRole('gridcell', {
        name: `2026-10-05: ${item.compact} (${item.exact}) tokens`,
      })
      fireEvent.mouseEnter(cell)

      const tooltip = within(await screen.findByRole('tooltip'))
      expect(tooltip.getByTitle(`${item.exact} tokens`)).toHaveTextContent(
        item.total >= 1000
          ? `${item.compact} (${item.exact}) tokens`
          : `${item.compact} tokens`
      )
      expect(tooltip.getByTitle(item.promptExact)).toHaveTextContent(
        item.promptDisplay
      )
      expect(tooltip.getByTitle(item.completionExact)).toHaveTextContent(
        item.completionDisplay
      )
      fireEvent.mouseLeave(cell)
      expect(screen.queryByRole('tooltip')).not.toBeInTheDocument()
    }
  )

  test('uses the resolved language for exact number separators', async () => {
    await i18n.init({
      lng: 'fr-CA',
      resources: { fr: en },
      interpolation: { escapeValue: false },
    })
    expect(i18n.resolvedLanguage).toBe('fr')
    apiGetMock.mockResolvedValue({
      data: {
        success: true,
        data: [
          {
            date: '2026-10-05',
            prompt_tokens: 1234,
            completion_tokens: 1000,
            total_tokens: 2234,
          },
        ],
      },
    })
    render(
      <I18nextProvider i18n={i18n}>
        <UserHeatmap />
      </I18nextProvider>
    )

    const heatmap = await screen.findByTestId('user-heatmap')
    // Preserve the French narrow no-break space when matching exact titles.
    const titleOptions = { collapseWhitespace: false }
    expect(
      within(heatmap).getByTitle('2\u202f234 tokens', titleOptions)
    ).toHaveTextContent('2.2k tokens')
    const cell = within(heatmap).getByRole('gridcell', {
      name: '2026-10-05: 2.2k (2\u202f234) tokens',
    })
    fireEvent.mouseEnter(cell)

    const tooltip = within(await screen.findByRole('tooltip'))
    expect(
      tooltip.getByTitle('2\u202f234 tokens', titleOptions)
    ).toHaveTextContent('2.2k (2 234) tokens')
    expect(tooltip.getByTitle('1\u202f234', titleOptions)).toHaveTextContent(
      '1.2k (1 234)'
    )
    expect(tooltip.getByTitle('1\u202f000', titleOptions)).toHaveTextContent(
      '1k (1 000)'
    )
  })
})
