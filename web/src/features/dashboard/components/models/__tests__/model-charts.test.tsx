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
For commercial licensing, please contact support@quantumnous.com.
*/
import { render } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import en from '@/i18n/locales/en.json'

vi.mock('@visactor/react-vchart', () => ({ VChart: () => null }))
vi.mock('@/features/dashboard/lib', async (importOriginal) => {
  const actual =
    await importOriginal<typeof import('@/features/dashboard/lib')>()
  return {
    ...actual,
    processChartData: vi.fn(actual.processChartData),
  }
})

import { processChartData } from '@/features/dashboard/lib'
import { ConsumptionDistributionChart } from '../consumption-distribution-chart'
import { ModelCharts } from '../model-charts'

const i18n = createInstance()

beforeEach(async () => {
  await i18n.init({
    lng: 'en',
    resources: { en },
    interpolation: { escapeValue: false },
  })
  vi.mocked(processChartData).mockClear()
})

describe('models charts share aggregation', () => {
  test('chart components consume precomputed chartData instead of aggregating inline', () => {
    const data = [
      {
        model_name: 'gpt-5',
        created_at: 1754006400,
        quota: 100,
        count: 1,
        token_used: 10,
      },
    ]

    const chartData = processChartData(data, 'day')
    vi.mocked(processChartData).mockClear()

    render(
      <I18nextProvider i18n={i18n}>
        <ConsumptionDistributionChart data={data} chartData={chartData} />
        <ModelCharts data={data} chartData={chartData} />
      </I18nextProvider>
    )

    // 聚合由父级一次完成；组件内不得重复执行完整聚合。
    expect(processChartData).not.toHaveBeenCalled()
  })
})
