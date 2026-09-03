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
import { ArrowDown, ArrowUp, RefreshCw, BarChart3 } from 'lucide-react'
import { useState, useMemo, useCallback } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'

import { ModelFilter } from './components/model-filter'
import { ModelLogCharts } from './components/model-log-charts'
import { SummaryCards } from './components/summary-cards'
import { TokenHeatmap } from './components/token-heatmap'
import { SORT_OPTIONS } from './constants'
import { useModelLogData } from './hooks/use-model-log-data'
import { getChartColors } from './lib/chart-colors'
import type { SortField, TokenRecordRecentItem } from './types'

const TIME_RANGE_OPTIONS = [
  { value: '1', labelKey: '1h' },
  { value: '6', labelKey: '6h' },
  { value: '24', labelKey: '24h' },
  { value: '168', labelKey: '7d' },
]

function sortItems(
  items: TokenRecordRecentItem[],
  sortField: SortField,
  sortDirection: 'asc' | 'desc'
): TokenRecordRecentItem[] {
  const sorted = [...items]
  const multiplier = sortDirection === 'asc' ? 1 : -1

  sorted.sort((a, b) => {
    let aVal: number, bVal: number

    switch (sortField) {
      case 'total_tokens':
        aVal = a.summary.completion_tokens || 0
        bVal = b.summary.completion_tokens || 0
        break
      case 'failed_rate':
        aVal = a.summary.failed_rate || 0
        bVal = b.summary.failed_rate || 0
        break
      case 'avg_tps':
        aVal = a.summary.avg_tps || 0
        bVal = b.summary.avg_tps || 0
        break
      default:
        return 0
    }

    if (aVal === bVal) {
      return a.model_name.localeCompare(b.model_name) * multiplier
    }
    return (aVal - bVal) * multiplier
  })

  return sorted
}

const SUMMARY_SKELETON_KEYS = ['requests', 'input', 'output', 'models']

function SummaryCardsSkeleton() {
  return (
    <div className='grid grid-cols-2 gap-3 lg:grid-cols-4'>
      {SUMMARY_SKELETON_KEYS.map((key) => (
        <div
          key={key}
          className='rounded-xl border bg-sky-50/70 p-3.5 dark:bg-sky-950/25'
        >
          <Skeleton className='h-3 w-20' />
          <Skeleton className='mt-2 h-7 w-24' />
        </div>
      ))}
    </div>
  )
}

export function ModelLogPage() {
  const { t } = useTranslation()
  const {
    loading,
    refreshing,
    items,
    summary,
    lastUpdatedAt,
    refreshData,
    timeRange,
    setTimeRange,
    autoRefresh,
    setAutoRefresh,
    countdown,
  } = useModelLogData()

  const [sortField, setSortField] = useState<SortField>('total_tokens')
  const [sortDirection, setSortDirection] = useState<'asc' | 'desc'>('desc')
  const [selectedModels, setSelectedModels] = useState<Set<string> | null>(null)

  const sortedItems = useMemo(
    () => sortItems(items, sortField, sortDirection),
    [items, sortField, sortDirection]
  )

  const modelNames = useMemo(
    () => sortedItems.map((item) => item.model_name),
    [sortedItems]
  )

  const effectiveSelected = useMemo(() => {
    if (selectedModels !== null) return selectedModels
    return new Set(modelNames)
  }, [selectedModels, modelNames])

  const colorRange = useMemo(
    () => getChartColors(modelNames.length),
    [modelNames.length]
  )

  const modelColorMap = useMemo(() => {
    const map = new Map<string, string>()
    modelNames.forEach((name, idx) => {
      map.set(name, colorRange[idx])
    })
    return map
  }, [modelNames, colorRange])

  const handleToggleModel = useCallback(
    (model: string) => {
      setSelectedModels((prev) => {
        const base = prev || new Set(modelNames)
        const next = new Set(base)
        if (next.has(model)) {
          next.delete(model)
        } else {
          next.add(model)
        }
        return next
      })
    },
    [modelNames]
  )

  const handleSelectAll = useCallback(() => {
    setSelectedModels(new Set(modelNames))
  }, [modelNames])

  const handleDeselectAll = useCallback(() => {
    setSelectedModels(new Set())
  }, [])

  const handleSort = (field: SortField | null) => {
    if (!field) return
    if (field === sortField) {
      setSortDirection((prev) => (prev === 'asc' ? 'desc' : 'asc'))
    } else {
      setSortField(field)
      setSortDirection('desc')
    }
  }

  let summaryContent = null
  if (loading) {
    summaryContent = <SummaryCardsSkeleton />
  } else if (summary) {
    summaryContent = <SummaryCards summary={summary} />
  }

  let modelContent
  if (loading) {
    modelContent = (
      <div className='flex justify-center py-16'>
        <RefreshCw className='text-muted-foreground size-8 animate-spin' />
      </div>
    )
  } else if (items.length === 0) {
    modelContent = (
      <div className='text-muted-foreground flex justify-center py-16 text-sm'>
        {t('No model log data in the selected time range')}
      </div>
    )
  } else {
    modelContent = (
      <div className='space-y-3'>
        <ModelFilter
          models={modelNames}
          selectedModels={effectiveSelected}
          onToggleModel={handleToggleModel}
          onSelectAll={handleSelectAll}
          onDeselectAll={handleDeselectAll}
          modelColorMap={modelColorMap}
        />
        <ModelLogCharts
          sortedItems={sortedItems}
          selectedModels={effectiveSelected}
        />
      </div>
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <div className='flex items-center gap-2'>
          <BarChart3 className='text-muted-foreground/60 size-4' />
          {t('Model Log')}
        </div>
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <div className='flex flex-wrap items-center gap-2'>
          {lastUpdatedAt > 0 && (
            <span className='text-muted-foreground text-xs'>
              {t('Last refreshed')}:{' '}
              {new Date(lastUpdatedAt * 1000).toLocaleString()}
            </span>
          )}
          <Select
            value={String(timeRange)}
            onValueChange={(value) => setTimeRange(Number(value))}
          >
            <SelectTrigger className='h-8 w-[90px]'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {TIME_RANGE_OPTIONS.map((opt) => (
                <SelectItem key={opt.value} value={opt.value}>
                  {opt.labelKey}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={sortField} onValueChange={handleSort}>
            <SelectTrigger className='h-8 w-[130px]'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {SORT_OPTIONS.map((opt) => (
                <SelectItem key={opt.value} value={opt.value}>
                  {t(opt.labelKey)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button
            variant='outline'
            size='icon'
            className='size-8'
            onClick={() =>
              setSortDirection((prev) => (prev === 'asc' ? 'desc' : 'asc'))
            }
          >
            {sortDirection === 'asc' ? (
              <ArrowUp className='size-4' />
            ) : (
              <ArrowDown className='size-4' />
            )}
          </Button>
          <Badge variant='secondary'>
            {t('Model Count')} {items.length}
          </Badge>
          <Button
            variant='outline'
            size='sm'
            onClick={refreshData}
            disabled={refreshing}
          >
            <RefreshCw
              className={`mr-1.5 size-3.5 ${refreshing ? 'animate-spin' : ''}`}
            />
            {t('Refresh')}
          </Button>
          <div className='flex items-center gap-1.5'>
            <Switch
              size='sm'
              checked={autoRefresh}
              onCheckedChange={setAutoRefresh}
            />
            <span className='text-muted-foreground text-xs'>
              {t('Auto refresh')}
            </span>
            {autoRefresh && (
              <span className='text-muted-foreground/60 font-mono text-xs tabular-nums'>
                {countdown}s
              </span>
            )}
          </div>
        </div>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          {summaryContent}

          {!loading && (
            <div className='mt-4'>
              <TokenHeatmap />
            </div>
          )}

          {modelContent}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
