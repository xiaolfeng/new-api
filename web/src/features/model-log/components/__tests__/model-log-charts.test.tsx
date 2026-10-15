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
import { fireEvent, render, screen, within } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import en from '@/i18n/locales/en.json'

import type {
  ChartDataPoint,
  TokenRecordHourCell,
  TokenRecordRecentItem,
} from '../../types'
import { ModelLogCharts } from '../model-log-charts'

interface RenderedChartSpec {
  data: { id: string; values: ChartDataPoint[] }[]
  axes: { orient: string; min?: number; max?: number; nice?: boolean }[]
  invalidType: string
}

// Keep the real data transforms and specs; only replace the canvas renderer.
vi.mock('@visactor/react-vchart', () => ({
  VChart: (props: { spec: RenderedChartSpec }) => (
    <div role='img' aria-label={props.spec.data[0].id}>
      {JSON.stringify(props.spec)}
    </div>
  ),
}))

vi.mock('@visactor/vchart', () => ({
  ThemeManager: { setCurrentTheme: vi.fn() },
}))

vi.mock('@/context/theme-provider', () => ({
  useTheme: () => ({ resolvedTheme: 'light' }),
}))

const i18n = createInstance()

beforeEach(async () => {
  await i18n.init({
    lng: 'en',
    resources: { en },
    interpolation: { escapeValue: false },
  })
})

function makeCell(
  values: Partial<TokenRecordHourCell> = {}
): TokenRecordHourCell {
  return {
    bucket_start_at: 1711454400,
    bucket_end_at: 1711457999,
    request_count: 0,
    prompt_tokens: 0,
    completion_tokens: 0,
    total_tokens: 0,
    total_use_time: 0,
    avg_tps: null,
    tps_sample_count: 0,
    thinking_tokens: 0,
    thinking_duration_ms: 0,
    avg_thinking_tps: null,
    thinking_sample_count: 0,
    output_tokens: 0,
    output_duration_ms: 0,
    avg_output_tps: null,
    output_sample_count: 0,
    tool_tokens: 0,
    tool_duration_ms: 0,
    avg_tool_tps: null,
    tool_sample_count: 0,
    failed_count: 0,
    failed_detail: {},
    is_current: false,
    ...values,
  }
}

function makeItem(
  modelName: string,
  cells: TokenRecordHourCell[]
): TokenRecordRecentItem {
  return {
    model_name: modelName,
    cells,
    summary: {
      request_count: 0,
      prompt_tokens: 0,
      completion_tokens: 0,
      total_tokens: 0,
      total_use_time: 0,
      avg_tps: null,
      tps_sample_count: 0,
      thinking_tokens: 0,
      thinking_duration_ms: 0,
      avg_thinking_tps: null,
      thinking_sample_count: 0,
      output_tokens: 0,
      output_duration_ms: 0,
      avg_output_tps: null,
      output_sample_count: 0,
      tool_tokens: 0,
      tool_duration_ms: 0,
      avg_tool_tps: null,
      tool_sample_count: 0,
      failed_count: 0,
      failed_rate: 0,
      failed_detail: {},
    },
  }
}

function renderCharts(
  items: TokenRecordRecentItem[],
  selectedModels = new Set(['model-a'])
) {
  return render(
    <I18nextProvider i18n={i18n}>
      <ModelLogCharts
        sortedItems={items}
        selectedModels={selectedModels}
        locale='en-US'
      />
    </I18nextProvider>
  )
}

async function readChart(name: string): Promise<RenderedChartSpec> {
  const chart = await screen.findByRole('img', { name })
  return JSON.parse(chart.textContent ?? '{}') as RenderedChartSpec
}

function selectTab(name: string) {
  fireEvent.click(screen.getByRole('button', { name }))
  expect(screen.getByRole('button', { name })).toHaveAttribute(
    'aria-pressed',
    'true'
  )
}

describe('ModelLogCharts metric panels', () => {
  it('shows the output token panel and only the selected model series', async () => {
    renderCharts([
      makeItem('model-a', [
        makeCell({ request_count: 2, completion_tokens: 120 }),
      ]),
      makeItem('model-b', [
        makeCell({ request_count: 1, completion_tokens: 900 }),
      ]),
    ])

    const panel = screen.getByRole('region', { name: 'Output Token Trend' })
    expect(
      within(panel).getByRole('heading', { name: 'Output Token Trend' })
    ).toBeVisible()
    expect(
      within(panel).getByText(
        'Hourly output token volume for each model in the selected time range.'
      )
    ).toBeVisible()
    const spec = await readChart('outputTokenData')
    expect(spec.data[0].values).toEqual([
      expect.objectContaining({ Model: 'model-a', Value: 120 }),
    ])
  })

  it.each([
    ['no models', [], new Set(['model-a'])],
    ['no hourly cells', [makeItem('model-a', [])], new Set(['model-a'])],
    [
      'padded empty hours',
      [makeItem('model-a', [makeCell()])],
      new Set(['model-a']),
    ],
    [
      'no selected models',
      [
        makeItem('model-a', [
          makeCell({ request_count: 1, completion_tokens: 120 }),
        ]),
      ],
      new Set<string>(),
    ],
    [
      'only an unselected model has data',
      [
        makeItem('model-a', [makeCell()]),
        makeItem('model-b', [
          makeCell({ request_count: 1, completion_tokens: 120 }),
        ]),
      ],
      new Set(['model-a']),
    ],
  ] as const)(
    'shows friendly empty states for %s instead of empty charts',
    async (_label, items, selection) => {
      renderCharts([...items], selection)
      expect(
        await screen.findByText(
          'No output token data in the selected time range.'
        )
      ).toBeVisible()
      expect(screen.queryByRole('img')).not.toBeInTheDocument()

      selectTab('Failure Rate Trend')
      expect(
        await screen.findByText('No request data in the selected time range.')
      ).toBeVisible()
      expect(screen.queryByRole('img')).not.toBeInTheDocument()
      expect(screen.queryByText('No failed requests')).not.toBeInTheDocument()
    }
  )

  it('preserves real zero-token observations and healthy zero-percent rates', async () => {
    renderCharts([
      makeItem('model-a', [makeCell({ request_count: 2 })]),
      makeItem('model-b', [makeCell({ failed_count: 3 })]),
    ])
    expect((await readChart('outputTokenData')).data[0].values[0].Value).toBe(0)
    expect(
      screen.queryByText('No output token data in the selected time range.')
    ).not.toBeInTheDocument()

    selectTab('Failure Rate Trend')
    const panel = screen.getByRole('region', { name: 'Failure Rate Trend' })
    expect(
      within(panel).getByText(
        'Hourly request failure rates for each model help identify model or upstream issues.'
      )
    ).toBeVisible()
    expect(within(panel).getByText('No failed requests')).toBeVisible()
    const spec = await readChart('failureRateData')
    expect(spec.data[0].values[0].Value).toBe(0)
    expect(spec.axes.find((axis) => axis.orient === 'left')).toMatchObject({
      min: 0,
      max: 100,
      nice: false,
    })
  })

  it('shows failure percentages and breaks the line at hours with no requests', async () => {
    renderCharts([
      makeItem('model-a', [
        makeCell({ request_count: 3, failed_count: 1 }),
        makeCell({ bucket_start_at: 1711458000 }),
        makeCell({ bucket_start_at: 1711461600, failed_count: 2 }),
      ]),
    ])
    selectTab('Failure Rate Trend')
    expect(await screen.findByText('Percentage')).toBeVisible()
    expect(screen.queryByText('No failed requests')).not.toBeInTheDocument()
    const spec = await readChart('failureRateData')
    expect(spec.data[0].values.map((point) => point.Value)).toEqual([
      25,
      null,
      100,
    ])
    expect(spec.invalidType).toBe('break')
  })

  it('does not label a rounded zero-percent rate as no failed requests', async () => {
    renderCharts([
      makeItem('model-a', [
        makeCell({ request_count: 100000, failed_count: 1 }),
      ]),
    ])
    selectTab('Failure Rate Trend')
    expect((await readChart('failureRateData')).data[0].values[0].Value).toBe(0)
    expect(screen.getByText('Percentage')).toBeVisible()
    expect(screen.queryByText('No failed requests')).not.toBeInTheDocument()
  })

  it('keeps both TPS panels and the estimated badge when switching tabs', async () => {
    renderCharts([
      makeItem('model-a', [
        makeCell({
          request_count: 1,
          completion_tokens: 100,
          avg_tps: 20,
          tps_sample_count: 1,
          avg_output_tps: 30,
          output_sample_count: 1,
        }),
      ]),
    ])
    selectTab('TPS Trends')
    expect(
      screen.getByRole('region', { name: 'Average TPS Trend' })
    ).toBeVisible()
    const estimated = screen.getByRole('region', {
      name: 'Estimated Phase TPS Trend',
    })
    expect(within(estimated).getByText('Estimated')).toBeVisible()
    expect((await readChart('averageTpsData')).data[0].values[0].Value).toBe(20)
    expect((await readChart('estimatedTpsData')).data[0].values[0].Value).toBe(
      30
    )
  })

  it('shows centered empty messages for both TPS panels without valid samples', async () => {
    renderCharts([makeItem('model-a', [makeCell()])])
    selectTab('TPS Trends')
    expect(
      await screen.findByText('No average TPS data in the selected time range.')
    ).toBeVisible()
    expect(
      screen.getByText(
        'No estimated phase TPS data in the selected time range.'
      )
    ).toBeVisible()
    expect(screen.getByText('Estimated')).toBeVisible()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
  })
})
