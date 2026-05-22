import { useEffect, useMemo, useState, useRef } from 'react'
import { VChart } from '@visactor/react-vchart'
import { useTranslation } from 'react-i18next'
import { useTheme } from '@/context/theme-provider'
import { VCHART_OPTION } from '@/lib/vchart'
import { CHART_TABS } from '../constants'
import {
  transformChartData,
  buildLineChartSpec,
} from '../lib/chart-specs'
import type {
  ChartTab,
  TokenRecordRecentItem,
  ChartDataPoint,
} from '../types'

// Reuse theme chart colors from dashboard
const THEME_CHART_COLOR_VARIABLES = [
  '--chart-1',
  '--chart-2',
  '--chart-3',
  '--chart-4',
  '--chart-5',
] as const

function getThemeChartColors(): string[] {
  if (typeof document === 'undefined') return []
  const bodyStyle = window.getComputedStyle(document.body)
  const rootStyle = window.getComputedStyle(document.documentElement)
  return THEME_CHART_COLOR_VARIABLES.map((name) =>
    (
      bodyStyle.getPropertyValue(name) || rootStyle.getPropertyValue(name)
    ).trim()
  ).filter(Boolean)
}

function getChartColors(count: number): string[] {
  const themeColors = getThemeChartColors()
  if (themeColors.length > 0) {
    return Array.from(
      { length: count },
      (_, index) => themeColors[index % themeColors.length]
    )
  }
  // Fallback: generate from HSL
  return Array.from({ length: count }, (_, i) => {
    const hue = (i * 360) / count
    return `hsl(${hue}, 65%, 55%)`
  })
}

interface ModelLogChartsProps {
  sortedItems: TokenRecordRecentItem[]
  selectedModels: Set<string>
}

export function ModelLogCharts({
  sortedItems,
  selectedModels,
}: ModelLogChartsProps) {
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
    () => sortedItems.map((item) => item.model_name),
    [sortedItems]
  )

  const colorDomain = useMemo(() => [...modelNames], [modelNames])
  const colorRange = useMemo(
    () => getChartColors(modelNames.length),
    [modelNames.length]
  )

  const { outputTokenData, tpsData, failureRateData } = useMemo(
    () => transformChartData(sortedItems, selectedModels),
    [sortedItems, selectedModels]
  )

  const specs = useMemo(() => {
    return {
      output_tokens: buildLineChartSpec(
        'outputTokenData',
        outputTokenData,
        colorDomain,
        colorRange
      ),
      tps: buildLineChartSpec(
        'tpsData',
        tpsData,
        colorDomain,
        colorRange
      ),
      failure_rate: buildLineChartSpec(
        'failureRateData',
        failureRateData,
        colorDomain,
        colorRange
      ),
    }
  }, [outputTokenData, tpsData, failureRateData, colorDomain, colorRange])

  const currentSpec = specs[activeTab]
  const dataKey = `${activeTab}-${sortedItems.length}-${resolvedTheme}`

  return (
    <div>
      <div className='bg-muted/60 mb-2 inline-flex h-7 w-full overflow-x-auto rounded-lg border p-0.5 sm:h-8 sm:w-auto'>
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
      <div className='h-[300px] p-1.5 sm:h-96 sm:p-2'>
        {themeReady && currentSpec && (
          <VChart
            key={dataKey}
            spec={{
              ...currentSpec,
              theme: resolvedTheme === 'dark' ? 'dark' : 'light',
              background: 'transparent',
            }}
            option={VCHART_OPTION}
          />
        )}
      </div>
    </div>
  )
}
