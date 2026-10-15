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
import { VChart } from '@visactor/react-vchart'
import { Activity, Gauge, Layers, ShieldAlert } from 'lucide-react'
import {
  type ReactNode,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
} from '@/components/ui/empty'
import { useTheme } from '@/context/theme-provider'
import { cn } from '@/lib/utils'
import { VCHART_OPTION } from '@/lib/vchart'

import { CHART_TABS } from '../constants'
import { buildStableModelColorMap } from '../lib/chart-colors'
import {
  buildEstimatedTpsChartSpec,
  buildLineChartSpec,
  transformChartData,
  transformEstimatedTpsData,
} from '../lib/chart-specs'
import type { ChartTab, TokenRecordRecentItem } from '../types'

interface ModelLogChartsProps {
  sortedItems: TokenRecordRecentItem[]
  selectedModels: Set<string>
  modelColorMap?: ReadonlyMap<string, string>
  locale?: string
}

interface MetricChartPanelProps {
  title: string
  description: string
  icon: ReactNode
  badge?: string
  badgeClassName?: string
  emptyMessage?: string
  children: ReactNode
}

function MetricChartPanel(props: MetricChartPanelProps) {
  const titleId = useId()

  return (
    <section
      aria-labelledby={titleId}
      className='bg-card/70 overflow-hidden rounded-xl border shadow-xs'
    >
      <div className='border-border/60 flex flex-col gap-2 border-b px-4 py-3 sm:flex-row sm:items-start sm:justify-between sm:px-5'>
        <div className='flex min-w-0 items-start gap-2.5'>
          <span className='bg-muted text-muted-foreground mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-lg'>
            {props.icon}
          </span>
          <div className='min-w-0'>
            <h3 id={titleId} className='text-sm font-semibold'>
              {props.title}
            </h3>
            <p className='text-muted-foreground mt-0.5 max-w-3xl text-xs leading-relaxed'>
              {props.description}
            </p>
          </div>
        </div>
        {props.badge && (
          <Badge
            variant='outline'
            className={cn('w-fit shrink-0', props.badgeClassName)}
          >
            {props.badge}
          </Badge>
        )}
      </div>
      <div className='h-[280px] p-1.5 sm:h-96 sm:p-2'>
        {props.emptyMessage ? (
          <Empty className='h-full'>
            <EmptyHeader>
              <EmptyMedia variant='icon'>{props.icon}</EmptyMedia>
              <EmptyDescription>{props.emptyMessage}</EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : (
          props.children
        )}
      </div>
    </section>
  )
}

export function ModelLogCharts(props: ModelLogChartsProps) {
  const { t } = useTranslation()
  const { resolvedTheme } = useTheme()
  const [activeTab, setActiveTab] = useState<ChartTab>('output_tokens')
  const targetTheme = resolvedTheme === 'dark' ? 'dark' : 'light'
  const [readyTheme, setReadyTheme] = useState<'dark' | 'light' | null>(null)
  const themeReady = readyTheme === targetTheme
  const themeManagerRef = useRef<
    (typeof import('@visactor/vchart'))['ThemeManager'] | null
  >(null)

  useEffect(() => {
    let cancelled = false
    const load = async () => {
      const vchart = await import('@visactor/vchart')
      if (cancelled) return
      themeManagerRef.current = vchart.ThemeManager
      vchart.ThemeManager.setCurrentTheme(targetTheme)
      setReadyTheme(targetTheme)
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [targetTheme])

  const modelNames = useMemo(
    () => props.sortedItems.map((item) => item.model_name),
    [props.sortedItems]
  )
  const resolvedColorMap = useMemo(
    () => props.modelColorMap ?? buildStableModelColorMap(modelNames),
    [props.modelColorMap, modelNames]
  )
  const colorDomain = useMemo(() => [...modelNames], [modelNames])
  const colorRange = useMemo(
    () => modelNames.map((name) => resolvedColorMap.get(name) ?? '#0ea5e9'),
    [modelNames, resolvedColorMap]
  )

  const { outputTokenData, averageTpsData, failureRateData } = useMemo(
    () => transformChartData(props.sortedItems, props.selectedModels),
    [props.sortedItems, props.selectedModels]
  )

  const hasOutputTokenData = outputTokenData.some(
    (point) => (point.SampleCount ?? 0) > 0 || (point.Value ?? 0) > 0
  )
  const hasFailureRateData = failureRateData.some(
    (point) => point.Value !== null
  )
  const hasFailedRequests = props.sortedItems.some(
    (item) =>
      props.selectedModels.has(item.model_name) &&
      item.cells.some((cell) => cell.failed_count > 0)
  )

  const phaseLabels = useMemo(
    () => ({
      thinking: t('Thinking'),
      output: t('Output'),
      tool: t('Tool'),
    }),
    [t]
  )
  const estimatedTpsData = useMemo(
    () =>
      transformEstimatedTpsData(
        props.sortedItems,
        props.selectedModels,
        phaseLabels
      ),
    [props.sortedItems, props.selectedModels, phaseLabels]
  )

  const isDark = resolvedTheme === 'dark'

  const specs = useMemo(
    () => ({
      output_tokens: buildLineChartSpec(
        'outputTokenData',
        outputTokenData,
        colorDomain,
        colorRange,
        {
          valueFormat: 'tokens',
          locale: props.locale,
          isDark,
        }
      ),
      average_tps: buildLineChartSpec(
        'averageTpsData',
        averageTpsData,
        colorDomain,
        colorRange,
        {
          valueFormat: 'tps',
          locale: props.locale,
          isDark,
        }
      ),
      estimated_tps: buildEstimatedTpsChartSpec(
        'estimatedTpsData',
        estimatedTpsData,
        resolvedColorMap,
        {
          locale: props.locale,
          isDark,
        }
      ),
      failure_rate: buildLineChartSpec(
        'failureRateData',
        failureRateData,
        colorDomain,
        colorRange,
        {
          valueFormat: 'percent',
          locale: props.locale,
          isDark,
        }
      ),
    }),
    [
      outputTokenData,
      averageTpsData,
      estimatedTpsData,
      failureRateData,
      colorDomain,
      colorRange,
      resolvedColorMap,
      props.locale,
      isDark,
    ]
  )

  const chartTheme = resolvedTheme === 'dark' ? 'dark' : 'light'
  const renderChart = (spec: typeof specs.output_tokens, key: string) =>
    themeReady && (
      <VChart
        key={`${key}-${resolvedTheme}`}
        spec={{
          ...spec,
          theme: chartTheme,
          background: 'transparent',
        }}
        option={VCHART_OPTION}
      />
    )

  return (
    <div>
      <div className='bg-muted/60 mb-3 inline-flex h-7 w-full overflow-x-auto rounded-lg border p-0.5 sm:h-8 sm:w-auto'>
        {CHART_TABS.map((tab) => (
          <button
            key={tab.value}
            type='button'
            aria-pressed={activeTab === tab.value}
            onClick={() => setActiveTab(tab.value)}
            className={`shrink-0 rounded-md px-3 text-xs font-medium transition-colors ${
              activeTab === tab.value
                ? 'bg-background text-foreground shadow-sm'
                : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            {t(tab.labelKey)}
          </button>
        ))}
      </div>

      {activeTab === 'output_tokens' && (
        <MetricChartPanel
          title={t('Output Token Trend')}
          description={t(
            'Hourly output token volume for each model in the selected time range.'
          )}
          icon={<Layers className='size-4' aria-hidden='true' />}
          emptyMessage={
            hasOutputTokenData
              ? undefined
              : t('No output token data in the selected time range.')
          }
        >
          {renderChart(specs.output_tokens, 'output-tokens')}
        </MetricChartPanel>
      )}

      {activeTab === 'tps' && (
        <div className='flex flex-col gap-4'>
          <MetricChartPanel
            title={t('Average TPS Trend')}
            description={t(
              'Average TPS uses the total output tokens and request duration recorded for each model.'
            )}
            icon={<Gauge className='size-4' aria-hidden='true' />}
            emptyMessage={
              averageTpsData.some((point) => point.Value !== null)
                ? undefined
                : t('No average TPS data in the selected time range.')
            }
          >
            {renderChart(specs.average_tps, 'average-tps')}
          </MetricChartPanel>

          <MetricChartPanel
            title={t('Estimated Phase TPS Trend')}
            description={t(
              'Thinking, output, and tool rates are estimated from streamed content and shown separately from the average.'
            )}
            icon={<Activity className='size-4' aria-hidden='true' />}
            badge={t('Estimated')}
            badgeClassName='border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300'
            emptyMessage={
              estimatedTpsData.length > 0
                ? undefined
                : t('No estimated phase TPS data in the selected time range.')
            }
          >
            {renderChart(specs.estimated_tps, 'estimated-tps')}
          </MetricChartPanel>
        </div>
      )}

      {activeTab === 'failure_rate' && (
        <MetricChartPanel
          title={t('Failure Rate Trend')}
          description={t(
            'Hourly request failure rates for each model help identify model or upstream issues.'
          )}
          icon={<ShieldAlert className='size-4' aria-hidden='true' />}
          badge={
            hasFailureRateData && !hasFailedRequests
              ? t('No failed requests')
              : t('Percentage')
          }
          badgeClassName={cn(
            hasFailureRateData && !hasFailedRequests && 'bg-muted/60'
          )}
          emptyMessage={
            hasFailureRateData
              ? undefined
              : t('No request data in the selected time range.')
          }
        >
          {renderChart(specs.failure_rate, 'failure-rate')}
        </MetricChartPanel>
      )}
    </div>
  )
}
