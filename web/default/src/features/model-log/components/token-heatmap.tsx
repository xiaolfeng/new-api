import { useTranslation } from 'react-i18next'
import { api } from '@/lib/api'
import { useEffect, useState, useMemo } from 'react'
import type { TokenRecordDailyItem } from '../types'

interface DayCell {
  date: string
  tokens: number
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
  const { t } = useTranslation()
  const [data, setData] = useState<TokenRecordDailyItem[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    const fetchData = async () => {
      setLoading(true)
      try {
        const res = await api.get('/api/token_record/daily')
        if (!cancelled && res.data?.success !== false) {
          const payload = res.data?.data ?? res.data
          setData(Array.isArray(payload) ? payload : [])
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

    for (let week = 0; week < 52; week++) {
      const days: DayCell[] = []
      for (let day = 0; day < 7; day++) {
        const d = new Date(startDate)
        d.setDate(d.getDate() + week * 7 + day)
        const dateStr = d.toISOString().slice(0, 10)
        const tokens = dataMap.get(dateStr)?.total_tokens || 0

        days.push({
          date: dateStr,
          tokens,
          level: getLevel(tokens, thresholds),
        })
      }
      weeks.push(days)

      const firstDayOfWeek = days[0].date
      const d = new Date(firstDayOfWeek)
      if (d.getDate() <= 7) {
        const monthKey = `${d.getFullYear()}-${d.getMonth()}`
        if (!seenMonths.has(monthKey)) {
          seenMonths.add(monthKey)
          monthLabels.push({
            weekIndex: week,
            month: d.toLocaleString('en-US', { month: 'short' }),
          })
        }
      }
    }

    return { weeks, monthLabels }
  }, [data])

  if (loading) {
    return (
    <div className='rounded-xl border bg-card p-4' data-testid='system-heatmap'>
        <div className='bg-muted mb-3 h-4 w-40 animate-pulse rounded' />
        <div className='bg-muted h-[110px] animate-pulse rounded' />
      </div>
    )
  }

  const gridHeight = 7 * 11 + 6 * 3

  return (
    <div className='rounded-xl border bg-card p-4'>
      <h3 className='mb-3 text-sm font-medium'>{t('Token Usage Heatmap')}</h3>

      <div className='flex items-start'>
        <div
          className='text-muted-foreground flex flex-col justify-between py-0 pr-2 text-[10px] leading-[11px]'
          style={{ height: `${gridHeight}px` }}
        >
          <span>Mon</span>
          <span>Wed</span>
          <span>Fri</span>
        </div>

        <div className='overflow-x-auto'>
          <div className='flex gap-[3px] pb-1'>
            {Array.from({ length: 52 }).map((_, i) => {
              const label = monthLabels.find((l) => l.weekIndex === i)
              return (
                <div
                  key={i}
                  className='text-muted-foreground relative w-[11px] text-[10px] leading-[11px]'
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
                className={`h-[11px] w-[11px] rounded-sm ${HEATMAP_COLORS[day.level]}`}
                title={`${day.date}: ${day.tokens.toLocaleString()} tokens`}
              />
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
