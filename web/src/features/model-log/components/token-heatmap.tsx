import { useEffect, useState, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { api } from '@/lib/api'
import dayjs from '@/lib/dayjs'
import { toIntlLocale } from '@/i18n/languages'

import type { TokenRecordDailyItem } from '../types'

interface DayCell {
  date: string
  tokens: number
  promptTokens: number
  completionTokens: number
  level: number
}

const HEATMAP_COLORS = [
  'bg-[#ebedf0] dark:bg-[#161b22]',
  'bg-[#9be9a8] dark:bg-[#0e4429]',
  'bg-[#40c463] dark:bg-[#006d32]',
  'bg-[#30a14e] dark:bg-[#26a641]',
  'bg-[#216e39] dark:bg-[#39d353]',
]

function getQuantileThresholds(values: number[]): number[] {
  const sorted = [...values].filter((v) => v > 0).sort((a, b) => a - b)
  if (sorted.length === 0) return [0, 0, 0, 0]

  const len = sorted.length
  const q1 = sorted[Math.floor(len * 0.25)] ?? sorted[0] ?? 0
  const q2 = sorted[Math.floor(len * 0.5)] ?? sorted[0] ?? 0
  const q3 = sorted[Math.floor(len * 0.75)] ?? sorted[0] ?? 0
  const max = sorted[len - 1] ?? 0

  return [q1, q2, q3, max]
}

function getLevel(value: number, thresholds: number[]): number {
  if (value <= 0) return 0
  if (value <= thresholds[0]) return 1
  if (value <= thresholds[1]) return 2
  if (value <= thresholds[2]) return 3
  return 4
}

export function TokenHeatmap() {
  const { t, i18n } = useTranslation()
  const [data, setData] = useState<TokenRecordDailyItem[]>([])
  const [loading, setLoading] = useState(true)
  const [hovered, setHovered] = useState<DayCell | null>(null)
  const [tooltipPos, setTooltipPos] = useState<{
    x: number
    y: number
    showBelow: boolean
  } | null>(null)

  useEffect(() => {
    let cancelled = false
    const fetchData = async () => {
      setLoading(true)
      try {
        const res = await api.get('/api/token_record/daily')
        if (!cancelled && res.data?.success !== false) {
          const payload = res.data?.data ?? res.data
          const raw = Array.isArray(payload) ? payload : []
          // Normalize date to YYYY-MM-DD (PostgreSQL may return "2026-03-26T00:00:00Z")
          const normalized = raw.map((item) => ({
            ...item,
            date: item.date ? dayjs(item.date).format('YYYY-MM-DD') : item.date,
          }))
          setData(normalized)
        }
      } catch (err) {
        if (!cancelled) {
          console.warn('Failed to fetch token heatmap data', err)
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    fetchData()
    return () => {
      cancelled = true
    }
  }, [])

  const totalTokens = useMemo(
    () => data.reduce((sum, item) => sum + (item.total_tokens || 0), 0),
    [data]
  )

  const { weeks, monthLabels } = useMemo(() => {
    const dataMap = new Map<string, TokenRecordDailyItem>()
    data.forEach((item) => {
      dataMap.set(item.date, item)
    })

    const allValues = data.map((item) => item.total_tokens || 0)
    const thresholds = getQuantileThresholds(allValues)

    const today = new Date()
    const thisMonday = new Date(today)
    const todayDayOfWeek = today.getDay()
    const daysToMonday = todayDayOfWeek === 0 ? 6 : todayDayOfWeek - 1
    thisMonday.setDate(thisMonday.getDate() - daysToMonday)

    const startDate = new Date(thisMonday)
    startDate.setDate(startDate.getDate() - 51 * 7)

    const weeks: DayCell[][] = []
    const seenMonths = new Set<string>()
    const monthLabels: { weekIndex: number; month: string }[] = []
    const monthFormatter = new Intl.DateTimeFormat(toIntlLocale(i18n.language), {
      month: 'short',
    })

    for (let week = 0; week < 52; week++) {
      const days: DayCell[] = []
      for (let day = 0; day < 7; day++) {
        const d = new Date(startDate)
        d.setDate(d.getDate() + week * 7 + day)
        const dateStr = dayjs(d).format('YYYY-MM-DD')
        const record = dataMap.get(dateStr)
        const tokens = record?.total_tokens || 0
        days.push({
          date: dateStr,
          tokens,
          promptTokens: record?.prompt_tokens || 0,
          completionTokens: record?.completion_tokens || 0,
          level: getLevel(tokens, thresholds),
        })
      }
      weeks.push(days)

      const firstDayOfWeek = days[0].date
      const firstDay = dayjs(firstDayOfWeek)
      if (firstDay.date() <= 7) {
        const monthKey = firstDay.format('YYYY-M')
        if (!seenMonths.has(monthKey)) {
          seenMonths.add(monthKey)
          monthLabels.push({
            weekIndex: week,
            month: monthFormatter.format(firstDay.toDate()),
          })
        }
      }

      // Mark the first month even if its first visible day is after the 7th
      if (week === 0) {
        const firstMonthKey = firstDay.format('YYYY-M')
        if (!seenMonths.has(firstMonthKey)) {
          seenMonths.add(firstMonthKey)
          monthLabels.unshift({
            weekIndex: 0,
            month: monthFormatter.format(firstDay.toDate()),
          })
        }
      }
    }

    return { weeks, monthLabels }
  }, [data, i18n.language])

  // Generate localized short weekday names (Mon, Wed, Fri equivalents)
  const weekdayFormatter = new Intl.DateTimeFormat(toIntlLocale(i18n.language), {
    weekday: 'short',
  })
  const monLabel = weekdayFormatter.format(new Date(2024, 0, 1)) // Jan 1, 2024 = Monday
  const wedLabel = weekdayFormatter.format(new Date(2024, 0, 3)) // Jan 3, 2024 = Wednesday
  const friLabel = weekdayFormatter.format(new Date(2024, 0, 5)) // Jan 5, 2024 = Friday

  const hasActivity = data.some((item) => item.total_tokens > 0)

  // Localized long date for tooltip header (e.g. "July 25, 2026" / "2026年7月25日")
  const longDateFormatter = new Intl.DateTimeFormat(
    toIntlLocale(i18n.language),
    { year: 'numeric', month: 'long', day: 'numeric' }
  )
  const formatLongDate = (dateStr: string) => {
    const d = dayjs(dateStr).toDate()
    return Number.isNaN(d.getTime()) ? dateStr : longDateFormatter.format(d)
  }

  const handleCellEnter = (day: DayCell, e: React.MouseEvent<HTMLDivElement>) => {
    const rect = e.currentTarget.getBoundingClientRect()
    setHovered(day)
    setTooltipPos({ x: rect.left + rect.width / 2, y: rect.top, showBelow: rect.top < 120 })
  }
  const handleCellLeave = () => {
    setHovered(null)
    setTooltipPos(null)
  }

  if (loading) {
    return (
      <div
        className='bg-card rounded-lg border p-4'
        data-testid='system-heatmap'
      >
        <div className='bg-muted mb-3 h-4 w-40 animate-pulse rounded' />
        <div className='bg-muted h-[110px] animate-pulse rounded' />
      </div>
    )
  }

  return (
    <div className='bg-card rounded-lg border p-4 sm:p-5'>
      <div className='mb-4 flex flex-wrap items-center justify-between gap-3'>
        <div className='flex items-baseline gap-2'>
          <h3 className='text-sm font-medium'>{t('Token Usage Heatmap')}</h3>
          <span className='text-muted-foreground text-xs'>
            {totalTokens.toLocaleString()} {t('tokens')}
          </span>
        </div>
        <div className='text-muted-foreground flex items-center gap-1.5 text-[10px]'>
          <span>{t('Less')}</span>
          {HEATMAP_COLORS.map((c, i) => (
            <div
              key={i}
              className={`h-[10px] w-[10px] rounded-[3px] ${c}`}
            />
          ))}
          <span>{t('More')}</span>
        </div>
      </div>

      <div className='flex items-start'>
        <div className='text-muted-foreground flex flex-col gap-[3px] pt-[18px] pr-2 text-[10px] leading-none'>
          <span className='flex h-[12px] items-center'>{monLabel}</span>
          <span className='h-[12px]' />
          <span className='flex h-[12px] items-center'>{wedLabel}</span>
          <span className='h-[12px]' />
          <span className='flex h-[12px] items-center'>{friLabel}</span>
          <span className='h-[12px]' />
        </div>

        <div className='overflow-x-auto'>
          <div className='flex gap-[3px] pb-1'>
            {Array.from({ length: 52 }).map((_, i) => {
              const label = monthLabels.find((l) => l.weekIndex === i)
              return (
                <div
                  key={i}
                  className='text-muted-foreground relative w-[12px] text-[10px] leading-[12px]'
                >
                  {label && (
                    <span className='absolute left-0 whitespace-nowrap'>
                      {label.month}
                    </span>
                  )}
                </div>
              )
            })}
          </div>

          <div
            className='grid grid-flow-col grid-rows-7 gap-[3px]'
            style={{ width: 'fit-content' }}
          >
            {weeks.flat().map((day, i) => (
              <div
                key={i}
                className={`relative h-[12px] w-[12px] rounded-[3px] transition-[outline] duration-150 hover:z-10 hover:outline-1 hover:outline-foreground/40 ${HEATMAP_COLORS[day.level]}`}
                role='gridcell'
                aria-label={`${day.date}: ${day.tokens.toLocaleString()} ${t('tokens')}`}
                onMouseEnter={(e) => handleCellEnter(day, e)}
                onMouseLeave={handleCellLeave}
              />
            ))}
          </div>
        </div>
      </div>

      {!hasActivity && (
        <p className='text-muted-foreground mt-3 text-center text-xs'>
          {t('No token usage data in this period')}
        </p>
      )}

      {hovered && tooltipPos && (
        <div
          role='tooltip'
          className='bg-foreground text-background pointer-events-none fixed z-50 min-w-[160px] rounded-md px-3 py-2 text-xs shadow-lg'
          style={{
            left: tooltipPos.x,
            top: tooltipPos.showBelow ? tooltipPos.y + 14 : tooltipPos.y - 10,
            transform: tooltipPos.showBelow
              ? 'translate(-50%, 0)'
              : 'translate(-50%, -100%)',
          }}
        >
          <div className='font-medium'>{formatLongDate(hovered.date)}</div>
          {hovered.tokens > 0 ? (
            <div className='mt-1.5 space-y-1'>
              <div className='flex items-center justify-between gap-3'>
                <span className='text-background/60'>{t('Total')}</span>
                <span className='font-medium tabular-nums'>
                  {hovered.tokens.toLocaleString()} {t('tokens')}
                </span>
              </div>
              <div className='flex items-center justify-between gap-3'>
                <span className='text-background/60'>{t('Prompt')}</span>
                <span className='tabular-nums'>
                  {hovered.promptTokens.toLocaleString()}
                </span>
              </div>
              <div className='flex items-center justify-between gap-3'>
                <span className='text-background/60'>{t('Completion')}</span>
                <span className='tabular-nums'>
                  {hovered.completionTokens.toLocaleString()}
                </span>
              </div>
            </div>
          ) : (
            <div className='text-background/60 mt-1'>
              {t('No token usage data in this period')}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
