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
export const TOOL_KIND_ALL = 'all'
export const TOOL_STATUS_ALL = 'all'

export const TOOL_KIND_FILTERS = [
  { value: TOOL_KIND_ALL, label: 'All Types' },
  { value: 'search', label: 'Search' },
  { value: 'fetch', label: 'Fetch' },
] as const

export const TOOL_STATUS_FILTERS = [
  { value: TOOL_STATUS_ALL, label: 'All Statuses' },
  { value: 'ok', label: 'Success' },
  { value: 'error', label: 'Failed' },
] as const

export const DEFAULT_TOOL_LOGS_DATA = {
  items: [] as never[],
  total: 0,
}
