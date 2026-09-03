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
import type { SortField, ChartTab } from './types'

export const SORT_OPTIONS: { value: SortField; labelKey: string }[] = [
  { value: 'total_tokens', labelKey: 'Output Tokens' },
  { value: 'failed_rate', labelKey: 'Failure Rate' },
  { value: 'avg_tps', labelKey: 'Output TPS' },
]

export const CHART_TABS: { value: ChartTab; labelKey: string }[] = [
  { value: 'output_tokens', labelKey: 'Output Token Trend' },
  { value: 'tps', labelKey: 'TPS Trends' },
  { value: 'failure_rate', labelKey: 'Failure Rate Trend' },
]
