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
import { Activity, Gauge } from 'lucide-react'
import { type ReactNode, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { useTheme } from '@/context/theme-provider'
import { cn } from '@/lib/utils'
import { VCHART_OPTION } from '@/lib/vchart'

import { CHART_TABS } from '../constants'
import { getChartColors } from '../lib/chart-colors'
import {
  buildLineChartSpec,
  transformChartData,
  transformEstimatedTpsData,
} from '../lib/chart-specs'
import type { ChartTab, TokenRecordRecentItem } from '../types'

const ESTIMATED_PHASE_COLORS = ['#8b5cf6', '#0ea5e9', '#f59e0b']

interface ModelLogChartsProps {
  sortedItems: TokenRecordRecentItem[]
  selectedModels: Set<string>
}

interface TpsChartPanelProps {
  title: string
  description: string
  icon: ReactNode
  badge?: string
  badgeClassName?: string
  children: ReactNode
}

function TpsChartPanel(props: TpsChartPanelProps) {
  return (
    <section className='bg-card/70 overflow-hidden rounded-xl border shadow-xs'>
      <div className='border-border/60 flex flex-col gap-2 border-b px-4 py-3 sm:flex-row sm:items-start sm:justify-between sm:px-5'>
        <div className='flex min-w-0 items-start gap-2.5'>
          <span className='bg-muted text-muted-foreground mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-lg'>
            {props.icon}
          </span>
          <div className='min-w-0'>
            <h3 className='text-sm font-semibold'>{props.title}</h3>
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
      {props.children}
    </section>
  )
}

export function ModelLogCharts(props: ModelLogChartsProps) {
  const { t } = useTranslation()
  const { resolvedTheme } = useTheme()
  const [activeTab, setActiveTab] = useState<ChartTab>('output_tokens')
  const [themeReady, setThemeReady] = useState(false)
  const themeManagerRef = useRef<
    (typeof import('@visactor/vchart'))['ThemeManager'] | null
  >(null)

  useEffect(() => {
    let cancelled = false
    const load = async () => {
      const vchart = await import('@visactor/vchart')
      if (cancelled) return
      themeManagerRef.current = vchart.ThemeManager
      vchart.ThemeManager.setCurrentTheme(
        resolvedTheme === 'dark' ? 'dark' : 'light'
      )
      setThemeReady(true)
    }
    setThemeReady(false)
    void load()
    return () => {
      cancelled = true
    }
  }, [resolvedTheme])

  const modelNames = useMemo(
    () => props.sortedItems.map((item) => item.model_name),
    [props.sortedItems]
  )
  const colorDomain = useMemo(() => [...modelNames], [modelNames])
  const colorRange = useMemo(
    () => getChartColors(modelNames.length),
    [modelNames.length]
  )

  const { outputTokenData, averageTpsData, failureRateData } = useMemo(
    () => transformChartData(props.sortedItems, props.selectedModels),
    [props.sortedItems, props.selectedModels]
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

  const specs = useMemo(
    () => ({
      output_tokens: buildLineChartSpec(
        'outputTokenData',
        outputTokenData,
        colorDomain,
        colorRange
      ),
      average_tps: buildLineChartSpec(
        'averageTpsData',
        averageTpsData,
        colorDomain,
        colorRange
      ),
      estimated_tps: buildLineChartSpec(
        'estimatedTpsData',
        estimatedTpsData,
        [phaseLabels.thinking, phaseLabels.output, phaseLabels.tool],
        ESTIMATED_PHASE_COLORS
      ),
      failure_rate: buildLineChartSpec(
        'failureRateData',
        failureRateData,
        colorDomain,
        colorRange
      ),
    }),
    [
      outputTokenData,
      averageTpsData,
      estimatedTpsData,
      failureRateData,
      colorDomain,
      colorRange,
      phaseLabels,
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

      {activeTab === 'tps' ? (
        <div className='space-y-4'>
          <TpsChartPanel
            title={t('Average TPS Trend')}
            description={t(
              'Average TPS uses the total output tokens and request duration recorded for each model.'
            )}
            icon={<Gauge className='size-4' aria-hidden='true' />}
          >
            <div className='h-[280px] p-1.5 sm:h-96 sm:p-2'>
              {renderChart(specs.average_tps, 'average-tps')}
            </div>
          </TpsChartPanel>

          <TpsChartPanel
            title={t('Estimated Phase TPS Trend')}
            description={t(
              'Thinking, output, and tool rates are estimated from streamed content and shown separately from the average.'
            )}
            icon={<Activity className='size-4' aria-hidden='true' />}
            badge={t('Estimated')}
            badgeClassName='border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300'
          >
            {estimatedTpsData.length > 0 ? (
              <div className='h-[280px] p-1.5 sm:h-96 sm:p-2'>
                {renderChart(specs.estimated_tps, 'estimated-tps')}
              </div>
            ) : (
              <div className='text-muted-foreground flex min-h-44 items-center justify-center px-6 text-center text-sm'>
                {t('No estimated phase TPS data in the selected time range.')}
              </div>
            )}
          </TpsChartPanel>
        </div>
      ) : (
        <div className='h-[300px] p-1.5 sm:h-96 sm:p-2'>
          {activeTab === 'output_tokens'
            ? renderChart(specs.output_tokens, activeTab)
            : renderChart(specs.failure_rate, activeTab)}
        </div>
      )}
    </div>
  )
}
