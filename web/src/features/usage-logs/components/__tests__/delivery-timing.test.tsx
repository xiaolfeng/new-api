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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { render, screen, within } from '@testing-library/react'
import { afterEach, expect, it } from 'vitest'

import { usageLogSchema, type UsageLog } from '../../data/schema'
import { LEGACY_TIMING_NOTE } from '../../lib/timing'
import type { LogOtherData } from '../../types'
import { useCommonLogsColumns } from '../columns/common-logs-columns'
import { CommonLogMobileCard } from '../common-log-mobile-card'
import { DetailsDialog } from '../dialogs/details-dialog'
import { StreamTpsCell } from '../timing-metrics-cell'
import { UsageLogsProvider } from '../usage-logs-provider'

const delivery = {
  version: 1,
  source: 'server_delivery',
  total_ms: 7109,
  ttft_ms: 7079,
  status: 'completed',
} as const
const log = usageLogSchema.parse({
  id: 1,
  user_id: 1,
  created_at: 1788840000,
  type: 2,
  model_name: 'test-model',
  content: '',
  use_time: 7,
  is_stream: true,
  prompt_tokens: 100,
  completion_tokens: 50,
})

const clients: QueryClient[] = []
afterEach(() => {
  for (const client of clients.splice(0)) client.clear()
})

function TimingViews(props: { log: UsageLog }) {
  const columns = useCommonLogsColumns(false, false)
  // oxlint-disable-next-line react/incompatible-library -- This test harness intentionally exercises the non-memoizable TanStack Table API.
  const table = useReactTable({
    data: [props.log],
    columns,
    getCoreRowModel: getCoreRowModel(),
  })
  const cells = new Map(
    table
      .getRowModel()
      .rows[0].getVisibleCells()
      .map((cell) => [cell.column.id, cell])
  )
  return (
    <>
      {['use_time', 'tps'].map((id) => {
        const cell = cells.get(id)
        if (!cell) throw new Error(`Missing timing column: ${id}`)
        return (
          <div key={id} data-testid={`desktop-${id}`}>
            {flexRender(cell.column.columnDef.cell, cell.getContext())}
          </div>
        )
      })}
      <div data-testid='mobile'>
        <CommonLogMobileCard log={props.log} cells={cells} />
      </div>
      <DetailsDialog
        log={props.log}
        isAdmin={false}
        isRoot={false}
        open
        onOpenChange={() => {}}
      />
    </>
  )
}

function renderViews(other: LogOtherData, stream = true) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  clients.push(client)
  render(
    <QueryClientProvider client={client}>
      <UsageLogsProvider>
        <TimingViews
          log={{ ...log, is_stream: stream, other: JSON.stringify(other) }}
        />
      </UsageLogsProvider>
    </QueryClientProvider>
  )
  return [
    screen.getByTestId('desktop-use_time'),
    screen.getByTestId('mobile'),
    screen.getByRole('dialog'),
  ]
}

it('renders matching delivery TTFT and duration in desktop, mobile and the real detail dialog', () => {
  const views = renderViews({
    delivery_timing: delivery,
    frt: 9999,
    bamboo_timing: { total_ms: 1000, ttft_ms: 500, output_tps: 80 },
  })
  for (const view of views) {
    expect(within(view).getByText('7.079s')).toBeVisible()
    expect(within(view).getByText('7.109s')).toBeVisible()
    expect(within(view).queryByText('7.0s')).not.toBeInTheDocument()
    expect(within(view).queryByText(LEGACY_TIMING_NOTE)).not.toBeInTheDocument()
  }
  const dialog = views[2]
  expect(within(dialog).getByText('Service-side delivery')).toBeVisible()
  expect(within(dialog).getByText('Completed')).toBeVisible()
  expect(within(dialog).getByText('Upstream timing')).toBeVisible()
  expect(within(dialog).getByText('80.0 t/s')).toBeVisible()
  expect(
    within(screen.getByTestId('desktop-tps')).getByText('N/A')
  ).toBeVisible()
  expect(
    within(screen.getByTestId('desktop-tps')).getByText('Upstream')
  ).toBeVisible()
})

it('preserves historical numbers and visibly annotates all timing views', () => {
  for (const view of renderViews({ frt: 7079 })) {
    expect(within(view).getByText('7.0s')).toBeVisible()
    expect(within(view).getByText('7.1s')).toBeVisible()
    expect(within(view).getByText(LEGACY_TIMING_NOTE)).toBeVisible()
  }
})

it.each([0, null])(
  'renders a zero TTFT distinctly from null (%s) across views',
  (first) => {
    for (const view of renderViews({
      delivery_timing: { ...delivery, ttft_ms: first },
      frt: 1200,
    })) {
      const label = within(view).getByText('First token')
      expect(label.parentElement).toHaveTextContent(
        first === null ? 'N/A' : '0.000s'
      )
      expect(within(view).queryByText('1.2s')).not.toBeInTheDocument()
    }
  }
)

it('renders zero total time in all views rather than hiding the detail timing', () => {
  for (const view of renderViews({
    delivery_timing: { ...delivery, total_ms: 0, ttft_ms: 0 },
  })) {
    expect(within(view).getAllByText('0.000s')).toHaveLength(2)
  }
  expect(screen.getByTestId('desktop-tps')).toHaveTextContent('N/A')
})

it('shows unavailable delivery metrics rather than legacy values for the invalid marker', () => {
  const views = renderViews({
    delivery_timing_invalid: true,
    delivery_timing: delivery,
    frt: 200,
    tps: 90,
  })
  for (const view of views) {
    expect(within(view).getByText('Duration').parentElement).toHaveTextContent(
      'N/A'
    )
    expect(
      within(view).getByText('First token').parentElement
    ).toHaveTextContent('N/A')
    expect(within(view).queryByText(LEGACY_TIMING_NOTE)).not.toBeInTheDocument()
    expect(within(view).queryByText('7.0s')).not.toBeInTheDocument()
  }
  expect(screen.getByTestId('desktop-tps')).toHaveTextContent('N/A')
})

it.each([true, false])(
  'uses delivery TPS in desktop, mobile and detail for stream=%s',
  (stream) => {
    const views = renderViews(
      {
        delivery_timing: { ...delivery, total_ms: 2000, ttft_ms: 1000 },
        tps: 888,
      },
      stream
    )
    const expected = stream ? '50.0' : '25.0'
    expect(screen.getByTestId('desktop-tps')).toHaveTextContent(expected)
    expect(within(views[1]).getByText(`${expected} t/s`)).toBeVisible()
    expect(within(views[2]).getByText(`${expected} t/s`)).toBeVisible()
  }
)

it('keeps every upstream hop separate from service-side delivery in the detail dialog', () => {
  const views = renderViews({
    delivery_timing: { ...delivery, status: 'write_error' },
    bamboo_timing: { total_ms: 800, output_tps: 200 },
    bamboo_timing_hops: [
      {
        hop_index: 1,
        total_ms: 1200,
        ttft_ms: 0,
        thinking_ms: 500,
        thinking_tps: -40,
        thinking_tokens: 20,
      },
      {
        hop_index: 2,
        total_ms: 800,
        ttft_ms: 300,
        content_ms: 400,
        tool_ms: 100,
        output_tps: 200,
        tool_tps: 30,
        output_tokens: 80,
        tool_tokens: 3,
      },
    ],
  })
  const dialog = views[2]
  expect(within(dialog).getByText('Write error')).toBeVisible()
  expect(within(dialog).getByText('Upstream hop 1')).toBeVisible()
  expect(within(dialog).getByText('Upstream hop 2')).toBeVisible()
  expect(within(dialog).getByText('1.200s')).toBeVisible()
  expect(within(dialog).getByText('0.800s')).toBeVisible()
  expect(within(dialog).getByText('~40.0 t/s')).toBeVisible()
  expect(within(dialog).getByText('200.0 t/s')).toBeVisible()
  expect(within(dialog).getByText('30.0 t/s')).toBeVisible()
  expect(within(dialog).getByText('7.109s')).toBeVisible()
})

it('shows a numeric zero rate distinctly from an unavailable rate', () => {
  render(
    <>
      <div data-testid='zero-rate'>
        <StreamTpsCell isStream deliveryTiming compact tokensPerSecond={0} />
      </div>
      <div data-testid='absent-rate'>
        <StreamTpsCell isStream deliveryTiming compact tokensPerSecond={null} />
      </div>
    </>
  )
  expect(screen.getByTestId('zero-rate')).toHaveTextContent('0.0 t/s')
  expect(screen.getByTestId('absent-rate')).toHaveTextContent('N/A')
})
