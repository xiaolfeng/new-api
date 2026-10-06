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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterAll, afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import en from '@/i18n/locales/en.json'
import type { UsageLog } from '../../data/schema'
import { useCommonLogsColumns } from '../columns/common-logs-columns'
import { useUsageLogsContext, UsageLogsProvider } from '../usage-logs-provider'

vi.mock('@lobehub/icons', () => ({}))
vi.mock('../../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../api')>()
  return {
    ...actual,
    getLogDetail: vi.fn(async () => ({ success: true, data: {} })),
  }
})
vi.hoisted(() => {
  vi.stubGlobal('localStorage', {
    getItem: () => null,
    setItem: () => undefined,
    removeItem: () => undefined,
  })
})
afterAll(() => vi.unstubAllGlobals())

function makeTestLog(): UsageLog {
  return {
    id: 1,
    user_id: 1,
    created_at: 1700000000,
    type: 2,
    content: 'hello world',
    username: 'test_user',
    token_name: 'test_token',
    model_name: 'claude-3-5-sonnet',
    quota: 1000,
    prompt_tokens: 100,
    completion_tokens: 50,
    use_time: 1.2,
    is_stream: true,
    channel: 1,
    channel_name: 'Main Channel',
    token_id: 1,
    group: 'default',
    ip: '127.0.0.1',
    other: '{}',
    request_id: 'req-abc-123',
    upstream_request_id: 'up-req-456',
  }
}

let client: QueryClient
const i18n = createInstance()

beforeEach(async () => {
  await i18n.init({
    lng: 'en',
    resources: { en },
    interpolation: { escapeValue: false },
  })
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
})

afterEach(() => {
  client.clear()
})

function DetailOpenProbe() {
  const { isDetailOpen } = useUsageLogsContext()
  return <span data-testid='detail-open'>{String(isDetailOpen)}</span>
}

function TableWrapper({ log }: { log: UsageLog }) {
  const columns = useCommonLogsColumns(true, true)
  // oxlint-disable-next-line react/incompatible-library
  const table = useReactTable({
    data: [log],
    columns,
    getCoreRowModel: getCoreRowModel(),
  })

  return (
    <UsageLogsProvider>
      <DetailOpenProbe />
      {table.getRowModel().rows.map((row) => (
        <div key={row.id} data-testid='row'>
          {row.getVisibleCells().map((cell) => (
            <div key={cell.id} data-testid={`cell-${cell.column.id}`}>
              {flexRender(cell.column.columnDef.cell, cell.getContext())}
            </div>
          ))}
        </div>
      ))}
    </UsageLogsProvider>
  )
}

describe('usage-logs detail open pauses auto refresh', () => {
  test('opening the details dialog flips the provider pause flag', async () => {
    render(
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <TableWrapper log={makeTestLog()} />
        </QueryClientProvider>
      </I18nextProvider>
    )

    expect(screen.getByTestId('detail-open')).toHaveTextContent('false')

    fireEvent.click(
      screen.getByTitle('Click to view full details')
    )

    await waitFor(() => {
      expect(screen.getByTestId('detail-open')).toHaveTextContent('true')
    })
  })
})
