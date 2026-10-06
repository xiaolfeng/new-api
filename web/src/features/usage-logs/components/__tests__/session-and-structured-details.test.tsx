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
import { fireEvent, render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterAll, afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import en from '@/i18n/locales/en.json'
import { getLogDetail } from '../../api'
import type { UsageLog } from '../../data/schema'
import { useCommonLogsColumns } from '../columns/common-logs-columns'
import { DetailsDialog } from '../dialogs/details-dialog'
import { UsageLogsProvider } from '../usage-logs-provider'

vi.mock('@lobehub/icons', () => ({}))
// 详情大字段已按需经 /api/log/detail 获取，测试直接 mock 端点返回。
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

function makeTestLog(partial: Partial<UsageLog>): UsageLog {
  return {
    id: 1,
    user_id: 1,
    created_at: 1700000000,
    type: 2,
    content: '',
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
    ...partial,
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
      <div>
        {table.getHeaderGroups().map((headerGroup) => (
          <div key={headerGroup.id} data-testid='headers'>
            {headerGroup.headers.map((header) => (
              <span key={header.id} data-testid={`col-${header.id}`}>
                {flexRender(header.column.columnDef.header, header.getContext())}
              </span>
            ))}
          </div>
        ))}
        {table.getRowModel().rows.map((row) => (
          <div key={row.id} data-testid='row'>
            {row.getVisibleCells().map((cell) => (
              <div key={cell.id} data-testid={`cell-${cell.column.id}`}>
                {flexRender(cell.column.columnDef.cell, cell.getContext())}
              </div>
            ))}
          </div>
        ))}
      </div>
    </UsageLogsProvider>
  )
}

describe('usage-logs common columns source and session regression tests', () => {
  test('columns include source and session columns', () => {
    const log = makeTestLog({
      other: JSON.stringify({
        client_source: 'Pi',
        session_name: 'Swift-Falcon',
        session_id: 'sess-123',
      }),
    })

    render(
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <TableWrapper log={log} />
        </QueryClientProvider>
      </I18nextProvider>
    )

    // Should have source column and session column in headers
    expect(screen.getByTestId('col-source')).toBeInTheDocument()
    expect(screen.getByTestId('col-session')).toBeInTheDocument()

    // Should render source cell with 'Pi'
    const sourceCell = screen.getByTestId('cell-source')
    expect(sourceCell).toHaveTextContent('Pi')

    // Should render session cell with 'Swift-Falcon'
    const sessionCell = screen.getByTestId('cell-session')
    expect(sessionCell).toHaveTextContent('Swift-Falcon')
  })
})

describe('DetailsDialog session and structured record regression tests', () => {
  test('displays Session Info section when session data exists', () => {
    const log = makeTestLog({
      other: JSON.stringify({
        client_source: 'Claude Code',
        session_name: 'Brave-Lion',
        session_id: 'session-xyz-789',
        agent_name: 'Code-Agent',
      }),
    })

    render(
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <DetailsDialog
            log={log}
            isAdmin
            isRoot
            open
            onOpenChange={() => {}}
          />
        </QueryClientProvider>
      </I18nextProvider>
    )

    // DetailsDialog should have Session Info section
    expect(screen.getByText('Session Info')).toBeInTheDocument()
    expect(screen.getByText('Brave-Lion')).toBeInTheDocument()
    expect(screen.getByText('session-xyz-789')).toBeInTheDocument()
    expect(screen.getByText('Code-Agent')).toBeInTheDocument()
  })

  test('structured consumption record details displays headers and tool calls instead of only raw json', async () => {
    const recordPayload = {
      headers: {
        'user-agent': 'pi/0.99.0 darwin',
        'x-session-id': 'sess-999',
      },
      openaiRequestBlocks: [
        { type: 'text', role: 'user', text: 'Hello, what is 2+2?' },
      ],
      openaiResponseBlocks: [
        {
          id: 'call_1',
          type: 'tool_call',
          name: 'calculator',
          arguments: { expr: '2+2' },
        },
      ],
    }

    const log = makeTestLog({})
    // 列表响应已剥离大字段，详情经端点按需返回。
    vi.mocked(getLogDetail).mockResolvedValue({
      success: true,
      data: { ...log, record: JSON.stringify(recordPayload) },
    })

    render(
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <DetailsDialog
            log={log}
            isAdmin
            isRoot
            open
            onOpenChange={() => {}}
          />
        </QueryClientProvider>
      </I18nextProvider>
    )

    // Click "View Record" button（详情端点异步返回后出现）
    const viewRecordButton = await screen.findByRole('button', { name: /View Record/i })
    expect(viewRecordButton).toBeInTheDocument()
    fireEvent.click(viewRecordButton)

    // Should open the sheet titled "Consumption Record Details"
    expect(screen.getByText('Consumption Record Details')).toBeInTheDocument()

    // It should render structured visual sections
    expect(screen.getByText('Request Headers')).toBeInTheDocument()
    expect(screen.getByText('user-agent')).toBeInTheDocument()
    expect(screen.getAllByText('pi/0.99.0 darwin').length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText('Tool Calls (1)')).toBeInTheDocument()
    expect(screen.getByText('calculator')).toBeInTheDocument()
  })
})
