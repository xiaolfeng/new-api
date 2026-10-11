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
import {
  afterAll,
  afterEach,
  beforeEach,
  describe,
  expect,
  test,
  vi,
} from 'vitest'

import en from '@/i18n/locales/en.json'

import { getLogDetail } from '../../api'
import type { UsageLog } from '../../data/schema'
import { ensureUniqueLogRowKeys, getLogRowId } from '../../lib/utils'
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
    log_entry_id: '',
    row_key: '',
    upstream_request_id: 'up-req-456',
    record: '',
    full_log: '',
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

function TableWrapper({ log, logs }: { log?: UsageLog; logs?: UsageLog[] }) {
  const columns = useCommonLogsColumns(true, true)
  const data = ensureUniqueLogRowKeys(logs ?? (log ? [log] : []))
  // oxlint-disable-next-line react/incompatible-library
  const table = useReactTable({
    data,
    columns,
    getRowId: (row, index) => getLogRowId(row, index),
    getCoreRowModel: getCoreRowModel(),
  })

  return (
    <UsageLogsProvider>
      <DetailOpenProbe />
      {table.getRowModel().rows.map((row) => (
        <div
          key={row.id}
          data-testid='row'
          data-row-id={row.id}
          data-log-type={String(row.original.type)}
        >
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

    fireEvent.click(screen.getByTitle('Click to view full details'))

    await waitFor(() => {
      expect(screen.getByTestId('detail-open')).toHaveTextContent('true')
    })
  })

  test('passes row_key to getLogDetail for new logs and replaces shared-request_id retry rows without stale leftovers', async () => {
    const getLogDetailMock = vi.mocked(getLogDetail)
    getLogDetailMock.mockClear()

    const retryErr1: UsageLog = {
      ...makeTestLog(),
      id: 1,
      type: 5,
      channel: 10,
      content: 'channel 10 timeout',
      request_id: 'req-shared-999',
      log_entry_id: 'le_err_1',
      row_key: 'le_err_1',
    }
    const retryErr2: UsageLog = {
      ...makeTestLog(),
      id: 2,
      type: 5,
      channel: 10,
      content: 'channel 10 timeout',
      request_id: 'req-shared-999',
      log_entry_id: 'le_err_2',
      row_key: 'le_err_2',
    }
    const finalConsume: UsageLog = {
      ...makeTestLog(),
      id: 3,
      type: 2,
      channel: 11,
      content: 'completed',
      request_id: 'req-shared-999',
      log_entry_id: 'le_ok_3',
      row_key: 'le_ok_3',
    }

    const { rerender } = render(
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <TableWrapper logs={[retryErr1, retryErr2, finalConsume]} />
        </QueryClientProvider>
      </I18nextProvider>
    )

    const initialRows = screen.getAllByTestId('row')
    expect(initialRows).toHaveLength(3)
    expect(initialRows.map((el) => el.getAttribute('data-row-id'))).toEqual([
      'le_err_1',
      'le_err_2',
      'le_ok_3',
    ])

    // Simulate search response replacing the list with only the consume row
    rerender(
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <TableWrapper logs={[finalConsume]} />
        </QueryClientProvider>
      </I18nextProvider>
    )

    const filteredRows = screen.getAllByTestId('row')
    expect(filteredRows).toHaveLength(1)
    expect(filteredRows[0].getAttribute('data-row-id')).toBe('le_ok_3')
    expect(filteredRows[0].getAttribute('data-log-type')).toBe('2')

    fireEvent.click(screen.getByTitle('Click to view full details'))
    await waitFor(() => {
      expect(getLogDetailMock).toHaveBeenCalledWith('req-shared-999', 'le_ok_3')
    })

    // Simulate empty search response
    rerender(
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <TableWrapper logs={[]} />
        </QueryClientProvider>
      </I18nextProvider>
    )
    expect(screen.queryAllByTestId('row')).toHaveLength(0)
  })
})
